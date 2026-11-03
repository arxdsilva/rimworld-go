package snow

import (
	"math"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/chunk"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
	"rimworld-go/pkg/randx"
)

// The work is phased by chunk like plant growth (game/plant_growth.go):
// each chunk is brought up to date once per climate interval, at its own
// tick, with the change of the climate step that began the interval. That
// change (Op) is derived from the weather and the clock alone, so a
// loaded game continues exactly; its fraction is dithered per (step,
// chunk) rather than carried. A chunk with no snow is skipped unless snow
// is falling, so a snowless map costs nothing.
//
// The layer is nil while no snow lies. The weather's SnowDepth (the HUD's
// map figure) is then the weather model's own; once the layer exists it is
// the average depth of the snow lying on the map. A nil layer with a
// global depth (an older save, the first step of a snowfall) reads that
// depth on every open cell and seeds the layer at the next climate step.
// When everything has melted and nothing falls the layer is dropped.

const (
	fallPerStep = 0.012 * 255 // depth units a climate step at full intensity (weather.State.Step)
	meltPerDeg  = 0.004 * 255 // depth units a climate step per degree C above freezing

	// Deep is the depth (30%) above which walking is slowed (game weatherSlow).
	Deep = 77
	// GrassCover is the depth (10%) that buries grass and stops it regrowing.
	GrassCover = 26
)

// State is the per-tile snow depth and the snow clearing area.
type State struct {
	depth []uint8  // per cell, row-major map-width wide; nil: none lying
	clear []uint64 // the snow clearing area, one bit per cell; nil: none
	ver   uint64   // moves with every change to the layer (view cache)

	// derived (rebuilt by recount; never saved)
	chunkSum []uint32 // per chunk, the sum of its depths
	clearN   []uint16 // per chunk, area cells deep enough to clear (clear.go)
	sum      uint64   // sum of every depth
	nz       int      // cells with snow lying
	view     []uint8  // the layer last handed out, at viewVer
	viewVer  uint64
	viewOK   bool
	viewFP   uint64 // View.SnowVer: the view's content hash

	clearVer     uint64 // moves with every change to the area (view cache)
	clearView    []uint8
	clearViewVer uint64
	clearViewOK  bool
	clearFP      uint64 // View.ClearVer: the area's content hash
}

// Layer is the operations on a State over its host. It holds nothing of
// its own: the game builds one per call, over the current map's State.
type Layer struct {
	s     *State
	h     Host
	steps Steps
}

// New is the layer over s.
func New(h Host, s *State, steps Steps) Layer { return Layer{s: s, h: h, steps: steps} }

// chunkGrid is the chunk grid the layer is phased over.
func (l Layer) chunkGrid() chunk.Grid {
	w, h := l.h.Size()
	return chunk.New(w, h)
}

// At is the snow depth on (x, y), 0..255.
func (l Layer) At(x, y int) uint8 {
	w, h := l.h.Size()
	if x < 0 || x >= w || y < 0 || y >= h {
		return 0
	}
	s := l.s
	if s.depth == nil {
		d := l.h.Weather().SnowDepth
		if d <= 0 || !l.open(x, y) {
			return 0
		}
		return units(d)
	}
	return s.depth[y*w+x]
}

// units is a 0..1 depth in layer units.
func units(d float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, d)) * 255)) }

// open reports a cell that holds snow: under the sky, not a wall, not
// water, not rock.
func (l Layer) open(x, y int) bool {
	t := l.h.Terrain(x, y)
	return l.h.UnderSky(x, y) && !terrain.IsWater(t) && !terrain.IsRockWall(t)
}

// ensure allocates an empty layer (and the derived counts).
func (l Layer) ensure() {
	s := l.s
	w, h := l.h.Size()
	if n := w * h; len(s.depth) != n {
		s.depth = make([]uint8, n)
		l.recount()
	}
}

// SetCell sets cell i's depth (clearing, tests), keeping the counts.
func (l Layer) SetCell(i int, v uint8) {
	l.ensure()
	s := l.s
	old := s.depth[i]
	if old == v {
		return
	}
	s.depth[i] = v
	c := l.chunkGrid().OfIndex(i)
	s.chunkSum[c] = s.chunkSum[c] - uint32(old) + uint32(v)
	s.sum = s.sum - uint64(old) + uint64(v)
	switch {
	case old == 0:
		s.nz++
	case v == 0:
		s.nz--
	}
	l.recountClearChunk(c) // clear.go
	s.ver++
	l.global()
}

// recount rebuilds the derived counts from the layer.
func (l Layer) recount() {
	s := l.s
	cg := l.chunkGrid()
	n := cg.Count()
	s.chunkSum = make([]uint32, n)
	s.clearN = make([]uint16, n)
	s.sum, s.nz = 0, 0
	w, _ := l.h.Size()
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
			l.recountClearChunk(c)
		}
	}
}

// global sets the map figure from the layer: the average depth of the
// snow lying, 0..1.
func (l Layer) global() {
	s := l.s
	if s.depth == nil {
		return
	}
	if s.nz == 0 {
		l.h.SetMapDepth(0)
		return
	}
	l.h.SetMapDepth(float64(s.sum) / float64(s.nz) / 255)
}

// Seed seeds the layer from the global depth on every open cell (an
// older save, or the first step of a snowfall). Called at the top of
// the weather phase on a climate step, before the step changes the
// weather.
func (l Layer) Seed() {
	s := l.s
	d := l.h.Weather().SnowDepth
	if s.depth != nil || d <= 0 {
		return
	}
	w, h := l.h.Size()
	v := units(d)
	s.depth = make([]uint8, w*h)
	l.h.EnsureRooms()
	for y := range h {
		for x := range w {
			if l.open(x, y) {
				s.depth[y*w+x] = v
			}
		}
	}
	l.recount()
	s.ver++
	l.global()
}

// SeedDue is the weather phase's first call: on a climate step it seeds
// the layer from a global depth that has none yet.
func (l Layer) SeedDue() {
	if uint64(l.h.Now())%l.steps.Climate == 0 {
		l.Seed()
	}
}

// Chunks brings this tick's chunks up to date (the weather phase, after
// the tick's weather step).
func (l Layer) Chunks() { l.ChunksAt(uint64(l.h.Now())) }

// ChunksAt runs the chunks whose phase is tick now.
func (l Layer) ChunksAt(now uint64) {
	s := l.s
	if s.depth == nil {
		return
	}
	ph := now % l.steps.Climate
	step := now - ph
	if ph == 0 {
		if s.nz == 0 && !l.h.Weather().Kind.Frozen() {
			*s = State{clear: s.clear, ver: s.ver + 1, clearVer: s.clearVer,
				clearView: s.clearView, clearViewVer: s.clearViewVer, clearViewOK: s.clearViewOK, clearFP: s.clearFP}
			l.h.SetMapDepth(0)
			return
		}
	}
	cg := l.chunkGrid()
	n := cg.Count()
	p := int(ph)
	ci := int(l.steps.Climate)
	ran := false
	for c := (p*n + ci - 1) / ci; c < n && c*ci/n == p; c++ {
		halve, delta := l.op(step, c)
		if s.chunkSum[c] == 0 && delta <= 0 {
			continue
		}
		if !ran {
			l.h.EnsureRooms()
			ran = true
		}
		l.chunk(cg, c, halve, delta)
	}
	if ran {
		l.global()
	}
}

// op is the change climate step `step` makes to chunk c: halve (a rain
// or storm block began), then add delta (fall minus melt), clamped. The
// fraction is dithered per (step, chunk).
func (l Layer) op(step uint64, c int) (halve bool, delta int) {
	wx := l.h.Weather()
	halve = step%l.steps.WeatherBlock == 0 && !l.h.WeatherHeld() && (wx.Target == weather.Rain || wx.Target == weather.Storm)
	f := 0.0
	if wx.Kind.Frozen() {
		f += fallPerStep * wx.Intensity
	}
	tempC := float64(l.h.Outdoor(clock.Tick(step))) / 10
	f -= meltPerDeg * math.Max(0, tempC)
	fl := math.Floor(f)
	delta = int(fl)
	if fr := f - fl; fr > 0 && dither(step, c) < fr {
		delta++
	}
	return halve, delta
}

// dither is a deterministic value in [0, 1) from (step, chunk).
func dither(step uint64, c int) float64 {
	return float64(randx.SplitMix64(randx.SplitMix64(step^0x5e0a)^uint64(c))>>11) / (1 << 53)
}

// chunk applies one step's change to chunk c: open cells take it,
// covered ones are cleared. The rooms must be current (EnsureRooms).
func (l Layer) chunk(cg chunk.Grid, c int, halve bool, delta int) {
	s := l.s
	x0, y0, x1, y1 := cg.Bounds(c)
	w, _ := l.h.Size()
	rg, roofs, cells := l.h.Cover()
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
	l.recountClearChunk(c) // clear.go
}
