package game

import (
	"fmt"
	"strings"

	"rimworld-go/internal/application/game/orders"
	"rimworld-go/internal/application/game/postpone"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
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
	// workTries caps the candidates a nearest-first giver tries per call.
	workTries = 8
	// orderBlockedTries: a player order blocked this many times is dropped.
	orderBlockedTries = 3
)

// postState is the postponed work (postpone.Store) and the player orders
// being followed.
type postState struct {
	postpone.Store
	// orders is the player order each colonist is following now, so a
	// blocked one can go back on its queue.
	orders map[entity.EntityID]queuedOrder
	ord    *orders.Orders // game/orders (AX-1), built by ordersOf (move.go); never saved here
}

func newPostState() postState {
	return postState{Store: postpone.New(), orders: make(map[entity.EntityID]queuedOrder)}
}

// postponeJob puts pawn id's failed job j off when it failed for being
// blocked or out of reach.
func (g *Game) postponeJob(id entity.EntityID, j job.Record) {
	if j.Fail != job.Blocked && j.Fail != job.Unreachable || !postpone.Postponable(j.Kind) {
		return
	}
	g.postpone(postpone.KeyOf(id, j), j.Fail)
}

// postpone puts the work k off: Blocked for postBlockedTicks, Unreachable
// for postUnreachTicks (or until walkability changes).
func (g *Game) postpone(k postpone.Key, why job.FailReason) {
	p := &g.postponed
	g.postSync()
	now := uint64(g.clock.Now())
	v := postpone.Val{Until: now + postBlockedTicks}
	if why == job.Unreachable {
		v = postpone.Val{Until: now + postUnreachTicks, Unreach: true}
	}
	p.Put(k, v)
}

// putOff records k as put off until tick until (not as out of reach): the
// dress cooldown (apparel.go).
func (g *Game) putOff(k postpone.Key, until uint64) {
	g.postSync()
	g.postponed.Put(k, postpone.Val{Until: until})
}

// postponeTile puts tile work (mining, roofing, cleaning, grass) off as
// out of reach.
func (g *Game) postponeTile(id entity.EntityID, kind job.Kind, x, y int) {
	g.postpone(postpone.Key{Pawn: id, Kind: kind, X: x, Y: y}, job.Unreachable)
}

// postponedFor reports whether pawn id has put this work off.
func (g *Game) postponedFor(id entity.EntityID, kind job.Kind, target entity.EntityID, x, y int) bool {
	p := &g.postponed
	if p.Len() == 0 {
		return false
	}
	g.postSync()
	k := postpone.Key{Pawn: id, Kind: kind, Target: target, X: x, Y: y}
	if !p.Has(id, kind) {
		if checkIndex(g) {
			if _, ok := p.Get(k); ok {
				panic(fmt.Sprintf("postponedFor(%v): unlisted entry", k))
			}
		}
		return false // nothing of this kind put off for this pawn
	}
	v, ok := p.Get(k)
	if !ok {
		return false
	}
	if uint64(g.clock.Now()) >= v.Until {
		p.Drop(k) // lazily pruned
		return false
	}
	return true
}

// postponedTargets is the entity targets of pawn id's put-off work of
// this kind, copied into buf (reused; empty when there are none). For any
// target not in it postponedFor(id, kind, target, 0, 0) is false and
// prunes nothing, so a giver's candidate loop may skip that call (and
// order its other, pure, filters as it likes) for such targets. Its
// postSync is unobservable: every read of the entries syncs first.
func (g *Game) postponedTargets(id entity.EntityID, kind job.Kind, buf []entity.EntityID) []entity.EntityID {
	p := &g.postponed
	buf = buf[:0]
	if p.Len() == 0 {
		return buf
	}
	g.postSync()
	for _, k := range p.Keys(id, kind) {
		if k.Target != 0 {
			buf = append(buf, k.Target)
		}
	}
	return buf
}

// postSync drops every out-of-reach entry once walkability has changed
// since they were recorded (a cheap revision compare otherwise).
func (g *Game) postSync() {
	p := &g.postponed
	rev := g.grid.Version()
	if p.Rev == rev {
		return
	}
	p.Rev = rev
	p.All(func(k postpone.Key, v postpone.Val) {
		if v.Unreach {
			p.Drop(k)
		}
	})
}

// postSweep runs once per postSweepEvery: it drops expired entries,
// entries whose pawn or target is gone, and stale order records.
func (g *Game) postSweep() {
	p := &g.postponed
	now := uint64(g.clock.Now())
	if now < p.SweepAt {
		return
	}
	p.SweepAt = now + postSweepEvery
	g.postSync()
	p.All(func(k postpone.Key, v postpone.Val) {
		if now >= v.Until || !g.postPawnHere(k.Pawn) || g.postTargetGone(k) {
			p.Drop(k)
		}
	})
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
func (g *Game) postTargetGone(k postpone.Key) bool {
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

func (g *Game) exportPostponed() []postpone.Entry {
	g.postSync() // what a continuing game would drop next
	return g.postponed.Export()
}

func (g *Game) importPostponed(in []postpone.Entry) {
	g.postponed.Load(in)
	g.postponed.Rev = g.grid.Version()
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
// first, at most postpone.ViewLines ("build wall: blocked, retry in 12 s",
// "haul to stockpile: can't reach"). Read-only.
func (g *Game) postponedLines(id entity.EntityID) []string {
	return g.postponedLinesIn(g.postponedTops(nil), id)
}

// postponedTops is, per pawn, the postponed work a view shows (not
// expired, not about to clear, target still there), its first
// postpone.ViewLines entries only, in one pass over the map (no sort): the
// snapshot gathers it once for every colonist. It reuses buf.
func (g *Game) postponedTops(buf []postpone.Top) []postpone.Top {
	p := &g.postponed
	buf = buf[:0]
	if p.Len() == 0 {
		return buf
	}
	now := uint64(g.clock.Now())
	stale := p.Rev != g.grid.Version() // out-of-reach entries about to clear
	p.All(func(k postpone.Key, v postpone.Val) {
		if now >= v.Until || v.Unreach && stale || g.postTargetGone(k) || k.Kind == job.Equip && k.Target == 0 {
			return // (an Equip with no target: a dress cooldown, not work)
		}
		var t *postpone.Top
		for i := range buf {
			if buf[i].Pawn == k.Pawn {
				t = &buf[i]
				break
			}
		}
		if t == nil {
			buf = append(buf, postpone.Top{Pawn: k.Pawn})
			t = &buf[len(buf)-1]
		}
		t.Add(postpone.Entry{Key: k, Until: v.Until, Unreach: v.Unreach})
	})
	return buf
}

// postponedLinesIn is postponedLines over tops, postponedTops' result.
func (g *Game) postponedLinesIn(tops []postpone.Top, id entity.EntityID) []string {
	var t *postpone.Top
	for i := range tops {
		if tops[i].Pawn == id {
			t = &tops[i]
			break
		}
	}
	if t == nil || t.N == 0 {
		return nil
	}
	now := uint64(g.clock.Now())
	var out []string
	for _, e := range t.E[:t.N] {
		if len(out) == postpone.ViewLines {
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
func (g *Game) postLabel(k postpone.Key) string {
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
			return k.Kind.String() + " " + pl.Species.DisplayName()
		}
		if t == 0 && k.Kind == job.Cut {
			return "cut grass"
		}
	case job.Hunt:
		if a, ok := g.animals.Get(t); ok {
			return "hunt " + animalLabel(a)
		}
	case job.Equip:
		if st, ok := g.stacks.Get(t); ok && st.Kind.IsApparel() {
			return "wear " + st.Kind.String()
		}
	case job.Mine:
		return "mine rock"
	case job.Smooth:
		if terrain.IsRockWall(g.terr.At(k.X, k.Y)) {
			return "smooth rock"
		}
		return "smooth floor"
	case job.Roof:
		return "build roof"
	case job.Unroof:
		return "remove roof"
	}
	return k.Kind.String()
}
