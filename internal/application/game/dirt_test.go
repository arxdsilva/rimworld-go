package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/stain"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// dirtGame is a dry, bare-soil map (no grass) at a clear midday.
func dirtGame(t testing.TB) *Game {
	t.Helper()
	g := New(Config{Seed: 3, Width: 30, Height: 14})
	jumpTo(g, 10, 12)
	g.SetWeather(weather.Clear, 0)
	g.weather.Wetness = 0
	s := g.seedGrass()
	clear(s.cells)
	return g
}

// roofOver puts a built roof over the rectangle (indoors without walls).
func roofOver(g *Game, x0, y0, x1, y1 int) {
	g.ensureRoofGrid()
	w := g.grid.Width()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			g.setRoof(y*w+x, roof.Natural)
		}
	}
}

func dirtAt(g *Game, x, y int) int {
	i := y*g.grid.Width() + x
	if i >= len(g.filth.cells) {
		return 0
	}
	return int(g.filth.cells[i])
}

func carried(g *Game, id entity.EntityID) int { return int(g.filth.carried[id]) }

func setCarried(g *Game, id entity.EntityID, c int) {
	if g.filth.carried == nil {
		g.filth.carried = map[entity.EntityID]uint16{}
	}
	g.filth.carried[id] = uint16(c)
}

// walkRow steps id east along row y from x0 to x1 through dirtStep.
func walkRow(g *Game, id entity.EntityID, x0, x1, y int) {
	for x := x0 + 1; x <= x1; x++ {
		g.positions.Set(id, Position{X: x, Y: y})
		g.dirtStep(id, Position{X: x, Y: y})
	}
}

func TestColonistTracksDirtIndoors(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	scheduleAll(g, schedule.Work)
	roofOver(g, 15, 3, 28, 8)
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	g.setDraft(id, true) // stays where it is sent
	g.cmdMove(id, 26, 5)
	g.Step(TicksPerCell * 60)
	if p, _ := g.positions.Get(id); p != (Position{26, 5}) {
		t.Fatalf("pawn at %+v, want 26,5", p)
	}
	w := g.World()
	if len(w.Dirt) == 0 {
		t.Fatal("walking in from the soil left no dirt indoors")
	}
	for k, d := range w.Dirt {
		if d.X < 15 {
			t.Fatalf("dirt outdoors at %d,%d", d.X, d.Y)
		}
		if k > 0 && (d.Y < w.Dirt[k-1].Y || d.Y == w.Dirt[k-1].Y && d.X <= w.Dirt[k-1].X) {
			t.Fatalf("dirt view not sorted: %+v", w.Dirt)
		}
	}
	// the trail fades: the doorway gets more than deep inside
	if dirtAt(g, 15, 5) <= dirtAt(g, 25, 5) {
		t.Fatalf("trail %d at the entrance, %d inside", dirtAt(g, 15, 5), dirtAt(g, 25, 5))
	}
	// what was dropped came off the boots
	h := dirtGame(t)
	hid, _ := h.SpawnNamedPawn(2, 5, "Bo")
	walkRow(h, hid, 2, 14, 5)
	before := carried(h, hid)
	if before != 12*filth.StepGain {
		t.Fatalf("12 soil steps carried %d, want %d", before, 12*filth.StepGain)
	}
	roofOver(h, 15, 3, 28, 8)
	walkRow(h, hid, 14, 26, 5)
	sum := 0
	for x := 15; x <= 26; x++ {
		sum += dirtAt(h, x, 5)
	}
	if sum == 0 || sum+carried(h, hid) != before {
		t.Fatalf("dropped %d, kept %d, carried %d", sum, carried(h, hid), before)
	}
	if pv := pawn(t, h, hid); pv.Dirt != carried(h, hid)*100/filth.Full {
		t.Fatalf("PawnView.Dirt = %d, want %d", pv.Dirt, carried(h, hid)*100/filth.Full)
	}
}

func TestNoPickupOnFloorOrStone(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	for x := 3; x <= 8; x++ {
		g.setTerrain(x, 5, terrain.Stone)
	}
	for x := 9; x <= 12; x++ {
		placeBuilt(t, g, construction.Floor, construction.South, x, 5)
	}
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	walkRow(g, id, 2, 12, 5)
	if c := carried(g, id); c != 0 {
		t.Fatalf("stone and floors gave %d dirt", c)
	}
	// clean ground sheds dirt already on the boots
	setCarried(g, id, 100)
	walkRow(g, id, 2, 3, 5)
	if c := carried(g, id); c != filth.Shed(100) {
		t.Fatalf("one stone step left %d, want %d", c, filth.Shed(100))
	}
	setCarried(g, id, 100)
	walkRow(g, id, 9, 10, 5)
	if c := carried(g, id); c != filth.Shed(100) {
		t.Fatalf("one floor step left %d, want %d", c, filth.Shed(100))
	}
	// grass halves the pickup
	g.grass.cells[5*g.grid.Width()+14] = 1
	setCarried(g, id, 0)
	walkRow(g, id, 13, 14, 5)
	if c := carried(g, id); c != filth.Gain(filth.StepGain, 0, true) {
		t.Fatalf("a grass step carried %d", c)
	}
}

func TestRainMuddierBoots(t *testing.T) {
	t.Parallel()
	dry, wet := dirtGame(t), dirtGame(t)
	wet.SetWeather(weather.Rain, 1)
	wet.weather.Wetness = 1
	a, _ := dry.SpawnNamedPawn(2, 5, "Ada")
	b, _ := wet.SpawnNamedPawn(2, 5, "Ada")
	walkRow(dry, a, 2, 12, 5)
	walkRow(wet, b, 2, 12, 5)
	if carried(wet, b) != 3*carried(dry, a) || carried(dry, a) == 0 {
		t.Fatalf("rain carried %d, dry %d: want three times", carried(wet, b), carried(dry, a))
	}
}

func TestOutdoorWorkSoilsBoots(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	in, _ := g.SpawnNamedPawn(20, 5, "Bo")
	roofOver(g, 18, 3, 22, 8)
	work := 0
	for _, k := range []job.Kind{job.Sow, job.Harvest, job.Cut, job.Mine, job.Haul} {
		setCarried(g, id, 0)
		setCarried(g, in, 0)
		for now := uint64(0); now < 300; now++ {
			g.dirtWork(id, job.Record{Kind: k, State: job.Do}, now)
			g.dirtWork(in, job.Record{Kind: k, State: job.Do}, now)
		}
		if carried(g, in) != 0 {
			t.Fatalf("%v under a roof soiled the boots", k)
		}
		want := 10 * filth.WorkGain
		if k == job.Haul {
			want = 0
		}
		if c := carried(g, id); c != want {
			t.Fatalf("%v for 300 ticks carried %d, want %d", k, c, want)
		}
		work++
	}
	// digging a grave soils the boots, building a wall does not
	g.placeFrameStuff(construction.Grave, item.Wood, construction.South, 8, 8)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 10, 8)
	grave, wall := g.layerFrameAt(8, 8, construction.LayerObject), g.layerFrameAt(10, 8, construction.LayerObject)
	for _, c := range []struct {
		target entity.EntityID
		want   int
	}{{grave, 10 * filth.WorkGain}, {wall, 0}} {
		setCarried(g, id, 0)
		for now := uint64(0); now < 300; now++ {
			g.dirtWork(id, job.Record{Kind: job.Construct, State: job.Do, Target: c.target}, now)
		}
		if carried(g, id) != c.want {
			t.Fatalf("construct %d carried %d, want %d", c.target, carried(g, id), c.want)
		}
	}
	// the real loop: mining a rock beside soil dirties the miner
	h := dirtGame(t)
	scheduleAll(h, schedule.Work)
	h.setTerrain(10, 5, terrain.Rock)
	mid, _ := h.SpawnNamedPawn(9, 5, "Cy")
	h.Enqueue(command.Command{Kind: command.DesignateMine, X: 10, Y: 5, X2: 10, Y2: 5})
	h.Step(MineWork + 40)
	if h.terr.At(10, 5) == terrain.Rock {
		t.Fatal("the rock was not mined")
	}
	if c := carried(h, mid); c < (MineWork/filth.WorkEvery-1)*filth.WorkGain {
		t.Fatalf("mining carried only %d", c)
	}
}

func TestCarpetWipesBoots(t *testing.T) {
	t.Parallel()
	left := map[item.Kind]int{}
	for _, k := range item.Carpets() {
		g := dirtGame(t)
		g.placeFrameStuff(construction.Carpet, k, construction.South, 6, 5)
		fr, ok := g.frames.Get(g.layerFrameAt(6, 5, construction.LayerObject))
		if !ok {
			t.Fatalf("no %s carpet frame", k)
		}
		g.completeFrame(g.layerFrameAt(6, 5, construction.LayerObject), fr)
		id, _ := g.SpawnNamedPawn(5, 5, "Ada")
		setCarried(g, id, 500)
		walkRow(g, id, 5, 6, 5)
		keep, soil := filth.Wipe(500, filth.Catch(k))
		if carried(g, id) != keep || dirtAt(g, 6, 5) != soil || soil == 0 {
			t.Fatalf("%s: kept %d (want %d), carpet %d (want %d)", k, carried(g, id), keep, dirtAt(g, 6, 5), soil)
		}
		left[k] = keep
	}
	if !(left[item.CarpetCloth] < left[item.CarpetLeather] && left[item.CarpetLeather] < left[item.CarpetStraw] &&
		left[item.CarpetStraw] < left[item.CarpetWood]) {
		t.Fatalf("wipe order: %v", left)
	}
	// placing one from the Architect without a material gets the default carpet
	g := dirtGame(t)
	placeBuilt(t, g, construction.Carpet, construction.South, 3, 3)
	b, ok := g.bldgs.Get(g.layerBldgAt(3, 3, construction.LayerObject))
	if !ok || b.Stuff != construction.DefaultStuff(construction.Carpet) {
		t.Fatalf("carpet = %+v %v", b, ok)
	}
}

func TestIndoorDirtNeverFades(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	roofOver(g, 2, 2, 6, 6)
	in, out := 4*g.grid.Width()+4, 4*g.grid.Width()+20
	g.addDirt(in, 500)
	g.addDirt(out, 500)
	for range 50 { // dry days: nothing fades
		g.stainDecay()
	}
	if dirtAt(g, 4, 4) != 500 || dirtAt(g, 20, 4) != 500 {
		t.Fatalf("dry weather faded dirt: %d %d", dirtAt(g, 4, 4), dirtAt(g, 20, 4))
	}
	g.SetWeather(weather.Rain, 1)
	for range 50 {
		g.stainDecay()
	}
	if dirtAt(g, 4, 4) != 500 {
		t.Fatalf("rain washed indoor dirt to %d", dirtAt(g, 4, 4))
	}
	if dirtAt(g, 20, 4) != 0 || slices.Contains(g.filth.nz, int32(out)) {
		t.Fatalf("rain left %d outdoor dirt", dirtAt(g, 20, 4))
	}
}

func TestWheatExtraStraw(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	scheduleAll(g, schedule.Work)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	pid, err := g.spawnPlant(plant.Wheat, 6, 5, plant.DefOf(plant.Wheat).GrowTicks)
	if err != nil {
		t.Fatal(err)
	}
	g.jobs.Set(id, job.Record{Kind: job.Harvest, State: job.Do, Target: pid, TX: 6, TY: 5, WorkLeft: 1})
	g.doHarvest(id, job.Record{Kind: job.Harvest, State: job.Do, Target: pid, TX: 6, TY: 5, WorkLeft: 1})
	straw, ok := stackOf(g, item.Straw)
	if !ok || straw.Qty != plant.DefOf(plant.Wheat).ExtraQty {
		t.Fatalf("straw = %+v %v", straw, ok)
	}
	if w, ok := stackOf(g, item.Wheat); !ok || w.Qty != plant.DefOf(plant.Wheat).Yield {
		t.Fatalf("wheat = %+v %v", w, ok)
	}
}

func TestCottonYieldsCloth(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	pid, _ := g.spawnPlant(plant.Cotton, 6, 5, plant.DefOf(plant.Cotton).GrowTicks)
	g.doHarvest(id, job.Record{Kind: job.Harvest, State: job.Do, Target: pid, TX: 6, TY: 5, WorkLeft: 1})
	if c, ok := stackOf(g, item.Cloth); !ok || c.Qty != plant.DefOf(plant.Cotton).Yield {
		t.Fatalf("cloth = %+v %v", c, ok)
	}
	if _, ok := stackOf(g, item.Straw); ok {
		t.Fatal("cotton left straw")
	}
}

// filthColony is a colony with a roofed hall south of the landing spot so
// colonists track dirt in, and blood in it to clean.
func filthColony(t testing.TB) *Game {
	t.Helper()
	g := NewColony(Config{Seed: 9, Width: 48, Height: 40})
	cx, cy := g.grid.Width()/2, g.grid.Height()/2
	roofOver(g, cx-4, cy-2, cx+4, cy+2)
	g.stamp(cx, cy+1, stain.Blood, 600)
	g.SetWeather(weather.Rain, 0.6)
	g.weather.Wetness = 0.8
	return g
}

func filthSig(g *Game) any {
	dirt, car := g.exportDirt()
	return []any{dirt, car, append([]int32(nil), g.filth.cleanQ...), g.exportHome(), g.exportStains(), g.World().Rooms}
}

func TestFilthDeterministic(t *testing.T) {
	t.Parallel()
	a, b := filthColony(t), filthColony(t)
	a.Step(6000)
	b.Step(6000)
	if !reflect.DeepEqual(filthSig(a), filthSig(b)) {
		t.Fatal("same seed, different dirt")
	}
	if d, _ := a.exportDirt(); len(d) == 0 {
		t.Fatal("no dirt tracked in 6000 ticks")
	}
}

func TestFilthSaveContinuesSameSeed(t *testing.T) {
	t.Parallel()
	a := filthColony(t)
	a.Step(3000)
	var buf bytes.Buffer
	if err := a.Save(&buf, "dirt"); err != nil {
		t.Fatal(err)
	}
	b, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(filthSig(a), filthSig(b)) {
		t.Fatal("load changed the dirt")
	}
	a.Step(3000)
	b.Step(3000)
	if !reflect.DeepEqual(filthSig(a), filthSig(b)) {
		t.Fatal("a loaded game tracks dirt differently")
	}
}

// recountRoomSum is roomSum from scratch: dirt plus stains per room.
func recountRoomSum(g *Game) []int {
	g.ensureRooms()
	sum := make([]int, len(g.rooms))
	for i, r := range g.roomGrid {
		if r < 0 {
			continue
		}
		sum[r] += dirtAt(g, i%g.grid.Width(), i/g.grid.Width()) + int(g.stains.tiles[[2]int{i % g.grid.Width(), i / g.grid.Width()}].Amount)
	}
	return sum
}

// checkFilthIndex verifies the incremental indexes against the layers.
func checkFilthIndex(t *testing.T, g *Game, step int) {
	t.Helper()
	w := g.grid.Width()
	var nz []int32
	for i, c := range g.filth.cells {
		if c != 0 {
			nz = append(nz, int32(i))
		}
	}
	if !slices.Equal(nz, g.filth.nz) {
		t.Fatalf("step %d: nz %v, want %v", step, g.filth.nz, nz)
	}
	if !slices.IsSorted(g.filth.cleanQ) || len(slices.Compact(slices.Clone(g.filth.cleanQ))) != len(g.filth.cleanQ) {
		t.Fatalf("step %d: cleanQ not sorted and unique", step)
	}
	for _, i := range g.filth.cleanQ {
		if g.filthTotal(int(i)) == 0 {
			t.Fatalf("step %d: clean tile %d still queued", step, i)
		}
	}
	for i := 0; i < w*g.grid.Height(); i++ {
		if g.inHome(i) && g.filthTotal(i) >= filth.CleanStart {
			if _, ok := slices.BinarySearch(g.filth.cleanQ, int32(i)); !ok {
				t.Fatalf("step %d: dirty Home tile %d not queued", step, i)
			}
		}
	}
	if !g.roomDirty {
		if got := g.filth.roomSum; !slices.Equal(got, recountRoomSum(g)) {
			t.Fatalf("step %d: roomSum %v, recount %v", step, got, recountRoomSum(g))
		}
	}
}

func TestRoomFilthIncrementalMatchesRebuild(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	box(t, g, 2, 2, 9, 9, [2]int{5, 9})
	box(t, g, 14, 2, 20, 8, [2]int{17, 8})
	g.paintHome(0, 0, 12, 13, true)
	rng := rand.New(rand.NewPCG(1, 2))
	w, h := g.grid.Width(), g.grid.Height()
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	for step := range 3000 {
		x, y := rng.IntN(w), rng.IntN(h)
		i := y*w + x
		switch rng.IntN(9) {
		case 0, 1:
			g.addDirt(i, rng.IntN(300))
		case 2:
			g.addDirt(i, -rng.IntN(300))
		case 3:
			g.stamp(x, y, stain.Blood, rng.IntN(400))
		case 4:
			d, s := filth.CleanStep(dirtAt(g, x, y), int(g.stains.tiles[[2]int{x, y}].Amount))
			g.addDirt(i, d-dirtAt(g, x, y))
			g.setStainAmount(x, y, s)
		case 5:
			if rng.IntN(20) == 0 {
				g.SetWeather(weather.Rain, rng.Float64())
				g.stainDecay()
				g.SetWeather(weather.Clear, 0)
			}
		case 6:
			setCarried(g, id, rng.IntN(filth.Full))
			g.positions.Set(id, Position{X: x, Y: y})
			g.dirtStep(id, Position{X: x, Y: y})
		case 7:
			if rng.IntN(30) == 0 { // a wall goes up or comes down: rooms rebuild
				if b := g.layerBldgAt(x, y, construction.LayerObject); b != 0 {
					g.removeBuilding(b)
				} else if g.grid.Walkable(x, y) {
					g.BuildNow("wall", "wood", "south", x, y)
				}
			}
		case 8:
			g.paintHome(x, y, x+rng.IntN(4), y+rng.IntN(4), rng.IntN(2) == 0)
		}
		if rng.IntN(5) == 0 {
			g.ensureRooms()
		}
		checkFilthIndex(t, g, step)
	}
}

func TestOldSaveSeedsHomeEnablesClean(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	placeBuilt(t, g, construction.Bed, construction.South, 20, 6)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.setPriority(id, work.Clean, 0) // before cleaning existed
	g.addDirt(6*g.grid.Width()+20, 300)
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	old := downgradeSave(t, buf.Bytes(), func(s *saveState) { s.Home, s.CleanWired, s.CleanQ = nil, false, nil })
	l, _, err := Load(bytes.NewReader(old))
	if err != nil {
		t.Fatal(err)
	}
	if l.prioOf(id)[work.Clean] == 0 || !pawn(t, l, id).WorkClean {
		t.Fatal("an old save leaves cleaning off")
	}
	wv := l.World()
	if wv.Home[6*wv.TerrainW+20] != 1 || wv.Home[4*wv.TerrainW+18] != 1 || wv.Home[3*wv.TerrainW+20] == 1 {
		t.Fatal("old save Home is not seeded around the bed")
	}
	if _, ok := slices.BinarySearch(l.filth.cleanQ, int32(6*l.grid.Width()+20)); !ok {
		t.Fatal("dirty tile by the bed not queued after load")
	}
	// a current save keeps a player who switched cleaning off
	g.setPriority(id, work.Clean, 0)
	buf.Reset()
	if err := g.Save(&buf, "new"); err != nil {
		t.Fatal(err)
	}
	n, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if n.prioOf(id)[work.Clean] != 0 {
		t.Fatal("a current save re-enabled cleaning")
	}
}

// BenchmarkDirtStep is one colonist step indoors (gate: <= 150 ns).
func BenchmarkDirtStep(b *testing.B) {
	g := dirtGame(b)
	roofOver(g, 10, 2, 20, 10)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	x := 0
	for b.Loop() {
		x = (x + 1) % 26
		setCarried(g, id, 400)
		g.dirtStep(id, Position{X: 2 + x, Y: 5})
	}
}

// BenchmarkRoomFilthRebuild recomputes tread and roomSum on a 100x100 map
// with a dirty room grid (gate: <= 0.5 ms).
func BenchmarkRoomFilthRebuild(b *testing.B) {
	g := New(Config{Seed: 1, Width: 100, Height: 100})
	for y := 0; y < 100; y += 10 {
		for x := 0; x < 100; x++ {
			g.BuildNow("wall", "wood", "south", x, y)
		}
	}
	for y := 1; y < 100; y++ {
		for x := 0; x < 100; x += 3 {
			if y%10 != 0 {
				g.addDirt(y*100+x, 50+x)
			}
		}
	}
	fillStains(g, 0, stain.Cap, 300)
	g.ensureRooms()
	for b.Loop() {
		g.rebuildFilth(g.roomGrid, len(g.rooms))
	}
}

// downgradeSave rewrites a save as an older version would have written it.
func downgradeSave(t *testing.T, data []byte, edit func(*saveState)) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	edit(&s)
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(info); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(&s); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
