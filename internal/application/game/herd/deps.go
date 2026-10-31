package herd

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything the herd needs from the simulation: value-only
// roles the game implements with small adapters (game/handling.go).
type Host interface {
	World
	Beasts
	Hands
	Larder
	Events
}

// World is the clock, the dice, messages and the colonists' names and
// skills.
type World interface {
	Now() uint64
	// Roll is a uniform draw in [0, n) from the simulation's stream.
	Roll(n int) int
	// Message posts a quiet message at (x, y).
	Message(x, y int, text string)
	// Name is colonist id's name.
	Name(id entity.EntityID) string
	// Animals is colonist id's Animals skill level.
	Animals(id entity.EntityID) int
}

// Beasts reads and writes the animals.
type Beasts interface {
	// AnimalIDs is every animal, ascending.
	AnimalIDs() []entity.EntityID
	Animal(id entity.EntityID) (animal.State, bool)
	SetAnimal(id entity.EntityID, a animal.State)
	// Pos is a creature's tile (ok false: none).
	Pos(id entity.EntityID) (x, y int, ok bool)
	// AnyAnimal reports an animal keep accepts (the job index first: no
	// scan when nothing can match).
	AnyAnimal(keep func(entity.EntityID, *animal.State) bool) bool
	// Nearest is the animal keep accepts nearest id, and its tile.
	Nearest(id entity.EntityID, keep func(entity.EntityID, *animal.State) bool) (entity.EntityID, int, int, bool)
	// Kill kills animal aid, by colonist by.
	Kill(aid, by entity.EntityID)
}

// Hands is the handler's job and its claims.
type Hands interface {
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, j job.Record)
	Job(id entity.EntityID) (job.Record, bool)
	// Owner is who holds a claim on target.
	Owner(target entity.EntityID) (entity.EntityID, bool)
	// Claim holds target for id (false: someone else holds it).
	Claim(id, target entity.EntityID) bool
	// BesideSpot is a free tile beside (x, y) for id.
	BesideSpot(id entity.EntityID, x, y int) (int, int, bool)
	PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool)
	// Work is the Animals work id does this tick (0: none).
	Work(id entity.EntityID) int
}

// Larder is the food and items a handler carries and makes.
type Larder interface {
	// NearestFood is the nearest free food stack accept takes for id
	// (0: none), and its tile.
	NearestFood(id entity.EntityID, accept func(item.Kind) bool) (entity.EntityID, int, int)
	// Take: id, at stack sid, picks up qty of it (split off a bigger
	// stack); false when the stack is gone or held.
	Take(id, sid entity.EntityID, qty int) (entity.EntityID, bool)
	Stack(sid entity.EntityID) (item.Stack, bool)
	Delete(sid entity.EntityID)
	// Drop spawns st at its tile, onto any stack it merges with.
	Drop(st item.Stack)
	// HaulToStorage starts id carrying stack sid to storage.
	HaulToStorage(id, sid entity.EntityID) bool
	// Policy is what tame animal aid's food policy lets it eat (FOOD-4).
	Policy(aid entity.EntityID) func(item.Kind) bool
}

// Events are the game's side of an outcome: names, logs, bonds, rage.
type Events interface {
	// Tamed: handler id tamed animal aid (the animal is made tame, named,
	// logged and may bond).
	Tamed(id, aid entity.EntityID)
	// TameFailed: id failed to tame aid (logged; aid waits out its
	// retry, and may go manhunter at its revenge chance).
	TameFailed(id, aid entity.EntityID)
	// FedAnimal logs that id fed animal aid.
	FedAnimal(id, aid entity.EntityID)
}
