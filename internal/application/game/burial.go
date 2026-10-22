package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Burial and cremation (SIM-017): bodies become corpse items, haulers
// carry them to graves, pyres burn them.
//
// A dying pawn (colonist or raider) keeps its entity, Status dead, and is
// Embodied: its body is now a CorpseHuman stack (Of = the pawn) that rots
// into Bones. A Bury job (Haul work) reserves the body and an empty grave,
// carries the body there and fills the grave in; a pyre's Cremate bill
// burns it. Either way the dead are laid to rest: g.graves keeps a record,
// keyed by the grave for a burial and by the deceased for a cremation
// (ashes, no grave), so a loaded game never gives them a body back. A
// buried or cremated raider then leaves the raider list.

const (
	// buryWork is the work of filling a grave in over a body.
	buryWork = 200
	// corpseSight: an unburied body this close (same room, or both
	// outdoors) upsets a colonist.
	corpseSight = 8
)

// graveRec is who lies in a filled grave (or was cremated).
type graveRec struct {
	Occupant  entity.EntityID
	Name      string
	Cause     string
	DiedDay   int
	BuriedDay int
	Headstone int
}

func isBody(k item.Kind) bool { return k == item.CorpseHuman || k == item.Bones }

// dayOf is the 1-based day number of tick t.
func dayOf(t uint64) int { return int(t/clock.TicksPerDay) + 1 }

// headstoneOf picks one of the four headstone styles from the occupant.
func headstoneOf(id entity.EntityID) int {
	x := uint64(id) + 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return int(x % 4)
}

// spawnCorpse turns a pawn that just died into a human corpse item lying
// where it fell.
func (g *Game) spawnCorpse(id entity.EntityID) {
	pos, ok := g.positions.Get(id)
	if !ok {
		return
	}
	if _, _, has := g.bodyOf(id); has {
		return
	}
	cid := g.entities.Next()
	g.itemIDs = append(g.itemIDs, cid)
	g.mergeOrSet(cid, item.Stack{Kind: item.CorpseHuman, Qty: 1, X: pos.X, Y: pos.Y, Of: id, DiedAt: uint64(g.clock.Now())})
}

// bodyOf is the corpse (or bones) of pawn of.
func (g *Game) bodyOf(of entity.EntityID) (entity.EntityID, item.Stack, bool) {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Of == of && isBody(st.Kind) {
			return id, st, true
		}
	}
	return 0, item.Stack{}, false
}

// mourn gives every living colonist the ColonistDied thought when a
// colonist (not a raider) dies.
func (g *Game) mourn(id entity.EntityID) {
	if g.isRaider(id) {
		return
	}
	for _, pid := range g.pawnIDs {
		if v, ok := g.vital.Get(pid); ok && pid != id && v.Status != health.Dead {
			g.remember(pid, mood.ColonistDied)
		}
	}
}

// comfort gives every living colonist LaidToRest.
func (g *Game) comfort() {
	for _, pid := range g.pawnIDs {
		if v, ok := g.vital.Get(pid); ok && v.Status != health.Dead {
			g.remember(pid, mood.LaidToRest)
		}
	}
}

// unburiedNear reports a human corpse (not bones) within corpseSight of
// p, in the same room or both outdoors. It walks the item store's dense
// array, never the map.
func (g *Game) unburiedNear(p Position) bool {
	found, room, known := false, 0, false
	g.stacks.All(func(_ entity.EntityID, st item.Stack) {
		if found || st.Kind != item.CorpseHuman {
			return
		}
		dx, dy := st.X-p.X, st.Y-p.Y
		if dx*dx+dy*dy > corpseSight*corpseSight {
			return
		}
		if !known {
			room, known = g.roomAt(p.X, p.Y), true
		}
		found = g.roomAt(st.X, st.Y) == room
	})
	return found
}

// graveFilled reports a grave someone already lies in.
func (g *Game) graveFilled(bid entity.EntityID) bool {
	_, ok := g.graves[bid]
	return ok
}

func (g *Game) setGrave(key entity.EntityID, rec graveRec) {
	if g.graves == nil {
		g.graves = make(map[entity.EntityID]graveRec)
	}
	g.graves[key] = rec
}

// laidToRest reports a pawn already buried or cremated.
func (g *Game) laidToRest(of entity.EntityID) bool {
	if _, ok := g.graves[of]; ok {
		return true
	}
	for _, rec := range g.graves { // membership only: order does not matter
		if rec.Occupant == of {
			return true
		}
	}
	return false
}

// restRecord describes the deceased of a body stack, laid to rest today.
func (g *Game) restRecord(st item.Stack) graveRec {
	v, _ := g.vital.Get(st.Of)
	return graveRec{Occupant: st.Of, Name: g.name(st.Of), Cause: v.Cause.String(),
		DiedDay: dayOf(st.DiedAt), BuriedDay: dayOf(uint64(g.clock.Now())), Headstone: headstoneOf(st.Of)}
}

// recordAshes notes that pawn of was cremated (no grave holds it).
func (g *Game) recordAshes(of entity.EntityID) {
	_, st, _ := g.bodyOf(of)
	st.Of = of
	if st.DiedAt == 0 {
		st.DiedAt = uint64(g.clock.Now())
	}
	g.setGrave(of, g.restRecord(st))
}

// retireRaider drops a raider whose body is gone from the raider list;
// its grave record keeps its name.
func (g *Game) retireRaider(of entity.EntityID) {
	if g.isRaider(of) {
		g.removeRaider(of)
	}
}

// layInGrave buries body stack cid in grave bid: the grave records the
// deceased and becomes a mound that blocks movement.
func (g *Game) layInGrave(bid, cid entity.EntityID) {
	st, ok := g.stacks.Get(cid)
	b, isB := g.bldgs.Get(bid)
	if !ok || !isB || !isBody(st.Kind) {
		return
	}
	rec := g.restRecord(st)
	g.setGrave(bid, rec)
	g.deleteItem(cid)
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		g.grid.SetWalkable(c[0], c[1], false)
	}
	for _, c := range cells {
		g.ejectPawns(c[0], c[1])
	}
	g.message(b.X, b.Y, "%s was laid to rest", rec.Name)
	g.comfort()
	g.retireRaider(rec.Occupant)
}

// cremate burns body stack cid on pyre bid with the recipe's wood.
func (g *Game) cremate(bid, cid entity.EntityID, d recipe.Def) {
	st, ok := g.stacks.Get(cid)
	if !ok {
		return
	}
	g.deleteItem(cid)
	b, _ := g.bldgs.Get(bid)
	g.burnWood(b.X, b.Y, d.ExtraQty)
	rec := g.restRecord(st)
	g.setGrave(st.Of, rec)
	g.message(b.X, b.Y, "%s was cremated", rec.Name)
	g.comfort()
	g.retireRaider(st.Of)
}

// freeWood is the wood lying free (not carried) on the map.
func (g *Game) freeWood() int {
	n := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.Wood && st.Carrier == 0 {
			n += st.Qty
		}
	}
	return n
}

// burnWood takes n wood from the free stacks nearest (x, y) (ties: lower
// id first).
func (g *Game) burnWood(x, y, n int) {
	for n > 0 {
		var best entity.EntityID
		var bst item.Stack
		bestD := 1 << 30
		for _, id := range g.itemIDs {
			st, ok := g.stacks.Get(id)
			if !ok || st.Kind != item.Wood || st.Carrier != 0 || st.Qty <= 0 {
				continue
			}
			if d := pathing.Dist(x, y, st.X, st.Y); d < bestD || d == bestD && id < best {
				best, bst, bestD = id, st, d
			}
		}
		if best == 0 {
			return
		}
		take := min(n, bst.Qty)
		n -= take
		if bst.Qty -= take; bst.Qty <= 0 {
			g.deleteItem(best)
		} else {
			g.stacks.Set(best, bst)
		}
	}
}

// cremating reports a pawn burning a body on pyre bid right now.
func (g *Game) cremating(bid entity.EntityID) bool {
	for _, id := range g.pawnIDs {
		if j, ok := g.jobs.Get(id); ok && j.Kind == job.Bill && j.Dest == bid && j.State == job.Do && j.Carry != 0 {
			return true
		}
	}
	return false
}

// assignBury gives hauler id the nearest free body and the empty grave
// nearest that body (reachable), reserving both.
func (g *Game) assignBury(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	var cid entity.EntityID
	var cst item.Stack
	bestD := 1 << 30
	g.stacks.All(func(iid entity.EntityID, st item.Stack) {
		if !isBody(st.Kind) || st.Carrier != 0 || st.Qty <= 0 {
			return
		}
		if d := pathing.Dist(pos.X, pos.Y, st.X, st.Y); d < bestD || d == bestD && iid < cid {
			if _, taken := g.reserve.EntityOwner(iid); !taken {
				cid, cst, bestD = iid, st, d
			}
		}
	})
	if cid == 0 {
		return false
	}
	gid, sx, sy, ok := g.graveFor(id, cst)
	if !ok {
		return false
	}
	rec := job.Record{Kind: job.Bury, Target: cid, TX: cst.X, TY: cst.Y, Dest: gid, DestX: sx, DestY: sy, WorkLeft: buryWork}
	if !g.startJob(id, rec) {
		return false
	}
	g.reserve.TryEntity(id, gid)
	return true
}

// graveFor is the empty, free grave nearest body st that hauler id can
// reach, and the tile to fill it from.
func (g *Game) graveFor(id entity.EntityID, st item.Stack) (entity.EntityID, int, int, bool) {
	type cand struct {
		id entity.EntityID
		d  int
		b  construction.Building
	}
	var cs []cand
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.Grave || b.Deconstruct || g.graveFilled(bid) {
			continue
		}
		if _, taken := g.reserve.EntityOwner(bid); taken {
			continue
		}
		cs = append(cs, cand{bid, pathing.Dist(st.X, st.Y, b.X, b.Y), b})
	}
	for len(cs) > 0 {
		bi := 0
		for i, c := range cs {
			if c.d < cs[bi].d || c.d == cs[bi].d && c.id < cs[bi].id {
				bi = i
			}
		}
		c := cs[bi]
		if x, y, ok := g.graveSpot(id, c.b); ok {
			return c.id, x, y, true
		}
		cs = append(cs[:bi], cs[bi+1:]...)
	}
	return 0, 0, 0, false
}

// graveSpot is where a hauler stands to fill grave b: the first walkable,
// reachable tile beside it, else the grave itself.
func (g *Game) graveSpot(id entity.EntityID, b construction.Building) (int, int, bool) {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	in := func(x, y int) bool {
		for _, c := range cells {
			if c[0] == x && c[1] == y {
				return true
			}
		}
		return false
	}
	for _, c := range cells {
		for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
			x, y := c[0]+d[0], c[1]+d[1]
			if in(x, y) || !g.grid.Walkable(x, y) {
				continue
			}
			if g.reachable(id, x, y) {
				return x, y, true
			}
		}
	}
	if g.reachable(id, b.X, b.Y) {
		return b.X, b.Y, true
	}
	return 0, 0, false
}

// doBury runs a burial job: pick the body up, carry it, fill the grave.
func (g *Game) doBury(id entity.EntityID, j job.Record) {
	fail := func(r job.FailReason) {
		j.State, j.Fail = job.Fail, r
		g.jobs.Set(id, j)
	}
	if b, ok := g.bldgs.Get(j.Dest); !ok || b.Kind != construction.Grave || g.graveFilled(j.Dest) {
		fail(job.TargetGone)
		return
	}
	if j.Carry == 0 {
		st, ok := g.stacks.Get(j.Target)
		if !ok || st.Carrier != 0 || !isBody(st.Kind) {
			fail(job.TargetGone)
			return
		}
		st.Carrier = id
		g.stacks.Set(j.Target, st)
		j.Carry = j.Target
		path, ok := g.pathTo(id, j.DestX, j.DestY)
		if !ok {
			fail(job.Unreachable) // phaseAI puts the body down
			return
		}
		j.Path, j.PathI = path, 0
		if len(path) > 0 {
			j.State = job.Goto
		}
		g.jobs.Set(id, j)
		return
	}
	if _, ok := g.stacks.Get(j.Carry); !ok {
		fail(job.TargetGone)
		return
	}
	if !g.workTick(id) {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.layInGrave(j.Dest, j.Carry)
	g.reserve.ReleaseOwner(id)
	j.Carry = 0
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// afterLoadBurial gives the dead of older saves (no body, never laid to
// rest) a corpse where they lie, dropping any weapon they still hold.
func (g *Game) afterLoadBurial() {
	for _, list := range [2][]entity.EntityID{g.pawnIDs, g.raiderIDs} {
		for _, id := range list {
			if v, ok := g.vital.Get(id); !ok || v.Status != health.Dead {
				continue
			}
			if _, _, has := g.bodyOf(id); has || g.laidToRest(id) {
				continue
			}
			g.spawnCorpse(id)
			g.dropRaiderGear(id)
		}
	}
}

// fillPawnDeath sets PawnView.Embodied: every dead pawn is its body (or
// already laid to rest).
func (g *Game) fillPawnDeath(pv *PawnView) {
	pv.Embodied = pv.Status == health.Dead.String()
}

// fillCorpseView sets ItemView.Of and OfName for a human corpse or bones,
// and names an animal corpse by its species (Kind "deer corpse", Animal
// "deer", SIM-019).
func (g *Game) fillCorpseView(iv *ItemView, st item.Stack) {
	if st.Kind == item.CorpseAnimal {
		s := animal.Species(st.Animal)
		iv.Kind, iv.Animal = s.Label()+" corpse", s.String()
		return
	}
	if !isBody(st.Kind) {
		return
	}
	iv.Of, iv.OfName = st.Of, g.name(st.Of)
}

// fillGraveView sets a grave's occupant, epitaph and headstone, and a
// pyre's Burning.
func (g *Game) fillGraveView(bv *BuildingView, id entity.EntityID, b construction.Building) {
	switch b.Kind {
	case construction.Grave:
		if rec, ok := g.graves[id]; ok {
			bv.Occupant, bv.Epitaph, bv.Headstone = rec.Name, epitaph(rec), rec.Headstone
		}
	case construction.Pyre:
		bv.Burning = g.cremating(id)
	}
}

func epitaph(r graveRec) string {
	return fmt.Sprintf("%s. Died day %d%s; laid to rest day %d.", r.Name, r.DiedDay, causePhrase(r.Cause), r.BuriedDay)
}

func causePhrase(cause string) string {
	switch cause {
	case health.CauseInjury.String():
		return " of wounds"
	case health.CauseHypothermia.String():
		return " of the cold"
	case health.CauseHeatstroke.String():
		return " of heatstroke"
	case health.CauseInfection.String():
		return " of infection"
	case health.CauseNoFood.String():
		return " of hunger"
	}
	return ""
}
