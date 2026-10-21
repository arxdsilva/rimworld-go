package ebiten

import (
	"image"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
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

type shadowKind uint8

const (
	shadowNone shadowKind = iota
	shadowBlob
	shadowBox
)

// shadowSpec is a caster's shape in tiles: footprint width w, height above
// ground h and depth d.
type shadowSpec struct {
	kind    shadowKind
	w, h, d float64
}

// shadowCaster is one caster this frame: for a blob, (x, y) is its ground
// pivot; for a box, the top-left of its ground footprint (w x d).
type shadowCaster struct {
	spec shadowSpec
	x, y float64
}

const (
	shadowMaxLen  = 3.0  // tiles: the longest shadow (ART §6.1 clamp)
	shadowFeather = 0.09 // tiles: the soft edge of box shadows
	blobMinDepth  = 0.35
)

// shadowColor is the shadow's dark cool tint (ART §6.1).
var shadowColor = [3]float32{18, 20, 30}

// shadowBlend is the layer's blend: the max of source and destination, so
// overlapping casters never darken twice.
var shadowBlend = ebiten.Blend{
	BlendFactorSourceRGB:        ebiten.BlendFactorOne,
	BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
	BlendFactorDestinationRGB:   ebiten.BlendFactorOne,
	BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
	BlendOperationRGB:           ebiten.BlendOperationMax,
	BlendOperationAlpha:         ebiten.BlendOperationMax,
}

// shadowSpecFor is the caster for sprite key: its metadata shadow block,
// or, until the art carries one, a fallback by family (m may be nil).
func shadowSpecFor(key string, m *artMeta) shadowSpec {
	fb := shadowFallback(key, m)
	if m == nil || m.Shadow.Kind == "" {
		return fb
	}
	s := shadowSpec{w: m.Shadow.W, h: m.Shadow.H, d: m.Shadow.D}
	switch m.Shadow.Kind {
	case "blob":
		s.kind = shadowBlob
	case "box":
		s.kind = shadowBox
	default:
		return shadowSpec{}
	}
	if s.w <= 0 {
		s.w = fb.w
	}
	if s.h <= 0 {
		s.h = fb.h
	}
	if s.d <= 0 {
		s.d = fb.d
	}
	if s.w <= 0 || s.h <= 0 { // nothing to fall back on
		s.w, s.h, s.d = max(s.w, 0.55), max(s.h, 1), max(s.d, blobMinDepth)
	}
	return s
}

// shadowFallback is the family default for sprites without a shadow block.
func shadowFallback(key string, m *artMeta) shadowSpec {
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
			canvasH = float64(m.Canvas[1]) / artTilePx
		}
	}
	switch fam {
	case "structure", "furniture", "rock":
		if strings.HasPrefix(name, "floor") {
			return shadowSpec{}
		}
		return shadowSpec{kind: shadowBox, w: fw, h: 1, d: fd}
	case "plant":
		return shadowSpec{kind: shadowBlob, w: 0.7 * fw, h: canvasH * 0.8, d: 0.4}
	case "pawn":
		return shadowSpec{kind: shadowBlob, w: 0.55, h: 1.7, d: 0.35}
	case "animal", "item":
		return shadowSpec{kind: shadowBlob, w: 0.55, h: canvasH * 0.7, d: 0.35}
	}
	return shadowSpec{}
}

// shadowVec is where the top of a caster of height h lands, relative to
// its base, in tiles.
func shadowVec(l lightState, h float64) (float64, float64) {
	if l.ShadowLen == nil {
		return 0, 0
	}
	n := l.ShadowLen(h)
	return l.ShadowDir[0] * n, l.ShadowDir[1] * n
}

// blobQuad is the blob's textured quad in world tiles (corners in texture
// order: top-left, top-right, bottom-left, bottom-right): w*1.1 across
// the shadow and max(d, 0.35)*0.8 along it, stretched along the shadow by
// its length and centred half that length away from the pivot. Texture x
// runs along the shadow.
func blobQuad(px, py float64, s shadowSpec, l lightState) [4][2]float64 {
	vx, vy := shadowVec(l, s.h)
	n := math.Hypot(vx, vy)
	ux, uy := 1.0, 0.0
	if n > 1e-9 {
		ux, uy = vx/n, vy/n
	}
	// the caster's silhouette is w wide whichever way the light comes
	// from; its depth runs along the shadow, which the length stretches
	along := math.Max(s.d, blobMinDepth)*0.8 + n
	across := s.w * 1.1
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

// boxHull is the box's shadow in world tiles: the footprint (x, y, w, d)
// swept along the shadow, i.e. the convex hull of the footprint and its
// copy shifted by the shadow vector, counter-clockwise in (x, y). It
// returns n of the up to 6 points used.
func boxHull(x, y, w, d float64, s shadowSpec, l lightState) (pts [8][2]float64, n int) {
	vx, vy := shadowVec(l, s.h)
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

// boxFootprint places a box caster's footprint (spec w x d) centred in
// the sprite's footprint (fw x fd) at tile (x, y).
func boxFootprint(x, y, fw, fd float64, s shadowSpec) (float64, float64) {
	return x + (fw-s.w)/2, y + (fd-s.d)/2
}

// shadowCompositeScale is the colour scale the layer is composited with:
// the shadow colour at the light's shadow alpha (premultiplied).
func shadowCompositeScale(l lightState) colorScale {
	a := float32(clamp01(l.ShadowAlpha))
	return colorScale{shadowColor[0] / 255 * a, shadowColor[1] / 255 * a, shadowColor[2] / 255 * a, a}
}

// --- casters ---------------------------------------------------------------------

// shadowState is the shadow pass's reusable state (kept on envState).
type shadowState struct {
	layer      *ebiten.Image // screen-sized, reallocated on resize
	blobImg    *ebiten.Image // procedural fallback for overlay/shadow_blob
	boxes      triBatch
	blobs      triBatch
	clear      triBatch
	casters    []shadowCaster
	specs      map[string]shadowSpec
	specsArt   *artCatalog
	composites int // layer composites, for tests
}

// spec is the cached caster for sprite key.
func (r *Renderer) shadowSpecOf(key string) shadowSpec {
	st := &r.env.shadow
	if st.specs == nil || st.specsArt != r.art {
		st.specs, st.specsArt = map[string]shadowSpec{}, r.art
	}
	if s, ok := st.specs[key]; ok {
		return s
	}
	var m *artMeta
	if sp := r.art.sprite(key); sp != nil {
		m = &sp.meta
	}
	s := shadowSpecFor(key, m)
	st.specs[key] = s
	return s
}

// footprintOf is a sprite's footprint in tiles (1x1 when unknown).
func (r *Renderer) footprintOf(key string) (float64, float64) {
	if sp := r.art.sprite(key); sp != nil && sp.meta.Footprint[0] > 0 && sp.meta.Footprint[1] > 0 {
		return float64(sp.meta.Footprint[0]), float64(sp.meta.Footprint[1])
	}
	return 1, 1
}

// skyOpen reports tile (x, y) of w is under open sky (no built or natural
// roof); without roof data every tile is.
func skyOpen(w game.WorldView, x, y int) bool {
	return !roofed(roofCellAt(w, x, y))
}

// shadowCasters lists this frame's casters standing on open-sky tiles
// within the view (widened by the longest shadow).
func (r *Renderer) shadowCasters(w game.WorldView, pawns []game.PawnView) []shadowCaster {
	st := &r.env.shadow
	out := st.casters[:0]
	v := r.env.view
	m := shadowMaxLen + 1
	inView := func(x, y, fw, fd float64) bool {
		return v.empty() || x+fw >= v.x0-m && y+fd >= v.y0-m && x <= v.x1+m && y <= v.y1+m
	}
	addBox := func(key string, x, y, fw, fd float64, scale float64) {
		s := r.shadowSpecOf(key)
		if s.kind == shadowNone || !inView(x, y, fw, fd) || !skyOpen(w, int(math.Floor(x)), int(math.Floor(y))) {
			return
		}
		s.w, s.h, s.d = s.w*scale, s.h*scale, s.d*scale
		if s.kind == shadowBlob {
			out = append(out, shadowCaster{spec: s, x: x + fw/2, y: y + fd/2})
			return
		}
		bx, by := boxFootprint(x, y, fw, fd, s)
		out = append(out, shadowCaster{spec: s, x: bx, y: by})
	}
	for _, b := range w.Buildings {
		if b.Floor || b.Kind == "sleeping spot" {
			continue
		}
		key := buildSpriteStuff(b.Kind, b.Stuff, b.Facing, 0, false, false)
		addBox(key, float64(b.X), float64(b.Y), float64(max(b.W, 1)), float64(max(b.H, 1)), 1)
	}
	r.addRockCasters(w, addBox)
	for _, p := range w.Plants {
		key, scale := plantSprite(p)
		key = r.seasonKey(key, w)
		fw, fd := r.footprintOf(key)
		addBox(key, float64(p.X), float64(p.Y), fw, fd, scale)
	}
	for _, it := range w.Items {
		if it.Carried {
			continue
		}
		key := itemSpriteAt(it.Kind, it.X, it.Y)
		addBox(key, float64(it.X), float64(it.Y), 1, 1, 1)
	}
	for _, an := range w.Animals {
		wx, wy := r.interpAnimal(an)
		key := "animal/" + an.Species + "_" + r.last[an.ID].facing.String()
		addBox(key, wx, wy, 1, 1, 1)
	}
	for _, p := range pawns {
		if p.Embodied {
			continue
		}
		wx, wy := r.interpWorld(p)
		addBox("pawn/body_south", wx, wy, 1, 1, 1)
	}
	st.casters = out
	return out
}

// addRockCasters adds the rock tiles at an outcrop's edge (inner rock is
// covered by its neighbours' sprites and casts nothing visible).
func (r *Renderer) addRockCasters(w game.WorldView, add func(key string, x, y, fw, fd, scale float64)) {
	if w.TerrainW <= 0 || len(w.Terrain) == 0 {
		return
	}
	rock := -1
	for i, n := range game.TerrainNames() {
		if n == "rock" {
			rock = i
		}
	}
	if rock < 0 {
		return
	}
	gw, gh := r.game.Width(), r.game.Height()
	isRock := func(x, y int) bool {
		if x < 0 || y < 0 || x >= gw || y >= gh {
			return true
		}
		i := y*w.TerrainW + x
		return i < len(w.Terrain) && int(w.Terrain[i]) == rock
	}
	x0, y0, x1, y1 := 0, 0, gw-1, gh-1
	if v := r.env.view; !v.empty() {
		m := shadowMaxLen + 1
		x0, y0 = max(int(v.x0-m), 0), max(int(v.y0-m), 0)
		x1, y1 = min(int(v.x1+m), gw-1), min(int(v.y1+m), gh-1)
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !isRock(x, y) || isRock(x-1, y) && isRock(x+1, y) && isRock(x, y-1) && isRock(x, y+1) {
				continue
			}
			add("rock/rock_wall_00", float64(x), float64(y), 1, 1, 1)
		}
	}
}

// roofRuns are the horizontal runs of roofed tiles (y, x0, x1 exclusive)
// in the tile range, where shadows are cleared.
func roofRuns(w game.WorldView, x0, y0, x1, y1 int, out [][3]int) [][3]int {
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

// --- batching ----------------------------------------------------------------------

// triBatch is a reusable triangle batch (uint32 indices).
type triBatch struct {
	verts []ebiten.Vertex
	idx   []uint32
}

func (b *triBatch) reset() { b.verts, b.idx = b.verts[:0], b.idx[:0] }

// vert adds a vertex at screen (x, y), texel (sx, sy), straight-alpha
// colour c at alpha a; it returns its index.
func (b *triBatch) vert(x, y, sx, sy float64, c color.RGBA, a float64) uint32 {
	b.verts = append(b.verts, ebiten.Vertex{DstX: float32(x), DstY: float32(y), SrcX: float32(sx), SrcY: float32(sy),
		ColorR: float32(c.R) / 255, ColorG: float32(c.G) / 255, ColorB: float32(c.B) / 255, ColorA: float32(clamp01(a))})
	return uint32(len(b.verts) - 1)
}

func (b *triBatch) tri(i, j, k uint32) { b.idx = append(b.idx, i, j, k) }

// rect is a solid axis-aligned rectangle (screen px) on the white texel,
// with per-corner alphas (top-left, top-right, bottom-left, bottom-right).
func (b *triBatch) rect(x, y, w, h float64, c color.RGBA, a00, a10, a01, a11 float64) {
	if w <= 0 || h <= 0 || a00+a10+a01+a11 <= 0 {
		return
	}
	i := b.vert(x, y, 1.5, 1.5, c, a00)
	b.vert(x+w, y, 1.5, 1.5, c, a10)
	b.vert(x, y+h, 1.5, 1.5, c, a01)
	b.vert(x+w, y+h, 1.5, 1.5, c, a11)
	b.tri(i, i+1, i+2)
	b.tri(i+1, i+3, i+2)
}

// line is a solid stroke from (x0, y0) to (x1, y1), width px wide.
func (b *triBatch) line(x0, y0, x1, y1, width float64, c color.RGBA, a float64) {
	dx, dy := x1-x0, y1-y0
	n := math.Hypot(dx, dy)
	if n == 0 || a <= 0 {
		return
	}
	px, py := -dy/n*width/2, dx/n*width/2
	i := b.vert(x0-px, y0-py, 1.5, 1.5, c, a)
	b.vert(x0+px, y0+py, 1.5, 1.5, c, a)
	b.vert(x1-px, y1-py, 1.5, 1.5, c, a)
	b.vert(x1+px, y1+py, 1.5, 1.5, c, a)
	b.tri(i, i+1, i+2)
	b.tri(i+1, i+3, i+2)
}

func (b *triBatch) flush(dst, src *ebiten.Image, blend ebiten.Blend) {
	if len(b.idx) > 0 {
		dst.DrawTriangles32(b.verts, b.idx, src, &ebiten.DrawTrianglesOptions{Blend: blend, Filter: ebiten.FilterLinear})
	}
	b.reset()
}

// --- drawing -------------------------------------------------------------------------

// blobTexture is overlay/shadow_blob, or a soft procedural ellipse until
// the art has one.
func (r *Renderer) blobTexture() *ebiten.Image {
	if r.art.has("overlay/shadow_blob") {
		if img := r.art.image("overlay/shadow_blob", color.NRGBA{}); img != nil {
			return img
		}
	}
	st := &r.env.shadow
	if st.blobImg == nil {
		const bw, bh = 64, 32
		src := image.NewNRGBA(image.Rect(0, 0, bw, bh))
		for y := 0; y < bh; y++ {
			for x := 0; x < bw; x++ {
				dx := (float64(x) + 0.5 - bw/2) / (bw / 2)
				dy := (float64(y) + 0.5 - bh/2) / (bh / 2)
				a := math.Exp(-3.2*(dx*dx+dy*dy)) - math.Exp(-3.2)
				src.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * clamp01(a/(1-math.Exp(-3.2))))})
			}
		}
		st.blobImg = ebiten.NewImageFromImage(src)
	}
	return st.blobImg
}

// addBlob queues a blob caster into the blob batch.
func (r *Renderer) addBlob(b *triBatch, c shadowCaster, l lightState, tex *ebiten.Image, tile float64) {
	q := blobQuad(c.x, c.y, c.spec, l)
	tw, th := float64(tex.Bounds().Dx()), float64(tex.Bounds().Dy())
	ox, oy := float64(tex.Bounds().Min.X), float64(tex.Bounds().Min.Y)
	white := color.RGBA{255, 255, 255, 255}
	uv := [4][2]float64{{ox, oy}, {ox + tw, oy}, {ox, oy + th}, {ox + tw, oy + th}}
	var i uint32
	for k, p := range q {
		sx, sy := r.worldToScreenF(p[0], p[1])
		j := b.vert(float64(sx), float64(sy), uv[k][0], uv[k][1], white, 1)
		if k == 0 {
			i = j
		}
	}
	b.tri(i, i+1, i+2)
	b.tri(i+1, i+3, i+2)
}

// addBox queues a box caster's swept footprint with a feathered skirt.
func (r *Renderer) addBox(b *triBatch, c shadowCaster, l lightState, tile float64) {
	pts, n := boxHull(c.x, c.y, c.spec.w, c.spec.d, c.spec, l)
	if n < 3 {
		return
	}
	white := color.RGBA{255, 255, 255, 255}
	f := shadowFeather
	base := uint32(len(b.verts))
	for i := 0; i < n; i++ {
		sx, sy := r.worldToScreenF(pts[i][0], pts[i][1])
		b.vert(float64(sx), float64(sy), 1.5, 1.5, white, 1)
	}
	for i := 1; i+1 < n; i++ {
		b.tri(base, base+uint32(i), base+uint32(i+1))
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
		a, bb := r.worldToScreenF(p[0]+nx, p[1]+ny)
		c2, d2 := r.worldToScreenF(q[0]+nx, q[1]+ny)
		b.vert(float64(a), float64(bb), 1.5, 1.5, white, 0)
		b.vert(float64(c2), float64(d2), 1.5, 1.5, white, 0)
	}
	for i := 0; i < n; i++ {
		pi, qi := base+uint32(i), base+uint32((i+1)%n)
		po, qo := outer+uint32(2*i), outer+uint32(2*i+1)
		b.tri(pi, qi, po)
		b.tri(qi, qo, po)
		prevOuter := outer + uint32(2*((i+n-1)%n)+1) // corner wedge at pi
		b.tri(pi, prevOuter, po)
	}
}

// drawShadows draws the frame's shadow layer onto screen.
func (r *Renderer) drawShadows(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	l := r.lit.light
	if l.ShadowAlpha <= 0 || l.ShadowLen == nil || sw <= 0 || sh <= 0 {
		return
	}
	r.setView(sw, sh, tile)
	st := &r.env.shadow
	casters := r.shadowCasters(w, r.drawPawns)
	if len(casters) == 0 {
		return
	}
	if st.layer == nil || st.layer.Bounds().Dx() != sw || st.layer.Bounds().Dy() != sh {
		if st.layer != nil {
			st.layer.Deallocate()
		}
		st.layer = ebiten.NewImage(sw, sh)
	} else {
		st.layer.Clear()
	}
	tex := r.blobTexture()
	st.boxes.reset()
	st.blobs.reset()
	for _, c := range casters {
		if c.spec.kind == shadowBox {
			r.addBox(&st.boxes, c, l, tile)
		} else {
			r.addBlob(&st.blobs, c, l, tex, tile)
		}
	}
	white := r.env.whiteImg()
	st.boxes.flush(st.layer, white, shadowBlend)
	st.blobs.flush(st.layer, tex, shadowBlend)
	// nothing falls on roofed ground
	x0, y0, x1, y1 := r.visibleTiles()
	var runs [][3]int
	runs = roofRuns(w, x0, y0, x1, y1, runs)
	if len(runs) > 0 {
		st.clear.reset()
		for _, run := range runs {
			sx0, sy0 := r.worldToScreen(run[1], run[0])
			sx1, sy1 := r.worldToScreen(run[2], run[0]+1)
			st.clear.rect(float64(sx0), float64(sy0), float64(sx1-sx0), float64(sy1-sy0), color.RGBA{255, 255, 255, 255}, 1, 1, 1, 1)
		}
		st.clear.flush(st.layer, white, ebiten.BlendDestinationOut)
	}
	cs := shadowCompositeScale(l)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.Scale(cs[0], cs[1], cs[2], cs[3])
	screen.DrawImage(st.layer, op)
	st.composites++
}
