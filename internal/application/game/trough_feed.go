package game

import (
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Trough feeding (SIM-024, plan 2.5), the simulation side of troughs.go.
// Haulers (Haul work, fillTroughs) carry filter-matching feed to troughs
// under half full as an ordinary haul whose destination is the trough's
// own tile; the load is cut to what the trough has room for when it is
// picked up, and what lands on the tile goes into the trough the same tick
// (troughDeliveries), off the map, like a campfire's fuel. Tame animals
// whose pen holds a trough eat from it once the pen's grass is gone.

const (
	// troughFillBelow: haulers top up a trough holding less than this.
	troughFillBelow = construction.TroughCapacity / 2
	// troughSweepEvery: troughs adopt their default filter and take in feed
	// lying on them this often (a trough just built, a load from a save).
	troughSweepEvery = 250
)

// isTrough reports a standing feeding trough.
func (g *Game) isTrough(id entity.EntityID) (construction.Building, bool) {
	if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.FeedingTrough {
		return *b, true
	}
	return construction.Building{}, false
}

// troughFeeds reports that a trough set to filter f takes item kind k.
func troughFeeds(f uint8, k item.Kind) bool {
	plants := animal.PlantFeed(k) || k == item.Straw
	switch f {
	case TroughPlants:
		return plants
	case TroughMeat:
		return k == item.Meat
	case TroughAny:
		return plants || k == item.Meat
	}
	return false
}

// troughPen is the enclosed pen trough b serves (pens.go troughMarker):
// the one holding its tile or a neighbouring cell (lowest marker first).
func (g *Game) troughPen(b construction.Building) (entity.EntityID, bool) {
	m, _ := g.troughMarker(b)
	return m, m != 0
}

// trough is trough id's state, created on first use with the filter its
// pen's animals want: meat when they all eat only meat, any when the pen
// mixes plant and meat eaters, else plants.
func (g *Game) trough(id entity.EntityID) troughState {
	if t, ok := g.troughs[id]; ok {
		return t
	}
	t := troughState{Filter: TroughPlants}
	if b, ok := g.isTrough(id); ok {
		if pen, ok := g.troughPen(b); ok {
			plants, meat := false, false
			for _, aid := range g.animalIDs {
				a, ok := g.animals.Get(aid)
				if !ok || !a.Tame {
					continue
				}
				p, _ := g.positions.Get(aid)
				if g.inPenCells(g.penRegion(pen), p.X, p.Y) {
					d := animal.EcologyOf(a.Species).Diet
					plants = plants || d.Plants()
					meat = meat || d == animal.Carnivore
				}
			}
			switch {
			case meat && plants:
				t.Filter = TroughAny
			case meat:
				t.Filter = TroughMeat
			}
		}
	}
	if g.troughs == nil {
		g.troughs = map[entity.EntityID]troughState{}
	}
	g.troughs[id] = t
	return t
}

// troughRoom is how many more feed items trough id takes.
func (g *Game) troughRoom(id entity.EntityID) int {
	return max(0, construction.TroughCapacity-g.troughFill(id))
}

// absorbInto puts the feed of stack sid (lying on trough id's tile) into
// the trough, as much as it takes and its filter allows.
func (g *Game) absorbInto(id, sid entity.EntityID) {
	st, ok := g.stacks.Get(sid)
	if !ok || st.Carrier != 0 {
		return
	}
	t := g.trough(id)
	n := min(st.Qty, g.troughRoom(id))
	if n <= 0 || !troughFeeds(t.Filter, st.Kind) || st.Rotten() {
		return
	}
	if t.Food == nil {
		t.Food = map[item.Kind]int{}
	}
	t.Food[st.Kind] += n
	g.troughs[id] = t
	g.takeItems(sid, st, n)
}

// troughDeliveries runs once a tick (phaseAnimals): a hauler bound for a
// trough carries no more than it takes, and the load it put down goes
// into the trough. Every troughSweepEvery ticks each trough also adopts
// its default filter and takes in any feed lying on it.
func (g *Game) troughDeliveries(now uint64) {
	for _, pid := range g.pawnIDs {
		j, _ := g.jobs.Get(pid)
		if j.Kind != job.Haul || j.Dest == 0 || len(j.Route) != 0 {
			continue
		}
		b, ok := g.isTrough(j.Dest)
		if !ok {
			continue
		}
		switch {
		case j.State == job.Complete:
			if sid := g.itemAt(b.X, b.Y); sid != 0 {
				g.absorbInto(j.Dest, sid)
			}
		case j.Carry != 0:
			g.trimTroughLoad(pid, j)
		}
	}
	if now%troughSweepEvery != 0 {
		return
	}
	for _, bid := range g.bldgIDs {
		b, ok := g.isTrough(bid)
		if !ok {
			continue
		}
		g.trough(bid)
		if sid := g.itemAt(b.X, b.Y); sid != 0 {
			g.absorbInto(bid, sid)
		}
	}
}

// trimTroughLoad cuts what hauler pid carries for a trough down to the
// room the trough has; the rest goes back down where it stands.
func (g *Game) trimTroughLoad(pid entity.EntityID, j job.Record) {
	st, ok := g.stacks.Get(j.Carry)
	room := g.troughRoom(j.Dest)
	if !ok || st.Qty <= max(room, 1) {
		return
	}
	pos, _ := g.positions.Get(pid)
	rest := st
	rest.Carrier, rest.X, rest.Y, rest.Qty = 0, pos.X, pos.Y, st.Qty-max(room, 1)
	st.Qty = max(room, 1)
	g.stacks.Set(j.Carry, st)
	rid := g.entities.Next()
	g.itemIDs = append(g.itemIDs, rid)
	g.mergeOrSet(rid, rest)
}

// fillTroughs is the Haul work giver for troughs: the emptiest trough
// under half full gets the nearest free feed its filter takes.
func (g *Game) fillTroughs(id entity.EntityID) bool {
	type cand struct {
		bid  entity.EntityID
		fill int
	}
	var low []cand
	for _, bid := range g.bldgIDs {
		b, ok := g.isTrough(bid)
		if !ok || b.Deconstruct {
			continue
		}
		if fill := g.troughFill(bid); fill < troughFillBelow {
			if owner, taken := g.reserve.TileOwner(b.X, b.Y); (!taken || owner == id) && !g.postponedFor(id, job.Haul, bid, 0, 0) {
				low = append(low, cand{bid, fill}) // SIM-027: not one put off lately
			}
		}
	}
	if len(low) == 0 {
		return false
	}
	sort.Slice(low, func(i, j int) bool {
		return low[i].fill < low[j].fill || low[i].fill == low[j].fill && low[i].bid < low[j].bid
	})
	for _, c := range low {
		b, _ := g.isTrough(c.bid)
		f := g.trough(c.bid).Filter
		var best entity.EntityID
		var bst item.Stack
		bestD := 1 << 30
		for _, sid := range g.itemIDs {
			sp := g.stacks.Ref(sid)
			if sp == nil || sp.Carrier != 0 || sp.Forbidden || sp.Qty <= 0 || sp.Rotten() || !troughFeeds(f, sp.Kind) {
				continue
			}
			st := *sp // a candidate (the loop only reads)
			if st.X == b.X && st.Y == b.Y {
				continue // already lying on the trough
			}
			if _, taken := g.reserve.EntityOwner(sid); taken {
				continue
			}
			if g.postponedFor(id, job.Haul, sid, 0, 0) {
				continue // SIM-027: out of reach lately
			}
			if d := pathing.Dist(b.X, b.Y, st.X, st.Y); d < bestD || d == bestD && sid < best {
				best, bst, bestD = sid, st, d
			}
		}
		if best == 0 {
			continue
		}
		rec := job.Record{Kind: job.Haul, Target: best, TX: bst.X, TY: bst.Y, Dest: c.bid, DestX: b.X, DestY: b.Y}
		if g.startJob(id, rec) {
			return true
		}
	}
	return false
}

// penTrough is the nearest trough of pen holding feed for diet, as a feed
// target.
func (g *Game) penTrough(a *animal.State, pen entity.EntityID, pos Position, diet animal.Diet) (animal.FeedTarget, bool) {
	var best animal.FeedTarget
	bestD, found := 1<<30, false
	for _, bid := range g.bldgIDs {
		b, ok := g.isTrough(bid)
		if !ok {
			continue
		}
		if p, ok := g.troughPen(b); !ok || p != pen || !g.troughHas(bid, diet) {
			continue
		}
		if d := weapon.Distance(pos.X, pos.Y, b.X, b.Y); d < bestD {
			best, bestD, found = animal.FeedTarget{Kind: animal.FeedTrough, ID: bid, X: b.X, Y: b.Y}, d, true
		}
	}
	return best, found
}

// troughHas reports trough id holding feed an animal of diet eats.
func (g *Game) troughHas(id entity.EntityID, diet animal.Diet) bool {
	if _, ok := g.isTrough(id); !ok {
		return false
	}
	for k, q := range g.troughs[id].Food {
		if q > 0 && animal.FeedValue(diet, k) > 0 {
			return true
		}
	}
	return false
}

// eatFromTrough feeds an animal of species s from trough id, up to want
// food, kind by kind in kind order; it returns the food gained.
func (g *Game) eatFromTrough(id entity.EntityID, s animal.Species, diet animal.Diet, want int) int {
	t, ok := g.troughs[id]
	if !ok {
		return 0
	}
	kinds := make([]item.Kind, 0, len(t.Food))
	for k := range t.Food {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	gained := 0
	for _, k := range kinds {
		per := animal.Meal(s, animal.FeedValue(diet, k))
		for per > 0 && t.Food[k] > 0 && gained < want {
			t.Food[k]--
			gained += per
		}
		if t.Food[k] <= 0 {
			delete(t.Food, k)
		}
	}
	g.troughs[id] = t
	return gained
}
