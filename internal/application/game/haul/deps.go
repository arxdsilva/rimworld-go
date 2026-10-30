package haul

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is what gathering needs from the simulation (a consumer-owned
// role, .cursor/SOLID_GO.md §7): stack reads and writes, a path for the
// hauler and the release of a pile's claim. Values only: a call on the
// tick path moves nothing to the heap.
type Host interface {
	Stack(id entity.EntityID) (item.Stack, bool)
	SetStack(id entity.EntityID, st item.Stack)
	DeleteItem(id entity.EntityID)
	// PathTo is pawn id's path to (x, y); an empty path with ok true
	// means it is there already.
	PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool)
	// Release frees owner's claim on target.
	Release(owner, target entity.EntityID)
}

// Claimer claims target for owner (the game's reservation table).
type Claimer interface {
	TryEntity(owner, target entity.EntityID) bool
}
