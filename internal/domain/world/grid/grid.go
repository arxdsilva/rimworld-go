package grid

// Grid is a 2D tile grid. Tiles are walkable by default.
type Grid struct {
	width, height int
	blocked       []uint64 // bit i set: tile index i is not walkable
	rev           uint64   // bumped on every walkability change
}

// New returns a Grid of the given size. Width and height must be positive.
func New(width, height int) *Grid {
	if width <= 0 || height <= 0 {
		panic("grid: width and height must be positive")
	}
	return &Grid{width: width, height: height, blocked: make([]uint64, (width*height+63)/64)}
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
	if uint(x) >= uint(g.width) || uint(y) >= uint(g.height) {
		return false
	}
	i := uint(y*g.width + x)
	return g.blocked[i>>6]&(1<<(i&63)) == 0
}

// SetWalkable marks (x, y) as walkable or blocked. It is a no-op for
// out-of-bounds coordinates.
func (g *Grid) SetWalkable(x, y int, walkable bool) {
	if !g.InBounds(x, y) {
		return
	}
	i := uint(g.index(x, y))
	w, bit := &g.blocked[i>>6], uint64(1)<<(i&63)
	if (*w&bit == 0) == walkable {
		return // unchanged
	}
	g.rev++
	*w ^= bit
}

// Version is a counter that changes whenever any tile's walkability does, so
// callers can drop reachability they cached cheaply.
func (g *Grid) Version() uint64 { return g.rev }

func (g *Grid) index(x, y int) int {
	return y*g.width + x
}

// Blocked lists the blocked cell indexes in ascending order (saved games).
func (g *Grid) Blocked() []int {
	n := 0
	for _, w := range g.blocked {
		n += popcount(w)
	}
	out := make([]int, 0, n)
	for wi, w := range g.blocked {
		for w != 0 {
			b := trailingZeros(w)
			out = append(out, wi*64+b)
			w &= w - 1
		}
	}
	return out
}

// Restore rebuilds a grid from Blocked output. Indexes outside the grid
// are ignored.
func Restore(width, height int, blocked []int) *Grid {
	g := New(width, height)
	for _, i := range blocked {
		if i >= 0 && i < width*height {
			g.blocked[i>>6] |= 1 << (uint(i) & 63)
		}
	}
	return g
}
