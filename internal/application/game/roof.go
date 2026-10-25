package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
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
//   - Natural roofs never collapse and cannot be removed; removing a
//     thick one (NaturalThick, MAPGEN-2: deep rock overhead) is refused
//     with ReasonThickRoof, and no drop pod lands under one.
// No RNG, no map iteration, no wall clock: ties go by row-major index.

// RoofCollapseView is a recent roof collapse, for presentation (dust).
type RoofCollapseView struct {
	X, Y int
	Tick uint64
}

// roofCollapse is a collapse stamp, kept for an hour.
type roofCollapse struct {
	x, y int
	tick uint64
}

// roofIndex is state derived from g.roofs (never saved).
type roofIndex struct {
	// plans: row-major indices of tiles with a build or removal plan,
	// ascending (so ties break by y, then x).
	plans []int32
	// counts: roofed cells per room id; valid only while countsOK.
	counts   []int
	countsOK bool
	// unsupported: a Built roof lost its support at the last rebuild.
	unsupported bool
}

// roofTries is how many of the nearest planned tiles a builder tries
// (each costs a path search) before giving up for now.
const roofTries = 3

const roofPlans = roof.PlanBuild | roof.PlanRemove

// ReasonThickRoof is why a thick natural roof can't be removed.
const ReasonThickRoof = "Thick rock overhead"

// ensureRoofGrid sizes the roof grid on first use.
func (g *Game) ensureRoofGrid() {
	if n := g.grid.Width() * g.grid.Height(); len(g.roofs) != n {
		g.roofs = make([]roof.Cell, n)
		g.roofx = roofIndex{}
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
	}
}

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
	counts := g.roofx.counts[:0]
	for range g.rooms {
		counts = append(counts, 0)
	}
	if len(g.roofs) == len(g.roomGrid) {
		for i, r := range g.roomGrid {
			if r >= 0 && g.roofs[i].Roofed() {
				counts[r]++
			}
		}
	}
	g.roofx.counts, g.roofx.countsOK = counts, true
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
	field := roof.SupportField(w, h, func(x, y int) bool { return grid[y*w+x] == room.Wall })
	for i, c := range g.roofs {
		n := c &^ roof.Supported
		if field[i] {
			n |= roof.Supported
		}
		if grid[i] == room.Wall {
			n &^= roof.PlanBuild
		}
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
		c := g.roofs[i]
		if c.Roofed() || !c.Has(roof.Supported) || c&(roofPlans|roof.NoAuto) != 0 {
			continue
		}
		g.setRoof(i, c|roof.PlanBuild)
	}
}

// designateRoof plans roofs over (build) or roof removal from (!build)
// the rectangle.
func (g *Game) designateRoof(x1, y1, x2, y2 int, build bool) {
	g.ensureRooms()
	g.ensureRoofGrid()
	minX, minY, maxX, maxY := g.clipRect(x1, y1, x2, y2)
	w := g.grid.Width()
	thick := -1 // the first thick roof a removal met
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			i := y*w + x
			c := g.roofs[i]
			if !build && thick < 0 && c.Thick() {
				thick = i
			}
			if build {
				c &^= roof.PlanRemove | roof.NoAuto
				if g.roomGrid[i] != room.Wall && c.Has(roof.Supported) && !c.Roofed() {
					c |= roof.PlanBuild
				}
			} else {
				c = c&^roof.PlanBuild | roof.NoAuto
				if c.Has(roof.Built) && !c.Has(roof.Natural) {
					c |= roof.PlanRemove
				}
			}
			g.setRoof(i, c)
		}
	}
	if thick >= 0 {
		g.message(thick%w, thick/w, "%s: that roof can't be removed.", ReasonThickRoof)
	}
}

// clipRect normalizes a rectangle and clips it to the map; an empty
// result has max < min.
func (g *Game) clipRect(x1, y1, x2, y2 int) (minX, minY, maxX, maxY int) {
	minX, minY, maxX, maxY = normalizeRect(x1, y1, x2, y2)
	return max(minX, 0), max(minY, 0), min(maxX, g.grid.Width()-1), min(maxY, g.grid.Height()-1)
}

// clearRoofPlans drops roof plans inside the (normalized) rectangle.
// NoAuto stays: a cancelled removal does not invite auto-roofing back.
func (g *Game) clearRoofPlans(minX, minY, maxX, maxY int) {
	if len(g.roofs) == 0 {
		return
	}
	minX, minY, maxX, maxY = g.clipRect(minX, minY, maxX, maxY)
	w := g.grid.Width()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			g.setRoof(y*w+x, g.roofs[y*w+x]&^roofPlans)
		}
	}
}

// roofWork reports the job a planned tile needs, if any: Roof for a
// supported, unroofed build plan, Unroof for a built roof planned for
// removal.
func roofWork(c roof.Cell) (job.Kind, bool) {
	switch {
	case c.Has(roof.PlanBuild|roof.Supported) && !c.Roofed():
		return job.Roof, true
	case c.Has(roof.PlanRemove|roof.Built) && !c.Has(roof.Natural):
		return job.Unroof, true
	}
	return job.None, false
}

// assignRoof gives builder id the nearest roof job, if any: the nearest
// unreserved planned tile (Chebyshev, as assignConstruct; ties by y, then
// x), trying a few in order until one is reachable.
func (g *Game) assignRoof(id entity.EntityID) bool {
	if len(g.roofx.plans) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	w := g.grid.Width()
	type cand struct{ i, d int }
	var best [roofTries]cand
	n := 0
	for _, p := range g.roofx.plans {
		i := int(p)
		if _, ok := roofWork(g.roofs[i]); !ok {
			continue
		}
		x, y := i%w, i/w
		if _, taken := g.reserve.TileOwner(x, y); taken {
			continue
		}
		if kind, _ := roofWork(g.roofs[i]); g.postponedFor(id, kind, 0, x, y) {
			continue // SIM-027: out of reach lately
		}
		c := cand{i, pathing.Dist(pos.X, pos.Y, x, y)}
		// keep the roofTries nearest, in (distance, index) order; plans
		// ascend by index, so a strict compare keeps the earlier tile
		k := n
		for k > 0 && c.d < best[k-1].d {
			k--
		}
		if k >= roofTries {
			continue
		}
		if n < roofTries {
			n++
		}
		copy(best[k+1:n], best[k:n-1])
		best[k] = c
	}
	for _, c := range best[:n] {
		x, y := c.i%w, c.i/w
		kind, _ := roofWork(g.roofs[c.i])
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
	if g.grid.Walkable(x, y) {
		return x, y, true
	}
	bx, by, bd := 0, 0, 1<<30
	for _, d := range [8][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		nx, ny := x+d[0], y+d[1]
		if !g.grid.Walkable(nx, ny) {
			continue
		}
		if dd := pathing.Dist(pos.X, pos.Y, nx, ny); dd < bd && g.reachable(id, nx, ny) {
			bx, by, bd = nx, ny, dd
		}
	}
	return bx, by, bd < 1<<30
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
		c &^= roof.Built | roof.PlanRemove
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

// checkRoofSupport collapses unsupported built roofs (climate interval):
// one notice per batch, at the first fallen tile (by y, then x), and a
// dust stamp kept for an hour. Natural roofs never fall; nobody is hurt.
func (g *Game) checkRoofSupport() {
	now := uint64(g.clock.Now())
	kept := g.roofCollapses[:0]
	for _, s := range g.roofCollapses {
		if now-s.tick < clock.TicksPerHour {
			kept = append(kept, s)
		}
	}
	g.roofCollapses = kept
	if g.roomDirty {
		g.ensureRooms() // support is recomputed on the rebuild
	}
	if !g.roofx.unsupported {
		return
	}
	g.roofx.unsupported = false
	w := g.grid.Width()
	first := -1
	for i, c := range g.roofs {
		if !c.Has(roof.Built) || c.Has(roof.Supported) || c.Has(roof.Natural) {
			continue
		}
		g.setRoof(i, c&^(roof.Built|roofPlans))
		if first < 0 {
			first = i
		}
	}
	if first < 0 {
		return
	}
	x, y := first%w, first/w
	g.message(x, y, "A roof collapsed near (%d, %d) — no support within %d tiles.", x, y, roof.SupportRange)
	g.roofCollapses = append(g.roofCollapses, roofCollapse{x: x, y: y, tick: now})
}

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
	g.roofs, g.roofx = nil, roofIndex{}
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
