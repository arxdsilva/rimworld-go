package game

import (
	"maps"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/stain"
	"rimworld-go/internal/domain/world/terrain"
)

// Tracked-in dirt (SIM-021): pawns pick dirt up on outdoor soil and field
// work, leave a trail of it indoors, and wipe it off on entrance carpets.
// Indoor dirt never fades; colonists clean it inside the Home area
// (clean.go). Rooms lose impressiveness to it (rooms.go).
//
// Invariants (checked by TestRoomFilthIncrementalMatchesRebuild):
//   - nz is exactly the sorted indexes of tiles with dirt;
//   - while rooms are not dirty, roomSum[r] is the dirt plus stain of
//     room r's cells (ensureRooms recomputes it on every rebuild, and the
//     incremental deltas are skipped while a rebuild is pending);
//   - cleanQ is sorted and unique, holds every Home tile whose dirt plus
//     stain is at least filth.CleanStart, and never a clean tile. A tile
//     leaves only at 0 (or once it is no longer Home), so a half-swept
//     tile is finished later (hysteresis).
//
// Every dirt change goes through addDirt and every stain change through
// stainMoved (stains.go); both end in filthMoved.

// DirtView is the dirt on one tile.
type DirtView struct {
	X, Y   int
	Amount float64 // 0..1 of filth.Full
}

// dirtCell is one dirty tile in a save.
type dirtCell struct {
	X, Y   int
	Amount uint16
}

// filthState is the dirt layer and its indexes.
type filthState struct {
	cells   []uint16                   // dirt per tile, row-major
	nz      []int32                    // sorted indexes of dirty tiles
	carried map[entity.EntityID]uint16 // dirt on each pawn's boots
	roomSum []int                      // dirt plus stain per room id
	tread   []uint8                    // per tile: 0x80 floored, low 7 bits carpet catch percent
	cleanQ  []int32                    // sorted Home tiles queued for cleaning
}

// tread bits.
const (
	treadFloor = 0x80
	treadCatch = 0x7f // carpet catch, percent
)

// insertSorted adds v to the sorted set s.
func insertSorted(s []int32, v int32) []int32 {
	k, found := slices.BinarySearch(s, v)
	if found {
		return s
	}
	return slices.Insert(s, k, v)
}

// removeSorted drops v from the sorted set s.
func removeSorted(s []int32, v int32) []int32 {
	k, found := slices.BinarySearch(s, v)
	if !found {
		return s
	}
	return slices.Delete(s, k, k+1)
}

// dirtAtIdx is the dirt on tile i.
func (g *Game) dirtAtIdx(i int) int {
	if i < 0 || i >= len(g.filth.cells) {
		return 0
	}
	return int(g.filth.cells[i])
}

// stainAtIdx is the stain amount on tile i.
func (g *Game) stainAtIdx(i int) int {
	if len(g.stains.tiles) == 0 {
		return 0
	}
	w := g.grid.Width()
	return int(g.stains.tiles[[2]int{i % w, i / w}].Amount)
}

// filthTotal is the dirt plus stain on tile i: what cleaning removes.
func (g *Game) filthTotal(i int) int { return g.dirtAtIdx(i) + g.stainAtIdx(i) }

// addDirt changes the dirt on tile i by delta (clamped to 0..filth.Full).
// It is the only write to the dirt layer.
func (g *Game) addDirt(i, delta int) {
	f := &g.filth
	n := g.grid.Width() * g.grid.Height()
	if delta == 0 || i < 0 || i >= n {
		return
	}
	if len(f.cells) != n {
		if delta < 0 {
			return
		}
		f.cells = make([]uint16, n)
	}
	old := int(f.cells[i])
	nv := max(0, min(filth.Full, old+delta))
	if nv == old {
		return
	}
	f.cells[i] = uint16(nv)
	switch {
	case old == 0:
		f.nz = insertSorted(f.nz, int32(i))
	case nv == 0:
		f.nz = removeSorted(f.nz, int32(i))
	}
	st := g.stainAtIdx(i)
	g.filthMoved(i, old+st, nv+st)
}

// filthMoved keeps roomSum and cleanQ in step with tile i's total going
// from old to nv. While a room rebuild is pending, roomSum is left alone:
// the rebuild recounts it.
func (g *Game) filthMoved(i, old, nv int) {
	f := &g.filth
	if !g.roomDirty && i < len(g.roomGrid) {
		if r := g.roomGrid[i]; r >= 0 && r < len(f.roomSum) {
			f.roomSum[r] += nv - old
		}
	}
	switch {
	case nv == 0:
		f.cleanQ = removeSorted(f.cleanQ, int32(i))
	case nv >= filth.CleanStart && g.inHome(i):
		f.cleanQ = insertSorted(f.cleanQ, int32(i))
	}
}

// setCarried sets the dirt on id's boots.
func (g *Game) setCarried(id entity.EntityID, c int) {
	f := &g.filth
	if c <= 0 {
		if len(f.carried) > 0 {
			delete(f.carried, id)
		}
		return
	}
	if f.carried == nil {
		f.carried = map[entity.EntityID]uint16{}
	}
	f.carried[id] = uint16(min(c, filth.Full))
}

// wetPm is the ground wetness, permille.
func (g *Game) wetPm() int { return int(g.weather.Wetness * 1000) }

// treadAt is the tread byte of tile i (rooms must be current).
func (g *Game) treadAt(i int) uint8 {
	if i < len(g.filth.tread) {
		return g.filth.tread[i]
	}
	return 0
}

// dirtStep moves dirt between a colonist's or raider's boots and the tile
// it stepped onto (called from stepPawn): a carpet wipes the boots, an
// indoor tile takes a share, bare outdoor soil adds some and clean outdoor
// ground (stone, floors) knocks a little off.
func (g *Game) dirtStep(id entity.EntityID, to Position) {
	if !g.grid.InBounds(to.X, to.Y) {
		return
	}
	g.ensureRooms() // tread is rebuilt with the rooms
	i := to.Y*g.grid.Width() + to.X
	c := int(g.filth.carried[id])
	t := g.treadAt(i)
	nc := c
	switch {
	case g.terr.At(to.X, to.Y) == terrain.ShallowWater:
		nc = wadeWash(c) // water.go: wading rinses the boots
		g.remember(id, mood.Soaked)
	case t&treadCatch != 0:
		if c == 0 {
			return
		}
		keep, soil := filth.Wipe(c, int(t&treadCatch)*10)
		nc = keep
		g.addDirt(i, soil)
	case !g.underSky(to.X, to.Y):
		keep, drop := filth.Deposit(c)
		nc = keep
		g.addDirt(i, drop)
	case t&treadFloor == 0 && g.terr.At(to.X, to.Y).Fertility() > 0:
		nc = filth.Carry(c, g.stepGain(to.X, to.Y, filth.StepGain, g.hasGrass(to.X, to.Y)))
	default:
		nc = filth.Shed(c)
	}
	if nc != c {
		g.setCarried(id, nc)
	}
}

// dirtWork soils the boots of a pawn working the open ground (sowing,
// harvesting, cutting, mining, digging a grave) every filth.WorkEvery
// ticks, staggered by id (called from phaseJobs while the job is in Do).
func (g *Game) dirtWork(id entity.EntityID, j job.Record, now uint64) {
	if (now+uint64(id))%filth.WorkEvery != 0 {
		return
	}
	switch j.Kind {
	case job.Sow, job.Harvest, job.Cut, job.Mine, job.Smooth:
	case job.Construct:
		if fr, ok := g.frames.Get(j.Target); !ok || fr.Kind != construction.Grave {
			return
		}
	default:
		return
	}
	pos, ok := g.positions.Get(id)
	if !ok || !g.grid.InBounds(pos.X, pos.Y) {
		return
	}
	g.ensureRooms()
	if g.treadAt(pos.Y*g.grid.Width()+pos.X)&treadFloor != 0 || !g.underSky(pos.X, pos.Y) ||
		g.terr.At(pos.X, pos.Y).Fertility() <= 0 {
		return
	}
	c := int(g.filth.carried[id])
	g.setCarried(id, filth.Carry(c, g.stepGain(pos.X, pos.Y, filth.WorkGain, false)))
}

// washDirt rinses dirt off open ground while rain or snow falls (called
// with stainDecay every climate interval). Indoor dirt never fades.
func (g *Game) washDirt() {
	if !g.weather.Kind.Falling() || len(g.filth.nz) == 0 || g.weather.Intensity <= 0 {
		return
	}
	wash := stain.RainWash
	if g.weather.Kind.Frozen() {
		wash = stain.SnowWash
	}
	dec := max(1, int(float64(filth.Full*climateInterval)*g.weather.Intensity/float64(wash)))
	w := g.grid.Width()
	// backwards: addDirt may drop the current entry, never an earlier one
	for k := len(g.filth.nz) - 1; k >= 0; k-- {
		i := int(g.filth.nz[k])
		if g.underSky(i%w, i/w) {
			g.addDirt(i, -dec)
		}
	}
}

// pruneCarried forgets the boots of pawns that are gone or dead.
func (g *Game) pruneCarried() {
	for id := range g.filth.carried {
		if v, ok := g.vital.Get(id); ok && v.Status != health.Dead {
			continue
		}
		delete(g.filth.carried, id)
	}
}

// rebuildFilth recomputes tread (floors and carpets) and roomSum for a
// new room grid (called from ensureRooms).
func (g *Game) rebuildFilth(grid []int, rooms int) {
	f := &g.filth
	w, n := g.grid.Width(), g.grid.Width()*g.grid.Height()
	if len(f.tread) != n {
		f.tread = make([]uint8, n)
	} else {
		clear(f.tread)
	}
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		d := construction.DefOf(b.Kind)
		if d.Layer != construction.LayerFloor && !d.Carpet {
			continue
		}
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			if !g.grid.InBounds(c[0], c[1]) {
				continue
			}
			i := c[1]*w + c[0]
			if d.Carpet {
				f.tread[i] = f.tread[i]&treadFloor | uint8(filth.Catch(b.Stuff)/10)&treadCatch
			} else {
				f.tread[i] |= treadFloor
			}
		}
	}
	if cap(f.roomSum) >= rooms {
		f.roomSum = f.roomSum[:rooms]
		clear(f.roomSum)
	} else {
		f.roomSum = make([]int, rooms)
	}
	if len(grid) != n {
		return
	}
	for _, i := range f.nz {
		if r := grid[i]; r >= 0 && r < rooms {
			f.roomSum[r] += int(f.cells[i])
		}
	}
	for k, s := range g.stains.tiles {
		if !g.grid.InBounds(k[0], k[1]) {
			continue
		}
		if r := grid[k[1]*w+k[0]]; r >= 0 && r < rooms {
			f.roomSum[r] += int(s.Amount)
		}
	}
}

// refreshFilth rebuilds the derived indexes after a layer was replaced
// wholesale (a load): cleanQ from the threshold, roomSum if rooms are
// current.
func (g *Game) refreshFilth() {
	f := &g.filth
	f.cleanQ = f.cleanQ[:0]
	for i, v := range g.homeArea.cells {
		if v != 0 && g.filthTotal(i) >= filth.CleanStart {
			f.cleanQ = append(f.cleanQ, int32(i))
		}
	}
	if !g.roomDirty && g.roomGrid != nil {
		g.rebuildFilth(g.roomGrid, len(g.rooms))
	}
}

// carriedPct is the dirt on id's boots, percent of filth.Full.
func (g *Game) carriedPct(id entity.EntityID) int {
	return int(g.filth.carried[id]) * 100 / filth.Full
}

// fillDirtView lists the dirty tiles, sorted y then x.
func (g *Game) fillDirtView(w *WorldView) {
	if len(g.filth.nz) == 0 {
		return
	}
	gw := g.grid.Width()
	w.Dirt = make([]DirtView, 0, len(g.filth.nz))
	for _, i := range g.filth.nz {
		w.Dirt = append(w.Dirt, DirtView{X: int(i) % gw, Y: int(i) / gw,
			Amount: float64(g.filth.cells[i]) / filth.Full})
	}
}

// exportDirt and importDirt save and restore the dirt layer.
func (g *Game) exportDirt() ([]dirtCell, map[entity.EntityID]uint16) {
	var cells []dirtCell
	gw := g.grid.Width()
	for _, i := range g.filth.nz {
		cells = append(cells, dirtCell{X: int(i) % gw, Y: int(i) / gw, Amount: g.filth.cells[i]})
	}
	var car map[entity.EntityID]uint16
	if len(g.filth.carried) > 0 {
		car = maps.Clone(g.filth.carried)
	}
	return cells, car
}

func (g *Game) importDirt(cells []dirtCell, carried map[entity.EntityID]uint16) {
	g.filth = filthState{}
	gw := g.grid.Width()
	for _, c := range cells {
		if c.Amount == 0 || !g.grid.InBounds(c.X, c.Y) {
			continue
		}
		if g.filth.cells == nil {
			g.filth.cells = make([]uint16, gw*g.grid.Height())
		}
		i := c.Y*gw + c.X
		if g.filth.cells[i] == 0 {
			g.filth.nz = append(g.filth.nz, int32(i))
		}
		g.filth.cells[i] = min(c.Amount, filth.Full)
	}
	slices.Sort(g.filth.nz)
	for id, v := range carried {
		g.setCarried(id, int(v))
	}
	g.refreshFilth()
}

// importCleanQ restores the half-swept tiles of a save (still Home and
// not clean), on top of the tiles over the threshold.
func (g *Game) importCleanQ(q []int32) {
	n := int32(g.grid.Width() * g.grid.Height())
	for _, i := range q {
		if i >= 0 && i < n && g.inHome(int(i)) && g.filthTotal(int(i)) > 0 {
			g.filth.cleanQ = insertSorted(g.filth.cleanQ, i)
		}
	}
}
