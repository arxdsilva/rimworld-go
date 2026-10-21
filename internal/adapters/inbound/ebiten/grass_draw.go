package ebiten

import "rimworld-go/internal/application/game"

// Grass tile mask bits (grassMask).
const (
	grassCovered uint8 = 1 // a building, frame or plant stands here: no tuft
	grassMarked  uint8 = 2 // marked for cutting: draw overlay/designation_cut
)

// grassScratch is reused by drawTerrainArt so no allocation happens per
// frame after warm-up (the renderer draws on one goroutine).
var grassScratch []uint8

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

// grassVisible reports whether a tuft is drawn at tile (x, y) on terrain t.
// With no simulation grass data (empty mask) it is the old renderer rule.
func grassVisible(mask []uint8, w game.WorldView, t string, x, y int) bool {
	if t != "soil" && t != "rich_soil" {
		return false
	}
	if len(mask) == 0 {
		return (tileHash(x, y)>>8)%grassEvery == 0
	}
	i := y*w.TerrainW + x
	return i >= 0 && i < len(mask) && w.Grass[i] == 1 && mask[i]&grassCovered == 0
}
