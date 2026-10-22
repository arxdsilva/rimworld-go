package game

import (
	"sort"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/temperature"
)

// Wild population (SIM-025, plan 3.1). A map carries about as many wild
// animals as its biome supports: biome.Wildlife per 10,000 tiles, fewer
// in winter. Once an hour the count is checked against that capacity: well
// below it (capLowPct) a group of the biome's animals may wander in at a
// reachable edge; well above it (capHighPct) the newest herd may walk off
// the map. The gap between the two thresholds, groups trimmed to the room
// left under capacity, departures that never take the count below the low
// threshold and a settling time after either keep it from flapping.

const (
	// capLowPct / capHighPct: the count, percent of capacity, below which
	// animals arrive and above which a herd leaves.
	capLowPct  = 70
	capHighPct = 130
	// arriveOdds / leaveOdds: 1 in this many per hour once past a threshold.
	arriveOdds = 24
	leaveOdds  = 12
	// capUrgentPct: below this share of capacity (hunting and hunger have
	// thinned the land out) a group comes with 1 chance in urgentOdds an
	// hour, and the next may follow after urgentSettle.
	capUrgentPct = 60
	urgentOdds   = 3
	urgentSettle = 2 * clock.TicksPerHour
	// preyOdds of every predatorOdds+preyOdds arriving groups are prey;
	// predators come only while they are fewer than that share of the
	// wild count (they do not die off as prey does).
	preyOdds     = 6
	predatorOdds = 1
	// popSettle: after an arrival or a departure the count is left to
	// settle this long before the next one.
	popSettle = 6 * clock.TicksPerHour
	// startCapPct: a new colony starts with this share of capacity.
	startCapPct = 80
	// winterColdPct / winterMildPct: winter capacity of a biome with a
	// yearly mean at or below 0 °C, and of any other.
	winterColdPct = 60
	winterMildPct = 80
	// departSeen: a departing animal last seen this close to its exit and
	// gone has left the map (rather than died on the way).
	departSeen = 2
)

// departure is a herd walking off the map because the land is overfull;
// it is announced once every member has left or died (saved).
type departure struct {
	Herd         uint32
	Species      animal.Species
	IDs          []entity.EntityID
	Last         []Position // each member's last known tile
	ExitX, ExitY int
	Left         int // members that walked off so far
}

// wildlifeCap is how many wild animals the map supports now: the biome's
// density over the map's area, cut in winter.
func (g *Game) wildlifeCap() int {
	bd := biome.DefOf(g.biome)
	c := bd.Wildlife * g.grid.Width() * g.grid.Height() / 10000
	if temperature.Season(g.clock.Now()) == "Winter" {
		pct := winterMildPct
		if bd.Climate.YearMean <= 0 {
			pct = winterColdPct
		}
		c = c * pct / 100
	}
	return c
}

// countsWild: an animal counted against capacity. Tame animals, leavers
// (herd crossers among them) and event animals are not: manhunters, hive
// insects, legendaries and tree-eaters, which only come with events.
func countsWild(a animal.State, now uint64) bool {
	if a.Tame || a.Leaving || a.Enraged(now) || a.Dead() {
		return false
	}
	d := animal.DefOf(a.Species)
	return !d.Hive && !d.Legendary && animal.EcologyOf(a.Species).Diet != animal.Dendrovore
}

// wildCount is the number of wild animals counted against capacity. It is
// counted at most once an in-game hour (the hourly check refreshes it and
// keeps it current through its own arrivals and departures); the cached
// value is saved, so a loaded game reads the same count.
func (g *Game) wildCount() int {
	now := uint64(g.clock.Now())
	if g.wl.CountHour != now/clock.TicksPerHour+1 {
		g.recountWild(now)
	}
	return g.wl.Count
}

// recountWild counts the wild animals (and the predators among them) now
// and caches the counts for the hour.
func (g *Game) recountWild(now uint64) int {
	n, preds := 0, 0
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && countsWild(a, now) {
			n++
			if isPredator(a.Species) {
				preds++
			}
		}
	}
	g.wl.Count, g.wl.Preds, g.wl.CountHour = n, preds, now/clock.TicksPerHour+1
	return n
}

// populationHour is the hourly capacity check (wildlife.go migrate).
func (g *Game) populationHour(now uint64) {
	capacity := g.wildlifeCap()
	n := g.recountWild(now)
	if now < g.wl.PopSettle || capacity <= 0 {
		return
	}
	switch {
	case n*100 < capacity*capLowPct:
		odds, settle := arriveOdds, uint64(popSettle)
		if n*100 < capacity*capUrgentPct {
			odds, settle = urgentOdds, urgentSettle
		}
		if g.rng.IntN(odds) != 0 {
			return
		}
		if k, pred := g.wildArrival(capacity-n, n, g.wl.Preds); k > 0 {
			g.wl.Count += k
			if pred {
				g.wl.Preds += k
			}
			g.wl.PopSettle = now + settle
		}
	case n*100 > capacity*capHighPct:
		if g.rng.IntN(leaveOdds) != 0 {
			return
		}
		if k, pred := g.herdLeaves(n, capacity); k > 0 {
			g.wl.Count -= k
			if pred {
				g.wl.Preds -= k
			}
			g.wl.PopSettle = now + popSettle
		}
	}
}

// pickWildGroup draws an arriving group's species: prey preyOdds times in
// predatorOdds+preyOdds (always prey while the n wild animals already
// hold their share of preds predators), from the biome's season-weighted
// wildlife (the other kind when the biome has none of the drawn one).
func (g *Game) pickWildGroup(n, preds int) (animal.Species, bool) {
	first, second := isPrey, isPredator
	if g.rng.IntN(preyOdds+predatorOdds) < predatorOdds && preds*(preyOdds+predatorOdds) < n {
		first, second = isPredator, isPrey
	}
	if s, ok := g.pickSpecies(first); ok {
		return s, true
	}
	return g.pickSpecies(second)
}

// wildArrival brings in one group of at most room animals at an edge the
// colony can be reached from, given n wild animals with preds predators;
// it returns how many came and whether they are predators.
func (g *Game) wildArrival(room, n, preds int) (int, bool) {
	if room <= 0 {
		return 0, false
	}
	s, ok := g.pickWildGroup(n, preds)
	if !ok {
		return 0, false
	}
	x, y, ok := g.edgeTile()
	if !ok {
		return 0, false
	}
	size := min(g.herdSizeOf(s), room)
	if isPredator(s) {
		size = min(size, startPredatorMax)
	}
	return len(g.spawnHerd(s, x, y, size)), isPredator(s)
}

// herdLeaves sends the newest herd (highest Herd id) whose going keeps the
// count at or above the low threshold off the map at the edge nearest it;
// it returns how many set off and whether they are predators.
func (g *Game) herdLeaves(n, capacity int) (int, bool) {
	now := uint64(g.clock.Now())
	herds := map[uint32][]entity.EntityID{}
	var order []uint32
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok || a.Herd == 0 || !countsWild(a, now) {
			continue
		}
		if _, seen := herds[a.Herd]; !seen {
			order = append(order, a.Herd)
		}
		herds[a.Herd] = append(herds[a.Herd], id)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] > order[j] })
	floor := (capacity*capLowPct + 99) / 100
	for _, h := range order {
		ids := herds[h]
		if n-len(ids) < floor {
			continue
		}
		a, _ := g.animals.Get(ids[0])
		g.sendHerdOff(h, ids)
		return len(ids), isPredator(a.Species)
	}
	return 0, false
}

// sendHerdOff starts every member of herd h walking to the map edge
// nearest its first member, and keeps a departure to announce.
func (g *Game) sendHerdOff(h uint32, ids []entity.EntityID) {
	lead, _ := g.positions.Get(ids[0])
	ex, ey := nearestEdge(lead, g.grid.Width(), g.grid.Height())
	if x, y, ok := g.walkableNear(ex, ey, 4); ok {
		ex, ey = x, y
	}
	a, _ := g.animals.Get(ids[0])
	d := departure{Herd: h, Species: a.Species, ExitX: ex, ExitY: ey}
	for _, id := range ids {
		p, _ := g.positions.Get(id)
		g.startLeaving(id, ex, ey)
		g.logEvent(id, logEntry{Kind: logLeaving})
		d.IDs = append(d.IDs, id)
		d.Last = append(d.Last, p)
	}
	g.wl.Departs = append(g.wl.Departs, d)
}

// trackDepartures follows the departing herds every tick (a handful of
// animals): one that vanished beside its exit walked off; once a herd is
// all gone, its leaving is announced.
func (g *Game) trackDepartures() {
	if len(g.wl.Departs) == 0 {
		return
	}
	kept := g.wl.Departs[:0]
	for _, d := range g.wl.Departs {
		alive := d.IDs[:0]
		last := d.Last[:0]
		for i, id := range d.IDs {
			if p, ok := g.positions.Get(id); ok {
				if a, isAnimal := g.animals.Get(id); isAnimal && a.Leaving {
					alive, last = append(alive, id), append(last, p)
				}
				continue // still here (or turned back: no longer leaving)
			}
			if weapon.Distance(d.Last[i].X, d.Last[i].Y, d.ExitX, d.ExitY) <= departSeen {
				d.Left++
			}
		}
		d.IDs, d.Last = alive, last
		if len(d.IDs) > 0 {
			kept = append(kept, d)
			continue
		}
		switch {
		case d.Left == 1:
			g.message(d.ExitX, d.ExitY, "A %s has left the area.", d.Species.Label())
		case d.Left > 1:
			g.message(d.ExitX, d.ExitY, "A group of %d %s has left the area.", d.Left, pluralLabel(d.Species))
		}
	}
	g.wl.Departs = kept
}

// startTarget is how many wild animals a new colony starts with.
func (g *Game) startTarget() int { return g.wildlifeCap() * startCapPct / 100 }
