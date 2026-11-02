package travel

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// The caravan hitching spot (wiki: Caravan, Caravan hitching spot): a
// forming caravan's members gather at a hitching spot (with several, each
// caravan picks one by its id; the wiki: one at random), or with goods or
// pack animals to load and no spot, at the colony's drop spot (the wiki: a
// spot in the home area). The goods are carried there and the animals led
// there and tied (pack.go); the members load too while they wait. Once
// all members are there and everything is loaded they wait hitchWait,
// then set off for the map edge together; after hitchLimit they go with
// whatever is loaded.
const (
	// hitchWait is how long the gathered caravan waits once ready
	// (original tuning).
	hitchWait = clock.TicksPerHour
	// hitchLimit ends a gathering that takes longer (original: half the
	// game's caravan forming limit).
	hitchLimit = clock.TicksPerDay / 2
	// hitchNear is how close to the spot counts as there (tiles).
	hitchNear = 3
)

// spots is the hitching spots, in building order.
func (o hitching) spots() []construction.Building {
	var out []construction.Building
	for _, id := range o.h.BuildingIDs() {
		if b, ok := o.h.Building(id); ok && b.Kind == construction.HitchingSpot {
			out = append(out, b)
		}
	}
	return out
}

// Hitch sends member (the i-th) of forming caravan cid to the caravan's
// gathering spot: false when it has none (no hitching spot and nothing
// to pack: it heads straight for the edge).
func (o hitching) Hitch(cid uint64, member entity.EntityID, i int) bool {
	g := o.s.hitch[cid]
	if g == nil {
		x, y := o.h.Home()
		if sp := o.spots(); len(sp) > 0 {
			b := sp[cid%uint64(len(sp))]
			x, y = b.X, b.Y
		} else if o.s.packs[cid] == nil {
			return false
		}
		g = &Gather{X: x, Y: y}
		if o.s.hitch == nil {
			o.s.hitch = map[uint64]*Gather{}
		}
		o.s.hitch[cid] = g
	}
	o.moveNear(member, g.X, g.Y, i)
	return true
}

// Gather is the forming check of caravan cid (formed at tick formed) with
// members still on the map: true while they gather, load and wait at its
// spot (the game leaves them be), false once they should head for the
// edge (or never had a spot). Ending the gathering ties its packing off
// (the pack animals join the cargo).
func (o hitching) Gather(cid uint64, members []entity.EntityID, formed, now uint64) bool {
	g := o.s.hitch[cid]
	if g == nil {
		return false
	}
	pk := packing(o)
	done := func() bool { delete(o.s.hitch, cid); pk.finish(cid); return false }
	if now-formed >= hitchLimit {
		return done()
	}
	all := true
	for i, id := range members {
		if !o.h.Standing(id) || !o.h.Drafted(id) {
			return done() // the game drops it
		}
		j := o.job(id)
		if j.Kind == job.LoadPods && j.Active() {
			all = false // loading: back at the spot when done
			continue
		}
		x, y := o.h.Pos(id)
		if max(abs(x-g.X), abs(y-g.Y)) > hitchNear {
			all = false
			if j.Kind != job.Move || !j.Active() {
				o.moveNear(id, g.X, g.Y, i)
			}
			continue
		}
		if !j.Active() || j.Kind.IsIdle() {
			pk.assign(id, cid) // a waiting member loads too
		}
	}
	switch {
	case !all || !pk.loaded(cid):
		g.Since = 0
	case g.Since == 0:
		g.Since = max(now, 1)
	case now-g.Since >= hitchWait:
		return done()
	}
	return true
}
