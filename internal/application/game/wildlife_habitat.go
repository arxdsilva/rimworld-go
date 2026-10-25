package game

import (
	"rimworld-go/internal/domain/animal"
)

// Habitat-driven herd placement (MAPGEN-2 §3.7). Setup games choose herd
// spawn points on dry, walkable, open, explored land the colony can reach
// and far enough from the landing spot; grazers lean toward grass and
// plants, predators spread out. Legacy colonies never come through here.

const (
	// habitatMinDist is the least Chebyshev distance of a herd point from
	// the landing spot.
	habitatMinDist = 25
	// habitatSamples is how many candidate cells one placement draws.
	habitatSamples = 48
	// forageRadius is the half-width of the window scanned for food.
	forageRadius = 3
	// plantForage is how many grass tiles one wild plant is worth.
	plantForage = 3
)

// habitat is a placement context for one call: the cells reachable from
// the landing spot and the predator herds already placed.
type habitat struct {
	g      *Game
	reach  []bool
	lx, ly int
	preds  [][2]int
}

// newHabitat floods the walkable cells from the landing spot (bounded by
// the map) and notes the living predators' positions.
func (g *Game) newHabitat() *habitat {
	w, hgt := g.grid.Width(), g.grid.Height()
	lx, ly := g.landingSpot()
	h := &habitat{g: g, reach: make([]bool, w*hgt), lx: lx, ly: ly}
	if !g.grid.Walkable(lx, ly) {
		x, y, ok := g.walkableNear(lx, ly, 6)
		if !ok {
			return h
		}
		lx, ly = x, y
	}
	queue := [][2]int{{lx, ly}}
	h.reach[ly*w+lx] = true
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			x, y := c[0]+d[0], c[1]+d[1]
			if !g.grid.InBounds(x, y) || h.reach[y*w+x] || !g.grid.Walkable(x, y) {
				continue
			}
			h.reach[y*w+x] = true
			queue = append(queue, [2]int{x, y})
		}
	}
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && !a.Tame && isPredator(a.Species) {
			if p, ok := g.positions.Get(id); ok {
				h.preds = append(h.preds, [2]int{p.X, p.Y})
			}
		}
	}
	return h
}

// open reports a cell a wild animal may stand on: dry, reachable from
// the landing spot, not roofed (sealed caves) and not fogged.
func (h *habitat) open(x, y int) bool {
	g := h.g
	return g.grid.InBounds(x, y) && h.reach[y*g.grid.Width()+x] && g.dryWalkable(x, y) &&
		!g.roofAt(x, y).Roofed() && !g.fogged(x, y)
}

// forage counts the grass and plants near (x, y).
func (h *habitat) forage(x, y int) int {
	g, n := h.g, 0
	for dy := -forageRadius; dy <= forageRadius; dy++ {
		for dx := -forageRadius; dx <= forageRadius; dx++ {
			if g.hasGrass(x+dx, y+dy) {
				n++
			}
			if g.grid.InBounds(x+dx, y+dy) && g.plantAt(x+dx, y+dy) != 0 {
				n += plantForage
			}
		}
	}
	return n
}

// pick chooses a herd point for species s: ok is false when no sampled
// cell qualified.
func (h *habitat) pick(s animal.Species) (int, int, bool) { return h.pickAt(s, false) }

// pickEdge is pick restricted to map-edge cells (later arrivals walk in
// from the edge, never appear mid-map).
func (h *habitat) pickEdge(s animal.Species) (int, int, bool) { return h.pickAt(s, true) }

func (h *habitat) pickAt(s animal.Species, edge bool) (int, int, bool) {
	g := h.g
	type cand struct{ x, y, w int }
	var cs []cand
	for i := 0; i < habitatSamples; i++ {
		w, hh := g.grid.Width(), g.grid.Height()
		x, y := g.rng.IntN(w), g.rng.IntN(hh)
		if edge {
			switch g.rng.IntN(4) {
			case 0:
				y = 0
			case 1:
				y = hh - 1
			case 2:
				x = 0
			default:
				x = w - 1
			}
		}
		if chebyshev(Position{X: x, Y: y}, Position{X: h.lx, Y: h.ly}) < habitatMinDist || !h.open(x, y) {
			continue
		}
		cs = append(cs, cand{x: x, y: y})
	}
	if len(cs) == 0 {
		return 0, 0, false
	}
	if isPredator(s) {
		best, bestD := 0, -1
		for i, c := range cs {
			d := 1 << 30
			for _, p := range h.preds {
				d = min(d, chebyshev(Position{X: c.x, Y: c.y}, Position{X: p[0], Y: p[1]}))
			}
			if d > bestD {
				best, bestD = i, d
			}
		}
		h.preds = append(h.preds, [2]int{cs[best].x, cs[best].y})
		return cs[best].x, cs[best].y, true
	}
	total := 0
	for i := range cs {
		cs[i].w = 1 + h.forage(cs[i].x, cs[i].y)
		total += cs[i].w
	}
	roll := g.rng.IntN(total)
	for _, c := range cs {
		if roll -= c.w; roll < 0 {
			return c.x, c.y, true
		}
	}
	return cs[len(cs)-1].x, cs[len(cs)-1].y, true
}

// isWildSpecies: a species that may live wild (never domestic or pet).
func isWildSpecies(s animal.Species) bool {
	b := animal.RosterOf(s).Behaviour
	return b != animal.Domestic && b != animal.Pet
}
