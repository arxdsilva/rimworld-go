// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"

	"rimworld-go/internal/application/game/roofing"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/terrain"
)

// Roofs (SIM-018): per-tile roofs that keep rooms warm and keep the sky
// (sun, moon, rain and snow) off. Closed rooms of at most
// roof.AutoMaxCells cells are planned automatically; the player plans
// more with DesignateRoof and takes roofs down with DesignateRemoveRoof.
//
// Invariants:
//   - g.roofs is empty or exactly w*h (sized on first use);
//   - every write goes through setRoof, which keeps roofx.plans (the
//     sorted, row-major indices of tiles with PlanBuild or PlanRemove) and
//     the per-room roofed counts consistent;
//   - Supported is derived from the room grid's boundaries on every room
//     rebuild; unsupported Built roofs fall at the next climate interval;
//   - Natural roof loss is armed by actual removal of a support; thick
//     roofs leave new rock when they collapse. Thin roofs may be removed.
// Collapse damage uses the saved body stream, never wall clock or map iteration.

// roofCollapse is a collapse stamp, kept for an hour.
type roofCollapse struct {
	x, y int
	tick uint64
}

// roofIndex is derived except for pending support loss, which is saved.
type roofIndex struct {
	// plans: row-major indices of tiles with a build or removal plan,
	// ascending (so ties break by y, then x).
	plans []int32
	// counts: roofed cells per room id; valid only while countsOK.
	counts   []int
	countsOK bool
	// unsupported: a Built roof lost its support at the last rebuild.
	unsupported bool
	loss        []int32 // pending natural-roof support losses; saved until the climate pass
	// ver moves whenever a tile gains or loses its roof (Roofed changes),
	// and never goes back, even when the grid is replaced: what is cached
	// from the roofs (a wind turbine's band, power_gen.go) is keyed on it.
	ver uint64
}

const roofPlans, roofCancel = roof.PlanBuild | roof.PlanRemove, roof.PlanBuild | roof.PlanRemove | roof.AreaBuild // Cancel: plans and build area

// ReasonThickRoof is why a thick natural roof can't be removed.
const ReasonThickRoof = "Thick rock overhead"

// ensureRoofGrid sizes the roof grid on first use.
func (g *Game) ensureRoofGrid() {
	if n := g.grid.Width() * g.grid.Height(); len(g.roofs) != n {
		g.roofs = make([]roof.Cell, n)
		g.roofx = roofIndex{ver: g.roofx.ver + 1}
	}
}

// setRoof writes tile i and keeps the derived index current.
func (g *Game) setRoof(i int, c roof.Cell) {
	old := g.roofs[i]
	if old == c {
		return
	}
	g.roofs[i] = c
	if wasPlan, isPlan := old&roofPlans != 0, c&roofPlans != 0; wasPlan != isPlan {
		k, found := slices.BinarySearch(g.roofx.plans, int32(i))
		if isPlan && !found {
			g.roofx.plans = slices.Insert(g.roofx.plans, k, int32(i))
		} else if !isPlan && found {
			g.roofx.plans = slices.Delete(g.roofx.plans, k, k+1)
		}
	}
	if old.Roofed() != c.Roofed() {
		g.roofx.countsOK = false
		g.roofx.ver++
	}
}

// roofVer counts the roof changes so far (see roofIndex.ver).
func (g *Game) roofVer() uint64 { return g.roofx.ver }

// roofAt is the roof cell at (x, y); out of bounds is no roof.
func (g *Game) roofAt(x, y int) roof.Cell {
	if !g.grid.InBounds(x, y) || len(g.roofs) == 0 {
		return 0
	}
	return g.roofs[y*g.grid.Width()+x]
}

// underSky reports a tile open to the sky: no roof over it. Walls and
// doors count as covered.
func (g *Game) underSky(x, y int) bool {
	if g.roomAt(x, y) == room.Wall {
		return false
	}
	return !g.roofAt(x, y).Roofed()
}

// roomRoofed is the roofed share of room r's cells, in permille.
func (g *Game) roomRoofed(r int) int {
	g.ensureRooms()
	if r < 0 || r >= len(g.rooms) {
		return 0
	}
	if !g.roofx.countsOK || len(g.roofx.counts) != len(g.rooms) {
		g.countRoofed()
	}
	return g.roofx.counts[r] * 1000 / max(1, g.rooms[r].contents.Cells)
}

// countRoofed recounts the roofed cells of every room (once per change).
func (g *Game) countRoofed() {
	g.roofx.counts = roof.CountRooms(g.roofx.counts, len(g.rooms), g.roomGrid, g.roofs)
	g.roofx.countsOK = true
}

// afterRoomsRebuilt runs after every room rebuild: support is recomputed
// and closed rooms are planned for roofs.
func (g *Game) afterRoomsRebuilt() {
	g.refreshRoofSupport()
	g.autoRoof()
}

// refreshRoofSupport recomputes the Supported flag of every tile from the
// room grid's boundaries (rock, walls and doors). Plans over walls are
// dropped; a Built roof left without support is flagged for collapse.
func (g *Game) refreshRoofSupport() {
	g.ensureRoofGrid()
	g.roofx.countsOK = false // room ids changed
	w, h := g.grid.Width(), g.grid.Height()
	grid := g.roomGrid
	cols := g.buildingCells(construction.Column) // A2: a column holds the roof like a wall
	field := roof.SupportField(w, h, func(x, y int) bool { return grid[y*w+x] == room.Wall || cols.has(y*w+x) })
	for i, c := range g.roofs {
		n := roof.WithSupport(c, field[i], grid[i] == room.Wall)
		if n.Has(roof.Built) && !n.Has(roof.Supported) && !n.Has(roof.Natural) {
			g.roofx.unsupported = true
		}
		g.setRoof(i, n)
	}
}

// autoRoof plans roofs over closed rooms of at most roof.AutoMaxCells.
func (g *Game) autoRoof() {
	for i, r := range g.roomGrid {
		if r < 0 || g.rooms[r].contents.Cells > roof.AutoMaxCells {
			continue
		}
		g.setRoof(i, roof.AutoPlan(g.roofs[i]))
	}
}

// designateRoof plans roofs over (build) or roof removal from (!build)
// the rectangle.
func (g *Game) designateRoof(x1, y1, x2, y2 int, build bool) {
	g.ensureRooms()
	g.ensureRoofGrid()
	minX, minY, maxX, maxY := roof.ClipRect(g.grid.Width(), g.grid.Height(), x1, y1, x2, y2)
	w := g.grid.Width()
	thick := -1 // the first thick roof a removal met
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			i := y*w + x
			c := g.roofs[i]
			if !build && thick < 0 && c.Thick() {
				thick = i
			}
			c = roof.Designate(c, g.roomGrid[i] == room.Wall, build)
			g.setRoof(i, c)
		}
	}
	if thick >= 0 {
		g.message(thick%w, thick/w, "%s: that roof can't be removed.", ReasonThickRoof)
	}
}

// clearRoofPlans drops the roof bits of mask inside the rectangle: Cancel
// the plans and the build area (NoAuto stays: a cancelled removal does not
// invite auto-roofing back); the Clear area orders one area each (AX-1).
func (g *Game) clearRoofPlans(minX, minY, maxX, maxY int, mask roof.Cell) {
	if len(g.roofs) == 0 {
		return
	}
	minX, minY, maxX, maxY = roof.ClipRect(g.grid.Width(), g.grid.Height(), minX, minY, maxX, maxY)
	w := g.grid.Width()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			g.setRoof(y*w+x, g.roofs[y*w+x]&^mask)
		}
	}
}

// roofWork reports the job a planned tile needs, if any: Roof for a
// supported, unroofed build plan, Unroof for a removable roof planned for
// removal.
func roofWork(c roof.Cell) (job.Kind, bool) { return roof.Work(c) }

// assignRoof gives builder id the nearest roof job, if any: the nearest
// unreserved planned tile (Chebyshev, as assignConstruct; ties by y, then
// x), trying a few in order until one is reachable.
func (g *Game) assignRoof(id entity.EntityID) bool {
	if len(g.roofx.plans) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	w := g.grid.Width()
	best, n := roof.NearestPlans(g.roofx.plans, w, pos.X, pos.Y, func(i int) bool {
		kind, ok := roofWork(g.roofs[i])
		if !ok {
			return false
		}
		x, y := i%w, i/w
		_, taken := g.reserve.TileOwner(x, y)
		return !taken && !g.postponedFor(id, kind, 0, x, y)
	})
	for _, i := range best[:n] {
		x, y := i%w, i/w
		kind, _ := roofWork(g.roofs[i])
		tx, ty, ok := g.roofSpot(id, pos, x, y)
		if !ok {
			g.postponeTile(id, kind, x, y)
			continue
		}
		work := roof.BuildWork
		if kind == job.Unroof {
			work = roof.RemoveWork
		}
		if g.startJob(id, job.Record{Kind: kind, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: work}) {
			g.reserve.TryTile(id, x, y)
			return true
		}
	}
	return false
}

// roofSpot is where a builder stands to roof (x, y): the tile itself if
// walkable (startJob checks the path), else the walkable 8-neighbour
// nearest the builder that it can reach.
func (g *Game) roofSpot(id entity.EntityID, pos Position, x, y int) (int, int, bool) {
	return roof.WorkSpot(pos.X, pos.Y, x, y, g.grid.Walkable, func(x, y int) bool { return g.reachable(id, x, y) })
}

// doRoof advances a Roof or Unroof job. A plan cancelled meanwhile (or a
// tile that no longer needs the work) fails the job, which releases the
// reservation.
func (g *Game) doRoof(id entity.EntityID, j job.Record) {
	if len(g.roofs) == 0 || !g.grid.InBounds(j.DestX, j.DestY) {
		g.failRoof(id, j)
		return
	}
	i := j.DestY*g.grid.Width() + j.DestX
	if kind, ok := roofWork(g.roofs[i]); !ok || kind != j.Kind {
		g.failRoof(id, j)
		return
	}
	u := g.workProgress(id, skConstruction, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	c := g.roofs[i]
	if j.Kind == job.Roof {
		c = c&^roof.PlanBuild | roof.Built
	} else {
		c &^= roof.Built | roof.Natural | roof.PlanRemove
	}
	g.setRoof(i, c) // invalidates the roofed counts heat reads
	g.reserve.ReleaseOwner(id)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) failRoof(id entity.EntityID, j job.Record) {
	g.reserve.ReleaseOwner(id)
	j.State, j.Fail = job.Fail, job.TargetGone
	g.jobs.Set(id, j)
}

// noteRoofLoss runs only at authoritative support removals. Persist the
// candidate set so read-model rebuilds or a save between removal and
// climate cannot suppress a cave-in.
func noteRoofLoss(g *Game, x, y int) {
	g.roofx.loss = roof.NoteLoss(g.roofx.loss, g.grid.Width(), g.grid.Height(), x, y, g.roofs)
}

// MiningRoofRisk is the detached native preview for removing one rock.
func (g *Game) MiningRoofRisk(x, y int) []roof.Risk {
	if !terrain.IsRockWall(g.terr.At(x, y)) {
		return nil
	}
	return roof.RemovalRisk(g.grid.Width(), g.grid.Height(), x, y, g.roofs, func(x, y int) bool {
		b, _ := g.bldgs.Get(g.layerBldgAt(x, y, construction.LayerObject))
		return g.isBoundary(x, y) || b.Kind == construction.Column
	})
}

// checkRoofSupport settles the batch at the climate interval. It is
// never invoked from World() or a preview.
func (g *Game) checkRoofSupport() {
	now := uint64(g.clock.Now())
	g.roofCollapses = slices.DeleteFunc(g.roofCollapses, func(s roofCollapse) bool { return now-s.tick >= clock.TicksPerHour })
	if g.roomDirty {
		g.ensureRooms()
	}
	if !g.roofx.unsupported && len(g.roofx.loss) == 0 {
		return
	}
	// A newly collapsed rock can be mined again before the incremental
	// topology rebuild observes it. Re-read pending loss footprints even
	// when two edits left the cached room topology unchanged.
	if len(g.roofx.loss) > 0 {
		g.refreshRoofSupportAround(g.roofx.loss)
	}
	g.roofx.unsupported = false
	falling := roof.Falling(g.roofs, g.roofx.loss)
	g.roofx.loss = nil
	w := g.grid.Width()
	for _, i := range falling {
		c := g.roofs[i]
		x, y := i%w, i/w
		if c.Thick() {
			roofing.Collapse(roofFallHost{combatDebris{g}}, x, y, true)
		} else {
			g.setRoof(i, c&^(roof.Built|roof.Natural|roofPlans))
			roofing.Collapse(roofFallHost{combatDebris{g}}, x, y, false)
		}
	}
	if len(falling) > 0 {
		i := falling[0]
		x, y := i%w, i/w
		g.threat("Roof collapse", x, y, "A roof collapsed near (%d, %d) — no support within %d tiles.", x, y, roof.SupportRange)
		g.roofCollapses = append(g.roofCollapses, roofCollapse{x: x, y: y, tick: now})
	}
}

type roofFallHost struct{ combatDebris }

func (h roofFallHost) Victims(x, y int) []entity.EntityID {
	g := h.g
	var out []entity.EntityID
	for _, ids := range [][]entity.EntityID{g.pawnIDs, g.raiderIDs, g.animalIDs} {
		for _, id := range ids {
			p, ok := g.positions.Get(id)
			if ok && p.X == x && p.Y == y && (g.onMap(id) || g.animals.Has(id)) {
				out = append(out, id)
			}
		}
	}
	return out
}
func (h roofFallHost) Crush(id entity.EntityID, thick bool) {
	g := h.g
	if thick {
		g.hurt(id, 99999, 0)
		return
	}
	rng := g.stream(streamBody)
	dmg := 15 + rng.IntN(16)
	if g.animals.Has(id) {
		g.hurt(id, dmg, 0)
		return
	}
	part := body.Jaw
	if rng.IntN(2) == 0 {
		part = body.Brain
	}
	hw := hits(g)
	hw.HurtHit(id, hw.LandOn(id, part, damage.Hit{Form: damage.FormCrush, Amount: dmg}, rng.IntN), 0)
}
func (h roofFallHost) ClearSite(x, y int) {
	g := h.g
	for layer := range construction.NumLayers {
		if bid := g.layerBldgAt(x, y, construction.Layer(layer)); bid != 0 {
			g.removeBuildingAs(bid, false)
		}
		if fid := g.layerFrameAt(x, y, construction.Layer(layer)); fid != 0 {
			g.cancelFrame(fid)
		}
	}
	if pid := g.plantAt(x, y); pid != 0 {
		g.removePlant(pid)
	}
}
func (h roofFallHost) FillRock(x, y int) {
	g := h.g
	g.setTerrain(x, y, terrain.Rock)
	i := y*g.grid.Width() + x
	g.setRoof(i, g.roofs[i]&^roofPlans|roof.Natural|roof.NaturalThick|roof.Supported)
}
func (h roofFallHost) Rubble(x, y int) { h.g.addDirt(y*h.g.grid.Width()+x, 250) }

// roofOnMined leaves a natural roof where rock was mined away.
func (g *Game) roofOnMined(x, y int) {
	if !g.grid.InBounds(x, y) {
		return
	}
	g.ensureRoofGrid()
	i := y*g.grid.Width() + x
	g.setRoof(i, g.roofs[i]&^(roof.Built|roofPlans)|roof.Natural)
}

// fillRoofView copies roofs and collapses into the view (World fills the
// sun clock).
func (g *Game) fillRoofView(w *WorldView) {
	w.RoofCells = shareLayer(&g.viewRoof, g.roofs) // indexes.go: shared, copy-on-write
	if len(g.roofCollapses) > 0 {
		w.RoofCollapses = make([]RoofCollapseView, len(g.roofCollapses))
		for i, s := range g.roofCollapses {
			w.RoofCollapses[i] = RoofCollapseView{X: s.x, Y: s.y, Tick: s.tick}
		}
	}
}

// exportRoofs is the roof grid for a save.
func (g *Game) exportRoofs() []uint8 {
	if len(g.roofs) == 0 {
		return nil
	}
	out := make([]uint8, len(g.roofs))
	for i, c := range g.roofs {
		out[i] = uint8(c)
	}
	return out
}

// importRoofs installs a saved roof grid; nil (an older save) leaves
// every room to be auto-roofed. Either way the rooms are rebuilt, which
// recomputes support and plans closed rooms.
func (g *Game) importRoofs(cells []uint8) {
	g.roofs, g.roofx = nil, roofIndex{ver: g.roofx.ver + 1}
	if n := g.grid.Width() * g.grid.Height(); len(cells) == n && n > 0 {
		g.ensureRoofGrid()
		for i, c := range cells {
			g.setRoof(i, roof.Cell(c))
		}
	}
	g.markRoomsDirty()
}

// RoofRooms completes every planned roof at once (dev scenarios, tests):
// closed rooms are auto-planned first.
func (g *Game) RoofRooms() {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.ensureRooms()
	for _, p := range slices.Clone(g.roofx.plans) {
		i := int(p)
		if k, ok := roofWork(g.roofs[i]); ok && k == job.Roof {
			g.setRoof(i, g.roofs[i]&^roof.PlanBuild|roof.Built)
		}
	}
}
