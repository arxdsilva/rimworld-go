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

// Paint sets (on) or clears the cells of area i in x0..x1, y0..y1.
func (s *Set) Paint(i, x0, y0, x1, y1 int, on bool) {
	// TODO(M9)
}

// Contains reports whether cell (x, y) is in area i.
func (s *Set) Contains(i, x, y int) bool {
	return false // TODO(M9)
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
