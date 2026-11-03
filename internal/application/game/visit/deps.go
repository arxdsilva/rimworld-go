// Package visit is the people who come to the colony from the world and
// leave again (ENG-003 M9, PLAY-024): visitor groups and the fighters a
// faction sends as military aid. It owns the visitor records and the aid
// calls (the game keeps and saves them) and reaches the simulation only
// through the roles below, on the first colony map.
package visit

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything visit needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10).
type Host interface {
	Bodies
	Movers
	Ground
	Arrivals
	Fighters
	Diplomacy
	Records
	Leaving
}

// Leaving is how a visit ends early or with company.
type Leaving interface {
	Turning  // turn.go
	Carrying // carry.go
}

// Bodies is a visitor as a person on the map.
type Bodies interface {
	// Present reports id still a visitor here: its raider record stands,
	// it is nobody's prisoner and its registry place is Visitor.
	Present(id entity.EntityID) bool
	Status(id entity.EntityID) health.Status
	// Hold keeps the prisoner walk-out off a visitor (its raider record
	// leaving, idle now, no retry).
	Hold(id entity.EntityID, now uint64)
	// GuestStep runs a rescued guest's care (true: care has it);
	// GuestDied notes a guest that died while down.
	GuestStep(id entity.EntityID) bool
	GuestDied(id entity.EntityID)
	// Depart takes id off the map; it stays a known person.
	Depart(id entity.EntityID)
	Standing(id entity.EntityID) bool
	Name(id entity.EntityID) string
}

// Records is where the visitor records live: nil while there are none
// and create is false; create makes the store.
type Records interface {
	Records(create bool) *[]Rec
}

// Movers is the visitors' jobs and walking.
type Movers interface {
	Pos(id entity.EntityID) (x, y int)
	Job(id entity.EntityID) job.Record
	SetJob(id entity.EntityID, rec job.Record)
	// Release frees every claim id holds.
	Release(id entity.EntityID)
	StartJob(id entity.EntityID, rec job.Record) bool
	WalkableNear(x, y, r int) (int, int, bool)
	// Free reports dry walkable ground no colonist stands on.
	Free(x, y int) bool
	Size() (w, h int)
}

// Ground is where visitors come from and stroll to, and the trader
// caravan's hooks (trade_caravan.go).
type Ground interface {
	Home() (x, y int)
	// Edge is a map-edge cell the colony can be reached from.
	Edge() (x, y int, ok bool)
	TraderHeld(id entity.EntityID) bool
	StrollRange(id entity.EntityID) int
	TraderGone(id entity.EntityID)
	// WorldRoll and TradeRoll draw from the world and trade streams.
	WorldRoll(n int) int
	TradeRoll(n int) int
}

// Arrivals is making and placing visitors.
type Arrivals interface {
	// NewPerson registers a new person of faction ref (f) with a bio.
	NewPerson(ref int, f *faction.Faction) entity.EntityID
	// Place stands registered person id on (x, y) as a visitor of ref.
	Place(id entity.EntityID, ref, x, y int, now uint64)
	// RelativeRoll and RelativeFor: now and then a visitor is a
	// colonist's relative (relative_world.go), announced by
	// RelativeLetter.
	RelativeRoll() bool
	RelativeFor(ref int, f *faction.Faction) entity.EntityID
	RelativeLetter(kin entity.EntityID, among string)
	// Letter posts a letter of kind ("good", "info", "caution").
	Letter(kind, title, text string, focus entity.EntityID)
}

// Fighters is the allies' fight.
type Fighters interface {
	RaiderIDs() []entity.EntityID
	StandingRaider(id entity.EntityID) bool
	// Arm gives fighter id weapon k.
	Arm(id entity.EntityID, k item.Kind)
	// Strike runs id's attack job j.
	Strike(id entity.EntityID, j job.Record, now uint64)
	// Engage sends id at (x, y) after the nearest foe it can see within
	// r; false when there is none.
	Engage(id entity.EntityID, x, y, r int, now uint64) bool
	// NearestColonist is the standing colonist nearest (x, y).
	NearestColonist(x, y int) (entity.EntityID, int, int, bool)
}

// Carrying is carrying a wounded friend (carry.go).
type Carrying interface {
	// Carrying is whom id carries (0: nobody).
	Carrying(id entity.EntityID) entity.EntityID
	// Pick lifts t onto id; PutDown sets down whoever id carries, if anyone.
	Pick(id, t entity.EntityID)
	PutDown(id entity.EntityID)
	// Beside is a free tile beside (x, y) that id can stand on.
	Beside(id entity.EntityID, x, y int) (int, int, bool)
	// Message posts a map message.
	Message(x, y int, text string)
}

// Diplomacy is the factions and their request cooldowns.
type Diplomacy interface {
	// Factions is the world's peoples (the host's own slice: a pointer
	// into it changes the faction).
	Factions() []faction.Faction
	Cooldown(id int) faction.Cooldowns
	SetCooldown(id int, cd faction.Cooldowns)
	// RelDay is the colony day of tick now.
	RelDay(now uint64) int
}
