package books

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/entity"
)

// The roles the books need from the simulation, each at most eight
// methods (.agents/specs/SOLID_GO.md §7, §10). The game hands in its existing
// adapters where one already fits (research's for Sites, haul's for
// Items, the orders' for Hands) and one small adapter for Mind and
// Colony.

// Sites is the buildings and the rooms they stand in.
type Sites interface {
	// StructVer moves whenever a building is added, removed or changed.
	StructVer() uint64
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	// Room is the room at (x, y) and whether it is indoors.
	Room(x, y int) (r int, indoors bool)
}

// Items is the stacks.
type Items interface {
	ItemIDs() []entity.EntityID
	// StackRef is stack id (nil: none), read only.
	StackRef(id entity.EntityID) *item.Stack
	SetStack(id entity.EntityID, st item.Stack)
	// StacksAt counts the stacks lying loose on (x, y).
	StacksAt(x, y int) int
	// TileTaken reports (x, y) held by someone (a seat in use).
	TileTaken(x, y int) bool
}

// Hands is a reader's job and its claims.
type Hands interface {
	Pos(id entity.EntityID) (x, y int)
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, j job.Record)
	// Spot is the walkable, reachable cell id reaches (x, y) from.
	Spot(id entity.EntityID, x, y int) (sx, sy int, ok bool)
	// Free reports target held by no one but id and not put off for
	// id's job kind k.
	Free(id, target entity.EntityID, k job.Kind) bool
	// Claim holds target for id (false: someone else holds it).
	Claim(id, target entity.EntityID) bool
	// Walk sends id, on job j, to (x, y) (false: no way there).
	Walk(id entity.EntityID, j job.Record, x, y int) bool
}

// Mind is what reading changes in the reader and the colony.
type Mind interface {
	// Joy is id's recreation need (recreation.Full is full).
	Joy(id entity.EntityID) int
	SetJoy(id entity.EntityID, v int)
	// Learn trains id's skill sk by xp (passion and learning rate apply).
	Learn(id entity.EntityID, sk skill.ID, xp int)
	// Level is id's level in sk.
	Level(id entity.EntityID, sk skill.ID) int
	// Study puts pts research points (at base cost) into project p;
	// Cost is p's cost to this colony, percent of its base.
	Study(p research.Project, pts int)
	Cost(p research.Project) int
	// Sighted reports id can see (sight above zero: reading needs it).
	Sighted(id entity.EntityID) bool
}

// Colony is the colony-wide reads and dice.
type Colony interface {
	// Gen moves whenever a loose stack arrives on, or leaves, a cell.
	Gen() uint64
	// Roll is a deterministic draw in [0, n) from the game's dice.
	Roll(n int) int
	// Allowed reports id's allowed area admits (x, y).
	Allowed(id entity.EntityID, x, y int) bool
	// ClaimTile holds (x, y) for id (one reader a seat).
	ClaimTile(id entity.EntityID, x, y int) bool
}
