package ebiten

import (
	"bytes"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
// else within the pass's cost budget (puddleBakeBudget), the rest showing
// their last bake a frame or two longer.
//
// Each image holds px texels a tile, framed by a one-texel pad sampled
// from the true neighbouring ground so bilinear taps across a chunk edge
// match the next chunk:
//   - snow (snowImgW square, snowPx a tile): premultiplied white
//     (m, m, m, m); a texel is the old per-tile veil quad's alpha at that
//     point: the four corner values (snowField over snowCorner)
//     interpolated across the tile, zero on tiles that are not open ground.
//     Drawing tints it (236, 240, 246) and scales it by the veil strength.
//   - wet (wetPxFor the zoom: 8 a tile live, 4 on chunk bakes, 2 on the
//     map bake; puddle_veil.go): alpha the darkening of open ground (the
//     wet darkening, deeper in the puddles), grey the reflection weight;
//     the draw tints the grey with the sky (puddleSky). Crisp puddle shores
//     need the 8 texels up close; far out 2 keep the memory small.

const (
	snowChunkTiles = 32 // tiles per chunk side
	snowPx         = 4  // baked snow texels per tile
	snowPad        = 1
	snowImgW       = snowChunkTiles*snowPx + 2*snowPad
	snowRing       = snowChunkTiles + 2 // tiles per side with the 1-tile ring
)

// snowChunk is one baked chunk.
type snowChunk struct {
	img    *ebiten.Image // the snow mask
	wetImg *ebiten.Image // the wet veil, wetPx texels a tile (nil: none)
	wetPx  int           // wet texels per tile baked at
	cells  []uint8       // the chunk's snow bytes and a one-tile ring (empty: legacy depth)
	depth  uint8         // legacy uniform depth baked (no per-tile layer)
	gen    uint64        // snowCache.gen baked at
	ver    uint64        // SnowVer cells were last verified at
	wet    uint8         // wetness bucket baked (wetBucket)
	plants uint64        // crop key baked (puddleCropKey), while wet > 0
	// pud is the puddle cover per wet texel (wetImgW(wetPx)²), nil when
	// the chunk holds no water: where raindrops ripple (puddleAt).
	pud    []uint8
	seen   uint32 // the last pass that walked it (snowCache.pass)
	built  bool
	any    bool // some snow
	anyWet bool // some wet veil
}

// snowEvictPasses: a chunk no pass walked for this many passes (seconds
// of frames) is dropped with its images, so panning a big map at close
// zoom does not keep every chunk's wet veil; it is rebaked on return.
const snowEvictPasses = 2400

// wetImgW is the side of a wet image at px texels a tile.
func wetImgW(px int) int { return snowChunkTiles*px + 2*snowPad }

// wetPxFor is the wet veil's texels a tile at this tile size.
func wetPxFor(tile float64) int {
	switch lodFor(tile) {
	case lodLive:
		return 8
	case lodChunk:
		return 4
	}
	return 2
}

// bakeCost is a chunk bake's share of the pass budget at px wet texels a
// tile: its texels over a 4-texel bake's.
func bakeCost(px int) int { return max(1, px*px/16) }

// snowCache holds the baked chunks of the current map.
type snowCache struct {
	chunks map[int]*snowChunk
	w, h   int
	// gen moves when what a chunk depends on besides its snow bytes moves:
	// terrain and buildings (StructVer: floors too), the room grid or the
	// roof layer (both shared read-only slices that change identity when
	// their content does).
	gen    uint64
	key    snowKey
	pix    []byte    // the snow bake's texels
	wpix   []byte    // the wet bake's texels
	height []float32 // the wet bake's texel heights (NaN: dry)
	ring   []byte
	corner []float64 // (snowRing+1)² corner veil alphas
	open   []bool    // snowRing² open tiles
	wb     wetBake   // the bake's wet inputs (puddle_veil.go)
	// wantPx is the wet detail this pass bakes at (0: snowPx).
	wantPx int
	// pass counts chunk walks; cropVer memoises each read-model chunk's
	// PlantVer for the pass stamped in cropPass.
	pass     uint32
	cropVer  []uint64
	cropPass []uint32
	budget   int // bake cost this pass may still spend on rebakes
	bakes    int // chunk bakes (tests)
	draws    int // snow-pass draws of the last pass (tests)
	wetDraws int // wet-veil draws of the last pass (tests)
}

type snowKey struct {
	structVer uint64
	rooms     *int
	roofs     *uint8
}

// release drops every bake.
func (sc *snowCache) release() {
	for _, c := range sc.chunks {
		for _, img := range []*ebiten.Image{c.img, c.wetImg} {
			if img != nil {
				img.Deallocate()
			}
		}
	}
	sc.chunks = nil
}

// evict drops the chunks no pass walked for snowEvictPasses.
func (sc *snowCache) evict() {
	for id, c := range sc.chunks {
		if sc.pass-c.seen <= snowEvictPasses {
			continue
		}
		for _, img := range []*ebiten.Image{c.img, c.wetImg} {
			if img != nil {
				img.Deallocate()
			}
		}
		delete(sc.chunks, id)
	}
}

// gather copies chunk (cx, cy)'s snow bytes and a one-tile ring (zero off
// the map) from cells, mw tiles wide, into sc.ring.
func (sc *snowCache) gather(cells []uint8, mw, mh, cx, cy int) []byte {
	if cap(sc.ring) < snowRing*snowRing {
		sc.ring = make([]byte, snowRing*snowRing)
	}
	ring := sc.ring[:snowRing*snowRing]
	for ry := 0; ry < snowRing; ry++ {
		y := cy*snowChunkTiles + ry - 1
		for rx := 0; rx < snowRing; rx++ {
			x := cx*snowChunkTiles + rx - 1
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
func (sc *snowCache) stale(c *snowChunk, w game.WorldView, depth, wet uint8, mw, mh, cx, cy int, plants func() uint64) bool {
	if !c.built || c.gen != sc.gen || c.wet != wet || wet > 0 && c.wetPx != sc.wantPx {
		return true
	}
	if wet > 0 && c.plants != plants() {
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

// bakeSnowChunk renders chunk (cx, cy) into c: its snow mask and its wet
// veil at the pass's detail (sc.wantPx).
func (r *Renderer) bakeSnowChunk(c *snowChunk, w game.WorldView, mw, mh, cx, cy int) {
	sc := &r.snowc
	e := &r.env
	if c.img == nil {
		c.img = ebiten.NewImage(snowImgW, snowImgW)
	}
	if cap(sc.pix) < 4*snowImgW*snowImgW {
		sc.pix = make([]byte, 4*snowImgW*snowImgW)
	}
	pix := sc.pix[:4*snowImgW*snowImgW]
	clear(pix)
	if cap(sc.corner) < (snowRing+1)*(snowRing+1) {
		sc.corner = make([]float64, (snowRing+1)*(snowRing+1))
		sc.open = make([]bool, snowRing*snowRing)
	}
	corner, open := sc.corner[:(snowRing+1)*(snowRing+1)], sc.open[:snowRing*snowRing]
	ox, oy := cx*snowChunkTiles-1, cy*snowChunkTiles-1 // tile of ring (0, 0)
	perTile := len(w.Snow) > 0
	c.any = false
	for ry := 0; ry < snowRing; ry++ {
		for rx := 0; rx < snowRing; rx++ {
			x, y := ox+rx, oy+ry
			open[ry*snowRing+rx] = x >= 0 && y >= 0 && x < mw && y < mh && r.groundOpen(w, x, y)
		}
	}
	for ry := 0; ry <= snowRing; ry++ {
		for rx := 0; rx <= snowRing; rx++ {
			x, y := ox+rx, oy+ry
			corner[ry*(snowRing+1)+rx] = snowField(float64(x), float64(y), e.snowCorner(x, y))
		}
	}
	eachTexel(snowPx, cx, cy, open, func(tx, ty, rx, ry int, u, v float64) {
		i := ry*(snowRing+1) + rx
		top := corner[i] + (corner[i+1]-corner[i])*u
		bot := corner[i+snowRing+1] + (corner[i+snowRing+2]-corner[i+snowRing+1])*u
		if m := uint8(255*(top+(bot-top)*v) + 0.5); m > 0 {
			o := 4 * (ty*snowImgW + tx)
			pix[o], pix[o+1], pix[o+2], pix[o+3] = m, m, m, m
			c.any = true
		}
	})
	c.img.WritePixels(pix)
	wet := wetBucket(w.Wetness)
	r.bakeWet(c, &w, open, mw, mh, cx, cy, wet)
	if perTile {
		c.cells = append(c.cells[:0], sc.gather(w.Snow, w.TerrainW, mh, cx, cy)...)
		c.depth = 0
	} else {
		c.cells = c.cells[:0]
		c.depth = legacySnowByte(e.snow)
	}
	c.wet, c.plants = wet, 0
	if wet > 0 {
		c.plants = r.puddleCropKey(cx, cy, mw, mh)
	}
	c.ver, c.gen, c.built = w.SnowVer, sc.gen, true
	sc.bakes++
}

// bakeWet renders chunk (cx, cy)'s wet veil at wetness bucket wet into
// c.wetImg, at sc.wantPx texels a tile; dry ground leaves no image.
func (r *Renderer) bakeWet(c *snowChunk, w *game.WorldView, open []bool, mw, mh, cx, cy int, wet uint8) {
	sc := &r.snowc
	px := sc.wantPx
	if px <= 0 {
		px = snowPx
	}
	c.wetPx, c.anyWet = px, false
	wb := &sc.wb
	r.prepareWet(wb, w, open, mw, mh, cx, cy, wet)
	if wb.aw <= 0 {
		if c.wetImg != nil {
			c.wetImg.Deallocate()
			c.wetImg = nil
		}
		c.pud = nil
		return
	}
	iw := wetImgW(px)
	if c.wetImg != nil && c.wetImg.Bounds().Dx() != iw {
		c.wetImg.Deallocate()
		c.wetImg = nil
	}
	if c.wetImg == nil {
		c.wetImg = ebiten.NewImage(iw, iw)
	}
	if cap(sc.wpix) < 4*iw*iw {
		sc.wpix = make([]byte, 4*iw*iw)
	}
	wpix := sc.wpix[:4*iw*iw]
	clear(wpix)
	pud := c.pud[:0]
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
	c.pud = nil
	if anyPud {
		c.pud = pud
	}
}

// eachTexel calls fn for every texel of a chunk (cx, cy) image at px
// texels a tile (its pad included) that lies on an open tile: the texel,
// its ring tile and the texel centre's place (u, v) in that tile.
func eachTexel(px, cx, cy int, open []bool, fn func(tx, ty, rx, ry int, u, v float64)) {
	iw := wetImgW(px)
	ox, oy := cx*snowChunkTiles-1, cy*snowChunkTiles-1
	for ty := 0; ty < iw; ty++ {
		py := float64(cy*snowChunkTiles) + (float64(ty-snowPad)+0.5)/float64(px)
		fy := math.Floor(py)
		ry := int(fy) - oy
		for tx := 0; tx < iw; tx++ {
			pxf := float64(cx*snowChunkTiles) + (float64(tx-snowPad)+0.5)/float64(px)
			fx := math.Floor(pxf)
			rx := int(fx) - ox
			if rx < 0 || ry < 0 || rx >= snowRing || ry >= snowRing || !open[ry*snowRing+rx] {
				continue
			}
			fn(tx, ty, rx, ry, pxf-fx, py-fy)
		}
	}
}

func legacySnowByte(d float64) uint8 { return uint8(255*clamp01(d) + 0.5) }

// syncSnowCache drops the bakes of another map and moves the generation
// when the ground under them changed; it returns the map's size.
func (r *Renderer) syncSnowCache(w game.WorldView) (mw, mh int) {
	sc := &r.snowc
	mw, mh = r.game.Width(), r.game.Height()
	if w.TerrainW > 0 {
		mw = w.TerrainW
	}
	if sc.chunks == nil || sc.w != mw || sc.h != mh {
		sc.release()
		sc.chunks, sc.w, sc.h = map[int]*snowChunk{}, mw, mh
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
// keys: the wet veil's, which runs first in the frame.
func (r *Renderer) eachSnowChunk(w game.WorldView, tile float64, crops bool, fn func(c *snowChunk, cx, cy, tx0, ty0, tx1, ty1 int)) {
	sc := &r.snowc
	mw, mh := r.syncSnowCache(w)
	x0, y0, x1, y1 := r.visibleTiles()
	if x1 < x0 || y1 < y0 {
		return
	}
	sc.pass++
	if sc.pass%256 == 0 {
		sc.evict()
	}
	sc.budget = puddleBakeBudget
	sc.wantPx = wetPxFor(tile)
	cost := bakeCost(sc.wantPx)
	depth, wet := legacySnowByte(r.env.snow), wetBucket(w.Wetness)
	ncx := (mw + snowChunkTiles - 1) / snowChunkTiles
	span := snowChunkTiles
	for cy := y0 / span; cy <= y1/span; cy++ {
		for cx := x0 / span; cx <= x1/span; cx++ {
			id := cy*ncx + cx
			c := sc.chunks[id]
			if c == nil {
				c = &snowChunk{}
				sc.chunks[id] = c
			}
			c.seen = sc.pass
			plants := func() uint64 { return c.plants }
			if crops {
				plants = func() uint64 { return r.puddleCropKey(cx, cy, mw, mh) }
			}
			if (!c.built || sc.budget >= cost) && sc.stale(c, w, depth, wet, mw, mh, cx, cy, plants) {
				if c.built {
					sc.budget -= cost
				}
				r.bakeSnowChunk(c, w, mw, mh, cx, cy)
			}
			fn(c, cx, cy, max(cx*span, x0), max(cy*span, y0), min(cx*span+span-1, x1), min(cy*span+span-1, y1))
		}
	}
}

// drawChunkMask stamps img, a chunk (cx, cy) mask at px texels a tile,
// over tiles tx0..tx1 × ty0..ty1, tinted col: lit per tile corner, or on
// the map tier as one flat lit quad (the masks are zero under roofs, and
// lamp falloff is invisible at a few pixels a tile).
func (r *Renderer) drawChunkMask(screen, img *ebiten.Image, px, cx, cy, tx0, ty0, tx1, ty1 int, tile float64, col [4]float32) {
	span := snowChunkTiles
	if lodFor(tile) == lodMap {
		sx, sy := r.worldToScreenF(float64(cx*span), float64(cy*span))
		r.drawChunkFlat(screen, img, snowPad, snowPad, float64(sx), float64(sy), tile*float64(span), float64(snowChunkTiles*px), col)
		return
	}
	r.drawBakedColor(screen, img, cx*span, cy*span, snowPad, snowPad, float64(px), tx0, ty0, tx1, ty1, col)
}

// drawSnowLayer stamps the snow veil (veil strength) over the open ground
// in view, tile physical pixels a tile. The wet veil under it is
// drawPuddleVeil's (puddle_veil.go), earlier in the frame.
func (r *Renderer) drawSnowLayer(screen *ebiten.Image, w game.WorldView, sw, sh int, tile, veil float64) {
	sc := &r.snowc
	sc.draws = 0
	v := float32(veil)
	snowCol := [4]float32{236.0 / 255 * v, 240.0 / 255 * v, 246.0 / 255 * v, v}
	r.eachSnowChunk(w, tile, false, func(c *snowChunk, cx, cy, tx0, ty0, tx1, ty1 int) {
		if v > 0 && c.any {
			r.drawChunkMask(screen, c.img, snowPx, cx, cy, tx0, ty0, tx1, ty1, tile, snowCol)
			sc.draws++
		}
	})
}
