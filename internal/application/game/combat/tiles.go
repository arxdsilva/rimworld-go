package combat

// Tiles finds free ground: moved unchanged from the game's combat.go
// (DEF-3, package budget). Raiders, visitors and fleeing creatures pick
// their spots with it.
type Tiles struct{ f Field }

// WalkableNear clamps (x, y) to the map and finds a walkable tile within
// r of it.
func (t *Tiles) WalkableNear(x, y, r int) (int, int, bool) {
	w, h := t.f.Size()
	x, y = min(max(x, 0), w-1), min(max(y, 0), h-1)
	for d := 0; d <= r; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if t.f.DryWalkable(x+dx, y+dy) {
					if !t.f.TileTaken(x+dx, y+dy) {
						return x + dx, y + dy, true
					}
				}
			}
		}
	}
	return 0, 0, false
}
