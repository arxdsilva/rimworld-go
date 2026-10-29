package craft

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything craft needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10). The game package implements each
// role with a small adapter type (craft_host.go). Every method takes and
// returns values: never a pointer to a caller's local, never a closure,
// so a host call on the tick path moves nothing to the heap.
type Host interface {
	Benches
	Stock
	Output
	Workers
	Crafters
}

// Benches reads the buildings benches, racks and cabinets are, and what
// they stand in. The id slice is the host's own, read only.
type Benches interface {
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	// Bench is building id when it is a completed bench (ok false for any
	// other building, copied only when it is one).
	Bench(id entity.EntityID) (construction.Building, bool)
	// ObjectAt is the object-layer building covering (x, y) (0: none).
	ObjectAt(x, y int) entity.EntityID
	// StructVer moves on every building or terrain change.
	StructVer() uint64
	Powered(id entity.EntityID) bool
	// Fuel is the fuel a fueled building holds (fuel.go: lamps and
	// generators share it).
	Fuel(id entity.EntityID) int
	// TemperatureAt is the air temperature at (x, y), whole degrees.
	TemperatureAt(x, y int) int
}

// Stock reads the colony's items and picks ingredients up.
type Stock interface {
	// Stock is the colony's count of kind k.
	Stock(k item.Kind) int
	// DriedStock is the colony's count of dried food.
	DriedStock() int
	// FreeWood is the wood lying free (not carried, not forbidden).
	FreeWood() int
	// Ingredient is the nearest free stack id may take that feeds in on
	// its own (0 when none; id 0 asks for any).
	Ingredient(id entity.EntityID, in recipe.Input) (entity.EntityID, item.Stack)
	Stack(id entity.EntityID) (item.Stack, bool)
	// Carry has carrier pick take of stack up: the whole stack, or a new
	// stack split off it. It returns the carried stack's id.
	Carry(carrier, stack entity.EntityID, take int) entity.EntityID
	// ItemIDs is every item stack's id (the host's slice, read only): the
	// multi-stack ingredient search (PRD-1).
	ItemIDs() []entity.EntityID
	// Merge adds carried stack from to carried stack into (same kind)
	// and deletes from: one load gathered from several stacks.
	Merge(into, from entity.EntityID)
}

// Output is what a finished bill or batch leaves behind.
type Output interface {
	DeleteItem(id entity.EntityID)
	// DropStack spawns qty of k at (x, y), merging with a stack there.
	DropStack(k item.Kind, qty, x, y int)
	// DropStackOf spawns a copy of st (rot, dried, claim) at (x, y).
	DropStackOf(st item.Stack, x, y int)
	// DropMeal drops qty meals cooked by cook at a bench of kind bench,
	// with the cook's poison chance rolled by the host.
	DropMeal(cook entity.EntityID, bench construction.Kind, k item.Kind, qty, x, y int)
	// Cremate burns body on pyre with the recipe's wood.
	Cremate(pyre, body entity.EntityID, d recipe.Def)
	// Message posts a plain message at (x, y).
	Message(x, y int, text string)
	// PutDown sets carried stack id down at (x, y) as it is, never merged
	// (an ingredient laid on its bench keeps its id and reservation).
	PutDown(id entity.EntityID, x, y int)
	// Store has carrier take the stack of kind k lying at (x, y) to the
	// best stockpile (zone 0) or to stockpile zone: a haul job replacing
	// its current one. False when there is no such stack or no room.
	Store(carrier entity.EntityID, k item.Kind, x, y int, zone uint64) bool
}

// Workers is pawns, their jobs, paths and reservations. The reservation
// table is replaced on load, so craft never keeps it.
type Workers interface {
	PawnIDs() []entity.EntityID
	Job(id entity.EntityID) (job.Record, bool)
	SetJob(id entity.EntityID, j job.Record)
	StartJob(id entity.EntityID, rec job.Record) bool
	PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool)
	Reachable(id entity.EntityID, x, y int) bool
	// Owner is who holds target reserved.
	Owner(target entity.EntityID) (entity.EntityID, bool)
	// Claim reserves target for owner.
	Claim(owner, target entity.EntityID) bool
}

// Crafters is skill, work speed, the bench work put off, the colonists'
// names and the tick.
type Crafters interface {
	// SkillLevel is id's level in sk (a recipe's MinSkill, a bill's
	// skill range).
	SkillLevel(id entity.EntityID, sk skill.ID) uint8
	// BenchWork is the work id does this tick at a bench that works at
	// pct percent (skills.go: work speed, skill, and its learning).
	BenchWork(id entity.EntityID, sk skill.ID, pct int) int
	// ButcherYield scales a carcass yield by id's skill.
	ButcherYield(id entity.EntityID, qty int) int
	// Postponed: id put bench off lately (blocked or out of reach).
	Postponed(id, bench entity.EntityID) bool
	// Postpone puts bench off for id: out of reach.
	Postpone(id, bench entity.EntityID)
	// RecipeLocked: research has not unlocked the recipe yet.
	RecipeLocked(id recipe.ID) bool
	// Name is colonist id's name ("" when id is no living colonist).
	Name(id entity.EntityID) string
	// Now is the current tick (a bill's ingredient retry cooldown).
	Now() uint64
}
