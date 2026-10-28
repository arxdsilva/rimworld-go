package ebiten

import "rimworld-go/internal/application/game"

// Grass tile mask bits (grassMask).
const (
	grassCovered uint8 = 1 // a building, frame or plant stands here: no tuft
	grassMarked  uint8 = 2 // marked for cutting: draw overlay/designation_cut
)

// grassMask fills buf (reused when big enough) with one byte per tile,
// row-major w.TerrainW wide: grassCovered for any tile under a building or
// frame footprint or holding a plant, grassMarked for grass marked for
// cutting. Hiding covered tiles here lets a paused designation or a fresh
// build hide the tuft at once.
func grassMask(w game.WorldView, buf []uint8) []uint8 {
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
				buf[yy*tw+xx] |= grassCovered
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
			buf[c[1]*tw+c[0]] |= grassMarked
		}
	}
	return buf
}

// grassFrameMask fills the renderer's grass mask (r.lod.grass.mask) for
// tiles x0..x1 × y0..y1: the rest of the buffer is left as it was and is
// not read for them. The covers come from the chunk lists over that
// range (and grassCoverReach up and left, where a footprint's anchor can
// lie) when w is the frame's world (lod.go), so no full-map pass runs
// per frame; any other world (tests) takes the whole-map grassMask.
func (r *Renderer) grassFrameMask(w *game.WorldView, x0, y0, x1, y1 int) {
	gb := &r.lod.grass
	if !r.frameLists(w) {
		gb.mask = grassMask(*w, gb.mask)
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
				buf[yy*tw+xx] |= grassCovered
			}
		}
	}
	cs := game.ChunkSize
	for cy := max(y0-grassCoverReach, 0) / cs; cy <= y1/cs; cy++ {
		for cx := max(x0-grassCoverReach, 0) / cs; cx <= x1/cs; cx++ {
			cv := r.chunkView(cx, cy)
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
	for _, c := range w.GrassCut {
		if c[0] >= x0 && c[0] <= x1 && c[1] >= y0 && c[1] <= y1 {
			buf[c[1]*tw+c[0]] |= grassMarked
		}
	}
}

// frameLists reports whether w is the world this frame's chunk lists
// index (r.view.W, synced by syncFrameChunks): the same shared lists.
func (r *Renderer) frameLists(w *game.WorldView) bool {
	d := &r.view.W
	return r.game != nil && len(r.lod.views) > 0 &&
		litFirst(w.Buildings) == litFirst(d.Buildings) && len(w.Buildings) == len(d.Buildings) &&
		litFirst(w.Plants) == litFirst(d.Plants) && len(w.Plants) == len(d.Plants) &&
		litFirst(w.Items) == litFirst(d.Items) && len(w.Items) == len(d.Items) &&
		litFirst(w.Frames) == litFirst(d.Frames) && len(w.Frames) == len(d.Frames)
}

// grassVisible reports whether a tuft is drawn at tile (x, y) on terrain
// t: grass ground (grassGroundName, the sim's rule) holding grass not
// covered in mask. With no simulation grass data (empty mask) it is the
// old hashed rule.
func grassVisible(mask []uint8, w game.WorldView, t string, x, y int) bool {
	if !grassGroundName(t) {
		return false
	}
	if len(mask) == 0 {
		return (tileHash(x, y)>>8)%grassEvery == 0
	}
	i := y*w.TerrainW + x
	return i >= 0 && i < len(mask) && w.Grass[i] == 1 && mask[i]&grassCovered == 0
}
