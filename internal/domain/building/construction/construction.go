// Package construction is buildable definitions, frames (blueprints that
// accept wood then work) and completed buildings.
package construction

import "rimworld-go/internal/domain/economy/item"

// Kind is a buildable structure.
type Kind uint8

const (
	Wall Kind = iota
	Bed
	Door
	Floor
	Table
	Chair
	Lamp
	kindCount
)

// Layer separates what can share a tile: one floor plus one object.
type Layer uint8

const (
	LayerObject Layer = iota
	LayerFloor
)

// Def is the static definition of a buildable kind.
type Def struct {
	Kind       Kind
	Name       string
	Wood       int // delivered wood required before work starts
	Work       int // construct ticks after materials are in place
	Layer      Layer
	BlocksMove bool // completed building makes its tile unwalkable
	Rotatable  bool
	StoneOK    bool // may be built from stone blocks instead of wood
}

// StoneWorkPct is the work multiplier (percent) for stone construction.
const StoneWorkPct = 160

var defs = [kindCount]Def{
	Wall:  {Kind: Wall, Name: "wall", Wood: 2, Work: 40, Layer: LayerObject, BlocksMove: true, StoneOK: true},
	Bed:   {Kind: Bed, Name: "bed", Wood: 3, Work: 60, Layer: LayerObject, Rotatable: true},
	Door:  {Kind: Door, Name: "door", Wood: 2, Work: 50, Layer: LayerObject},
	Floor: {Kind: Floor, Name: "floor", Wood: 1, Work: 20, Layer: LayerFloor, StoneOK: true},
	Table: {Kind: Table, Name: "table", Wood: 3, Work: 50, Layer: LayerObject},
	Chair: {Kind: Chair, Name: "chair", Wood: 2, Work: 40, Layer: LayerObject, Rotatable: true},
	Lamp:  {Kind: Lamp, Name: "lamp", Wood: 2, Work: 40, Layer: LayerObject},
}

// DefOf returns the definition for k.
func DefOf(k Kind) Def {
	if k >= kindCount {
		return Def{}
	}
	return defs[k]
}

// All lists every buildable definition in Kind order.
func All() []Def {
	out := make([]Def, 0, kindCount)
	for _, d := range defs {
		out = append(out, d)
	}
	return out
}

// Parse returns the Kind for a definition name.
func Parse(name string) (Kind, bool) {
	for _, d := range defs {
		if d.Name == name {
			return d.Kind, true
		}
	}
	return 0, false
}

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool { return k < kindCount }

func (k Kind) String() string {
	if k >= kindCount {
		return "unknown"
	}
	return defs[k].Name
}

// WoodCost is delivered wood required before construction work.
func WoodCost(k Kind) int { return DefOf(k).Wood }

// WorkTicks is Do-phase labor after materials are delivered.
func WorkTicks(k Kind) int { return DefOf(k).Work }

// DeconstructTicks is the labor to take a building down.
func DeconstructTicks(k Kind) int { return (DefOf(k).Work + 1) / 2 }

// Refund is the material returned by deconstruction (half, rounded down).
func Refund(k Kind) int { return DefOf(k).Wood / 2 }

// StuffOK reports whether k may be built from material s.
func StuffOK(k Kind, s item.Kind) bool {
	switch s {
	case item.Wood:
		return true
	case item.Stone:
		return DefOf(k).StoneOK
	}
	return false
}

// WorkFor is the construct work for k built from s.
func WorkFor(k Kind, s item.Kind) int {
	w := DefOf(k).Work
	if s == item.Stone {
		w = w * StoneWorkPct / 100
	}
	return w
}

// Facing is the orientation of a rotatable building.
type Facing uint8

const (
	South Facing = iota
	East
	North
	West
)

func (f Facing) String() string {
	switch f {
	case East:
		return "east"
	case North:
		return "north"
	case West:
		return "west"
	default:
		return "south"
	}
}

// ParseFacing maps a raw command value to a Facing (unknown -> South).
func ParseFacing(v uint8) Facing {
	if v > uint8(West) {
		return South
	}
	return Facing(v)
}

// Frame is a blueprint that accepts wood then construct work.
type Frame struct {
	Kind     Kind
	Stuff    item.Kind
	Facing   Facing
	X, Y     int
	Wood     int
	Need     int
	WorkLeft int
}

// Ready reports materials are in place.
func (f Frame) Ready() bool { return f.Wood >= f.Need }

// Building is a completed structure.
type Building struct {
	Kind        Kind
	Stuff       item.Kind
	Facing      Facing
	X, Y        int
	Claimant    uint64 // EntityID of the pawn who claimed a bed; 0 if free
	Deconstruct bool   // designated for removal
	WorkLeft    int    // remaining deconstruct work once started
}
