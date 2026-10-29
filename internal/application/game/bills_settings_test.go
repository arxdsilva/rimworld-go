package game

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PRD-1: bills gather ingredients from several stacks, and their settings
// (store mode, radius, ingredient filter, skill range, worker, resume
// level, order) steer who makes what from what and where it goes.

// clubGame is a crafter at (1, 1), a workbench at (5, 2) (work spot
// (5, 3)) with a "make one club" bill, on a 20×12 map.
func clubGame(t *testing.T) (g *Game, crafter, bench entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 1, Width: 20, Height: 12})
	crafter, _ = g.SpawnNamedPawn(1, 1, "Ada")
	bench = buildNow(t, g, construction.Workbench, item.Wood, construction.South, 5, 2)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	if len(g.craft.Bills.Of(bench)) != 1 {
		t.Fatal("no club bill")
	}
	return g, crafter, bench
}

func crafterMask() work.Mask { return work.Mask(0).Set(work.Craft, true) }

// billCmd is a bill-settings command on bill i of the bench at (5, 2).
func billCmd(k command.Kind, i int) command.Command {
	return command.Command{Kind: k, X: 5, Y: 2, Index: i}
}

// runBillTicks steps g (needs kept full) until done or n ticks, noting each
// job kind the pawn held.
func runBillTicks(g *Game, pawn entity.EntityID, n int, done func() bool) map[job.Kind]bool {
	seen := map[job.Kind]bool{}
	for i := 0; i < n && !done(); i++ {
		if i%100 == 0 {
			g.SetNeeds(pawn, needs.Full, needs.Full)
		}
		g.Step(1)
		j, _ := g.jobs.Get(pawn)
		seen[j.Kind] = true
	}
	return seen
}

func TestBillGathersWoodFromSeveralStacks(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	g.SpawnStack(item.Wood, 2, 9, 6)
	g.SpawnStack(item.Wood, 1, 11, 8)
	g.SpawnStack(item.Wood, 2, 2, 8)
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	seen := runBillTicks(g, ada, 6000, done)
	if !done() {
		t.Fatal("no club from wood in three small stacks")
	}
	if !seen[job.HaulIngredients] {
		t.Fatal("the crafter never gathered ingredients")
	}
	if g.stockOf(item.Club) != 1 || g.stockOf(item.Wood) != 1 {
		t.Fatalf("clubs %d wood %d", g.stockOf(item.Club), g.stockOf(item.Wood))
	}
}

// storedClub makes a club with store mode st (zone z) beside two
// stockpiles, A at (12..13, 1..2) and B at (12..13, 8..9), and returns
// the game, the job the crafter takes as the club is done, and the two
// zones' ids.
func storedClub(t *testing.T, st recipe.StoreMode, toB bool) (*Game, job.Record, uint64) {
	t.Helper()
	g, ada, bench := clubGame(t)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 1, X2: 13, Y2: 2})
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 8, X2: 13, Y2: 9})
	g.SpawnStack(item.Wood, 10, 3, 5)
	g.Step(1)
	zb := zoneID(t, g, 12, 8)
	cfg := billCmd(command.ConfigureBill, 0)
	cfg.Store = uint8(st)
	if toB {
		cfg.Zone = zb
	}
	g.Enqueue(cfg)
	var after job.Record
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	runBillTicks(g, ada, 6000, done)
	if !done() {
		t.Fatalf("store %v: no club", st)
	}
	after, _ = g.jobs.Get(ada)
	return g, after, zb
}

func TestBillStoreModes(t *testing.T) {
	t.Parallel()
	g, j, _ := storedClub(t, recipe.BestStockpile, false)
	if j.Kind != job.Haul || g.stacks.Ref(j.Target) == nil || g.stacks.Ref(j.Target).Kind != item.Club {
		t.Fatalf("best stockpile: the crafter carries the club off (job %+v)", j)
	}
	if !g.inMatchingStockpileAt(item.Club, j.DestX, j.DestY) {
		t.Fatalf("taken to %d,%d, not a stockpile", j.DestX, j.DestY)
	}
	_, j, _ = storedClub(t, recipe.DropOnFloor, false)
	if j.Kind == job.Haul {
		t.Fatalf("drop on floor: the crafter hauled it (%+v)", j)
	}
	g, j, zb := storedClub(t, recipe.StockpileZone, true)
	if j.Kind != job.Haul || zoneID(t, g, j.DestX, j.DestY) != zb {
		t.Fatalf("specific stockpile: job %+v", j)
	}
}

func TestBillRadiusAndIngredientFilter(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	g.SpawnStack(item.Wood, 10, 17, 10)
	cfg := billCmd(command.ConfigureBill, 0)
	cfg.Radius = 4
	g.Enqueue(cfg)
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	runBillTicks(g, ada, 600, done)
	if done() {
		t.Fatal("took wood from outside the radius")
	}
	if v := g.World().Buildings[0].Bills[0]; v.Status != BillNoInput || v.Radius != 4 {
		t.Fatalf("view %+v", v)
	}
	forbid := billCmd(command.SetBillIngredient, 0)
	forbid.Stuff = uint8(item.Wood)
	g.Enqueue(forbid)
	g.SpawnStack(item.Wood, 10, 6, 5)
	runBillTicks(g, ada, 600, done)
	if done() {
		t.Fatal("used forbidden wood")
	}
	if v := g.World().Buildings[0].Bills[0]; v.Ingredients.Len() != 1 || v.Ingredients.At(0).Allowed {
		t.Fatalf("ingredients %+v", v.Ingredients)
	}
	forbid.Enabled = true
	g.Enqueue(forbid)
	runBillTicks(g, ada, 6000, done)
	if !done() {
		t.Fatal("no club once wood is allowed")
	}
}

func TestBillWorkerAndSkillRange(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	bo, _ := g.SpawnNamedPawn(2, 1, "Bo")
	g.SpawnStack(item.Wood, 10, 3, 5)
	cfg := billCmd(command.ConfigureBill, 0)
	cfg.Worker = uint64(bo)
	g.Enqueue(cfg)
	g.Step(1)
	if v := g.World().Buildings[0].Bills[0]; v.Worker != uint64(bo) || v.WorkerName != "Bo" || v.NoWorker != "" {
		t.Fatalf("view %+v", v)
	}
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	for i := 0; i < 6000 && !done(); i++ {
		if i%100 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
			g.SetNeeds(bo, needs.Full, needs.Full)
		}
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Bill || j.Kind == job.HaulIngredients {
			t.Fatal("Ada took Bo's bill")
		}
	}
	if !done() {
		t.Fatal("Bo never made the club")
	}

	g, ada, _ = clubGame(t)
	cfg.Worker, cfg.SkillMin = 0, 20
	g.Enqueue(cfg)
	g.Step(1)
	if v := g.World().Buildings[0].Bills[0]; v.NoWorker != craft.NoWorkerNone || v.SkillMin != 20 {
		t.Fatalf("skill 20 wanted: %+v", v)
	}
	if g.craft.Shop.Assign(ada, crafterMask()) {
		t.Fatal("an unskilled crafter took the bill")
	}
}

func TestReorderBillCommand(t *testing.T) {
	t.Parallel()
	g, _, bench := clubGame(t)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Spear), Mode: uint8(recipe.Forever)})
	g.Step(1)
	r := billCmd(command.ReorderBill, 1)
	r.Count = -1
	g.Enqueue(r)
	g.Step(1)
	if bl := g.craft.Bills.Of(bench); bl[0].Recipe != recipe.Spear || bl[1].Recipe != recipe.Club {
		t.Fatalf("order %v", bl)
	}
	if v := g.World().Buildings[0].Bills; v[0].Label != "Make wooden spear" {
		t.Fatalf("views %+v", v)
	}
}

func TestBillSettingsSurviveSaveAndLoad(t *testing.T) {
	t.Parallel()
	g, _, bench := clubGame(t)
	cfg := billCmd(command.ConfigureBill, 0)
	cfg.Store, cfg.Zone, cfg.Radius, cfg.SkillMin, cfg.SkillMax, cfg.Worker, cfg.UnpauseAt = 2, 7, 9, 3, 15, 42, 1
	g.Enqueue(cfg)
	forbid := billCmd(command.SetBillIngredient, 0)
	forbid.Stuff = uint8(item.Wood)
	g.Enqueue(forbid)
	g.Step(1)
	want := g.craft.Bills.Of(bench)[0]
	got := saveLoad(t, g).craft.Bills.Of(bench)[0]
	if got.Store != recipe.StockpileZone || got.Zone != 7 || got.Radius != 9 || got.SkillMin != 3 || got.SkillMax != 15 ||
		got.Worker != 42 || got.UnpauseAt != 1 || len(got.Forbid) != 1 || got.Forbid[0] != item.Wood || got.SearchAt != want.SearchAt {
		t.Fatalf("loaded %+v, saved %+v", got, want)
	}
}

// TestBillWithoutIngredientsDoesNotRescanEveryTick: the PRD-1 retry
// cooldown in a whole game — a bill with nothing to gather is looked at
// again only after craft.IngredientRetry ticks, then works as soon as it
// may.
func TestBillWithoutIngredientsDoesNotRescanEveryTick(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	runBillTicks(g, ada, 50, func() bool { return false })
	at := g.craft.Bills.Of(bench)[0].SearchAt
	if at == 0 {
		t.Fatal("no retry set for a bill with nothing to gather")
	}
	runBillTicks(g, ada, 50, func() bool { return false })
	if g.craft.Bills.Of(bench)[0].SearchAt != at {
		t.Fatal("searched again inside the cooldown")
	}
	g.SpawnStack(item.Wood, 10, 3, 5)
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	runBillTicks(g, ada, 6000, done)
	if !done() || uint64(g.clock.Now()) < at {
		t.Fatalf("club done %v at tick %d, cooldown to %d", done(), g.clock.Now(), at)
	}
}
