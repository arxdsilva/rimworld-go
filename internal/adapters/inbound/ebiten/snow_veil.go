package ebiten

// Per-tile snow for the ground cover (PLAY-015 §4): the veil, cover decals
// and drifts read each tile's own depth from WorldView.Snow (0..255), so
// roofed ground, cleared paths and shovelled yards show bare. Without the
// layer (no snow lying, an older sim) every tile reads the map figure
// WorldView.SnowDepth, as before.

// snowDepthAt is tile (x, y)'s snow depth, 0..1.
func (e *envState) snowDepthAt(x, y int) float64 {
	if len(e.snowCells) == 0 || e.terrW <= 0 {
		return e.snow
	}
	if x < 0 || y < 0 || x >= e.terrW {
		return 0
	}
	i := y*e.terrW + x
	if i >= len(e.snowCells) {
		return 0
	}
	return float64(e.snowCells[i]) / 255
}

// snowCorner is the snow depth at the corner shared by tiles (cx-1, cy-1)
// .. (cx, cy): the mean of those on the map, so the veil shades smoothly
// from a tile to its neighbours.
func (e *envState) snowCorner(cx, cy int) float64 {
	if len(e.snowCells) == 0 || e.terrW <= 0 {
		return e.snow
	}
	h := len(e.snowCells) / e.terrW
	sum, n := 0, 0
	for y := cy - 1; y <= cy; y++ {
		for x := cx - 1; x <= cx; x++ {
			if x < 0 || y < 0 || x >= e.terrW || y >= h {
				continue
			}
			sum += int(e.snowCells[y*e.terrW+x])
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n) / 255
}
