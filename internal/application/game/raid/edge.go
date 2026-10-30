package raid

// EdgeTile is a random walkable tile on the map edge from which the
// colony can be reached (breaking through its walls and doors if need
// be), so nothing arrives stuck in a cut-off pocket: up to a dozen dry
// edge tiles are tried, else the first dry one. Moved from the game's
// raids.go (DEF-4); the rolls are the game's main stream in the same order.
func (p *Pods) EdgeTile() (int, int, bool) {
	h := p.h
	w, ht := h.Size()
	hx, hy := h.Home()
	if x, y, ok := h.WalkableNear(hx, hy, 6); ok {
		hx, hy = x, y
	}
	var fx, fy int
	found, checks := false, 0
	for try := 0; try < 200; try++ {
		var x, y int
		switch h.Roll(4) {
		case 0:
			x, y = h.Roll(w), 0
		case 1:
			x, y = h.Roll(w), ht-1
		case 2:
			x, y = 0, h.Roll(ht)
		default:
			x, y = w-1, h.Roll(ht)
		}
		if !h.DryWalkable(x, y) {
			continue // never arrive in the water
		}
		if !found {
			fx, fy, found = x, y, true // fallback: any walkable edge tile
		}
		if checks++; checks > 12 {
			break
		}
		if p.a.Reach(x, y, hx, hy) {
			return x, y, true
		}
	}
	return fx, fy, found
}
