// Package item is ground (and carried) resource stacks.
package item

import "rimworld-go/internal/domain/sim/entity"

// Kind is a stack category.
type Kind uint8

const (
	Food Kind = iota // packaged survival ration
	Wood
	Stone     // stone blocks
	Berries   // raw, foraged
	Amberroot // raw, farmed
	Meat      // raw, hunted
	Meal      // cooked
	Club      // melee weapon
	Rifle     // ranged weapon
	numKinds
)

var kindNames = [numKinds]string{"food", "wood", "stone", "berries", "amberroot", "meat", "meal", "club", "rifle"}

// IsWeapon reports whether k can be equipped.
func (k Kind) IsWeapon() bool { return k == Club || k == Rifle }

// nutrition is hunger restored per unit eaten (needs.Full = 10000).
var nutrition = [numKinds]int{Food: 4000, Berries: 1500, Amberroot: 2000, Meat: 2200, Meal: 6000}

// preference ranks food choice: higher is eaten first.
var preference = [numKinds]int{Meal: 4, Food: 3, Amberroot: 2, Berries: 2, Meat: 1}

// Nutrition is the hunger restored by eating one unit of k (0 if inedible).
func (k Kind) Nutrition() int {
	if k >= numKinds {
		return 0
	}
	return nutrition[k]
}

// Preference ranks edible kinds; higher is chosen first.
func (k Kind) Preference() int {
	if k >= numKinds {
		return 0
	}
	return preference[k]
}

// Raw reports uncooked food (eating it is unpleasant).
func (k Kind) Raw() bool { return k == Berries || k == Amberroot || k == Meat }

// Foods lists every edible kind.
func Foods() []Kind { return []Kind{Food, Berries, Amberroot, Meat, Meal} }

func (k Kind) String() string {
	if k >= numKinds {
		return "unknown"
	}
	return kindNames[k]
}

// ParseKind returns the kind for a name.
func ParseKind(name string) (Kind, bool) {
	for i, n := range kindNames {
		if n == name {
			return Kind(i), true
		}
	}
	return 0, false
}

// IsFood reports whether k can be eaten.
func (k Kind) IsFood() bool { return k.Nutrition() > 0 }

// IsStuff reports whether k is a building material.
func (k Kind) IsStuff() bool { return k == Wood || k == Stone }

const StackLimit = 75

// Stack is a positive quantity of one kind at one location model.
type Stack struct {
	Kind    Kind
	Qty     int
	X, Y    int
	Carrier entity.EntityID // 0 = on ground or in a frame
}

// CanMerge reports whether a and b may combine on the same tile.
func CanMerge(a, b Stack) bool {
	return a.Kind == b.Kind && a.Carrier == 0 && b.Carrier == 0 && a.X == b.X && a.Y == b.Y
}

// Add returns qty after adding n, capped, and whether the result is still positive.
func Add(qty, n int) int {
	v := qty + n
	if v < 0 {
		return 0
	}
	if v > StackLimit {
		return StackLimit
	}
	return v
}

// Take removes n from qty. ok is false if qty would go negative.
func Take(qty, n int) (int, bool) {
	if n <= 0 || qty < n {
		return qty, false
	}
	return qty - n, true
}
