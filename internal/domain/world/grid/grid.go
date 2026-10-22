package grid

import "sort"

// Grid is a 2D tile grid. Tiles are walkable by default.
type Grid struct {
	width, height int
	blocked       map[int]bool // index -> not walkable
	rev           uint64       // bumped on every walkability change
}

// New returns a Grid of the given size. Width and height must be positive.
func New(width, height int) *Grid {
	if width <= 0 || height <= 0 {
		panic("grid: width and height must be positive")
	}
	return &Grid{width: width, height: height, blocked: make(map[int]bool)}
}

// Width returns the grid width in tiles.
func (g *Grid) Width() int { return g.width }

// Height returns the grid height in tiles.
func (g *Grid) Height() int { return g.height }

// InBounds reports whether (x, y) is within the grid.
func (g *Grid) InBounds(x, y int) bool {
	return x >= 0 && x < g.width && y >= 0 && y < g.height
}

// Walkable reports whether (x, y) can be entered. Out-of-bounds tiles are
// never walkable.
func (g *Grid) Walkable(x, y int) bool {
	if !g.InBounds(x, y) {
		return false
	}
	return !g.blocked[g.index(x, y)]
}

// SetWalkable marks (x, y) as walkable or blocked. It is a no-op for
// out-of-bounds coordinates.
func (g *Grid) SetWalkable(x, y int, walkable bool) {
	if !g.InBounds(x, y) {
		return
	}
	i := g.index(x, y)
	if g.blocked[i] != walkable {
		return // unchanged
	}
	g.rev++
	if walkable {
		delete(g.blocked, i)
	} else {
		g.blocked[i] = true
	}
}

// Version is a counter that changes whenever any tile's walkability does, so
// callers can drop reachability they cached cheaply.
func (g *Grid) Version() uint64 { return g.rev }

func (g *Grid) index(x, y int) int {
	return y*g.width + x
}

// Blocked lists the blocked cell indexes in ascending order (saved games).
func (g *Grid) Blocked() []int {
	out := make([]int, 0, len(g.blocked))
	for i, b := range g.blocked {
		if b {
			out = append(out, i)
		}
	}
	sort.Ints(out)
	return out
}

// Restore rebuilds a grid from Blocked output.
func Restore(width, height int, blocked []int) *Grid {
	g := New(width, height)
	for _, i := range blocked {
		g.blocked[i] = true
	}
	return g
}
