package ship

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything the ship needs from the simulation, as
// consumer-owned roles of at most eight methods (.cursor/SOLID_GO.md §7).
type Host interface {
	Sites
	Crew
	Colony
}

// Sites is the map's buildings.
type Sites interface {
	// StructVer moves whenever a building is added, removed or changed.
	StructVer() uint64
	// BuildingIDs is every building, ascending (the host's slice, read only).
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	// Lift takes building id off the map, nothing left behind (the launch).
	Lift(id entity.EntityID)
}

// Crew is the colonists who board the ship.
type Crew interface {
	// Ready reports id a colonist on the map who can walk (not downed,
	// dead or away).
	Ready(id entity.EntityID) bool
	Name(id entity.EntityID) string
	// Spot is a reachable cell beside (x, y) that id can stand on.
	Spot(id entity.EntityID, x, y int) (sx, sy int, ok bool)
	StartJob(id entity.EntityID, rec job.Record) bool
	SetJob(id entity.EntityID, j job.Record)
	// Board takes colonist id off the map into casket (the stasis keeps
	// it unchanged); false: it could not.
	Board(id, casket entity.EntityID) bool
	// Wake brings sleeper id back onto the map beside casket.
	Wake(id, casket entity.EntityID) bool
}

// Colony is the colony-wide clock, raids and notices.
type Colony interface {
	Now() uint64
	// Seed is the colony's base seed (the raid spacing hashes it).
	Seed() uint64
	// Raid sends a countdown raid through the storyteller: mechanoids
	// when mech, else people from a hostile faction.
	Raid(mech bool)
	// Letter posts a letter at (x, y): a red one when danger.
	Letter(title, text string, x, y int, danger bool)
	// Message posts a quiet message at (x, y).
	Message(x, y int, text string)
	// Changed is called after a change the views show.
	Changed()
}
