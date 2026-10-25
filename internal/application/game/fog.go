package game

// fogState is the fog of war (MAPGEN-2 §5): which tiles are unexplored.
// The zero value is no fog (legacy colonies, older saves).
// Stub: owned by FOG-SIM.
type fogState struct {
	hidden []uint64 // bitset, 1 = unexplored; nil = no fog
	ver    uint64
}

// fogged reports an unexplored tile.
func (g *Game) fogged(x, y int) bool {
	if g.fog.hidden == nil || !g.grid.InBounds(x, y) {
		return false
	}
	i := y*g.grid.Width() + x
	return g.fog.hidden[i>>6]&(1<<(uint(i)&63)) != 0
}

// Fogged reports whether tile (x, y) is unexplored (fog of war).
func (g *Game) Fogged(x, y int) bool { return g.fogged(x, y) }
