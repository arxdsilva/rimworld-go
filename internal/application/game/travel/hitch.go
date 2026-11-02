package travel

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// The caravan hitching spot (wiki: Caravan hitching spot): with one on
// the map, a forming caravan's members first gather around it and wait
// there; once all have come (or the wait runs out) they set off for the
// map edge together. With several spots each caravan picks one by its id
// (the wiki: one at random). Pack animals and loading are not modelled.
const (
	// hitchWait is how long the gathered members wait at the spot
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
// hitching spot: false with no spot on the map (it heads straight for the
// edge).
func (o hitching) Hitch(cid uint64, member entity.EntityID, i int) bool {
	g := o.s.hitch[cid]
	if g == nil {
		sp := o.spots()
		if len(sp) == 0 {
			return false
		}
		b := sp[cid%uint64(len(sp))]
		g = &Gather{X: b.X, Y: b.Y}
		if o.s.hitch == nil {
			o.s.hitch = map[uint64]*Gather{}
		}
		o.s.hitch[cid] = g
	}
	o.moveNear(member, g.X, g.Y, i)
	return true
}

// Gather is the forming check of caravan cid (formed at tick formed) with
// members still on the map: true while they gather and wait at its spot
// (the game leaves them be), false once they should head for the edge
// (or never had a spot).
func (o hitching) Gather(cid uint64, members []entity.EntityID, formed, now uint64) bool {
	g := o.s.hitch[cid]
	if g == nil {
		return false
	}
	done := func() bool { delete(o.s.hitch, cid); return false }
	if now-formed >= hitchLimit {
		return done()
	}
	all := true
	for i, id := range members {
		if !o.h.Standing(id) || !o.h.Drafted(id) {
			return done() // the game drops it
		}
		x, y := o.h.Pos(id)
		if max(abs(x-g.X), abs(y-g.Y)) <= hitchNear {
			continue
		}
		all = false
		if j := o.job(id); j.Kind != job.Move || !j.Active() {
			o.moveNear(id, g.X, g.Y, i)
		}
	}
	switch {
	case !all:
		g.Since = 0
	case g.Since == 0:
		g.Since = max(now, 1)
	case now-g.Since >= hitchWait:
		return done()
	}
	return true
}
