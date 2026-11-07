package quest

import (
	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/sim/clock"
)

// Cadence is the offer clock (QST-020..024): the window whose one offer
// is pending and its tick. Saved.
type Cadence struct {
	// Started: a window was ever scheduled (Window and At are valid).
	Started bool
	Window  int
	At      uint64
}

// windowTicks is one cadence window in ticks.
const windowTicks = qd.WindowDays * clock.TicksPerDay

// popDue handles every heap entry due at now, earliest first. Only due
// entries are touched: the heap's top is checked once more and left.
func (q *Quests) popDue(now uint64) {
	for {
		e, ok := q.heap.top()
		if !ok || e.Tick > now {
			return
		}
		q.heap.pop()
		if e.ID == 0 {
			if q.cad.Started && e.Tick == q.cad.At {
				q.fireOffer(e.Tick, now)
			}
			continue
		}
		if cur, ok := q.due[e.ID]; !ok || cur != e {
			continue // stale: the quest moved on
		}
		delete(q.due, e.ID)
		if i, ok := q.find(e.ID); ok {
			q.fire(i, e, now)
		}
		if cur, ok := q.due[e.ID]; ok && cur == e {
			// The timer did not move (a domain that refused the move):
			// drop it rather than pop it again forever.
			delete(q.due, e.ID)
		}
	}
}

// fire runs quest i's due timer e.
func (q *Quests) fire(i int, e entry, now uint64) {
	switch e.Ev {
	case qd.EvAcceptEnd:
		q.expire(i, now)
	case qd.EvDeadline:
		q.fail(i, now, qd.Timeout)
	case qd.EvPart:
		q.partDue(i, e.Part, now)
	default:
		q.touched(i)
	}
}

// fireOffer is the cadence's offer for the pending window, at its tick:
// a kind picked by weight is built and offered; when none can be, the
// window is lost (no catch-up, QST-024). The next window is scheduled at
// once, so its tick is always ahead.
func (q *Quests) fireOffer(at, now uint64) {
	c := q.ctx(at)
	if k, ok := q.pick(c); ok {
		q.build(k, c)
	}
	q.scheduleOffer(q.cad.Window+1, now)
}

// scheduleOffer schedules window w's offer (or the current window's, if
// later). A window whose rolled tick is already past is lost: an older
// save loaded mid-window offers nothing at once (QST-149).
func (q *Quests) scheduleOffer(w int, now uint64) {
	landed := q.r.World.Landed()
	if now < landed {
		return
	}
	if cur := qd.WindowOf(now - landed); cur > w {
		w = cur
	}
	at := landed + qd.OfferTick(w, q.roll(windowTicks))
	if at < now {
		w++
		at = landed + qd.OfferTick(w, q.roll(windowTicks))
	}
	if at <= now {
		at = now + 1 // never due again in the same pop
	}
	q.cad = Cadence{Started: true, Window: w, At: at}
	q.heap.push(entry{Tick: at})
}

// roll is a roll in [0, n) from the quest stream, as a tick count.
func (q *Quests) roll(n int) uint64 { return uint64(q.ctx(0).Intn(n)) }

// pick chooses the kind to offer by weight among those that may be
// offered (QST-033, QST-036, QST-037): Violent kinds only with quest
// threats on, Joins kinds scaled by the colony-size curve.
func (q *Quests) pick(c Ctx) (qd.KindID, bool) {
	threats := q.r.World.QuestThreats()
	colonists := -1
	var w [qd.NumKinds]int
	total := 0
	for k := qd.KindID(1); k < qd.NumKinds; k++ {
		kind, d := q.kinds[k], q.defs[k]
		if kind == nil || (d.Violent && !threats) {
			continue
		}
		wt := d.Weight
		if d.Joins {
			if colonists < 0 {
				colonists = q.r.World.Colonists()
			}
			wt = wt * qd.JoinerWeight(colonists) / 1000
		}
		if wt <= 0 || !kind.CanOffer(c) {
			continue
		}
		w[k], total = wt, total+wt
	}
	if total == 0 {
		return 0, false
	}
	r := c.Intn(total)
	for k := qd.KindID(1); k < qd.NumKinds; k++ {
		if r < w[k] {
			return k, true
		}
		r -= w[k]
	}
	return 0, false
}

// relations diffs each faction's relation against the snapshot and checks
// the live quests of every faction that changed (QST-010..012). O(factions)
// when nothing changed.
func (q *Quests) relations(now uint64) {
	n := q.r.Goodwill.Count()
	for f := 0; f < n; f++ {
		r := q.r.Goodwill.Relation(f)
		switch {
		case f < len(q.rel):
			if q.rel[f] == r {
				continue
			}
			q.rel[f] = r
		default:
			q.rel = append(q.rel, r)
		}
		q.fids = q.idx.find(q.fids[:0], key{keyFaction, uint64(f)})
		for _, id := range q.fids {
			q.checkRelation(id, now)
		}
	}
	if n < len(q.rel) {
		q.rel = q.rel[:n]
	}
}

// checkRelation fails quest id when its giver or target now stands wrong
// (qd.FailOnRelation; an offer expires with the reason instead).
func (q *Quests) checkRelation(id qd.ID, now uint64) {
	i, ok := q.find(id)
	if !ok || q.list[i].State.Terminal() {
		return
	}
	n := q.list[i]
	g := q.r.Goodwill
	if r, bad := qd.FailOnRelation(n, q.defOf(n.Kind), relOf(g, n.Giver), relOf(g, n.Target)); bad {
		q.fail(i, now, r)
	}
}
