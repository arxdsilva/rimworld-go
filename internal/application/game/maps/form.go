package maps

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Forming a caravan (ENG-003 M9). Its members are drafted (schedules and
// work leave them alone) and walk to the nearest map edge (job.ExitMap),
// where each is stashed off the map; the caravan sets out once nobody is
// still walking.

// Form sends the listed members (comma-separated entity ids) to world
// tile dest.
func (f Fleet) Form(members string, dest int) {
	p, cur := f.h.Planet(), f.h.Current()
	home := f.tileOf(cur) // MAP-01: the map it forms on
	if p == nil || home < 0 {
		f.h.Note("No world map: this colony cannot send caravans.")
		return
	}
	if dest < 0 || dest >= p.TileCount() || dest == home {
		f.h.Note("Pick a destination for the caravan.")
		return
	}
	ids := travel.ParseIDs(members, func(id entity.EntityID) bool { return f.h.Candidate(id) && f.Of(id) == nil })
	if len(ids) == 0 {
		f.h.Note("Nobody can join the caravan.")
		return
	}
	route := p.Route(home, dest)
	if len(route) < 2 {
		f.h.Note("The caravan has no way to get there.")
		return
	}
	now := f.h.Now()
	c := Caravan{
		C:    caravan.Caravan{ID: f.h.NewID(), Tile: home, Dest: dest, Path: slices.Clone(route[1:]), State: caravan.Forming},
		Home: home, Out: route, Formed: now, Map: cur,
	}
	cargo := f.h.Cargo()
	cargo.Pack(c.C.ID, members, len(ids)) // game/travel (AX-9): goods and pack animals
	for _, id := range ids {
		f.h.SetDraft(id, true) // AX-9: a hitching spot first
		if !cargo.Hitch(c.C.ID, id, len(c.C.Members)) && !f.h.StartExit(id) {
			f.h.SetDraft(id, false)
			continue
		}
		c.C.Members = append(c.C.Members, uint64(id))
		c.Exiting = append(c.Exiting, id)
	}
	if len(c.C.Members) == 0 {
		f.h.Note("Nobody can reach the map edge.")
		return
	}
	*f.list = append(*f.list, c)
	f.h.Note(fmt.Sprintf("The caravan is forming: %d heading for the map edge.", len(c.C.Members)))
}

// ExitMap: pawn id reached the map edge on job.ExitMap. A forming
// caravan's member is stashed with it (the last one sets it out); false
// for anyone else (the caller ends the job).
func (f Fleet) ExitMap(id entity.EntityID) bool {
	c := f.Of(id)
	if c == nil || c.C.State != caravan.Forming || !slices.Contains(c.Exiting, id) {
		return false
	}
	cid := entity.EntityID(c.C.ID)
	if !f.h.Stash(id, cid) {
		return false
	}
	c = f.At(cid)
	c.Exiting = without(c.Exiting, id)
	if len(c.Exiting) == 0 {
		f.depart(c, f.h.Now())
	}
	return true
}

// CheckForming drops forming members who can no longer go (down, broken,
// undrafted, or too long on the way), re-sends those whose walk failed for
// want of a search slot, and sets out or cancels caravans with nobody left
// walking.
func (f Fleet) CheckForming(now uint64) {
	cargo, cur := f.h.Cargo(), f.h.Current()
	for i := 0; i < len(*f.list); i++ {
		c := &(*f.list)[i]
		if c.C.State != caravan.Forming || c.Map != cur || cargo.Gather(c.C.ID, c.Exiting, c.Formed, now) { // AX-9: waiting at the hitching spot
			continue
		}
		for _, id := range slices.Clone(c.Exiting) {
			j, _ := f.h.Job(id)
			switch {
			case !f.h.Standing(id) || f.h.Breaking(id) || !f.h.Drafted(id) || now-c.Formed >= formLimit:
			case j.Kind == job.ExitMap && j.Active():
				continue
			case j.Kind == job.ExitMap && j.State == job.Fail && j.Fail == job.Reserved && f.h.StartExit(id):
				continue // a deferred search: tried again
			case j.Kind != job.ExitMap && f.h.StartExit(id):
				continue // an order or a shot in between: back on the way
			}
			f.dropMember(c, id)
		}
		if len(c.Exiting) > 0 {
			continue
		}
		if len(c.C.Members) == 0 {
			f.h.Note("The caravan was called off: nobody left to go.")
			*f.list = slices.Delete(*f.list, i, i+1)
			i--
			continue
		}
		f.depart(c, now)
	}
}

// dropMember takes a forming member out of caravan c (it stays home).
func (f Fleet) dropMember(c *Caravan, id entity.EntityID) {
	c.Exiting = without(c.Exiting, id)
	c.C.Members = slices.DeleteFunc(c.C.Members, func(m uint64) bool { return m == uint64(id) })
	if f.h.Drafted(id) {
		f.h.SetDraft(id, false)
	}
}

// depart sets a formed caravan on its way: its pace (pace), its food,
// the departure letter.
func (f Fleet) depart(c *Caravan, now uint64) {
	var names []string
	for _, m := range c.C.Members {
		names = append(names, f.h.Name(entity.EntityID(m)))
	}
	f.pace(c)
	c.C.FoodDays = FoodDays
	c.C.State = caravan.Travelling
	c.NextTalk = now + talkPeriod
	f.h.Letter(false, "Caravan departed", fmt.Sprintf("%s set out for %s with %d days of food.",
		travel.JoinNames(names), f.h.TileName(c.C.Dest), FoodDays/1000), 0)
}

// pace sets c's speed inputs (MAP-09, wiki: Caravan): the slowest member
// still walking (the downed are carried), the load against what it can
// carry (game/travel), and its best mount when there is one per person.
func (f Fleet) pace(c *Caravan) {
	slowest := int32(1000)
	for _, m := range c.C.Members {
		if v := f.h.Capacity(entity.EntityID(m), body.Moving); v > 0 {
			slowest = min(slowest, v)
		}
	}
	c.C.Slowest = max(slowest, 1)
	c.C.Load = int32((1000 - min(max(f.h.Cargo().Pace(c.C.ID, len(c.C.Members)), 250), 1000)) * 1000 / 750)
	mounts, best := 0, int32(0)
	for _, a := range c.C.Animals {
		if s, ok := f.h.Species(entity.EntityID(a)); ok && caravan.MountFactor(s) > 0 {
			mounts, best = mounts+1, max(best, caravan.MountFactor(s))
		}
	}
	c.C.Mount = 0
	if mounts > 0 && mounts >= len(c.C.Members) {
		c.C.Mount = best
	}
}

// Return sends caravan id home: a forming one is called off (its members
// come back in), one on the road turns round.
func (f Fleet) Return(id entity.EntityID) {
	c := f.At(id)
	if c == nil {
		return
	}
	switch c.C.State {
	case caravan.Forming:
		for _, m := range slices.Clone(c.Exiting) {
			f.dropMember(c, m)
		}
		if len(c.C.Members) > 0 && c.Map != f.h.Current() {
			f.enterMap(c, c.Map) // MAP-01: back onto the map it formed on, at the tick's end
			return
		}
		if len(c.C.Members) > 0 {
			f.Home(c)
		}
		f.drop(id)
	case caravan.Travelling, caravan.Arrived, caravan.Visiting:
		f.startReturn(c)
		if c = f.At(id); c != nil && c.C.State == caravan.Home {
			f.drop(id)
		}
	}
}

// without is ids less id, in a new array.
func without(ids []entity.EntityID, id entity.EntityID) []entity.EntityID {
	i := slices.Index(ids, id)
	if i < 0 {
		return ids
	}
	out := make([]entity.EntityID, 0, len(ids)-1)
	return append(append(out, ids[:i]...), ids[i+1:]...)
}
