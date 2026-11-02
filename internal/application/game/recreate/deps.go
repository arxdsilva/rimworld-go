// Package recreate is the colony's recreation at buildings (AX-6; wiki:
// Recreation, Horseshoes pin, Hoopstone ring, Game of Ur, Chess table,
// Billiards table, Poker table, Tube/Flatscreen/Megascreen television,
// Telescope): which recreation piece a colonist with free time goes to,
// the game it plays, the screen it watches or the sky it studies there,
// the recreation and skill it gains, and the tolerance each joy kind
// builds (the variety rule, recreation.Tolerance).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package and
// reaches the simulation only through Host. Its state (tolerances, the
// fractional recreation carried, retry cooldowns) hangs off the game's
// social state and is saved as saveState.Recreation. It is deterministic:
// no random numbers, no wall clock, buildings and pawns walked in the
// host's order.
package recreate

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is what recreation needs from the simulation, as consumer-owned
// roles; the game implements the first five with its rest adapters.
type Host interface {
	Bodies
	Buildings
	Claims
	Routine
	Rooms
	Senses
	Feelings
}

// Bodies places and tasks a pawn.
type Bodies interface {
	Pos(id entity.EntityID) (x, y int)
	SetJob(id entity.EntityID, j job.Record)
	StartJob(id entity.EntityID, rec job.Record) bool
}

// Buildings reads the buildings (the host's own id slice, read only).
type Buildings interface {
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
}

// Claims checks and holds the cells a pawn uses.
type Claims interface {
	AreaAllows(id entity.EntityID, x, y int) bool
	Reachable(id entity.EntityID, x, y int) bool
	ReserveTile(id entity.EntityID, x, y int)
	// TileFree: walkable, no bed over it, and not held by another pawn.
	TileFree(id entity.EntityID, x, y int) bool
}

// Routine reads a pawn's job.
type Routine interface {
	Job(id entity.EntityID) job.Record
}

// Rooms reads what stands on a cell and the roof over it.
type Rooms interface {
	ObjectAt(x, y int) entity.EntityID
	Roof(x, y int) (roofed, thick bool)
}

// Senses is the world as a recreation piece needs it.
type Senses interface {
	Now() uint64
	// Powered reports building id has power now.
	Powered(id entity.EntityID) bool
	// Sky is the daylight now, permille (0 night, 1000 noon).
	Sky() int
	// Sight is pawn id's sight capacity, permille.
	Sight(id entity.EntityID) int
	// PawnIDs are the colonists (read only).
	PawnIDs() []entity.EntityID
	// Wealth is the colony's wealth (read once a day at most).
	Wealth() int
}

// Feelings changes a pawn's recreation and skills.
type Feelings interface {
	Joy(id entity.EntityID) int
	// AddJoy raises id's recreation by n points, clamped to full.
	AddJoy(id entity.EntityID, n int)
	Learn(id entity.EntityID, sk skill.ID, xp int)
	// Enjoyed records a pastime finished (a fine recreation room's mood).
	Enjoyed(id entity.EntityID)
}
