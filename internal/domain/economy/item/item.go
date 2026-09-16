// Package item is ground (and carried) resource stacks.
package item

import "rimworld-go/internal/domain/sim/entity"

// Kind is a stack category. Only food and wood exist in this slice.
type Kind uint8

const (
	Food Kind = iota
	Wood
)

func (k Kind) String() string {
	if k == Wood {
		return "wood"
	}
	return "food"
}

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
