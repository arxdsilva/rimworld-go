// Package construction is wall/bed frames and completed buildings.
package construction

// Kind is a buildable structure.
type Kind uint8

const (
	Wall Kind = iota
	Bed
)

func (k Kind) String() string {
	if k == Bed {
		return "bed"
	}
	return "wall"
}

// WoodCost is delivered wood required before construction work.
func WoodCost(k Kind) int {
	if k == Bed {
		return 3
	}
	return 2
}

// WorkTicks is Do-phase labor after materials are delivered.
func WorkTicks(k Kind) int {
	if k == Bed {
		return 60
	}
	return 40
}

// Frame is a blueprint that accepts wood then construct work.
type Frame struct {
	Kind     Kind
	X, Y     int
	Wood     int
	Need     int
	WorkLeft int
}

// Ready reports materials are in place.
func (f Frame) Ready() bool { return f.Wood >= f.Need }

// Building is a completed wall or bed.
type Building struct {
	Kind     Kind
	X, Y     int
	Claimant uint64 // EntityID of the pawn who claimed a bed; 0 if free
}
