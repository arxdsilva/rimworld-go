package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/weather"
)

// WRK-001..008, HRV-001..010: the tie-break between work types, the order
// inside them, who harvests what and what a harvest yields.

// TestSowsBeforeHauling (WRK-001): everything at 3, a zone to sow and a pile
// on the ground: the colonist sows first.
func TestSowsBeforeHauling(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	for _, pid := range append([]entity.EntityID(nil), g.plantIDs...) {
		g.removePlant(pid)
	}
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	setSkill(g, id, skPlants, 10)
	zoneCmd(g, command.DesignateStockpile, 20, 5, 21, 6, 0)
	spawn(t, g, item.Wood, 10, 6, 6)
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: 8, Y: 5, X2: 9, Y2: 5})
	g.Step(1)
	sawn := false
	for range 400 {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
		j, _ := g.jobs.Get(id)
		sawn = sawn || j.Kind == job.Sow
		if j.Kind == job.Haul && sowCellsLeft(g, 8, 5, 9, 5) {
			t.Fatalf("hauling with cells still to sow: %+v", j)
		}
	}
	if !sawn {
		t.Fatal("the colonist never sowed")
	}
}

// sowCellsLeft reports a cell of the box with no plant yet.
func sowCellsLeft(g *Game, x0, y0, x1, y1 int) bool {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if g.plantAt(x, y) == 0 {
				return true
			}
		}
	}
	return false
}

// TestBuilderSuppliesItsOwnFrame (WRK-003): Haul off, Construct on: the
// builder carries the wood to the frame and builds it.
func TestBuilderSuppliesItsOwnFrame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	id, _ := g.SpawnNamedPawn(4, 4, "Ada")
	onlyWork(g, id, work.Construct)
	spawn(t, g, item.Wood, 50, 6, 6)
	woodWalls(g, 10, 7, 1)
	if len(g.frameIDs) != 1 {
		t.Fatalf("%d frames", len(g.frameIDs))
	}
	for i := 0; i < 4000 && len(g.frameIDs) > 0; i++ {
		runFed(g, 1, id)
	}
	if len(g.frameIDs) != 0 {
		fr, _ := g.frames.Get(g.frameIDs[0])
		j, _ := g.jobs.Get(id)
		t.Fatalf("the frame still stands: %+v, job %+v", fr, j)
	}
}

// TestHarvesterFinishesTheFieldThenCooks (HRV-001): Cook at 1, Haul at 3 and a
// bill waiting: the harvester clears the field, and only then cooks.
func TestHarvesterFinishesTheFieldThenCooks(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 8, 10, 12)
	placeBuilt(t, g, construction.Stove, construction.South, 3, 9)
	g.FuelCampfire(3, 9)
	spawn(t, g, item.Rice, 10, 3, 2)
	applyNow(g, command.Command{Kind: command.AddBill, X: 3, Y: 9, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 1})
	setPrio(g, id, command.WorkCook, 4) // no bill before the first bush
	firstHarvestDone(t, g, id, bushes)
	setPrio(g, id, command.WorkCook, 1)
	cooking := false
	for i := 0; i < 20000 && !cooking; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Bill || j.Kind == job.HaulIngredients || j.Kind == job.PackIngredients {
			if n := harvestedOf(g, bushes); n != len(bushes) {
				t.Fatalf("tick %d: cooking after %d of %d bushes: %+v", i, n, len(bushes), j)
			}
			cooking = true
		}
	}
	if !cooking {
		t.Fatal("the bill was never started")
	}
}

// TestHarvesterFinishesTheFieldThenRefines (HRV-001): Doctor at 1 and a bench bill
// waiting: a bill is not a patient, so the harvester clears the field first.
func TestHarvesterFinishesTheFieldThenRefines(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 8, 10, 12)
	placeBuilt(t, g, construction.Workbench, construction.South, 3, 9)
	spawn(t, g, item.HerbalMedicine, 4, 3, 2)
	applyNow(g, command.Command{Kind: command.AddBill, X: 3, Y: 9, Recipe: uint8(recipe.RefineMedicine), Mode: uint8(recipe.DoCount), Count: 1})
	setSkill(g, id, skill.Medicine, 8)
	setPrio(g, id, command.WorkDoctor, 4) // no bill before the first bush
	firstHarvestDone(t, g, id, bushes)
	setPrio(g, id, command.WorkDoctor, 1)
	refining := false
	for i := 0; i < 20000 && !refining; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Bill || j.Kind == job.HaulIngredients || j.Kind == job.PackIngredients {
			if n := harvestedOf(g, bushes); n != len(bushes) {
				t.Fatalf("tick %d: refining after %d of %d bushes: %+v", i, n, len(bushes), j)
			}
			refining = true
		}
	}
	if !refining {
		t.Fatal("the bill was never started")
	}
}

// TestFireInterruptsTheSweep (HRV-001): a fire in Home takes the picker off
// the field with its mark on; when it is out the sweep goes on.
func TestFireInterruptsTheSweep(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 8, 10, 12)
	firstHarvestDone(t, g, id, bushes)
	g.paintHome(0, 0, 39, 11, true)
	burn(g, 7, 8, 30)
	j := untilJob(t, g, id, 1000, func(j job.Record) bool { return j.Kind == job.FightFire })
	if !g.haul.sweep.Armed(id) {
		t.Fatalf("the fire took the mark: %+v", j)
	}
	if n := harvestedOf(g, bushes); n >= len(bushes) {
		t.Fatalf("all %d bushes picked before the fire", n)
	}
	for i := 0; i < 4000 && g.anyFire(); i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
	}
	if g.anyFire() {
		t.Fatal("the fire never went out")
	}
	sweepAll(t, g, id, bushes)
}

// TestFireFarAwayEndsTheSweep (HRV-001): a fire that takes the picker past
// Radius of the field ends the sweep the next time it looks: no walk back.
func TestFireFarAwayEndsTheSweep(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6)
	setPrio(g, id, command.WorkPlantCut, 4)
	setPrio(g, id, command.WorkHaul, 1)
	setPrio(g, id, command.WorkDoctor, 0) // a burn tended on the spot is not the point here
	firstHarvestDone(t, g, id, bushes)
	g.paintHome(0, 0, 39, 11, true)
	burn(g, 36, 5, 30)
	untilJob(t, g, id, 1000, func(j job.Record) bool { return j.Kind == job.FightFire })
	for i := 0; i < 4000 && g.anyFire(); i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
	}
	if g.anyFire() {
		t.Fatal("the fire never went out")
	}
	j := untilJob(t, g, id, 2000, func(j job.Record) bool { return j.Kind == job.Harvest || j.Kind == job.Haul })
	if j.Kind != job.Haul || g.haul.sweep.Armed(id) {
		t.Fatalf("job %+v, armed %v: the sweep should be over", j, g.haul.sweep.Armed(id))
	}
}

// TestSweepSkipsAPlantOutsideTheArea (HRV-002): the nearest ripe plant lies
// outside the picker's allowed area: the sweep goes past it to the next one.
func TestSweepSkipsAPlantOutsideTheArea(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 10)
	setPrio(g, id, command.WorkHaul, 1)
	applyNow(g, command.Command{Kind: command.CreateArea})
	applyNow(g, command.Command{Kind: command.PaintArea, Index: 1, X: 0, Y: 0, X2: 39, Y2: 11, Enabled: true})
	applyNow(g, command.Command{Kind: command.PaintArea, Index: 1, X: 6, Y: 5, X2: 6, Y2: 5, Enabled: false}) // not the second bush
	applyNow(g, command.Command{Kind: command.AssignArea, Pawn: uint64(id), Index: 1})
	firstHarvestDone(t, g, id, bushes)
	if !g.haul.sweep.Armed(id) {
		t.Fatal("not marked")
	}
	j := untilJob(t, g, id, 200, func(j job.Record) bool { return j.Kind == job.Harvest && j.Target != bushes[0] || j.Kind == job.Haul })
	if j.Target != bushes[2] {
		t.Fatalf("job %+v, want the bush inside the area", j)
	}
	if !g.haul.sweep.Armed(id) {
		t.Fatal("the plant outside ended the sweep")
	}
	if p, _ := g.plants.Get(bushes[1]); !p.Designated {
		t.Fatal("the bush outside the area was picked")
	}
}

// harvestScene is a picker at (2, 5) with only the given work on, a ripe
// marked wild bush at (6, 5) and a ripe potato on a grow zone at (8, 5).
func harvestScene(t *testing.T, types ...work.Type) (*Game, entity.EntityID, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	for _, pid := range append([]entity.EntityID(nil), g.plantIDs...) {
		g.removePlant(pid)
	}
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	setSkill(g, id, skPlants, 12)
	onlyWork(g, id, types...)
	bush, err := g.spawnPlant(plant.BerryBush, 6, 5, plant.DefOf(plant.BerryBush).GrowTicks)
	if err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 6, Y: 5, X2: 6, Y2: 5})
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: 8, Y: 5, X2: 8, Y2: 5, Build: uint8(plant.Potato)})
	g.ApplyPending()
	crop, err := g.spawnPlant(plant.Potato, 8, 5, plant.DefOf(plant.Potato).GrowTicks)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := g.plants.Get(crop)
	p.Zone, p.Designated = true, true
	g.setPlant(crop, p)
	g.notePlantMark(crop)
	return g, id, bush, crop
}

// TestGrowHarvestsCropsAndCutHarvestsWild (HRV-003): only Grow on leaves the
// wild bush standing, only Plant cut on leaves the zone crop.
func TestGrowHarvestsCropsAndCutHarvestsWild(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		on   work.Type
		take bool // the crop is harvested, not the bush
	}{{"grow", work.Grow, true}, {"cut", work.PlantCut, false}} {
		g, id, bush, crop := harvestScene(t, c.on)
		for range 3000 {
			runFed(g, 1, id)
		}
		bp, _ := g.plants.Get(bush)
		_, cropLeft := g.plants.Get(crop)
		if c.take && (!bp.Designated || cropLeft) {
			t.Errorf("%s: bush still marked %v, crop still there %v", c.name, bp.Designated, cropLeft)
		}
		if !c.take && (bp.Designated || !cropLeft) {
			t.Errorf("%s: bush still marked %v, crop still there %v", c.name, bp.Designated, cropLeft)
		}
	}
}

// TestPlantCutTakesWildBeforeCutting (HRV-003): the Plant cut slot harvests a
// marked wild plant before a marked cut, nearer or not.
func TestPlantCutTakesWildBeforeCutting(t *testing.T) {
	t.Parallel()
	g, id, _, _ := harvestScene(t, work.PlantCut)
	if _, err := g.spawnPlant(plant.Pine, 3, 5, plant.DefOf(plant.Pine).GrowTicks); err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: 3, Y: 5, X2: 3, Y2: 5})
	g.ApplyPending()
	g.Step(2)
	if j, _ := g.jobs.Get(id); j.Kind != job.Harvest {
		t.Fatalf("job %+v, want the wild harvest first", j)
	}
}

// TestForcedHarvestChecksTheRightWork (HRV-003): the order asks for Plant cut
// on a wild plant and Grow on a zone crop; a cut order names plant cutting.
func TestForcedHarvestChecksTheRightWork(t *testing.T) {
	t.Parallel()
	g, id, _, _ := harvestScene(t, work.Grow)
	if _, why := g.forcedJob(id, command.OrderHarvest, 6, 5, 0); why != ReasonNoPlantCut {
		t.Fatalf("wild with Plant cut off: %q", why)
	}
	if j, why := g.forcedJob(id, command.OrderHarvest, 8, 5, 0); why != "" || j.Kind != job.Harvest {
		t.Fatalf("crop with Grow on: %+v %q", j, why)
	}
	if _, why := g.forcedJob(id, command.OrderCut, 6, 5, 0); why != ReasonNoPlantCut {
		t.Fatalf("cut with Plant cut off: %q", why)
	}
	g, id, _, _ = harvestScene(t, work.PlantCut)
	if j, why := g.forcedJob(id, command.OrderHarvest, 6, 5, 0); why != "" || j.Kind != job.Harvest {
		t.Fatalf("wild with Plant cut on: %+v %q", j, why)
	}
	if _, why := g.forcedJob(id, command.OrderHarvest, 8, 5, 0); why != ReasonNoGrow {
		t.Fatalf("crop with Grow off: %q", why)
	}
}

// TestHarvestJobShowsItsWorkType (HRV-003): the dossier reads a wild plant's
// harvest as Plant cut and a crop's as Grow.
func TestHarvestJobShowsItsWorkType(t *testing.T) {
	t.Parallel()
	g, _, bush, crop := harvestScene(t, work.Grow)
	if got := workTypeOfJob(g, job.Record{Kind: job.Harvest, Target: bush}); got != int(work.PlantCut) {
		t.Fatalf("wild harvest reads as %d", got)
	}
	if got := workTypeOfJob(g, job.Record{Kind: job.Harvest, Target: crop}); got != int(work.Grow) {
		t.Fatalf("crop harvest reads as %d", got)
	}
}

// TestPlantWorkProgress (HRV-009): sowing, harvesting and cutting follow the
// wiki's Plant Work Speed; another skill at the same level keeps its curve.
func TestPlantWorkProgress(t *testing.T) {
	t.Parallel()
	for level, want := range map[uint8]int{0: 100, 7: 885, 8: 1000, 20: 2380} {
		g := New(Config{Seed: 3, Width: 12, Height: 8})
		id, _ := g.SpawnNamedPawn(2, 2, "Ada")
		setSkill(g, id, skill.Plants, level)
		got := 0
		for range 1000 {
			got += g.workProgress(id, skPlants, true)
			g.clock.Advance(1)
		}
		if got != want {
			t.Errorf("Plants %d: %d units in 1000 ticks, want %d", level, got, want)
		}
	}
	g := New(Config{Seed: 3, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	got := 0
	for range 1000 {
		got += g.workProgress(id, skill.Mining, true)
		g.clock.Advance(1)
	}
	if got != 1000 {
		t.Errorf("Mining 7: %d units in 1000 ticks, want 1000", got)
	}
}

// harvestFinished runs doHarvest on plant pid until it is done.
func harvestFinished(t *testing.T, g *Game, id, pid entity.EntityID) {
	t.Helper()
	for range 40 {
		j := job.Record{Kind: job.Harvest, State: job.Do, Target: pid, WorkLeft: 1}
		g.jobs.Set(id, j)
		g.doHarvest(id, j)
		if j, _ := g.jobs.Get(id); j.State == job.Complete {
			return
		}
		g.clock.Advance(1)
	}
	t.Fatal("the harvest never finished")
}

// TestWastedHarvestDropsNothing (HRV-006, HRV-010): at Plants 0 about 40 % of
// the harvests come to nothing, straw too, and the rest are whole.
func TestWastedHarvestDropsNothing(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 40, Height: 12})
	for _, pid := range append([]entity.EntityID(nil), g.plantIDs...) {
		g.removePlant(pid)
	}
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	setSkill(g, id, skPlants, 0)
	d := plant.DefOf(plant.Wheat)
	const n = 100
	for i := range n {
		pid, err := g.spawnPlant(plant.Wheat, 2+i%30, 2+i/30, d.GrowTicks)
		if err != nil {
			t.Fatal(err)
		}
		harvestFinished(t, g, id, pid)
		if !g.haul.sweep.Armed(id) {
			t.Fatal("the harvest did not mark the sweep")
		}
		if _, ok := g.plants.Get(pid); ok {
			t.Fatal("the plant stayed")
		}
	}
	wheat, straw := g.stockOf(item.Wheat), g.stockOf(item.Straw)
	whole := wheat / d.Yield
	if wheat%d.Yield != 0 || straw != whole*d.ExtraQty {
		t.Fatalf("%d wheat and %d straw: each harvest gives all %d and %d, or nothing", wheat, straw, d.Yield, d.ExtraQty)
	}
	if whole < 45 || whole > 75 {
		t.Fatalf("%d of %d harvests came through at a 40 %% waste chance", whole, n)
	}
}

// TestSkilledHarvestYieldsMore (HRV-005, HRV-007): Plants 20 gives 113 %, 8
// gives 100 %, and neither touches the generator.
func TestSkilledHarvestYieldsMore(t *testing.T) {
	t.Parallel()
	for level, want := range map[uint8]int{8: 100, 20: 113} {
		g := New(Config{Seed: 9, Width: 20, Height: 12})
		id, _ := g.SpawnNamedPawn(1, 1, "Ada")
		setSkill(g, id, skPlants, level)
		pid, _ := g.spawnPlant(plant.Corn, 3, 3, plant.DefOf(plant.Corn).GrowTicks)
		g2 := New(Config{Seed: 9, Width: 20, Height: 12})
		g2.SpawnNamedPawn(1, 1, "Ada")
		g2.spawnPlant(plant.Corn, 3, 3, plant.DefOf(plant.Corn).GrowTicks)
		harvestFinished(t, g, id, pid)
		if got := g.stockOf(item.Corn); got != plant.DefOf(plant.Corn).Yield*want/100 {
			t.Errorf("Plants %d: %d corn, want %d", level, got, plant.DefOf(plant.Corn).Yield*want/100)
		}
		if g.rng.IntN(1<<30) != g2.rng.IntN(1<<30) {
			t.Errorf("Plants %d: the harvest rolled", level)
		}
	}
}

// TestPlantYieldReadsTheBody (HRV-008): a lost hand lowers the yield the
// harvest uses, and the dossier's line follows.
func TestPlantYieldReadsTheBody(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	setSkill(g, id, skPlants, 20)
	if got := g.plantYield(id); got != 1130 {
		t.Fatalf("healthy at Plants 20: %d", got)
	}
	k := keyOf(g.bodies[id])
	if want := plant.HarvestYieldPermil(20, int(k[1])/10, int(k[2])/10); g.plantYield(id) != want {
		t.Fatalf("plantYield %d, want %d", g.plantYield(id), want)
	}
}

// TestCutGivesTheNewAmountWithoutARoll (HRV-007): a cut's crop never comes to
// nothing and never shrinks below 100 %.
func TestCutGivesTheNewAmountWithoutARoll(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	setSkill(g, id, skPlants, 0)
	pid, _ := g.spawnPlant(plant.Corn, 3, 3, plant.DefOf(plant.Corn).GrowTicks)
	for range 40 {
		j := job.Record{Kind: job.Cut, State: job.Do, Target: pid, WorkLeft: 1}
		g.jobs.Set(id, j)
		g.doCut(id, j)
		if j, _ := g.jobs.Get(id); j.State == job.Complete {
			break
		}
		g.clock.Advance(1)
	}
	if got := g.stockOf(item.Corn); got != plant.DefOf(plant.Corn).Yield {
		t.Fatalf("%d corn from a cut at Plants 0, want %d", got, plant.DefOf(plant.Corn).Yield)
	}
}

// TestHarvestWasteSameSeedSameHash: a poor picker's waste rolls come from the
// game's own generator: the same seed plays out the same.
func TestHarvestWasteSameSeedSameHash(t *testing.T) {
	t.Parallel()
	run := func() uint64 {
		g, id, bushes := sweepScene(t, 11, 4, 6, 8, 10, 12)
		setSkill(g, id, skPlants, 2)
		firstHarvestDone(t, g, id, bushes)
		g.Step(6000)
		return g.StateHash()
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("same seed: %x, %x", a, b)
	}
}

// TestCleanClearsSnowFirst (WRK-006): snow to shovel and filth to mop: snow.
func TestCleanClearsSnowFirst(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	g.SetWeather(weather.Clear, 0)
	g.paintHome(0, 0, g.grid.Width()-1, g.grid.Height()-1, true)
	g.addDirt(tileIdx(g, 4, 6), 300) // the nearer job
	g.paintSnowClear(8, 5, 8, 5, true)
	g.snowOf().SetCell(snowIdx(g, 8, 5), 200)
	if !g.assignWorkType(id, work.Clean) {
		t.Fatal("no Clean job")
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.ClearSnow {
		t.Fatalf("job %+v, want the snow first", j)
	}
}

// TestHandleOrder (WRK-005): slaughter, then milking, then taming.
func TestHandleOrder(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 24, Height: 14})
	ada, _ := g.SpawnNamedPawn(4, 7, "Ada")
	onlyWork(g, ada, work.Handle)
	setSkill(g, ada, skill.Animals, 8)
	goat, _ := g.SpawnAnimal("goat", 6, 7)
	spawn(t, g, item.Potatoes, 60, 4, 9)
	applyNow(g, command.Command{Kind: command.DesignateTame, X: 6, Y: 7, X2: 6, Y2: 7})
	bess := spawnSexed(t, g, "cow", 15, 10, true, true, "Bess")
	setAnimal(g, bess, func(a *animal.State) { a.Product = animal.ProductFull })
	bull := spawnSexed(t, g, "cow", 20, 10, false, true, "Bull")
	applyNow(g, command.Command{Kind: command.DesignateSlaughter, X: 20, Y: 10, X2: 20, Y2: 10, Enabled: true})
	first := func() job.Record {
		g.jobs.Set(ada, job.Record{})
		g.reserve.ReleaseOwner(ada)
		if !g.assignWorkType(ada, work.Handle) {
			t.Fatal("no Handle job")
		}
		j, _ := g.jobs.Get(ada)
		return j
	}
	if j := first(); j.Kind != job.Slaughter || j.Target != bull {
		t.Fatalf("job %+v, want the slaughter first", j)
	}
	setAnimal(g, bull, func(a *animal.State) { a.Designated = false })
	if j := first(); j.Kind != job.Milk || j.Target != bess {
		t.Fatalf("job %+v, want the milking before the taming", j)
	}
	setAnimal(g, bess, func(a *animal.State) { a.Product = 0 })
	if j := first(); j.Kind != job.Tame || j.Target != goat {
		t.Fatalf("job %+v, want the taming last", j)
	}
}

// TestConstructOrder (WRK-004): a planned roof before a frame ready to build,
// the nearer one or not.
func TestConstructOrder(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	if !g.BuildNow("wall", "wood", "south", 2, 5) {
		t.Fatal("wall")
	}
	id, _ := g.SpawnNamedPawn(16, 5, "Ada")
	onlyWork(g, id, work.Construct)
	g.designateRoof(4, 5, 4, 5, true)
	woodWalls(g, 18, 5, 1)
	fid := g.frameIDs[0]
	fr, _ := g.frames.Get(fid)
	fr.Wood = fr.Need
	g.frames.Set(fid, fr)
	if !g.assignWorkType(id, work.Construct) {
		t.Fatal("no Construct job")
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.Roof {
		t.Fatalf("job %+v, want the roof first", j)
	}
}

// TestHaulOrder (WRK-007): refuelling before a hopper, the nearer one or not.
func TestHaulOrder(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	buildNow(t, g, construction.Hopper, item.Steel, construction.South, 4, 4)
	spawn(t, g, item.Potatoes, 40, 6, 4)
	placeBuilt(t, g, construction.Stove, construction.South, 20, 8)
	spawn(t, g, item.Wood, 20, 19, 10)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	onlyWork(g, id, work.Haul)
	if !g.assignWorkType(id, work.Haul) {
		t.Fatal("no Haul job")
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.Refuel {
		t.Fatalf("job %+v, want the refuel first", j)
	}
	g.jobs.Set(id, job.Record{})
	g.reserve.ReleaseOwner(id)
	g.deleteItem(g.itemAt(19, 10)) // no fuel left: the hopper
	if !g.assignWorkType(id, work.Haul) {
		t.Fatal("no Haul job")
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.FillHopper {
		t.Fatalf("job %+v, want the hopper", j)
	}
}
