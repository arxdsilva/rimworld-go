package env

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Grass tile mask bits (GrassMask).
const (
	GrassCovered uint8 = 1 // a building, frame or plant stands here: no tuft
	GrassMarked  uint8 = 2 // marked for cutting: draw overlay/designation_cut
)

// GrassMask fills buf (reused when big enough) with one byte per tile,
// row-major w.TerrainW wide: GrassCovered for any tile under a building or
// frame footprint or holding a plant, GrassMarked for grass marked for
// cutting. Hiding covered tiles here lets a paused designation or a fresh
// build hide the tuft at once.
func GrassMask(w game.WorldView, buf []uint8) []uint8 {
	n := len(w.Grass)
	if cap(buf) < n {
		buf = make([]uint8, n)
	}
	buf = buf[:n]
	for i := range buf {
		buf[i] = 0
	}
	tw := w.TerrainW
	if tw <= 0 {
		return buf
	}
	th := n / tw
	cover := func(x, y, cw, ch int) {
		cw, ch = max(cw, 1), max(ch, 1)
		for yy := max(y, 0); yy < min(y+ch, th); yy++ {
			for xx := max(x, 0); xx < min(x+cw, tw); xx++ {
				buf[yy*tw+xx] |= GrassCovered
			}
		}
	}
	for _, b := range w.Buildings {
		cover(b.X, b.Y, b.W, b.H)
	}
	for _, f := range w.Frames {
		cover(f.X, f.Y, f.W, f.H)
	}
	for _, p := range w.Plants {
		cover(p.X, p.Y, 1, 1)
	}
	for _, c := range w.GrassCut {
		if c[0] >= 0 && c[0] < tw && c[1] >= 0 && c[1] < th {
			buf[c[1]*tw+c[0]] |= GrassMarked
		}
	}
	return buf
}

// GrassFrameMask fills the grass mask (Grass.mask) for tiles x0..x1 ×
// y0..y1: the rest of the buffer is left as it was and is not read for
// them. The covers come from c's chunk lists over that range (and
// grassCoverReach up and left, where a footprint's anchor can lie) when w
// is the frame's world (lod.go), so no full-map pass runs per frame; any
// other world (tests) takes the whole-map GrassMask.
func (e *Layer) GrassFrameMask(w *game.WorldView, x0, y0, x1, y1 int, c Chunks) {
	gb := e.Grass
	if !c.FrameLists(ListsOf(w)) {
		gb.mask = GrassMask(*w, gb.mask)
		return
	}
	n := len(w.Grass)
	if cap(gb.mask) < n {
		gb.mask = make([]uint8, n)
	}
	buf := gb.mask[:n]
	gb.mask = buf
	tw := w.TerrainW
	if tw <= 0 || n == 0 {
		return
	}
	th := n / tw
	x0, y0, x1, y1 = max(x0, 0), max(y0, 0), min(x1, tw-1), min(y1, th-1)
	if x1 < x0 || y1 < y0 {
		return
	}
	for y := y0; y <= y1; y++ {
		clear(buf[y*tw+x0 : y*tw+x1+1])
	}
	cover := func(x, y, cw, ch int) {
		cw, ch = max(cw, 1), max(ch, 1)
		for yy := max(y, y0); yy < min(y+ch, y1+1); yy++ {
			for xx := max(x, x0); xx < min(x+cw, x1+1); xx++ {
				buf[yy*tw+xx] |= GrassCovered
			}
		}
	}
	cs := game.ChunkSize
	for cy := max(y0-grassCoverReach, 0) / cs; cy <= y1/cs; cy++ {
		for cx := max(x0-grassCoverReach, 0) / cs; cx <= x1/cs; cx++ {
			cv := c.ChunkView(cx, cy)
			for _, i := range cv.Buildings {
				if int(i) < len(w.Buildings) {
					b := &w.Buildings[i]
					cover(b.X, b.Y, b.W, b.H)
				}
			}
			for _, i := range cv.Frames {
				if int(i) < len(w.Frames) {
					f := &w.Frames[i]
					cover(f.X, f.Y, f.W, f.H)
				}
			}
			for _, i := range cv.Plants {
				if int(i) < len(w.Plants) {
					p := &w.Plants[i]
					cover(p.X, p.Y, 1, 1)
				}
			}
		}
	}
	for _, g := range w.GrassCut {
		if g[0] >= x0 && g[0] <= x1 && g[1] >= y0 && g[1] <= y1 {
			buf[g[1]*tw+g[0]] |= GrassMarked
		}
	}
}

// GrassVisible reports whether a tuft is drawn at tile (x, y) on terrain
// t: grass ground (grassGroundName, the sim's rule) holding grass not
// covered in mask. With no simulation grass data (empty mask) it is the
// old hashed rule.
func GrassVisible(mask []uint8, w game.WorldView, t string, x, y int) bool {
	if !grassGroundName(t) {
		return false
	}
	if len(mask) == 0 {
		return (sprite.TileHash(x, y)>>8)%GrassEvery == 0
	}
	i := y*w.TerrainW + x
	return i >= 0 && i < len(mask) && w.Grass[i] == 1 && mask[i]&GrassCovered == 0
}
