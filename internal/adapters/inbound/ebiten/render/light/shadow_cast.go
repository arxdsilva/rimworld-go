package light

import (
	"cmp"
	_ "embed"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Realistic creature shadows (ART-012): pawns and animals cast their own
// silhouette, drawn by the same figure code with a Cast set, projected
// from the feet away from the light, softened with distance by
// shadow_blur.kage, plus a contact shadow and campfire/lamp shadows.
//
// The projection is one affine map per caster about its feet pivot
// (sprite-local: +x right, +y down, screen px):
//
//	[x']   [ Px   −Dx·m·κ  ] [x]
//	[y'] = [ Py   −Dy·m·κ·ρ] [y]
//
// (Dx, Dy) is the unit ground direction the shadow falls, m its slope
// (shadow length per tile of height: ShadowLen(h)/h for the sun or moon,
// tip/h for a point light), κ the sprite's height per pixel (tiles of
// height per tile of screen, from its alpha bounds) and ρ the extra
// y-foreshortening. The sprite's width maps unscaled onto P = (−Dy, Dx),
// the axis across the shadow: under a southern sun P = (1, 0) and x is
// untouched; under a level east or west light the width becomes the
// shadow's north-south thickness, where keeping it on x (as the plan
// first had it) folded the silhouette into a hairline along its length.

//go:embed shadow_blur.kage
var shadowBlurKage []byte

// castKage draws a silhouette: the vertex colour (height in R, the
// light's alpha in G, B and A, premultiplied) times the sprite's
// coverage, sampled bilinearly. A plain DrawTriangles would multiply in
// the sprite's own colour.
const castKage = `//kage:unit pixels

package main

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	p := srcPos - 0.5
	f := fract(p)
	b := floor(p) + 0.5
	a00 := imageSrc0At(b).a
	a10 := imageSrc0At(b + vec2(1, 0)).a
	a01 := imageSrc0At(b + vec2(0, 1)).a
	a11 := imageSrc0At(b + vec2(1, 1)).a
	return color * mix(mix(a00, a10, f.x), mix(a01, a11, f.x), f.y)
}
`

const (
	// shadowRho is the y-foreshortening applied on top of ShadowDir. The
	// sun's ShadowDir is already foreshortened (LightDir's ProjShearK on y), and
	// box and blob shadows use it as is, so silhouettes do too (ρ = 1);
	// a second foreshortening turned creature shadows away from the walls' beside
	// them.
	shadowRho = 1.0
	// LyingKappa flattens a body lying on the ground (≈0.25 tiles thick).
	LyingKappa = 0.25

	// contact shadows: a soft pool under the feet (ambient occlusion)
	contactWidthK    = 0.9
	contactDepth     = 0.22 // tiles
	contactSunK      = 0.55
	contactMinAlpha  = 0.18
	contactRoofAlpha = 0.20
	contactLyingW    = 0.8 // tiles: a body lying on the ground

	// point lights: campfires and lamps
	CampfireLightH     = 0.5 // tiles above the ground
	CampfireLightReach = 4.5
	LampLightH         = 1.6
	LampLightReach     = LampDefaultRadius // a lamp's reach is its light radius
	pointShadowAlpha   = 0.30
	pointMinTipK       = 0.3 // the shortest tip, × the caster's height
	flickerAlpha       = 0.08
	flickerLen         = 0.05
	daylightFullEl     = 15.0 // degrees: point-light shadows gone at this sun
)

// Cast, while set on the renderer (passed to Lit.Draw), makes a lit draw
// a silhouette into the shadow layer instead of a lit sprite onto the
// screen.
type Cast struct {
	Layer *ebiten.Image // the frame's shadow layer
	Proj  ebiten.GeoM   // pivot-anchored projection for the current caster
	Alpha float32       // this light's strength (sun, moon or a point light)
	// PivotY and HeightPx encode height in red: 0 on the caster's ground
	// line (screen y, before the projection), 1 HeightPx above it.
	PivotY, HeightPx float64
	// OnDraw, when set (tests), sees every image drawn in cast mode.
	OnDraw func(img *ebiten.Image)

	shader *ebiten.Shader
	verts  [4]ebiten.Vertex
	opts   ebiten.DrawTrianglesShaderOptions
}

// Silhouette draws img (placed by geo) as a projected silhouette into
// c's layer: unlit, max-blended, height in R for the blur.
func (c *Cast) Silhouette(img *ebiten.Image, geo ebiten.GeoM) {
	if c.OnDraw != nil {
		c.OnDraw(img)
	}
	if c.shader == nil || c.Layer == nil || c.Alpha <= 0 {
		return
	}
	b := img.Bounds()
	CastQuad(&c.verts, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), geo, c)
	c.opts.Images[0] = img
	c.opts.Blend = ShadowBlend
	c.Layer.DrawTrianglesShader32(c.verts[:], Indices[:], c.shader, &c.opts)
}

// CastQuad writes the silhouette vertices (top-left, top-right,
// bottom-left, bottom-right) of the source rectangle (sx, sy, w, h)
// placed by geo: projected by c.proj, coloured (H, 1, 1, 1)·alpha
// premultiplied, where H is the vertex's height above c's ground line
// as a share of c.heightPx (clamped 0..1).
func CastQuad(vs *[4]ebiten.Vertex, sx, sy, w, h float32, geo ebiten.GeoM, c *Cast) {
	a := c.Alpha
	for i := range vs {
		u, v := float32(i&1)*w, float32(i>>1)*h
		x, y := geo.Apply(float64(u), float64(v))
		hgt := 0.0
		if c.HeightPx > 0 {
			hgt = gfx.Clamp01((c.PivotY - y) / c.HeightPx)
		}
		dx, dy := c.Proj.Apply(x, y)
		vs[i] = ebiten.Vertex{
			DstX: float32(dx), DstY: float32(dy),
			SrcX: sx + u, SrcY: sy + v,
			ColorR: float32(hgt) * a, ColorG: a, ColorB: a, ColorA: a,
		}
	}
}

// SpriteKappa is κ for a caster h tiles tall whose opaque part is
// opaquePx source pixels tall at ppt pixels per tile (its metadata
// density, PLAY-022): tiles of height per tile of sprite.
func SpriteKappa(h, opaquePx, ppt float64) float64 {
	if opaquePx <= 0 || ppt <= 0 {
		return 0
	}
	return h * ppt / opaquePx
}

// CastProjection is the silhouette projection about the screen pivot
// (pivotX, pivotY) for a shadow falling along (dirX, dirY) with slope m.
func CastProjection(pivotX, pivotY, kappa, dirX, dirY, m, rho float64) ebiten.GeoM {
	px, py := 1.0, 0.0 // across the shadow
	if n := math.Hypot(dirX, dirY); n > 1e-9 {
		px, py = -dirY/n, dirX/n
	}
	var mat ebiten.GeoM
	mat.SetElement(0, 0, px)
	mat.SetElement(1, 0, py)
	mat.SetElement(0, 1, -dirX*m*kappa)
	mat.SetElement(1, 1, -dirY*m*kappa*rho)
	var g ebiten.GeoM
	g.Translate(-pivotX, -pivotY)
	g.Concat(mat)
	g.Translate(pivotX, pivotY)
	return g
}

// ShadowProjection is the projection of a caster hRef tiles tall under
// the sun or moon: slope ShadowLen(hRef)/hRef, so the top of a caster
// lands exactly at ShadowLen(hRef) (the existing clamp included).
func ShadowProjection(pivotX, pivotY, kappa float64, light State, hRef, rho float64) ebiten.GeoM {
	m := 0.0
	if light.ShadowLen != nil && hRef > 0 {
		m = light.ShadowLen(hRef) / hRef
	}
	return CastProjection(pivotX, pivotY, kappa, light.ShadowDir[0], light.ShadowDir[1], m, rho)
}

// --- the light plan -------------------------------------------------------

// PointLight is a campfire or lamp casting creature shadows: ground
// position (tiles), height h, reach, the flicker phase (−1..1) and, for a
// lamp, 1 + its index in the light grid's sources (0: a campfire, whose
// reach no wall stops).
type PointLight struct {
	ID                 uint64
	X, Y, H, Reach, fl float64
	lamp               int
}

// CastLight is one silhouette to draw: the shadow's direction, slope and
// alpha.
type CastLight struct {
	DirX, DirY, M, Alpha float64
}

// Plan is what one creature casts this frame.
type Plan struct {
	Sun     bool // a sun or moon silhouette (drawn before the roof clear)
	SunCast CastLight
	Points  [2]CastLight // point-light silhouettes (after the clear)
	Np      int
	Contact float64 // the contact pool's alpha
}

// PlanCreature plans a creature h tiles tall standing at (px, py) (its
// feet, tiles): a sun or moon silhouette on open sky while there are
// shadows, the two nearest point lights in reach (fading out with
// daylight: the caller passes 0 for a roofed creature, so lamps indoors
// cast by day too), and a contact pool always.
func PlanCreature(px, py, h float64, sky bool, l State, daylight float64, lights []PointLight) Plan {
	var pl Plan
	sa := gfx.Clamp01(l.ShadowAlpha)
	if sky && sa > 0 && l.ShadowLen != nil && h > 0 {
		pl.Sun = true
		pl.SunCast = CastLight{l.ShadowDir[0], l.ShadowDir[1], l.ShadowLen(h) / h, sa}
	}
	if daylight < 1 && h > 0 {
		var idx [2]int
		n := NearestLights(px, py, lights, &idx)
		for i := 0; i < n; i++ {
			if cl, ok := PointCast(px, py, h, lights[idx[i]], daylight); ok {
				pl.Points[pl.Np] = cl
				pl.Np++
			}
		}
	}
	pl.Contact = contactRoofAlpha
	if sky {
		pl.Contact = math.Max(contactSunK*sa, contactMinAlpha)
	}
	return pl
}

// NearestLights writes the indices of the (up to) two lights in reach
// of (px, py) nearest to it, by distance then id, and returns how many.
func NearestLights(px, py float64, lights []PointLight, out *[2]int) int {
	n := 0
	var dist [2]float64
	better := func(d float64, id uint64, j int) bool {
		return d < dist[j] || d == dist[j] && id < lights[out[j]].ID
	}
	for i, L := range lights {
		d := math.Hypot(px-L.X, py-L.Y)
		if d > L.Reach {
			continue
		}
		switch {
		case n < 2:
			out[n], dist[n] = i, d
			n++
			if n == 2 && better(dist[1], lights[out[1]].ID, 0) {
				out[0], out[1], dist[0], dist[1] = out[1], out[0], dist[1], dist[0]
			}
		case better(d, L.ID, 0):
			out[1], dist[1] = out[0], dist[0]
			out[0], dist[0] = i, d
		case better(d, L.ID, 1):
			out[1], dist[1] = i, d
		}
	}
	return n
}

// PointCast is a point light's shadow of a creature h tiles tall at (px,
// py): away from the light, with the tip at d·h/(H−h) (the longest when
// the light is no higher than the caster), clamped to [0.3h, 3] tiles,
// at 0.30·falloff·(1−daylight); flicker sways both.
func PointCast(px, py, h float64, L PointLight, daylight float64) (CastLight, bool) {
	dx, dy := px-L.X, py-L.Y
	d := math.Hypot(dx, dy)
	if d > L.Reach || d < 1e-6 || h <= 0 {
		return CastLight{}, false
	}
	a := pointShadowAlpha * (1 - d/L.Reach) * (1 - gfx.Clamp01(daylight)) * (1 + flickerAlpha*L.fl)
	if a <= 0.005 {
		return CastLight{}, false
	}
	tip := ShadowMaxLen
	if h < L.H {
		tip = d * h / (L.H - h)
	}
	tip = math.Min(math.Max(tip*(1+flickerLen*L.fl), pointMinTipK*h), ShadowMaxLen)
	return CastLight{DirX: dx / d, DirY: dy / d, M: tip / h, Alpha: a}, true
}

// SunElevation is the sun's elevation (radians) in l, 0 below the
// horizon: SunL is LightDir's unit vector, y foreshortened by ProjShearK.
func SunElevation(l State) float64 {
	if l.SunL[2] <= 0 {
		return 0
	}
	return math.Atan2(float64(l.SunL[2]), math.Hypot(float64(l.SunL[0]), float64(l.SunL[1])/ProjShearK))
}

// DaylightOf is how much the sun at elevation el (radians) washes out
// point-light shadows: 0 at the horizon, 1 from 15°.
func DaylightOf(el float64) float64 {
	return gfx.Clamp01(el * 180 / math.Pi / daylightFullEl)
}

// BlurRadii are the penumbra blur's radius terms in screen px, r0 + r1·H,
// at a zoom (tile px over TileSize) and sun elevation ratio (0 at the
// horizon or under the moon, 1 at the season's noon): low sun is softer.
func BlurRadii(zoom, ratio float64) (r0, r1 float64) {
	return 0.6 * zoom, 3.5 * zoom * (1 + 0.6*(1-gfx.Clamp01(ratio)))
}

// ShadowTint is the shadow colour (0..255): bluer at low sun.
func ShadowTint(ratio float64) [3]float32 {
	return [3]float32{ShadowColor[0], ShadowColor[1], 34 - 4*float32(gfx.Clamp01(ratio))}
}

// FireFlicker is a campfire's flicker (−1..1) on the presentation clock,
// the flame's own (drawFlameScreen) recentred.
func FireFlicker(t float64) float64 {
	return math.Sin(t*0.31) * math.Sin(t*0.13+2)
}

// --- the creature pass ----------------------------------------------------

// Creature is a pawn or animal casting this frame. The renderer fills it
// (Casters.Creatures); Fig is its figure pose, resolved once per frame,
// for drawing the silhouette (Casters.CastFigure).
type Creature[F any] struct {
	Pawn   *game.PawnView   // or nil
	Animal *game.AnimalView // or nil
	WX, WY float64          // drawn tile corner (interpolated)
	Px, Py float64          // the feet: the ground pivot, tiles
	W, H   float64          // footprint width and height, tiles
	Opaque float64          // opaque height, source px (κ = h/opaque)
	PPT    float64          // the sprite's px per tile (art.Meta.PxPerTile)
	Lying  bool             // flat on the ground
	Sky    bool             // on open-sky ground
	Tx, Ty int              // the tile it stands on (sky, lamp reach)
	Fig    F
}

// castCreature draws c's figure as a silhouette for one light into the
// shadow layer: host runs the figure code in cast mode, its draws
// projected from c's feet.
func (st *Shadows[F]) castCreature(v *render.View, host Casters[F], c *Creature[F], cl CastLight, tile float64) {
	cs := &st.cast
	if !st.castInit {
		st.castInit = true
		cs.shader, _ = ebiten.NewShader([]byte(castKage)) // nil: no silhouettes, contact pools stay
	}
	sx, sy := v.WorldToScreenF(c.Px, c.Py)
	k := SpriteKappa(c.H, c.Opaque, cmp.Or(c.PPT, art.TilePx))
	if c.Lying {
		k *= LyingKappa
	}
	cs.Layer = st.Layer
	cs.Proj = CastProjection(float64(sx), float64(sy), k, cl.DirX, cl.DirY, cl.M, shadowRho)
	if k := st.scale; k != 0 && k != 1 {
		cs.Proj.Scale(k, k) // the layer is smaller than the screen
	}
	cs.Alpha = float32(cl.Alpha)
	cs.PivotY = float64(sy)
	cs.HeightPx = c.Opaque * tile / cmp.Or(c.PPT, art.TilePx)
	host.CastFigure(c, cs, st.Scratch, tile)
	st.Casts++
}

// addContact queues c's contact pool: a soft ellipse under the feet at
// alpha a, footprint width × 0.9 by 0.22 tiles (wider for a body lying
// on the ground), with no height so it stays crisp.
func (st *Shadows[F]) addContact(v *render.View, c *Creature[F], a float64, tex *ebiten.Image) {
	if a <= 0 {
		return
	}
	cw := c.W * contactWidthK
	if c.Lying {
		cw = math.Max(cw, contactLyingW)
	}
	x0, x1 := c.Px-cw/2, c.Px+cw/2
	y0, y1 := c.Py-contactDepth/2, c.Py+contactDepth/2
	addTexQuad(v, &st.Contacts, [4][2]float64{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}, tex, a)
}

// PointLights lists the frame's creature-shadow lights near view: the
// light sources (light_sources.go, through lit's grid synced to v) —
// whatever glows now: burning campfires and torch lamps, and lamps while
// powered (PLAY-021). Daylight no longer empties the list: Draw washes it
// out per creature, only on open sky, so roofed rooms keep their lamp
// shadows by day. Campfires flicker on v.Present.
func (st *Shadows[F]) PointLights(v *render.View, lit *Lit, view Rect) []PointLight {
	out := st.lights[:0]
	g := lit.Sync(v)
	for k := range g.srcs {
		s := &g.srcs[k]
		var L PointLight
		switch s.Kind {
		case Campfire:
			L = PointLight{H: CampfireLightH, Reach: CampfireLightReach, fl: FireFlicker(float64(v.Present) + float64(s.ID)*17)}
		case Lamp:
			L = PointLight{H: LampLightH, Reach: s.Radius, lamp: 1 + k}
		default:
			continue
		}
		L.ID = s.ID
		L.X, L.Y = float64(s.X)+0.5, float64(s.Y)+0.5
		mg := L.Reach + ShadowMaxLen + 1
		if !view.Empty() && (L.X < view.X0-mg || L.Y < view.Y0-mg || L.X > view.X1+mg || L.Y > view.Y1+mg) {
			continue
		}
		out = append(out, L)
	}
	st.lights = out
	return out
}

// creatureLights writes into st.vis the lights in ls that reach tile (tx,
// ty), a creature's, by the light grid g (a lamp stops at walls and rooms,
// so no lamp shadow falls through a wall; campfires keep their reach), and
// returns them.
func (st *Shadows[F]) creatureLights(g *Grid, tx, ty int, ls []PointLight) []PointLight {
	vis := st.vis[:0]
	for _, L := range ls {
		if L.lamp == 0 || g.lampReaches(L.lamp-1, tx, ty) {
			vis = append(vis, L)
		}
	}
	st.vis = vis
	return vis
}
