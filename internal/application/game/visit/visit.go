package visit

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Visitors (ENG-003, M9). A friendly or neutral faction sends two to four
// of its people: they walk in from an edge, stroll near the colony for
// half a day (job.Visit: walk to a spot, linger) and walk out again (a
// trader caravan stays longer and closer). They are registered people
// (Where Visitor, their faction) on the raider list as non-hostile pawns,
// so colonists can chat with them and capture or arrest them; the visit
// itself is driven from here, out of the raid and prisoner AI. Leaving is
// one-way: once a visitor turns for the edge it never strolls again.

const (
	// stay is how long a group strolls before it leaves.
	stay = clock.TicksPerDay / 2
	// linger is how long a visitor stands at a spot it walked to.
	linger = 600
	// StrollRange is how far from the colony's heart they stroll.
	StrollRange = 10
	// retry: a walk that failed is tried again this much later.
	retry = 250
	// giveUp: a leaving visitor still on the map this long after its
	// leave time slips away where it stands.
	giveUp = clock.TicksPerDay / 2
	// Inset keeps visitors this far in from the border (the
	// released-prisoner rule takes anyone standing on it off the map).
	Inset = 2
	// spotR is how far from an edge point a spot is looked for.
	spotR = 12
)

// Rec is one visitor on the map (saved).
type Rec struct {
	ID      entity.EntityID
	Faction int // 1 + faction index
	LeaveAt uint64
	Leaving bool
	RetryAt uint64
	// Ally: a fighter sent as military aid (aid.go, PLAY-024); false for
	// every ordinary visitor and in older saves.
	Ally bool
	// Down: it was down last tick, so a death now is a death while down
	// (care.Hospitality.Died, SIM-063).
	Down bool
}

// Visits is the visitor records (the host's, Records) over the host.
type Visits struct{ h Host }

// Of is the visits over h.
func Of(h Host) Visits { return Visits{h} }

// recs is the visitor records (empty while there are none).
func (v Visits) recs() []Rec {
	if p := v.h.Records(false); p != nil {
		return *p
	}
	return nil
}

// ofRef is the faction of reference ref (1 + index; nil: none).
func ofRef(fs []faction.Faction, ref int) *faction.Faction {
	if ref <= 0 || ref > len(fs) {
		return nil
	}
	return &fs[ref-1]
}

// Factions are the factions (1 + index) that may send visitors: human,
// not hostile, not defeated.
func (v Visits) Factions() []int {
	var out []int
	for i, f := range v.h.Factions() {
		if f.Type.Human() && f.Relation != faction.Hostile && !f.Defeated {
			out = append(out, i+1)
		}
	}
	return out
}

// Arrive brings a visitor group in from a map edge.
func (v Visits) Arrive(now uint64) bool {
	facs := v.Factions()
	if len(facs) == 0 {
		return false
	}
	h := v.h
	ref := facs[h.WorldRoll(len(facs))]
	f := ofRef(h.Factions(), ref)
	ex, ey, ok := h.Edge()
	if !ok {
		return false
	}
	n := 2 + h.WorldRoll(3)
	wantKin := h.RelativeRoll()
	var kin entity.EntityID
	var used [][2]int
	var ids []entity.EntityID
	for k := 0; k < n; k++ {
		x, y, ok := v.InsetLand(ex, ey, used)
		if !ok {
			break
		}
		used = append(used, [2]int{x, y})
		if wantKin {
			wantKin = false
			if kin = h.RelativeFor(ref, f); kin != 0 {
				v.Place(kin, ref, x, y, now)
				ids = append(ids, kin)
				continue
			}
		}
		ids = append(ids, v.Spawn(ref, f, x, y, now))
	}
	if len(ids) == 0 {
		return false
	}
	h.Letter("info", "Visitors", fmt.Sprintf("%d people of %s have come to visit. They will stay about half a day.", len(ids), f.Name), ids[0])
	if kin != 0 {
		h.RelativeLetter(kin, "visitors from "+f.Name)
	}
	return true
}

// InsetLand is a free dry cell near (x, y) at least Inset from the
// border.
func (v Visits) InsetLand(x, y int, used [][2]int) (int, int, bool) {
	w, h := v.h.Size()
	x = min(max(x, Inset), w-1-Inset)
	y = min(max(y, Inset), h-1-Inset)
	return travel.Ring(x, y, spotR, func(nx, ny int) bool { // game/travel
		return Inside(nx, ny, w, h) && v.h.Free(nx, ny) && !slices.Contains(used, [2]int{nx, ny})
	})
}

// Inside reports (x, y) at least Inset in from the border of a w×h map.
func Inside(x, y, w, h int) bool {
	return x >= Inset && y >= Inset && x < w-Inset && y < h-Inset
}

// Spawn puts one new visitor of faction ref on (x, y).
func (v Visits) Spawn(ref int, f *faction.Faction, x, y int, now uint64) entity.EntityID {
	id := v.h.NewPerson(ref, f)
	v.Place(id, ref, x, y, now)
	return id
}

// Place stands registered person id on (x, y) as a visitor of ref,
// staying stay.
func (v Visits) Place(id entity.EntityID, ref, x, y int, now uint64) {
	v.h.Place(id, ref, x, y, now)
	p := v.h.Records(true)
	*p = append(*p, Rec{ID: id, Faction: ref, LeaveAt: now + stay})
}

// SetStay makes visitor id leave at leaveAt (a trader caravan's longer
// stay, a stranger waiting for an answer).
func (v Visits) SetStay(id entity.EntityID, leaveAt uint64) {
	rs := v.recs()
	for i := range rs {
		if rs[i].ID == id {
			rs[i].LeaveAt = leaveAt
		}
	}
}

// IDs lists the visitors on the map, ascending.
func (v Visits) IDs() []entity.EntityID {
	out := make([]entity.EntityID, 0, len(v.recs()))
	for _, r := range v.recs() {
		out = append(out, r.ID)
	}
	slices.Sort(out)
	return out
}

// Tick drives every visitor (aid: the allies among them): keep it out of
// the released-prisoner walk-out, stroll near the colony, and leave when
// the stay is over.
func (v Visits) Tick(a Aid, now uint64) {
	list := v.h.Records(false)
	if list == nil {
		return
	}
	kept := (*list)[:0]
	for i := range *list {
		r := &(*list)[i] // in place: a guest leaving updates the record mid-step
		if v.step(a, r, now) {
			kept = append(kept, *r)
			continue
		}
		v.h.TraderGone(r.ID) // trade_caravan.go (PLAY-024): the trader's goods
		if r.Ally {
			a.gone(*r) // a letter for one who fell
		}
		if r.Down && v.h.Status(r.ID) == health.Dead {
			v.h.GuestDied(r.ID) // care/guest.go: died while down
		}
	}
	clear((*list)[len(kept):])
	*list = kept
}

// step runs visitor r for one tick; false: it is no longer a visitor on
// the map (gone, dead, taken prisoner).
func (v Visits) step(a Aid, r *Rec, now uint64) bool {
	h := v.h
	if !h.Present(r.ID) {
		return false
	}
	st := h.Status(r.ID)
	if st == health.Dead {
		return false
	}
	h.Hold(r.ID, now)
	if r.Down = st != health.Alive; r.Down {
		return true // down: it waits for help (or capture)
	}
	if h.GuestStep(r.ID) {
		return true // care/guest.go: rescued, it heals in the colony's bed
	}
	x, y := h.Pos(r.ID)
	j := h.Job(r.ID)
	if !r.Leaving && now >= r.LeaveAt {
		r.Leaving, r.RetryAt = true, 0
		h.Release(r.ID)
		h.SetJob(r.ID, job.Record{})
		j = job.Record{}
	}
	w, ht := h.Size()
	if r.Leaving && (min(x, y, w-1-x, ht-1-y) <= Inset || j.Kind == job.Move && j.State == job.Complete || now >= r.LeaveAt+giveUp) {
		h.Depart(r.ID)
		return false
	}
	if r.Ally && !r.Leaving {
		return a.step(r, x, y, j, now) // aid.go: fight, else stroll
	}
	if j.State == job.Complete || j.State == job.Fail || j.State == job.Do && j.Kind != job.Visit {
		// a finished walk, or another system's job left standing (a step
		// aside): nothing runs a raider-list pawn's job but this
		h.Release(r.ID)
		j = job.Record{}
		h.SetJob(r.ID, j)
	}
	if r.Leaving { // walk to the nearest edge band; the check above sends it home there
		if j.Active() || now < r.RetryAt {
			return true
		}
		ex, ey := raid.NearestEdge(x, y, w, ht)
		tx, ty, ok := v.InsetLand(ex, ey, nil)
		if !ok {
			tx, ty, ok = h.WalkableNear(ex, ey, spotR) // a wet edge: any way out
		}
		if !ok || !h.StartJob(r.ID, job.Record{Kind: job.Move, TX: tx, TY: ty}) {
			r.RetryAt = now + retry
		}
		return true
	}
	v.wander(r, j, now, true)
	return true
}

// wander keeps visitor r strolling: linger at its spot, let a walk run,
// hold a trader being traded with (traders), else walk to a new spot.
func (v Visits) wander(r *Rec, j job.Record, now uint64, traders bool) {
	switch {
	case j.Kind == job.Visit && j.State == job.Do:
		v.Linger(r.ID, j)
	case j.Active():
	case traders && v.h.TraderHeld(r.ID): // trade_caravan.go: a trader stays put while traded with
	case now >= r.RetryAt:
		if !v.stroll(r.ID) {
			r.RetryAt = now + retry
		}
	}
}

// stroll sends visitor id to a spot near the colony to linger.
func (v Visits) stroll(id entity.EntityID) bool {
	h := v.h
	hx, hy := h.Home()
	rg := h.StrollRange(id) // trade_caravan.go: a trader caravan camps closer
	x := hx + h.WorldRoll(2*rg+1) - rg
	y := hy + h.WorldRoll(2*rg+1) - rg
	tx, ty, ok := h.WalkableNear(x, y, 3)
	w, ht := h.Size()
	if !ok || !Inside(tx, ty, w, ht) {
		return false
	}
	return h.StartJob(id, job.Record{Kind: job.Visit, TX: tx, TY: ty, WorkLeft: linger})
}

// Linger counts a visitor's stay at its spot down; the visit ends when it
// is over.
func (v Visits) Linger(id entity.EntityID, j job.Record) {
	if j.WorkLeft > 0 {
		j.WorkLeft--
	}
	if j.WorkLeft == 0 {
		j.State = job.Complete
	}
	v.h.SetJob(id, j)
}
