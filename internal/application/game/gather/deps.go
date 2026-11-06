package gather

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything gather needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10). Every method takes and returns
// values: gatherings run on the tick path of every pawn.
type Host interface {
	Folk
	Walks
	Grounds
	Hearts
	Tidings
	Keeper
}

// Keeper is where the state lives.
type Keeper interface {
	// State is the gathering state: nil while there is none and create is
	// false.
	State(create bool) *State
}

// Folk is who can come.
type Folk interface {
	Alive(id entity.EntityID) bool
	Romanceable(id entity.EntityID) bool
	// OnMap reports id on the map that holds gatherings.
	OnMap(id entity.EntityID) bool
	Drafted(id entity.EntityID) bool
	Breaking(id entity.EntityID) bool
	Colonist(id entity.EntityID) bool
	PawnIDs() []entity.EntityID
	Mood(id entity.EntityID) int
}

// Walks is the guests' jobs.
type Walks interface {
	Job(id entity.EntityID) job.Record
	SetJob(id entity.EntityID, j job.Record)
	// Interrupt stops what id is doing.
	Interrupt(id entity.EntityID)
	StartJob(id entity.EntityID, rec job.Record) bool
	ClaimTile(id entity.EntityID, x, y int)
	TileTaken(x, y int) bool
	Reachable(id entity.EntityID, x, y int) bool
	ReachableSpot(id entity.EntityID, x, y int) (int, int, bool)
}

// Grounds is where a gathering is held.
type Grounds interface {
	Pos(id entity.EntityID) (x, y int, ok bool)
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	Walkable(x, y int) bool
	BedCovering(x, y int) bool
	Home() (x, y int)
	ThreatActive(now uint64) bool
}

// Hearts is couples, needs and memories.
type Hearts interface {
	Pair(a, b entity.EntityID) social.Pair
	SetPair(a, b entity.EntityID, p social.Pair)
	// Partners are id's social edges.
	Partners(id entity.EntityID) []entity.EntityID
	// Needy reports id tired, or hungry with food to seek; Bedtime its
	// sleeping hours while it still wants rest.
	Needy(id entity.EntityID) bool
	Bedtime(id entity.EntityID) bool
	// LovinCheck is id's staggered look at lovin' (rest/lovin.go).
	LovinCheck(id entity.EntityID, now uint64)
	AddJoy(id entity.EntityID, n int)
	Remember(id entity.EntityID, t mood.Kind, other entity.EntityID, now uint64)
}

// Tidings is names, messages and the rest.
type Tidings interface {
	Name(id entity.EntityID) string
	Bio(id entity.EntityID) (pawngen.Pawn, bool)
	SetSurname(id entity.EntityID, last string)
	// Note posts a message, GoodLetter a good letter, at (x, y): the text
	// a format and args (the catalog's key, OPT-001).
	Note(x, y int, focus entity.EntityID, format string, args ...any)
	GoodLetter(title string, x, y int, focus entity.EntityID, format string, args ...any)
	RomanceLetter(title, text string, focus entity.EntityID)
	History(kind, text string, a, b entity.EntityID)
	// Roll draws from the social stream.
	Roll(n int) int
}
