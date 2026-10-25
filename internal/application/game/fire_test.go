package game

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/fire"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/stain"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// fireGame is a w x h map under a clear, still sky with no grass (grass
// tiles are laid with grassRect).
func fireGame(t testing.TB, w, h int) *Game {
	t.Helper()
	g := New(Config{Seed: 7, Width: w, Height: h})
	g.SetWeather(weather.Clear, 0)
	g.HoldWeather()
	g.weather.WindSpeed = 0
	clear(g.seedGrass().cells)
	return g
}

// grassRect lays grass on x0..x1, y0..y1 (inclusive).
func grassRect(g *Game, x0, y0, x1, y1 int) {
	s := g.seedGrass()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			s.cells[y*g.grid.Width()+x] = 1
		}
	}
}

// fireStep advances to the next fire step and runs it (no other phase:
// nothing walks off).
func fireStep(g *Game) {
	now := uint64(g.clock.Now())
	g.clock.Advance(int(fire.StepTicks - now%fire.StepTicks))
	g.stepFires()
}

// fireLattice lights a fire of size at every third cell, so no two
// fires share a neighbour, and returns how many.
func fireLattice(g *Game, size int) int {
	n := 0
	for y := 1; y < g.grid.Height()-1; y += 3 {
		for x := 1; x < g.grid.Width()-1; x += 3 {
			g.startFire(x, y, size)
			n++
		}
	}
	return n
}

// spreadCounts runs one fire step over a lattice and counts the new fires
// by their offset from their source.
func spreadCounts(g *Game) map[[2]int]int {
	before := map[[2]int]bool{}
	for _, f := range g.fire.fires {
		before[[2]int{f.X, f.Y}] = true
	}
	fireStep(g)
	out := map[[2]int]int{}
	for _, f := range g.fire.fires {
		if before[[2]int{f.X, f.Y}] {
			continue
		}
		sx, sy := (f.X/3)*3+1, (f.Y/3)*3+1
		out[[2]int{f.X - sx, f.Y - sy}]++
	}
	return out
}

func total(m map[[2]int]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func TestFireSpreadRate(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 180, 180)
	grassRect(g, 0, 0, 179, 179)
	n := fireLattice(g, 80)
	got := total(spreadCounts(g))
	// size 84 after growing, into grass: 0.5 x 0.84 x 0.06 per neighbour
	want := float64(n*8) * 0.5 * 0.84 * 0.06
	if float64(got) < want*0.85 || float64(got) > want*1.15 {
		t.Fatalf("%d fires spread to %d cells, want about %.0f", n, got, want)
	}
	// below spread size nothing spreads
	g2 := fireGame(t, 30, 30)
	grassRect(g2, 0, 0, 29, 29)
	fireLattice(g2, fire.SpreadSize-fire.Grow-1)
	if c := total(spreadCounts(g2)); c != 0 {
		t.Fatalf("small fires spread to %d cells", c)
	}
}

func TestFireSpreadsDownwind(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 180, 180)
	grassRect(g, 0, 0, 179, 179)
	g.weather.WindSpeed, g.weather.WindDir = 3, 0 // blowing east
	fireLattice(g, 80)
	c := spreadCounts(g)
	east := c[[2]int{1, -1}] + c[[2]int{1, 0}] + c[[2]int{1, 1}]
	west := c[[2]int{-1, -1}] + c[[2]int{-1, 0}] + c[[2]int{-1, 1}]
	if west == 0 || float64(east) < float64(west)*1.25 || float64(east) > float64(west)*1.8 {
		t.Fatalf("downwind %d vs upwind %d, want about 1.5x", east, west)
	}
}

func TestRoofShieldsFromOutdoorFire(t *testing.T) {
	t.Parallel()
	open := fireGame(t, 120, 120)
	grassRect(open, 0, 0, 119, 119)
	fireLattice(open, 80)
	roofed := fireGame(t, 120, 120)
	grassRect(roofed, 0, 0, 119, 119)
	roofed.roofs = make([]roof.Cell, 120*120)
	for i := range roofed.roofs {
		roofed.roofs[i] = roof.Built
	}
	fireLattice(roofed, 80)
	for _, f := range roofed.fire.fires { // the fires burn in the open
		roofed.roofs[f.Y*120+f.X] = 0
	}
	o, r := total(spreadCounts(open)), total(spreadCounts(roofed))
	if r == 0 || float64(r) > float64(o)*0.3 || float64(r) < float64(o)*0.1 {
		t.Fatalf("into roofed cells %d vs open %d, want about a fifth", r, o)
	}
}

func TestFireBurnsOutWithoutFuel(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 10, 10)
	g.startFire(5, 5, 30)
	for i := 1; i <= 5; i++ {
		fireStep(g)
		if want := max(0, 30-i*fire.Starve); g.fireAt(5, 5) != want {
			t.Fatalf("step %d: size %d, want %d", i, g.fireAt(5, 5), want)
		}
	}
	if len(g.fire.fires) != 0 || g.fire.rt.cell != nil {
		t.Fatal("a burnt-out fire is dropped with its index")
	}
}

func TestRainDousesOnlyOutdoorFires(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 10, 10)
	grassRect(g, 0, 0, 9, 9)
	g.SetWeather(weather.Rain, 0.8)
	g.roofs = make([]roof.Cell, 100)
	g.roofs[5*10+7] = roof.Built
	g.startFire(2, 5, 30) // open sky
	g.startFire(7, 5, 30) // under a roof
	fireStep(g)
	if got := g.fireAt(2, 5); got != 30+fire.Grow-fire.Drench {
		t.Errorf("outdoor fire in rain: %d", got)
	}
	if got := g.fireAt(7, 5); got != 30+fire.Grow {
		t.Errorf("roofed fire in rain: %d", got)
	}
	g.SetWeather(weather.Rain, 0.1) // a drizzle doesn't douse
	fireStep(g)
	if got := g.fireAt(2, 5); got != 30+2*fire.Grow-fire.Drench {
		t.Errorf("outdoor fire in drizzle: %d", got)
	}
}

func TestFireDamagesBuildings(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 12, 8)
	wood := buildNow(t, g, construction.Wall, item.Wood, construction.South, 3, 3)
	stone := buildNow(t, g, construction.Wall, item.GraniteBlocks, construction.South, 6, 3)
	g.startFire(3, 3, 40)
	if g.ignite(6, 3, 40) {
		t.Fatal("a stone wall caught fire")
	}
	fireStep(g)
	b, _ := g.bldgs.Get(wood)
	if b.Damage != fire.BuildingDamage(44) {
		t.Fatalf("wooden wall damage %d, want %d", b.Damage, fire.BuildingDamage(44))
	}
	g.startFire(6, 3, 40) // forced (developer): stone still takes nothing
	fireStep(g)
	if s, _ := g.bldgs.Get(stone); s.Damage != 0 {
		t.Fatalf("stone wall damage %d", s.Damage)
	}
	for i := 0; i < 60 && g.layerBldgAt(3, 3, construction.LayerObject) != 0; i++ {
		fireStep(g)
	}
	if g.layerBldgAt(3, 3, construction.LayerObject) != 0 {
		t.Fatal("the wooden wall never burned down")
	}
	if g.layerFrameAt(3, 3, construction.LayerObject) != 0 || len(g.looseAt(3, 3)) != 0 {
		t.Fatal("a burnt wall leaves no blueprint and no refund")
	}
	if s := g.stains.tiles[[2]int{3, 3}]; s.Kind != stain.Ash || s.Amount == 0 {
		t.Fatalf("no ash where the wall stood: %+v", s)
	}
	if !hasHistory(g, "fire", "wall burned down") || !hasMessage(g, "The wall burned down.") {
		t.Fatal("a lost building is noted in the History and the messages")
	}
}

func hasHistory(g *Game, kind, text string) bool {
	for _, h := range g.history {
		if h.Kind == kind && strings.Contains(h.Text, text) {
			return true
		}
	}
	return false
}

func TestFireBurnsPlantsGrassAndItems(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 12, 8)
	grassRect(g, 2, 2, 2, 2)
	tree, err := g.SpawnPlantSpecies("tree", 5, 2, 100)
	if err != nil {
		t.Fatal(err)
	}
	wood, _ := g.SpawnStack(item.Wood, 10, 8, 2)
	steel, _ := g.SpawnStack(item.Steel, 10, 8, 4)
	for _, c := range [][2]int{{2, 2}, {5, 2}, {8, 2}, {8, 4}} {
		g.startFire(c[0], c[1], 40)
	}
	for i := 0; i < fire.GrassBurnSteps; i++ {
		if !g.hasGrass(2, 2) {
			t.Fatalf("grass gone after %d steps", i)
		}
		fireStep(g)
	}
	if g.hasGrass(2, 2) || g.stains.tiles[[2]int{2, 2}].Kind != stain.Ash {
		t.Fatal("grass burns away to ash")
	}
	for i := fire.GrassBurnSteps; i < fire.ItemBurnSteps; i++ {
		fireStep(g)
	}
	if _, ok := g.stacks.Get(wood); ok {
		t.Fatal("wood on a burning cell survived")
	}
	if _, ok := g.stacks.Get(steel); !ok {
		t.Fatal("steel burned")
	}
	if g.fireAt(8, 4) != 0 {
		t.Fatal("a fire on steel alone burns out")
	}
	for i := fire.ItemBurnSteps; i < fire.PlantBurnSteps(plant.Tree); i++ {
		if _, ok := g.plants.Get(tree); !ok {
			t.Fatalf("tree gone after %d steps", i)
		}
		fireStep(g)
	}
	if _, ok := g.plants.Get(tree); ok {
		t.Fatal("the tree never burned")
	}
	if g.stains.tiles[[2]int{5, 2}].Kind != stain.Ash {
		t.Fatal("a burnt tree leaves ash")
	}
	for i := 0; i < 20; i++ {
		fireStep(g)
	}
	if len(g.fire.fires) != 0 {
		t.Fatalf("fires left with nothing to burn: %+v", g.fire.fires)
	}
}

func TestFireBurnsPawnsAndAnimals(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 12, 8)
	p, _ := g.SpawnPawn(3, 3)
	beside, _ := g.SpawnPawn(5, 3)
	buck, _ := g.SpawnAnimal("dustbuck", 8, 3)
	g.startFire(3, 3, 90)
	g.startFire(4, 3, 90)
	g.startFire(8, 3, 40)
	fireStep(g)
	b := g.bodies[p]
	if b == nil || !slices.ContainsFunc(b.Wounds, func(w body.Wound) bool { return w.Kind == body.Burn }) {
		t.Fatal("a colonist in fire gets burn wounds")
	}
	if v, _ := g.vital.Get(beside); v.Injury != fire.AdjacentDamage(94) {
		t.Fatalf("beside a big fire: injury %d, want %d", v.Injury, fire.AdjacentDamage(94))
	}
	if a, _ := g.animals.Get(buck); a.Injury != fire.CreatureDamage(44) {
		t.Fatalf("animal in fire: injury %d, want %d", a.Injury, fire.CreatureDamage(44))
	}
	for i := 0; i < 200; i++ {
		if v, _ := g.vital.Get(p); v.Status == health.Dead {
			break
		}
		g.fire.fires[g.fireIndex(3, 3)].Size = fire.MaxSize
		fireStep(g)
	}
	if v, _ := g.vital.Get(p); v.Status != health.Dead || v.Cause != health.CauseBurns {
		t.Fatalf("left in the fire: %+v, want dead of burns", v)
	}
	if !hasMessage(g, "burned to death") {
		t.Fatal("a death by fire is announced")
	}
}

func TestFireCollapsesBridge(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	g.SetWeather(weather.Clear, 0)
	g.HoldWeather()
	bridgeNow(t, g, 7, 3)
	if _, err := g.SpawnStack(item.GraniteBlocks, 5, 7, 3); err != nil {
		t.Fatal(err)
	}
	if !g.ignite(7, 3, 60) {
		t.Fatal("a wooden bridge burns")
	}
	for i := 0; i < 80 && g.layerBldgAt(7, 3, construction.LayerFoundation) != 0; i++ {
		fireStep(g)
	}
	if g.layerBldgAt(7, 3, construction.LayerFoundation) != 0 {
		t.Fatal("the bridge never burned through")
	}
	if len(g.looseAt(7, 3)) != 0 {
		t.Fatal("what lay on the bridge sank with it")
	}
	if !hasMessage(g, "sank") {
		t.Fatal("the sinking is announced (the bridge cascade)")
	}
	if g.terr.At(7, 3) != terrain.DeepWater {
		t.Fatal("terrain never burns")
	}
}

func TestFireHeatsItsRoom(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	jumpTo(g, 45, 0)
	g.SetWeather(weather.Clear, 0)
	g.HoldWeather()
	hut(t, g, 1, 1, 7, 7)
	hut(t, g, 10, 1, 16, 7)
	g.ensureRooms()
	r := g.roomAt(4, 4)
	g.heatRooms()
	before := g.roomTemps[r]
	other := g.roomTemps[g.roomAt(13, 4)]
	g.startFire(4, 4, 100)
	g.heatRooms()
	ctrl := g.roomTemps[g.roomAt(13, 4)]
	if g.roomTemps[r]-before <= ctrl-other {
		t.Fatalf("the burning room warmed %v, the other %v", g.roomTemps[r]-before, ctrl-other)
	}
}

func TestOutdoorFireWarmsAround(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 30, 30)
	base := g.temperatureAt(10, 10)
	g.startFire(10, 10, 50)
	if d := g.temperatureAt(12, 12) - base; d != 50*fireOutdoorHeat {
		t.Fatalf("2 cells off: +%v, want %v", d, 50*fireOutdoorHeat)
	}
	if d := g.temperatureAt(13, 10) - base; d != 0 {
		t.Fatalf("3 cells off: +%v, want 0", d)
	}
}

func TestFireIgnitionSources(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 12, 8)
	grassRect(g, 0, 0, 11, 7)
	if g.ignite(-1, 0, 10) || g.startFire(12, 0, 10) {
		t.Fatal("off the map")
	}
	g.Apply(command.Command{Kind: command.DevStartFire, X: 2, Y: 2})
	g.ApplyPending()
	if g.fireAt(2, 2) != devFireSize {
		t.Fatalf("dev fire size %d", g.fireAt(2, 2))
	}
	pos := Position{X: 8, Y: 4}
	g.burst(0, pos)
	n := 0
	for _, d := range neighbours8 {
		if g.fireAt(8+d[0], 4+d[1]) > 0 {
			n++
		}
	}
	if g.fireAt(8, 4) != burstFireSize || n != 8 {
		t.Fatalf("burst: centre %d, %d neighbours alight", g.fireAt(8, 4), n)
	}
	// a fire lit again keeps the bigger size
	g.startFire(2, 2, 5)
	if g.fireAt(2, 2) != devFireSize {
		t.Fatal("relighting shrank a fire")
	}
}

func TestDouseFire(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 10, 10)
	g.startFire(3, 3, 25)
	ver := g.fire.ver
	if !g.douseFire(3, 3, 10) || g.fireAt(3, 3) != 15 || g.fire.ver == ver {
		t.Fatal("douse takes size off")
	}
	g.douseFire(3, 3, 20)
	if g.fireAt(3, 3) != 0 || len(g.fire.fires) != 0 || g.douseFire(3, 3, 1) {
		t.Fatal("douse puts a fire out")
	}
}

func TestFireView(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 10, 10)
	if w := g.World(); w.Fires != nil {
		t.Fatal("no fires, no view")
	}
	g.startFire(4, 2, 20)
	g.startFire(1, 2, 30)
	w := g.World()
	want := []FireView{{X: 1, Y: 2, Size: 30}, {X: 4, Y: 2, Size: 20}}
	if !slices.Equal(w.Fires, want) || w.FireVer != g.fire.ver {
		t.Fatalf("view %+v ver %d", w.Fires, w.FireVer)
	}
	ver := w.FireVer
	fireStep(g)
	if w2 := g.World(); w2.FireVer == ver || w2.Fires[0].Size != 24 {
		t.Fatalf("a step moves the view: %+v", w2.Fires)
	}
}

func TestFireSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 30, 30)
	grassRect(g, 0, 0, 29, 29)
	g.startFire(10, 10, 60)
	g.startFire(20, 5, 40)
	for i := 0; i < 5; i++ {
		fireStep(g)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "fire"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.fire.fires, g.fire.fires) || h.fireAt(10, 10) != g.fireAt(10, 10) {
		t.Fatalf("loaded fires %+v, want %+v", h.fire.fires, g.fire.fires)
	}
	h.weather.WindSpeed = g.weather.WindSpeed
	for i := 0; i < 10; i++ {
		fireStep(g)
		fireStep(h)
	}
	if !slices.Equal(h.fire.fires, g.fire.fires) {
		t.Fatal("a loaded blaze burns on as the unbroken one does")
	}
}

func TestOldSaveHasNoFires(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 10, 10)
	var buf bytes.Buffer
	if err := g.Save(&buf, "nofire"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.fire.fires) != 0 || h.fire.rt.cell != nil || h.exportFires() != nil {
		t.Fatal("a save without fires loads without")
	}
}

// BenchmarkFireStep500 is one fire step of a 500-cell blaze over grass
// (PLAY-015 budget: 2 ms).
func BenchmarkFireStep500(b *testing.B) {
	g := fireGame(b, 60, 60)
	grassRect(g, 0, 0, 59, 59)
	for y := 10; y < 30; y++ {
		for x := 10; x < 35; x++ {
			g.startFire(x, y, 60)
		}
	}
	if len(g.fire.fires) != 500 {
		b.Fatalf("%d fires", len(g.fire.fires))
	}
	g.clock.Advance(int(fire.StepTicks - uint64(g.clock.Now())%fire.StepTicks))
	snap := slices.Clone(g.fire.fires)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		g.fire.fires = append(g.fire.fires[:0], snap...)
		g.reindexFires()
		b.StartTimer()
		g.stepFires()
	}
}

func TestBurstInsideAFireStepLightsAfterIt(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 12, 8)
	grassRect(g, 0, 0, 11, 7)
	buck, err := g.SpawnAnimal("emberbuck", 5, 4)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := g.animals.Get(buck)
	a.Injury = 1 << 20 // one more hurt kills it
	g.animals.Set(buck, a)
	g.startFire(5, 4, 30)
	fireStep(g)
	if _, ok := g.animals.Get(buck); ok {
		t.Fatal("the emberbuck survived its fire")
	}
	if g.fire.rt.stepping || len(g.fire.rt.pending) != 0 {
		t.Fatal("the step ended with fires still pending")
	}
	if g.fireAt(4, 4) != burstFireSize || g.fireAt(6, 5) != burstFireSize {
		t.Fatalf("the burst's fires: %+v", g.fire.fires)
	}
}
