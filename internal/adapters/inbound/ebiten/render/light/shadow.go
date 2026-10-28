package light

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/roof"
)

// Moving shadows (ART §6.1): every caster (metadata shadow.kind blob or
// box) on open-sky ground casts a soft shadow away from the sun (or the
// moon), long at dawn and dusk, short at noon. Shadows are drawn into one
// offscreen layer and composited at a single alpha, so overlaps never
// double-darken, after floors and zones and before every object.
//
// The geometry is in world tiles and pure (tested without a GPU): a blob
// is the shadow_blob texture on a quad stretched along the shadow; a box
// is the exact sweep of its footprint along the shadow (the convex hull of
// the footprint and its shifted copy) with a feathered skirt, so adjacent
// walls merge seamlessly under the max blend.

// ShadowKind is a caster's shape: none, a blob or a box.
type ShadowKind uint8

const (
	ShadowNone ShadowKind = iota
	ShadowBlob
	ShadowBox
)

// ShadowSpec is a caster's shape in tiles: footprint width w, height above
// ground h and depth d.
type ShadowSpec struct {
	Kind    ShadowKind
	W, H, D float64
}

// Caster is one caster this frame: for a blob, (x, y) is its ground
// pivot; for a box, the top-left of its ground footprint (w x d).
type Caster struct {
	Spec ShadowSpec
	X, Y float64
}

const (
	ShadowMaxLen  = 3.0  // tiles: the longest shadow (ART §6.1 clamp)
	shadowFeather = 0.09 // tiles: the soft edge of box shadows
	blobMinDepth  = 0.35
)

// ShadowColor is the shadow's dark cool tint at noon (ART §6.1); it
// tilts bluer at low sun (ShadowTint).
var ShadowColor = [3]float32{18, 20, 30}

// shadowRGB is the layer colour of shapes with no height (boxes, blobs
// and contact pools): R holds height for the penumbra blur, so 0.
var shadowRGB = color.RGBA{0, 255, 255, 255}

// ShadowBlend is the layer's blend: the max of source and destination, so
// overlapping casters never darken twice.
var ShadowBlend = ebiten.Blend{
	BlendFactorSourceRGB:        ebiten.BlendFactorOne,
	BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
	BlendFactorDestinationRGB:   ebiten.BlendFactorOne,
	BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
	BlendOperationRGB:           ebiten.BlendOperationMax,
	BlendOperationAlpha:         ebiten.BlendOperationMax,
}

// ShadowSpecFor is the caster for sprite key: its metadata shadow block,
// or, until the art carries one, a fallback by family (m may be nil).
func ShadowSpecFor(key string, m *art.Meta) ShadowSpec {
	fb := ShadowFallback(key, m)
	if m == nil || m.Shadow.Kind == "" {
		return fb
	}
	s := ShadowSpec{W: m.Shadow.W, H: m.Shadow.H, D: m.Shadow.D}
	switch m.Shadow.Kind {
	case "blob":
		s.Kind = ShadowBlob
	case "box":
		s.Kind = ShadowBox
	default:
		return ShadowSpec{}
	}
	if s.W <= 0 {
		s.W = fb.W
	}
	if s.H <= 0 {
		s.H = fb.H
	}
	if s.D <= 0 {
		s.D = fb.D
	}
	if s.W <= 0 || s.H <= 0 { // nothing to fall back on
		s.W, s.H, s.D = max(s.W, 0.55), max(s.H, 1), max(s.D, blobMinDepth)
	}
	return s
}

// ShadowFallback is the family default for sprites without a shadow block.
func ShadowFallback(key string, m *art.Meta) ShadowSpec {
	fam, name, _ := strings.Cut(key, "/")
	fw, fd, canvasH := 1.0, 1.0, 1.0
	if m != nil {
		if m.Footprint[0] > 0 {
			fw = float64(m.Footprint[0])
		}
		if m.Footprint[1] > 0 {
			fd = float64(m.Footprint[1])
		}
		if m.Canvas[1] > 0 {
			canvasH = float64(m.Canvas[1]) / m.PxPerTile()
		}
	}
	switch fam {
	case "structure", "furniture", "rock":
		if strings.HasPrefix(name, "floor") {
			return ShadowSpec{}
		}
		return ShadowSpec{Kind: ShadowBox, W: fw, H: 1, D: fd}
	case "plant":
		return ShadowSpec{Kind: ShadowBlob, W: 0.7 * fw, H: canvasH * 0.8, D: 0.4}
	case "pawn":
		return ShadowSpec{Kind: ShadowBlob, W: 0.55, H: 1.7, D: 0.35}
	case "animal", "item":
		return ShadowSpec{Kind: ShadowBlob, W: 0.55, H: canvasH * 0.7, D: 0.35}
	}
	return ShadowSpec{}
}

// shadowVec is where the top of a caster of height h lands, relative to
// its base, in tiles.
func shadowVec(l State, h float64) (float64, float64) {
	if l.ShadowLen == nil {
		return 0, 0
	}
	n := l.ShadowLen(h)
	return l.ShadowDir[0] * n, l.ShadowDir[1] * n
}

// BlobQuad is the blob's textured quad in world tiles (corners in texture
// order: top-left, top-right, bottom-left, bottom-right): w*1.1 across
// the shadow and max(d, 0.35)*0.8 along it, stretched along the shadow by
// its length and centred half that length away from the pivot. Texture x
// runs along the shadow.
func BlobQuad(px, py float64, s ShadowSpec, l State) [4][2]float64 {
	vx, vy := shadowVec(l, s.H)
	n := math.Hypot(vx, vy)
	ux, uy := 1.0, 0.0
	if n > 1e-9 {
		ux, uy = vx/n, vy/n
	}
	// the caster's silhouette is w wide whichever way the light comes
	// from; its depth runs along the shadow, which the length stretches
	along := math.Max(s.D, blobMinDepth)*0.8 + n
	across := s.W * 1.1
	cx, cy := px+vx/2, py+vy/2
	hx, hy := ux*along/2, uy*along/2    // half along
	qx, qy := -uy*across/2, ux*across/2 // half across
	return [4][2]float64{
		{cx - hx - qx, cy - hy - qy},
		{cx + hx - qx, cy + hy - qy},
		{cx - hx + qx, cy - hy + qy},
		{cx + hx + qx, cy + hy + qy},
	}
}

// BoxHull is the box's shadow in world tiles: the footprint (x, y, w, d)
// swept along the shadow, i.e. the convex hull of the footprint and its
// copy shifted by the shadow vector, counter-clockwise in (x, y). It
// returns n of the up to 6 points used.
func BoxHull(x, y, w, d float64, s ShadowSpec, l State) (pts [8][2]float64, n int) {
	vx, vy := shadowVec(l, s.H)
	var in [8][2]float64
	c := [4][2]float64{{x, y}, {x + w, y}, {x + w, y + d}, {x, y + d}}
	for i, p := range c {
		in[i] = p
		in[i+4] = [2]float64{p[0] + vx, p[1] + vy}
	}
	for i := 1; i < len(in); i++ { // insertion sort by x, then y
		for j := i; j > 0 && (in[j][0] < in[j-1][0] || in[j][0] == in[j-1][0] && in[j][1] < in[j-1][1]); j-- {
			in[j], in[j-1] = in[j-1], in[j]
		}
	}
	cross := func(o, a, b [2]float64) float64 {
		return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0])
	}
	var h [16][2]float64
	k := 0
	for _, p := range in { // lower hull
		for k >= 2 && cross(h[k-2], h[k-1], p) <= 1e-12 {
			k--
		}
		h[k] = p
		k++
	}
	lo := k + 1
	for i := len(in) - 2; i >= 0; i-- { // upper hull
		p := in[i]
		for k >= lo && cross(h[k-2], h[k-1], p) <= 1e-12 {
			k--
		}
		h[k] = p
		k++
	}
	n = min(k-1, len(pts))
	copy(pts[:n], h[:n])
	return pts, n
}

// BoxFootprint places a box caster's footprint (spec w x d) centred in
// the sprite's footprint (fw x fd) at tile (x, y).
func BoxFootprint(x, y, fw, fd float64, s ShadowSpec) (float64, float64) {
	return x + (fw-s.W)/2, y + (fd-s.D)/2
}

// ShadowCompositeScale is the colour scale the layer is composited with:
// the shadow colour at the light's shadow alpha (premultiplied).
func ShadowCompositeScale(l State) [4]float32 {
	a := float32(gfx.Clamp01(l.ShadowAlpha))
	return [4]float32{ShadowColor[0] / 255 * a, ShadowColor[1] / 255 * a, ShadowColor[2] / 255 * a, a}
}

// --- the pass ----------------------------------------------------------------------

// Rect is the visible world rectangle in tiles (the renderer's setView).
type Rect struct{ X0, Y0, X1, Y1 float64 }

// Empty reports no view (nothing set yet): nothing is culled.
func (v Rect) Empty() bool { return v.X1 <= v.X0 || v.Y1 <= v.Y0 }

// Casters is what the shadow pass needs from the renderer each frame: the
// casters in view (their sprite keys, chunk lists and creature motion are
// the renderer's), a figure drawn as a silhouette, the on-map tiles in
// view and a white texel. F is the renderer's figure pose per creature
// (Creature.Fig), resolved once per frame when it lists the creatures.
type Casters[F any] interface {
	// Objects lists this frame's box and blob casters (buildings, rocks,
	// plants and items) on open-sky tiles in view.
	Objects(w game.WorldView, pawns []game.PawnView) []Caster
	// Creatures lists the pawns and animals in view that cast a silhouette.
	Creatures(w game.WorldView, pawns []game.PawnView) []Creature[F]
	// CastFigure draws c's figure onto dst with cast as the renderer's
	// cast state, so every draw of it lands in the layer as a silhouette.
	CastFigure(c *Creature[F], cast *Cast, dst *ebiten.Image, tile float64)
	// VisibleTiles is the on-map tile range in view.
	VisibleTiles() (x0, y0, x1, y1 int)
	// White is a white texel for solid fills.
	White() *ebiten.Image
}

// Shadows is the shadow pass's reusable state (kept on the renderer's
// environment). F is the renderer's figure pose (Casters).
type Shadows[F any] struct {
	Layer *ebiten.Image // screen-sized, reallocated on resize
	// Scratch is screen-sized too: the blur's target, and before that the
	// discard target of figure draws in cast mode (anything a figure
	// draws directly, such as a hit flash, lands there and is overwritten).
	Scratch  *ebiten.Image
	blobImg  *ebiten.Image // procedural fallback for overlay/shadow_blob
	boxes    gfx.TriBatch
	blobs    gfx.TriBatch
	clear    gfx.TriBatch
	Contacts gfx.TriBatch
	// scale is the layer's size relative to the screen this frame (0.5 on
	// a high-DPI display, else 1); the draws and the blur radii follow it.
	scale float64
	// Objects and Creatures are the renderer's reused caster lists
	// (Casters fills them).
	Objects    []Caster
	Creatures  []Creature[F]
	plans      []Plan
	lights     []PointLight
	vis        []PointLight // the lights reaching the creature being planned
	cast       Cast
	castInit   bool
	blur       *ebiten.Shader
	blurErr    error
	BlurOpts   ebiten.DrawRectShaderOptions
	specs      map[string]ShadowSpec
	specsArt   *art.Catalog
	Composites int // layer composites, for tests
	Casts      int // creature silhouettes drawn, for tests
	// Off skips the whole shadow pass (Options > Graphics, PLAY-019).
	Off bool
}

// SetOff turns the shadow pass off or on. Turning it off frees the two
// screen-sized layers; they come back on the first frame with shadows on.
func (st *Shadows[F]) SetOff(off bool) {
	st.Off = off
	if !off {
		return
	}
	for _, img := range []**ebiten.Image{&st.Layer, &st.Scratch} {
		if *img != nil {
			(*img).Deallocate()
			*img = nil
		}
	}
}

// SpecOf is the cached caster for sprite key in catalog c.
func (st *Shadows[F]) SpecOf(c *art.Catalog, key string) ShadowSpec {
	if st.specs == nil || st.specsArt != c {
		st.specs, st.specsArt = map[string]ShadowSpec{}, c
	}
	if s, ok := st.specs[key]; ok {
		return s
	}
	var m *art.Meta
	if sp := c.Sprite(key); sp != nil {
		m = &sp.Meta
	}
	s := ShadowSpecFor(key, m)
	st.specs[key] = s
	return s
}

// roofCellAt is the roof flags at (x, y) of w; 0 out of bounds.
func roofCellAt(w game.WorldView, x, y int) uint8 {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return 0
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoofCells) {
		return 0
	}
	return w.RoofCells[i]
}

// roofed reports a built or natural roof.
func roofed(c uint8) bool { return c&uint8(roof.Built|roof.Natural) != 0 }

// RoofRuns are the horizontal runs of roofed tiles (y, x0, x1 exclusive)
// in the tile range, where shadows are cleared.
func RoofRuns(w game.WorldView, x0, y0, x1, y1 int, out [][3]int) [][3]int {
	out = out[:0]
	if len(w.RoofCells) == 0 {
		return out
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; {
			if !roofed(roofCellAt(w, x, y)) {
				x++
				continue
			}
			end := x + 1
			for end <= x1 && roofed(roofCellAt(w, end, y)) {
				end++
			}
			out = append(out, [3]int{y, x, end})
			x = end
		}
	}
	return out
}

// --- drawing -------------------------------------------------------------------------

// blobTexture is overlay/shadow_blob, or a soft procedural ellipse until
// the art has one.
func (st *Shadows[F]) blobTexture(c *art.Catalog) *ebiten.Image {
	if c.Has("overlay/shadow_blob") {
		if img := c.Image("overlay/shadow_blob", color.NRGBA{}); img != nil {
			return img
		}
	}
	if st.blobImg == nil {
		const bw, bh = 64, 32
		src := image.NewNRGBA(image.Rect(0, 0, bw, bh))
		for y := 0; y < bh; y++ {
			for x := 0; x < bw; x++ {
				dx := (float64(x) + 0.5 - bw/2) / (bw / 2)
				dy := (float64(y) + 0.5 - bh/2) / (bh / 2)
				a := math.Exp(-3.2*(dx*dx+dy*dy)) - math.Exp(-3.2)
				src.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * gfx.Clamp01(a/(1-math.Exp(-3.2))))})
			}
		}
		st.blobImg = ebiten.NewImageFromImage(src)
	}
	return st.blobImg
}

// addTexQuad queues the whole of tex on a quad (world tiles, corners in
// texture order) at alpha a, with no height, placed by v's camera.
func addTexQuad(v *render.View, b *gfx.TriBatch, q [4][2]float64, tex *ebiten.Image, a float64) {
	tw, th := float64(tex.Bounds().Dx()), float64(tex.Bounds().Dy())
	ox, oy := float64(tex.Bounds().Min.X), float64(tex.Bounds().Min.Y)
	uv := [4][2]float64{{ox, oy}, {ox + tw, oy}, {ox, oy + th}, {ox + tw, oy + th}}
	var i uint32
	for k, p := range q {
		sx, sy := v.WorldToScreenF(p[0], p[1])
		j := b.Vert(float64(sx), float64(sy), uv[k][0], uv[k][1], shadowRGB, a)
		if k == 0 {
			i = j
		}
	}
	b.Tri(i, i+1, i+2)
	b.Tri(i+1, i+3, i+2)
}

// AddBox queues a box caster's swept footprint with a feathered skirt,
// its core at alpha a, placed by v's camera.
func AddBox(v *render.View, b *gfx.TriBatch, c Caster, l State, a float64) {
	pts, n := BoxHull(c.X, c.Y, c.Spec.W, c.Spec.D, c.Spec, l)
	if n < 3 {
		return
	}
	white := shadowRGB
	f := shadowFeather
	base := uint32(len(b.Verts))
	for i := 0; i < n; i++ {
		sx, sy := v.WorldToScreenF(pts[i][0], pts[i][1])
		b.Vert(float64(sx), float64(sy), 1.5, 1.5, white, a)
	}
	for i := 1; i+1 < n; i++ {
		b.Tri(base, base+uint32(i), base+uint32(i+1))
	}
	outer := base + uint32(n) // two outer vertices per edge
	for i := 0; i < n; i++ {
		p, q := pts[i], pts[(i+1)%n]
		dx, dy := q[0]-p[0], q[1]-p[1]
		ln := math.Hypot(dx, dy)
		if ln == 0 {
			ln = 1
		}
		nx, ny := dy/ln*f, -dx/ln*f // outward for a counter-clockwise hull
		a, bb := v.WorldToScreenF(p[0]+nx, p[1]+ny)
		c2, d2 := v.WorldToScreenF(q[0]+nx, q[1]+ny)
		b.Vert(float64(a), float64(bb), 1.5, 1.5, white, 0)
		b.Vert(float64(c2), float64(d2), 1.5, 1.5, white, 0)
	}
	for i := 0; i < n; i++ {
		pi, qi := base+uint32(i), base+uint32((i+1)%n)
		po, qo := outer+uint32(2*i), outer+uint32(2*i+1)
		b.Tri(pi, qi, po)
		b.Tri(qi, qo, po)
		prevOuter := outer + uint32(2*((i+n-1)%n)+1) // corner wedge at pi
		b.Tri(pi, prevOuter, po)
	}
}

// Draw draws the frame's shadow layer onto screen. The layer holds
// absolute alpha (max-blended, so overlaps never double-darken) and the
// caster height in red:
//
//	(a) box and blob casters at the sun or moon's shadow alpha
//	(b) creature silhouettes for the sun or moon (open sky only)
//	    -- roofed tiles cleared --
//	(c) creature silhouettes for campfires and lamps (kept indoors; the
//	    sun washes them out on open sky only)
//	(d) contact pools under every creature
//
// then one contact-hardening blur into scratch and one composite. w is
// the frame's world (v.W) and view the visible rectangle in tiles; the
// light is lit's (its grid synced to v gives the point lights), and host
// lists the casters and draws the figures.
func (st *Shadows[F]) Draw(v *render.View, lit *Lit, host Casters[F], view Rect, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if sw <= 0 || sh <= 0 || st.Off {
		return
	}
	l := lit.Light
	sa := 0.0
	if l.ShadowLen != nil {
		sa = float64(ShadowCompositeScale(l)[3])
	}
	var casters []Caster
	if sa > 0 {
		casters = host.Objects(w, v.Pawns)
	}
	creatures := host.Creatures(w, v.Pawns)
	if len(casters) == 0 && len(creatures) == 0 {
		return
	}
	k := ShadowLayerScale(v.Cam.Dev)
	st.scale = k
	lw, lh := int(math.Ceil(float64(sw)*k)), int(math.Ceil(float64(sh)*k))
	st.Layer = gfx.FitLayer(st.Layer, lw, lh)
	st.Layer.Clear()
	st.Scratch = gfx.FitLayer(st.Scratch, lw, lh)
	k32 := float32(k)
	st.boxes.K, st.blobs.K, st.clear.K, st.Contacts.K = k32, k32, k32, k32
	tex := st.blobTexture(v.Art)
	white := host.White()
	st.boxes.Reset()
	st.blobs.Reset()
	for _, c := range casters {
		if c.Spec.Kind == ShadowBox {
			AddBox(v, &st.boxes, c, l, sa)
		} else {
			addTexQuad(v, &st.blobs, BlobQuad(c.X, c.Y, c.Spec, l), tex, sa)
		}
	}
	st.boxes.Flush(st.Layer, white, ShadowBlend)
	st.blobs.Flush(st.Layer, tex, ShadowBlend)

	sunEl := SunElevation(l)
	daylight := DaylightOf(sunEl)
	lights := st.PointLights(v, lit, view)
	plans := st.plans[:0]
	for i := range creatures {
		c := &creatures[i]
		// the sun washes out point-light shadows on open sky only; under a
		// roof lamps cast by day too (PLAY-020 §3b)
		dl := 0.0
		if c.Sky {
			dl = daylight
		}
		var near []PointLight
		if dl < 1 { // full daylight on open sky: no point-light shadow to plan
			near = st.creatureLights(&lit.Grid, c.Tx, c.Ty, lights)
		}
		plans = append(plans, PlanCreature(c.Px, c.Py, c.H, c.Sky, l, dl, near))
	}
	st.plans = plans
	for i := range creatures {
		if plans[i].Sun {
			st.castCreature(v, host, &creatures[i], plans[i].SunCast, tile)
		}
	}
	// nothing from the sky falls on roofed ground
	x0, y0, x1, y1 := host.VisibleTiles()
	var runs [][3]int
	runs = RoofRuns(w, x0, y0, x1, y1, runs)
	if len(runs) > 0 {
		st.clear.Reset()
		for _, run := range runs {
			sx0, sy0 := v.WorldToScreen(run[1], run[0])
			sx1, sy1 := v.WorldToScreen(run[2], run[0]+1)
			st.clear.Rect(float64(sx0), float64(sy0), float64(sx1-sx0), float64(sy1-sy0), color.RGBA{255, 255, 255, 255}, 1, 1, 1, 1)
		}
		st.clear.Flush(st.Layer, white, ebiten.BlendDestinationOut)
	}
	st.Contacts.Reset()
	for i := range creatures {
		c := &creatures[i]
		for j := 0; j < plans[i].Np; j++ {
			st.castCreature(v, host, c, plans[i].Points[j], tile)
		}
		st.addContact(v, c, plans[i].Contact, tex)
	}
	st.Contacts.Flush(st.Layer, tex, ShadowBlend)

	maxEl := SeasonMaxEl(w.DayOfYear) * math.Pi / 180
	ratio := gfx.Clamp01(sunEl / maxEl)
	out := st.Layer
	if bs := st.shadowBlurShader(); bs != nil {
		r0, r1 := BlurRadii(tile*k/camera.TileSize, ratio)
		st.BlurOpts.Uniforms["R0"], st.BlurOpts.Uniforms["R1"] = float32(r0), float32(r1)
		st.BlurOpts.Images[0] = st.Layer
		st.BlurOpts.Blend = ebiten.BlendCopy
		st.Scratch.DrawRectShader(lw, lh, bs, &st.BlurOpts)
		out = st.Scratch
	}
	tint := ShadowTint(ratio)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.Scale(tint[0]/255, tint[1]/255, tint[2]/255, 1)
	if lw != sw || lh != sh {
		op.GeoM.Scale(float64(sw)/float64(lw), float64(sh)/float64(lh))
		op.Filter = ebiten.FilterLinear
	}
	screen.DrawImage(out, op)
	st.Composites++
}

// ShadowLayerScale is the shadow layer's size relative to the screen: half
// on a display whose device scale is 2 or more (the blur hides the lost
// detail), full otherwise.
func ShadowLayerScale(dev float64) float64 {
	if dev >= 2 {
		return 0.5
	}
	return 1
}

// shadowBlurShader is shadow_blur.kage, compiled on first use; nil when
// it failed (the layer is then composited unblurred).
func (st *Shadows[F]) shadowBlurShader() *ebiten.Shader {
	if st.blur == nil && st.blurErr == nil {
		st.blur, st.blurErr = ebiten.NewShader(shadowBlurKage)
		st.BlurOpts.Uniforms = map[string]any{}
	}
	return st.blur
}
