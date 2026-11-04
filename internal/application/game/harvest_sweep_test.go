package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// HAUL-003: a colonist who finishes a harvest goes on harvesting every
// ripe marked plant within haul.Radius of where it stands before it
// hauls anything.

// sweepScene is a 40x12 map with a skilled grower at (2, 5) on full needs,
// a stockpile at (30..32, 9..10) and a ripe berry bush marked for harvest
// on row 5 at each of xs.
func sweepScene(t *testing.T, seed uint64, xs ...int) (*Game, entity.EntityID, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 40, Height: 12})
	for _, pid := range append([]entity.EntityID(nil), g.plantIDs...) {
		g.removePlant(pid) // a clean field: no generated plants
	}
	id, err := g.SpawnNamedPawn(2, 5, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	setSkill(g, id, skill.Plants, 12) // no fumbles: every harvest leaves a pile to haul
	zoneCmd(g, command.DesignateStockpile, 30, 9, 32, 10, 0)
	bushes := make([]entity.EntityID, 0, len(xs))
	for _, x := range xs {
		pid, err := g.SpawnPlantSpecies("berry_bush", x, 5, plant.DefOf(plant.BerryBush).GrowTicks)
		if err != nil {
			t.Fatal(err)
		}
		bushes = append(bushes, pid)
		g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: x, Y: 5, X2: x, Y2: 5})
	}
	g.Step(1)
	return g, id, bushes
}

// setPrio sets pawn id's priority for work w (0: off) at once.
func setPrio(g *Game, id entity.EntityID, w command.WorkType, v int) {
	applyNow(g, command.Command{Kind: command.SetWorkPriority, Pawn: uint64(id), Work: w, Count: v})
}

// harvestedOf counts the bushes picked (a picked bush regrows unmarked).
func harvestedOf(g *Game, bushes []entity.EntityID) int {
	n := 0
	for _, pid := range bushes {
		if p, ok := g.plants.Get(pid); !ok || !p.Designated {
			n++
		}
	}
	return n
}

// untilJob steps g until pawn id's job satisfies ok, at most limit ticks.
func untilJob(t *testing.T, g *Game, id entity.EntityID, limit int, ok func(job.Record) bool) job.Record {
	t.Helper()
	for range limit {
		if j, _ := g.jobs.Get(id); ok(j) {
			return j
		}
		g.Step(1)
	}
	j, _ := g.jobs.Get(id)
	t.Fatalf("after %d ticks the job is %+v", limit, j)
	return j
}

// firstHarvestDone steps until the first bush is picked and the grower is
// marked for the sweep.
func firstHarvestDone(t *testing.T, g *Game, id entity.EntityID, bushes []entity.EntityID) {
	t.Helper()
	for range 4000 {
		if harvestedOf(g, bushes) > 0 {
			return
		}
		g.Step(1)
	}
	t.Fatal("the first bush was never picked")
}

// sweepAll runs the five-bush field until every berry is stored; it fails
// on a Haul job before the fifth bush is picked.
func sweepAll(t *testing.T, g *Game, id entity.EntityID, bushes []entity.EntityID) {
	t.Helper()
	for i := 0; i < 20000 && storedIn(g, "berries", 30, 9, 32, 10) == 0; i++ {
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Haul && harvestedOf(g, bushes) < len(bushes) {
			t.Fatalf("tick %d: hauling after %d of %d bushes", i, harvestedOf(g, bushes), len(bushes))
		}
		g.Step(1)
	}
	if n := harvestedOf(g, bushes); n != len(bushes) {
		t.Fatalf("picked %d of %d bushes", n, len(bushes))
	}
	if storedIn(g, "berries", 30, 9, 32, 10) == 0 {
		t.Fatal("no berries reached the stockpile")
	}
	if g.haul.sweep.Armed(id) {
		t.Fatal("the grower is still marked after the sweep")
	}
}

// TestHarvestSweepPicksTheFieldBeforeHauling: five ripe marked bushes and
// a stockpile; the grower picks all five before the first haul.
func TestHarvestSweepPicksTheFieldBeforeHauling(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 8, 10, 12)
	sweepAll(t, g, id, bushes)
}

// TestHarvestSweepSkipsAFarPlant: with hauling at priority 1, a ripe bush 20
// tiles off is left for later; the grower hauls the first pile first.
func TestHarvestSweepSkipsAFarPlant(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 26)
	setPrio(g, id, command.WorkHaul, 1) // Plant cut comes before Haul at equal priority (WRK-001)
	firstHarvestDone(t, g, id, bushes)
	j := untilJob(t, g, id, 2000, func(j job.Record) bool { return j.Kind == job.Haul || j.Kind == job.Harvest && j.Target == bushes[1] })
	if j.Kind != job.Haul {
		t.Fatalf("job %+v, want the haul before the far bush", j)
	}
	if g.haul.sweep.Armed(id) {
		t.Fatal("the mark stayed with no bush in reach")
	}
}

// TestHarvestSweepNeedsTheWork: a picker whose Plant cut work, which wild
// bushes are (HRV-003), is turned off while it picks a prioritized bush does
// not go on to the next one.
func TestHarvestSweepNeedsTheWork(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6)
	setPrio(g, id, command.WorkHaul, 0) // nothing else to do: the second bush is left alone
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(id), X: 4, Y: 5, Order: command.OrderHarvest})
	if j, _ := g.jobs.Get(id); j.Kind != job.Harvest || !j.Forced || j.Target != bushes[0] {
		t.Fatalf("forced job %+v, want the harvest of the first bush", j)
	}
	setPrio(g, id, command.WorkPlantCut, 0)
	setPrio(g, id, command.WorkHaul, 3)
	firstHarvestDone(t, g, id, bushes)
	for range 2000 {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.Harvest {
			t.Fatalf("a picker with Plant cut off picked again: %+v", j)
		}
	}
	if g.haul.sweep.Armed(id) {
		t.Fatal("the mark stayed with Plant cut off")
	}
}

// TestHarvestSweepHungerComesFirst: a grower who gets hungry mid-sweep
// eats before the next bush.
func TestHarvestSweepHungerComesFirst(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 8)
	if _, err := g.SpawnStack(item.SurvivalMeal, 5, 2, 8); err != nil {
		t.Fatal(err)
	}
	firstHarvestDone(t, g, id, bushes)
	if !g.haul.sweep.Armed(id) {
		t.Fatal("the grower was not marked by the harvest")
	}
	g.SetNeeds(id, needs.EatAt-1, needs.Full)
	j := untilJob(t, g, id, 200, func(j job.Record) bool { return j.Kind.Eating() || j.Kind == job.Harvest && j.Target != bushes[0] })
	if !j.Kind.Eating() {
		t.Fatalf("job %+v, want a hungry grower to eat first", j)
	}
}

// TestHarvestSweepDraftStops: drafting a grower mid-sweep stops it.
func TestHarvestSweepDraftStops(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 8)
	firstHarvestDone(t, g, id, bushes)
	applyNow(g, command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	picked := harvestedOf(g, bushes)
	for range 1500 {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.Harvest && j.State == job.Do {
			t.Fatalf("a drafted grower harvests: %+v", j)
		}
	}
	if n := harvestedOf(g, bushes); n != picked {
		t.Fatalf("picked %d while drafted, want %d", n, picked)
	}
}

// TestHarvestSweepBeatsAnUrgentHaul: with Haul at priority 1 and Plant cut at
// 3, a sweep already started still picks the whole field first.
func TestHarvestSweepBeatsAnUrgentHaul(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4, 6, 8, 10, 12)
	setPrio(g, id, command.WorkHaul, 1)
	setPrio(g, id, command.WorkPlantCut, 3)
	sweepAll(t, g, id, bushes)
}

// TestHarvestSweepNoFlap: once the mark is gone, a bush that ripens
// during the haul does not interrupt it (AGENTS.md hysteresis rule).
func TestHarvestSweepNoFlap(t *testing.T) {
	t.Parallel()
	g, id, bushes := sweepScene(t, 1, 4)
	late, err := g.SpawnPlantSpecies("berry_bush", 6, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	firstHarvestDone(t, g, id, bushes)
	untilJob(t, g, id, 2000, func(j job.Record) bool { return j.Kind == job.Haul })
	if g.haul.sweep.Armed(id) {
		t.Fatal("the mark stayed into the haul")
	}
	p, _ := g.plants.Get(late) // it ripens and is marked mid-haul
	p.Growth, p.Designated = plant.DefOf(plant.BerryBush).GrowTicks, true
	g.setPlant(late, p)
	g.notePlantMark(late)
	for i := 0; storedIn(g, "berries", 30, 9, 32, 10) == 0; i++ {
		if j, _ := g.jobs.Get(id); j.Kind != job.Haul {
			t.Fatalf("tick %d: the haul gave way to %+v", i, j)
		}
		if i > 4000 {
			t.Fatal("the haul never delivered")
		}
		g.Step(1)
	}
}

// sweepRun plays the five-bush sweep for a fixed time, saving and loading
// mid-sweep when reload, and returns the final state hash.
func sweepRun(t *testing.T, reload bool) uint64 {
	t.Helper()
	g, id, bushes := sweepScene(t, 7, 4, 6, 8, 10, 12)
	firstHarvestDone(t, g, id, bushes)
	if !g.haul.sweep.Armed(id) {
		t.Fatal("the grower was not marked by the harvest")
	}
	if reload {
		before := g.StateHash()
		g = saveLoad(t, g)
		if !g.haul.sweep.Armed(id) {
			t.Fatal("the mark was lost in the save")
		}
		if g.StateHash() != before {
			t.Fatal("a save and load moved the hash")
		}
	}
	g.Step(3000)
	return g.StateHash()
}

// TestHarvestSweepSameSeedAndSaveHash: the sweep is deterministic, and a
// save mid-sweep plays on exactly as the original.
func TestHarvestSweepSameSeedAndSaveHash(t *testing.T) {
	t.Parallel()
	x := sweepRun(t, false)
	if y := sweepRun(t, false); x != y {
		t.Fatalf("same seed: %x, %x", x, y)
	}
	if z := sweepRun(t, true); x != z {
		t.Fatalf("a save moved it: %x, %x", x, z)
	}
}
