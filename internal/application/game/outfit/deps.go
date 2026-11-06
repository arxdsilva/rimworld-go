// Package outfit is how colonists dress (PLAY-017, TEMP-4): the best
// outfit for the day, fetched piece by piece, with each changed slot
// kept as it is for Cool, and the apparel policies that decide what may
// be worn (GEAR-3, OUT-001…005).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The game
// implements the roles below with small adapters (apparel.go) and builds
// a Dresser where it needs one; the free-clothes Index is per map state
// the game keeps (never saved).
package outfit

import (
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Wardrobe is what colonists have on, and what a change of clothes does
// to the map.
type Wardrobe interface {
	Worn(id entity.EntityID) craft.Outfit
	SetWorn(id entity.EntityID, w craft.Outfit)
	// Drop sets st down at (x, y), as a garment taken off falls.
	Drop(st item.Stack, x, y int)
	// Note records a change of clothes in id's own log (putOn: put on,
	// else taken off).
	Note(id entity.EntityID, putOn bool, k item.Kind)
	// Use takes the one garment worn off stack sid (st its state).
	Use(sid entity.EntityID, st item.Stack)
	Drafted(id entity.EntityID) bool
}

// Stock is the map's item stacks.
type Stock interface {
	// ItemIDs are the map's item ids, ascending (read only).
	ItemIDs() []entity.EntityID
	// Stack is item sid (nil: gone).
	Stack(sid entity.EntityID) *item.Stack
	// Owner is who reserved item sid.
	Owner(sid entity.EntityID) (entity.EntityID, bool)
}

// Weather is the clock and the temperatures a colonist dresses for.
type Weather interface {
	Now() uint64
	// DailyMean is today's mean outdoor temperature.
	DailyMean() int
	// Comfort is id's comfort shift (cold added to the floor, heat to the
	// ceiling).
	Comfort(id entity.EntityID) (cold, heat int)
	TempAt(x, y int) int
}

// Walks is a pawn's place, its work and what it put off.
type Walks interface {
	Pos(id entity.EntityID) (x, y int)
	Postponed(id entity.EntityID, kind job.Kind, target entity.EntityID, x, y int) bool
	PutOff(id entity.EntityID, kind job.Kind, target entity.EntityID, x, y int, until uint64)
	// Allowed: (x, y) is inside id's allowed area.
	Allowed(id entity.EntityID, x, y int) bool
	Reachable(id entity.EntityID, x, y int) bool
	StartJob(id entity.EntityID, rec job.Record) bool
	Job(id entity.EntityID) job.Record
	Interrupt(id entity.EntityID)
}

// Roles are the game's sides of dressing.
type Roles struct {
	Wardrobe Wardrobe
	Stock    Stock
	Weather  Weather
	Walks    Walks
}
