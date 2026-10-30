package raid

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Thieves, burners and rival factions (DEF-4). wiki: Raid, human raiders
// may steal nearby loot and then flee the map; raiders may set wooden
// walls or crops on fire. A Steal raid goes for the nearest stack worth
// taking, picks it up and runs for the edge (dropping it where it falls);
// a Burn raid sets up to burnFires fires on wooden buildings and crops,
// then assaults. With nothing to steal or burn they assault like anyone.
// Raid groups of rival factions on the map at once fight each other
// (Mischief.Rival: the game's faction relations).

const (
	// stealReach and burnReach: how far a thief or a burner looks for its
	// mark; mischiefEvery how often one on its way looks again; burnFires
	// the fires a burner sets before it fights (original tuning).
	stealReach    = 60
	burnReach     = 40
	mischiefEvery = 120
	burnFires     = 3
)

// mischief runs a thief or a burner at (x, y). It reports whether it
// acted (false: it fights).
func (rs *Raiders) mischief(id entity.EntityID, r Raider, j job.Record, x, y int, now uint64) bool {
	if rs.m == nil || now < r.RetryAt {
		return false
	}
	switch rs.h.Groups()[r.Group].Plan {
	case Steal:
		return rs.steal(id, r, j, x, y, now)
	case Burn:
		return r.Deeds < burnFires && rs.burn(id, r, j, x, y, now)
	}
	return false
}

// steal: take the stack underfoot and run, else walk to the nearest one.
func (rs *Raiders) steal(id entity.EntityID, r Raider, j job.Record, x, y int, now uint64) bool {
	h := rs.h
	if j.Kind == job.Move && j.Active() && (now+uint64(id))%mischiefEvery != 0 {
		return true // on its way
	}
	stack, sx, sy, ok := rs.m.Loot(x, y, stealReach)
	if !ok {
		r.RetryAt = now + RaiderRetry*5
		h.SetRaider(id, r)
		return false
	}
	if sx != x || sy != y {
		if !h.StartJob(id, job.Record{Kind: job.Move, TX: sx, TY: sy}) {
			r.RetryAt = now + RaiderRetry
			h.SetRaider(id, r)
			return false
		}
		return true
	}
	k, n, ok := rs.m.Steal(id, stack)
	if !ok {
		return false
	}
	r.Loot, r.LootQty, r.Mode = k, n, Fleeing
	r.Deeds++
	h.SetRaider(id, r)
	h.Interrupt(id, job.Cancelled)
	text := fmt.Sprintf("%s stole %d %s and is running off with it.", h.Name(id), n, k)
	h.Message(x, y, text)
	rs.log(text)
	return true
}

// burn: set the mark alight once beside it, else walk up to it.
func (rs *Raiders) burn(id entity.EntityID, r Raider, j job.Record, x, y int, now uint64) bool {
	h := rs.h
	if j.Kind == job.Move && j.Active() && (now+uint64(id))%mischiefEvery != 0 {
		return true
	}
	tx, ty, ok := rs.m.Flammable(x, y, burnReach)
	reach := burnReach
	if ok {
		reach = weapon.Distance(x, y, tx, ty) - 1
	}
	if cx, cy, cok := rs.crop(x, y, reach); cok { // CBT-4: a nearer crop burns first
		tx, ty, ok = cx, cy, true
	}
	if !ok {
		r.RetryAt = now + RaiderRetry*5
		h.SetRaider(id, r)
		return false
	}
	if weapon.Distance(x, y, tx, ty) > 1 {
		wx, wy, ok := h.WalkableNear(tx, ty, 1)
		if !ok || !h.StartJob(id, job.Record{Kind: job.Move, TX: wx, TY: wy}) {
			r.RetryAt = now + RaiderRetry
			h.SetRaider(id, r)
			return false
		}
		return true
	}
	if !rs.m.Ignite(tx, ty) {
		r.RetryAt = now + RaiderRetry
		h.SetRaider(id, r)
		return false
	}
	r.Deeds++
	r.RetryAt = now + RaiderRetry // a moment before the next mark
	h.SetRaider(id, r)
	if r.Deeds == 1 {
		text := fmt.Sprintf("%s is setting the colony on fire!", h.Name(id))
		h.Message(tx, ty, text)
		rs.log(text)
	}
	return true
}

// rivalry refreshes, once a tick, whether raid groups of rival factions
// stand on the map together (one pass over the raiders).
func (rs *Raiders) rivalry(now uint64) bool {
	if rs.rivalAt == now && rs.rivalOK {
		return rs.rivals
	}
	rs.rivalAt, rs.rivalOK, rs.rivals = now, true, false
	if rs.m == nil {
		return false
	}
	h := rs.h
	groups := h.Groups()
	first := 0
	for _, id := range h.RaiderIDs() {
		r := h.Raider(id)
		if r.Group >= len(groups) || !h.StandingRaider(id) {
			continue
		}
		f := groups[r.Group].Faction
		switch {
		case f == 0 || f == first:
		case first == 0:
			first = f
		case rs.m.Rival(first, f):
			rs.rivals = true
			return true
		}
	}
	return false
}

// rival is the nearest standing raider of a rival faction within sight of
// (x, y) and closer than best tiles (ok false: none).
func (rs *Raiders) rival(r Raider, x, y, best int, now uint64) (entity.EntityID, int, int, bool) {
	if !rs.rivalry(now) {
		return 0, 0, 0, false
	}
	h := rs.h
	groups := h.Groups()
	mine := groups[r.Group].Faction
	if mine == 0 {
		return 0, 0, 0, false
	}
	var tid entity.EntityID
	var tx, ty int
	best = min(best, SightRange+1)
	for _, id := range h.RaiderIDs() {
		o := h.Raider(id)
		if o.Group >= len(groups) || groups[o.Group].Faction == mine || groups[o.Group].Faction == 0 || !h.StandingRaider(id) {
			continue
		}
		ox, oy, _ := h.Pos(id)
		if d := weapon.Distance(x, y, ox, oy); d < best && rs.m.Rival(mine, groups[o.Group].Faction) {
			tid, tx, ty, best = id, ox, oy, d
		}
	}
	return tid, tx, ty, tid != 0
}

// crop is the nearest sown crop within r of (x, y), ring by ring (ok
// false: none, or no Tactics role). wiki: Raid, burners set crops alight.
func (rs *Raiders) crop(x, y, r int) (int, int, bool) {
	if rs.t == nil {
		return 0, 0, false
	}
	for d := 0; d <= r; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if max(dx, -dx, dy, -dy) == d && rs.h.InBounds(x+dx, y+dy) && rs.t.Crop(x+dx, y+dy) {
					return x + dx, y + dy, true
				}
			}
		}
	}
	return 0, 0, false
}
