package game

import (
	"rimworld-go/internal/application/game/orders"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
	"slices"
)

// Moving built objects (UI-011): MoveBuilding places a reinstall frame
// (Frame.Reinstall = the building) at the destination; builders work it
// like any blueprint (no materials), and on completion the same building
// entity moves there, keeping its bills, fuel, contents, owners and HP.

// Move refusal reasons (player-facing; the usual placement reasons
// ReasonTree, ReasonItem and ReasonSpot are passed through).
const (
	ReasonCantMove = "can't move this"
	// ReasonSwapPending: a material change is under way (PLAY-011).
	ReasonSwapPending = "Material change in progress"
	ReasonBounds      = "out of bounds"
	ReasonMoveHit     = "blocked"
	ReasonThere       = "already there"
)

// minReinstallWork is the least work a reinstall takes, in ticks.
const minReinstallWork = 60

// CanMoveTo reports whether building bid may be reinstalled at x, y facing
// facing, and why not ("can't move this", "blocked", "out of bounds",
// "already there" ...). Its own cells count as free.
func (g *Game) CanMoveTo(bid entity.EntityID, facing string, x, y int) (bool, string) {
	return g.canMoveTo(bid, parseFacingName(facing), x, y)
}

// canMoveTo is CanPlaceAt's footprint rule with bid's own cells, work spot
// and pending reinstall frame treated as free.
func (g *Game) canMoveTo(bid entity.EntityID, f construction.Facing, x, y int) (bool, string) {
	b, ok := g.bldgs.Get(bid)
	if ok && isNatural(b.Kind) {
		return false, ReasonNatural // setup_mapgen.go: a steam vent stays
	}
	if !ok || !construction.Movable(b.Kind) || b.Deconstruct {
		return false, ReasonCantMove
	}
	if g.swapFrameFor(bid) != 0 {
		return false, ReasonSwapPending
	}
	def := construction.DefOf(b.Kind)
	if !def.Rotatable {
		f = construction.South
	}
	if x == b.X && y == b.Y && f == b.Facing {
		return false, ReasonThere
	}
	own := func(cx, cy int) bool { return construction.Covers(b.Kind, b.Facing, b.X, b.Y, cx, cy) }
	cells, spot, hasSpot := construction.Footprint(b.Kind, f, x, y)
	for _, c := range cells {
		if !g.grid.InBounds(c[0], c[1]) {
			return false, ReasonBounds
		}
	}
	for _, c := range cells {
		cx, cy := c[0], c[1]
		if g.otherBldgAt(bid, cx, cy, def.Layer) || g.otherFrameAt(bid, cx, cy, def.Layer) ||
			terrain.IsRockWall(g.terr.At(cx, cy)) || (!g.grid.Walkable(cx, cy) && !own(cx, cy)) {
			return false, ReasonMoveHit
		}
		if pid := g.plantAt(cx, cy); pid != 0 {
			if p, _ := g.plants.Get(pid); plant.DefOf(p.Species).Trunk && p.Species != plant.Cactus {
				return false, ReasonTree
			}
		}
		if def.BlocksMove && !own(cx, cy) && g.itemAt(cx, cy) != 0 {
			return false, ReasonItem
		}
	}
	if hasSpot {
		sx, sy := spot[0], spot[1]
		if !g.grid.InBounds(sx, sy) || (!g.grid.Walkable(sx, sy) && !own(sx, sy)) || g.otherBlockingFrameAt(bid, sx, sy) {
			return false, ReasonSpot
		}
	}
	if def.BlocksMove {
		for _, c := range cells {
			if g.otherWorkSpot(bid, c[0], c[1]) {
				return false, ReasonSpot
			}
		}
	}
	return true, ""
}

// otherBldgAt: a building other than bid on layer covers (x, y).
func (g *Game) otherBldgAt(bid entity.EntityID, x, y int, layer construction.Layer) bool {
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if ok && id != bid && construction.DefOf(b.Kind).Layer == layer && construction.Covers(b.Kind, b.Facing, b.X, b.Y, x, y) {
			return true
		}
	}
	return false
}

// otherFrameAt: a frame on layer covers (x, y), bid's own reinstall excluded.
func (g *Game) otherFrameAt(bid entity.EntityID, x, y int, layer construction.Layer) bool {
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if ok && fr.Reinstall != uint64(bid) && construction.DefOf(fr.Kind).Layer == layer &&
			construction.Covers(fr.Kind, fr.Facing, fr.X, fr.Y, x, y) {
			return true
		}
	}
	return false
}

// otherBlockingFrameAt: a planned blocker covers (x, y), bid's own excluded.
func (g *Game) otherBlockingFrameAt(bid entity.EntityID, x, y int) bool {
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if ok && fr.Reinstall != uint64(bid) && construction.DefOf(fr.Kind).BlocksMove &&
			construction.Covers(fr.Kind, fr.Facing, fr.X, fr.Y, x, y) {
			return true
		}
	}
	return false
}

// otherWorkSpot: (x, y) is the work spot of a building or frame not bid's.
func (g *Game) otherWorkSpot(bid entity.EntityID, x, y int) bool {
	at := func(k construction.Kind, f construction.Facing, bx, by int) bool {
		_, s, ok := construction.Footprint(k, f, bx, by)
		return ok && s[0] == x && s[1] == y
	}
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && id != bid && at(b.Kind, b.Facing, b.X, b.Y) {
			return true
		}
	}
	for _, id := range g.frameIDs {
		if fr, ok := g.frames.Get(id); ok && fr.Reinstall != uint64(bid) && at(fr.Kind, fr.Facing, fr.X, fr.Y) {
			return true
		}
	}
	return false
}

// moveBuilding plans the move of bid (a second plan replaces the first).
func (g *Game) moveBuilding(bid entity.EntityID, f construction.Facing, x, y int) {
	if f > construction.West {
		f = construction.South
	}
	if ok, _ := g.canMoveTo(bid, f, x, y); !ok {
		return
	}
	b, _ := g.bldgs.Get(bid)
	if !construction.DefOf(b.Kind).Rotatable {
		f = construction.South
	}
	g.dropReinstall(bid)
	id := g.entities.Next()
	g.frameIDs = append(g.frameIDs, id)
	g.frames.Set(id, construction.Frame{
		Kind: b.Kind, Stuff: b.Stuff, Facing: f, X: x, Y: y,
		// Need 0: always Ready, so haulers never supply it
		WorkLeft:  max(minReinstallWork, construction.WorkFor(b.Kind, b.Stuff)/2),
		Reinstall: uint64(bid),
	})
	g.noteFrame(id) // indexes.go
}

// dropReinstall deletes any reinstall frame planned for bid.
func (g *Game) dropReinstall(bid entity.EntityID) {
	kept := g.frameIDs[:0]
	for _, fid := range g.frameIDs {
		if fr, ok := g.frames.Get(fid); ok && fr.Reinstall == uint64(bid) {
			g.frames.Delete(fid)
			continue
		}
		kept = append(kept, fid)
	}
	g.frameIDs = kept
}

// completeMove finishes reinstall frame id: the building moves there.
func (g *Game) completeMove(id entity.EntityID, fr construction.Frame) {
	g.frames.Delete(id)
	kept := g.frameIDs[:0]
	for _, fid := range g.frameIDs {
		if fid != id {
			kept = append(kept, fid)
		}
	}
	g.frameIDs = kept
	bid := entity.EntityID(fr.Reinstall)
	b, ok := g.bldgs.Get(bid)
	if !ok || b.Deconstruct {
		return // gone or being taken down: the move is dropped
	}
	g.releaseSwap(bid) // placement.go: a material change left behind is built new
	blocks := construction.DefOf(b.Kind).BlocksMove
	oldCells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	b.X, b.Y, b.Facing = fr.X, fr.Y, fr.Facing
	g.bldgs.Set(bid, b)
	g.noteBuilding(bid) // indexes.go: it stands somewhere new
	if blocks {
		for _, c := range oldCells {
			g.grid.SetWalkable(c[0], c[1], g.cellWalkable(c[0], c[1]))
			g.fogOpen(c[0], c[1]) // fog.go
		}
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	g.clearPlanFootprint(b.Kind, cells) // plan.go
	for _, c := range cells {
		if pid := g.plantAt(c[0], c[1]); pid != 0 {
			g.removePlant(pid)
		}
		g.clearGrass(c[0], c[1]) // grass.go
	}
	if blocks {
		for _, c := range cells {
			g.grid.SetWalkable(c[0], c[1], false)
		}
		for _, c := range cells {
			g.ejectPawns(c[0], c[1])
		}
	}
	g.markRoomsDirty()
	g.structVer++ // SIM-029: link/autotile caches
	// sleepers and users were headed for (or lying on) the old cells
	for _, pid := range g.pawnIDs {
		j, ok := g.jobs.Get(pid)
		if ok && j.Kind != job.None && j.Target == bid && j.State != job.Idle && j.State != job.Complete {
			g.interrupt(pid, job.Interrupted)
		}
	}
}

// movingTo is where building bid's planned move goes, nil when none.
func (g *Game) movingTo(bid entity.EntityID) *[2]int {
	if _, ok := g.bldgs.Get(bid); !ok {
		return nil
	}
	for _, fid := range g.frameIDs {
		if fr, ok := g.frames.Get(fid); ok && fr.Reinstall == uint64(bid) {
			return &[2]int{fr.X, fr.Y}
		}
	}
	return nil
}

// --- game/orders (AX-1): the Architect's remaining orders and zone clears.

// ordersOf is the orders component, built on first use (never saved: its
// marks travel as saveState.Orders).
func ordersOf(g *Game) *orders.Orders {
	if g.postponed.ord == nil {
		g.postponed.ord = orders.New(ordersHost{g})
	}
	return g.postponed.ord
}

// applyOrder routes the AX-1 designations (tick.go applyOne).
func applyOrder(g *Game, c command.Command) {
	minX, minY, maxX, maxY := zone.Normalize(c.X, c.Y, c.X2, c.Y2)
	o := ordersOf(g)
	switch c.Kind {
	case command.DesignateUninstall:
		o.Uninstall.Mark(minX, minY, maxX, maxY, c.Enabled)
	case command.Reinstall:
		if why := o.Install.Place(entity.EntityID(c.Target), construction.Facing(c.Facing), c.X, c.Y); why != "" {
			g.message(c.X, c.Y, "Can't reinstall here: %s.", why)
		}
	case command.DesignateStrip:
		o.Strip.Mark(minX, minY, maxX, maxY, c.Enabled)
	case command.DesignateOpen:
		o.Open.Mark(minX, minY, maxX, maxY, c.Enabled)
	case command.DesignateClaim:
		o.Claim.Mark(minX, minY, maxX, maxY)
	case command.DesignateChopWood:
		markCutWhere(g, minX, minY, maxX, maxY, c.Enabled, orders.Choppable)
	case command.DesignateSlaughter:
		markHuntWhere(g, minX, minY, maxX, maxY, c.Enabled, orders.Slaughterable)
	case command.DesignateRemoveFloor:
		markDeconstructWhere(g, minX, minY, maxX, maxY, c.Enabled, construction.LayerFloor)
	case command.DesignateDeleteZone:
		g.zones.Clear(zone.Rect{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}) // zones.go: shrinks or splits
	case command.DesignateClearRoofArea:
		g.clearRoofPlans(minX, minY, maxX, maxY, roof.PlanBuild|roof.AreaBuild)
	case command.DesignateClearNoRoofArea:
		g.clearRoofPlans(minX, minY, maxX, maxY, roof.PlanRemove|roof.NoAuto)
	}
}

// unmarkOrder is undesignate for the AX-1 orders (tick.go).
func unmarkOrder(g *Game, o command.Order, minX, minY, maxX, maxY int) {
	switch o {
	case command.OrderUninstall:
		ordersOf(g).Uninstall.Mark(minX, minY, maxX, maxY, false)
	case command.OrderStrip:
		ordersOf(g).Strip.Mark(minX, minY, maxX, maxY, false)
	case command.OrderOpen:
		ordersOf(g).Open.Mark(minX, minY, maxX, maxY, false)
	case command.OrderChopWood:
		markCutWhere(g, minX, minY, maxX, maxY, false, orders.Tree)
	case command.OrderSlaughter:
		markHuntWhere(g, minX, minY, maxX, maxY, false, orders.Slaughterable)
	case command.OrderRemoveFloor:
		markDeconstructWhere(g, minX, minY, maxX, maxY, false, construction.LayerFloor)
	}
}

// crateFrame reports reinstall frame fr waiting for a crate (builders
// leave it to the orders' install job, never construct it bare).
func crateFrame(g *Game, fr construction.Frame) bool {
	return fr.Reinstall != 0 && g.stacks.Ref(entity.EntityID(fr.Reinstall)) != nil
}

// fillOrderViews sets the AX-1 building view fields (building_view.go).
func fillOrderViews(g *Game, bv *BuildingView, id entity.EntityID, b construction.Building) {
	bv.AX.Uninstall, bv.AX.OpenMarked = b.Uninstall, ordersOf(g).Open.Marked(id, b)
}

// ordersHost implements orders.Host over the game.
type ordersHost struct{ g *Game }

func (h ordersHost) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h ordersHost) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h ordersHost) SetBuilding(id entity.EntityID, b construction.Building) { h.g.bldgs.Set(id, b) }
func (h ordersHost) Seen(x, y int) bool                                      { return !h.g.fogged(x, y) }
func (h ordersHost) Message(x, y int, text string)                           { h.g.message(x, y, "%s", text) }
func (h ordersHost) Claimed(id entity.EntityID) {
	b, _ := h.g.bldgs.Get(id)
	h.g.autoHome(b.Kind, b.Facing, b.X, b.Y) // home.go: claimed, it joins the Home area (wiki)
	h.g.markRoomsDirty()
	h.g.structVer++
}
func (h ordersHost) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h ordersHost) StartJob(id entity.EntityID, rec job.Record) bool {
	return h.g.startJob(id, rec)
}
func (h ordersHost) SetJob(id entity.EntityID, j job.Record) { h.g.jobs.Set(id, j) }
func (h ordersHost) Work(id entity.EntityID, skilled bool) int {
	if skilled {
		return h.g.workProgress(id, skConstruction, true)
	}
	return h.g.workProgress(id, skNone, false)
}
func (h ordersHost) Spot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.reachableSpot(id, x, y)
}
func (h ordersHost) Free(id, target entity.EntityID, k job.Kind) bool {
	o, taken := h.g.reserve.EntityOwner(target)
	return (!taken || o == id) && !h.g.postponedFor(id, k, target, 0, 0)
}
func (h ordersHost) Claim(id, target entity.EntityID) bool { return h.g.reserve.TryEntity(id, target) }
func (h ordersHost) Walk(id entity.EntityID, j job.Record, x, y int) bool {
	path, ok := h.g.pathTo(id, x, y)
	if ok {
		j.Path, j.PathI, j.TX, j.TY, j.State = path, 0, x, y, job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		h.g.jobs.Set(id, j)
	}
	return ok
}
func (h ordersHost) Stack(id entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(id) }
func (h ordersHost) Pack(id entity.EntityID, st item.Stack) entity.EntityID {
	h.g.removeBuildingAs(id, false) // the crate is the building: nothing refunded
	sid, err := h.g.SpawnStack(st.Kind, st.Qty, st.X, st.Y)
	if err == nil {
		h.g.putStack(sid, st)
	}
	return sid
}
func (h ordersHost) Carry(id, by entity.EntityID) {
	if st, ok := h.g.stacks.Get(id); ok {
		st.Carrier = by
		h.g.putStack(id, st)
	}
}
func (h ordersHost) Fits(k construction.Kind, s item.Kind, f construction.Facing, x, y int) (bool, string) {
	return h.g.canPlaceStuffAt(k, s, f, x, y)
}
func (h ordersHost) FrameIDs() []entity.EntityID { return h.g.frameIDs }
func (h ordersHost) Frame(id entity.EntityID) (construction.Frame, bool) {
	return h.g.frames.Get(id)
}
func (h ordersHost) SetFrame(id entity.EntityID, fr construction.Frame) { h.g.frames.Set(id, fr) }
func (h ordersHost) AddFrame(fr construction.Frame) entity.EntityID {
	id := h.g.entities.Next()
	h.g.frameIDs = append(h.g.frameIDs, id)
	h.g.frames.Set(id, fr)
	h.g.noteFrame(id) // indexes.go
	return id
}
func (h ordersHost) DropFrame(id entity.EntityID) { h.g.cancelFrame(id) }
func (h ordersHost) Install(id, crate entity.EntityID) {
	g := h.g
	fr, ok := g.frames.Get(id)
	st, has := g.stacks.Get(crate)
	if !ok || !has {
		return
	}
	g.deleteItem(crate)
	fr.Reinstall = 0
	g.completeFrame(id, fr) // a new building on the frame's id, as a blueprint finishes
	if b, ok := g.bldgs.Get(id); ok {
		b.Quality, b.Damage = st.Quality, st.Wear
		g.bldgs.Set(id, b)
	}
}
func (h ordersHost) Interred(id entity.EntityID) bool { return h.g.graveFilled(id) }
func (h ordersHost) Exhume(id entity.EntityID) {
	g := h.g
	rec, ok := g.graves[id]
	b, isB := g.bldgs.Get(id)
	if !ok || !isB {
		return
	}
	delete(g.graves, id)
	x, y, _ := g.combat.Tiles.WalkableNear(b.X, b.Y, 3)
	k := item.CorpseHuman
	if rec.Occupant == 0 {
		k = item.Bones // an animal's remains
	}
	sid := g.entities.Next()
	g.itemIDs = append(g.itemIDs, sid)
	g.mergeOrSet(sid, item.Stack{Kind: k, Qty: 1, X: x, Y: y, Of: rec.Occupant, DiedAt: uint64(max(rec.DiedDay-1, 0)) * clock.TicksPerDay})
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		g.grid.SetWalkable(c[0], c[1], g.cellWalkable(c[0], c[1])) // an empty grave is open ground again
	}
	g.structVer++
	g.message(b.X, b.Y, "%s was lifted out of the %s.", rec.Name, construction.DefOf(b.Kind).Name)
}
func (h ordersHost) OpenCasket(id entity.EntityID, on bool) {
	if on {
		h.g.orderOpenCasket(id) // casket.go
	} else if b, ok := h.g.bldgs.Get(id); ok && b.Deconstruct {
		b.Deconstruct, b.WorkLeft = false, 0
		h.g.bldgs.Set(id, b)
	}
}
func (h ordersHost) Sealed(id entity.EntityID) bool {
	b, ok := h.g.bldgs.Get(id)
	return ok && b.Kind == construction.SleeperCasket && b.Occupant != 0
}
func (h ordersHost) PeopleIn(minX, minY, maxX, maxY int) []entity.EntityID {
	var out []entity.EntityID
	for _, ids := range [][]entity.EntityID{h.g.pawnIDs, h.g.raiderIDs} {
		for _, id := range ids {
			if p, ok := h.g.positions.Get(id); ok && p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY {
				out = append(out, id)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
func (h ordersHost) Strippable(id entity.EntityID) bool {
	v, ok := h.g.vital.Get(id)
	p, _ := h.g.positions.Get(id)
	return ok && v.Status == health.Downed && !h.g.fogged(p.X, p.Y) && careGear{h.g}.HasGear(id)
}

// fillCrateView names a crated building's kind (ItemView.AX.Building) and
// material (ItemView.Stuff); anything else is left as it is.
func fillCrateView(iv *ItemView, st item.Stack) {
	if st.Kind == item.MinifiedBuilding {
		iv.AX.Building, iv.Stuff = construction.Kind(st.Building).String(), st.Stuff.String()
	}
}
