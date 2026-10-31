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
	Makers
	Weathering
	Workshop
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
	// FreeWood is the wood lying free (not carried, not forbidden).
	FreeWood() int
	Stack(id entity.EntityID) (item.Stack, bool)
	// Carry has carrier pick take of stack up: the whole stack, or a new
	// stack split off it. It returns the carried stack's id.
	Carry(carrier, stack entity.EntityID, take int) entity.EntityID
	// ItemIDs is every item stack's id (the host's slice, read only): the
	// multi-stack ingredient search (PRD-1).
	ItemIDs() []entity.EntityID
	// InSource reports (x, y) lies in storage zone or group src, a bill's
	// ingredient source (RES-1); SourceName is src's name ("" none).
	InSource(src uint64, x, y int) bool
	SourceName(src uint64) string
}

// Output is what a finished bill or batch leaves behind.
type Output interface {
	DeleteItem(id entity.EntityID)
	// DropStack spawns qty of k at (x, y), merging with a stack there.
	DropStack(k item.Kind, qty, x, y int)
	// DropStackOf spawns a copy of st (rot, dried, claim) at (x, y).
	DropStackOf(st item.Stack, x, y int)
	// DropMeal drops meal stack st (kind, quantity and FOOD-4's
	// ingredients) cooked by cook at a bench of kind bench, with the
	// cook's poison chance rolled by the host.
	DropMeal(cook entity.EntityID, bench construction.Kind, st item.Stack, x, y int)
	// Cremate burns body on pyre with the recipe's wood.
	Cremate(pyre, body entity.EntityID, d recipe.Def)
	// Message posts a plain message at (x, y).
	Message(x, y int, text string)
	// Store has carrier take the stack of kind k lying at (x, y) to the
	// best stockpile (zone 0) or to stockpile zone: a haul job replacing
	// its current one. False when there is no such stack or no room.
	Store(carrier entity.EntityID, k item.Kind, x, y int, zone uint64) bool
	// ButcheredHuman: butcher cut up a human body (FOOD-3: the butcher's
	// and the onlookers' moods).
	ButcheredHuman(butcher entity.EntityID)
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
	// Claims is the reservation table, asked afresh on every use.
	Claims() Claims
}

// Claims is the reservation table (RES-1): a bench and the ingredients a
// bill takes are claimed for its worker; an ingredient as a count, so
// two crafters share a stack, and the bench's work spot as a tile.
type Claims interface {
	EntityOwner(target entity.EntityID) (entity.EntityID, bool)
	TryEntity(owner, target entity.EntityID) bool
	// TryCount claims n of stack target, which holds total.
	TryCount(owner, target entity.EntityID, n, total int) bool
	// FreeFor is how much of stack target (holding total) owner may still
	// claim.
	FreeFor(owner, target entity.EntityID, total int) int
	ReleaseEntity(owner, target entity.EntityID)
	TileOwner(x, y int) (entity.EntityID, bool)
	TryTile(owner entity.EntityID, x, y int) bool
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

// Makers is the dice of made things and the inspiration they spend
// (PRD-2).
type Makers interface {
	// Roll is a uniform draw in [0, n) from the host's craft stream.
	Roll(n int) int
	// UseInspiration ends id's creativity inspiration and reports whether
	// it had one: the thing it makes rolls inspired.
	UseInspiration(id entity.EntityID) bool
}

// Weathering is what rot and deterioration read and write (PRD-2), and
// the worn outfits that wear out.
type Weathering interface {
	// Exposed: (x, y) lies under the open sky (no roof).
	Exposed(x, y int) bool
	// Raining: rain (not snow) is falling.
	Raining() bool
	// Wet: (x, y) is water or marsh.
	Wet(x, y int) bool
	SetStack(id entity.EntityID, st item.Stack)
	// Pos is pawn id's tile (ok false when it has none).
	Pos(id entity.EntityID) (x, y int, ok bool)
	Outfit(id entity.EntityID) Outfit
	SetOutfit(id entity.EntityID, o Outfit)
}

// Workshop is what bench work speed, forge fuel and sculptures read and
// write (PRD-3).
type Workshop interface {
	// Light is the light on (x, y) in permille (1000: full daylight).
	Light(x, y int) int
	// Cleanliness is the cleanliness of the room at (x, y) in hundredths
	// (room.CleanlinessLabel); indoors is false outside any room or under
	// the open sky.
	Cleanliness(x, y int) (hundredths int, indoors bool)
	// UseFuel burns the fuel work units of forge work take (one log per
	// WorkPerWood units).
	UseFuel(id entity.EntityID, work int)
	// CanInstall reports Install would set a building of kind k down at
	// (x, y): the placement rule allows it there.
	CanInstall(k construction.Kind, x, y int) bool
	// Install sets a finished building of kind k in stuff, of stored
	// quality q, down at (x, y) facing south; the caller has checked
	// CanInstall (0: the footprint is taken after all).
	Install(k construction.Kind, stuff item.Kind, q uint8, x, y int) entity.EntityID
}
