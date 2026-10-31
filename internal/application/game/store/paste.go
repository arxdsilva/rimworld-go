package store

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// The nutrient paste dispenser (STO-2, wiki: Nutrient paste dispenser,
// Hopper). A powered dispenser turns PasteCost nutrition of raw food from
// the hoppers on its sides into one nutrient paste meal for a colonist
// at its interaction spot: the hoppers are drawn from southwest first,
// then counterclockwise; a hopper short of food is emptied into the meal
// and the next one is tried; with too little in all of them nothing is
// taken and no meal is made. A colonist who has no better meal (paste is
// preferred over raw food, below a simple meal) walks to the spot, takes
// the meal (DispenseWork) and eats it as any meal (a table if one is in
// reach).

const (
	// PasteCost is the raw-food nutrition one paste meal takes: 0.3
	// nutrition for a 0.9 meal (wiki; FOOD-1 scale, needs.Full = 1.0), six
	// units of raw food.
	PasteCost = 3000
	// MaxHoppers is how many hoppers one dispenser draws from (wiki:
	// Hopper; its 3x4 sides less the spot hold exactly that many).
	MaxHoppers = 13
	// DispenseWork is the ticks a colonist spends taking a meal (original
	// tuning, about the vanilla collect time).
	DispenseWork = 50
)

// Paste is the dispenser's rules; it holds no state (the hoppers' food
// is ordinary stacks in their storage zones).
type Paste struct{}

// Hoppers lists the hopper buildings on dispenser b's sides in draw
// order: the south side west to east, the east side south to north, the
// north side east to west, the west side north to south, never the
// interaction spot, at most MaxHoppers.
func (Paste) Hoppers(m Machines, b construction.Building) (out [MaxHoppers][2]int, n int) {
	cells, spot, hasSpot := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if len(cells) == 0 {
		return out, 0
	}
	r := zone.Rect{MinX: cells[0][0], MinY: cells[0][1], MaxX: cells[0][0], MaxY: cells[0][1]}
	for _, c := range cells {
		r.MinX, r.MinY, r.MaxX, r.MaxY = min(r.MinX, c[0]), min(r.MinY, c[1]), max(r.MaxX, c[0]), max(r.MaxY, c[1])
	}
	try := func(x, y int) {
		if n >= MaxHoppers || hasSpot && x == spot[0] && y == spot[1] {
			return
		}
		if h, ok := m.Building(m.ObjectAt(x, y)); ok && h.Kind == construction.Hopper && h.X == x && h.Y == y {
			out[n] = [2]int{x, y}
			n++
		}
	}
	for x := r.MinX; x <= r.MaxX; x++ {
		try(x, r.MaxY+1)
	}
	for y := r.MaxY; y >= r.MinY; y-- {
		try(r.MaxX+1, y)
	}
	for x := r.MaxX; x >= r.MinX; x-- {
		try(x, r.MinY-1)
	}
	for y := r.MinY; y <= r.MaxY; y++ {
		try(r.MinX-1, y)
	}
	return out, n
}

// hopperFood is the raw food stack lying in the hopper on c (ok false
// for none, a forbidden stack or anything not raw food).
func hopperFood(l Larder, c [2]int) (entity.EntityID, item.Stack, bool) {
	id := l.ItemAt(c[0], c[1])
	st, ok := l.Stack(id)
	if !ok || st.Qty <= 0 || st.Forbidden || !zone.Universe(zone.KindHopper).Has(st.Kind) || st.Kind.Nutrition() <= 0 {
		return 0, item.Stack{}, false
	}
	return id, st, true
}

// Food is the raw-food nutrition in dispenser b's hoppers.
func (p Paste) Food(m Machines, l Larder, b construction.Building) int {
	hs, n := p.Hoppers(m, b)
	sum := 0
	for _, c := range hs[:n] {
		if _, st, ok := hopperFood(l, c); ok {
			sum += st.Qty * animal.StackNutrition(st)
		}
	}
	return sum
}

// usable reports building id is a powered dispenser whose hoppers hold a
// meal's worth of food.
func (p Paste) usable(m Machines, l Larder, id entity.EntityID) (construction.Building, bool) {
	b, ok := m.Building(id)
	if !ok || b.Kind != construction.NutrientPasteDispenser || b.Deconstruct || b.Forbidden || !m.Powered(id) {
		return b, false
	}
	return b, p.Food(m, l, b) >= PasteCost
}

// Ready reports some dispenser could make a meal now.
func (p Paste) Ready(m Machines, l Larder) bool {
	for _, id := range m.BuildingIDs() {
		if _, ok := p.usable(m, l, id); ok {
			return true
		}
	}
	return false
}

// Find is the usable dispenser whose spot is nearest colonist id (ties:
// the lower id) and that spot; ok false when none is in reach.
func (p Paste) Find(m Machines, l Larder, d Diners, id entity.EntityID) (bid entity.EntityID, sx, sy int, ok bool) {
	px, py := d.Position(id)
	best := 1 << 30
	for _, oid := range m.BuildingIDs() {
		b, use := p.usable(m, l, oid)
		if !use {
			continue
		}
		_, spot, has := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		if dist := pathing.Dist(px, py, spot[0], spot[1]); has && dist < best && d.Reachable(id, spot[0], spot[1]) {
			bid, sx, sy, ok, best = oid, spot[0], spot[1], true, dist
		}
	}
	return bid, sx, sy, ok
}

// Job is the DispensePaste job at dispenser bid's spot (sx, sy).
func (Paste) Job(bid entity.EntityID, sx, sy int) job.Record {
	return job.Record{Kind: job.DispensePaste, Target: bid, TX: sx, TY: sy, WorkLeft: DispenseWork}
}

// Dispense takes PasteCost nutrition from dispenser bid's hoppers in draw
// order; ok false (nothing taken) when it can't make a meal.
func (p Paste) Dispense(m Machines, l Larder, bid entity.EntityID) bool {
	b, ok := p.usable(m, l, bid)
	if !ok {
		return false
	}
	hs, n := p.Hoppers(m, b)
	need := PasteCost
	for _, c := range hs[:n] {
		id, st, ok := hopperFood(l, c)
		if !ok {
			continue
		}
		nut := animal.StackNutrition(st) // eggs by species (FOOD-1)
		take := min(st.Qty, (need+nut-1)/nut)
		need -= take * nut
		if st.Qty -= take; st.Qty <= 0 {
			l.DeleteItem(id)
		} else {
			l.SetStack(id, st)
		}
		if need <= 0 {
			break
		}
	}
	return true
}

// Do steps colonist id's DispensePaste job at the spot: after the work
// the dispenser makes a meal on the colonist's tile and the job becomes
// eating it (a table first, when one is in reach). A dispenser that went
// dark or ran dry fails the job.
func (p Paste) Do(m Machines, l Larder, d Diners, id entity.EntityID, j job.Record) {
	if j.WorkLeft--; j.WorkLeft > 0 {
		d.SetJob(id, j)
		return
	}
	x, y := d.Position(id)
	if !p.Dispense(m, l, j.Target) {
		j.State, j.Fail = job.Fail, job.TargetGone
		d.SetJob(id, j)
		return
	}
	meal := l.Spawn(item.NutrientPasteMeal, 1, x, y)
	eat := job.Record{Kind: job.Eat, Target: meal, TX: x, TY: y, WorkLeft: needs.EatWorkTicks, State: job.Do}
	d.Release(id, j.Target) // the next colonist may use the dispenser
	if meal == 0 || !d.Claim(id, meal) {
		j.State, j.Fail = job.Fail, job.TargetGone
		d.SetJob(id, j)
		return
	}
	d.SetJob(id, eat)
}
