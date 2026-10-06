package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestMovePawnFollowsPath(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, err := g.SpawnNamedPawn(1, 1, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 6, Y: 1})
	g.Step(TicksPerCell * 20)
	p := pawn(t, g, id)
	if p.X != 6 || p.Y != 1 {
		t.Fatalf("pawn at (%d,%d), want (6,1) job=%s", p.X, p.Y, p.Job)
	}
	if p.Name != "Ada" {
		t.Fatalf("name = %q", p.Name)
	}
	if p.Job != "idle" {
		t.Fatalf("job after arrive = %s, want idle", p.Job)
	}
}

func TestMovePawnBlockedIsUnreachable(t *testing.T) {
	g := New(Config{Seed: 1, Width: 6, Height: 3})
	id, err := g.SpawnPawn(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 3; y++ {
		g.grid.SetWalkable(3, y, false)
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 5, Y: 1})
	g.Step(2)
	p := pawn(t, g, id)
	if p.X != 0 || p.Y != 1 {
		t.Fatalf("should stay put, at (%d,%d)", p.X, p.Y)
	}
}

func TestEatRestoresHunger(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	id, err := g.SpawnNamedPawn(2, 2, "Bo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Food, 4, 4, 2); err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.CriticalHunger, 9000)
	g.Step(TicksPerCell*20 + needs.EatWorkTicks + 5)
	if !g.Ate(id) {
		t.Fatalf("pawn did not eat, job=%s hunger=%d", pawn(t, g, id).Job, pawn(t, g, id).Hunger)
	}
	if pawn(t, g, id).Hunger <= needs.Percent(needs.CriticalHunger) {
		t.Fatalf("hunger percent %d, expected restored", pawn(t, g, id).Hunger)
	}
}

func TestFirstPlayableHaulBedEatSleep(t *testing.T) {
	g := NewColony(Config{Seed: 1, Width: 20, Height: 16})
	for _, p := range g.Snapshot() {
		g.SetNeeds(p.ID, 4000, 8500)
	}
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 10, Y: 5, X2: 12, Y2: 7})
	g.Enqueue(command.Command{Kind: command.DesignateBed, X: 6, Y: 7}) // inside the open start area
	g.Step(80000)

	world := g.World()
	bedDone := false
	for _, b := range world.Buildings {
		if b.Kind == "bed" {
			bedDone = true
		}
	}
	if !bedDone {
		t.Fatalf("expected a completed bed, frames=%d buildings=%d", len(world.Frames), len(world.Buildings))
	}
	foodInStock := false
	for _, it := range world.Items {
		if it.Kind == "food" && !it.Carried {
			for _, z := range world.Stockpiles {
				if it.X >= z.MinX && it.X <= z.MaxX && it.Y >= z.MinY && it.Y <= z.MaxY {
					foodInStock = true
				}
			}
		}
	}
	if !foodInStock {
		t.Fatalf("expected food in a stockpile, items=%v zones=%v", world.Items, world.Stockpiles)
	}
	ate, slept := false, false
	for _, p := range g.Snapshot() {
		if p.Ate {
			ate = true
		}
		if p.SleptInBed {
			slept = true
		}
	}
	if !ate {
		t.Fatal("expected a pawn to eat")
	}
	if !slept {
		t.Fatal("expected a pawn to sleep in the bed")
	}
}

func TestHarvestYieldsFood(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	id, err := g.SpawnNamedPawn(2, 2, "Cora")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, 8000, 9000)
	if _, err := g.SpawnPlant(4, 2); err != nil {
		t.Fatal(err)
	}
	before := g.World().Food
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 4, Y: 2, X2: 4, Y2: 2})
	g.Step(TicksPerCell*20 + 80)
	if g.World().Food <= before {
		t.Fatalf("food %d, want more than %d; plants=%v items=%v job=%s", g.World().Food, before, g.World().Plants, g.World().Items, pawn(t, g, id).Job)
	}
}

func TestStartKitHasNamedPawnsFoodWoodPlants(t *testing.T) {
	g := NewColony(Config{Seed: 1, Width: 20, Height: 16})
	snap := g.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("pawns = %d, want 3", len(snap))
	}
	want := map[string]bool{"Ada": true, "Bo": true, "Cora": true}
	for _, p := range snap {
		if !want[p.Name] {
			t.Fatalf("unexpected name %q", p.Name)
		}
		delete(want, p.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing names %v", want)
	}
	w := g.World()
	if w.Food < 1 || w.Wood < 1 || len(w.Plants) < 1 {
		t.Fatalf("start kit food=%d wood=%d plants=%d", w.Food, w.Wood, len(w.Plants))
	}
}

func TestCompletedWallBlocksWalkability(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, err := g.SpawnNamedPawn(1, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Wood, 10, 2, 2); err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, 8000, 9000)
	g.Enqueue(command.Command{Kind: command.DesignateWall, X: 4, Y: 2})
	g.Step(TicksPerCell*40 + 200)
	if g.grid.Walkable(4, 2) {
		t.Fatalf("wall tile still walkable; buildings=%v frames=%v job=%s", g.World().Buildings, g.World().Frames, pawn(t, g, id).Job)
	}
}

func TestExhaustionForcesSleepOnGround(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	id, err := g.SpawnNamedPawn(2, 2, "Bo")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, 8000, 0)
	g.Step(1)
	p := pawn(t, g, id)
	if p.Status != "downed" {
		t.Fatalf("status=%s, want downed", p.Status)
	}
	if p.Job == "idle" {
		t.Fatalf("exhausted pawn should sleep, job=%s status=%s", p.Job, p.Status)
	}
	if p.Reason != "no bed" {
		t.Fatalf("inspector reason=%q, want no bed", p.Reason)
	}
	g.Step(needs.CriticalRest/needs.SleepGain + 10)
	p = pawn(t, g, id)
	if p.Rest < needs.Percent(needs.CriticalRest) {
		t.Fatalf("rest still %d after forced sleep", p.Rest)
	}
	if p.Status != "alive" {
		t.Fatalf("status=%s after rest recovered, want alive", p.Status)
	}
}

func TestNoFoodReachesDownedThenDeath(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	id, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, 8, 9000)
	g.Step(needs.HungerPeriod*20 + 50)
	v := g.VitalOf(id)
	if v.Status != health.Downed {
		t.Fatalf("status=%s reason=%s, want downed", v.Status, v.Cause)
	}
	if v.Cause != health.CauseNoFood {
		t.Fatalf("cause=%s, want no food", v.Cause)
	}
	g.Step(health.StarvationTicks + 5)
	v = g.VitalOf(id)
	if v.Status != health.Dead {
		t.Fatalf("status=%s, want dead", v.Status)
	}
	p := pawn(t, g, id)
	if p.Reason != "no food" {
		t.Fatalf("inspector reason=%q", p.Reason)
	}
}

func TestStarvationDownsWhileSleeping(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	id, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, 5, needs.CriticalRest)
	g.Step(needs.HungerPeriod*10 + needs.SleepWorkMin)
	v := g.VitalOf(id)
	if v.Status != health.Downed {
		t.Fatalf("status=%s cause=%s job=%s, want downed while/after sleep", v.Status, v.Cause, pawn(t, g, id).Job)
	}
	if v.Cause != health.CauseNoFood {
		t.Fatalf("cause=%s, want no food", v.Cause)
	}
}

func TestDeadPawnReleasesBed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, err := g.SpawnNamedPawn(1, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Wood, 10, 2, 2); err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, 8000, 9000)
	g.Enqueue(command.Command{Kind: command.DesignateBed, X: 4, Y: 2})
	g.Step(TicksPerCell*80 + 400)
	g.SetNeeds(id, 8000, needs.CriticalRest)
	g.Step(TicksPerCell*20 + needs.SleepWorkMin + 5)
	if !g.SleptInBed(id) {
		t.Fatalf("expected bed claim before death; buildings=%v job=%s", g.World().Buildings, pawn(t, g, id).Job)
	}
	g.SetNeeds(id, 0, 5000)
	g.Step(2)
	g.Step(health.StarvationTicks + 5)
	if g.VitalOf(id).Status != health.Dead {
		t.Fatalf("status=%s, want dead", g.VitalOf(id).Status)
	}
	for _, b := range g.World().Buildings {
		if b.Kind == "bed" && b.Claimant != 0 {
			t.Fatalf("dead pawn still claims bed %+v", b)
		}
	}
}

func pawn(t *testing.T, g *Game, id entity.EntityID) PawnView {
	t.Helper()
	for _, p := range g.Snapshot() {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("pawn %d missing", id)
	return PawnView{}
}
