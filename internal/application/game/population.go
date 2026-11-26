// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"sort"

	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/temperature"
)

// Wildlife uses a biome/area weight budget and hysteretic migration.
// Species weights differ; hunting and hunger change the occupied budget.

const (
	// capLowPct and capHighPct bound migration hysteresis.
	capLowPct  = 70
	capHighPct = 130
	// arriveOdds / leaveOdds: 1 in this many per hour once past a threshold.
	arriveOdds = 24
	leaveOdds  = 12
	// Under urgent density, arrival checks and settling run more often.
	capUrgentPct = 60
	urgentOdds   = 3
	urgentSettle = 2 * clock.TicksPerHour
	// PopSettle prevents rapid arrival/departure reversals.
	popSettle = 6 * clock.TicksPerHour
	// startCapPct: a new colony starts with this share of capacity.
	startCapPct = animal.StartCapPct
	// Winter capacity by biome average temperature.
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

// wildlifeCap is the biome/area ecosystem budget in hundredths, including
// the existing winter and volcanic-weather reductions.
func (g *Game) wildlifeCap() int {
	bd := biome.DefOf(g.biome)
	c := animal.EcosystemCapacity(bd.AnimalDensity100, g.grid.Width()*g.grid.Height())
	if temperature.SeasonPolicy(g.clock.Now(), g.clock.Calendar()) == "Winter" {
		pct := winterMildPct
		if bd.Climate.YearMean <= 0 {
			pct = winterColdPct
		}
		c = c * pct / 100
	}
	return c * g.incidents.WildlifePermil(uint64(g.clock.Now())) / 1000 // TEMP-2: a volcanic winter halves it (arrivals and the start target follow)
}

// wildCount caches ecosystem weight within an hour, converting headcount
// caches from old saves on first access.
func (g *Game) wildCount() int {
	now := uint64(g.clock.Now())
	if !g.wl.WeightSchema || g.wl.CountHour != now/clock.TicksPerHour+1 {
		g.recountWild(now)
	}
	return g.wl.Count
}

// recountWild totals live wildlife and predator weights for this hour.
func (g *Game) recountWild(now uint64) int {
	n, preds := 0, 0
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && animal.CountsWild(a, now) && !foreignPackAnimal(g, id) {
			w := animal.EcosystemWeight(a.Species)
			n += w
			if isPredator(a.Species) {
				preds += w
			}
		}
	}
	g.wl.Count, g.wl.Preds, g.wl.CountHour, g.wl.WeightSchema = n, preds, now/clock.TicksPerHour+1, true
	return n
}

// populationHour is the hourly capacity check (wildlife.go migrate).
func (g *Game) populationHour(now uint64) {
	capacity := g.wildlifeCap()
	n := g.recountWild(now)
	if capacity <= 0 || n > 0 && now < g.wl.PopSettle {
		return
	}
	switch {
	case n*100 < capacity*capLowPct:
		odds, settle := arriveOdds, uint64(popSettle)
		if n*100 < capacity*capUrgentPct {
			odds, settle = urgentOdds, urgentSettle
		}
		if g.rng.IntN(odds) != 0 && n > 0 {
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

// wildArrival brings in one group of at most room animals at an edge the
// colony can be reached from, given n wild animals with preds predators;
// it returns how many came and whether they are predators.
func (g *Game) wildArrival(room, n, preds int) (int, bool) {
	if room <= 0 {
		return 0, false
	}
	// Keep arrivals within the same hysteresis band whenever one animal fits.
	// An empty map may admit one indivisible animal even when no species fits.
	upper := (g.wildlifeCap()*capHighPct)/100 - n
	s, ok := animal.PickEcosystemGroup(n, preds, upper, g.rng.IntN, g.pickSpecies)
	if !ok && n == 0 {
		s, ok = animal.PickEcosystemGroup(n, preds, -1, g.rng.IntN, g.pickSpecies)
	}
	if !ok {
		return 0, false
	}
	var hab *habitat
	var x, y int
	if g.hasSetup() {
		hab = g.newHabitat()       // wildlife_habitat.go
		x, y, ok = hab.PickEdge(s) // at a map edge, never mid-map
		if !ok {
			x, y, ok = g.edgeTile()
			hab = nil
		}
	} else {
		x, y, ok = g.edgeTile()
	}
	if !ok {
		return 0, false
	}
	weight := animal.EcosystemWeight(s)
	size := min(g.herdSizeOf(s), max(1, room/weight))
	if isPredator(s) {
		size = min(size, startPredatorMax)
	}
	return len(g.spawnHerdIn(hab, s, x, y, size)) * weight, isPredator(s)
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
		if !ok || a.Herd == 0 || !animal.CountsWild(a, now) {
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
		a, _ := g.animals.Get(ids[0])
		weight := len(ids) * animal.EcosystemWeight(a.Species)
		if n-weight < floor {
			continue
		}
		g.sendHerdOff(h, ids)
		return weight, isPredator(a.Species)
	}
	return 0, false
}

// sendHerdOff starts every member of herd h walking to the map edge
// nearest its first member, and keeps a departure to announce.
func (g *Game) sendHerdOff(h uint32, ids []entity.EntityID) {
	lead, _ := g.positions.Get(ids[0])
	ex, ey := nearestEdge(lead, g.grid.Width(), g.grid.Height())
	if x, y, ok := g.combat.Tiles.WalkableNear(ex, ey, 4); ok {
		ex, ey = x, y
	}
	a, _ := g.animals.Get(ids[0])
	d := departure{Herd: h, Species: a.Species, ExitX: ex, ExitY: ey}
	for _, id := range ids {
		p, _ := g.positions.Get(id)
		g.startLeaving(id, ex, ey)
		g.logEvent(id, logEntry{Kind: evlog.Leaving})
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
			g.message(d.ExitX, d.ExitY, "A group of %d %s has left the area.", d.Left, animal.Plural(d.Species))
		}
	}
	g.wl.Departs = kept
}

// startTarget is how many wild animals a new colony starts with.
func (g *Game) startTarget() int { return g.wildlifeCap() * startCapPct / 100 }
