package store

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Machines reads the buildings (a consumer-owned role, SOLID_GO §7).
// The id slice is the host's own, read only.
type Machines interface {
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	// ObjectAt is the object-layer building covering (x, y) (0: none).
	ObjectAt(x, y int) entity.EntityID
	// Powered reports a consumer running on power now.
	Powered(id entity.EntityID) bool
}

// Larder reads and writes the loose stacks.
type Larder interface {
	// ItemAt is the first stack lying loose on (x, y) (0: none).
	ItemAt(x, y int) entity.EntityID
	Stack(id entity.EntityID) (item.Stack, bool)
	SetStack(id entity.EntityID, st item.Stack)
	DeleteItem(id entity.EntityID)
	// Spawn lays a new stack on (x, y) and names it (0: off the map).
	Spawn(k item.Kind, qty, x, y int) entity.EntityID
	// StacksAt is how many loose stacks lie on (x, y).
	StacksAt(x, y int) int
}

// Items lists the stacks for the urgent marks.
type Items interface {
	ItemIDs() []entity.EntityID
	Stack(id entity.EntityID) (item.Stack, bool)
}

// Diners is what the dispenser job needs from the colonists.
type Diners interface {
	Position(id entity.EntityID) (x, y int)
	SetJob(id entity.EntityID, j job.Record)
	// Claim reserves target for owner; Release frees that claim.
	Claim(owner, target entity.EntityID) bool
	Release(owner, target entity.EntityID)
	Reachable(id entity.EntityID, x, y int) bool
}
