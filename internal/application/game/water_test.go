package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/terrain"
)

// fillTerrain sets every tile of x0..x1, y0..y1 (inclusive) to t.
func fillTerrain(g *Game, x0, y0, x1, y1 int, t terrain.Type) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			g.setTerrain(x, y, t)
		}
	}
}

// buildNow puts a finished building of kind and stuff at (x, y).
func buildNow(t *testing.T, g *Game, k construction.Kind, stuff item.Kind, f construction.Facing, x, y int) entity.EntityID {
	t.Helper()
	g.placeFrameStuff(k, stuff, f, x, y)
	if fid := g.layerFrameAt(x, y, construction.DefOf(k).Layer); fid != 0 {
		fr, _ := g.frames.Get(fid)
		g.completeFrame(fid, fr)
	}
	bid := g.layerBldgAt(x, y, construction.DefOf(k).Layer)
	if bid == 0 {
		t.Fatalf("no %s at %d,%d", k, x, y)
	}
	return bid
}

func TestWaterPlacementRefused(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	fillTerrain(g, 5, 2, 7, 4, terrain.ShallowWater)
	fillTerrain(g, 10, 2, 12, 4, terrain.DeepWater)
	// shallow and deep water refuse walls and floors alike, with the water reason
	for _, c := range []struct {
		kind string
		x, y int
	}{{"wall", 6, 3}, {"floor", 6, 3}, {"wall", 11, 3}, {"floor", 11, 3}} {
		if ok, r := g.CanPlace(c.kind, c.x, c.y); ok || r != ReasonWater {
			t.Errorf("%s on water (%d,%d): %v %q, want refused with %q", c.kind, c.x, c.y, ok, r, ReasonWater)
		}
	}
	// a footprint that touches water at one cell is refused whole
	if ok, r := g.CanPlaceAt("bed", "wood", "south", 7, 4); ok || r != ReasonWater {
		t.Errorf("bed half in water: %v %q", ok, r)
	}
	// dry ground next to it still builds
	if ok, r := g.CanPlace("wall", 8, 3); !ok {
		t.Errorf("wall on dry ground: %q", r)
	}
	// a stockpile painted over water and land claims only the land
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 4, Y: 3, X2: 8, Y2: 3})
	for _, z := range g.zoneViews() {
		for _, c := range z.Cells {
			if g.isWaterAt(c[0], c[1]) {
				t.Fatalf("stockpile painted on water at %v", c)
			}
		}
		if len(z.Cells) != 3 { // x 4, 8 and none of the shallow 5..7
			t.Fatalf("stockpile has %d cells, want the 2 dry ones: %v", len(z.Cells), z.Cells)
		}
	}
}

func TestPathAvoidsShallow(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	// a 4-wide shallow band with a dry gap at the bottom: wading costs more than the detour
	fillTerrain(g, 10, 0, 13, 9, terrain.ShallowWater)
	path, ok := pathing.Find(g.costedQuery(id), 2, 5, 20, 5)
	if !ok {
		t.Fatal("no path")
	}
	for _, p := range path {
		if g.terr.At(p.X, p.Y) == terrain.ShallowWater {
			t.Fatalf("the path wades at %v although dry ground is a short way round", p)
		}
	}
	// with no way round it wades rather than fail
	fillTerrain(g, 10, 10, 13, 11, terrain.ShallowWater)
	path, ok = pathing.Find(g.costedQuery(id), 2, 5, 20, 5)
	if !ok {
		t.Fatal("shallow water is not a wall")
	}
	waded := 0
	for _, p := range path {
		if g.terr.At(p.X, p.Y) == terrain.ShallowWater {
			waded++
		}
	}
	if waded != 4 {
		t.Fatalf("waded %d tiles, want the 4-tile band crossed straight", waded)
	}
	// deep water blocks outright
	fillTerrain(g, 20, 0, 20, 11, terrain.DeepWater)
	if _, ok := pathing.Find(g.costedQuery(id), 2, 5, 25, 5); ok {
		t.Fatal("deep water must block")
	}
	// the extra cost is (WalkCost-100)/10: shallow 20, mud 4, marsh 10, a road costs none
	g.setTerrain(1, 1, terrain.Mud)
	g.setTerrain(2, 1, terrain.Marsh)
	g.setTerrain(3, 1, terrain.Track)
	for _, c := range []struct{ x, want int }{{1, 4}, {2, 10}, {3, 0}, {5, 0}} {
		if got := g.groundExtra(c.x, 1); got != c.want {
			t.Errorf("groundExtra(%d) = %d, want %d", c.x, got, c.want)
		}
	}
	if g.groundExtra(12, 5) != 20 {
		t.Errorf("shallow extra = %d, want 20", g.groundExtra(12, 5))
	}
}

func TestMudDirtGain(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	soil, _ := g.SpawnNamedPawn(2, 3, "Ada")
	mud, _ := g.SpawnNamedPawn(2, 7, "Bo")
	fillTerrain(g, 3, 7, 12, 7, terrain.Mud)
	walkRow(g, soil, 2, 12, 3)
	walkRow(g, mud, 2, 12, 7)
	want := 10 * filth.StepGain
	if got := carried(g, soil); got != want {
		t.Fatalf("10 soil steps carried %d, want %d", got, want)
	}
	// mud is wet ground (800 permille even on a dry day) and doubles the gain
	perStep := 2 * filth.Gain(filth.StepGain, 800, false)
	if got := carried(g, mud); got != 10*perStep || got <= 3*want {
		t.Fatalf("10 mud steps carried %d, want %d (more than 3x soil's %d)", got, 10*perStep, want)
	}
}

func TestWadingSoakedAndWashes(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	fillTerrain(g, 5, 5, 7, 5, terrain.ShallowWater)
	setCarried(g, id, 400)
	g.positions.Set(id, Position{X: 5, Y: 5})
	g.dirtStep(id, Position{X: 5, Y: 5})
	if got := carried(g, id); got != 200 {
		t.Fatalf("one wading step left %d dirt, want half of 400", got)
	}
	if !remembers(g, id, mood.Soaked) {
		t.Fatal("wading should leave the soaked thought")
	}
	g.dirtStep(id, Position{X: 6, Y: 5})
	g.dirtStep(id, Position{X: 7, Y: 5})
	if got := carried(g, id); got != 50 {
		t.Fatalf("three wading steps left %d dirt, want 50", got)
	}
	// someone who never wades is not soaked
	dry, _ := g.SpawnNamedPawn(2, 8, "Bo")
	g.dirtStep(dry, Position{X: 3, Y: 8})
	if remembers(g, dry, mood.Soaked) {
		t.Fatal("dry feet are not soaked")
	}
}

func TestNothingSpawnsInWater(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 40, Height: 20})
	fillTerrain(g, 0, 0, 39, 19, terrain.ShallowWater)
	fillTerrain(g, 14, 4, 30, 15, terrain.Soil) // a dry island in the middle
	for y := 0; y < 20; y++ {
		g.setTerrain(39, y, terrain.Soil) // and a dry strip on the east edge
	}
	g.SpawnNamedPawn(20, 10, "Ada")
	// edge entries
	for i := 0; i < 60; i++ {
		x, y, ok := g.edgeTile()
		if !ok || g.isWaterAt(x, y) {
			t.Fatalf("edgeTile = %d,%d ok=%v on water", x, y, ok)
		}
		if x, y, ok := g.mapEdgeTile(); ok && g.isWaterAt(x, y) {
			t.Fatalf("mapEdgeTile on water at %d,%d", x, y)
		}
		if x, y, ok := g.randomTileAway(5); ok && g.isWaterAt(x, y) {
			t.Fatalf("randomTileAway on water at %d,%d", x, y)
		}
	}
	// animals refuse water tiles outright
	if _, err := g.SpawnAnimal("deer", 3, 3); err == nil {
		t.Fatal("an animal was spawned in the water")
	}
	// raiders arrive on dry ground
	g.Raid(4)
	for _, rid := range g.raiderIDs {
		p, _ := g.positions.Get(rid)
		if g.isWaterAt(p.X, p.Y) {
			t.Fatalf("raider arrived in water at %v", p)
		}
	}
	// a dropped stack in the water floats to the nearest dry tile
	g.dropStack(item.Wood, 5, 10, 10)
	found := false
	for _, iid := range g.itemIDs {
		if st, _ := g.stacks.Get(iid); st.Kind == item.Wood {
			found = true
			if g.isWaterAt(st.X, st.Y) {
				t.Fatalf("stack left on water at %d,%d", st.X, st.Y)
			}
		}
	}
	if !found {
		t.Fatal("the dropped stack vanished")
	}
}

func TestComponentFramesWait(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.SpawnStack(item.Steel, 100, 4, 4)
	id := hauler(t, g, 6, 6)
	g.placeFrameStuff(construction.HospitalBed, item.Steel, construction.South, 14, 6)
	fid := g.layerFrameAt(14, 6, construction.LayerObject)
	fr, ok := g.frames.Get(fid)
	if !ok || fr.CompNeed != 2 || fr.Need != 45 {
		t.Fatalf("hospital bed frame %+v, want 45 steel and 2 components", fr)
	}
	// steel arrives, but with no components on the map the frame stays unbuilt
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if fr, _ = g.frames.Get(fid); fr.Wood == fr.Need {
			break
		}
	}
	g.Step(600)
	fr, _ = g.frames.Get(fid)
	if fr.Wood != 45 || fr.Comp != 0 || fr.Ready() {
		t.Fatalf("steel in, no components: wood %d comp %d ready %v", fr.Wood, fr.Comp, fr.Ready())
	}
	views := g.World().Frames
	if len(views) != 1 || views[0].Status != "Needs 2 components" {
		t.Fatalf("frame inspector says %+v, want \"Needs 2 components\"", views)
	}
	// components appear: they are hauled after the material
	g.SpawnStack(item.Components, 3, 5, 4)
	for i := 0; i < 4000; i++ {
		g.Step(1)
		if fr, _ = g.frames.Get(fid); fr.Ready() {
			break
		}
	}
	fr, _ = g.frames.Get(fid)
	if !fr.Ready() || fr.Comp != 2 {
		t.Fatalf("components delivered: comp %d ready %v", fr.Comp, fr.Ready())
	}
	if n := g.countItem(item.Components); n != 1 {
		t.Fatalf("%d components left on the map, want the 1 spare", n)
	}
	if len(g.haul.incoming) != 0 || len(g.haul.deliveries) != 0 {
		t.Fatalf("delivery bookkeeping left over: %v %v", g.haul.incoming, g.haul.deliveries)
	}
	_ = id
}

func TestCancelledFrameReturnsComponents(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	g.placeFrameStuff(construction.ToolCabinet, item.Steel, construction.South, 8, 5)
	fid := g.layerFrameAt(8, 5, construction.LayerObject)
	fr, _ := g.frames.Get(fid)
	fr.Wood, fr.Comp = fr.Need, fr.CompNeed
	g.frames.Set(fid, fr)
	g.cancelFrame(fid)
	if g.countItem(item.Steel) != 30 || g.countItem(item.Components) != 1 {
		t.Fatalf("cancelled frame returned steel %d components %d", g.countItem(item.Steel), g.countItem(item.Components))
	}
}

func TestMachineryOreYieldsComponents(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnNamedPawn(1, 2, "Bo")
	if !g.SetGround(6, 0, 6, 4, "rock", "slate", "machinery") {
		t.Fatal("set ground")
	}
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: 6, Y: 2, X2: 6, Y2: 2})
	g.Step(TicksPerCell*20 + MineWork*3/2 + 20)
	if g.TerrainAt(6, 2) != "stone" {
		t.Fatalf("mined tile = %s", g.TerrainAt(6, 2))
	}
	if g.stockOf(item.Components) != 2 || g.stockOf(item.Steel) != 0 || g.stockOf(item.SlateChunk) != 0 {
		t.Fatalf("components %d steel %d chunks %d, want 2 components only", g.stockOf(item.Components), g.stockOf(item.Steel), g.stockOf(item.SlateChunk))
	}
}

func TestDeconstructRefundsHalfComponents(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	g.SpawnNamedPawn(2, 2, "Bo")
	buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 8, 4)
	buildNow(t, g, construction.ToolCabinet, item.Steel, construction.South, 12, 4)
	g.Apply(command.Command{Kind: command.DesignateDeconstruct, X: 8, Y: 4, X2: 12, Y2: 5})
	g.Step(TicksPerCell*30 + 600)
	// 2 components -> 1; 1 component -> 0 (rounded down); steel 22 and 15
	if g.countItem(item.Components) != 1 || g.countItem(item.Steel) != 22+15 {
		t.Fatalf("components %d steel %d, want 1 and 37", g.countItem(item.Components), g.countItem(item.Steel))
	}
}

func TestPodShellDeconstructsToSteel(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	g.SpawnNamedPawn(2, 2, "Bo")
	buildNow(t, g, construction.PodShell, item.Steel, construction.South, 8, 4)
	if !g.grid.Walkable(8, 4) {
		t.Fatal("a pod shell is walkable")
	}
	g.Apply(command.Command{Kind: command.DesignateDeconstruct, X: 8, Y: 4, X2: 8, Y2: 4})
	g.Step(TicksPerCell*20 + 60)
	if g.countItem(item.Steel) != 6 || g.layerBldgAt(8, 4, construction.LayerObject) != 0 {
		t.Fatalf("steel %d, shell gone %v: want 6 steel", g.countItem(item.Steel), g.layerBldgAt(8, 4, construction.LayerObject) == 0)
	}
}

func TestMedicineChoiceNoFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(14, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	buildNow(t, g, construction.Bed, item.Wood, construction.South, 15, 2)
	g.SpawnStack(item.HerbalMedicine, 3, 6, 9)
	g.SpawnStack(item.IndustrialMedicine, 2, 7, 9)
	// the tiers: infected or severely hurt -> industrial first, a scratch -> herbal first
	if tiers := g.medicineTiers(patient); tiers[0] != item.HerbalMedicine {
		t.Fatalf("an unhurt patient: %v", tiers)
	}
	g.Infect(patient, 40)
	if tiers := g.medicineTiers(patient); tiers[0] != item.IndustrialMedicine || tiers[1] != item.HerbalMedicine {
		t.Fatalf("an infected patient: %v", tiers)
	}
	g.bodies[patient].RemoveCond(body.CondInfection) // MED-02: infections are conditions
	// a severe injury: the dose is picked at the start and carried, so it
	// never changes mid-job even when the patient stops being a severe case
	g.Injure(patient, 75)
	if tiers := g.medicineTiers(patient); tiers[0] != item.IndustrialMedicine {
		t.Fatalf("a severely hurt patient: %v", tiers)
	}
	var carried item.Kind
	eased := false
	for i := 0; i < 600 && !g.tended(patient); i++ {
		g.Step(5)
		g.SetNeeds(doc, 1<<20, 1<<20)
		g.SetNeeds(patient, 1<<20, 1<<20)
		j, _ := g.jobs.Get(doc)
		if j.Kind != job.Tend || j.Carry == 0 {
			continue
		}
		st, _ := g.stacks.Get(j.Carry)
		if carried == 0 {
			carried = st.Kind
		}
		if st.Kind != carried {
			t.Fatalf("the dose flapped from %v to %v mid-job", carried, st.Kind)
		}
		if !eased && j.State == job.Do {
			eased = true
			v, _ := g.vital.Get(patient)
			v.Injury = 30 // no longer severe: the chosen dose still holds
			g.vital.Set(patient, v)
		}
	}
	if !eased {
		t.Fatal("the tend never reached its work")
	}
	if carried != item.IndustrialMedicine {
		t.Fatalf("the doctor carried %v, want industrial medicine for a severe injury", carried)
	}
	if g.countItem(item.IndustrialMedicine) != 1 || g.countItem(item.HerbalMedicine) != 3 {
		t.Fatalf("industrial %d herbal %d, want one industrial dose used", g.countItem(item.IndustrialMedicine), g.countItem(item.HerbalMedicine))
	}
	// without industrial medicine the herbal dose is the fallback
	h := New(Config{Seed: 2, Width: 20, Height: 12})
	p, _ := h.SpawnNamedPawn(14, 6, "Patient")
	d, _ := h.SpawnNamedPawn(4, 6, "Doc")
	h.SpawnStack(item.HerbalMedicine, 1, 6, 9)
	h.Infect(p, 40)
	rec := h.withMedicine(d, job.Record{Kind: job.Tend, Target: p, TX: 14, TY: 6})
	if st, _ := h.stacks.Get(rec.Dest); rec.Dest == 0 || st.Kind != item.HerbalMedicine {
		t.Fatalf("fallback dose %+v", st)
	}
}

// TestIndustrialCareTendsBest (MED-02): with the same doctor, an
// industrial tend's quality always reaches at least the best herbal one
// (potency 1000 vs 600, max 1000 vs 700).
func TestIndustrialCareTendsBest(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 12, Height: 8})
	doc, _ := g.SpawnNamedPawn(4, 5, "Doc")
	patient, _ := g.SpawnNamedPawn(5, 5, "Patient")
	minInd, maxHerb := int32(1<<30), int32(0)
	for i := 0; i < 40; i++ {
		minInd = min(minInd, g.tendQualityFor(doc, patient, item.IndustrialMedicine))
		maxHerb = max(maxHerb, g.tendQualityFor(doc, patient, item.HerbalMedicine))
	}
	if minInd < maxHerb || maxHerb > 700 {
		t.Fatalf("industrial at least %d, herbal at most %d", minInd, maxHerb)
	}
}

func TestToolCabinetBonusCap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 12})
	bench := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 10, 5)
	if got := g.cabinetBonus(bench); got != 0 {
		t.Fatalf("no cabinets: %d", got)
	}
	buildNow(t, g, construction.ToolCabinet, item.Steel, construction.South, 13, 2)
	if got := g.cabinetBonus(bench); got != 6 {
		t.Fatalf("one cabinet: %d%%, want 6", got)
	}
	buildNow(t, g, construction.ToolCabinet, item.Steel, construction.South, 8, 8)
	if got := g.cabinetBonus(bench); got != 12 {
		t.Fatalf("two cabinets: %d%%, want 12", got)
	}
	third := buildNow(t, g, construction.ToolCabinet, item.Steel, construction.South, 11, 8)
	if got := g.cabinetBonus(bench); got != 12 {
		t.Fatalf("a third cabinet adds nothing: %d%%, want the cap of 12", got)
	}
	// out of reach (7 tiles away) does not count
	far := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 30, 5)
	buildNow(t, g, construction.ToolCabinet, item.Steel, construction.South, 37, 5)
	if got := g.cabinetBonus(far); got != 0 {
		t.Fatalf("cabinet 7 tiles away: %d%%, want 0", got)
	}
	// the count follows the buildings (cache invalidated by the structure version)
	g.removeBuilding(third)
	if got := g.cabinetBonus(bench); got != 12 {
		t.Fatalf("after removing one of three: %d%%", got)
	}
}

// TestHospitalBedHeal (MED-02): a hospital bed adds 10 % to a tend's
// quality and heals faster than a bed, which heals faster than the
// ground; standing gets no lying bonus.
func TestHospitalBedHeal(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(5, 5, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 5, "Doc")
	mean := func() int32 {
		var sum int32
		for i := 0; i < 60; i++ {
			sum += g.tendQualityFor(doc, patient, noMedicine)
		}
		return sum / 60
	}
	floor := mean()
	if g.lyingHeal(patient) != 0 {
		t.Fatal("standing: no lying bonus")
	}
	bid := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 5, 5)
	b, _ := g.bldgs.Get(bid)
	g.bldgs.Set(bid, b.Claim(uint64(patient)))
	if !g.inHospitalBed(patient) {
		t.Fatal("the patient lies in the hospital bed")
	}
	if g.lyingHeal(patient) != body.LieHospital {
		t.Fatalf("hospital bed heal %d", g.lyingHeal(patient))
	}
	if bed := mean(); bed < floor+hospitalTendOffset*3/4 {
		t.Fatalf("tend quality in a hospital bed %d, on the floor %d: want about +%d", bed, floor, hospitalTendOffset)
	}
	g.down(patient, health.CauseInjury)
	g.removeBuilding(bid)
	if g.lyingHeal(patient) != body.LieGround {
		t.Fatalf("downed on the ground heals %d", g.lyingHeal(patient))
	}
}
func TestRescuePrefersHospitalBed(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(5, 5, "Patient")
	near := buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 5)
	got, ok := g.freeBedFor(patient, Position{X: 5, Y: 5})
	if !ok || got != near {
		t.Fatalf("only a plain bed: %v %v", got, ok)
	}
	hosp := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 16, 8)
	if got, ok := g.freeBedFor(patient, Position{X: 5, Y: 5}); !ok || got != hosp {
		t.Fatalf("a far hospital bed should beat a near plain bed: got %v want %v", got, hosp)
	}
}

func TestStockpilesTakeNewGoods(t *testing.T) {
	t.Parallel()
	for _, k := range []item.Kind{item.Components, item.IndustrialMedicine} {
		if !allows(zone.Zone{}, k) {
			t.Errorf("a stockpile should take %v", k)
		}
	}
}
