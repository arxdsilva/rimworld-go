package ebiten

// Per-tile snow for the ground cover (PLAY-015 §4): the veil, cover decals
// and drifts read each tile's own depth from WorldView.Snow (0..255), so
// roofed ground, cleared paths and shovelled yards show bare. Without the
// layer (no snow lying, an older sim) every tile reads the map figure
// WorldView.SnowDepth, as before.

// SnowDepthAt is tile (x, y)'s snow depth, 0..1.
func (e *envState) SnowDepthAt(x, y int) float64 {
	if len(e.SnowCells) == 0 || e.TerrW <= 0 {
		return e.Snow
	}
	if x < 0 || y < 0 || x >= e.TerrW {
		return 0
	}
	i := y*e.TerrW + x
	if i >= len(e.SnowCells) {
		return 0
	}
	return float64(e.SnowCells[i]) / 255
}

// SnowCorner is the snow depth at the corner shared by tiles (cx-1, cy-1)
// .. (cx, cy): the mean of those on the map, so the veil shades smoothly
// from a tile to its neighbours.
func (e *envState) SnowCorner(cx, cy int) float64 {
	if len(e.SnowCells) == 0 || e.TerrW <= 0 {
		return e.Snow
	}
	h := len(e.SnowCells) / e.TerrW
	sum, n := 0, 0
	for y := cy - 1; y <= cy; y++ {
		for x := cx - 1; x <= cx; x++ {
			if x < 0 || y < 0 || x >= e.TerrW || y >= h {
				continue
			}
			sum += int(e.SnowCells[y*e.TerrW+x])
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n) / 255
}
