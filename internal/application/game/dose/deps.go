// Package dose is drug use (ENG-003, M2; MED-07): a colonist's drug state and
// policy, the hourly wearing off, the search for a dose, and what a dose
// does. The states stay on the game (saved as Drugs, in save.go's types, so
// the save's type names stay put); this package holds the behaviour, over
// the roles below (.cursor/SOLID_GO.md §10). The game implements them with
// small adapters (dose_host.go) and builds the component where it needs it.
package dose

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is the roles, one field each: a struct of them and not one big
// interface, so that building the component allocates nothing per tick.
type Host struct {
	Pawns  Pawns
	Minds  Minds
	Bodies Bodies
	Stock  Stock
}

// Pawns is who a pawn is and what it is doing.
type Pawns interface {
	Now() uint64
	Colonist(id entity.EntityID) bool
	Pos(id entity.EntityID) (x, y int)
	Job(id entity.EntityID) (job.Record, bool)
	SetJob(id entity.EntityID, j job.Record)
	StartJob(id entity.EntityID, rec job.Record) bool
	Interrupt(id entity.EntityID, why job.FailReason)
}

// Minds is the recreation, rest, traits, memories and log a dose touches.
type Minds interface {
	Joy(id entity.EntityID) int
	SetJoy(id entity.EntityID, joy int)
	Needs(id entity.EntityID) (needs.Pair, bool)
	SetNeeds(id entity.EntityID, n needs.Pair)
	// Traits is id's traits (nil for a legacy colonist).
	Traits(id entity.EntityID) []trait.ID
	Remember(id entity.EntityID, k mood.Kind)
	LogIngested(id entity.EntityID, name string)
	// DrugRolls is the drug stream's draw, taken before each dose.
	DrugRolls() func(int) int
}

// Bodies is a pawn's body, and how its changes reach the rest of the game.
type Bodies interface {
	Body(id entity.EntityID) *body.Body
	Vital(id entity.EntityID) (health.Record, bool)
	EnsureBody(id entity.EntityID, v health.Record) *body.Body
	// Settle has the stats and the downing rule follow a changed body.
	Settle(id entity.EntityID, b *body.Body, v health.Record)
	// DieOfOverdose tells the colony and kills id.
	DieOfOverdose(id entity.EntityID)
	// Vomit soils (x, y) as a disease's vomit does.
	Vomit(x, y int)
	// MedicalRoll draws from the medical stream.
	MedicalRoll(n int) int
}

// Stock is the items on the map.
type Stock interface {
	ItemIDs() []entity.EntityID
	Stack(id entity.EntityID) (item.Stack, bool)
	PutStack(id entity.EntityID, st item.Stack)
	DeleteItem(id entity.EntityID)
	// EntityOwner is who has reserved id.
	EntityOwner(id entity.EntityID) (entity.EntityID, bool)
}
