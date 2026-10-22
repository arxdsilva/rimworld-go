package game

import (
	"sort"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Creature and food indexes (SIM-024): bucket grids so an animal's
// questions ("the nearest threat within 5", "prey within 22", "grass or a
// carcass within 30") look at the few buckets around it instead of every
// creature, item and plant on the map. Both are rebuilt from the
// authoritative lists, never edited in place, so they hold no state of
// their own: the creature index once per tick (phaseAnimals, or on the
// first query of a tick), the food index on the first food search of a
// tick. Queries confirm every hit against the live stores, and search one
// bucket ring wider than asked, so a creature that stepped (one tile at
// most) since the rebuild is still found where it now stands.

// idxBucket is the side of one index bucket in tiles.
const idxBucket = 8

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

// bucketGrid is a compressed bucket list: bucket b's entries are
// ents[start[b]:start[b+1]].
type bucketGrid[T any] struct {
	bw, bh int
	start  []int32
	ents   []T
	tmp    []int32 // scratch: each input's bucket, then the fill cursors
}

func (b *bucketGrid[T]) reset(w, h int) {
	b.bw, b.bh = (w+idxBucket-1)/idxBucket, (h+idxBucket-1)/idxBucket
	n := b.bw*b.bh + 1
	if cap(b.start) < n {
		b.start = make([]int32, n)
	}
	b.start = b.start[:n]
	clear(b.start)
	b.ents = b.ents[:0]
}

func (b *bucketGrid[T]) bucket(x, y int) int {
	bx, by := clampInt(x/idxBucket, 0, b.bw-1), clampInt(y/idxBucket, 0, b.bh-1)
	return by*b.bw + bx
}

// fill builds the grid from n items whose bucket is at(i) and whose entry
// is ent(i), in two passes (count, place) keeping each bucket in input
// order.
func (b *bucketGrid[T]) fill(n int, at func(i int) int, ent func(i int) T) {
	if cap(b.tmp) < n+len(b.start) {
		b.tmp = make([]int32, n+len(b.start))
	}
	buckets, next := b.tmp[:n], b.tmp[n:n+len(b.start)-1]
	for i := 0; i < n; i++ {
		bi := at(i)
		buckets[i] = int32(bi)
		if bi >= 0 {
			b.start[bi+1]++
		}
	}
	for i := 1; i < len(b.start); i++ {
		b.start[i] += b.start[i-1]
	}
	total := int(b.start[len(b.start)-1])
	if cap(b.ents) < total {
		b.ents = make([]T, total)
	}
	b.ents = b.ents[:total]
	copy(next, b.start[:len(b.start)-1])
	for i := 0; i < n; i++ {
		if bi := buckets[i]; bi >= 0 {
			b.ents[next[bi]] = ent(i)
			next[bi]++
		}
	}
}

// each calls fn for every entry in the buckets overlapping the square of
// Chebyshev radius r around (x, y); fn returns false to stop.
func (b *bucketGrid[T]) each(x, y, r int, fn func(T) bool) {
	if b.bw == 0 {
		return
	}
	bx0, by0 := clampInt((x-r)/idxBucket, 0, b.bw-1), clampInt((y-r)/idxBucket, 0, b.bh-1)
	bx1, by1 := clampInt((x+r)/idxBucket, 0, b.bw-1), clampInt((y+r)/idxBucket, 0, b.bh-1)
	for by := by0; by <= by1; by++ {
		for bx := bx0; bx <= bx1; bx++ {
			bi := by*b.bw + bx
			for _, e := range b.ents[b.start[bi]:b.start[bi+1]] {
				if !fn(e) {
					return
				}
			}
		}
	}
}

// creatureIndex is the per-tick creature grid.
type creatureIndex struct {
	grid  bucketGrid[idxEntry]
	ok    bool
	at    uint64 // tick it was built
	count int    // creatures it was built over (a spawn or removal rebuilds)
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
	ix.grid.reset(g.grid.Width(), g.grid.Height())
	np, nr := len(g.pawnIDs), len(g.raiderIDs)
	idOf := func(i int) (entity.EntityID, uint8) {
		switch {
		case i < np:
			return g.pawnIDs[i], ckPawn
		case i < np+nr:
			return g.raiderIDs[i-np], ckRaider
		}
		return g.animalIDs[i-np-nr], ckAnimal
	}
	ix.grid.fill(g.creatureCount(), func(i int) int {
		id, _ := idOf(i)
		p, ok := g.positions.Get(id)
		if !ok {
			return -1
		}
		return ix.grid.bucket(p.X, p.Y)
	}, func(i int) idxEntry {
		id, k := idOf(i)
		return idxEntry{id: id, kind: k}
	})
	ix.ok, ix.at, ix.count = true, uint64(g.clock.Now()), g.creatureCount()
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
	g.creatures().grid.each(x, y, r+1, func(e idxEntry) bool {
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
	g.creatures().grid.each(x, y, r+1, func(e idxEntry) bool {
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
	items  bucketGrid[entity.EntityID]
	plants bucketGrid[entity.EntityID]
	ok     bool
	at     uint64
	ni, np int
}

// food is the food index, rebuilt once per tick (and when an item or plant
// list changed size) on first use.
func (g *Game) food() *foodIndex {
	fx := &g.wl.food
	if fx.ok && fx.at == uint64(g.clock.Now()) && fx.ni == len(g.itemIDs) && fx.np == len(g.plantIDs) {
		return fx
	}
	w, h := g.grid.Width(), g.grid.Height()
	fx.items.reset(w, h)
	fx.items.fill(len(g.itemIDs), func(i int) int {
		st, ok := g.stacks.Get(g.itemIDs[i])
		if !ok || st.Carrier != 0 {
			return -1
		}
		return fx.items.bucket(st.X, st.Y)
	}, func(i int) entity.EntityID { return g.itemIDs[i] })
	fx.plants.reset(w, h)
	fx.plants.fill(len(g.plantIDs), func(i int) int {
		p, ok := g.plants.Get(g.plantIDs[i])
		if !ok {
			return -1
		}
		return fx.plants.bucket(p.X, p.Y)
	}, func(i int) entity.EntityID { return g.plantIDs[i] })
	fx.ok, fx.at, fx.ni, fx.np = true, uint64(g.clock.Now()), len(g.itemIDs), len(g.plantIDs)
	return fx
}
