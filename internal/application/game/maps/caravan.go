// Package maps is the world beyond one map (ENG-003 M9, MAP-*): the
// caravans that leave a map, travel the planet and come back or enter
// another map, and the open maps' lifecycle. It owns its state; the game
// package keeps it in its world state and saves it, and reaches the
// simulation only through the roles in deps.go.
package maps

import (
	"slices"

	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

const (
	// FoodDays is the food a caravan sets out with (and restocks to at a
	// friendly settlement), in milli-days.
	FoodDays int32 = 5000
	// visitTicks is the stay at a friendly settlement.
	visitTicks = clock.TicksPerDay
	// talkPeriod is how often two members talk on the road.
	talkPeriod = 2 * clock.TicksPerHour
	// talkJoy is the recreation a talk on the road gives each side.
	talkJoy = recreation.Full / 25
	// deepTalkPct is the share of talks on the road that go deep.
	deepTalkPct = 25
	// FormCheck: forming members are looked at this often; one that is
	// still on the map formLimit after the order stays behind.
	FormCheck = 60
	formLimit = clock.TicksPerDay
)

// Caravan is one caravan with the colony's bookkeeping around it (saved).
type Caravan struct {
	C caravan.Caravan
	// Home is the colony's tile; Out the outbound route, home first and
	// the destination last (the way back is the travelled part reversed).
	Home int
	Out  []int
	// Back: the caravan is on its way home.
	Back bool
	// Exiting are members still walking to the map edge (Forming).
	Exiting []entity.EntityID
	// Formed is when it was ordered; NextTalk the next talk on the road.
	Formed, NextTalk uint64
	// Map is the map it formed on (MAP-01; 0 the first colony).
	Map uint32
	// NextEnc is when it may next meet trouble; Ask a demand waiting for
	// its answer: the caravan stands still meanwhile (MAP-08).
	NextEnc uint64
	Ask     Ask
}

// Clone is a deep copy of c.
func (c Caravan) Clone() Caravan {
	c.C.Members = slices.Clone(c.C.Members)
	c.C.Animals = slices.Clone(c.C.Animals)
	c.C.Path = slices.Clone(c.C.Path)
	c.Out = slices.Clone(c.Out)
	c.Exiting = slices.Clone(c.Exiting)
	return c
}

// Fleet is the caravans (ascending id, the caller's slice) and the open
// maps' atlas over the host.
type Fleet struct {
	list *[]Caravan
	a    *Atlas
	h    Host
}

// Of is the fleet in list with atlas a.
func Of(list *[]Caravan, a *Atlas, h Host) Fleet { return Fleet{list, a, h} }

// At is caravan id (nil: none).
func (f Fleet) At(id entity.EntityID) *Caravan {
	for i := range *f.list {
		if (*f.list)[i].C.ID == uint64(id) {
			return &(*f.list)[i]
		}
	}
	return nil
}

// Of is the caravan pawn id belongs to (nil: none).
func (f Fleet) Of(id entity.EntityID) *Caravan {
	for i := range *f.list {
		if slices.Contains((*f.list)[i].C.Members, uint64(id)) {
			return &(*f.list)[i]
		}
	}
	return nil
}

// drop removes caravan id.
func (f Fleet) drop(id entity.EntityID) {
	*f.list = slices.DeleteFunc(*f.list, func(c Caravan) bool { return c.C.ID == uint64(id) })
}

// Land: pod passengers (stashed) become a caravan arrived on t (home by
// land), animals riding with it; its id.
func (f Fleet) Land(members, animals []uint64, t int) uint64 {
	home := f.h.ColonyTile()
	c := Caravan{C: caravan.Caravan{ID: f.h.NewID(), Members: members, Animals: animals, Tile: t, Dest: t,
		State: caravan.Arrived, FoodDays: FoodDays}, Home: home, Out: f.h.Planet().Route(home, t)}
	for _, m := range members {
		f.h.Rehold(entity.EntityID(m), entity.EntityID(c.C.ID))
	}
	*f.list = append(*f.list, c)
	f.Arrived(&(*f.list)[len(*f.list)-1], f.h.Now())
	return c.C.ID
}

// Medic is c's best medic: a standing member able to doctor with the
// highest Medicine level (at least 1; ties to the lowest id); 0 when there
// is none.
func (f Fleet) Medic(c *Caravan) entity.EntityID {
	var best entity.EntityID
	bestLvl := 0
	for _, m := range c.C.Members {
		id := entity.EntityID(m)
		lvl := f.h.MedicLevel(id)
		if lvl < 1 {
			continue
		}
		if lvl > bestLvl || lvl == bestLvl && id < best {
			best, bestLvl = id, lvl
		}
	}
	return best
}

// care has c's medic tend whatever is due on its members, in member order
// (hourly on the road; WS-M).
func (f Fleet) care(c *Caravan, now uint64) {
	medic := f.Medic(c)
	if medic == 0 {
		return
	}
	for _, m := range slices.Clone(c.C.Members) {
		id := entity.EntityID(m)
		if f.h.TendDue(id, now) && f.h.MayTend(medic, id) {
			f.h.Tend(medic, id, now)
		}
	}
}

// DevArrive sets caravan id down at tile as if it had walked there.
func DevArrive(f Fleet, id entity.EntityID, tile int, now uint64) {
	if c := f.At(id); c != nil {
		c.C.Tile, c.C.Dest, c.C.Path, c.C.State = tile, tile, nil, caravan.Arrived
		f.Arrived(c, now)
	}
}

// DevDemand stages a road demand on caravan id (MAP-08 rolls one at random).
func DevDemand(f Fleet, id entity.EntityID, now uint64) {
	if c := f.At(id); c != nil {
		f.Waylay(c, Demand, now)
	}
}
