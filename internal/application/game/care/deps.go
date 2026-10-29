package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything care needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10). The game package implements each
// role with a small adapter type (care_host.go). Every method takes and
// returns values: never a pointer to a caller's local, never a closure,
// so a host call on the tick path moves nothing to the heap.
type Host interface {
	Bodies
	Roster
	Beds
	Reach
	Jobs
	Health
	Treatment
	Events
}

// Bodies reads and writes a pawn's place, vital record and job.
type Bodies interface {
	// Pos is id's tile (ok false when it has none).
	Pos(id entity.EntityID) (x, y int, ok bool)
	// Place puts id on tile (x, y).
	Place(id entity.EntityID, x, y int)
	Vital(id entity.EntityID) (health.Record, bool)
	SetVital(id entity.EntityID, v health.Record)
	Job(id entity.EntityID) (job.Record, bool)
	SetJob(id entity.EntityID, j job.Record)
	// Now is the simulation tick.
	Now() uint64
	// BleedPerDay is id's blood loss rate (0 without a body).
	BleedPerDay(id entity.EntityID) int32
}

// Roster answers who a pawn is. The id slices are the host's own, read
// only.
type Roster interface {
	PawnIDs() []entity.EntityID
	RaiderIDs() []entity.EntityID
	IsPrisoner(id entity.EntityID) bool
	// IsColonist: a pawn of the colony (not a raider).
	IsColonist(id entity.EntityID) bool
	Drafted(id entity.EntityID) bool
	Name(id entity.EntityID) string
	// CareGroup is the default care group id belongs to.
	CareGroup(id entity.EntityID) medcare.Group
	// DoctorBlocked: id can't doctor now (dead or down, in a mental
	// break, or incapable of Doctor work).
	DoctorBlocked(id entity.EntityID) bool
}

// Beds reads and writes buildings and the beds pawns lie in.
type Beds interface {
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	SetBuilding(id entity.EntityID, b construction.Building)
	// BedUnder is the claimed bed covering id's tile (0 when none).
	BedUnder(id entity.EntityID) entity.EntityID
	// SleepCell is where id lies in bed b.
	SleepCell(id entity.EntityID, b construction.Building) (x, y int)
	// ReleaseBed gives up every bed slot id holds.
	ReleaseBed(id entity.EntityID)
	SetSleptAt(id, bed entity.EntityID)
}

// Reach is paths and reservations. The reservation table is replaced on
// load, so care never keeps it: it asks through these methods.
type Reach interface {
	// BesideSpot is the reachable tile beside (x, y) nearest id, not
	// under a bed.
	BesideSpot(id entity.EntityID, x, y int) (sx, sy int, ok bool)
	PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool)
	// Owner is who holds target reserved.
	Owner(target entity.EntityID) (entity.EntityID, bool)
	// Claim reserves target for owner.
	Claim(owner, target entity.EntityID) bool
	// BedHazard: bed is no safe place to lay a patient now (burning, or
	// its room at a deadly temperature; SIM-063).
	BedHazard(bed entity.EntityID) bool
}

// Jobs starts and stops pawns' jobs.
type Jobs interface {
	StartJob(id entity.EntityID, rec job.Record) bool
	Interrupt(id entity.EntityID, reason job.FailReason)
	// Breaking: id is in a mental break (it takes no orders).
	Breaking(id entity.EntityID) bool
	// GoEat sends a hungry id to eat when there is food (false: not
	// hungry, or nothing to eat).
	GoEat(id entity.EntityID) bool
	// SleepInBed sends id to its own bed or the nearest free one it may
	// claim.
	SleepInBed(id entity.EntityID) bool
	// SleepInPlace lays id down where it stands.
	SleepInPlace(id entity.EntityID)
	// LieInHospitalBed sends a patient waiting for surgery to a free
	// hospital bed.
	LieInHospitalBed(id entity.EntityID) bool
}

// Health is what care asks about a pawn's condition and work.
type Health interface {
	// NeedsTending: a living humanlike with something a tend would help.
	NeedsTending(id entity.EntityID) bool
	// Standing: a colonist alive and on its feet.
	Standing(id entity.EntityID) bool
	// DoesDoctor: id has Doctor work enabled in its priorities.
	DoesDoctor(id entity.EntityID) bool
	// IsPatient: a pawn or an animal (someone with a care level).
	IsPatient(id entity.EntityID) bool
	// HealingInjury is the part of id's injury rest can still heal.
	HealingInjury(id entity.EntityID) int
	// SickRest: sick enough to go to bed (an infection, a disease at its
	// major stage or worse, food poisoning at its major stage).
	SickRest(id entity.EntityID) bool
	// SickStill: still too sick to get up (an infection, a disease not
	// yet immune or cured, food poisoning not yet recovering).
	SickStill(id entity.EntityID) bool
	// AwaitingSurgery: id has an operation it waits for.
	AwaitingSurgery(id entity.EntityID) bool
}

// Treatment is medicine and the tend itself.
type Treatment interface {
	// MedicineTiers is the medicine kinds a tend for patient may use, in
	// the order tried.
	MedicineTiers(patient entity.EntityID) [3]item.Kind
	// NearestStack is the nearest free stack of kind k id could fetch
	// one of (0 when none).
	NearestStack(id entity.EntityID, k item.Kind) (entity.EntityID, item.Stack)
	// PickUpDose: doctor id reached j.Dest; it takes one dose and heads
	// for the patient. It returns the updated job and false while still
	// walking or when the job had to end (the host has stored it then).
	PickUpDose(id entity.EntityID, j job.Record) (job.Record, bool)
	DeleteItem(id entity.EntityID)
	// MedicineWork is doctor id's work done this tick (Medicine skill).
	MedicineWork(id entity.EntityID) int
	// TendQuality rolls doctor's tend on patient with the dose j carries
	// (permille, on the host's medical stream).
	TendQuality(doctor, patient entity.EntityID, j job.Record) int32
	// ApplyTend tends every wound of patient at quality q now: the body,
	// standing up when it can, and the vital record.
	ApplyTend(patient entity.EntityID, q int32)
	// Feed: patient eats from the food stack carry its feeder brought
	// (SIM-064; the warden's prisoner feeding, generalised).
	Feed(patient, carry entity.EntityID)
}

// Events records what care did, for logs and opinions.
type Events interface {
	// Rescued: rescuer laid patient in a bed.
	Rescued(rescuer, patient entity.EntityID)
	// TendedBy: doctor finished tending patient.
	TendedBy(patient, doctor entity.EntityID)
	// Message shows the player text at tile (x, y).
	Message(x, y int, text string)
}

// Prison is what capture and arrest need from the simulation (SIM-062).
// It is a second constructor argument, not a role of Host, so Host stays
// at eight roles.
type Prison interface {
	// IsRaider: id is a raider on the map (prisoners included).
	IsRaider(id entity.EntityID) bool
	// IsVisitor: id is a friendly visitor on the map.
	IsVisitor(id entity.EntityID) bool
	// PersonName names anyone, colonist, raider or visitor.
	PersonName(id entity.EntityID) string
	// PrisonBedFor is the free prisoner bed nearest (x, y) target may be
	// laid in.
	PrisonBedFor(target entity.EntityID, x, y int) (entity.EntityID, bool)
	// DropGear leaves target's weapon on the ground (a captive keeps none).
	DropGear(target entity.EntityID)
	// MakePrisoner turns target into the colony's prisoner, taken by
	// captor; arrested marks an arrest (a visitor's faction resents it).
	MakePrisoner(target, captor entity.EntityID, arrested bool)
	// ArrestRoll rolls arrester's attempt on target on the host's prison
	// stream: true when target gives in.
	ArrestRoll(arrester, target entity.EntityID) bool
	// StartFight starts a social fight: starter attacks other.
	StartFight(starter, other entity.EntityID)
}

// Guests is what rescuing non-colonists needs (SIM-063): visitors,
// traders, allies and faction-less strangers on the map.
type Guests interface {
	// IsGuest: id is a non-hostile humanlike on the map that is neither a
	// colonist nor a prisoner.
	IsGuest(id entity.EntityID) bool
	// FactionOf is id's faction (0: none, a stranger).
	FactionOf(id entity.EntityID) uint64
	// Goodwill changes faction's goodwill by delta, for reason.
	Goodwill(faction uint64, delta int, reason string)
	// Leave sends a recovered guest off the map.
	Leave(id entity.EntityID)
	// JoinOffer asks the player whether stranger id, rescued, may join.
	JoinOffer(id entity.EntityID)
}

// Social is what visiting the sick needs (SIM-065).
type Social interface {
	// Opinion is from's opinion of to.
	Opinion(from, to entity.EntityID) int
	// Awake: id is not asleep.
	Awake(id entity.EntityID) bool
	// Interact runs one social interaction between a and b now.
	Interact(a, b entity.EntityID)
	// Recreation is id's recreation need; Recreate adds amount to it.
	Recreation(id entity.EntityID) int
	Recreate(id entity.EntityID, amount int)
}

// Gear is what stripping needs (SIM-065).
type Gear interface {
	// StripGear drops target's apparel and weapon as items where it lies;
	// false when it had nothing to drop.
	StripGear(target entity.EntityID) bool
	// HasGear: target wears apparel or holds a weapon to strip.
	HasGear(target entity.EntityID) bool
}
