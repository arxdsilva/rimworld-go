// Package geology is what natural rock is made of: the stone type of each
// rock or stone-ground cell and the ore veins inside rock. It is a pure,
// deterministic layer generated after terrain; it never touches the sim RNG.
package geology

// Stone is a natural stone type. Append only: saved.
type Stone uint8

const (
	None Stone = iota
	Sandstone
	Granite
	Limestone
	Slate
	Marble
	numStones
)

var stoneNames = [numStones]string{"none", "sandstone", "granite", "limestone", "slate", "marble"}
var stoneLabels = [numStones]string{"", "Sandstone", "Granite", "Limestone", "Slate", "Marble"}

func (s Stone) String() string {
	if s >= numStones {
		return "none"
	}
	return stoneNames[s]
}

// Label is the player-facing name ("Granite").
func (s Stone) Label() string {
	if s >= numStones {
		return ""
	}
	return stoneLabels[s]
}

// ParseStone maps a name back to its stone.
func ParseStone(name string) (Stone, bool) {
	for i := Sandstone; i < numStones; i++ {
		if stoneNames[i] == name {
			return i, true
		}
	}
	return None, false
}

// Stones lists the five real stones in enum order.
func Stones() []Stone { return []Stone{Sandstone, Granite, Limestone, Slate, Marble} }

// Ore is a resource vein inside rock. Append only: saved.
type Ore uint8

const (
	NoOre Ore = iota
	Steel
	Silver
	Gold
	// Machinery is buried machinery: mined for components (NG-1 §4.8).
	Machinery
	numOres
)

var oreNames = [numOres]string{"", "steel", "silver", "gold", "machinery"}
var oreLabels = [numOres]string{"", "Compacted steel", "Silver ore", "Gold ore", "Buried machinery"}

func (o Ore) String() string {
	if o >= numOres {
		return ""
	}
	return oreNames[o]
}

// Label is the player-facing name ("Compacted steel").
func (o Ore) Label() string {
	if o >= numOres {
		return ""
	}
	return oreLabels[o]
}

// Ores lists the real ores in enum order.
func Ores() []Ore { return []Ore{Steel, Silver, Gold} }

// Map is the per-cell geology, row-major. Fields are exported for saves.
type Map struct {
	W, H  int
	Stone []Stone
	Ore   []Ore
}

// NewMap is an empty w×h map.
func NewMap(w, h int) Map {
	return Map{W: w, H: h, Stone: make([]Stone, w*h), Ore: make([]Ore, w*h)}
}

func (m Map) in(x, y int) bool {
	return x >= 0 && y >= 0 && x < m.W && y < m.H && len(m.Stone) == m.W*m.H
}

// At is the stone at (x, y); None out of bounds.
func (m Map) At(x, y int) Stone {
	if !m.in(x, y) {
		return None
	}
	return m.Stone[y*m.W+x]
}

// OreAt is the ore at (x, y); NoOre out of bounds.
func (m Map) OreAt(x, y int) Ore {
	if !m.in(x, y) || len(m.Ore) != len(m.Stone) {
		return NoOre
	}
	return m.Ore[y*m.W+x]
}

// Set sets the stone at (x, y); out of bounds is ignored.
func (m *Map) Set(x, y int, s Stone) {
	if m.in(x, y) {
		m.Stone[y*m.W+x] = s
	}
}

// SetOre sets the ore at (x, y); out of bounds is ignored.
func (m *Map) SetOre(x, y int, o Ore) {
	if m.in(x, y) && len(m.Ore) == len(m.Stone) {
		m.Ore[y*m.W+x] = o
	}
}

// Salt keeps the geology RNG stream (randx.New(seed^Salt)) apart from the
// simulation's.
const Salt uint64 = 0x9E3779B97F4A7C15
