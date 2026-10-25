package game

// cellWalkable is whether pawns can stand on (x, y) (MAPGEN-3): the
// terrain is walkable (or a finished bridge covers it) and no building
// blocks it. Stub: BRIDGE-GAME adds the bridge and uses it at every site
// that writes the grid's walkable bit.
func (g *Game) cellWalkable(x, y int) bool {
	return g.terr.At(x, y).Walkable() && g.layerBlockerAt(x, y) == 0
}
