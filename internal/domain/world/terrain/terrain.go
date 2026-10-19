package terrain

// Type is a ground type. Rock is natural rock wall: impassable until mined.
type Type uint8

const (
	Soil Type = iota
	RichSoil
	Gravel
	Stone // rough stone ground
	Rock  // natural rock wall
	numTypes
)

var names = [numTypes]string{"soil", "rich_soil", "gravel", "stone", "rock"}

func (t Type) String() string {
	if t >= numTypes {
		return "unknown"
	}
	return names[t]
}

// Walkable reports whether pawns can stand on t.
func (t Type) Walkable() bool { return t != Rock }

// Fertility is the growth-rate multiplier in percent (0 = nothing grows).
func (t Type) Fertility() int {
	switch t {
	case Soil:
		return 100
	case RichSoil:
		return 140
	case Gravel:
		return 70
	default:
		return 0
	}
}

// Names lists the type names in Type order (read models index by code).
func Names() []string { return names[:] }

// Map is a dense terrain grid, row-major.
type Map struct {
	W, H  int
	Cells []Type
}

// NewMap returns a w x h map of soil.
func NewMap(w, h int) Map {
	return Map{W: w, H: h, Cells: make([]Type, w*h)}
}

// At returns the type at (x, y); out-of-bounds reads as Rock.
func (m Map) At(x, y int) Type {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return Rock
	}
	return m.Cells[y*m.W+x]
}

// Set writes the type at (x, y) if in bounds.
func (m Map) Set(x, y int, t Type) {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return
	}
	m.Cells[y*m.W+x] = t
}
