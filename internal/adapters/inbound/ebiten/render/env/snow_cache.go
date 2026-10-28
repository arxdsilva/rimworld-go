package env

import (
	"bytes"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// The snow veil and the wet veil of open ground (PLAY-015 §4, PERF-002,
// PLAY-022).
//
// Both are masks baked per 32-tile map chunk into small images and stamped
// through the lit shader (lod.go drawBakedColor / drawChunkFlat), so they
// dim at night, catch lamp light and go dark under roofs like the ground
// below them. A frame costs one or a few draws per visible chunk and no
// per-tile work. A chunk is rebaked when its own snow bytes, the wetness
// bucket, its crops or the ground it covers changed, or its wet image is
// at another detail than the zoom wants: at once when it was never baked,
// else within the pass's cost budget (PuddleBakeBudget), the rest showing
// their last bake a frame or two longer.
//
// Each image holds px texels a tile, framed by a one-texel pad sampled
// from the true neighbouring ground so bilinear taps across a chunk edge
// match the next chunk:
//   - snow (SnowImgW square, SnowPx a tile): premultiplied white
//     (m, m, m, m); a texel is the old per-tile veil quad's alpha at that
//     point: the four corner values (snowField over snowCorner)
//     interpolated across the tile, zero on tiles that are not open ground.
//     Drawing tints it (236, 240, 246) and scales it by the veil strength.
//   - wet (WetPxFor the zoom: 8 a tile live, 4 on chunk bakes, 2 on the
//     map bake; puddle_veil.go): alpha the darkening of open ground (the
//     wet darkening, deeper in the puddles), grey the reflection weight;
//     the draw tints the grey with the sky (PuddleSky). Crisp puddle shores
//     need the 8 texels up close; far out 2 keep the memory small.

const (
	SnowChunkTiles = 32 // tiles per chunk side
	SnowPx         = 4  // baked snow texels per tile
	SnowPad        = 1
	SnowImgW       = SnowChunkTiles*SnowPx + 2*SnowPad
	snowRing       = SnowChunkTiles + 2 // tiles per side with the 1-tile ring
)

// SnowChunk is one baked chunk.
type SnowChunk struct {
	img    *ebiten.Image // the snow mask
	wetImg *ebiten.Image // the wet veil, wetPx texels a tile (nil: none)
	WetPx  int           // wet texels per tile baked at
	cells  []uint8       // the chunk's snow bytes and a one-tile ring (empty: legacy depth)
	depth  uint8         // legacy uniform depth baked (no per-tile layer)
	gen    uint64        // SnowCache.gen baked at
	ver    uint64        // SnowVer cells were last verified at
	wet    uint8         // wetness bucket baked (WetBucket)
	Plants uint64        // crop key baked (puddleCropKey), while wet > 0
	// Pud is the puddle cover per wet texel (WetImgW(wetPx)²), nil when
	// the chunk holds no water: where raindrops ripple (PuddleAt).
	Pud    []uint8
	seen   uint32 // the last pass that walked it (SnowCache.pass)
	built  bool
	Any    bool // some snow
	anyWet bool // some wet veil
}

// SnowEvictPasses: a chunk no pass walked for this many passes (seconds
// of frames) is dropped with its images, so panning a big map at close
// zoom does not keep every chunk's wet veil; it is rebaked on return.
const SnowEvictPasses = 2400

// WetImgW is the side of a wet image at px texels a tile.
func WetImgW(px int) int { return SnowChunkTiles*px + 2*SnowPad }

// WetPxFor is the wet veil's texels a tile at this tile size.
func WetPxFor(tile float64) int {
	switch tierFor(tile) {
	case tierLive:
		return 8
	case tierChunk:
		return 4
	}
	return 2
}

// bakeCost is a chunk bake's share of the pass budget at px wet texels a
// tile: its texels over a 4-texel bake's.
func bakeCost(px int) int { return max(1, px*px/16) }

// SnowCache holds the baked chunks of the current map.
type SnowCache struct {
	Chunks map[int]*SnowChunk
	w, h   int
	// gen moves when what a chunk depends on besides its snow bytes moves:
	// terrain and buildings (StructVer: floors too), the room grid or the
	// roof layer (both shared read-only slices that change identity when
	// their content does).
	gen    uint64
	key    snowKey
	Pix    []byte    // the snow bake's texels
	Wpix   []byte    // the wet bake's texels
	height []float32 // the wet bake's texel heights (NaN: dry)
	ring   []byte
	corner []float64 // (snowRing+1)² corner veil alphas
	open   []bool    // snowRing² open tiles
	wb     WetBake   // the bake's wet inputs (puddle_veil.go)
	// WantPx is the wet detail this pass bakes at (0: SnowPx).
	WantPx int
	// pass counts chunk walks; cropVer memoises each read-model chunk's
	// PlantVer for the pass stamped in cropPass.
	pass     uint32
	cropVer  []uint64
	cropPass []uint32
	budget   int // bake cost this pass may still spend on rebakes
	Bakes    int // chunk bakes (tests)
	Draws    int // snow-pass draws of the last pass (tests)
	WetDraws int // wet-veil draws of the last pass (tests)
}

type snowKey struct {
	structVer uint64
	rooms     *int
	roofs     *uint8
}

// release drops every bake.
func (sc *SnowCache) release() {
	for _, c := range sc.Chunks {
		for _, img := range []*ebiten.Image{c.img, c.wetImg} {
			if img != nil {
				img.Deallocate()
			}
		}
	}
	sc.Chunks = nil
}

// evict drops the chunks no pass walked for SnowEvictPasses.
func (sc *SnowCache) evict() {
	for id, c := range sc.Chunks {
		if sc.pass-c.seen <= SnowEvictPasses {
			continue
		}
		for _, img := range []*ebiten.Image{c.img, c.wetImg} {
			if img != nil {
				img.Deallocate()
			}
		}
		delete(sc.Chunks, id)
	}
}

// gather copies chunk (cx, cy)'s snow bytes and a one-tile ring (zero off
// the map) from cells, mw tiles wide, into sc.ring.
func (sc *SnowCache) gather(cells []uint8, mw, mh, cx, cy int) []byte {
	if cap(sc.ring) < snowRing*snowRing {
		sc.ring = make([]byte, snowRing*snowRing)
	}
	ring := sc.ring[:snowRing*snowRing]
	for ry := 0; ry < snowRing; ry++ {
		y := cy*SnowChunkTiles + ry - 1
		for rx := 0; rx < snowRing; rx++ {
			x := cx*SnowChunkTiles + rx - 1
			var v uint8
			if x >= 0 && y >= 0 && x < mw && y < mh {
				if i := y*mw + x; i < len(cells) {
					v = cells[i]
				}
			}
			ring[ry*snowRing+rx] = v
		}
	}
	return ring
}

// stale reports chunk c needs a rebake: never built, the ground changed,
// another wetness bucket or wet detail, its crops moved (only while wet),
// or its own snow differs from what it baked. The bytes are compared only
// when SnowVer moved since the chunk last agreed with it.
func (sc *SnowCache) stale(c *SnowChunk, w game.WorldView, depth, wet uint8, mw, mh, cx, cy int, plants func() uint64) bool {
	if !c.built || c.gen != sc.gen || c.wet != wet || wet > 0 && c.WetPx != sc.WantPx {
		return true
	}
	if wet > 0 && c.Plants != plants() {
		return true
	}
	if len(w.Snow) == 0 {
		return len(c.cells) != 0 || c.depth != depth
	}
	if len(c.cells) == 0 {
		return true
	}
	if w.SnowVer != 0 && c.ver == w.SnowVer {
		return false
	}
	if bytes.Equal(sc.gather(w.Snow, w.TerrainW, mh, cx, cy), c.cells) {
		c.ver = w.SnowVer
		return false
	}
	return true
}

// BakeSnowChunk renders chunk (cx, cy) into c: its snow mask and its wet
// veil at the pass's detail (sc.WantPx); ch is the chunk index the wet
// bake reads floors and crops from.
func (e *Layer) BakeSnowChunk(c *SnowChunk, w game.WorldView, mw, mh, cx, cy int, ch Chunks) {
	sc := e.SnowC
	if c.img == nil {
		c.img = ebiten.NewImage(SnowImgW, SnowImgW)
	}
	if cap(sc.Pix) < 4*SnowImgW*SnowImgW {
		sc.Pix = make([]byte, 4*SnowImgW*SnowImgW)
	}
	pix := sc.Pix[:4*SnowImgW*SnowImgW]
	clear(pix)
	if cap(sc.corner) < (snowRing+1)*(snowRing+1) {
		sc.corner = make([]float64, (snowRing+1)*(snowRing+1))
		sc.open = make([]bool, snowRing*snowRing)
	}
	corner, open := sc.corner[:(snowRing+1)*(snowRing+1)], sc.open[:snowRing*snowRing]
	ox, oy := cx*SnowChunkTiles-1, cy*SnowChunkTiles-1 // tile of ring (0, 0)
	perTile := len(w.Snow) > 0
	c.Any = false
	for ry := 0; ry < snowRing; ry++ {
		for rx := 0; rx < snowRing; rx++ {
			x, y := ox+rx, oy+ry
			open[ry*snowRing+rx] = x >= 0 && y >= 0 && x < mw && y < mh && e.GroundOpen(w, x, y)
		}
	}
	for ry := 0; ry <= snowRing; ry++ {
		for rx := 0; rx <= snowRing; rx++ {
			x, y := ox+rx, oy+ry
			corner[ry*(snowRing+1)+rx] = snowField(float64(x), float64(y), e.SnowCorner(x, y))
		}
	}
	eachTexel(SnowPx, cx, cy, open, func(tx, ty, rx, ry int, u, v float64) {
		i := ry*(snowRing+1) + rx
		top := corner[i] + (corner[i+1]-corner[i])*u
		bot := corner[i+snowRing+1] + (corner[i+snowRing+2]-corner[i+snowRing+1])*u
		if m := uint8(255*(top+(bot-top)*v) + 0.5); m > 0 {
			o := 4 * (ty*SnowImgW + tx)
			pix[o], pix[o+1], pix[o+2], pix[o+3] = m, m, m, m
			c.Any = true
		}
	})
	c.img.WritePixels(pix)
	wet := WetBucket(w.Wetness)
	e.bakeWet(c, &w, open, mw, mh, cx, cy, wet, ch)
	if perTile {
		c.cells = append(c.cells[:0], sc.gather(w.Snow, w.TerrainW, mh, cx, cy)...)
		c.depth = 0
	} else {
		c.cells = c.cells[:0]
		c.depth = legacySnowByte(e.Snow)
	}
	c.wet, c.Plants = wet, 0
	if wet > 0 {
		c.Plants = e.SnowC.puddleCropKey(cx, cy, mw, mh, ch)
	}
	c.ver, c.gen, c.built = w.SnowVer, sc.gen, true
	sc.Bakes++
}

// bakeWet renders chunk (cx, cy)'s wet veil at wetness bucket wet into
// c.wetImg, at sc.WantPx texels a tile; dry ground leaves no image.
func (e *Layer) bakeWet(c *SnowChunk, w *game.WorldView, open []bool, mw, mh, cx, cy int, wet uint8, ch Chunks) {
	sc := e.SnowC
	px := sc.WantPx
	if px <= 0 {
		px = SnowPx
	}
	c.WetPx, c.anyWet = px, false
	wb := &sc.wb
	e.prepareWet(wb, w, open, mw, mh, cx, cy, wet, ch)
	if wb.aw <= 0 {
		if c.wetImg != nil {
			c.wetImg.Deallocate()
			c.wetImg = nil
		}
		c.Pud = nil
		return
	}
	iw := WetImgW(px)
	if c.wetImg != nil && c.wetImg.Bounds().Dx() != iw {
		c.wetImg.Deallocate()
		c.wetImg = nil
	}
	if c.wetImg == nil {
		c.wetImg = ebiten.NewImage(iw, iw)
	}
	if cap(sc.Wpix) < 4*iw*iw {
		sc.Wpix = make([]byte, 4*iw*iw)
	}
	wpix := sc.Wpix[:4*iw*iw]
	clear(wpix)
	pud := c.Pud[:0]
	if wb.pools {
		if cap(pud) < iw*iw {
			pud = make([]uint8, iw*iw)
		}
		pud = pud[:iw*iw]
		clear(pud)
	}
	// the height of every texel that may hold water (NaN: dry), then each
	// texel's slope across its neighbours
	if cap(sc.height) < iw*iw {
		sc.height = make([]float32, iw*iw)
	}
	hgt := sc.height[:iw*iw]
	nan := float32(math.NaN())
	for i := range hgt {
		hgt[i] = nan
	}
	if wb.pools {
		eachTexel(px, cx, cy, open, func(tx, ty, rx, ry int, u, v float64) {
			if wb.wet(rx, ry) {
				hgt[ty*iw+tx] = float32(wb.height(rx, ry, u, v))
			}
		})
	}
	step := float64(px)                      // texels a tile
	diff := func(i, d, lo, hi int) float64 { // the slope through texel i along stride d
		c := float64(hgt[i])
		a, b := c, c
		n := 0.0
		if lo > 0 && hgt[i-d] == hgt[i-d] {
			a, n = float64(hgt[i-d]), n+1
		}
		if hi > 0 && hgt[i+d] == hgt[i+d] {
			b, n = float64(hgt[i+d]), n+1
		}
		if n == 0 {
			return 0
		}
		return (b - a) / n * step
	}
	anyPud := false
	eachTexel(px, cx, cy, open, func(tx, ty, rx, ry int, u, v float64) {
		a, refl, cov := wb.aw, wb.aw*wetRefl, 0.0
		if i := ty*iw + tx; hgt[i] == hgt[i] {
			sx := diff(i, 1, tx, iw-1-tx)
			sy := diff(i, iw, ty, iw-1-ty)
			a, refl, cov = wb.shade(float64(hgt[i]), math.Hypot(sx, sy), wb.snowFade(rx, ry, u, v))
		}
		if a <= 0 {
			return
		}
		o := 4 * (ty*iw + tx)
		g := uint8(255*refl + 0.5)
		wpix[o], wpix[o+1], wpix[o+2], wpix[o+3] = g, g, g, uint8(255*a+0.5)
		c.anyWet = true
		if cov > 0 && wb.pools {
			if p := uint8(255*cov + 0.5); p > 0 {
				pud[ty*iw+tx] = p
				anyPud = true
			}
		}
	})
	c.wetImg.WritePixels(wpix)
	c.Pud = nil
	if anyPud {
		c.Pud = pud
	}
}

// eachTexel calls fn for every texel of a chunk (cx, cy) image at px
// texels a tile (its pad included) that lies on an open tile: the texel,
// its ring tile and the texel centre's place (u, v) in that tile.
func eachTexel(px, cx, cy int, open []bool, fn func(tx, ty, rx, ry int, u, v float64)) {
	iw := WetImgW(px)
	ox, oy := cx*SnowChunkTiles-1, cy*SnowChunkTiles-1
	for ty := 0; ty < iw; ty++ {
		py := float64(cy*SnowChunkTiles) + (float64(ty-SnowPad)+0.5)/float64(px)
		fy := math.Floor(py)
		ry := int(fy) - oy
		for tx := 0; tx < iw; tx++ {
			pxf := float64(cx*SnowChunkTiles) + (float64(tx-SnowPad)+0.5)/float64(px)
			fx := math.Floor(pxf)
			rx := int(fx) - ox
			if rx < 0 || ry < 0 || rx >= snowRing || ry >= snowRing || !open[ry*snowRing+rx] {
				continue
			}
			fn(tx, ty, rx, ry, pxf-fx, py-fy)
		}
	}
}

func legacySnowByte(d float64) uint8 { return uint8(255*gfx.Clamp01(d) + 0.5) }

// syncSnowCache drops the bakes of another map and moves the generation
// when the ground under them changed; it returns the map's size.
func (sc *SnowCache) sync(v *render.View, w game.WorldView) (mw, mh int) {
	mw, mh = v.MapW, v.MapH
	if w.TerrainW > 0 {
		mw = w.TerrainW
	}
	if sc.Chunks == nil || sc.w != mw || sc.h != mh {
		sc.release()
		sc.Chunks, sc.w, sc.h = map[int]*SnowChunk{}, mw, mh
		sc.key = snowKey{}
		sc.gen++
	}
	key := snowKey{structVer: w.StructVer}
	if len(w.RoomGrid) > 0 {
		key.rooms = &w.RoomGrid[0]
	}
	if len(w.RoofCells) > 0 {
		key.roofs = &w.RoofCells[0]
	}
	if key != sc.key {
		sc.key = key
		sc.gen++
	}
	return mw, mh
}

// eachSnowChunk walks the chunks over the visible tiles at tile size
// tile, rebaking stale ones (a chunk never baked at once, the others
// within the pass's cost budget), and calls fn with each chunk and its
// visible tile range. Only a pass with crops set checks the chunks' crop
// keys: the wet veil's, which runs first in the frame. ch is the chunk
// index the bakes read.
func (e *Layer) eachSnowChunk(v *render.View, w game.WorldView, tile float64, crops bool, ch Chunks, fn func(c *SnowChunk, cx, cy, tx0, ty0, tx1, ty1 int)) {
	sc := e.SnowC
	mw, mh := sc.sync(v, w)
	x0, y0, x1, y1 := e.VisibleTiles(v)
	if x1 < x0 || y1 < y0 {
		return
	}
	sc.pass++
	if sc.pass%256 == 0 {
		sc.evict()
	}
	sc.budget = PuddleBakeBudget
	sc.WantPx = WetPxFor(tile)
	cost := bakeCost(sc.WantPx)
	depth, wet := legacySnowByte(e.Snow), WetBucket(w.Wetness)
	ncx := (mw + SnowChunkTiles - 1) / SnowChunkTiles
	span := SnowChunkTiles
	for cy := y0 / span; cy <= y1/span; cy++ {
		for cx := x0 / span; cx <= x1/span; cx++ {
			id := cy*ncx + cx
			c := sc.Chunks[id]
			if c == nil {
				c = &SnowChunk{}
				sc.Chunks[id] = c
			}
			c.seen = sc.pass
			plants := func() uint64 { return c.Plants }
			if crops {
				plants = func() uint64 { return e.SnowC.puddleCropKey(cx, cy, mw, mh, ch) }
			}
			if (!c.built || sc.budget >= cost) && sc.stale(c, w, depth, wet, mw, mh, cx, cy, plants) {
				if c.built {
					sc.budget -= cost
				}
				e.BakeSnowChunk(c, w, mw, mh, cx, cy, ch)
			}
			fn(c, cx, cy, max(cx*span, x0), max(cy*span, y0), min(cx*span+span-1, x1), min(cy*span+span-1, y1))
		}
	}
}

// drawChunkMask stamps img, a chunk (cx, cy) mask at px texels a tile,
// over tiles tx0..tx1 × ty0..ty1, tinted col: lit per tile corner, or on
// the map tier as one flat lit quad (the masks are zero under roofs, and
// lamp falloff is invisible at a few pixels a tile), through s.
func drawChunkMask(v *render.View, screen, img *ebiten.Image, px, cx, cy, tx0, ty0, tx1, ty1 int, tile float64, col [4]float32, s Stamp) {
	span := SnowChunkTiles
	if tierFor(tile) == tierMap {
		sx, sy := v.WorldToScreenF(float64(cx*span), float64(cy*span))
		s.DrawChunkFlat(screen, img, SnowPad, SnowPad, float64(sx), float64(sy), tile*float64(span), float64(SnowChunkTiles*px), col)
		return
	}
	s.DrawBakedColor(screen, img, cx*span, cy*span, SnowPad, SnowPad, float64(px), tx0, ty0, tx1, ty1, col)
}

// drawSnowLayer stamps the snow veil (veil strength) over the open ground
// in view, tile physical pixels a tile. The wet veil under it is
// DrawPuddleVeil's (puddle_veil.go), earlier in the frame.
func (e *Layer) drawSnowLayer(v *render.View, screen *ebiten.Image, w game.WorldView, tile, veil float64, s Stamp, ch Chunks) {
	sc := e.SnowC
	sc.Draws = 0
	a := float32(veil)
	snowCol := [4]float32{236.0 / 255 * a, 240.0 / 255 * a, 246.0 / 255 * a, a}
	e.eachSnowChunk(v, w, tile, false, ch, func(c *SnowChunk, cx, cy, tx0, ty0, tx1, ty1 int) {
		if a > 0 && c.Any {
			drawChunkMask(v, screen, c.img, SnowPx, cx, cy, tx0, ty0, tx1, ty1, tile, snowCol, s)
			sc.Draws++
		}
	})
}
