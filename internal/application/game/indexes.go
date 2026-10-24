package game

import (
	"bytes"
	"fmt"
	"sync/atomic"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Per-cell indexes (NG-1 §6.3 E1): plantAt, layerBldgAt, layerBlockerAt,
// layerFrameAt and blockingFrameAt answer from a slot per cell (and layer)
// instead of scanning every plant, building or frame.
//
// The indexes are verified, not trusted: a slot holds the id last placed
// on that cell, and a lookup re-checks it against the store (still there,
// same layer, still covering the cell). So removals and moves away need no
// bookkeeping; only arrivals do — spawnPlant, placeFrameStuff, moveBuilding
// (frames), completeFrame and completeMove (buildings) call the note*
// hooks. A cell where two live entries of one layer meet is marked
// conflicted and falls back to the scan, which keeps the scan's
// first-in-list order exactly. An index is (re)built from the id lists on
// first use, and whenever the store behind it was replaced (Load).
//
// Tests switch on indexSelfCheck (scale_golden_test.go), which checks
// every indexed answer against the original scan and panics on a
// difference, across the whole game test suite.
var indexSelfCheck bool

// selfCheckEvery: check one indexed answer in this many (tests under the
// race detector sample, which keeps the suite's run time in bounds).
var selfCheckEvery uint64 = 1

var selfCheckN atomic.Uint64

// selfCheck reports whether this lookup is checked against the scan.
func selfCheck() bool {
	return indexSelfCheck && (selfCheckEvery <= 1 || selfCheckN.Add(1)%selfCheckEvery == 0)
}

// cellSlots is one id per cell, for a fixed number of layers.
type cellSlots struct {
	w, h     int
	slots    [][]entity.EntityID // [layer][cell]
	conflict map[int]bool        // cells answered by scan
	offMap   bool                // some footprint reached off the map: off-map queries scan
	src      any                 // the store the index was built from
	built    bool
}

func (c *cellSlots) reset(w, h, layers int, src any) {
	c.w, c.h, c.src, c.built, c.offMap = w, h, src, true, false
	c.conflict = nil
	if len(c.slots) != layers {
		c.slots = make([][]entity.EntityID, layers)
	}
	for l := range c.slots {
		if len(c.slots[l]) != w*h {
			c.slots[l] = make([]entity.EntityID, w*h)
		} else {
			clear(c.slots[l])
		}
	}
}

func (c *cellSlots) stale(w, h int, src any) bool {
	return !c.built || c.w != w || c.h != h || c.src != src
}

// cell is the slot index of (x, y), or -1 off the map.
func (c *cellSlots) cell(x, y int) int {
	if uint(x) >= uint(c.w) || uint(y) >= uint(c.h) {
		return -1
	}
	return y*c.w + x
}

// put records id on (x, y) in layer l; live reports whether an id
// already there still covers the cell (then the cell is conflicted).
func (c *cellSlots) put(l, x, y int, id entity.EntityID, live func(entity.EntityID) bool) {
	i := c.cell(x, y)
	if i < 0 {
		c.offMap = true
		return
	}
	if old := c.slots[l][i]; old != 0 && old != id && live(old) {
		if c.conflict == nil {
			c.conflict = map[int]bool{}
		}
		c.conflict[i] = true
		return
	}
	c.slots[l][i] = id
}

// coversCell reports whether a k building or frame facing f at (x, y)
// covers (px, py), without allocating for single-tile kinds.
func coversCell(k construction.Kind, f construction.Facing, x, y, px, py int) bool {
	if s := construction.DefOf(k).Size; s[0] <= 1 && s[1] <= 1 {
		return x == px && y == py
	}
	return construction.Covers(k, f, x, y, px, py)
}

func layerOf(k construction.Kind) construction.Layer { return construction.DefOf(k).Layer }

const numLayers = int(construction.LayerFloor) + 1

// ---- plants ----

func (g *Game) plantIndex() *cellSlots {
	ix := &g.idx.plants
	if ix.stale(g.grid.Width(), g.grid.Height(), g.plants) {
		ix.reset(g.grid.Width(), g.grid.Height(), 1, g.plants)
		for _, id := range g.plantIDs {
			if p, ok := g.plants.Get(id); ok {
				ix.put(0, p.X, p.Y, id, g.plantLive(p.X, p.Y))
			}
		}
	}
	return ix
}

func (g *Game) plantLive(x, y int) func(entity.EntityID) bool {
	return func(id entity.EntityID) bool {
		p, ok := g.plants.Get(id)
		return ok && p.X == x && p.Y == y
	}
}

// notePlant indexes a plant just added to plantIDs.
func (g *Game) notePlant(id entity.EntityID, p plant.Plant) {
	ix := &g.idx.plants
	if ix.stale(g.grid.Width(), g.grid.Height(), g.plants) {
		return // built from the lists on first use
	}
	ix.put(0, p.X, p.Y, id, g.plantLive(p.X, p.Y))
}

func (g *Game) plantAt(x, y int) entity.EntityID {
	id := g.plantAtIndexed(x, y)
	if selfCheck() {
		if s := g.plantAtScan(x, y); s != id {
			panic(fmt.Sprintf("plantAt(%d,%d) index %d, scan %d", x, y, id, s))
		}
	}
	return id
}

func (g *Game) plantAtIndexed(x, y int) entity.EntityID {
	ix := g.plantIndex()
	i := ix.cell(x, y)
	if i < 0 || ix.conflict[i] {
		if i < 0 && !ix.offMap {
			return 0
		}
		return g.plantAtScan(x, y)
	}
	if id := ix.slots[0][i]; id != 0 {
		if p := g.plants.Ref(id); p != nil && p.X == x && p.Y == y {
			return id
		}
	}
	return 0
}

func (g *Game) plantAtScan(x, y int) entity.EntityID {
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if ok && p.X == x && p.Y == y {
			return id
		}
	}
	return 0
}

// ---- buildings and frames ----

// placed is what the building and frame indexes need of either.
type placed struct {
	kind   construction.Kind
	facing construction.Facing
	x, y   int
}

// layered is a per-layer cell index over the building store (b) or the
// frame store (f).
type layered struct {
	ix *cellSlots
	b  *component.Store[construction.Building]
	f  *component.Store[construction.Frame]
}

func (lx layered) get(id entity.EntityID) (placed, bool) {
	if lx.b != nil {
		b, ok := lx.b.Get(id)
		return placed{b.Kind, b.Facing, b.X, b.Y}, ok
	}
	f, ok := lx.f.Get(id)
	return placed{f.Kind, f.Facing, f.X, f.Y}, ok
}

// live: id still exists, in layer l, covering (x, y).
func (lx layered) live(id entity.EntityID, l construction.Layer, x, y int) bool {
	p, ok := lx.get(id)
	return ok && layerOf(p.kind) == l && coversCell(p.kind, p.facing, p.x, p.y, x, y)
}

func (lx layered) note(id entity.EntityID) {
	p, ok := lx.get(id)
	if !ok {
		return
	}
	l := layerOf(p.kind)
	cells, _, _ := construction.Footprint(p.kind, p.facing, p.x, p.y)
	for _, c := range cells {
		lx.ix.put(int(l), c[0], c[1], id, func(o entity.EntityID) bool { return lx.live(o, l, c[0], c[1]) })
	}
}

// at is the indexed id in layer l on (x, y); scan reports the cell must
// be answered by scanning.
func (lx layered) at(x, y int, l construction.Layer) (id entity.EntityID, scan bool) {
	i := lx.ix.cell(x, y)
	if i < 0 {
		return 0, lx.ix.offMap
	}
	if lx.ix.conflict[i] {
		return 0, true
	}
	if id := lx.ix.slots[l][i]; id != 0 && lx.live(id, l, x, y) {
		return id, false
	}
	return 0, false
}

func (g *Game) bldgIndex() layered {
	lx := layered{ix: &g.idx.bldgs, b: g.bldgs}
	if lx.ix.stale(g.grid.Width(), g.grid.Height(), g.bldgs) {
		lx.ix.reset(g.grid.Width(), g.grid.Height(), numLayers, g.bldgs)
		for _, id := range g.bldgIDs {
			lx.note(id)
		}
	}
	return lx
}

func (g *Game) frameIndex() layered {
	lx := layered{ix: &g.idx.frames, f: g.frames}
	if lx.ix.stale(g.grid.Width(), g.grid.Height(), g.frames) {
		lx.ix.reset(g.grid.Width(), g.grid.Height(), numLayers, g.frames)
		for _, id := range g.frameIDs {
			lx.note(id)
		}
	}
	return lx
}

// noteBuilding indexes building id where it now stands (completed or
// moved).
func (g *Game) noteBuilding(id entity.EntityID) {
	if !g.idx.bldgs.stale(g.grid.Width(), g.grid.Height(), g.bldgs) {
		layered{ix: &g.idx.bldgs, b: g.bldgs}.note(id)
	}
}

// noteFrame indexes a frame just added to frameIDs.
func (g *Game) noteFrame(id entity.EntityID) {
	if !g.idx.frames.stale(g.grid.Width(), g.grid.Height(), g.frames) {
		layered{ix: &g.idx.frames, f: g.frames}.note(id)
	}
}

// layerBlockerAt is a completed movement-blocking building covering (x, y).
func (g *Game) layerBlockerAt(x, y int) entity.EntityID {
	id := g.layerBlockerAtIndexed(x, y)
	if selfCheck() {
		if s := g.layerBlockerAtScan(x, y); s != id {
			panic(fmt.Sprintf("layerBlockerAt(%d,%d) index %d, scan %d", x, y, id, s))
		}
	}
	return id
}

func (g *Game) layerBlockerAtIndexed(x, y int) entity.EntityID {
	lx := g.bldgIndex()
	var found entity.EntityID
	for l := 0; l < numLayers; l++ {
		id, scan := lx.at(x, y, construction.Layer(l))
		if scan {
			return g.layerBlockerAtScan(x, y)
		}
		if id == 0 {
			continue
		}
		if b, _ := g.bldgs.Get(id); construction.DefOf(b.Kind).BlocksMove {
			if found != 0 {
				return g.layerBlockerAtScan(x, y) // two blockers: the list order decides
			}
			found = id
		}
	}
	return found
}

func (g *Game) layerBlockerAtScan(x, y int) entity.EntityID {
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if ok && construction.DefOf(b.Kind).BlocksMove && construction.Covers(b.Kind, b.Facing, b.X, b.Y, x, y) {
			return id
		}
	}
	return 0
}

func (g *Game) layerBldgAt(x, y int, layer construction.Layer) entity.EntityID {
	id := g.layerBldgAtIndexed(x, y, layer)
	if selfCheck() {
		if s := g.layerBldgAtScan(x, y, layer); s != id {
			panic(fmt.Sprintf("layerBldgAt(%d,%d,%d) index %d, scan %d", x, y, layer, id, s))
		}
	}
	return id
}

func (g *Game) layerBldgAtIndexed(x, y int, layer construction.Layer) entity.EntityID {
	if int(layer) >= numLayers {
		return g.layerBldgAtScan(x, y, layer)
	}
	id, scan := g.bldgIndex().at(x, y, layer)
	if scan {
		return g.layerBldgAtScan(x, y, layer)
	}
	return id
}

func (g *Game) layerBldgAtScan(x, y int, layer construction.Layer) entity.EntityID {
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if ok && construction.DefOf(b.Kind).Layer == layer && construction.Covers(b.Kind, b.Facing, b.X, b.Y, x, y) {
			return id
		}
	}
	return 0
}

func (g *Game) layerFrameAt(x, y int, layer construction.Layer) entity.EntityID {
	id := g.layerFrameAtIndexed(x, y, layer)
	if selfCheck() {
		if s := g.layerFrameAtScan(x, y, layer); s != id {
			panic(fmt.Sprintf("layerFrameAt(%d,%d,%d) index %d, scan %d", x, y, layer, id, s))
		}
	}
	return id
}

func (g *Game) layerFrameAtIndexed(x, y int, layer construction.Layer) entity.EntityID {
	if int(layer) >= numLayers {
		return g.layerFrameAtScan(x, y, layer)
	}
	id, scan := g.frameIndex().at(x, y, layer)
	if scan {
		return g.layerFrameAtScan(x, y, layer)
	}
	return id
}

func (g *Game) layerFrameAtScan(x, y int, layer construction.Layer) entity.EntityID {
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if ok && construction.DefOf(fr.Kind).Layer == layer && construction.Covers(fr.Kind, fr.Facing, fr.X, fr.Y, x, y) {
			return id
		}
	}
	return 0
}

// blockingFrameAt is a planned movement-blocking object covering (x, y)
// (so interaction spots are not planned under future benches/walls).
func (g *Game) blockingFrameAt(x, y int) entity.EntityID {
	id := g.blockingFrameAtIndexed(x, y)
	if selfCheck() {
		if s := g.blockingFrameAtScan(x, y); s != id {
			panic(fmt.Sprintf("blockingFrameAt(%d,%d) index %d, scan %d", x, y, id, s))
		}
	}
	return id
}

func (g *Game) blockingFrameAtIndexed(x, y int) entity.EntityID {
	lx := g.frameIndex()
	var found entity.EntityID
	for l := 0; l < numLayers; l++ {
		id, scan := lx.at(x, y, construction.Layer(l))
		if scan {
			return g.blockingFrameAtScan(x, y)
		}
		if id == 0 {
			continue
		}
		if fr, _ := g.frames.Get(id); construction.DefOf(fr.Kind).BlocksMove {
			if found != 0 {
				return g.blockingFrameAtScan(x, y)
			}
			found = id
		}
	}
	return found
}

func (g *Game) blockingFrameAtScan(x, y int) entity.EntityID {
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if ok && construction.DefOf(fr.Kind).BlocksMove && construction.Covers(fr.Kind, fr.Facing, fr.X, fr.Y, x, y) {
			return id
		}
	}
	return 0
}

// ---- read models ----

// viewCache keeps the last World() and Snapshot() for reuse until the
// state can have changed: the key is the tick and viewGen, which Step and
// ApplyPending bump. It is off by default; SetViewCache turns it on for a
// caller (the renderer) that only changes the game through Step, Enqueue
// and ApplyPending — a dev hook that edits the state directly (BuildNow,
// SpawnStack, ...) is not seen until the next tick.
type viewCache struct {
	on                 bool
	world              WorldView
	worldKey, pawnsKey [2]uint64
	worldOK, pawnsOK   bool
	pawns              []PawnView
	// postBuf is the snapshot's reused postponed-work scratch (postponedTops).
	postBuf []postTop
	// logs memoises logLines per creature (eventlog.go).
	logs map[entity.EntityID]logLinesMemo
	// stains memoises the sorted stains (stains.go fillStainView).
	stains stainMemo
	// plants memoises the plant views (plant_view.go).
	plants plantMemo
}

// SetViewCache turns the read-model cache on or off (see viewCache). The
// cached views are shared between calls: callers must not modify them.
func (g *Game) SetViewCache(on bool) {
	g.views = viewCache{on: on}
}

func (g *Game) viewKey() [2]uint64 { return [2]uint64{uint64(g.clock.Now()), g.viewGen} }

// layerCache is a per-tile read-model layer handed out as a shared,
// read-only slice and rebuilt into a fresh one when its version moves.
type layerCache struct {
	cells []uint8
	ver   uint64
	ok    bool
}

// shareLayer is a per-tile layer for a read model: the slice handed out
// last time while src still holds the same values, else a fresh copy
// (copy-on-write: a slice once handed out is never written again, and an
// unchanged layer keeps its identity, so renderer caches keyed on it
// hold). Comparing is a fraction of the copy and allocates nothing.
func shareLayer[T ~uint8](c *layerCache, src []T) []uint8 {
	if len(src) == 0 {
		return nil
	}
	if raw, ok := any(src).([]uint8); ok { // plain bytes: memcmp and memmove
		if c.ok && bytes.Equal(c.cells, raw) {
			return c.cells
		}
		out := make([]uint8, len(raw))
		copy(out, raw)
		c.cells, c.ok = out, true
		return out
	}
	if c.ok && len(c.cells) == len(src) {
		same := true
		for i, v := range src {
			if c.cells[i] != uint8(v) {
				same = false
				break
			}
		}
		if same {
			return c.cells
		}
	}
	out := make([]uint8, len(src))
	for i, v := range src {
		out[i] = uint8(v)
	}
	c.cells, c.ok = out, true
	return out
}

// terrainView is WorldView.Terrain: terrain codes per tile, rebuilt when
// structVer (bumped by every setTerrain) moves.
func (g *Game) terrainView() []uint8 {
	c := &g.viewTerr
	if c.ok && c.ver == g.structVer && len(c.cells) == len(g.terr.Cells) {
		if selfCheck() {
			for i, t := range g.terr.Cells {
				if c.cells[i] != uint8(t) {
					panic("terrainView: terrain changed without a structVer bump")
				}
			}
		}
		return c.cells
	}
	cells := make([]uint8, len(g.terr.Cells))
	for i, t := range g.terr.Cells {
		cells[i] = uint8(t)
	}
	c.cells, c.ver, c.ok = cells, g.structVer, true
	return cells
}

// ---- lists ----

// dropIDs removes every id matching drop from ids in place, keeping the
// order of the rest (the in-order compaction removePlant, deleteItem and
// the batched rot and frost passes share).
func dropIDs(ids []entity.EntityID, drop func(entity.EntityID) bool) []entity.EntityID {
	kept := ids[:0]
	for _, id := range ids {
		if !drop(id) {
			kept = append(kept, id)
		}
	}
	return kept
}

// mineCache is mineList's sorted designations, per minesVer.
type mineCache struct {
	list [][2]int
	ver  uint64
	ok   bool
}

// seedCache is weatherSeed's terrain hash, per structVer.
type seedCache struct {
	seed, ver uint64
	ok        bool
}

// indexes are the derived lookup structures above (never saved).
type indexes struct {
	plants, bldgs, frames cellSlots
	jobs                  jobIndexes // jobindex.go
}
