package rest

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything rest needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10). The game package implements each
// role with a small adapter type (rest_host.go). Every method takes and
// returns values: never a pointer to a caller's local, never a closure,
// so a host call on the tick path moves nothing to the heap.
type Host interface {
	Bodies
	Bedding
	Claims
	Routine
	Patients
	Rooms
	Events
	Couples
}

// Bodies reads and writes a sleeper's place, vital record, needs and job.
type Bodies interface {
	// Pos is id's tile (0, 0 when it has none).
	Pos(id entity.EntityID) (x, y int)
	Vital(id entity.EntityID) (health.Record, bool)
	SetVital(id entity.EntityID, v health.Record)
	Needs(id entity.EntityID) needs.Pair
	SetNeeds(id entity.EntityID, n needs.Pair)
	SetJob(id entity.EntityID, j job.Record)
	// StartJob gives id the job (false: it could not start, e.g. out of
	// reach).
	StartJob(id entity.EntityID, rec job.Record) bool
	// FloorSpot is where id standing on (x, y) lies down with no bed:
	// there, or the nearest cell inside its allowed area.
	FloorSpot(id entity.EntityID, x, y int) (fx, fy int)
}

// Bedding reads and writes beds and who slept where. The building id
// slice is the host's own, read only. Where a pawn slept stays on the
// host: orders, surgery, prisoners and caskets write it too.
type Bedding interface {
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	SetBuilding(id entity.EntityID, b construction.Building)
	// SleptAt is the bed id last went to sleep in (0: the ground); ok
	// false when it has no record.
	SleptAt(id entity.EntityID) (bed entity.EntityID, ok bool)
	SetSleptAt(id, bed entity.EntityID)
	// ForgetSleptAt drops id's record.
	ForgetSleptAt(id entity.EntityID)
	// ReleaseBed gives up every bed slot id holds.
	ReleaseBed(id entity.EntityID)
	// SleepCell is where id lies in bed b (its slot's head cell).
	SleepCell(id entity.EntityID, b construction.Building) (x, y int)
}

// Claims answers who may hold which bed.
type Claims interface {
	// AreaAllows: (x, y) lies inside id's allowed area (true when
	// unrestricted).
	AreaAllows(id entity.EntityID, x, y int) bool
	// MayOwnBed: id is a living colonist (a bed owner the player may
	// assign).
	MayOwnBed(id entity.EntityID) bool
	// Partner is the partner id shares a double bed with: id's main
	// partner when both are colonists who may court and the partner is
	// on the map (0: none).
	Partner(id entity.EntityID) entity.EntityID
}

// Routine is the schedule and the meals that can wake a sleeper, and the
// job a pawn is on.
type Routine interface {
	// Activity is what id's schedule says for the current hour.
	Activity(id entity.EntityID) schedule.Activity
	// SeekFood: id should go looking for food now.
	SeekFood(id entity.EntityID) bool
	// Job is id's current job.
	Job(id entity.EntityID) job.Record
}

// Patients is what sleep asks the care package about a sleeper.
type Patients interface {
	// OnBedRest: id is on bed rest (care/rest.go).
	OnBedRest(id entity.EntityID) bool
	// MayUseMedicalBed: id may lie in a medical bed (a patient).
	MayUseMedicalBed(id entity.EntityID) bool
	// SurgeryWait: a patient on bed rest whose sleep job has workLeft
	// left; one that waited for a surgeon who never came gives up.
	SurgeryWait(id entity.EntityID, workLeft int)
}

// Rooms is what the thoughts on waking read about where a pawn slept,
// and what bed stats and animal beds read about the map (BED-1).
type Rooms interface {
	// RoomAt is the room at (x, y) (negative: none, outdoors).
	RoomAt(x, y int) int
	UnderSky(x, y int) bool
	// RoomScore is room r's quality score.
	RoomScore(r int) int
	// ObjectAt is the object-layer building covering (x, y) (0: none),
	// from the host's cell index.
	ObjectAt(x, y int) entity.EntityID
	// TameAnimal: id is a tame animal on the map (one that may hold an
	// animal bed).
	TameAnimal(id entity.EntityID) bool
}

// Events records what sleep did, for logs and mood.
type Events interface {
	// Remember gives id the thought k now.
	Remember(id entity.EntityID, k mood.Kind)
	// ForgetGroup drops id's memories in k's group.
	ForgetGroup(id entity.EntityID, k mood.Kind)
	// Slept logs that id woke, in a bed or on the ground.
	Slept(id entity.EntityID, inBed bool)
	// RememberWith gives id the thought k about other now (lovin.go).
	RememberWith(id entity.EntityID, k mood.Kind, other entity.EntityID)
}

// Couples is what lovin' reads and writes about a couple and whether each
// is up to it (lovin.go). The cooldown is saved with the gatherings and
// the roll is on the social stream, so both stay on the host.
type Couples interface {
	// Mate is id's main partner (0: none), whoever and wherever.
	Mate(id entity.EntityID) entity.EntityID
	// Opinion is from's opinion of to.
	Opinion(from, to entity.EntityID) int32
	// Age is id's age in years.
	Age(id entity.EntityID) int
	// LovinAt is the next tick id may make love (0: any time).
	LovinAt(id entity.EntityID) uint64
	SetLovinAt(id entity.EntityID, tick uint64)
	// SocialRoll is the social stream's next draw in [0, n).
	SocialRoll(n int) int
	// Standing: alive, not in a mental break, not drafted.
	Standing(id entity.EntityID) bool
	// Hurt is id's pain and consciousness, permille (ok false: no body).
	Hurt(id entity.EntityID) (pain, conscious int32, ok bool)
}
