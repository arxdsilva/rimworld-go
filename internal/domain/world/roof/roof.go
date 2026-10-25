// Package roof holds the per-tile roof model (SIM-018): which tiles are
// roofed, by colonists or by overhead rock, which are planned for building
// or removal, and which a wall, door or rock holds up. It is headless and
// deterministic; the game owns the grid and the jobs that change it.
package roof

// Cell is the roof state of one tile, a set of flags.
type Cell uint8

const (
	// Built: a colonist-built roof.
	Built Cell = 1 << iota
	// Natural: overhead rock left by mining. It never collapses and
	// cannot be removed.
	Natural
	// PlanBuild: a roof is planned here (by the player or auto-roofing).
	PlanBuild
	// PlanRemove: the built roof here is planned for removal.
	PlanRemove
	// Supported: a support (wall, door or rock) lies within SupportRange.
	// Derived on every room rebuild; never saved as authoritative.
	Supported
	// NoAuto: the player removed this roof; auto-roofing leaves it off.
	NoAuto
	// NaturalThick (with Natural): deep overhead rock from map generation
	// (MAPGEN-2). Nothing drops through it and no tool removes it.
	NaturalThick
)

const (
	// SupportRange is how far (Chebyshev tiles) a roof reaches from the
	// nearest support.
	SupportRange = 6
	// AutoMaxCells is the largest room colonists roof on their own.
	AutoMaxCells = 150
	// BuildWork and RemoveWork are the work ticks per tile.
	BuildWork  = 30
	RemoveWork = 15
)

// Roofed reports any roof, built or natural.
func (c Cell) Roofed() bool { return c&(Built|Natural) != 0 }

// Thick reports deep natural rock overhead.
func (c Cell) Thick() bool { return c.Has(Natural | NaturalThick) }

// Has reports every flag of f set.
func (c Cell) Has(f Cell) bool { return c&f == f }

// SupportField reports, per tile (row-major, w wide), whether a tile with
// support(x, y) lies within SupportRange (Chebyshev distance), by a
// multi-source breadth-first search from every support.
func SupportField(w, h int, support func(x, y int) bool) []bool {
	out := make([]bool, w*h)
	if w <= 0 || h <= 0 {
		return out
	}
	// dist is the BFS depth + 1 (0 = unvisited); the queue is seeded with
	// every support in row-major order, so the result is deterministic.
	dist := make([]uint8, w*h)
	queue := make([]int32, 0, 64)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if support(x, y) {
				i := y*w + x
				dist[i] = 1
				out[i] = true
				queue = append(queue, int32(i))
			}
		}
	}
	for head := 0; head < len(queue); head++ {
		i := int(queue[head])
		d := dist[i]
		if int(d) > SupportRange {
			continue // depth SupportRange reached: no further
		}
		x, y := i%w, i/w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := x+dx, y+dy
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				j := ny*w + nx
				if dist[j] != 0 {
					continue
				}
				dist[j] = d + 1
				out[j] = true
				queue = append(queue, int32(j))
			}
		}
	}
	return out
}
