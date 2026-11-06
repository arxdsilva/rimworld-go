package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// GearRoles is the game's side of Gear (adapters: game/sense_gear_host.go). Each
// role takes and returns values only; a new need is a new embedded role.
type GearRoles interface {
	GearPawns
	GearStock
	GearJobs
	GearFood
	GearWorn
	GearOrders
}

// GearJobs is how Gear sends a colonist on an errand.
type GearJobs interface {
	Pos(id entity.EntityID) (x, y int)
	Now() uint64
	StartJob(id entity.EntityID, rec job.Record) bool
	Job(id entity.EntityID) job.Record
	SetJob(id entity.EntityID, j job.Record)
	// Rested: id is not tired enough to sleep.
	Rested(id entity.EntityID) bool
	// Free: id is rested and neither at work nor at bedtime.
	Free(id entity.EntityID) bool
}

// GearFood is the food side of an errand.
type GearFood interface {
	// NearestMeal is the closest meal id may take: loose, unclaimed, allowed by
	// its diet and its area.
	NearestMeal(id entity.EntityID) (stack entity.EntityID, x, y int, ok bool)
	Allows(id entity.EntityID, k item.Kind) bool
	// StoreCell is where the colony would store st (a stockpile with room).
	StoreCell(id entity.EntityID, st item.Stack) (x, y int, ok bool)
	// Lay sets the stack id carries down where id stands, as the colony's.
	Lay(id, stack entity.EntityID)
	// Hunger is id's food bar; Hungry its band (with the game's hysteresis).
	Hunger(id entity.EntityID) int
	Hungry(id entity.EntityID) bool
}

// GearWorn is what a colonist wears and wields.
type GearWorn interface {
	// Worn is the slot of the n-th garment id wears, in the read model's order.
	Worn(id entity.EntityID, n int) (slot int, ok bool)
	TakeOff(id entity.EntityID, slot int) bool
	Armed(id entity.EntityID) bool
	DropWeapon(id entity.EntityID) bool
	HasSpare(id entity.EntityID) bool
	DropSpare(id entity.EntityID) bool
	// Interrupt ends id's job the usual way (a carried stack goes back or down).
	Interrupt(id entity.EntityID)
}

// Gear is what a colonist carries off the map: the pack, spare meals, and
// what it has been told to put away (CSE-005, 036…039).
type Gear struct {
	s *Sense
	r GearRoles
}

func newGear(s *Sense, r GearRoles) Gear { return Gear{s: s, r: r} }

// Assign is the colonist's turn when it has nothing to do: eat a meal from the
// pack when hungry, put away what is marked, take a spare meal in free time.
// True: a job began. It reads one pawn's pack and three maps when nothing is
// marked, and allocates nothing.
func (g Gear) Assign(id entity.EntityID) bool {
	if c, ok := g.s.FromPack[id]; ok && g.r.Job(id).Carry != c {
		delete(g.s.FromPack, id) // eaten or set down: no longer in its hands
	}
	p, packed := g.s.Packs[id]
	if packed && g.eatFromPack(id, p) || packed && g.unloadNext(id, p) {
		return true
	}
	return g.takeSpare(id, p)
}

// end finishes a job of Gear's.
func (g Gear) end(id entity.EntityID, j job.Record, ok bool) {
	if ok {
		j.State = job.Complete
	} else {
		j.State, j.Fail = job.Fail, job.TargetGone
	}
	g.r.SetJob(id, j)
}
