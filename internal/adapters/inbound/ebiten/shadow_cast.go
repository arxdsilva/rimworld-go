package ebiten

import (
	_ "embed"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Realistic creature shadows (ART-012): pawns and animals cast their own
// silhouette, drawn by the same figure code with r.cast set, projected
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
	// sun's ShadowDir is already foreshortened (lightDir's projShearK on y), and
	// box and blob shadows use it as is, so silhouettes do too (ρ = 1);
	// a second foreshortening turned creature shadows away from the walls' beside
	// them.
	shadowRho = 1.0
	// lyingKappa flattens a body lying on the ground (≈0.25 tiles thick).
	lyingKappa = 0.25

	// contact shadows: a soft pool under the feet (ambient occlusion)
	contactWidthK    = 0.9
	contactDepth     = 0.22 // tiles
	contactSunK      = 0.55
	contactMinAlpha  = 0.18
	contactRoofAlpha = 0.20
	contactLyingW    = 0.8 // tiles: a body lying on the ground

	// point lights: campfires and lamps
	campfireLightH     = 0.5 // tiles above the ground
	campfireLightReach = 4.5
	lampLightH         = 1.6
	lampLightReach     = lampDefaultRadius // a lamp's reach is its light radius
	pointShadowAlpha   = 0.30
	pointMinTipK       = 0.3 // the shortest tip, × the caster's height
	flickerAlpha       = 0.08
	flickerLen         = 0.05
	daylightFullEl     = 15.0 // degrees: point-light shadows gone at this sun
)

// shadowCast, while set on the renderer, makes drawLit draw silhouettes
// into the shadow layer instead of lit sprites onto the screen.
type shadowCast struct {
	layer *ebiten.Image // the frame's shadow layer
	proj  ebiten.GeoM   // pivot-anchored projection for the current caster
	alpha float32       // this light's strength (sun, moon or a point light)
	// pivotY and heightPx encode height in red: 0 on the caster's ground
	// line (screen y, before the projection), 1 heightPx above it.
	pivotY, heightPx float64
	// onDraw, when set (tests), sees every image drawn in cast mode.
	onDraw func(img *ebiten.Image)

	shader *ebiten.Shader
	verts  [4]ebiten.Vertex
	opts   ebiten.DrawTrianglesShaderOptions
}

// castSilhouette draws img (placed by geo) as a projected silhouette into
// r.cast's layer: unlit, max-blended, height in R for the blur.
func (r *Renderer) castSilhouette(img *ebiten.Image, geo ebiten.GeoM) {
	c := r.cast
	if c.onDraw != nil {
		c.onDraw(img)
	}
	if c.shader == nil || c.layer == nil || c.alpha <= 0 {
		return
	}
	b := img.Bounds()
	castQuad(&c.verts, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), geo, c)
	c.opts.Images[0] = img
	c.opts.Blend = shadowBlend
	c.layer.DrawTrianglesShader32(c.verts[:], litIndices[:], c.shader, &c.opts)
}

// castQuad writes the silhouette vertices (top-left, top-right,
// bottom-left, bottom-right) of the source rectangle (sx, sy, w, h)
// placed by geo: projected by c.proj, coloured (H, 1, 1, 1)·alpha
// premultiplied, where H is the vertex's height above c's ground line
// as a share of c.heightPx (clamped 0..1).
func castQuad(vs *[4]ebiten.Vertex, sx, sy, w, h float32, geo ebiten.GeoM, c *shadowCast) {
	a := c.alpha
	for i := range vs {
		u, v := float32(i&1)*w, float32(i>>1)*h
		x, y := geo.Apply(float64(u), float64(v))
		hgt := 0.0
		if c.heightPx > 0 {
			hgt = clamp01((c.pivotY - y) / c.heightPx)
		}
		dx, dy := c.proj.Apply(x, y)
		vs[i] = ebiten.Vertex{
			DstX: float32(dx), DstY: float32(dy),
			SrcX: sx + u, SrcY: sy + v,
			ColorR: float32(hgt) * a, ColorG: a, ColorB: a, ColorA: a,
		}
	}
}

// spriteKappa is κ for a caster h tiles tall whose opaque part is
// opaquePx source pixels tall: tiles of height per tile of sprite.
func spriteKappa(h, opaquePx float64) float64 {
	if opaquePx <= 0 {
		return 0
	}
	return h * artTilePx / opaquePx
}

// castProjection is the silhouette projection about the screen pivot
// (pivotX, pivotY) for a shadow falling along (dirX, dirY) with slope m.
func castProjection(pivotX, pivotY, kappa, dirX, dirY, m, rho float64) ebiten.GeoM {
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

// shadowProjection is the projection of a caster hRef tiles tall under
// the sun or moon: slope ShadowLen(hRef)/hRef, so the top of a caster
// lands exactly at ShadowLen(hRef) (the existing clamp included).
func shadowProjection(pivotX, pivotY, kappa float64, light lightState, hRef, rho float64) ebiten.GeoM {
	m := 0.0
	if light.ShadowLen != nil && hRef > 0 {
		m = light.ShadowLen(hRef) / hRef
	}
	return castProjection(pivotX, pivotY, kappa, light.ShadowDir[0], light.ShadowDir[1], m, rho)
}

// --- the light plan -------------------------------------------------------

// pointLight is a campfire or lamp casting creature shadows: ground
// position (tiles), height h, reach, the flicker phase (−1..1) and, for a
// lamp, 1 + its index in the light grid's sources (0: a campfire, whose
// reach no wall stops).
type pointLight struct {
	id                 uint64
	x, y, h, reach, fl float64
	lamp               int
}

// castLight is one silhouette to draw: the shadow's direction, slope and
// alpha.
type castLight struct {
	dirX, dirY, m, alpha float64
}

// creaturePlan is what one creature casts this frame.
type creaturePlan struct {
	sun     bool // a sun or moon silhouette (drawn before the roof clear)
	sunCast castLight
	points  [2]castLight // point-light silhouettes (after the clear)
	np      int
	contact float64 // the contact pool's alpha
}

// planCreature plans a creature h tiles tall standing at (px, py) (its
// feet, tiles): a sun or moon silhouette on open sky while there are
// shadows, the two nearest point lights in reach (fading out with
// daylight: the caller passes 0 for a roofed creature, so lamps indoors
// cast by day too), and a contact pool always.
func planCreature(px, py, h float64, sky bool, l lightState, daylight float64, lights []pointLight) creaturePlan {
	var pl creaturePlan
	sa := clamp01(l.ShadowAlpha)
	if sky && sa > 0 && l.ShadowLen != nil && h > 0 {
		pl.sun = true
		pl.sunCast = castLight{l.ShadowDir[0], l.ShadowDir[1], l.ShadowLen(h) / h, sa}
	}
	if daylight < 1 && h > 0 {
		var idx [2]int
		n := nearestLights(px, py, lights, &idx)
		for i := 0; i < n; i++ {
			if cl, ok := pointCast(px, py, h, lights[idx[i]], daylight); ok {
				pl.points[pl.np] = cl
				pl.np++
			}
		}
	}
	pl.contact = contactRoofAlpha
	if sky {
		pl.contact = math.Max(contactSunK*sa, contactMinAlpha)
	}
	return pl
}

// nearestLights writes the indices of the (up to) two lights in reach
// of (px, py) nearest to it, by distance then id, and returns how many.
func nearestLights(px, py float64, lights []pointLight, out *[2]int) int {
	n := 0
	var dist [2]float64
	better := func(d float64, id uint64, j int) bool {
		return d < dist[j] || d == dist[j] && id < lights[out[j]].id
	}
	for i, L := range lights {
		d := math.Hypot(px-L.x, py-L.y)
		if d > L.reach {
			continue
		}
		switch {
		case n < 2:
			out[n], dist[n] = i, d
			n++
			if n == 2 && better(dist[1], lights[out[1]].id, 0) {
				out[0], out[1], dist[0], dist[1] = out[1], out[0], dist[1], dist[0]
			}
		case better(d, L.id, 0):
			out[1], dist[1] = out[0], dist[0]
			out[0], dist[0] = i, d
		case better(d, L.id, 1):
			out[1], dist[1] = i, d
		}
	}
	return n
}

// pointCast is a point light's shadow of a creature h tiles tall at (px,
// py): away from the light, with the tip at d·h/(H−h) (the longest when
// the light is no higher than the caster), clamped to [0.3h, 3] tiles,
// at 0.30·falloff·(1−daylight); flicker sways both.
func pointCast(px, py, h float64, L pointLight, daylight float64) (castLight, bool) {
	dx, dy := px-L.x, py-L.y
	d := math.Hypot(dx, dy)
	if d > L.reach || d < 1e-6 || h <= 0 {
		return castLight{}, false
	}
	a := pointShadowAlpha * (1 - d/L.reach) * (1 - clamp01(daylight)) * (1 + flickerAlpha*L.fl)
	if a <= 0.005 {
		return castLight{}, false
	}
	tip := shadowMaxLen
	if h < L.h {
		tip = d * h / (L.h - h)
	}
	tip = math.Min(math.Max(tip*(1+flickerLen*L.fl), pointMinTipK*h), shadowMaxLen)
	return castLight{dirX: dx / d, dirY: dy / d, m: tip / h, alpha: a}, true
}

// sunElevation is the sun's elevation (radians) in l, 0 below the
// horizon: SunL is lightDir's unit vector, y foreshortened by projShearK.
func sunElevation(l lightState) float64 {
	if l.SunL[2] <= 0 {
		return 0
	}
	return math.Atan2(float64(l.SunL[2]), math.Hypot(float64(l.SunL[0]), float64(l.SunL[1])/projShearK))
}

// daylightOf is how much the sun at elevation el (radians) washes out
// point-light shadows: 0 at the horizon, 1 from 15°.
func daylightOf(el float64) float64 {
	return clamp01(el * 180 / math.Pi / daylightFullEl)
}

// blurRadii are the penumbra blur's radius terms in screen px, r0 + r1·H,
// at a zoom (tile px over TileSize) and sun elevation ratio (0 at the
// horizon or under the moon, 1 at the season's noon): low sun is softer.
func blurRadii(zoom, ratio float64) (r0, r1 float64) {
	return 0.6 * zoom, 3.5 * zoom * (1 + 0.6*(1-clamp01(ratio)))
}

// shadowTint is the shadow colour (0..255): bluer at low sun.
func shadowTint(ratio float64) [3]float32 {
	return [3]float32{shadowColor[0], shadowColor[1], 34 - 4*float32(clamp01(ratio))}
}

// fireFlicker is a campfire's flicker (−1..1) on the presentation clock,
// the flame's own (drawFlameScreen) recentred.
func fireFlicker(t float64) float64 {
	return math.Sin(t*0.31) * math.Sin(t*0.13+2)
}

// --- the creature pass ----------------------------------------------------

// creatureCaster is a pawn or animal casting this frame.
type creatureCaster struct {
	pawn   *game.PawnView   // or nil
	animal *game.AnimalView // or nil
	wx, wy float64          // drawn tile corner (interpolated)
	px, py float64          // the feet: the ground pivot, tiles
	w, h   float64          // footprint width and height, tiles
	opaque float64          // opaque height, source px (κ = h/opaque)
	lying  bool             // flat on the ground
	sky    bool             // on open-sky ground
	tx, ty int              // the tile it stands on (sky, lamp reach)
	fp     pawnFigurePose
}

// creatureCasters lists the pawns and animals in view (widened by the
// longest shadow) that cast: not in bed, carried (the rescuer's figure
// draws them), or an embodied corpse, and with art.
func (r *Renderer) creatureCasters(w game.WorldView, pawns []game.PawnView) []creatureCaster {
	st := &r.env.shadow
	out := st.creatures[:0]
	v := r.env.view
	mg := shadowMaxLen + 1
	inView := func(x, y float64) bool {
		return v.empty() || x+1 >= v.x0-mg && y+1 >= v.y0-mg && x <= v.x1+mg && y <= v.y1+mg
	}
	add := func(c creatureCaster, key string, feetX, feetY float64) {
		sp := r.art.sprite(key)
		if sp == nil {
			return // no art (the fallback marker casts nothing)
		}
		s := r.shadowSpecOf(key)
		if s.h <= 0 {
			return
		}
		c.w, c.h, c.opaque = s.w, s.h, sp.opaqueHeightPx()
		c.px, c.py = c.wx+feetX, c.wy+feetY
		c.tx, c.ty = int(math.Floor(c.wx+0.5)), int(math.Floor(c.wy+0.5))
		c.sky = skyOpen(w, c.tx, c.ty)
		out = append(out, c)
	}
	for i := range pawns {
		p := &pawns[i]
		if p.Embodied || p.CarriedBy != 0 || p.Lying && p.InBed {
			continue
		}
		wx, wy := r.interpWorld(*p)
		if !inView(wx, wy) {
			continue
		}
		add(creatureCaster{pawn: p, wx: wx, wy: wy, lying: figureLying(*p), fp: r.pawnFigurePoseOf(*p)}, "pawn/body_south", pawnFeetX, pawnFeetY)
	}
	for i := range w.Animals {
		an := &w.Animals[i]
		// cull on the cell first: the drawn position stays within a few
		// tiles of it (interpolation and corrections), and most of a big
		// map's animals are far off screen
		if fx, fy := float64(an.X), float64(an.Y); !v.empty() &&
			(fx+1+drawnReach < v.x0-mg || fy+1+drawnReach < v.y0-mg || fx-drawnReach > v.x1+mg || fy-drawnReach > v.y1+mg) {
			continue
		}
		wx, wy := r.interpAnimal(*an)
		if !inView(wx, wy) {
			continue
		}
		add(creatureCaster{animal: an, wx: wx, wy: wy}, r.animalBaseKey(*an), figurePivotX, figurePivotY)
	}
	st.creatures = out
	return out
}

// castCreature draws c's figure as a silhouette for one light into the
// shadow layer: the figure code runs in cast mode, its draws projected
// from c's feet.
func (r *Renderer) castCreature(w game.WorldView, c creatureCaster, cl castLight, tile float64) {
	st := &r.env.shadow
	cs := &st.cast
	if !st.castInit {
		st.castInit = true
		cs.shader, _ = ebiten.NewShader([]byte(castKage)) // nil: no silhouettes, contact pools stay
	}
	sx, sy := r.worldToScreenF(c.px, c.py)
	k := spriteKappa(c.h, c.opaque)
	if c.lying {
		k *= lyingKappa
	}
	cs.layer = st.layer
	cs.proj = castProjection(float64(sx), float64(sy), k, cl.dirX, cl.dirY, cl.m, shadowRho)
	cs.alpha = float32(cl.alpha)
	cs.pivotY = float64(sy)
	cs.heightPx = c.opaque * tile / artTilePx
	r.cast = cs
	if c.pawn != nil {
		r.drawPawnFigure(st.scratch, *c.pawn, c.wx, c.wy, tile, c.fp)
	} else {
		r.drawAnimalFigure(st.scratch, *c.animal, c.wx, c.wy, tile)
	}
	r.cast = nil
	st.casts++
}

// addContact queues c's contact pool: a soft ellipse under the feet at
// alpha a, footprint width × 0.9 by 0.22 tiles (wider for a body lying
// on the ground), with no height so it stays crisp.
func (r *Renderer) addContact(b *triBatch, c creatureCaster, a float64, tex *ebiten.Image) {
	if a <= 0 {
		return
	}
	cw := c.w * contactWidthK
	if c.lying {
		cw = math.Max(cw, contactLyingW)
	}
	x0, x1 := c.px-cw/2, c.px+cw/2
	y0, y1 := c.py-contactDepth/2, c.py+contactDepth/2
	r.addTexQuad(b, [4][2]float64{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}, tex, a)
}

// pointLights lists the frame's creature-shadow lights near the view:
// the light sources (light_sources.go, through the synced light grid) —
// whatever glows now: burning campfires and torch lamps, and lamps while
// powered (PLAY-021). Daylight no
// longer empties the list: drawShadows washes it out per creature, only
// on open sky, so roofed rooms keep their lamp shadows by day.
func (r *Renderer) pointLights() []pointLight {
	st := &r.env.shadow
	out := st.lights[:0]
	g := r.syncLight()
	v := r.env.view
	for k := range g.srcs {
		s := &g.srcs[k]
		var L pointLight
		switch s.Kind {
		case lightCampfire:
			L = pointLight{h: campfireLightH, reach: campfireLightReach, fl: fireFlicker(float64(r.present) + float64(s.ID)*17)}
		case lightLamp:
			L = pointLight{h: lampLightH, reach: s.Radius, lamp: 1 + k}
		default:
			continue
		}
		L.id = s.ID
		L.x, L.y = float64(s.X)+0.5, float64(s.Y)+0.5
		mg := L.reach + shadowMaxLen + 1
		if !v.empty() && (L.x < v.x0-mg || L.y < v.y0-mg || L.x > v.x1+mg || L.y > v.y1+mg) {
			continue
		}
		out = append(out, L)
	}
	st.lights = out
	return out
}

// creatureLights writes into st.vis the lights in ls that reach creature
// c's tile by the light grid (a lamp stops at walls and rooms, so no lamp
// shadow falls through a wall; campfires keep their reach), and returns
// them.
func (r *Renderer) creatureLights(c creatureCaster, ls []pointLight) []pointLight {
	st := &r.env.shadow
	g := &r.lit.grid
	vis := st.vis[:0]
	for _, L := range ls {
		if L.lamp == 0 || g.lampReaches(L.lamp-1, c.tx, c.ty) {
			vis = append(vis, L)
		}
	}
	st.vis = vis
	return vis
}
