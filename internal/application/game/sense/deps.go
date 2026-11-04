// Package sense holds the small everyday habits of a colonist's day: tidying
// the home, strolling when idle. Each is a component of its own over the
// roles below (.cursor/SOLID_GO.md §10); the game package implements the
// roles with small adapters (sense_host.go), and builds a component where
// it needs one, so there is no state here and nothing is saved.
package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/entity"
)

// Filth is the cleaning queue and what lies on a tile.
type Filth interface {
	// Queue is the sorted queue of Home tiles to clean; the backing array is
	// the host's, and Assign compacts it in place.
	Queue() []int32
	SetQueue(q []int32)
	Dirt(i int) int
	Stain(i int) int
	AddDirt(i, delta int)
	SetStain(x, y, amount int)
	// CleanFree: nobody holds tile (x, y), id hasn't put cleaning it off and
	// its allowed area admits it.
	CleanFree(id entity.EntityID, x, y int) bool
}

// Ground is the map's shape and its Home area.
type Ground interface {
	Width() int
	InBounds(x, y int) bool
	// HomeCells is the Home area by tile index (non-zero: Home), read per
	// tile of the queue, so a slice and not a call.
	HomeCells() []uint8
}

// Walks sends a pawn somewhere.
type Walks interface {
	Pos(id entity.EntityID) (x, y int)
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, j job.Record)
	Reachable(id entity.EntityID, x, y int) bool
	// ReachableSpot is the tile id works (x, y) from.
	ReachableSpot(id entity.EntityID, x, y int) (tx, ty int, ok bool)
	// PostponeTile puts tile work off as out of reach.
	PostponeTile(id entity.EntityID, kind job.Kind, x, y int)
	ClaimTile(id entity.EntityID, x, y int)
	TileTaken(x, y int) bool
}

// Labor is who works and how fast.
type Labor interface {
	PawnIDs() []entity.EntityID
	IsRaider(id entity.EntityID) bool
	HasLabor(id entity.EntityID) bool
	Priorities(id entity.EntityID) work.Priorities
	SetPriority(id entity.EntityID, t work.Type, v int)
	// WorkUnits is id's work speed this tick, with no skill in it (0: none).
	WorkUnits(id entity.EntityID) int
}

// Strolls is what a stroll asks of the pawn and the tile.
type Strolls interface {
	// Roams: id's temperament takes it farther.
	Roams(id entity.EntityID) bool
	// Roll draws from the main random stream.
	Roll(n int) int
	// OpenTile: id may stand on (x, y): walkable, no bed on it, no fire
	// danger, inside its allowed area.
	OpenTile(id entity.EntityID, x, y int) bool
}

// Shelter is the sky and the roof for sheltered strolls (CSE-041).
type Shelter interface {
	// ShouldHide: id is a colonist on its home map with no lord duty, and
	// the sky is unfit: rain or storm, or fallout without full toxic
	// resistance. Weather changes only each 6-hour block, so this holds
	// steady between blocks.
	ShouldHide(id entity.EntityID) bool
	// Roofed: (x, y) lies under a roof.
	Roofed(x, y int) bool
	// Home: (x, y) is in the Home area.
	Home(x, y int) bool
}

// Manners is whose rooms an idle pawn leaves alone (CSE-042).
type Manners interface {
	// Rude: id would intrude standing on (x, y): another's bedroom, a
	// hospital, a prison cell or barracks, a cooled store. Hostile pawns
	// and prisoners never are.
	Rude(id entity.EntityID, x, y int) bool
}
