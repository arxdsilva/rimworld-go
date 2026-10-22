package game

import (
	"fmt"
	"sort"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Postponed work (SIM-027): a job that failed because the way was blocked
// or the target was out of reach is put off for a while instead of being
// picked again at once, so the colonist gets on with other work.

const (
	// postBlockedTicks: work whose way was blocked (a colonist standing in
	// a doorway) is retried after this long.
	postBlockedTicks = 900
	// postUnreachTicks: work out of reach is retried after an hour, or as
	// soon as walkability changes anywhere (a wall built, mined, removed).
	postUnreachTicks = clock.TicksPerHour
	// postSweepEvery: expired entries and entries whose target is gone are
	// swept this often (lookups also drop expired entries lazily).
	postSweepEvery = clock.TicksPerHour
	// postViewLines caps the postponed-work lines in a colonist's view.
	postViewLines = 3
	// workTries caps the candidates a nearest-first giver tries per call.
	workTries = 8
	// orderBlockedTries: a player order blocked this many times is dropped.
	orderBlockedTries = 3
)

// postKey names one piece of postponed work: a pawn, a job kind and its
// target (an entity, or a tile when Target is 0).
type postKey struct {
	Pawn   entity.EntityID
	Kind   job.Kind
	Target entity.EntityID
	X, Y   int
}

// postVal is when the work may be tried again, and whether it was put off
// for being out of reach (cleared when walkability changes).
type postVal struct {
	Until   uint64
	Unreach bool
}

// postEntry is one saved postponement.
type postEntry struct {
	Key     postKey
	Until   uint64
	Unreach bool // absent in older saves
}

// postState is the postponed work and the player orders being followed.
// Maps are only looked up in decisions; whole-map passes (walkability
// clear, sweep) only delete, so iteration order never matters.
type postState struct {
	m       map[postKey]postVal
	rev     uint64 // grid revision the out-of-reach entries were judged at
	sweepAt uint64
	// orders is the player order each colonist is following now, so a
	// blocked one can go back on its queue.
	orders map[entity.EntityID]queuedOrder
}

func newPostState() postState {
	return postState{m: make(map[postKey]postVal), orders: make(map[entity.EntityID]queuedOrder)}
}

// postponable reports job kinds whose givers skip postponed targets.
func postponable(k job.Kind) bool {
	switch k {
	case job.Haul, job.Construct, job.Deconstruct, job.Harvest, job.Cut, job.Mine, job.Sow,
		job.Bill, job.Hunt, job.Refuel, job.Repair, job.Research, job.Clean, job.Bury,
		job.Roof, job.Unroof:
		return true
	}
	return false
}

// postKeyOf is the postponement key of pawn id's job j: its work target
// (the bench or fire for bills and refuelling; the frame, trough or load
// once a haul is carrying), else the tile worked on.
func postKeyOf(id entity.EntityID, j job.Record) postKey {
	target := j.Target
	switch j.Kind {
	case job.Refuel, job.Bill:
		target = j.Dest
	case job.Haul:
		if j.Carry != 0 {
			target = j.Carry
			if j.Dest != 0 {
				target = j.Dest
			}
		}
	case job.Bury:
		if j.Carry != 0 {
			target = j.Carry
		}
	}
	if target != 0 {
		return postKey{Pawn: id, Kind: j.Kind, Target: target}
	}
	switch j.Kind {
	case job.Mine, job.Clean, job.Roof, job.Unroof, job.Cut:
		return postKey{Pawn: id, Kind: j.Kind, X: j.DestX, Y: j.DestY}
	}
	return postKey{Pawn: id, Kind: j.Kind, X: j.TX, Y: j.TY}
}

// postponeJob puts pawn id's failed job j off when it failed for being
// blocked or out of reach.
func (g *Game) postponeJob(id entity.EntityID, j job.Record) {
	if j.Fail != job.Blocked && j.Fail != job.Unreachable || !postponable(j.Kind) {
		return
	}
	g.postpone(postKeyOf(id, j), j.Fail)
}

// postpone puts the work k off: Blocked for postBlockedTicks, Unreachable
// for postUnreachTicks (or until walkability changes).
func (g *Game) postpone(k postKey, why job.FailReason) {
	p := &g.postponed
	g.postSync()
	now := uint64(g.clock.Now())
	v := postVal{Until: now + postBlockedTicks}
	if why == job.Unreachable {
		v = postVal{Until: now + postUnreachTicks, Unreach: true}
	}
	p.m[k] = v
}

// postponeTile puts tile work (mining, roofing, cleaning, grass) off as
// out of reach.
func (g *Game) postponeTile(id entity.EntityID, kind job.Kind, x, y int) {
	g.postpone(postKey{Pawn: id, Kind: kind, X: x, Y: y}, job.Unreachable)
}

// postponedFor reports whether pawn id has put this work off.
func (g *Game) postponedFor(id entity.EntityID, kind job.Kind, target entity.EntityID, x, y int) bool {
	p := &g.postponed
	if len(p.m) == 0 {
		return false
	}
	g.postSync()
	k := postKey{id, kind, target, x, y}
	v, ok := p.m[k]
	if !ok {
		return false
	}
	if uint64(g.clock.Now()) >= v.Until {
		delete(p.m, k) // lazily pruned
		return false
	}
	return true
}

// postSync drops every out-of-reach entry once walkability has changed
// since they were recorded (a cheap revision compare otherwise).
func (g *Game) postSync() {
	p := &g.postponed
	rev := g.grid.Version()
	if p.rev == rev {
		return
	}
	p.rev = rev
	for k, v := range p.m {
		if v.Unreach {
			delete(p.m, k)
		}
	}
}

// postSweep runs once per postSweepEvery: it drops expired entries,
// entries whose pawn or target is gone, and stale order records.
func (g *Game) postSweep() {
	p := &g.postponed
	now := uint64(g.clock.Now())
	if now < p.sweepAt {
		return
	}
	p.sweepAt = now + postSweepEvery
	g.postSync()
	for k, v := range p.m {
		if now >= v.Until || !g.postPawnHere(k.Pawn) || g.postTargetGone(k) {
			delete(p.m, k)
		}
	}
	for id := range p.orders {
		if !g.followingOrder(id) {
			delete(p.orders, id)
		}
	}
}

// postPawnHere: the pawn is still on the map and not dead.
func (g *Game) postPawnHere(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	return ok && v.Status != health.Dead
}

// postTargetGone reports an entity target that no longer exists.
func (g *Game) postTargetGone(k postKey) bool {
	t := k.Target
	if t == 0 {
		return false
	}
	if _, ok := g.frames.Get(t); ok {
		return false
	}
	if _, ok := g.bldgs.Get(t); ok {
		return false
	}
	if _, ok := g.stacks.Get(t); ok {
		return false
	}
	if _, ok := g.plants.Get(t); ok {
		return false
	}
	if _, ok := g.animals.Get(t); ok {
		return false
	}
	return true
}

// Blocked player orders (SIM-027): a forced or queued order that fails
// because the way is blocked goes back to the front of the queue and is
// retried after postBlockedTicks (the colonist works meanwhile); after
// orderBlockedTries it is dropped with a message. One out of reach is
// dropped at once, with a message.

// trackOrder remembers q as the player order pawn id now follows.
func (g *Game) trackOrder(id entity.EntityID, q queuedOrder) {
	q.retryAt = 0
	g.postponed.orders[id] = q
}

// forcedJobEnded handles the end of pawn id's forced job j (phaseAI reset).
func (g *Game) forcedJobEnded(id entity.EntityID, j job.Record) {
	p := &g.postponed
	q, ok := p.orders[id]
	delete(p.orders, id)
	if !ok || j.State != job.Fail {
		return
	}
	pos, _ := g.positions.Get(id)
	switch j.Fail {
	case job.Blocked:
		q.tries++
		if q.tries >= orderBlockedTries {
			g.message(pos.X, pos.Y, "%s couldn't get through to %s.", g.name(id), g.orderText(id, q))
			return
		}
		q.retryAt = uint64(g.clock.Now()) + postBlockedTicks
		g.orderQueue[id] = append([]queuedOrder{q}, g.orderQueue[id]...)
	case job.Unreachable:
		g.message(pos.X, pos.Y, "%s can't reach %s.", g.name(id), g.orderText(id, q))
	}
}

// orderText is q's label for a sentence ("build the wall" style: the
// menu label with a lower-case first letter).
func (g *Game) orderText(id entity.EntityID, q queuedOrder) string {
	l := q.label
	if l == "" {
		l = g.orderLabel(id, q.cmd)
	}
	if l == "" {
		return "the order"
	}
	return strings.ToLower(l[:1]) + l[1:]
}

func (g *Game) exportPostponed() []postEntry {
	g.postSync() // what a continuing game would drop next
	out := make([]postEntry, 0, len(g.postponed.m))
	for k, v := range g.postponed.m {
		out = append(out, postEntry{k, v.Until, v.Unreach})
	}
	sortPostEntries(out)
	return out
}

func sortPostEntries(out []postEntry) {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Key, out[j].Key
		if a.Pawn != b.Pawn {
			return a.Pawn < b.Pawn
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Y < b.Y
	})
}

func (g *Game) importPostponed(in []postEntry) {
	g.postponed.m = make(map[postKey]postVal, len(in))
	for _, e := range in {
		g.postponed.m[e.Key] = postVal{e.Until, e.Unreach}
	}
	g.postponed.rev = g.grid.Version()
}

// exportOrders is the player order each colonist follows (saved).
func (g *Game) exportOrders() map[entity.EntityID]savedOrder {
	out := make(map[entity.EntityID]savedOrder, len(g.postponed.orders))
	for id, q := range g.postponed.orders {
		out[id] = savedOrder{Cmd: q.cmd, Label: q.label, Tries: q.tries}
	}
	return out
}

func (g *Game) importOrders(in map[entity.EntityID]savedOrder) {
	g.postponed.orders = make(map[entity.EntityID]queuedOrder, len(in))
	for id, o := range in {
		g.postponed.orders[id] = queuedOrder{cmd: o.Cmd, label: o.Label, tries: o.Tries}
	}
}

// postponedLines is pawn id's postponed work for its view, soonest retry
// first, at most postViewLines ("build wall: blocked, retry in 12 s",
// "haul to stockpile: can't reach"). Read-only.
func (g *Game) postponedLines(id entity.EntityID) []string {
	p := &g.postponed
	if len(p.m) == 0 {
		return nil
	}
	now := uint64(g.clock.Now())
	stale := p.rev != g.grid.Version() // out-of-reach entries about to clear
	var es []postEntry
	for k, v := range p.m {
		if k.Pawn != id || now >= v.Until || v.Unreach && stale || g.postTargetGone(k) {
			continue
		}
		es = append(es, postEntry{k, v.Until, v.Unreach})
	}
	sortPostEntries(es)
	sort.SliceStable(es, func(i, j int) bool { return es[i].Until < es[j].Until })
	var out []string
	for _, e := range es {
		if len(out) == postViewLines {
			break
		}
		if e.Unreach {
			out = append(out, g.postLabel(e.Key)+": can't reach")
			continue
		}
		secs := (e.Until - now + 59) / 60 // seconds at 1x speed
		out = append(out, fmt.Sprintf("%s: blocked, retry in %d s", g.postLabel(e.Key), secs))
	}
	return out
}

// postLabel names postponed work for the view.
func (g *Game) postLabel(k postKey) string {
	t := k.Target
	switch k.Kind {
	case job.Construct:
		if fr, ok := g.frames.Get(t); ok {
			return "build " + fr.Kind.String()
		}
		return "build"
	case job.Deconstruct, job.Repair, job.Refuel:
		if b, ok := g.bldgs.Get(t); ok {
			return k.Kind.String() + " " + b.Kind.String()
		}
	case job.Bill:
		if b, ok := g.bldgs.Get(t); ok {
			return "work at " + b.Kind.String()
		}
	case job.Haul:
		if fr, ok := g.frames.Get(t); ok {
			return "haul to " + fr.Kind.String()
		}
		if b, ok := g.bldgs.Get(t); ok {
			return "haul to " + b.Kind.String()
		}
		if st, ok := g.stacks.Get(t); ok {
			return "haul " + strings.ReplaceAll(st.Kind.String(), "_", " ")
		}
	case job.Harvest, job.Cut:
		if pl, ok := g.plants.Get(t); ok {
			return k.Kind.String() + " " + plantLabel(pl.Species)
		}
		if t == 0 && k.Kind == job.Cut {
			return "cut grass"
		}
	case job.Hunt:
		if a, ok := g.animals.Get(t); ok {
			return "hunt " + animalLabel(a)
		}
	case job.Mine:
		return "mine rock"
	case job.Roof:
		return "build roof"
	case job.Unroof:
		return "remove roof"
	}
	return k.Kind.String()
}
