package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/region"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
)

// Incremental rooms (NG-1 §6.4, E2). The room partition lives in a
// region.Rooms fed each cell's inputs (boundary, stone wall, smooth
// floor). A rebuild re-reads only the cells whose inputs may have changed
// since the last one — those of every changed building's footprint
// (found by diffing the building list) and every changed terrain cell
// (setTerrain) — and re-floods only their chunks. The room grid is
// patched where rooms changed, temperatures are remapped only for changed
// rooms, roof support is recomputed only around changed boundaries.
// Every output is identical to the full rebuild (rebuildRoomsFull, which
// still runs with regions off and is the self-check's reference).

// roomIndex is the incremental room state (derived, never saved).
type roomIndex struct {
	rooms   *region.Rooms
	grid    *grid.Grid // the grid's map it was built for (a load replaces it)
	sigs    []bldgSig  // buildings at the last rebuild, ascending id
	touched []int32    // terrain cells changed since the last rebuild
	all     bool       // too many touched: re-read every cell
	out     []int      // the room grid it produced last (g.roomGrid unless replaced)
	stamp   []uint32   // roof support scratch
	gen     uint32
}

// bldgSig is what of a building the rooms read.
type bldgSig struct {
	id     entity.EntityID
	kind   construction.Kind
	stuff  item.Kind
	facing construction.Facing
	x, y   int
}

// roomTouch records a terrain change at (x, y) for the next rebuild.
func (g *Game) roomTouch(x, y int) {
	ri := &g.regs.rooms
	if ri.rooms == nil || ri.all {
		return
	}
	if len(ri.touched) >= g.grid.Width()*g.grid.Height()/16 {
		ri.all, ri.touched = true, ri.touched[:0]
		return
	}
	ri.touched = append(ri.touched, int32(y*g.grid.Width()+x))
}

// roomInputs is cell (x, y)'s room inputs, as rebuildRoomsFull reads them:
// a boundary (rock, a wall or a door), a boundary that counts as stone
// (stone stuff or natural rock), open ground floored by itself (smooth
// stone with no floor building).
func (g *Game) roomInputs(x, y int) (boundary, stone, smooth bool) {
	t := g.terr.At(x, y)
	rock := terrain.IsRockWall(t)
	b, ok := g.bldgs.Get(g.layerBldgAt(x, y, construction.LayerObject))
	boundary = rock || ok && (b.Kind == construction.Wall || b.Kind == construction.Door)
	stone = boundary && (ok && (item.IsStoneBlocks(b.Stuff) || b.Stuff == item.Stone) || rock)
	smooth = !boundary && t == terrain.StoneSmooth && g.layerBldgAt(x, y, construction.LayerFloor) == 0
	return boundary, stone, smooth
}

// buildingSigs lists every building's signature, ascending id.
func (g *Game) buildingSigs(out []bldgSig) []bldgSig {
	out = out[:0]
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok {
			out = append(out, bldgSig{id: id, kind: b.Kind, stuff: b.Stuff, facing: b.Facing, x: b.X, y: b.Y})
		}
	}
	if !slices.IsSortedFunc(out, func(a, b bldgSig) int { return int(a.id) - int(b.id) }) {
		slices.SortFunc(out, func(a, b bldgSig) int { return int(a.id) - int(b.id) })
	}
	return out
}

// syncRooms feeds the room map every cell whose inputs may have changed
// since the last rebuild. first reports a new map (every cell fed); bchanged
// lists the cells whose boundary flipped.
func (g *Game) syncRooms() (first bool, bchanged []int32) {
	ri := &g.regs.rooms
	w, h := g.grid.Width(), g.grid.Height()
	feed := func(i int) {
		x, y := i%w, i/w
		b, s, sm := g.roomInputs(x, y)
		if ri.rooms.SetCell(x, y, b, s, sm) {
			bchanged = append(bchanged, int32(i))
		}
	}
	if ri.rooms == nil || ri.grid != g.grid || ri.rooms.Width() != w || ri.rooms.Height() != h {
		*ri = roomIndex{rooms: region.NewRooms(w, h), grid: g.grid}
		for i := 0; i < w*h; i++ {
			feed(i)
		}
		ri.sigs = g.buildingSigs(nil)
		return true, nil
	}
	next := g.buildingSigs(make([]bldgSig, 0, len(ri.sigs)))
	if ri.all {
		for i := 0; i < w*h; i++ {
			feed(i)
		}
	} else {
		touch := func(s bldgSig) {
			cells, _, _ := construction.Footprint(s.kind, s.facing, s.x, s.y)
			for _, c := range cells {
				if g.grid.InBounds(c[0], c[1]) {
					feed(c[1]*w + c[0])
				}
			}
		}
		old := ri.sigs
		a, b := 0, 0
		for a < len(old) || b < len(next) {
			switch {
			case b == len(next) || a < len(old) && old[a].id < next[b].id:
				touch(old[a])
				a++
			case a == len(old) || next[b].id < old[a].id:
				touch(next[b])
				b++
			default:
				if old[a] != next[b] {
					touch(old[a])
					touch(next[b])
				}
				a++
				b++
			}
		}
		for _, i := range ri.touched {
			feed(int(i))
		}
	}
	ri.sigs, ri.touched, ri.all = next, ri.touched[:0], false
	return false, bchanged
}

// rebuildRoomsIncremental is ensureRooms' rebuild through the room map.
func (g *Game) rebuildRoomsIncremental() {
	ri := &g.regs.rooms
	first, bchanged := g.syncRooms()
	rm := ri.rooms
	rm.Update()
	w, h := g.grid.Width(), g.grid.Height()
	n := rm.Count()
	// the grid ensureRooms last produced, or a full one
	full := first || g.roomGrid == nil || len(g.roomGrid) != w*h || len(ri.out) != w*h || &ri.out[0] != &g.roomGrid[0]
	var grid []int
	if full {
		grid = make([]int, w*h)
		for i := range grid {
			grid[i] = rm.At(i%w, i/w)
		}
	} else {
		grid = g.roomGrid
		copied := false
		rm.EachChanged(func(i int) {
			if v := rm.At(i%w, i/w); v != grid[i] {
				if !copied { // the view shares the old grid: copy on write
					grid, copied = slices.Clone(grid), true
				}
				grid[i] = v
			}
		})
	}
	stats := make([]roomStats, n)
	for r := range stats {
		c := rm.Counts(r)
		stats[r].contents = room.Contents{Cells: int(c.Cells), Floored: int(c.Smooth), TotalWalls: int(c.Walls), StoneWalls: int(c.StoneWalls)}
	}
	g.countRoomBuildings(grid, stats)
	for i := range stats {
		stats[i].score = room.Impressiveness(stats[i].contents)
		stats[i].role = room.RoleOf(stats[i].contents)
		stats[i].lit = stats[i].contents.Lamps > 0
	}
	if selfCheck() {
		g.checkRoomsAgainstDetect(grid, stats)
	}
	if full {
		g.roomTemps = g.remapRoomTemps(grid, n)
	} else {
		g.roomTemps = g.remapRoomTempsChanged(rm, grid, n)
	}
	g.rebuildFilth(grid, n) // filth.go: tread and room filth sums
	g.roomGrid, g.rooms, g.roomDirty = grid, stats, false
	ri.out = grid
	if full {
		g.afterRoomsRebuilt() // roof.go
	} else {
		g.refreshRoofSupportAround(bchanged)
		g.autoRoofRooms(rm)
	}
}

// countRoomBuildings adds what the buildings bring to each room (by the
// room of a building's first footprint cell).
func (g *Game) countRoomBuildings(grid []int, stats []roomStats) {
	w := g.grid.Width()
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		r := grid[cells[0][1]*w+cells[0][0]]
		if r < 0 {
			continue
		}
		c := &stats[r].contents
		switch {
		case b.Kind == construction.Floor:
			c.Floored++
		case b.Kind.Sleeps():
			c.Beds++
		case b.Kind == construction.Table:
			c.Tables++
		case b.Kind == construction.Chair:
			c.Chairs++
		case b.Kind == construction.Lamp:
			c.Lamps++
		case construction.DefOf(b.Kind).Bench:
			c.Benches++
		}
	}
}

// checkRoomsAgainstDetect, under the test self-check, proves an
// incremental rebuild against the full one.
func (g *Game) checkRoomsAgainstDetect(grid []int, stats []roomStats) {
	w, h := g.grid.Width(), g.grid.Height()
	want, sizes := room.Detect(w, h, g.isBoundary)
	if !slices.Equal(grid, want) {
		panic("rooms: incremental room grid differs from room.Detect")
	}
	ws := g.roomStatsFull(want, sizes)
	if len(ws) != len(stats) {
		panic("rooms: room count differs")
	}
	for r := range ws {
		if ws[r] != stats[r] {
			panic(fmt.Sprintf("rooms: room %d stats %+v, full rebuild %+v", r, stats[r], ws[r]))
		}
	}
}

// remapRoomTempsChanged is remapRoomTemps for an incremental rebuild: a
// room with exactly an old room's cells keeps that room's air (the average
// of identical values); a changed room averages over its cells as
// remapRoomTemps does; cells that left a room for the outdoors keep their
// air as residual. Cells of unchanged rooms hold no residual (it is only
// ever kept for outdoor cells and dropped when a cell joins a room).
func (g *Game) remapRoomTempsChanged(rm *region.Rooms, newGrid []int, n int) []temperature.Temp {
	now := g.clock.Now()
	old := g.roomGrid
	out := make([]temperature.Temp, n)
	outside := int(g.climate.Outdoor(now))
	if g.residual == nil {
		g.residual = map[int]temperature.Temp{}
	}
	rm.EachChanged(func(i int) {
		if o := old[i]; newGrid[i] == room.Outdoors && o >= 0 && o < len(g.roomTemps) {
			g.residual[i] = g.roomTemps[o] // a breached room keeps its air for a while
		}
	})
	for r := 0; r < n; r++ {
		if s := rm.Same(r); s >= 0 && s < len(g.roomTemps) {
			out[r] = g.roomTemps[s]
			continue
		}
		sum, cnt := 0, 0
		rm.EachCell(r, func(i int) {
			t := outside
			switch o := old[i]; {
			case o >= 0 && o < len(g.roomTemps):
				t = int(g.roomTemps[o])
			case g.residual[i] != 0:
				t = int(g.residual[i])
			}
			delete(g.residual, i)
			sum += t
			cnt++
		})
		if cnt > 0 {
			out[r] = temperature.Temp(sum / cnt)
		}
	}
	return out
}

// refreshRoofSupportAround is refreshRoofSupport for the cells within
// roof.SupportRange of a boundary that changed: only their support can
// have changed. (A built roof left unsupported elsewhere was flagged when
// it lost its support and stays flagged until checkRoofSupport drops it.)
func (g *Game) refreshRoofSupportAround(bchanged []int32) {
	g.ensureRoofGrid()
	g.roofx.countsOK = false // room ids changed
	w, h := g.grid.Width(), g.grid.Height()
	if len(bchanged) > w*h/256 {
		g.refreshRoofSupport()
		return
	}
	ri := &g.regs.rooms
	if len(ri.stamp) != w*h {
		ri.stamp, ri.gen = make([]uint32, w*h), 0
	}
	if ri.gen++; ri.gen == 0 {
		clear(ri.stamp)
		ri.gen = 1
	}
	grid := g.roomGrid
	const r = roof.SupportRange
	supported := func(x, y int) bool {
		for yy := max(0, y-r); yy <= min(h-1, y+r); yy++ {
			row := grid[yy*w : yy*w+w]
			for xx := max(0, x-r); xx <= min(w-1, x+r); xx++ {
				if row[xx] == room.Wall {
					return true
				}
			}
		}
		return false
	}
	for _, c := range bchanged {
		cx, cy := int(c)%w, int(c)/w
		for y := max(0, cy-r); y <= min(h-1, cy+r); y++ {
			for x := max(0, cx-r); x <= min(w-1, cx+r); x++ {
				i := y*w + x
				if ri.stamp[i] == ri.gen {
					continue
				}
				ri.stamp[i] = ri.gen
				cell := g.roofs[i]
				n := cell &^ roof.Supported
				if supported(x, y) {
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
	}
}

// autoRoofRooms is autoRoof over the cells of the small rooms only (the
// same cells autoRoof's full scan acts on).
func (g *Game) autoRoofRooms(rm *region.Rooms) {
	for r := range g.rooms {
		if g.rooms[r].contents.Cells > roof.AutoMaxCells {
			continue
		}
		rm.EachCell(r, func(i int) {
			c := g.roofs[i]
			if c.Roofed() || !c.Has(roof.Supported) || c&(roofPlans|roof.NoAuto) != 0 {
				return
			}
			g.setRoof(i, c|roof.PlanBuild)
		})
	}
}
