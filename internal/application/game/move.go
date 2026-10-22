package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// Moving built objects (UI-011): MoveBuilding places a reinstall frame
// (Frame.Reinstall = the building) at the destination; builders work it
// like any blueprint (no materials), and on completion the same building
// entity moves there, keeping its bills, fuel, contents, owners and HP.

// Move refusal reasons (player-facing; the usual placement reasons
// ReasonTree, ReasonItem and ReasonSpot are passed through).
const (
	ReasonCantMove = "can't move this"
	ReasonBounds   = "out of bounds"
	ReasonMoveHit  = "blocked"
	ReasonThere    = "already there"
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
	if !ok || !construction.Movable(b.Kind) || b.Deconstruct {
		return false, ReasonCantMove
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
	blocks := construction.DefOf(b.Kind).BlocksMove
	oldCells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	b.X, b.Y, b.Facing = fr.X, fr.Y, fr.Facing
	g.bldgs.Set(bid, b)
	if blocks {
		for _, c := range oldCells {
			g.grid.SetWalkable(c[0], c[1], g.terr.At(c[0], c[1]).Walkable() && g.layerBlockerAt(c[0], c[1]) == 0)
		}
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
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
