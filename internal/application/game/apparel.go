package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/sim/entity"
)

// Clothes. Each colonist wears at most one garment per slot (body, head)
// over its everyday clothes; garments shift its comfortable temperature
// band. Colonists dress themselves from free clothes when it helps.

// worn is what a colonist has on, indexed by item.Slot.
type worn [3]item.Kind

// bandsOf is a colonist's comfortable range given what it wears.
func (g *Game) bandsOf(id entity.EntityID) exposure.Bands {
	b := bandsWearing(g.apparel[id])
	if c, h := g.comfortShift(id); c != 0 || h != 0 {
		b = b.Wearing(-c, h) // skills.go: build and traits (Cold-hardy)
	}
	return b
}

func bandsWearing(w worn) exposure.Bands {
	b := exposure.Everyday
	for _, k := range w {
		if k != 0 {
			c, h := k.Insulation()
			b = b.Wearing(c, h)
		}
	}
	return b
}

// wear puts on the garment stack sid (one item), dropping whatever was in
// that slot.
func (g *Game) wear(id, sid entity.EntityID, st item.Stack) {
	pos, _ := g.positions.Get(id)
	w := g.apparel[id]
	slot := st.Kind.Slot()
	if old := w[slot]; old != 0 {
		g.dropStack(old, 1, pos.X, pos.Y)
	}
	w[slot] = st.Kind
	g.apparel[id] = w
	if st.Qty <= 1 {
		g.deleteItem(sid)
	} else {
		st.Qty--
		g.stacks.Set(sid, st)
	}
}

// Wearing lists the garments a colonist has on (dev and tests).
func (g *Game) Wearing(id entity.EntityID) []string {
	var out []string
	for _, k := range g.apparel[id] {
		if k != 0 {
			out = append(out, k.String())
		}
	}
	return out
}

// fit scores bands b for the season: how much of the day's likely range
// (mean +-10 C) it leaves uncomfortable. Lower is better.
func fit(b exposure.Bands, mean int) int {
	lo, hi := mean-100, mean+100
	miss := 0
	if lo < b.Min {
		miss += b.Min - lo
	}
	if hi > b.Max {
		miss += hi - b.Max
	}
	return miss
}

// assignWear sends a colonist to put on a free garment that suits the
// season better than what it has in that slot.
func (g *Game) assignWear(id entity.EntityID) bool {
	mean := int(g.climate.DailyMean(g.clock.Now()))
	pos, _ := g.positions.Get(id)
	cur := fit(g.bandsOf(id), mean)
	// take off a garment that makes the season worse (a parka in summer)
	for slot, k := range g.apparel[id] {
		if k == 0 {
			continue
		}
		w := g.apparel[id]
		w[slot] = 0
		if cur-fit(bandsWearing(w), mean) >= 30 {
			g.dropStack(k, 1, pos.X, pos.Y)
			g.apparel[id] = w
			return false
		}
	}
	var best entity.EntityID
	var bst item.Stack
	bestGain, bestD := 0, 1<<30
	for _, sid := range g.itemIDs {
		sp := g.stacks.Ref(sid)
		if sp == nil || sp.Carrier != 0 || sp.Forbidden || !sp.Kind.IsApparel() {
			continue
		}
		st := *sp // a garment: copied (the loop only reads)
		if _, taken := g.reserve.EntityOwner(sid); taken {
			continue
		}
		w := g.apparel[id]
		w[st.Kind.Slot()] = st.Kind
		gain := cur - fit(bandsWearing(w), mean)
		if gain < 30 { // not worth the walk (3 C or less)
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if gain > bestGain || gain == bestGain && d < bestD {
			best, bst, bestGain, bestD = sid, st, gain, d
		}
	}
	if best == 0 || !g.reachable(id, bst.X, bst.Y) {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Equip, Target: best, TX: bst.X, TY: bst.Y})
}
