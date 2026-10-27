package ebiten

import (
	"bytes"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The snow veil and the wet darkening of open ground (PLAY-015 §4, PERF-002).
//
// Both are masks baked per 32-tile map chunk into one small white image and
// stamped through the lit shader (lod.go drawBakedColor / drawChunkFlat), so
// snow dims at night, catches lamp light and goes dark under roofs like the
// ground below it. A frame costs one or a few draws per visible chunk and
// no per-tile work; a chunk is rebaked only when its own snow bytes (or
// the ground it covers) changed.
//
// Chunk image layout (snowImgW = 32*snowPx + 2*snowPad = 130 square, twice
// as tall): the top square is the snow mask, the bottom square the open-
// ground mask (wetness). Each is premultiplied white (m, m, m, m) at snowPx
// texels a tile, framed by a one-texel pad sampled from the true
// neighbouring ground so bilinear taps across a chunk edge match the
// next chunk. A texel is the old per-tile veil quad's alpha at that point:
// the four corner values (snowField over snowCorner) interpolated across
// the tile, zero on tiles that are not open ground. The mask is the veil
// strength 1; drawing tints it (236, 240, 246) and scales it by the veil.

const (
	snowChunkTiles = 32 // tiles per chunk side
	snowPx         = 4  // baked texels per tile
	snowPad        = 1
	snowImgW       = snowChunkTiles*snowPx + 2*snowPad
	snowImgH       = 2 * snowImgW
	snowRing       = snowChunkTiles + 2 // tiles per side with the 1-tile ring
)

// snowChunk is one baked chunk.
type snowChunk struct {
	img     *ebiten.Image
	cells   []uint8 // the chunk's snow bytes and a one-tile ring (empty: legacy depth)
	depth   uint8   // legacy uniform depth baked (no per-tile layer)
	gen     uint64  // snowCache.gen baked at
	ver     uint64  // SnowVer cells were last verified at
	built   bool
	any     bool // some snow
	anyOpen bool // some open ground
}

// snowCache holds the baked chunks of the current map.
type snowCache struct {
	chunks map[int]*snowChunk
	w, h   int
	// gen moves when what a chunk depends on besides its snow bytes moves:
	// terrain (StructVer), the room grid or the roof layer (both shared
	// read-only slices that change identity when their content does).
	gen    uint64
	key    snowKey
	pix    []byte
	ring   []byte
	corner []float64 // (snowRing+1)² corner veil alphas
	open   []bool    // snowRing² open tiles
	bakes  int       // chunk bakes (tests)
	draws  int       // snow-pass draws of the last pass (tests)
}

type snowKey struct {
	structVer uint64
	rooms     *int
	roofs     *uint8
}

// release drops every bake.
func (sc *snowCache) release() {
	for _, c := range sc.chunks {
		if c.img != nil {
			c.img.Deallocate()
		}
	}
	sc.chunks = nil
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
// or its own snow differs from what it baked. The bytes are compared only
// when SnowVer moved since the chunk last agreed with it.
func (sc *snowCache) stale(c *snowChunk, w game.WorldView, depth uint8, mw, mh, cx, cy int) bool {
	if !c.built || c.gen != sc.gen {
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

// bake renders chunk (cx, cy) into c.
func (r *Renderer) bakeSnowChunk(c *snowChunk, w game.WorldView, mw, mh, cx, cy int) {
	sc := &r.snowc
	e := &r.env
	if c.img == nil {
		c.img = ebiten.NewImage(snowImgW, snowImgH)
	}
	if cap(sc.pix) < 4*snowImgW*snowImgH {
		sc.pix = make([]byte, 4*snowImgW*snowImgH)
	}
	pix := sc.pix[:4*snowImgW*snowImgH]
	clear(pix)
	if cap(sc.corner) < (snowRing+1)*(snowRing+1) {
		sc.corner = make([]float64, (snowRing+1)*(snowRing+1))
		sc.open = make([]bool, snowRing*snowRing)
	}
	corner, open := sc.corner[:(snowRing+1)*(snowRing+1)], sc.open[:snowRing*snowRing]
	ox, oy := cx*snowChunkTiles-1, cy*snowChunkTiles-1 // tile of ring (0, 0)
	perTile := len(w.Snow) > 0
	c.any, c.anyOpen = false, false
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
	for ty := 0; ty < snowImgW; ty++ {
		py := float64(cy*snowChunkTiles) + (float64(ty-snowPad)+0.5)/snowPx
		fy := math.Floor(py)
		ry := int(fy) - oy
		v := py - fy
		for tx := 0; tx < snowImgW; tx++ {
			px := float64(cx*snowChunkTiles) + (float64(tx-snowPad)+0.5)/snowPx
			fx := math.Floor(px)
			rx := int(fx) - ox
			if rx < 0 || ry < 0 || rx >= snowRing || ry >= snowRing || !open[ry*snowRing+rx] {
				continue
			}
			u := px - fx
			i := ry*(snowRing+1) + rx
			top := corner[i] + (corner[i+1]-corner[i])*u
			bot := corner[i+snowRing+1] + (corner[i+snowRing+2]-corner[i+snowRing+1])*u
			m := uint8(255*(top+(bot-top)*v) + 0.5)
			o := 4 * (ty*snowImgW + tx)
			if m > 0 {
				pix[o], pix[o+1], pix[o+2], pix[o+3] = m, m, m, m
				c.any = true
			}
			o = 4 * ((ty+snowImgW)*snowImgW + tx)
			pix[o], pix[o+1], pix[o+2], pix[o+3] = 255, 255, 255, 255
			c.anyOpen = true
		}
	}
	c.img.WritePixels(pix)
	if perTile {
		c.cells = append(c.cells[:0], sc.gather(w.Snow, w.TerrainW, mh, cx, cy)...)
		c.depth = 0
	} else {
		c.cells = c.cells[:0]
		c.depth = legacySnowByte(e.snow)
	}
	c.ver, c.gen, c.built = w.SnowVer, sc.gen, true
	sc.bakes++
}

func legacySnowByte(d float64) uint8 { return uint8(255*clamp01(d) + 0.5) }

// drawSnowLayer stamps the wet darkening (wet 0..1) and the snow veil
// (veil strength) over the open ground in view, tile physical pixels a
// tile. Chunks rebake only when stale (snow_cache.go header).
func (r *Renderer) drawSnowLayer(screen *ebiten.Image, w game.WorldView, sw, sh int, tile, wet, veil float64) {
	sc := &r.snowc
	sc.draws = 0
	mw, mh := r.game.Width(), r.game.Height()
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
	depth := legacySnowByte(r.env.snow)
	x0, y0, x1, y1 := r.visibleTiles()
	if x1 < x0 || y1 < y0 {
		return
	}
	ncx := (mw + snowChunkTiles - 1) / snowChunkTiles
	flat := lodFor(tile) == lodMap
	wetCol := [4]float32{}
	if k := float32(0.16 * wet); k > 0 {
		wetCol = [4]float32{20.0 / 255 * k, 24.0 / 255 * k, 30.0 / 255 * k, k}
	}
	v := float32(veil)
	snowCol := [4]float32{236.0 / 255 * v, 240.0 / 255 * v, 246.0 / 255 * v, v}
	span := snowChunkTiles
	for cy := y0 / span; cy <= y1/span; cy++ {
		for cx := x0 / span; cx <= x1/span; cx++ {
			id := cy*ncx + cx
			c := sc.chunks[id]
			if c == nil {
				c = &snowChunk{}
				sc.chunks[id] = c
			}
			if sc.stale(c, w, depth, mw, mh, cx, cy) {
				r.bakeSnowChunk(c, w, mw, mh, cx, cy)
			}
			tx0, ty0 := max(cx*span, x0), max(cy*span, y0)
			tx1, ty1 := min(cx*span+span-1, x1), min(cy*span+span-1, y1)
			pass := func(row int, col [4]float32) {
				if flat {
					sx, sy := r.worldToScreenF(float64(cx*span), float64(cy*span))
					r.drawChunkFlat(screen, c.img, snowPad, float64(row*snowImgW+snowPad), float64(sx), float64(sy),
						tile*float64(span), snowChunkTiles*snowPx, col)
					return
				}
				r.drawBakedColor(screen, c.img, cx*span, cy*span, snowPad, float64(row*snowImgW+snowPad), snowPx,
					tx0, ty0, tx1, ty1, col)
			}
			if wetCol[3] > 0 && c.anyOpen {
				pass(1, wetCol)
			}
			if v > 0 && c.any {
				pass(0, snowCol)
				sc.draws++
			}
		}
	}
}
