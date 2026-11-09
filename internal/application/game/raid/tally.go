package raid

import "rimworld-go/internal/domain/sim/entity"

// tally is the raiders on their feet, counted once a tick (EAI-020): each
// group's standing raiders (broken: the flee share) and the standing
// raiders of factions with whether rival ones share the map (rival). Every
// raider used to count its whole group, and scan every raider for a
// rival, each tick. One pass over RaiderIDs on the tick's first ask, again
// when a raider or a group comes or goes; one downed later in the same
// tick counts from the next. Flight stays one way.
type tally struct {
	at     uint64
	ok     bool
	n      int   // len(RaiderIDs) it was taken over
	stand  []int // standing raiders per group
	fac    []facRaider
	rivals bool
}

// facRaider is a standing raider of faction (1 + its index).
type facRaider struct {
	id      entity.EntityID
	faction int
}

// count is the tally at now.
func (rs *Raiders) count(now uint64) *tally {
	h, t := rs.h, &rs.tl
	ids, groups := h.RaiderIDs(), h.Groups()
	if t.ok && t.at == now && t.n == len(ids) && len(t.stand) == len(groups) {
		return t
	}
	t.at, t.ok, t.n, t.rivals = now, true, len(ids), false
	if cap(t.stand) < len(groups) {
		t.stand = make([]int, len(groups))
	}
	t.stand = t.stand[:len(groups)]
	clear(t.stand)
	t.fac = t.fac[:0]
	first := 0
	for _, id := range ids {
		r := h.Raider(id)
		if r.Group < 0 || r.Group >= len(groups) || !h.StandingRaider(id) {
			continue
		}
		t.stand[r.Group]++
		f := groups[r.Group].Faction
		if f == 0 {
			continue
		}
		t.fac = append(t.fac, facRaider{id: id, faction: f})
		switch {
		case rs.m == nil || f == first || t.rivals:
		case first == 0:
			first = f
		case rs.m.Rival(first, f):
			t.rivals = true
		}
	}
	return t
}
