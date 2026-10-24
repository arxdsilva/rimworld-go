package area

// MaxAreas is how many areas a colony can have.
const MaxAreas = 10

// Area is one named area: a bitset over the map, row-major.
type Area struct {
	Name string
	Bits []uint64
}

// Set is the colony's areas over a W x H map (saved).
type Set struct {
	W, H  int
	Areas []Area
}

// New returns an empty set for a w x h map.
func New(w, h int) *Set { return &Set{W: w, H: h} }

// Add creates an area and returns its index, or -1 at MaxAreas.
func (s *Set) Add(name string) int {
	if len(s.Areas) >= MaxAreas {
		return -1
	}
	s.Areas = append(s.Areas, Area{Name: name, Bits: make([]uint64, (s.W*s.H+63)/64)})
	return len(s.Areas) - 1
}

// Rename renames area i; it reports whether i exists.
func (s *Set) Rename(i int, name string) bool {
	if i < 0 || i >= len(s.Areas) {
		return false
	}
	s.Areas[i].Name = name
	return true
}

// Paint sets (on) or clears the cells of area i in x0..x1, y0..y1
// (inclusive, any corner order, clipped to the map).
func (s *Set) Paint(i, x0, y0, x1, y1 int, on bool) {
	if i < 0 || i >= len(s.Areas) {
		return
	}
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	x0, y0 = max(x0, 0), max(y0, 0)
	x1, y1 = min(x1, s.W-1), min(y1, s.H-1)
	bits := s.Areas[i].Bits
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			n := y*s.W + x
			if on {
				bits[n/64] |= 1 << (n % 64)
			} else {
				bits[n/64] &^= 1 << (n % 64)
			}
		}
	}
}

// Contains reports whether cell (x, y) is in area i.
func (s *Set) Contains(i, x, y int) bool {
	if i < 0 || i >= len(s.Areas) || x < 0 || y < 0 || x >= s.W || y >= s.H {
		return false
	}
	n := y*s.W + x
	return s.Areas[i].Bits[n/64]&(1<<(n%64)) != 0
}

// Count is the number of cells in area i.
func (s *Set) Count(i int) int {
	if i < 0 || i >= len(s.Areas) {
		return 0
	}
	c := 0
	for _, w := range s.Areas[i].Bits {
		for ; w != 0; w &= w - 1 {
			c++
		}
	}
	return c
}

// Export is a copy of the set for a save (nil when there is none).
func (s *Set) Export() *Set {
	if s == nil || len(s.Areas) == 0 {
		return nil
	}
	out := &Set{W: s.W, H: s.H, Areas: make([]Area, len(s.Areas))}
	for i, a := range s.Areas {
		out.Areas[i] = Area{Name: a.Name, Bits: append([]uint64(nil), a.Bits...)}
	}
	return out
}

// Import restores a saved set (nil: none).
func Import(s *Set) *Set { return s.Export() }
