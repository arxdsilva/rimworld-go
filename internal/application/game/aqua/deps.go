package aqua

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// Host is everything the tanks need from the simulation: value-only roles
// the game implements with one small adapter (game/fuel.go).
type Host interface {
	World
	Site
	Handlers
	Hands
}

// World is the clock and the player's messages.
type World interface {
	Now() uint64
	// Message posts a quiet message at (x, y).
	Message(x, y int, text string)
}

// Site reads the tank buildings and the world around them.
type Site interface {
	// Tank is fish tank id's tile (ok false: no such tank on this map).
	Tank(id entity.EntityID) (x, y int, ok bool)
	// Workable reports a tank handlers may go to (not marked for removal,
	// forbidden or an enemy's).
	Workable(id entity.EntityID) bool
	Powered(id entity.EntityID) bool
	// Air is the temperature at (x, y).
	Air(x, y int) temperature.Temp
	// Feed is the ticks of feed tank id holds.
	Feed(id entity.EntityID) int
	// DropFish spawns qty fish, allowed, at or beside (x, y).
	DropFish(x, y, qty int)
}

// Handlers reads the colony's handlers.
type Handlers interface {
	// Animals is colonist id's Animals skill level.
	Animals(id entity.EntityID) int
	// BestAnimals is the best Animals level in the colony.
	BestAnimals() int
	// Pos is a creature's tile.
	Pos(id entity.EntityID) (x, y int, ok bool)
	// Work is the Animals work id does this tick (0: none).
	Work(id entity.EntityID) int
}

// Hands is the handler's job and its claims.
type Hands interface {
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, j job.Record)
	// Owner is who holds a claim on target.
	Owner(target entity.EntityID) (entity.EntityID, bool)
	// BesideTank is a free tile beside tank's footprint that id can reach.
	BesideTank(id, tank entity.EntityID) (int, int, bool)
	// Postponed reports work id has put off (retry cooldown), PutOff puts it
	// off as out of reach.
	Postponed(id entity.EntityID, kind job.Kind, tank entity.EntityID) bool
	PutOff(id entity.EntityID, kind job.Kind, tank entity.EntityID)
}
