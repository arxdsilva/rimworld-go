package game

import (
	"math"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/chunk"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// Per-tile snow (PLAY-015 §4): 0..255 per cell (255 = deepest) on open
// ground, i.e. unroofed, not a wall, not water, not rock. It follows the
// weather model's own rates (weather.State.Step): it builds up while snow
// falls, by intensity, melts above freezing and is halved when a rain or
// storm block begins. Roofed cells stay bare.
//
// The work is phased by chunk like plant growth (plant_growth.go): each
// chunk is brought up to date once per climate interval, at its own tick,
// with the change of the climate step that began the interval. That
// change (snowOp) is derived from the weather and the clock alone, so a
// loaded game continues exactly; its fraction is dithered per (step,
// chunk) rather than carried. A chunk with no snow is skipped unless snow
// is falling, so a snowless map costs nothing.
//
// The layer is nil while no snow lies. g.weather.SnowDepth (the HUD's map
// figure) is then the weather model's own; once the layer exists it is
// the average depth of the snow lying on the map. A nil layer with a
// global depth (an older save, the first step of a snowfall) reads that
// depth on every open cell and seeds the layer at the next climate step.
// When everything has melted and nothing falls the layer is dropped.

const (
	snowFallPerStep = 0.012 * 255 // depth units a climate step at full intensity (weather.State.Step)
	snowMeltPerDeg  = 0.004 * 255 // depth units a climate step per degree C above freezing
	snowDeep        = 77          // depth (30%) above which walking is slowed (weatherSlow)
	snowGrassCover  = 26          // depth (10%) that buries grass and stops it regrowing
)

// snowState is the per-tile snow depth and the snow clearing area.
type snowState struct {
	depth []uint8  // per cell, row-major grid.Width() wide; nil: none lying
	clear []uint64 // the snow clearing area, one bit per cell; nil: none
	ver   uint64   // moves with every change to the layer (view cache)

	// derived (rebuilt by snowRecount; never saved)
	chunkSum []uint32 // per chunk, the sum of its depths
	clearN   []uint16 // per chunk, area cells deep enough to clear (snow_clear.go)
	sum      uint64   // sum of every depth
	nz       int      // cells with snow lying
	view     []uint8  // the layer last handed out, at viewVer
	viewVer  uint64
	viewOK   bool
	viewFP   uint64 // WorldView.SnowVer: the view's content hash

	clearVer     uint64 // moves with every change to the area (view cache)
	clearView    []uint8
	clearViewVer uint64
	clearViewOK  bool
	clearFP      uint64 // WorldView.SnowClearVer: the area's content hash
}

// snowChunkGrid is the chunk grid the layer is phased over.
func (g *Game) snowChunkGrid() chunk.Grid { return chunk.New(g.grid.Width(), g.grid.Height()) }

// snowAt is the snow depth on (x, y), 0..255.
func (g *Game) snowAt(x, y int) uint8 {
	if !g.grid.InBounds(x, y) {
		return 0
	}
	s := &g.snow
	if s.depth == nil {
		if g.weather.SnowDepth <= 0 || !g.snowOpen(x, y) {
			return 0
		}
		return snowUnits(g.weather.SnowDepth)
	}
	return s.depth[y*g.grid.Width()+x]
}

// snowUnits is a 0..1 depth in layer units.
func snowUnits(d float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, d)) * 255)) }

// snowOpen reports a cell that holds snow: under the sky, not a wall, not
// water, not rock.
func (g *Game) snowOpen(x, y int) bool {
	t := g.terr.At(x, y)
	return g.underSky(x, y) && !terrain.IsWater(t) && !terrain.IsRockWall(t)
}

// ensureSnow allocates an empty layer (and the derived counts).
func (g *Game) ensureSnow() {
	s := &g.snow
	if n := g.grid.Width() * g.grid.Height(); len(s.depth) != n {
		s.depth = make([]uint8, n)
		g.snowRecount()
	}
}

// setSnowCell sets cell i's depth (clearing, tests), keeping the counts.
func (g *Game) setSnowCell(i int, v uint8) {
	g.ensureSnow()
	s := &g.snow
	old := s.depth[i]
	if old == v {
		return
	}
	s.depth[i] = v
	c := g.snowChunkGrid().OfIndex(i)
	s.chunkSum[c] = s.chunkSum[c] - uint32(old) + uint32(v)
	s.sum = s.sum - uint64(old) + uint64(v)
	switch {
	case old == 0:
		s.nz++
	case v == 0:
		s.nz--
	}
	g.recountClearChunk(c) // snow_clear.go
	s.ver++
	g.snowGlobal()
}

// snowRecount rebuilds the derived counts from the layer.
func (g *Game) snowRecount() {
	s := &g.snow
	cg := g.snowChunkGrid()
	n := cg.Count()
	s.chunkSum = make([]uint32, n)
	s.clearN = make([]uint16, n)
	s.sum, s.nz = 0, 0
	w := g.grid.Width()
	for i, v := range s.depth {
		if v == 0 {
			continue
		}
		s.chunkSum[cg.Of(i%w, i/w)] += uint32(v)
		s.sum += uint64(v)
		s.nz++
	}
	if s.depth != nil {
		for c := range n {
			g.recountClearChunk(c)
		}
	}
}

// snowGlobal sets the map figure from the layer: the average depth of the
// snow lying, 0..1.
func (g *Game) snowGlobal() {
	s := &g.snow
	if s.depth == nil {
		return
	}
	if s.nz == 0 {
		g.weather.SnowDepth = 0
		return
	}
	g.weather.SnowDepth = float64(s.sum) / float64(s.nz) / 255
}

// seedSnow seeds the layer from the global depth on every open cell (an
// older save, or the first step of a snowfall). Called at the top of
// phaseWeather on a climate step, before the step changes the weather.
func (g *Game) seedSnow() {
	s := &g.snow
	if s.depth != nil || g.weather.SnowDepth <= 0 {
		return
	}
	w, h := g.grid.Width(), g.grid.Height()
	v := snowUnits(g.weather.SnowDepth)
	s.depth = make([]uint8, w*h)
	g.ensureRooms()
	for y := range h {
		for x := range w {
			if g.snowOpen(x, y) {
				s.depth[y*w+x] = v
			}
		}
	}
	g.snowRecount()
	s.ver++
	g.snowGlobal()
}

// phaseSnowSeed is phaseWeather's first call: on a climate step it seeds
// the layer from a global depth that has none yet.
func (g *Game) phaseSnowSeed() {
	if uint64(g.clock.Now())%climateInterval == 0 {
		g.seedSnow()
	}
}

// snowChunks brings this tick's chunks up to date (phaseWeather, after the
// tick's weather step).
func (g *Game) snowChunks() { g.snowChunksAt(uint64(g.clock.Now())) }

// snowChunksAt runs the chunks whose phase is tick now.
func (g *Game) snowChunksAt(now uint64) {
	s := &g.snow
	if s.depth == nil {
		return
	}
	ph := now % climateInterval
	step := now - ph
	if ph == 0 {
		if s.nz == 0 && !g.weather.Kind.Frozen() {
			*s = snowState{clear: s.clear, ver: s.ver + 1, clearVer: s.clearVer,
				clearView: s.clearView, clearViewVer: s.clearViewVer, clearViewOK: s.clearViewOK, clearFP: s.clearFP}
			g.weather.SnowDepth = 0
			return
		}
	}
	cg := g.snowChunkGrid()
	n := cg.Count()
	p := int(ph)
	ran := false
	for c := (p*n + climateInterval - 1) / climateInterval; c < n && c*climateInterval/n == p; c++ {
		halve, delta := g.snowOp(step, c)
		if s.chunkSum[c] == 0 && delta <= 0 {
			continue
		}
		if !ran {
			g.ensureRooms()
			ran = true
		}
		g.snowChunk(cg, c, halve, delta)
	}
	if ran {
		g.snowGlobal()
	}
}

// snowOp is the change climate step `step` makes to chunk c: halve (a rain
// or storm block began), then add delta (fall minus melt), clamped. The
// fraction is dithered per (step, chunk).
func (g *Game) snowOp(step uint64, c int) (halve bool, delta int) {
	wx := g.weather
	halve = step%weatherBlockTicks == 0 && !g.weatherFix && (wx.Target == weather.Rain || wx.Target == weather.Storm)
	f := 0.0
	if wx.Kind.Frozen() {
		f += snowFallPerStep * wx.Intensity
	}
	tempC := float64(g.climate.Outdoor(clock.Tick(step))) / 10
	f -= snowMeltPerDeg * math.Max(0, tempC)
	fl := math.Floor(f)
	delta = int(fl)
	if fr := f - fl; fr > 0 && snowDither(step, c) < fr {
		delta++
	}
	return halve, delta
}

// snowDither is a deterministic value in [0, 1) from (step, chunk).
func snowDither(step uint64, c int) float64 {
	return float64(mix64s(mix64s(step^0x5e0a)^uint64(c))>>11) / (1 << 53)
}

func mix64s(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}

// snowChunk applies one step's change to chunk c: open cells take it,
// covered ones are cleared. The rooms must be current (ensureRooms).
func (g *Game) snowChunk(cg chunk.Grid, c int, halve bool, delta int) {
	s := &g.snow
	x0, y0, x1, y1 := cg.Bounds(c)
	w := g.grid.Width()
	rg, roofs, cells := g.roomGrid, g.roofs, g.terr.Cells
	var sum uint32
	nz, oldNZ := 0, 0
	changed := false
	for y := y0; y < y1; y++ {
		for i := y*w + x0; i < y*w+x1; i++ {
			old := s.depth[i]
			if old != 0 {
				oldNZ++
			}
			v := 0
			if (i >= len(rg) || rg[i] != room.Wall) && (i >= len(roofs) || !roofs[i].Roofed()) &&
				!terrain.IsWater(cells[i]) && !terrain.IsRockWall(cells[i]) {
				v = int(old)
				if halve {
					v >>= 1
				}
				v = min(max(v+delta, 0), 255)
			}
			if uint8(v) != old {
				s.depth[i] = uint8(v)
				changed = true
			}
			if v != 0 {
				sum += uint32(v)
				nz++
			}
		}
	}
	if !changed {
		return
	}
	s.ver++
	s.sum = s.sum - uint64(s.chunkSum[c]) + uint64(sum)
	s.nz += nz - oldNZ
	s.chunkSum[c] = sum
	g.recountClearChunk(c) // snow_clear.go
}
