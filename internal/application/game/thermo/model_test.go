package thermo

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
)

// fake is a test host: a map of rooms and walls, buildings and switches.
type fake struct {
	w, h    int
	grid    []int
	roofs   []roof.Cell
	out     temperature.Temp
	ids     []entity.EntityID
	bldgs   map[entity.EntityID]construction.Building
	on      map[entity.EntityID]bool // burning, powered, open, worked: one switch per building
	target  map[entity.EntityID]int
	active  map[entity.EntityID]bool
	bodies  []Body
	fires   []Fire
	sv, rvr uint64
}

func (f *fake) RoomGrid() ([]int, int, int)    { return f.grid, f.w, f.h }
func (f *fake) Roof(i int) roof.Cell           { return f.roofs[i] }
func (f *fake) RoofVer() uint64                { return f.rvr }
func (f *fake) Outdoor() temperature.Temp      { return f.out }
func (f *fake) StructVer() uint64              { return f.sv }
func (f *fake) BuildingIDs() []entity.EntityID { return f.ids }
func (f *fake) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	return b, ok
}
func (f *fake) Burning(id entity.EntityID) bool { return f.on[id] }
func (f *fake) Powered(id entity.EntityID) bool { return f.on[id] }
func (f *fake) Open(id entity.EntityID) bool    { return f.on[id] }
func (f *fake) Worked(id entity.EntityID) bool  { return f.on[id] }
func (f *fake) Thermostat(id entity.EntityID) (int, bool) {
	t, ok := f.target[id]
	if !ok {
		t = 210
	}
	return t, f.active[id]
}
func (f *fake) SetThermoActive(id entity.EntityID, on bool) { f.active[id] = on }
func (f *fake) AppendBodies(dst []Body) []Body              { return append(dst, f.bodies...) }
func (f *fake) AppendFires(dst []Fire) []Fire               { return append(dst, f.fires...) }

// newFake is a w x h map with roofed rooms: each rect (x0, y0, n) an n x n
// interior ringed by one wall, room ids in order.
func newFake(w, h int, rects ...[3]int) *fake {
	f := &fake{w: w, h: h, grid: make([]int, w*h), roofs: make([]roof.Cell, w*h),
		bldgs: map[entity.EntityID]construction.Building{}, on: map[entity.EntityID]bool{},
		target: map[entity.EntityID]int{}, active: map[entity.EntityID]bool{}}
	for i := range f.grid {
		f.grid[i] = room.Outdoors
	}
	for r, c := range rects {
		for y := c[1] - 1; y <= c[1]+c[2]; y++ {
			for x := c[0] - 1; x <= c[0]+c[2]; x++ {
				i := y*w + x
				f.roofs[i] = roof.Built
				if x < c[0] || y < c[1] || x >= c[0]+c[2] || y >= c[1]+c[2] {
					f.grid[i] = room.Wall
				} else {
					f.grid[i] = r
				}
			}
		}
	}
	return f
}

// add places a building (switched on).
func (f *fake) add(k construction.Kind, facing construction.Facing, x, y int) entity.EntityID {
	id := entity.EntityID(len(f.ids) + 1)
	f.ids = append(f.ids, id)
	f.bldgs[id] = construction.Building{Kind: k, Facing: facing, X: x, Y: y}
	f.on[id] = true
	f.sv++
	return id
}

// model is a model over f with every room at t.
func model(f *fake, t temperature.Temp, rooms int) *Model {
	m := New(f)
	m.Remap(nil, f.grid, rooms, f.out, t)
	return m
}

func steps(m *Model, n int) (lo, hi temperature.Temp) {
	lo, hi = 1<<20, -1<<20
	for range n {
		m.Step()
		lo, hi = min(lo, m.Temps[0]), max(hi, m.Temps[0])
	}
	return lo, hi
}

// TestPassiveCoolerStopsAt17: wiki: Passive cooler, it cools down to
// 17 C and no further, and holds there without bouncing (no-flap).
func TestPassiveCoolerStopsAt17(t *testing.T) {
	f := newFake(12, 12, [3]int{3, 3, 5})
	f.out = 300
	pc := f.add(construction.PassiveCooler, construction.South, 4, 4)
	m := model(f, 300, 1)
	steps(m, 500) // settle
	lo, hi := steps(m, 500)
	if lo != 170 || hi != 170 {
		t.Fatalf("passive cooler room %v..%v at 30 C outside, want held at 17 C", lo, hi)
	}
	if r := m.BuildingRate(pc); r != -110 {
		t.Fatalf("passive cooler shows %d tenths of heat/s, want -110", r)
	}
	f.on[pc] = false // out of wood
	if _, hi := steps(m, 300); hi <= 200 {
		t.Fatalf("an empty passive cooler still cools: %v", hi)
	}
}

// TestCampfireAndTorchCaps: wiki: Campfire heats up to 28 C, Torch lamp
// up to 23 C, whatever the outside.
func TestCampfireAndTorchCaps(t *testing.T) {
	for _, c := range []struct {
		k   construction.Kind
		cap temperature.Temp
	}{{construction.Campfire, 280}, {construction.TorchLamp, 230}} {
		f := newFake(12, 12, [3]int{3, 3, 3})
		f.out = 150
		f.add(c.k, construction.South, 4, 4)
		m := model(f, 150, 1)
		steps(m, 800)
		if lo, hi := steps(m, 200); lo != c.cap || hi != c.cap {
			t.Fatalf("%s room %v..%v, want held at %v", c.k, lo, hi, c.cap)
		}
	}
}

// TestBenchHeatsOnlyWhileWorked: a fueled stove (wiki 4 heat/s) adds heat
// only while someone works at it.
func TestBenchHeatsOnlyWhileWorked(t *testing.T) {
	f := newFake(12, 12, [3]int{3, 3, 4})
	stove := f.add(construction.Stove, construction.South, 4, 4)
	m := model(f, 0, 1)
	if m.BuildingRate(stove) != 40 {
		t.Fatalf("a worked stove shows %d, want 40", m.BuildingRate(stove))
	}
	steps(m, 50)
	warm := m.Temps[0]
	f.on[stove] = false
	if m.BuildingRate(stove) != 0 {
		t.Fatal("an idle stove shows heat")
	}
	steps(m, 300)
	if m.Temps[0] >= warm || warm <= 0 {
		t.Fatalf("worked %v, idle after %v", warm, m.Temps[0])
	}
}

// TestBodyHeatUpTo40: wiki: Temperature, a humanlike pushes 0.3 heat/s
// per body size, another creature 60 % of that, only at 40 C or below.
func TestBodyHeatUpTo40(t *testing.T) {
	if bodyHeat(Body{Size: 1000, Human: true}) != 30 || bodyHeat(Body{Size: 1000}) != 18 {
		t.Fatal("body heat rates")
	}
	f := newFake(12, 12, [3]int{3, 3, 3})
	for range 4 {
		f.bodies = append(f.bodies, Body{X: 4, Y: 4, Size: 1000, Human: true})
	}
	m := model(f, 0, 1)
	if r := m.RoomRates()[0]; r != 12 {
		t.Fatalf("four people: %d tenths of heat/s, want 12", r)
	}
	steps(m, 30)
	if m.Temps[0] <= 0 {
		t.Fatal("bodies warm a room")
	}
	m.Temps[0] = 410
	m.sync()
	m.roomHeat()
	if m.sum[0] != 0 {
		t.Fatalf("at 41 C bodies push %d", m.sum[0])
	}
}

// TestCoolerHotSideHeats: a cooler between two rooms takes heat from the
// cold side and gives the hot side more (the pumped heat and the motor's).
func TestCoolerHotSideHeats(t *testing.T) {
	f := newFake(20, 10, [3]int{2, 2, 5}, [3]int{9, 2, 5})
	// share the wall at x 8 (rooms 0: 2..6, 1: 9..13; walls at 7 and 8):
	// put the cooler in a one-cell wall at x 7 by making 8 room 1's
	for y := 2; y <= 6; y++ {
		f.grid[y*f.w+8] = 1
	}
	f.grid[4*f.w+7] = room.Wall
	cooler := f.add(construction.Cooler, construction.West, 7, 4) // cold side west: room 0
	f.target[cooler] = -500
	f.out = 200
	m := model(f, 200, 2)
	m.Step()
	cold, hot := m.fine[0]-temperature.Temp(200).Milli(), m.fine[1]-temperature.Temp(200).Milli()
	if !(cold < 0 && hot > 0) {
		t.Fatalf("cold side moved %d, hot side %d", cold, hot)
	}
	// the hot side has 6 more cells: per cell it gets less, in all more
	if got, want := int(hot)*30, -int(cold)*25*coolerHotPct/100; got < want*9/10 || got > want*11/10 {
		t.Fatalf("hot side gained %d degree-milli-cells, want about %d", got, want)
	}
	if r := m.RoomRates(); r[0] >= 0 || r[1] <= 0 {
		t.Fatalf("room rates %v", r)
	}
}

// TestRemapKeepsAirAndResidual: a rebuild that changes nothing keeps the
// air and the view version; a breach leaves residual air behind.
func TestRemapKeepsAirAndResidual(t *testing.T) {
	f := newFake(12, 12, [3]int{3, 3, 3})
	m := model(f, 150, 1)
	m.Step()
	ver, air := m.Ver(), m.fine[0]
	m.Remap(f.grid, f.grid, 1, f.out, 0)
	if m.Ver() != ver || m.fine[0] != air {
		t.Fatalf("an identical rebuild moved ver %d -> %d, air %d -> %d", ver, m.Ver(), air, m.fine[0])
	}
	open := append([]int(nil), f.grid...)
	for i, r := range open {
		if r == 0 {
			open[i] = room.Outdoors
		}
	}
	m.Remap(f.grid, open, 0, f.out, 0)
	if len(m.Residual) != 9 || m.Ver() == ver {
		t.Fatalf("breach: %d residual cells, ver %d", len(m.Residual), m.Ver())
	}
	s := m.Export()
	n := New(f)
	n.Import(m.Temps, m.Residual, s)
	if n.Ver() != m.Ver() {
		t.Fatal("import keeps the view version")
	}
}

// TestDoorAndVentExchange: an open door to the outside pulls a room
// toward the outside air; a vent mixes two rooms faster than a door would
// and never past their common air; shut, neither moves anything.
func TestDoorAndVentExchange(t *testing.T) {
	f := newFake(20, 10, [3]int{2, 2, 5}, [3]int{9, 2, 5})
	for y := 2; y <= 6; y++ { // one shared wall at x 7: room 1 takes x 8
		f.grid[y*f.w+8] = 1
	}
	door := f.add(construction.Door, construction.South, 4, 7) // room 0's south wall, outdoors below
	vent := f.add(construction.Vent, construction.East, 7, 4)
	f.out = -200
	m := model(f, 200, 2)
	m.ensure()
	if len(m.c.links) != 2 {
		t.Fatalf("links %+v", m.c.links)
	}
	f.on[vent] = false
	m.set(0, 20000)
	m.set(1, 20000)
	m.Step()
	shutDoor := m.fine[0]
	m.set(0, 20000)
	m.set(1, 20000)
	f.on[door] = false
	m.Step()
	if !(shutDoor < m.fine[0]) {
		t.Fatalf("open door %d, shut %d", shutDoor, m.fine[0])
	}
	// the vent alone: a hot and a cold room even out, never crossing
	f.on[vent] = true
	f.out = 0
	m.set(0, 30000)
	m.set(1, 0)
	for range 30 {
		m.Step()
		if m.fine[0] < m.fine[1] {
			t.Fatalf("vented rooms crossed: %d < %d", m.fine[0], m.fine[1])
		}
	}
	if d := m.fine[0] - m.fine[1]; d > 500 {
		t.Fatalf("an open vent left %d between the rooms", d)
	}
}

// TestHeaterThermostatInModel: a heater works below target -1 C, pushes
// up to its target only, and idles once the room is past target +0.5 C.
func TestHeaterThermostatInModel(t *testing.T) {
	f := newFake(12, 12, [3]int{3, 3, 5})
	f.out = -100
	h := f.add(construction.Heater, construction.South, 4, 4)
	m := model(f, -100, 1)
	lo, hi := steps(m, 400)
	if !f.active[h] || hi != 210 || lo > 0 {
		t.Fatalf("heater active %v, room %v..%v", f.active[h], lo, hi)
	}
	if m.BuildingRate(h) != 210 || m.RoomRates()[0] != 210 {
		t.Fatalf("rates %d %v", m.BuildingRate(h), m.RoomRates())
	}
	f.out = 300 // summer: the room passes the band, the heater idles
	steps(m, 400)
	if f.active[h] || m.BuildingRate(h) != 0 {
		t.Fatalf("a heater in a 30 C room works: %v", f.active[h])
	}
}

// TestRoomRoofKinds: the stats line names the roof.
func TestRoomRoofKinds(t *testing.T) {
	f := newFake(20, 10, [3]int{2, 2, 3}, [3]int{9, 2, 3})
	for x := 2; x <= 4; x++ {
		f.roofs[2*f.w+x] = roof.Natural | roof.NaturalThick
	}
	for y := 2; y <= 4; y++ {
		for x := 9; x <= 11; x++ {
			f.roofs[y*f.w+x] = roof.Natural | roof.NaturalThick
		}
	}
	m := model(f, 0, 2)
	if a, b := m.RoomOf(0), m.RoomOf(1); a.RoofKind != "mixed" || b.RoofKind != "thick" || a.ExposedWalls != 12 {
		t.Fatalf("roof kinds %q %q, walls %d", a.RoofKind, b.RoofKind, a.ExposedWalls)
	}
	if (m.RoomOf(5) != Room{}) {
		t.Fatal("no such room")
	}
}

// changes is a fake incremental rebuild: nothing changed, room r same as r.
type changes struct{ cells [][]int }

func (c changes) EachChanged(func(int)) {}
func (c changes) Same(r int) int        { return r }
func (c changes) EachCell(r int, fn func(int)) {
	for _, i := range c.cells[r] {
		fn(i)
	}
}

// TestRemapChangedMatchesRemap: an incremental rebuild that changes
// nothing keeps every room's air exactly; a save restores the working
// precision, an older save (no fine values) the tenths.
func TestRemapChangedMatchesRemap(t *testing.T) {
	f := newFake(12, 12, [3]int{3, 3, 3})
	f.add(construction.Campfire, construction.South, 4, 4)
	m := model(f, 0, 1)
	steps(m, 7)
	air := m.fine[0]
	m.RemapChanged(changes{}, f.grid, f.grid, 1, f.out)
	if m.fine[0] != air {
		t.Fatalf("incremental rebuild moved the air %d -> %d", air, m.fine[0])
	}
	n := New(f)
	n.Import(append([]temperature.Temp(nil), m.Temps...), nil, Save{})
	if n.fine[0] != m.Temps[0].Milli() {
		t.Fatal("an older save takes the tenths")
	}
	n.Import(m.Temps, nil, m.Export())
	if n.fine[0] != air {
		t.Fatal("a save keeps the working precision")
	}
}
