package visit

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/kit"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Military aid (PLAY-024 Phase 4, original tuning in
// .agents/handoffs/_plans/PLAY024_PLAN.md). "Request aid" over the radio costs a friendly
// faction's goodwill and queues a Call; after DelayMinH..DelayMaxH hours
// a group of AllyMin..AllyMax armed fighters walks in from a map edge.
// They are visitors marked Ally, so they stay out of the colonist bar,
// take no orders and are never turret, colonist or draft targets. Each
// tick a standing ally fights the standing raider or hostile animal it
// is on, looks for a new one every allyScan ticks, and otherwise strolls
// near the colony. Raiders fight them back (RaidTarget). The group leaves
// (the visitor leave path) once no hostile has been about for AllyQuiet,
// or allyStay after it came. Arrival, departure and every ally that falls
// get a letter; a death costs no goodwill.

const (
	// DelayMinH..DelayMaxH: hours from the request to arrival.
	DelayMinH = 2
	DelayMaxH = 4
	// AllyMin..AllyMax fighters come.
	AllyMin = 3
	AllyMax = 5
	// allyStay is the longest a group stays; AllyQuiet the calm (no
	// hostile seen) after which it leaves sooner. Leaving is one-way, so
	// the group never flaps between staying and going.
	allyStay  = clock.TicksPerDay
	AllyQuiet = 2 * clock.TicksPerHour
	// allyScan is how often (staggered by id) an ally without a fight
	// looks for one; it doubles as the retry after a failed attack.
	allyScan = 30
	// allySight is how far an ally sees a foe to go for (Chebyshev).
	allySight = 20
	// allyRetry: an arrival that found no way in tries again this much
	// later.
	allyRetry = clock.TicksPerHour
)

// Comms refusals for military aid.
const (
	ReasonNoFighters = "they have no fighters to send"
	ReasonAidComing  = "their fighters are on the way"
	ReasonAlliesHere = "their fighters are already here"
)

// Call is military aid from faction Faction (faction.Faction.ID), saved
// in the trade block: due at At until it arrives (Here); then it stays
// until LeaveAt at most, and Calm is the last tick a hostile was about.
// Leaving: the group has turned for home.
type Call struct {
	Faction int
	At      uint64
	Here    bool
	LeaveAt uint64
	Calm    uint64
	Leaving bool
}

// Aid is the aid calls (the caller's slice) over the visits.
type Aid struct {
	calls *[]Call
	v     Visits
}

// Aid is the aid calls in calls.
func (v Visits) Aid(calls *[]Call) Aid { return Aid{calls, v} }

// Calls is the aid calls in request order.
func (a Aid) Calls() []Call { return *a.calls }

// ref is faction id's visitor reference (1 + index; 0: none).
func (a Aid) ref(id int) int {
	for i, f := range a.v.h.Factions() {
		if f.ID == id {
			return i + 1
		}
	}
	return 0
}

// byID is faction id (nil: none).
func (a Aid) byID(id int) *faction.Faction { return ofRef(a.v.h.Factions(), a.ref(id)) }

// RequestOK reports whether faction f sends military aid if asked now,
// or why not.
func (a Aid) RequestOK(f faction.Faction, now uint64) (bool, string) {
	if !f.Type.Human() {
		return false, ReasonNoFighters
	}
	if ok, why := faction.CanRequestAid(f, a.v.h.Cooldown(f.ID), now); !ok {
		return false, why
	}
	if c := a.Of(f.ID); c != nil {
		if c.Here {
			return false, ReasonAlliesHere
		}
		return false, ReasonAidComing
	}
	return true, ""
}

// Request charges faction f for military aid, starts its cooldown and
// queues the group; it reports the hours until it arrives.
func (a Aid) Request(f *faction.Faction, now uint64) int {
	h := a.v.h
	faction.Adjust(f, -faction.AidCost, fmt.Sprintf("Day %d: military aid requested (%d goodwill)", h.RelDay(now)+1, -faction.AidCost))
	cd := h.Cooldown(f.ID)
	cd.AidUntil = now + faction.AidCooldownDays*clock.TicksPerDay
	h.SetCooldown(f.ID, cd)
	hours := DelayMinH + h.TradeRoll(DelayMaxH-DelayMinH+1)
	a.Add(Call{Faction: f.ID, At: now + uint64(hours)*clock.TicksPerHour})
	return hours
}

// Add queues c in arrival order (ties: lower faction id first).
func (a Aid) Add(c Call) {
	cs := *a.calls
	i, _ := slices.BinarySearchFunc(cs, c, func(a, b Call) int {
		if a.At != b.At {
			if a.At < b.At {
				return -1
			}
			return 1
		}
		return a.Faction - b.Faction
	})
	*a.calls = slices.Insert(cs, i, c)
}

// Of is faction id's aid call, coming or here (nil: none).
func (a Aid) Of(id int) *Call {
	for i := range *a.calls {
		if (*a.calls)[i].Faction == id {
			return &(*a.calls)[i]
		}
	}
	return nil
}

// Tick runs the aid calls once a tick: due groups arrive, groups on the
// map note whether hostiles are about and turn for home when it has been
// calm long enough or their stay is over, and a group whose fighters are
// all gone (left, dead or taken) is dropped.
func (a Aid) Tick(now uint64) {
	kept := (*a.calls)[:0]
	for _, c := range *a.calls {
		switch {
		case !c.Here && now >= c.At:
			if !a.arrive(&c, now) {
				continue // the faction can no longer send it
			}
		case c.Here:
			if len(a.AllyIDs(c.Faction)) == 0 {
				continue // all gone
			}
			if a.hostilesAbout(c.Faction) {
				c.Calm = now
			}
			if !c.Leaving && (now >= c.LeaveAt || now-c.Calm >= AllyQuiet) {
				a.leave(&c)
			}
		}
		kept = append(kept, c)
	}
	clear((*a.calls)[len(kept):])
	*a.calls = kept
	if len(kept) == 0 {
		*a.calls = nil // a save without aid keeps no empty list
	}
}

// arrive brings aid call c's fighters in from a map edge; false: the
// faction fell or turned hostile and the call is dropped. With no way in
// it tries again allyRetry later.
func (a Aid) arrive(c *Call, now uint64) bool {
	h, v := a.v.h, a.v
	ref := a.ref(c.Faction)
	f := ofRef(h.Factions(), ref)
	if f == nil || f.Defeated || f.Relation == faction.Hostile {
		return false
	}
	ex, ey, ok := h.Edge()
	if !ok {
		c.At = now + allyRetry
		return true
	}
	n := AllyMin + h.TradeRoll(AllyMax-AllyMin+1)
	var used [][2]int
	var ids []entity.EntityID
	for k := 0; k < n; k++ {
		x, y, ok := v.InsetLand(ex, ey, used)
		if !ok {
			break
		}
		used = append(used, [2]int{x, y})
		id := v.Spawn(ref, f, x, y, now)
		a.mark(id, now+allyStay)
		h.Arm(id, kit.Ally(faction.DefOf(f.Type).Tech, k)) // domain combat/kit
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		c.At = now + allyRetry
		return true
	}
	c.Here, c.LeaveAt, c.Calm = true, now+allyStay, now
	h.Letter("good", "Military aid", ids[0], "%d fighters of %s have come to help defend the colony. They will fight any raiders and hostile animals, and go home once all is quiet, within a day.", len(ids), f.Name)
	return true
}

// mark makes visitor id an ally leaving at leaveAt.
func (a Aid) mark(id entity.EntityID, leaveAt uint64) {
	rs := a.v.recs()
	for i := range rs {
		if r := &rs[i]; r.ID == id {
			r.Ally, r.LeaveAt = true, leaveAt
		}
	}
}

// AllyIDs lists faction id's allies on the map, ascending.
func (a Aid) AllyIDs(id int) []entity.EntityID {
	ref := a.ref(id)
	var out []entity.EntityID
	for _, r := range a.v.recs() {
		if r.Ally && !r.Turned && r.Faction == ref {
			out = append(out, r.ID)
		}
	}
	slices.Sort(out)
	return out
}

// IsAlly reports id is an ally on the map.
func (a Aid) IsAlly(id entity.EntityID) bool {
	for _, r := range a.v.recs() {
		if r.ID == id {
			return r.Ally && !r.Turned
		}
	}
	return false
}

// hostilesAbout reports a hostile is still about for faction id's group:
// a standing raider on the map (the raider list, never the animals), or
// one of its fighters in a fight (a hostile animal it went for).
func (a Aid) hostilesAbout(id int) bool {
	h := a.v.h
	for _, rid := range h.RaiderIDs() {
		if h.StandingRaider(rid) {
			return true
		}
	}
	for _, aid := range a.AllyIDs(id) {
		if j := h.Job(aid); j.Kind == job.Attack && j.Active() {
			return true
		}
	}
	return false
}

// leave turns aid group c for home: every fighter drops what it is doing
// and takes the visitor leave path.
func (a Aid) leave(c *Call) {
	c.Leaving = true
	h, ref := a.v.h, a.ref(c.Faction)
	var first entity.EntityID
	rs := a.v.recs()
	for i := range rs {
		r := &rs[i]
		if !r.Ally || r.Turned || r.Faction != ref || r.Leaving {
			continue
		}
		if first == 0 {
			first = r.ID
		}
		r.Leaving, r.RetryAt = true, 0
		h.Release(r.ID)
		h.SetJob(r.ID, job.Record{})
	}
	name := "Your allies"
	if f := a.byID(c.Faction); f != nil {
		name = "The fighters of " + f.Name
	}
	h.Letter("info", "Allies leave", first, "%s are heading home.", name)
}

// step runs standing ally r (not leaving) for one tick: keep fighting its
// foe, look for a new one on its cadence, else stroll near the colony
// like any visitor.
func (a Aid) step(r *Rec, x, y int, j job.Record, now uint64) bool {
	h := a.v.h
	if j.Kind == job.Attack && j.Active() {
		h.Strike(r.ID, j, now)
		return true
	}
	if j.State == job.Complete || j.State == job.Fail || j.State == job.Do && j.Kind != job.Visit {
		h.Release(r.ID)
		j = job.Record{}
		h.SetJob(r.ID, j)
	}
	if (now+uint64(r.ID))%allyScan == 0 && h.Engage(r.ID, x, y, allySight, now) {
		return true
	}
	a.v.wander(r, j, now, false)
	return true
}

// gone notes ally r leaving the visitor list: one that died gets a letter
// (no goodwill is lost).
func (a Aid) gone(r Rec) {
	h := a.v.h
	if h.Status(r.ID) != health.Dead {
		return
	}
	of := ""
	if f := ofRef(h.Factions(), r.Faction); f != nil {
		of = " of " + f.Name
	}
	h.Letter("caution", "Ally killed", r.ID, "%s%s fell defending the colony.", h.Name(r.ID), of)
}

// RaidTarget is what a raider goes for from (x, y): the nearest standing
// colonist or ally (ties: the colonist).
func (a Aid) RaidTarget(x, y int) (entity.EntityID, int, int, bool) {
	h := a.v.h
	best, bx, by, ok := h.NearestColonist(x, y)
	bestD := 1 << 30
	if ok {
		bestD = weapon.Distance(x, y, bx, by)
	}
	for _, r := range a.v.recs() {
		if !r.Ally || r.Turned || r.Leaving || !h.Standing(r.ID) {
			continue
		}
		ax, ay := h.Pos(r.ID)
		if d := weapon.Distance(x, y, ax, ay); d < bestD { // visitors ascend: the lower id wins a tie
			best, bx, by, bestD = r.ID, ax, ay, d
		}
	}
	return best, bx, by, best != 0
}
