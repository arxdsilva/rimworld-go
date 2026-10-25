package game

import (
	"container/heap"
	"encoding/binary"
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/chunk"
	"rimworld-go/internal/domain/world/plant"
)

// Grass (UI-011): ground cover on soil, owned by the simulation so that
// building, sowing and Cut plants remove it. Seeded from terrain by
// plant.GrassAt (the rule the renderer used) on the temperate map, by the
// biome's Grass permille elsewhere (SIM-023). A cut or grazed tile grows
// back after the biome's Regrow days (a min-heap, popped in phasePlants);
// building, flooring and sowing leave it bare.

// GrassCutWork is the labour to cut one grass tile.
const GrassCutWork = 25

// grassState is the per-game grass data. It is seeded lazily (first use,
// World(), or exportGrass) from terrain, so new games, generated maps and old
// saves need no changes to New/NewColony. Tiles under a building, frame or
// plant at seed time start bare.
type grassState struct {
	seeded bool
	cells  []uint8 // 1 = grass, row-major grid.Width() wide
	cut    map[[2]int]bool
	// regrow is the grazed or cut tiles growing back (SIM-023), a min-heap
	// on At.
	regrow []grassRegrow
	// growth (PLAY-010) is each regrowing tile's growth percent, allocated
	// on the first regrowth: 0 means full (or no grass), so the zero state
	// is today's all-full grass. growing lists, per chunk, the tiles with
	// a nonzero growth (exactly those), in the order they regrew;
	// nGrowing counts them.
	growth   []uint8
	growing  [][]int32
	nGrowing int
	// ver changes with any grass or growth change; view is the growth
	// layer last handed out, built at viewVer. fp is the fingerprint of
	// the grass and growth (WorldView.GrassVer), taken at fpVer: a content
	// hash rather than the counter, so a loaded game reads the same
	// version as the game it was saved from.
	ver     uint64
	view    []uint8
	viewVer uint64
	fp      uint64
	fpVer   uint64
	fpOK    bool
}

// grassRegrow is one bare tile due to grow back at tick At (saved).
type grassRegrow struct {
	At   uint64
	Cell int32
}

func (g *Game) grassSt() *grassState { return &g.grass }

// seedGrass builds the grass layer from terrain once. Safe to call early
// (e.g. at the end of New) so seeding never depends on when World() is read.
func (g *Game) seedGrass() *grassState {
	s := g.grassSt()
	if s.seeded {
		return s
	}
	w, h := g.grid.Width(), g.grid.Height()
	s.seeded = true
	s.ver++
	s.cells = make([]uint8, w*h)
	s.cut = map[[2]int]bool{}
	bd := biome.DefOf(g.biome)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := g.terr.At(x, y)
			if bd.Legacy && plant.GrassAt(t, x, y) || !bd.Legacy && plant.GrassAtBiome(t, x, y, bd.Grass) {
				s.cells[y*w+x] = 1
			}
		}
	}
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok {
			g.bareFootprint(s, b.Kind, b.Facing, b.X, b.Y)
		}
	}
	for _, id := range g.frameIDs {
		if f, ok := g.frames.Get(id); ok {
			g.bareFootprint(s, f.Kind, f.Facing, f.X, f.Y)
		}
	}
	for _, id := range g.plantIDs {
		if p, ok := g.plants.Get(id); ok && g.grid.InBounds(p.X, p.Y) {
			s.cells[p.Y*w+p.X] = 0
		}
	}
	return s
}

func (g *Game) bareFootprint(s *grassState, k construction.Kind, f construction.Facing, x, y int) {
	cells, _, _ := construction.Footprint(k, f, x, y)
	for _, c := range cells {
		if g.grid.InBounds(c[0], c[1]) {
			s.cells[c[1]*g.grid.Width()+c[0]] = 0
		}
	}
}

func (g *Game) hasGrass(x, y int) bool {
	if !g.grid.InBounds(x, y) {
		return false
	}
	return g.seedGrass().cells[y*g.grid.Width()+x] == 1
}

// clearGrass removes grass at (x, y), and any cut mark there. Before the
// layer is seeded it is a no-op: the seed already excludes covered tiles.
func (g *Game) clearGrass(x, y int) {
	s := g.grassSt()
	if !s.seeded || !g.grid.InBounds(x, y) {
		return
	}
	i := y*g.grid.Width() + x
	if s.cells[i] != 0 || s.growth != nil && s.growth[i] != 0 {
		s.ver++
	}
	s.cells[i] = 0
	delete(s.cut, [2]int{x, y})
	g.stopGrowing(s, int32(i))
}

// markGrass marks (on) or unmarks grass tiles in the rectangle for cutting.
func (g *Game) markGrass(minX, minY, maxX, maxY int, on bool) {
	s := g.grassSt()
	if !on && !s.seeded {
		return // nothing can be marked yet
	}
	s = g.seedGrass()
	w := g.grid.Width()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !g.grid.InBounds(x, y) {
				continue
			}
			k := [2]int{x, y}
			if !on {
				delete(s.cut, k)
			} else if s.cells[y*w+x] == 1 {
				s.cut[k] = true
			}
		}
	}
}

// unmarkGrass drops cut marks on grass in the (normalized) rectangle.
func (g *Game) unmarkGrass(minX, minY, maxX, maxY int) { g.markGrass(minX, minY, maxX, maxY, false) }

// grassCutList is the cut marks sorted by y, then x.
func (g *Game) grassCutList() [][2]int {
	s := g.grassSt()
	out := make([][2]int, 0, len(s.cut))
	for k := range s.cut {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][1] != out[j][1] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

// fillGrassView copies grass, its cut marks and its growth into the view.
func (g *Game) fillGrassView(w *WorldView) {
	s := g.seedGrass()
	w.Grass = shareLayer(&g.viewGrass, s.cells) // indexes.go: shared, copy-on-write
	w.GrassCut = g.grassCutList()
	w.GrassGrowth, w.GrassVer = g.grassGrowthView(s), grassFingerprint(s)
}

// grassFingerprint is a content hash of the grass and its growth, kept
// until the grass changes.
func grassFingerprint(s *grassState) uint64 {
	if s.fpOK && s.fpVer == s.ver {
		return s.fp
	}
	h := uint64(0xcbf29ce484222325)
	mix := func(b []uint8) {
		for len(b) >= 8 {
			h = (h ^ binary.LittleEndian.Uint64(b)) * 0x100000001b3
			h ^= h >> 29
			b = b[8:]
		}
		for _, v := range b {
			h = (h ^ uint64(v)) * 0x100000001b3
		}
	}
	mix(s.cells)
	if s.nGrowing > 0 {
		mix(s.growth)
	}
	s.fp, s.fpVer, s.fpOK = h|1, s.ver, true // never 0: 0 reads "no grass data"
	return s.fp
}

// exportGrass returns the persisted form: the grass bitmap (one byte per
// tile) and the cut marks. Nil bitmap means "unseeded, reseed on load".
func (g *Game) exportGrass() ([]uint8, [][2]int) {
	s := g.seedGrass()
	return append([]uint8(nil), s.cells...), g.grassCutList()
}

// importGrass restores saved grass. A nil or wrong-sized bitmap (old save)
// leaves the layer unseeded so it reseeds from terrain on first use.
func (g *Game) importGrass(cells []uint8, cut [][2]int) {
	s := g.grassSt()
	ver := s.ver + 1 // GrassVer only moves forward, across loads too
	*s = grassState{ver: ver}
	if len(cells) != g.grid.Width()*g.grid.Height() {
		return
	}
	s.seeded = true
	s.cells = append([]uint8(nil), cells...)
	s.cut = map[[2]int]bool{}
	for _, k := range cut {
		if g.grid.InBounds(k[0], k[1]) && s.cells[k[1]*g.grid.Width()+k[0]] == 1 {
			s.cut[k] = true
		}
	}
}

// assignCutGrass gives grower id the nearest reachable unreserved marked
// grass tile. It is the fallback of assignCut: marked plants (trees, bushes)
// are taken first, grass when no plant is left to cut. Candidates are checked
// nearest first (ties by y, then x), at most 16 reachability probes.
func (g *Game) assignCutGrass(id entity.EntityID) bool {
	s := g.grassSt()
	if len(s.cut) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	type cand struct{ x, y, d int }
	cs := make([]cand, 0, len(s.cut))
	for _, k := range g.grassCutList() {
		if _, taken := g.reserve.TileOwner(k[0], k[1]); taken {
			continue
		}
		if g.postponedFor(id, job.Cut, 0, k[0], k[1]) {
			continue // SIM-027: out of reach lately
		}
		cs = append(cs, cand{k[0], k[1], pathing.Dist(pos.X, pos.Y, k[0], k[1])})
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].d < cs[j].d })
	for i, c := range cs {
		if i >= 16 {
			break
		}
		tx, ty, ok := g.reachableSpot(id, c.x, c.y)
		if !ok {
			g.postponeTile(id, job.Cut, c.x, c.y)
			continue
		}
		if !g.startJob(id, job.Record{Kind: job.Cut, TX: tx, TY: ty, DestX: c.x, DestY: c.y, WorkLeft: GrassCutWork}) {
			return false
		}
		g.reserve.TryTile(id, c.x, c.y)
		return true
	}
	return false
}

// doCutGrass runs a job.Cut with Target 0 (a grass tile at DestX, DestY).
func (g *Game) doCutGrass(id entity.EntityID, j job.Record) {
	if !g.grassSt().cut[[2]int{j.DestX, j.DestY}] {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skPlants, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.clearGrass(j.DestX, j.DestY)
	g.scheduleRegrow(j.DestX, j.DestY)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// grazeGrass eats the grass on (x, y) down to bare ground and schedules it
// to grow back; false if there was no grass (SIM-023, grazing animals).
func (g *Game) grazeGrass(x, y int) bool {
	if !g.hasGrass(x, y) {
		return false
	}
	g.clearGrass(x, y)
	g.scheduleRegrow(x, y)
	return true
}

// regrowJitter is the spread of a tile's regrow time, percent either way.
const regrowJitter = 20

// regrowHeap orders pending regrowth by tick, then cell (deterministic).
type regrowHeap []grassRegrow

func (h regrowHeap) Len() int { return len(h) }
func (h regrowHeap) Less(i, j int) bool {
	return h[i].At < h[j].At || h[i].At == h[j].At && h[i].Cell < h[j].Cell
}
func (h regrowHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *regrowHeap) Push(x any)   { *h = append(*h, x.(grassRegrow)) }
func (h *regrowHeap) Pop() any {
	old := *h
	v := old[len(old)-1]
	*h = old[:len(old)-1]
	return v
}

// scheduleRegrow queues bare tile (x, y) to grow grass back in the biome's
// Regrow days, +-regrowJitter percent; a biome whose grass never regrows
// (Regrow 0) queues nothing.
func (g *Game) scheduleRegrow(x, y int) {
	days := biome.DefOf(g.biome).Regrow
	if days <= 0 || !g.grid.InBounds(x, y) {
		return
	}
	base := uint64(days) * clock.TicksPerDay
	span := base * 2 * regrowJitter / 100
	at := uint64(g.clock.Now()) + base - span/2 + uint64(g.rng.IntN(int(span)+1))
	heap.Push((*regrowHeap)(&g.grass.regrow), grassRegrow{At: at, Cell: int32(y*g.grid.Width() + x)})
}

// regrowGrass grows back every queued tile that is due, unless it was
// built on, floored, sown or turned to barren ground meanwhile.
func (g *Game) regrowGrass() {
	s := g.grassSt()
	now := uint64(g.clock.Now())
	if len(s.regrow) == 0 || s.regrow[0].At > now {
		return
	}
	h := (*regrowHeap)(&s.regrow)
	s = g.seedGrass()
	w, n := g.grid.Width(), int32(len(s.cells))
	for len(s.regrow) > 0 && s.regrow[0].At <= now {
		c := heap.Pop(h).(grassRegrow).Cell
		if c < 0 || c >= n || s.cells[c] == 1 {
			continue
		}
		if x, y := int(c)%w, int(c)/w; g.grassCanRegrow(x, y) {
			s.cells[c] = 1
			g.startGrowing(s, c, plant.GrassShoots) // PLAY-010: shoots, filling out over days
		}
	}
}

// grassCanRegrow reports bare grass ground with nothing built, planned or
// growing on it.
func (g *Game) grassCanRegrow(x, y int) bool {
	if !plant.GrassGround(g.terr.At(x, y), biome.DefOf(g.biome).Legacy) || !g.grid.Walkable(x, y) {
		return false
	}
	if g.plantAt(x, y) != 0 {
		return false
	}
	return g.layerBldgAt(x, y, construction.LayerObject) == 0 && g.layerBldgAt(x, y, construction.LayerFloor) == 0 &&
		g.layerFrameAt(x, y, construction.LayerObject) == 0 && g.layerFrameAt(x, y, construction.LayerFloor) == 0
}

// ---- grass growth (PLAY-010) ----
//
// A grazed or cut tile regrows as shoots (plant.GrassShoots) and fills
// out to full over plant.GrassFillTicks of mild weather. Only regrowing
// tiles carry a growth value; they are listed per chunk and grown in the
// staggered per-chunk plant pass (plant_growth.go), so the work is the
// few regrowing tiles of this tick's chunks, never a map scan.

// grassGrowthAt is the growth percent of the grass on (x, y): 100 for a
// full tile, 0 where there is no grass.
func (g *Game) grassGrowthAt(x, y int) int {
	if !g.hasGrass(x, y) {
		return 0
	}
	s := g.grassSt()
	if s.growth == nil {
		return plant.GrassFull
	}
	if v := s.growth[y*g.grid.Width()+x]; v != 0 {
		return int(v)
	}
	return plant.GrassFull
}

// startGrowing sets tile i growing at growth v (full when v >= 100).
func (g *Game) startGrowing(s *grassState, i int32, v int) {
	s.ver++
	if v >= plant.GrassFull {
		g.stopGrowing(s, i)
		return
	}
	cg := chunk.New(g.grid.Width(), g.grid.Height())
	if s.growth == nil {
		s.growth = make([]uint8, len(s.cells))
	}
	if s.growing == nil {
		s.growing = make([][]int32, cg.Count())
	}
	if s.growth[i] == 0 {
		c := cg.OfIndex(int(i))
		s.growing[c] = append(s.growing[c], i)
		s.nGrowing++
	}
	s.growth[i] = uint8(max(v, 1))
}

// stopGrowing takes tile i out of the growing set (full, or bare).
func (g *Game) stopGrowing(s *grassState, i int32) {
	if s.growth == nil || s.growth[i] == 0 {
		return
	}
	s.growth[i] = 0
	s.ver++
	c := chunk.New(g.grid.Width(), g.grid.Height()).OfIndex(int(i))
	l := s.growing[c]
	for k, v := range l {
		if v == i {
			s.growing[c] = append(l[:k], l[k+1:]...)
			s.nGrowing--
			return
		}
	}
}

// setGrassGrowth sets the growth of the grass on (x, y) (tests, fixtures).
func (g *Game) setGrassGrowth(x, y, v int) {
	if !g.hasGrass(x, y) {
		return
	}
	g.startGrowing(g.grassSt(), int32(y*g.grid.Width()+x), v)
}

// growGrassChunk grows chunk c's regrowing grass over the climate
// interval that ends now: plant.GrassGrowSpan at the tile's air
// temperature. Snow cover and frost pause it; a tile reaching full, or
// one that lost its grass, leaves the growing set.
func (g *Game) growGrassChunk(c int, now uint64) {
	s := g.grassSt()
	if s.nGrowing == 0 || len(s.growing[c]) == 0 {
		return
	}
	w := g.grid.Width()
	l := s.growing[c]
	kept := l[:0]
	for _, i := range l {
		if s.cells[i] == 0 { // bared behind clearGrass's back (fixtures)
			s.growth[i] = 0
			s.nGrowing--
			s.ver++
			continue
		}
		x, y := int(i)%w, int(i)/w
		if g.snowAt(x, y) >= snowGrassCover { // snow.go: this tile's own snow (PLAY-015)
			kept = append(kept, i)
			continue
		}
		tf := plant.TempFactor(int(g.temperatureAt(x, y)))
		n := plant.GrassGrowSpan(tf, now-min(now, climateInterval), now)
		if n <= 0 {
			kept = append(kept, i)
			continue
		}
		s.ver++
		if v := int(s.growth[i]) + n; v < plant.GrassFull {
			s.growth[i] = uint8(v)
			kept = append(kept, i)
			continue
		}
		s.growth[i] = 0 // full
		s.nGrowing--
	}
	clear(l[len(kept):]) // tidy the dropped tail
	s.growing[c] = kept
}

// grassGrowthView is the read model's growth layer: nil while every grass
// tile is full, else 0..100 per tile (full grass reads 100). A slice once
// handed out is never written again; a new one is built when the grass
// changed.
func (g *Game) grassGrowthView(s *grassState) []uint8 {
	if s.nGrowing == 0 {
		return nil
	}
	if s.view != nil && s.viewVer == s.ver {
		return s.view
	}
	out := make([]uint8, len(s.cells))
	for i, c := range s.cells {
		if c == 0 {
			continue
		}
		if v := s.growth[i]; v != 0 {
			out[i] = v
		} else {
			out[i] = plant.GrassFull
		}
	}
	s.view, s.viewVer = out, s.ver
	return out
}

// exportGrassGrowth is the saved growth layer (0: full or no grass), nil
// while nothing is growing, so saves of full maps stay as they were.
func (g *Game) exportGrassGrowth() []uint8 {
	s := g.grassSt()
	if s.nGrowing == 0 {
		return nil
	}
	return append([]uint8(nil), s.growth...)
}

// importGrassGrowth restores a saved growth layer after importGrass; a
// missing (older save) or malformed layer leaves every tile full.
func (g *Game) importGrassGrowth(v []uint8) {
	s := g.grassSt()
	s.growth, s.growing, s.nGrowing, s.view = nil, nil, 0, nil
	if !s.seeded || len(v) != len(s.cells) {
		return
	}
	for i, x := range v {
		if x != 0 && s.cells[i] == 1 {
			g.startGrowing(s, int32(i), int(x))
		}
	}
}
