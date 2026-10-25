package area

// MaxAreas is how many areas a colony can have.
const MaxAreas = 10

// Area is one named area: a bitset over the map, row-major. ID is stable
// for the colony's lifetime (never reused); Color is its overlay tint.
type Area struct {
	Name  string
	Bits  []uint64
	ID    uint16
	Color [4]uint8
}

// Set is the colony's areas over a W x H map (saved).
type Set struct {
	W, H  int
	Areas []Area
	// LastID is the highest id handed out (PLAY-015); ids start at 1.
	LastID uint16
}

// palette is the areas' overlay colours (our own), cycled by id.
var palette = [MaxAreas][4]uint8{
	{92, 168, 120, 255}, {196, 98, 104, 255}, {96, 132, 204, 255}, {214, 170, 74, 255}, {160, 104, 196, 255},
	{78, 178, 186, 255}, {206, 128, 70, 255}, {140, 170, 80, 255}, {198, 108, 164, 255}, {120, 120, 150, 255},
}

// ColorOf is the overlay colour of the area with id.
func ColorOf(id uint16) [4]uint8 {
	if id == 0 {
		return palette[0]
	}
	return palette[int(id-1)%len(palette)]
}

// New returns an empty set for a w x h map.
func New(w, h int) *Set { return &Set{W: w, H: h} }

// Add creates an area and returns its index, or -1 at MaxAreas.
func (s *Set) Add(name string) int {
	if len(s.Areas) >= MaxAreas {
		return -1
	}
	s.ensureIDs()
	s.LastID++
	s.Areas = append(s.Areas, Area{Name: name, Bits: make([]uint64, (s.W*s.H+63)/64), ID: s.LastID, Color: ColorOf(s.LastID)})
	return len(s.Areas) - 1
}

// ensureIDs gives a set saved before ids (PLAY-015) ids 1..n in order.
func (s *Set) ensureIDs() {
	for i := range s.Areas {
		if s.Areas[i].ID == 0 {
			s.LastID++
			s.Areas[i].ID = s.LastID
			s.Areas[i].Color = ColorOf(s.LastID)
		}
	}
}

// Index is the position of the area with id, or -1.
func (s *Set) Index(id uint16) int {
	if s == nil || id == 0 {
		return -1
	}
	for i, a := range s.Areas {
		if a.ID == id {
			return i
		}
	}
	return -1
}

// Delete removes the area with id; it reports whether it existed. Later
// areas keep their ids (positions shift down).
func (s *Set) Delete(id uint16) bool {
	i := s.Index(id)
	if i < 0 {
		return false
	}
	s.Areas = append(s.Areas[:i], s.Areas[i+1:]...)
	return true
}

// Invert swaps area i's member and non-member cells across the map; the
// padding bits past the last cell stay clear.
func (s *Set) Invert(i int) bool {
	if i < 0 || i >= len(s.Areas) {
		return false
	}
	bits, n := s.Areas[i].Bits, s.W*s.H
	for k := range bits {
		bits[k] = ^bits[k]
	}
	if r := n % 64; r != 0 && len(bits) > 0 {
		bits[len(bits)-1] &= 1<<r - 1
	}
	return true
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
	out := &Set{W: s.W, H: s.H, Areas: make([]Area, len(s.Areas)), LastID: s.LastID}
	for i, a := range s.Areas {
		out.Areas[i] = Area{Name: a.Name, Bits: append([]uint64(nil), a.Bits...), ID: a.ID, Color: a.Color}
	}
	return out
}

// Import restores a saved set (nil: none); a set saved before ids gets
// ids 1..n in order.
func Import(s *Set) *Set {
	out := s.Export()
	if out != nil {
		out.ensureIDs()
	}
	return out
}
