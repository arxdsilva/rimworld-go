package game

import (
	"math/bits"
	"slices"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/region"
)

// Walkability regions (NG-1 §6.4, E2). The region map mirrors the grid's
// walkability and the special cells movement layers treat differently —
// doors, fences, gates, spike traps — taken from the very sets the path
// queries read (buildingCells, fences), so a region answer always matches
// what A* would find. It is derived state: rebuilt from the grid after a
// load, never saved. Every reachability question is a label comparison
// instead of a search; pathing.Find gives up at once on a target in
// another component instead of flooding the start's.

// regionState is the region map and what it was built from.
type regionState struct {
	off       bool // regions unused: plain searches (tests compare runs: bit-identical)
	exactLong bool // long paths searched exactly and unbudgeted (tests: E2 alone is bit-identical)
	m         *region.Map
	grid      *grid.Grid // the grid mirrored (a load replaces it)
	gridVer   uint64
	flagsOK   bool
	structVer uint64
	fenceGen  uint64
	doors     tileSet // the door and trap sets the flags came from
	traps     tileSet
	special   []specialCell // flagged cells, ascending
	bits      [region.Layers][]uint64
	bitsOK    [region.Layers]bool
	hive      hiveExitCache // hive.go
	rooms     roomIndex     // rooms_index.go
	budget    searchBudget  // paths.go
}

// hiveExitCache is hiveExit's last answer and the structure it was for.
type hiveExitCache struct {
	ok                 bool
	grid               *grid.Grid
	gridVer, structVer uint64
	hx, hy, x, y       int
	found              bool
}

type specialCell struct {
	i int32
	f region.Flags
}

// regionsNow is the region map brought up to date with the grid, the
// buildings and the fences.
func (g *Game) regionsNow() *region.Map {
	rs := &g.regs
	w, h := g.grid.Width(), g.grid.Height()
	if rs.m == nil || rs.grid != g.grid || rs.m.Width() != w || rs.m.Height() != h {
		rs.m, rs.grid, rs.flagsOK, rs.special = region.New(w, h), g.grid, false, nil
		rs.bitsOK = [region.Layers]bool{}
		rs.m.SetBlockedWords(g.grid.BlockedBits())
		rs.gridVer = g.grid.Version()
	}
	if v := g.grid.Version(); v != rs.gridVer {
		rs.m.SetBlockedWords(g.grid.BlockedBits())
		rs.gridVer = v
		rs.bitsOK = [region.Layers]bool{}
	}
	f := g.fences()
	if !rs.flagsOK || rs.structVer != g.structVer || rs.fenceGen != f.gen {
		g.syncRegionFlags(f)
	}
	return rs.m
}

// syncRegionFlags recomputes the flagged cells and feeds the changes to
// the map.
func (g *Game) syncRegionFlags(f *fenceIndex) {
	rs := &g.regs
	w, n := g.grid.Width(), g.grid.Width()*g.grid.Height()
	doors := g.buildingCells(construction.Door)
	traps := g.buildingCells(construction.SpikeTrap)
	var cells []int32
	for _, s := range []tileSet{doors, traps} {
		for wi, word := range s {
			for word != 0 {
				if i := wi<<6 + bits.TrailingZeros64(word); i < n {
					cells = append(cells, int32(i))
				}
				word &= word - 1
			}
		}
	}
	if f.n > 0 {
		for _, id := range g.bldgIDs {
			b, ok := g.bldgs.Get(id)
			if !ok || !construction.DefOf(b.Kind).AnimalBlock {
				continue
			}
			fp, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
			for _, c := range fp {
				if g.grid.InBounds(c[0], c[1]) {
					cells = append(cells, int32(c[1]*w+c[0]))
				}
			}
		}
	}
	slices.Sort(cells)
	cells = slices.Compact(cells)
	next := make([]specialCell, 0, len(cells))
	for _, i := range cells {
		var fl region.Flags
		if doors.has(int(i)) {
			fl |= region.Door
		}
		if traps.has(int(i)) {
			fl |= region.Trap
		}
		if f.n > 0 {
			switch f.cells[i] {
			case fencePost:
				fl |= region.Fence
			case fenceGate:
				fl |= region.Gate
			}
		}
		if fl != 0 {
			next = append(next, specialCell{i, fl})
		}
	}
	// diff against the previous flags (both ascending)
	old := rs.special
	a, b := 0, 0
	for a < len(old) || b < len(next) {
		switch {
		case b == len(next) || a < len(old) && old[a].i < next[b].i:
			rs.m.SetFlags(int(old[a].i)%w, int(old[a].i)/w, 0)
			a++
		case a == len(old) || next[b].i < old[a].i:
			rs.m.SetFlags(int(next[b].i)%w, int(next[b].i)/w, next[b].f)
			b++
		default:
			rs.m.SetFlags(int(next[b].i)%w, int(next[b].i)/w, next[b].f)
			a++
			b++
		}
	}
	rs.special, rs.doors, rs.traps = next, doors, traps
	rs.flagsOK, rs.structVer, rs.fenceGen = true, g.structVer, f.gen
	rs.bitsOK = [region.Layers]bool{}
}

// layerBits is layer l's blocked-cell bitset (pathing.Bitset): the grid's,
// plus the flagged cells l may not enter.
func (g *Game) layerBits(l region.Layer) []uint64 {
	g.regionsNow()
	if l == region.Colonist {
		return g.grid.BlockedBits()
	}
	rs := &g.regs
	if !rs.bitsOK[l] {
		rs.bits[l] = append(rs.bits[l][:0], g.grid.BlockedBits()...)
		bl := l.Blocks()
		for _, s := range rs.special {
			if s.f&bl != 0 {
				rs.bits[l][s.i>>6] |= 1 << (uint(s.i) & 63)
			}
		}
		rs.bitsOK[l] = true
	}
	return rs.bits[l]
}

// sameSet reports that two cached tile sets are the same one (not just
// equal): a query built before a building change holds an older set.
func sameSet(a, b tileSet) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// moverLayer is the layer pathTo plans id with (pathQuery's choice).
func (g *Game) moverLayer(id entity.EntityID) region.Layer {
	switch {
	case g.sapper(id):
		return region.Sapper
	case g.isRaider(id):
		return region.Hostile
	}
	return region.Colonist
}

// reachRegions answers reachable(id, x, y) from the region labels, with
// pathTo's rules: the mover's own cell is reached; a walkable target must
// share a component with the start; an unwalkable one is reached from
// beside it (already adjacent, or any walkable 8-neighbour reachable).
func (g *Game) reachRegions(id entity.EntityID, x, y int) bool {
	pos, _ := g.positions.Get(id)
	if pos.X == x && pos.Y == y {
		return true
	}
	l := g.moverLayer(id)
	m := g.regionsNow()
	if m.Label(l, x, y) >= 0 {
		return m.Reach(l, pos.X, pos.Y, x, y)
	}
	if pathing.Adjacent(pos.X, pos.Y, x, y) {
		return true
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx != 0 || dy != 0) && m.Label(l, x+dx, y+dy) >= 0 && m.Reach(l, pos.X, pos.Y, x+dx, y+dy) {
				return true
			}
		}
	}
	return false
}

// --- pathing.Reacher / pathing.Bitset on the game's path queries ---------

// plainQuery hides a query's optional extensions (self-check searches).
type plainQuery struct{ pathing.Query }

// checkReach, under the test self-check, proves a "no path" region answer
// with a full search.
func checkReach(q pathing.Query, sx, sy, tx, ty int, got bool) bool {
	if !got && selfCheck() {
		if _, ok := pathing.Find(plainQuery{q}, sx, sy, tx, ty); ok {
			panic("regions: Reach said no, A* found a path")
		}
	}
	return got
}

// colonyQuery is the grid as the colony's own creatures walk it, with the
// region extensions (tame animals with no fences on the map).
type colonyQuery struct{ g *Game }

func (q colonyQuery) Width() int             { return q.g.grid.Width() }
func (q colonyQuery) Height() int            { return q.g.grid.Height() }
func (q colonyQuery) Walkable(x, y int) bool { return q.g.grid.Walkable(x, y) }
func (q colonyQuery) BlockedBits() []uint64  { return q.g.grid.BlockedBits() }
func (q colonyQuery) Reach(sx, sy, tx, ty int) bool {
	if q.g.noRegions() {
		return true
	}
	return checkReach(q, sx, sy, tx, ty, q.g.regionsNow().Reach(region.Colonist, sx, sy, tx, ty))
}

// hostileReady reports that h's door set is the current one the regions
// were built from.
func (h hostileMap) regionsReady() bool {
	if h.g.noRegions() {
		return false
	}
	h.g.regionsNow()
	return sameSet(h.doors, h.g.regs.doors)
}

func (h hostileMap) Reach(sx, sy, tx, ty int) bool {
	if !h.regionsReady() {
		return true
	}
	return checkReach(h, sx, sy, tx, ty, h.g.regs.m.Reach(region.Hostile, sx, sy, tx, ty))
}

func (h hostileMap) BlockedBits() []uint64 {
	if !h.regionsReady() {
		return nil
	}
	return h.g.layerBits(region.Hostile)
}

// sapperMap embeds hostileMap but also shuts traps: it must not inherit
// hostileMap's extensions.
func (s sapperMap) regionsReady() bool {
	return s.hostileMap.regionsReady() && sameSet(s.traps, s.g.regs.traps)
}

func (s sapperMap) Reach(sx, sy, tx, ty int) bool {
	if !s.regionsReady() {
		return true
	}
	return checkReach(s, sx, sy, tx, ty, s.g.regs.m.Reach(region.Sapper, sx, sy, tx, ty))
}

func (s sapperMap) BlockedBits() []uint64 {
	if !s.regionsReady() {
		return nil
	}
	return s.g.layerBits(region.Sapper)
}

// regionLayer is the layer a fence query matches: tame on the colony's
// grid, wild on the hostile map; ok is false for any other base or once
// the fences it copied were rebuilt.
func (q fenceQuery) regionLayer() (region.Layer, bool) {
	if q.g == nil || q.g.noRegions() || !q.g.fence.built || q.g.fence.gen != q.gen {
		return 0, false
	}
	switch b := q.Query.(type) {
	case colonyQuery:
		if q.gateOK {
			return region.Tame, true
		}
	case *grid.Grid:
		if q.gateOK && b == q.g.grid {
			return region.Tame, true
		}
	case hostileMap:
		if !q.gateOK && b.regionsReady() {
			return region.Wild, true
		}
	}
	return 0, false
}

func (q fenceQuery) Reach(sx, sy, tx, ty int) bool {
	l, ok := q.regionLayer()
	if !ok {
		return true
	}
	return checkReach(q, sx, sy, tx, ty, q.g.regionsNow().Reach(l, sx, sy, tx, ty))
}

func (q fenceQuery) BlockedBits() []uint64 {
	l, ok := q.regionLayer()
	if !ok {
		return nil
	}
	return q.g.layerBits(l)
}

// crowdCost forwards its base query's extensions (its own ExtraCost only
// adds cost; walkability is the base's).
func (c crowdCost) Reach(sx, sy, tx, ty int) bool {
	switch q := c.Query.(type) {
	case pathing.Reacher:
		return q.Reach(sx, sy, tx, ty)
	case *grid.Grid:
		if q == c.g.grid && !c.g.noRegions() {
			return checkReach(q, sx, sy, tx, ty, c.g.regionsNow().Reach(region.Colonist, sx, sy, tx, ty))
		}
	}
	return true
}

func (c crowdCost) BlockedBits() []uint64 {
	if b, ok := c.Query.(pathing.Bitset); ok {
		return b.BlockedBits()
	}
	return nil
}

// regionsOffAll switches the region answers off in every game (tests:
// the run must match one without them).
var regionsOffAll bool

// noRegions reports that g answers with plain searches and full room
// rebuilds.
func (g *Game) noRegions() bool { return g.regs.off || regionsOffAll }
