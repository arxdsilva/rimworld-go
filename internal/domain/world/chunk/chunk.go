package chunk

import "math/bits"

// Size is a chunk's side in tiles; Shift is log2(Size).
const (
	Size  = 16
	Shift = 4
)

// Grid is the chunk layout of a W×H tile map: CW×CH chunks, row-major.
// Edge chunks are clipped to the map.
type Grid struct {
	W, H   int // tiles
	CW, CH int // chunks
}

// New lays out chunks over a w×h map.
func New(w, h int) Grid {
	return Grid{W: w, H: h, CW: (w + Size - 1) >> Shift, CH: (h + Size - 1) >> Shift}
}

// Count is the number of chunks.
func (g Grid) Count() int { return g.CW * g.CH }

// Of is the chunk holding tile (x, y), which must be on the map.
func (g Grid) Of(x, y int) int { return (y>>Shift)*g.CW + x>>Shift }

// OfIndex is the chunk holding row-major tile index i.
func (g Grid) OfIndex(i int) int { return g.Of(i%g.W, i/g.W) }

// Bounds is chunk c's tile rectangle [x0, x1) × [y0, y1).
func (g Grid) Bounds(c int) (x0, y0, x1, y1 int) {
	x0, y0 = (c%g.CW)<<Shift, (c/g.CW)<<Shift
	return x0, y0, min(x0+Size, g.W), min(y0+Size, g.H)
}

// Left and Up are the chunks west and north of c, or -1 at the map edge.
func (g Grid) Left(c int) int {
	if c%g.CW == 0 {
		return -1
	}
	return c - 1
}

// Up is the chunk north of c, or -1 on the top row.
func (g Grid) Up(c int) int {
	if c < g.CW {
		return -1
	}
	return c - g.CW
}

// Set is a bitset of chunk indexes.
type Set []uint64

// NewSet holds n chunks.
func NewSet(n int) Set { return make(Set, (n+63)/64) }

// Add marks chunk c (ignored when negative or off the set).
func (s Set) Add(c int) {
	if c >= 0 && c>>6 < len(s) {
		s[c>>6] |= 1 << (uint(c) & 63)
	}
}

// Has reports chunk c.
func (s Set) Has(c int) bool {
	return c >= 0 && c>>6 < len(s) && s[c>>6]&(1<<(uint(c)&63)) != 0
}

// Any reports a marked chunk.
func (s Set) Any() bool {
	for _, w := range s {
		if w != 0 {
			return true
		}
	}
	return false
}

// Clear unmarks every chunk.
func (s Set) Clear() { clear(s) }

// Each calls fn for every marked chunk in ascending index order.
func (s Set) Each(fn func(c int)) {
	for wi, w := range s {
		for w != 0 {
			fn(wi<<6 + bits.TrailingZeros64(w))
			w &= w - 1
		}
	}
}
