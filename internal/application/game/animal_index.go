package game

import (
	"sort"

	"rimworld-go/internal/application/game/spatial"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Creature and food indexes (SIM-024): bucket grids so an animal's
// questions ("the nearest threat within 5", "prey within 22", "grass or a
// carcass within 30") look at the few buckets around it instead of every
// creature, item and plant on the map. Both are rebuilt from the
// authoritative lists, never edited in place, so they hold no state of
// their own: the creature index on the first query of a tick (again on
// an arrival), the food index on the first food search of a tick.
// Queries confirm every hit against the live stores, and search one
// bucket ring wider than asked, so a creature that stepped (one tile at
// most) since the rebuild is still found where it now stands.

// Creature kinds in the index (a query asks for a mask of them).
const (
	ckPawn   uint8 = 1 << iota // a colonist
	ckRaider                   // a raider
	ckAnimal                   // an animal
	ckAll    = ckPawn | ckRaider | ckAnimal
)

type idxEntry struct {
	id   entity.EntityID
	kind uint8
}

// creatureIndex is the per-tick creature grid.
type creatureIndex struct {
	grid   spatial.Grid[idxEntry]
	ok     bool
	at     uint64 // tick it was built
	count  int    // creatures it was built over (a spawn or removal rebuilds)
	builds uint64 // rebuilds so far (tests)
}

// creatureCount is how many creatures the lists hold now.
func (g *Game) creatureCount() int { return len(g.pawnIDs) + len(g.raiderIDs) + len(g.animalIDs) }

// creatures is the creature index, rebuilt when it is from an earlier tick
// or the creature lists changed size since.
func (g *Game) creatures() *creatureIndex {
	ix := &g.wl.idx
	if !ix.ok || ix.at != uint64(g.clock.Now()) || ix.count != g.creatureCount() {
		g.rebuildCreatures()
	}
	return ix
}

// rebuildCreatures indexes every pawn, raider and animal by position.
func (g *Game) rebuildCreatures() {
	ix := &g.wl.idx
	ix.grid.Reset(g.grid.Width(), g.grid.Height())
	n := g.creatureCount()
	ix.grid.Begin(n)
	lists := [3][]entity.EntityID{g.pawnIDs, g.raiderIDs, g.animalIDs}
	i := 0
	for _, ids := range lists {
		for _, id := range ids {
			ix.grid.Add(i, bucketOf(g, &ix.grid, id))
			i++
		}
	}
	ix.grid.Seal()
	i = 0
	for k, ids := range lists {
		kind := ckPawn << k // ckPawn, ckRaider, ckAnimal
		for _, id := range ids {
			ix.grid.Put(i, idxEntry{id: id, kind: kind})
			i++
		}
	}
	ix.ok, ix.at, ix.count = true, uint64(g.clock.Now()), n
	ix.builds++
}

// bucketOf is the bucket of creature id's tile in b (-1: no position).
func bucketOf[T any](g *Game, b *spatial.Grid[T], id entity.EntityID) int {
	p, ok := g.positions.Get(id)
	if !ok {
		return -1
	}
	return b.Bucket(p.X, p.Y)
}

// invalidateCreatures drops the index (a creature appeared mid-tick).
func (g *Game) invalidateCreatures() { g.wl.idx.ok = false }

// nearestWhere is the creature of a kind in kinds nearest to (x, y) within
// Chebyshev radius r that keep accepts, with its position; ties go to the
// lower id.
func (g *Game) nearestWhere(x, y, r int, kinds uint8, keep func(id entity.EntityID, p Position) bool) (entity.EntityID, Position, bool) {
	var best entity.EntityID
	var bp Position
	bestD := r + 1
	g.creatures().grid.Each(x, y, r+1, func(e idxEntry) bool {
		if e.kind&kinds == 0 {
			return true
		}
		p, ok := g.positions.Get(e.id)
		if !ok {
			return true
		}
		d := weapon.Distance(x, y, p.X, p.Y)
		if d > r || d > bestD || d == bestD && e.id > best {
			return true
		}
		if keep == nil || keep(e.id, p) {
			best, bp, bestD = e.id, p, d
		}
		return true
	})
	return best, bp, best != 0
}

// within calls fn, in ascending id order, for every creature of a kind in
// kinds within Chebyshev radius r of (x, y).
func (g *Game) within(x, y, r int, kinds uint8, fn func(id entity.EntityID, p Position)) {
	type hit struct {
		id entity.EntityID
		p  Position
	}
	var hits []hit
	g.creatures().grid.Each(x, y, r+1, func(e idxEntry) bool {
		if e.kind&kinds == 0 {
			return true
		}
		if p, ok := g.positions.Get(e.id); ok && weapon.Distance(x, y, p.X, p.Y) <= r {
			hits = append(hits, hit{e.id, p})
		}
		return true
	})
	sort.Slice(hits, func(i, j int) bool { return hits[i].id < hits[j].id })
	for _, h := range hits {
		fn(h.id, h.p)
	}
}

// foodIndex is the per-tick grid of loose items and plants food searches
// look in.
type foodIndex struct {
	items  spatial.Grid[entity.EntityID]
	plants spatial.Grid[entity.EntityID]
	ok     bool
	at     uint64
	ni, np int
	builds uint64 // rebuilds so far (tests)
	// the plant grid's own key (plants never move: see food)
	plantsOK       bool
	plantGen       uint64
	plantSrc       any
	plantW, plantH int
}

// food is the food index, rebuilt once per tick (and when an item or plant
// list changed size) on first use.
func (g *Game) food() *foodIndex {
	fx := &g.wl.food
	if fx.ok && fx.at == uint64(g.clock.Now()) && fx.ni == len(g.itemIDs) && fx.np == len(g.plantIDs) {
		return fx
	}
	w, h := g.grid.Width(), g.grid.Height()
	fx.items.Reset(w, h)
	fx.items.Begin(len(g.itemIDs))
	for i, id := range g.itemIDs {
		bi := -1
		if st, ok := g.stacks.Get(id); ok && st.Carrier == 0 {
			bi = fx.items.Bucket(st.X, st.Y)
		}
		fx.items.Add(i, bi)
	}
	fx.items.Seal()
	for i, id := range g.itemIDs {
		fx.items.Put(i, id)
	}
	// Plants never move: the plant grid is rebuilt only when the plant
	// list changed since (or the store was replaced: Load), which is the
	// grid a rebuild would make anyway.
	if !fx.plantsOK || fx.plantGen != g.idx.jobs.plantGen || fx.plantSrc != g.plants || fx.plantW != w || fx.plantH != h {
		g.fillFoodPlants(&fx.plants, w, h)
		fx.plantsOK, fx.plantGen, fx.plantSrc, fx.plantW, fx.plantH = true, g.idx.jobs.plantGen, g.plants, w, h
	} else if checkIndex(g) {
		var fresh spatial.Grid[entity.EntityID]
		g.fillFoodPlants(&fresh, w, h)
		if !spatial.Same(&fresh, &fx.plants) {
			panic("food index: the plant list changed without plantsChanged")
		}
	}
	fx.ok, fx.at, fx.ni, fx.np = true, uint64(g.clock.Now()), len(g.itemIDs), len(g.plantIDs)
	fx.builds++
	return fx
}

// fillFoodPlants buckets every plant, in list order.
func (g *Game) fillFoodPlants(b *spatial.Grid[entity.EntityID], w, h int) {
	b.Reset(w, h)
	b.Begin(len(g.plantIDs))
	for i, id := range g.plantIDs {
		bi := -1
		if p, ok := g.plants.Get(id); ok {
			bi = b.Bucket(p.X, p.Y)
		}
		b.Add(i, bi)
	}
	b.Seal()
	for i, id := range g.plantIDs {
		b.Put(i, id)
	}
}
