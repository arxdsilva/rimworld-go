package snow

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// fakeHost is a w x h open map under weather the test sets.
type fakeHost struct {
	w, h   int
	now    clock.Tick
	wx     weather.State
	held   bool
	temp   temperature.Temp
	rooms  []int
	roofs  []roof.Cell
	cells  []terrain.Type
	px, py int

	taken       map[[2]int]bool
	blocked     map[[2]int]bool // not allowed
	postponed   map[[2]int]bool
	unreachable map[[2]int]bool
	work        int
	refuseJob   bool
	claims      [][2]int
	jobs        []job.Record
}

func newFake(w, h int) *fakeHost {
	return &fakeHost{w: w, h: h, cells: make([]terrain.Type, w*h), work: 10,
		taken: map[[2]int]bool{}, blocked: map[[2]int]bool{}, postponed: map[[2]int]bool{}, unreachable: map[[2]int]bool{}}
}

func (f *fakeHost) Now() clock.Tick                        { return f.now }
func (f *fakeHost) Weather() weather.State                 { return f.wx }
func (f *fakeHost) SetMapDepth(d float64)                  { f.wx.SnowDepth = d }
func (f *fakeHost) SkipClear() bool                        { return false }
func (f *fakeHost) WeatherHeld() bool                      { return f.held }
func (f *fakeHost) Outdoor(clock.Tick) temperature.Temp    { return f.temp }
func (f *fakeHost) Size() (int, int)                       { return f.w, f.h }
func (f *fakeHost) EnsureRooms()                           {}
func (f *fakeHost) Terrain(x, y int) terrain.Type          { return f.cells[y*f.w+x] }
func (f *fakeHost) Pos(entity.EntityID) (int, int)         { return f.px, f.py }
func (f *fakeHost) Work(entity.EntityID) int               { return f.work }
func (f *fakeHost) TileTaken(x, y int) bool                { return f.taken[[2]int{x, y}] }
func (f *fakeHost) SetJob(_ entity.EntityID, j job.Record) { f.jobs = append(f.jobs, j) }
func (f *fakeHost) ClaimTile(_ entity.EntityID, x, y int) {
	f.claims = append(f.claims, [2]int{x, y})
}
func (f *fakeHost) Cover() ([]int, []roof.Cell, []terrain.Type) { return f.rooms, f.roofs, f.cells }
func (f *fakeHost) UnderSky(x, y int) bool {
	i := y*f.w + x
	return i >= len(f.roofs) || !f.roofs[i].Roofed()
}
func (f *fakeHost) Allowed(_ entity.EntityID, x, y int) bool { return !f.blocked[[2]int{x, y}] }
func (f *fakeHost) PostponedFor(_ entity.EntityID, _ job.Kind, x, y int) bool {
	return f.postponed[[2]int{x, y}]
}
func (f *fakeHost) PostponeTile(_ entity.EntityID, _ job.Kind, x, y int) {
	f.postponed[[2]int{x, y}] = true
}
func (f *fakeHost) ReachableSpot(_ entity.EntityID, x, y int) (int, int, bool) {
	return x, y, !f.unreachable[[2]int{x, y}]
}
func (f *fakeHost) StartJob(_ entity.EntityID, rec job.Record) bool {
	if f.refuseJob {
		return false
	}
	f.jobs = append(f.jobs, rec)
	return true
}

const (
	testClimate = 250
	testBlock   = 15000
)

func layer(f *fakeHost) (Layer, *State) {
	s := &State{}
	return New(f, s, Steps{Climate: testClimate, WeatherBlock: testBlock}), s
}

func TestOpRainBlockHalves(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	f.temp = -50
	l, _ := layer(f)
	block := uint64(testBlock) * 7
	f.wx.Kind, f.wx.Target = weather.Rain, weather.Rain
	if halve, _ := l.op(block, 0); !halve {
		t.Fatal("a rain block's first step halves the snow")
	}
	if halve, _ := l.op(block+testClimate, 0); halve {
		t.Fatal("only the block's first step halves")
	}
	f.wx.Target = weather.Storm
	if halve, _ := l.op(block, 0); !halve {
		t.Fatal("a storm block halves too")
	}
	f.wx.Kind, f.wx.Target = weather.Snow, weather.Snow
	if halve, _ := l.op(block, 0); halve {
		t.Fatal("a snow block does not halve")
	}
	f.wx.Target = weather.Rain
	f.held = true
	if halve, _ := l.op(block, 0); halve {
		t.Fatal("held weather starts no block")
	}
}

func TestRainBlockHalvesCells(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	l, _ := layer(f)
	i := 8*f.w + 8
	l.SetCell(i, 200)
	cg := l.chunkGrid()
	l.chunk(cg, cg.Of(8, 8), true, 0)
	if d := l.At(8, 8); d != 100 {
		t.Fatalf("halved depth %d, want 100", d)
	}
}

func TestOpFallAndMeltRates(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	f.temp = -50
	f.wx.Kind, f.wx.Target, f.wx.Intensity = weather.Snow, weather.Snow, 1
	l, _ := layer(f)
	sum := 0
	for c := range 100 {
		_, d := l.op(uint64(testClimate*(c+40)), c)
		sum += d
	}
	if sum < 290 || sum > 322 { // 3.06 a step at full intensity, dithered
		t.Fatalf("100 full-intensity steps add %d, want about 306", sum)
	}
	f.temp = 100
	f.wx.Kind, f.wx.Target, f.wx.Intensity = weather.Clear, weather.Clear, 0
	if _, d := l.op(uint64(testClimate*40), 0); d > -10 || d < -11 {
		t.Fatalf("10 C melts %d a step, want -10 or -11", d)
	}
}

func TestClearWorkScalesWithDepth(t *testing.T) {
	t.Parallel()
	if a, b := ClearWork(ClearStart), ClearWork(255); a < ClearMinWork || b != ClearMaxWork || a >= b {
		t.Fatalf("work %d..%d", a, b)
	}
}

// TestPaintVersionMovesOnlyOnChange: a no-op paint keeps the area's
// version (the view cache), a real one moves it.
func TestPaintVersionMovesOnlyOnChange(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	l, s := layer(f)
	l.PaintClear(5, 4, 2, 2, true)
	v0 := s.clearVer
	if v0 == 0 || !l.InClear(2*f.w+2) || !l.InClear(4*f.w+5) || l.InClear(4*f.w+6) {
		t.Fatal("rectangle painted")
	}
	l.PaintClear(2, 2, 2, 2, true)
	l.PaintClear(-5, -5, -1, -1, true)
	if s.clearVer != v0 {
		t.Fatal("a no-op paint keeps the version")
	}
	l.PaintClear(2, 2, 2, 2, false)
	if s.clearVer == v0 || l.InClear(2*f.w+2) {
		t.Fatal("clearing moves the version")
	}
}

func TestPaintOffWithoutAreaAllocatesNothing(t *testing.T) {
	t.Parallel()
	l, s := layer(newFake(40, 20))
	l.PaintClear(1, 1, 3, 3, false)
	if s.clear != nil {
		t.Fatal("clearing nothing makes no area")
	}
}

// TestGlobalDepthReadsOnOpenCells: with no layer the global depth reads on
// open ground only, and seeding copies exactly that.
func TestGlobalDepthReadsOnOpenCells(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	f.roofs = make([]roof.Cell, 40*20)
	f.roofs[3*40+3] = roof.Built
	f.cells[10*40+30] = terrain.ShallowWater
	f.wx.SnowDepth = 0.4
	f.now = testClimate * 3
	l, s := layer(f)
	for _, c := range []struct {
		x, y int
		want uint8
	}{{10, 10, 102}, {3, 3, 0}, {30, 10, 0}, {-1, 0, 0}, {40, 0, 0}} {
		if d := l.At(c.x, c.y); d != c.want {
			t.Fatalf("(%d,%d) reads %d before seeding, want %d", c.x, c.y, d, c.want)
		}
	}
	l.SeedDue()
	if s.depth == nil || l.At(10, 10) != 102 || l.At(3, 3) != 0 || l.At(30, 10) != 0 {
		t.Fatal("the layer seeds what the global depth read")
	}
	if d := f.wx.SnowDepth; d != 102.0/255 {
		t.Fatalf("map figure %v, want the average of what lies", d)
	}
}

func TestSeedDueOnlyOnClimateStep(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	f.wx.SnowDepth = 0.4
	f.now = testClimate + 1
	l, s := layer(f)
	l.SeedDue()
	if s.depth != nil {
		t.Fatal("mid-interval: no seed")
	}
}

// TestChunksVisitEachOnceAndDrop: over one interval every chunk moves
// once, and a layer with nothing left is dropped at the next step.
func TestChunksVisitEachOnceAndDrop(t *testing.T) {
	t.Parallel()
	f := newFake(64, 64)
	f.temp = -50
	f.wx.Kind, f.wx.Target, f.wx.Intensity = weather.Clear, weather.Clear, 0
	l, s := layer(f)
	cg := l.chunkGrid()
	x, y := 40, 40
	f.rooms = make([]int, 64*64)
	f.rooms[y*64+x+1] = room.Wall
	l.SetCell(y*64+x, 100)
	l.SetCell(y*64+x+1, 100) // under a wall: cleared by its chunk's pass
	base := uint64(testBlock)*3 + testClimate
	for ph := range uint64(testClimate) {
		l.ChunksAt(base + ph)
	}
	if l.At(x, y) != 100 || l.At(x+1, y) != 0 {
		t.Fatalf("after one interval %d, %d; want 100, 0", l.At(x, y), l.At(x+1, y))
	}
	if s.chunkSum[cg.Of(x, y)] != 100 || s.nz != 1 || s.sum != 100 {
		t.Fatalf("counts sum %d nz %d", s.sum, s.nz)
	}
	l.SetCell(y*64+x, 0)
	if f.wx.SnowDepth != 0 {
		t.Fatalf("map figure %v over bare ground", f.wx.SnowDepth)
	}
	l.PaintClear(1, 1, 1, 1, true)
	l.ChunksAt(base + testClimate)
	if s.depth != nil || s.clear == nil {
		t.Fatal("a melted layer is dropped, the area kept")
	}
}

func clearable(f *fakeHost, l Layer, cells ...[2]int) {
	for _, c := range cells {
		l.PaintClear(c[0], c[1], c[0], c[1], true)
		l.SetCell(c[1]*f.w+c[0], 200)
	}
}

func TestAssignClearNearestFree(t *testing.T) {
	t.Parallel()
	f := newFake(64, 64)
	f.px, f.py = 0, 0
	l, _ := layer(f)
	clearable(f, l, [2]int{2, 0}, [2]int{4, 0}, [2]int{6, 0}, [2]int{8, 0}, [2]int{40, 40})
	f.taken[[2]int{2, 0}] = true
	f.blocked[[2]int{4, 0}] = true
	f.postponed[[2]int{6, 0}] = true
	if !l.AssignClear(1) {
		t.Fatal("no job")
	}
	want := ClearWork(200)
	j := f.jobs[0]
	if j.Kind != job.ClearSnow || j.DestX != 8 || j.DestY != 0 || j.WorkLeft != want || j.Count != want {
		t.Fatalf("job %+v", j)
	}
	if !slices.Equal(f.claims, [][2]int{{8, 0}}) {
		t.Fatalf("claims %v", f.claims)
	}
}

func TestAssignClearPostponesUnreachable(t *testing.T) {
	t.Parallel()
	f := newFake(64, 64)
	l, _ := layer(f)
	clearable(f, l, [2]int{2, 0}, [2]int{4, 0})
	f.unreachable[[2]int{2, 0}] = true
	if !l.AssignClear(1) || f.jobs[0].DestX != 4 {
		t.Fatalf("jobs %+v", f.jobs)
	}
	if !f.postponed[[2]int{2, 0}] {
		t.Fatal("an unreachable cell is postponed")
	}
}

func TestAssignClearRefused(t *testing.T) {
	t.Parallel()
	f := newFake(64, 64)
	l, _ := layer(f)
	if l.AssignClear(1) {
		t.Fatal("no layer, no job")
	}
	clearable(f, l, [2]int{2, 0})
	f.refuseJob = true
	if l.AssignClear(1) || len(f.claims) != 0 {
		t.Fatal("a refused job claims nothing")
	}
}

// TestAssignClearProbesManyNearestFirst: with more cells than probes the
// nearest still wins, across chunks.
func TestAssignClearProbesManyNearestFirst(t *testing.T) {
	t.Parallel()
	f := newFake(128, 128)
	f.px, f.py = 60, 60
	l, _ := layer(f)
	l.PaintClear(0, 0, 127, 127, true)
	for y := 0; y < 128; y += 3 {
		for x := 0; x < 128; x += 3 {
			l.SetCell(y*128+x, 90)
		}
	}
	l.SetCell(61*128+61, 90)
	if !l.AssignClear(1) || f.jobs[0].DestX != 60 || f.jobs[0].DestY != 60 {
		t.Fatalf("jobs %+v", f.jobs)
	}
}

func TestDoClearLifecycle(t *testing.T) {
	t.Parallel()
	f := newFake(64, 64)
	l, _ := layer(f)
	clearable(f, l, [2]int{2, 0})
	j := job.Record{Kind: job.ClearSnow, State: job.Do, DestX: 2, DestY: 0, WorkLeft: 15, Count: 15}
	l.DoClear(1, j)
	if got := f.jobs[0]; got.State != job.Do || got.WorkLeft != 5 || l.At(2, 0) == 0 {
		t.Fatalf("one tick of work %+v", got)
	}
	f.work = 0
	l.DoClear(1, f.jobs[0])
	if got := f.jobs[1]; got.State != job.Do || got.WorkLeft != 5 {
		t.Fatalf("no labour %+v", got)
	}
	f.work = 10
	l.DoClear(1, f.jobs[1])
	if got := f.jobs[2]; got.State != job.Complete || l.At(2, 0) != 0 {
		t.Fatalf("finished %+v, depth %d", got, l.At(2, 0))
	}
}

func TestDoClearEnds(t *testing.T) {
	t.Parallel()
	f := newFake(64, 64)
	l, _ := layer(f)
	clearable(f, l, [2]int{2, 0}, [2]int{4, 0})
	l.SetCell(2, 0)
	cases := []struct {
		name  string
		j     job.Record
		state job.State
	}{
		{"off the map", job.Record{State: job.Do, DestX: -1, WorkLeft: 5, Count: 5}, job.Fail},
		{"outside the area", job.Record{State: job.Do, DestX: 9, WorkLeft: 5, Count: 5}, job.Fail},
		{"melted before it started", job.Record{State: job.Do, DestX: 2, WorkLeft: 5, Count: 5}, job.Fail},
		{"melted while shovelling", job.Record{State: job.Do, DestX: 2, WorkLeft: 3, Count: 5}, job.Complete},
	}
	for _, c := range cases {
		f.jobs = nil
		l.DoClear(1, c.j)
		if got := f.jobs[0]; got.State != c.state {
			t.Fatalf("%s: state %v, want %v", c.name, got.State, c.state)
		}
	}
}

func TestViewSharedUntilChanged(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	l, _ := layer(f)
	if v := l.View(); v.Snow != nil || v.Clear != nil || v.SnowVer != 0 || v.ClearVer != 0 {
		t.Fatalf("no snow, no area: %+v", v)
	}
	l.SetCell(8*f.w+8, 90)
	l.PaintClear(1, 1, 2, 3, true)
	v := l.View()
	if v.Snow[8*f.w+8] != 90 || v.Clear[3*f.w+2] != 1 || v.Clear[3*f.w+3] != 0 {
		t.Fatal("view per cell")
	}
	u := l.View()
	if &u.Snow[0] != &v.Snow[0] || &u.Clear[0] != &v.Clear[0] || u.SnowVer != v.SnowVer || u.ClearVer != v.ClearVer {
		t.Fatal("an unchanged view is shared, not copied")
	}
	l.SetCell(8*f.w+8, 91)
	if w := l.View(); w.SnowVer == v.SnowVer || v.Snow[8*f.w+8] != 90 {
		t.Fatal("a change makes a new view and leaves the old one alone")
	}
}

func TestExportImport(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	l, s := layer(f)
	l.PaintClear(3, 3, 6, 5, true)
	l.SetCell(4*f.w+4, 200)
	v := l.View()
	depth, clear := l.Export()

	m, ms := layer(f)
	m.Import(depth, clear)
	if !slices.Equal(ms.depth, s.depth) || !slices.Equal(ms.clear, s.clear) || ms.sum != s.sum || ms.nz != s.nz {
		t.Fatal("a round trip changes the layer")
	}
	if !slices.Equal(ms.clearN, s.clearN) || !slices.Equal(ms.chunkSum, s.chunkSum) {
		t.Fatal("a round trip changes the derived counts")
	}
	if w := m.View(); w.SnowVer != v.SnowVer || w.ClearVer != v.ClearVer {
		t.Fatal("a loaded game reads the versions it was saved with")
	}

	m.Import(depth[:3], clear[:1]) // an older or damaged save
	if ms.depth != nil || ms.clear != nil {
		t.Fatal("malformed layers load as none")
	}
	if d, c := m.Export(); d != nil || c != nil {
		t.Fatal("none exports as none")
	}
}
