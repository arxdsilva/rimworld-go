package game

// PLAY-015 snow contract (snow.go fills the behaviour).

// snowState is the per-tile snow depth and the snow clearing area. Stub:
// SNOW.
type snowState struct {
	depth []uint8
	clear []uint64
	ver   uint64
}

// snowAt is the snow depth on (x, y), 0..255.
func (g *Game) snowAt(x, y int) uint8 {
	i := y*g.grid.Width() + x
	if !g.grid.InBounds(x, y) || i >= len(g.snow.depth) {
		return 0
	}
	return g.snow.depth[i]
}
