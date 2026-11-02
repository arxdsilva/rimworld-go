package industry

import (
	"testing"

	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

// fakeHost is a small map for the industry's rules.
type fakeHost struct {
	w, h    int
	terr    terrain.Map
	bldgs   map[entity.EntityID]construction.Building
	ids     []entity.EntityID
	sv      uint64
	off     map[entity.EntityID]bool
	drops   map[item.Kind]int
	letters []string
	started []job.Kind
	seed    uint64
	unit    int
}

func newFake(w, h int) *fakeHost {
	return &fakeHost{w: w, h: h, terr: terrain.NewMap(w, h), bldgs: map[entity.EntityID]construction.Building{},
		off: map[entity.EntityID]bool{}, drops: map[item.Kind]int{}, seed: 42, unit: 1}
}

func (f *fakeHost) add(id entity.EntityID, b construction.Building) {
	f.bldgs[id] = b
	f.ids = append(f.ids, id)
	f.sv++
}

func (f *fakeHost) StructVer() uint64              { return f.sv }
func (f *fakeHost) BuildingIDs() []entity.EntityID { return f.ids }
func (f *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	return b, ok
}
func (f *fakeHost) Powered(id entity.EntityID) bool { return !f.off[id] }
func (f *fakeHost) Size() (int, int)                { return f.w, f.h }
func (f *fakeHost) Terrain(x, y int) terrain.Type   { return f.terr.At(x, y) }
func (f *fakeHost) SetTerrain(x, y int, t terrain.Type) {
	f.terr.Set(x, y, t)
	f.sv++
}
func (f *fakeHost) Ocean(x, y int) bool                                             { return false }
func (f *fakeHost) StoneAt(x, y int) geology.Stone                                  { return geology.Granite }
func (f *fakeHost) Pos(id entity.EntityID) (int, int)                               { return 0, 0 }
func (f *fakeHost) Free(worker, bench entity.EntityID, k job.Kind, sx, sy int) bool { return true }
func (f *fakeHost) Start(worker, bench entity.EntityID, k job.Kind, sx, sy, ticks int) bool {
	f.started = append(f.started, k)
	return true
}
func (f *fakeHost) Work(id entity.EntityID, mining bool) int { return f.unit }
func (f *fakeHost) Seed() uint64                             { return f.seed }
func (f *fakeHost) Drop(k item.Kind, n, x, y int)            { f.drops[k] += n }
func (f *fakeHost) Letter(title, text string, x, y int)      { f.letters = append(f.letters, title) }
func (f *fakeHost) Changed()                                 {}

// A find lays a lump of the wiki's size for its ore, the same for the
// same seed, and a deep drill brings it up portion by portion until the
// deposit is exhausted, then stone chunks.
func TestFindAndDrill(t *testing.T) {
	f := newFake(40, 40)
	var s State
	if !s.find(f) || len(s.ore) == 0 || len(f.letters) != 1 {
		t.Fatalf("find: %d cells, letters %v", len(s.ore), f.letters)
	}
	kind := item.Kind(s.ore[0].Ore)
	var d oreDef
	for _, od := range oreDefs {
		if od.kind == kind {
			d = od
		}
	}
	if n := len(s.ore); n < d.minSize || n > d.maxSz {
		t.Fatalf("%s lump of %d cells, wiki %d-%d", kind, n, d.minSize, d.maxSz)
	}
	var again State
	again.find(f)
	if len(again.ore) != len(s.ore) || again.ore[0] != s.ore[0] {
		t.Fatal("the same seed finds the same lump")
	}
	// a drill on the lump's first cell
	c := s.ore[0]
	x, y := int(c.I)%40, int(c.I)/40
	f.add(1, construction.Building{Kind: construction.DeepDrill, X: x, Y: y})
	ops := s.Ops(f)
	if ops.DrillOre(f.bldgs[1]) != kind.String() {
		t.Fatalf("drill ore %q", ops.DrillOre(f.bldgs[1]))
	}
	f.unit = DrillCycle // one cycle a tick
	for range 10000 {
		ops.Do(7, 1)
		if ops.DrillOre(f.bldgs[1]) == "" {
			break
		}
	}
	if ops.DrillOre(f.bldgs[1]) != "" || f.drops[kind] == 0 || f.letters[len(f.letters)-1] != "Deep ore exhausted" {
		t.Fatalf("drill: %d %s brought up, letters %v", f.drops[kind], kind, f.letters)
	}
	ops.Do(7, 1)
	if f.drops[item.GraniteChunk] != 1 {
		t.Fatalf("an exhausted drill brings up chunks: %v", f.drops)
	}
	f.off[1] = true
	if ops.Do(7, 1) != Blocked {
		t.Fatal("no power: blocked")
	}
}

// A scanner finds within ScanMax work and its cycle shows progress.
func TestScannerFinds(t *testing.T) {
	f := newFake(30, 30)
	var s State
	f.add(2, construction.Building{Kind: construction.GroundScanner, X: 3, Y: 3})
	ops := s.Ops(f)
	f.unit = 1000
	for i := 0; i <= ScanMax/1000 && len(s.ore) == 0; i++ {
		ops.Do(7, 2)
	}
	if len(s.ore) == 0 || s.finds != 1 {
		t.Fatalf("no find within %d work", ScanMax)
	}
	if !ops.Assign(7, false, 3) || f.started[0] != job.ScanGround {
		t.Fatalf("assign: %v", f.started)
	}
	var v axview.Building
	s.View(f, &v, 2, f.bldgs[2])
	if v.Cycle < 0 || v.Cycle >= 1000 {
		t.Fatalf("cycle %d", v.Cycle)
	}
	ver, dep := s.DeepOre(30)
	if ver == 0 || len(dep) != len(s.ore) {
		t.Fatal("the world view lists the found ore")
	}
	if _, again := s.DeepOre(30); &again[0] != &dep[0] {
		t.Fatal("unchanged ore hands out the same slice")
	}
}

// A powered moisture pump dries marsh, mud and shallow water within its
// growing radius; moving river water stays; unpowered it stops.
func TestPumpDries(t *testing.T) {
	f := newFake(20, 20)
	for y := range 20 {
		for x := range 20 {
			f.terr.Set(x, y, terrain.Marsh)
		}
	}
	f.terr.Set(10, 11, terrain.ShallowWater)
	f.terr.Set(11, 10, terrain.Mud)
	f.terr.Set(9, 10, terrain.ShallowWater)
	var s State
	flow := make([]uint8, 400)
	flow[10*20+9] = 3 // flowing east
	s.SetFlow(flow, 20, 20)
	f.add(3, construction.Building{Kind: construction.MoisturePump, X: 10, Y: 10})
	p := s.Pumps(f)
	p.Advance(PumpTileTicks) // radius 1
	if f.terr.At(10, 11) != terrain.Gravel || f.terr.At(11, 10) != terrain.Soil || f.terr.At(9, 10) != terrain.ShallowWater {
		t.Fatalf("radius 1: %v %v %v", f.terr.At(10, 11), f.terr.At(11, 10), f.terr.At(9, 10))
	}
	if f.terr.At(12, 10) != terrain.Marsh || p.Radius(3) != 1 {
		t.Fatalf("beyond the radius stays wet; radius %d", p.Radius(3))
	}
	f.off[3] = true
	p.Advance(10 * PumpTileTicks)
	if p.Radius(3) != 1 {
		t.Fatal("an unpowered pump stops")
	}
	f.off[3] = false
	for range 70 {
		p.Advance(PumpTileTicks)
	}
	if p.Radius(3) != 6 || f.terr.At(16, 10) != terrain.Soil || f.terr.At(17, 10) != terrain.Marsh {
		t.Fatalf("full radius 6.9: %d, %v %v", p.Radius(3), f.terr.At(16, 10), f.terr.At(17, 10))
	}
}

// A watermill's wheel must sit in moving water; it makes the wiki's
// 1100 W, 330 W with another in its exclusion area.
func TestWatermill(t *testing.T) {
	f := newFake(40, 20)
	flow := make([]uint8, 800)
	for x := range 40 {
		for _, y := range []int{10, 11} {
			f.terr.Set(x, y, terrain.ShallowWater)
			flow[y*40+x] = 3
		}
	}
	var s State
	s.SetFlow(flow, 40, 20)
	m := s.Mill(f)
	if ok, _ := m.Place(construction.Watermill, construction.South, 2, 5); !ok { // wheel on row 10
		t.Fatal("wheel in the river: placeable")
	}
	if ok, r := m.Place(construction.Watermill, construction.South, 2, 3); ok || r != ReasonNoRiver {
		t.Fatal("wheel on dry ground: refused")
	}
	f.add(1, construction.Building{Kind: construction.Watermill, Facing: construction.South, X: 2, Y: 5})
	if w := m.Output(1); w != MillW {
		t.Fatalf("alone: %d W", w)
	}
	f.add(2, construction.Building{Kind: construction.Watermill, Facing: construction.South, X: 10, Y: 5})
	if w := m.Output(1); w != MillCrowdedW {
		t.Fatalf("crowded: %d W", w)
	}
	f.add(3, construction.Building{Kind: construction.Watermill, Facing: construction.South, X: 30, Y: 5})
	if w := m.Output(3); w != MillW {
		t.Fatalf("far apart: %d W", w)
	}
	var v axview.Building
	s.View(f, &v, 3, f.bldgs[3])
	if v.Flow != 1000 {
		t.Fatalf("flow %d", v.Flow)
	}
}

// Save round trip keeps the flow, the ore and the progress of standing
// machines only.
func TestSaveRoundTrip(t *testing.T) {
	f := newFake(30, 30)
	var s State
	s.SetFlow(make([]uint8, 900), 30, 30) // still: no river
	s.find(f)
	s.setWork(5, 77)
	s.setWork(6, 88)
	v := s.Export(func(id entity.EntityID) bool { return id == 5 })
	var r State
	r.Import(v)
	if len(r.ore) != len(s.ore) || r.finds != 1 || r.work[5] != 77 || r.work[6] != 0 || r.flow != nil {
		t.Fatalf("round trip: %+v", v)
	}
}
