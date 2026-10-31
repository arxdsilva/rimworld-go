package craft

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/sim/entity"
)

// Layered dress plan (TEMP-4): the outfit a colonist dresses in for the
// day, one choice per layer slot. The game gathers the choices (what is
// on, bare, and the garments it could fetch) and keeps the hysteresis
// (its gain threshold and slot cooldowns, game/apparel.go); Best only
// searches.

// Option is one choice for a slot: Kind (0: bare) from stack Stack at
// (X, Y), Walk away; Stack 0 is what the colonist has on now, or bare.
type Option struct {
	Kind       item.Kind
	Stack      entity.EntityID
	X, Y, Walk int
	cold, heat int // the kind's insulation (Best fills them)
	occ        item.SlotSet
	cover      item.Cover
}

// Plan is one choice per slot and how it suits the day.
type Plan struct {
	Pick                [Slots]Option
	Naked               bool // torso or legs left uncovered
	Miss, Surplus, Walk int
	Off                 int // garments on now that it leaves bare
}

// Next is the nearest garment p still has to fetch (Stack 0: none).
func (p *Plan) Next() Option {
	var n Option
	for _, o := range p.Pick {
		if o.Stack != 0 && (n.Stack == 0 || o.Walk < n.Walk) {
			n = o
		}
	}
	return n
}

// better orders plans: covered before naked (wiki: the Naked thought),
// then less discomfort, fewer garments taken off (no shirt comes off
// just to spare insulation under a parka), less surplus insulation, a
// shorter walk.
func (p *Plan) better(b *Plan) bool {
	switch {
	case p.Naked != b.Naked:
		return !p.Naked
	case p.Miss != b.Miss:
		return p.Miss < b.Miss
	case p.Off != b.Off:
		return p.Off < b.Off
	case p.Surplus != b.Surplus:
		return p.Surplus < b.Surplus
	}
	return p.Walk < b.Walk
}

// planSlots are the slots Best chooses for; any other (the belt, worn
// only by order) keeps its first option.
var planSlots = [...]item.Slot{item.Body, item.Middle, item.SkinTorso, item.Legs, item.SkinLegs, item.Head}

// Best is the best plan from opts, one list per slot whose first entry is
// what is on now, for a colonist whose bare comfortable range (comfort
// shift included) is base, on a day of mean (tenths of a degree C). Two
// garments never share a slot they occupy (Outfit.Conflicts). Each
// garment counts at its kind's insulation (Normal, default material) so
// the optimistic plan over kinds bounds the full one. Ties keep the
// earlier choices: all first entries, what is on, wins a tie.
func Best(opts *[Slots][]Option, base exposure.Bands, mean int) Plan {
	for s := range opts {
		for i := range opts[s] {
			o := &opts[s][i]
			o.cold, o.heat = o.Kind.Insulation()
			o.occ, o.cover = o.Kind.Occupies(), o.Kind.Covers()
		}
	}
	sr := search{opts: opts, base: base, mean: mean}
	for s := range opts {
		if len(opts[s]) > 0 {
			sr.cur.Pick[s] = opts[s][0]
		}
	}
	keep := sr.cur.Pick
	sr.walk(0, 0, 0, 0, 0, 0, 0)
	if !sr.found { // what is on conflicts (never saved so): keep it
		sr.best = Plan{Pick: keep, Naked: true, Miss: 1 << 30}
	}
	return sr.best
}

// search is Best's depth-first walk over planSlots.
type search struct {
	opts      *[Slots][]Option
	base      exposure.Bands
	mean      int
	cur, best Plan
	found     bool
}

// walk picks slot planSlots[i] onwards, given the slots occupied, the
// regions covered, the insulation, the walk and the garments taken off of
// the picks so far.
func (sr *search) walk(i int, occ item.SlotSet, cover item.Cover, cold, heat, walk, off int) {
	if i == len(planSlots) {
		b := exposure.Bands{Min: sr.base.Min - cold, Max: sr.base.Max + heat}
		p := &sr.cur
		p.Naked = cover&(item.CoverTorso|item.CoverLegs) != item.CoverTorso|item.CoverLegs
		p.Miss, p.Surplus, p.Walk, p.Off = DayFit(b, sr.mean), DaySurplus(b, sr.mean), walk, off
		if !sr.found || p.better(&sr.best) {
			sr.best, sr.found = *p, true
		}
		return
	}
	s := planSlots[i]
	if len(sr.opts[s]) == 0 { // no choices: bare
		sr.cur.Pick[s] = Option{}
		sr.walk(i+1, occ, cover, cold, heat, walk, off)
		return
	}
	on := sr.opts[s][0].Kind != 0 // the first option is what is on
	for _, o := range sr.opts[s] {
		if o.occ&occ != 0 {
			continue // shares a slot with an earlier pick
		}
		took := off
		if on && o.Kind == 0 {
			took++
		}
		sr.cur.Pick[s] = o
		sr.walk(i+1, occ|o.occ, cover|o.cover, cold+o.cold, heat+o.heat, walk+o.Walk, took)
	}
}
