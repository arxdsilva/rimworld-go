package game

import (
	"slices"

	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Batched material delivery (HAUL-001): a hauler takes a full load, feeds
// a route of frames (or a fuel user) and leaves the rest in the nearest
// stockpile; the in-flight amount per frame keeps haulers from
// over-delivering. With no stockpile room for leftovers it carries just
// what the route needs.

// haulShare is the part of a hauler's load promised to one frame.
type haulShare = haul.Share

// haulState is the delivery bookkeeping: incoming is material on its way
// to each frame (the sum of every hauler's share), deliveries each
// hauler's shares in route order. Maps are only looked up, never ranged
// over in decisions.
type haulState struct {
	incoming   map[entity.EntityID]int
	deliveries map[entity.EntityID][]haulShare
	// storedAt (PLAY-023, haul_priority.go): the tick each stack was set
	// down in a storage zone below the best one taking it.
	storedAt map[entity.EntityID]uint64
	cands    []haul.Cand // HAUL-002 scratch (planGather): never saved
	picks    []haul.Pick
	store    store.Store // STO-2: storage buildings, urgent marks, the paste dispenser (game/store)
}

func (g *Game) exportIncoming() map[entity.EntityID]int { return g.haul.incoming }

func (g *Game) importIncoming(m map[entity.EntityID]int) { g.haul.incoming = m }

func (g *Game) exportDeliveries() map[entity.EntityID][]haulShare { return g.haul.deliveries }

func (g *Game) importDeliveries(m map[entity.EntityID][]haulShare) { g.haul.deliveries = m }

// slotNeed is what slot s of frame fid still lacks once loads on the way
// land. Components and secondary materials (AX-4) are wanted once the
// material is delivered or fully promised.
func (g *Game) slotNeed(fid entity.EntityID, fr construction.Frame, s haul.Slot) int {
	if s.Key != fid && fr.Wood+g.haul.incoming[fid] < fr.Need {
		return 0
	}
	return s.Need - s.Have - g.haul.incoming[s.Key]
}

type freeStack struct {
	id entity.EntityID
	st item.Stack
}

type needyFrame struct {
	id    entity.EntityID // the slot key (haul.Keys)
	stuff item.Kind
	need  int
	first int // pathing bonus: smashed buildings first
}

// planDelivery picks the first frame and the stack to feed it (nearest
// free stack of its material to the pawn, smashed buildings first, ties to
// the lower stack id) and chains a route from there.
func (g *Game) planDelivery(id entity.EntityID, pos Position) (job.Record, []haulShare, bool) {
	var frames []needyFrame
	var kinds []item.Kind // the materials wanted, distinct
	for _, fid := range g.frameIDs {
		fr, ok := g.frames.Get(fid)
		if !ok {
			continue
		}
		var buf [5]haul.Slot
		slots, open := haul.Slots(buf[:0], fid, fr), false
		for i := range slots { // Need becomes what each slot still lacks
			slots[i].Need = g.slotNeed(fid, fr, slots[i])
			open = open || slots[i].Need > 0
		}
		if !open {
			continue
		}
		if _, taken := g.reserve.EntityOwner(fid); taken {
			continue
		}
		if g.postponedFor(id, job.Haul, fid, 0, 0) {
			continue // SIM-027: blocked on the way there lately
		}
		if g.deckOutOfReach(id, fr) {
			continue // bridge.go: a span is fed from the shore, cell by cell
		}
		for _, sl := range slots {
			if sl.Need <= 0 {
				continue
			}
			nf := needyFrame{id: sl.Key, stuff: sl.Kind, need: sl.Need}
			if fr.Rebuild {
				nf.first = rebuildFirst
			}
			frames = append(frames, nf)
			if !slices.Contains(kinds, sl.Kind) {
				kinds = append(kinds, sl.Kind)
			}
		}
	}
	if len(frames) == 0 {
		return job.Record{}, nil, false
	}
	var free []freeStack
	for _, iid := range g.itemIDs {
		sp := g.stacks.Ref(iid)
		if sp == nil || sp.Qty <= 0 || sp.Carrier != 0 || sp.Forbidden || !slices.Contains(kinds, sp.Kind) {
			continue
		}
		st := *sp // a candidate (the loop only reads)
		if st.Qty = g.reserve.FreeFor(id, iid, sp.Qty); st.Qty <= 0 {
			continue // RES-1: a delivery takes what other pawns haven't claimed
		}
		if g.postponedFor(id, job.Haul, iid, 0, 0) {
			continue // SIM-027: out of reach or blocked lately
		}
		free = append(free, freeStack{iid, st})
	}
	var bestStack, bestFrame entity.EntityID
	var bst item.Stack
	bestNeed, bestD := 0, 1<<30
	for _, nf := range frames {
		for _, fs := range free {
			if fs.st.Kind != nf.stuff {
				continue
			}
			d := pathing.Dist(pos.X, pos.Y, fs.st.X, fs.st.Y) - nf.first
			if d < bestD || (d == bestD && fs.id < bestStack) {
				bestD, bestStack, bestFrame, bst, bestNeed = d, fs.id, nf.id, fs.st, nf.need
			}
		}
	}
	if bestStack == 0 {
		return job.Record{}, nil, false
	}
	rec, shares := g.routeFrom(bestStack, bst, bestFrame, bestNeed)
	return rec, shares, true
}

// routeFrom builds a delivery of stack sid starting at slot first (a
// frame's haul.SlotKey; its share is firstNeed): it chains the nearest same-material frames with
// need, each within haul.RouteReach of the last, until the route needs a full
// load or holds haul.RouteFrames. The load is a full CarryLimit when a
// stockpile cell can take the leftovers, else just what the route needs.
func (g *Game) routeFrom(sid entity.EntityID, st item.Stack, first entity.EntityID, firstNeed int) (job.Record, []haulShare) {
	fid0, _ := haul.SlotFrame(first)
	fr0, _ := g.frames.Get(fid0)
	route := []entity.EntityID{first}
	needs := []int{firstNeed}
	sum, lx, ly := firstNeed, fr0.X, fr0.Y
	for len(route) < haul.RouteFrames && sum < item.CarryLimit {
		var next entity.EntityID
		nd, nneed, nx, ny := 1<<30, 0, 0, 0
		for _, fid := range g.frameIDs {
			fr, ok := g.frames.Get(fid)
			if !ok {
				continue
			}
			var buf [5]haul.Slot
			for _, sl := range haul.Slots(buf[:0], fid, fr) { // the first slot of the load's kind with need
				d := pathing.Dist(lx, ly, fr.X, fr.Y)
				if sl.Kind != st.Kind || slices.Contains(route, sl.Key) || d > haul.RouteReach || d > nd || (d == nd && sl.Key > next) {
					continue
				}
				need := g.slotNeed(fid, fr, sl)
				if _, taken := g.reserve.EntityOwner(fid); need <= 0 || taken {
					continue
				}
				next, nd, nneed, nx, ny = sl.Key, d, need, fr.X, fr.Y
				break
			}
		}
		if next == 0 {
			break
		}
		route, needs = append(route, next), append(needs, nneed)
		sum, lx, ly = sum+nneed, nx, ny
	}
	rec := job.Record{Kind: job.Haul, Target: sid, TX: st.X, TY: st.Y, Dest: fid0, DestX: fr0.X, DestY: fr0.Y}
	take := min(st.Qty, item.CarryLimit)
	if take > sum {
		if sx, sy, ok := g.stockpileCell(st, lx, ly); ok {
			rec.SpareX, rec.SpareY, rec.HasSpare = sx, sy, true
		} else {
			take = sum // nowhere to store the rest: just what is needed
		}
	}
	shares := haul.Shares(route, needs, take)
	rec.Route = haul.Route(shares)
	rec.Count = take
	return rec, shares
}

// startDelivery starts a planned delivery and books its shares.
func (g *Game) startDelivery(id entity.EntityID, rec job.Record, shares []haulShare) bool {
	if !g.startJob(id, rec) {
		return false
	}
	if len(shares) == 0 {
		return true
	}
	if g.haul.incoming == nil {
		g.haul.incoming = make(map[entity.EntityID]int)
	}
	if g.haul.deliveries == nil {
		g.haul.deliveries = make(map[entity.EntityID][]haulShare)
	}
	g.haul.deliveries[id] = shares
	for _, s := range shares {
		g.haul.incoming[s.Frame] += s.Qty
	}
	return true
}

// releaseDelivery gives back every share hauler id still holds. Every job
// end (completion, failure, interrupt, death, replacement) calls it.
func (g *Game) releaseDelivery(id entity.EntityID) {
	shares, ok := g.haul.deliveries[id]
	if !ok {
		return
	}
	for _, s := range shares {
		g.dropIncoming(s.Frame, s.Qty)
	}
	delete(g.haul.deliveries, id)
}

// releaseStaleDelivery frees the shares of a hauler no longer on a
// delivery (a job swapped without an interrupt).
func (g *Game) releaseStaleDelivery(id entity.EntityID, j job.Record) {
	if len(g.haul.deliveries) == 0 {
		return
	}
	if j.Kind == job.Haul && len(j.Route) > 0 && j.Active() {
		return
	}
	g.releaseDelivery(id)
}

func (g *Game) dropIncoming(fid entity.EntityID, q int) {
	if n := g.haul.incoming[fid] - q; n > 0 {
		g.haul.incoming[fid] = n
	} else {
		delete(g.haul.incoming, fid)
	}
}

// consumeShare drops hauler id's share for frame fid (delivered or
// skipped).
func (g *Game) consumeShare(id, fid entity.EntityID) {
	shares := g.haul.deliveries[id]
	for i, s := range shares {
		if s.Frame != fid {
			continue
		}
		g.dropIncoming(fid, s.Qty)
		shares = append(shares[:i:i], shares[i+1:]...)
		if len(shares) == 0 {
			delete(g.haul.deliveries, id)
		} else {
			g.haul.deliveries[id] = shares
		}
		return
	}
}

// trimDelivery cuts hauler id's shares to what it actually picked up.
func (g *Game) trimDelivery(id entity.EntityID, carried int) {
	shares := g.haul.deliveries[id]
	if len(shares) == 0 {
		return
	}
	kept := shares[:0]
	for _, s := range shares {
		q := min(s.Qty, carried)
		carried -= q
		if q < s.Qty {
			g.dropIncoming(s.Frame, s.Qty-q)
		}
		if q > 0 {
			kept = append(kept, haulShare{Frame: s.Frame, Qty: q})
		}
	}
	if len(kept) == 0 {
		delete(g.haul.deliveries, id)
	} else {
		g.haul.deliveries[id] = kept
	}
}

// routeWalk heads a loaded delivery to its next live stop: the next frame
// still short of its material, else the spare stockpile cell when there
// is something left. It reports false when there is nowhere to walk (the
// pawn already stands at the stop, or the route is over): the caller acts
// at once.
func (g *Game) routeWalk(id entity.EntityID, j *job.Record, kind item.Kind) bool {
	for j.RouteI < len(j.Route) {
		key := j.Route[j.RouteI]
		if j.Kind != job.Haul {
			return false // a fuel user: the job walks there itself
		}
		fid, _ := haul.SlotFrame(key)
		fr, ok := g.frames.Get(fid)
		if ok && haul.SlotOpen(fr, key, kind) {
			if path, ok := g.pathTo(id, fr.X, fr.Y); ok {
				j.Dest, j.DestX, j.DestY = fid, fr.X, fr.Y
				return g.walkOn(id, j, path)
			}
		}
		g.consumeShare(id, key) // gone, filled or out of reach: skip it
		j.RouteI++
	}
	if !j.HasSpare {
		return false
	}
	path, ok := g.pathTo(id, j.SpareX, j.SpareY)
	if !ok {
		j.HasSpare = false // drop it where it stands
		return false
	}
	j.Dest, j.DestX, j.DestY = 0, j.SpareX, j.SpareY
	return g.walkOn(id, j, path)
}

func (g *Game) walkOn(id entity.EntityID, j *job.Record, path []pathing.Point) bool {
	j.Path, j.PathI = path, 0
	if len(path) == 0 {
		return false
	}
	j.State = job.Goto
	g.jobs.Set(id, *j)
	return true
}

// routeArrive runs a loaded delivery at its current stop: it feeds the
// frame there and walks on, or stores the leftover once the route is
// done.
func (g *Game) routeArrive(id entity.EntityID, j job.Record) {
	for {
		st, ok := g.stacks.Get(j.Carry)
		if !ok {
			j.State, j.Fail = job.Fail, job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		if j.RouteI >= len(j.Route) {
			g.storeLeftover(id, j, st)
			return
		}
		key := j.Route[j.RouteI]
		fid, _ := haul.SlotFrame(key)
		if fr, ok := g.frames.Get(fid); ok && haul.SlotOpen(fr, key, st.Kind) {
			sl, _ := haul.SlotOf(fid, fr, key)
			give := min(st.Qty, sl.Need-sl.Have)
			st.Qty -= give
			g.frames.Set(fid, haul.Give(fr, key, give)) // the material, components or a secondary material (AX-4)
		}
		g.consumeShare(id, key)
		j.RouteI++
		if st.Qty <= 0 {
			g.deleteItem(j.Carry)
			g.endDelivery(id, j)
			return
		}
		g.putStack(j.Carry, st)
		if g.routeWalk(id, &j, st.Kind) {
			return
		}
	}
}

// storeLeftover puts what is left of a load down at the spare stockpile
// cell; a spare that stopped taking it is swapped for the nearest cell
// that does, and with none the load is dropped where the pawn stands.
func (g *Game) storeLeftover(id entity.EntityID, j job.Record, st item.Stack) {
	pos, _ := g.positions.Get(id)
	x, y := pos.X, pos.Y
	if !j.HasSpare || x != j.SpareX || y != j.SpareY || !g.spareTakes(st, x, y) {
		// no spare, out of reach or no longer taking it: the nearest cell
		// that does (the reserved spare itself is never re-picked)
		if cx, cy, ok := g.stockpileCell(st, x, y); ok && g.reserve.TryTile(id, cx, cy) {
			if path, ok := g.pathTo(id, cx, cy); ok && len(path) > 0 {
				j.SpareX, j.SpareY, j.HasSpare = cx, cy, true
				j.Dest, j.DestX, j.DestY = 0, cx, cy
				g.walkOn(id, &j, path)
				return
			}
		}
	}
	st.Carrier = 0
	st.X, st.Y = x, y
	g.mergeOrSet(j.Carry, st)
	g.noteStored(x, y) // haul_priority.go
	g.endDelivery(id, j)
}

// spareTakes: the stockpile cell (x, y) still takes stack st (the zone
// allows it, and the tile is empty or holds the same kind with room).
func (g *Game) spareTakes(st item.Stack, x, y int) bool {
	if z, ok := g.storageAt(x, y); !ok || !z.AllowsStack(st) { // STO-2: the special filters too
		return false
	}
	oid := g.itemAt(x, y)
	if oid == 0 {
		return true
	}
	o, _ := g.stacks.Get(oid)
	return o.Kind == st.Kind && o.Qty+st.Qty <= cellCap(g, o.Kind, x, y)
}

// cellCap is how many of k one stack on (x, y) holds: StackLimit, a hopper's HopperStack.
func cellCap(g *Game, k item.Kind, x, y int) int {
	z, _ := g.storageAt(x, y)
	return z.Kind.StackCap(k.StackLimit())
}

// endDelivery completes a delivery whose load is all handed over.
func (g *Game) endDelivery(id entity.EntityID, j job.Record) {
	g.releaseDelivery(id)
	j.Carry = 0
	j.State = job.Complete
	j.Path = nil
	g.jobs.Set(id, j)
}
