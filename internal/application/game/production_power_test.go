package game

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 CONSUMERS: the electric benches work their base kind's bills
// (or research) 40% faster, and only on power.

// benchStuff is what bench kind k is built of here: steel for an
// electric bench, wood for the rest.
func benchStuff(k construction.Kind) item.Kind {
	if craft.ElectricBench(k) {
		return item.Steel
	}
	return item.Wood
}

// cookGame is a cook, raw food and a stove of kind k at (5, 2) with a
// bill for one simple meal; an electric stove is on power (a generator
// at (8, 1)) when powered.
func cookGame(t *testing.T, k construction.Kind, powered bool) (g *Game, cook, stove, gen entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 1, Width: 16, Height: 8})
	cook, _ = g.SpawnNamedPawn(1, 1, "Ada")
	stove = buildNow(t, g, k, benchStuff(k), construction.South, 5, 2)
	if powered {
		gen = onPower(t, g, 5, 2, 8, 1)
	}
	g.SpawnStack(item.Berries, 6, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	if len(g.craft.Bills.Of(stove)) != 1 {
		t.Fatalf("%s takes no simple meal bill", k)
	}
	return g, cook, stove, gen
}

// cookTicks steps g until the meal is cooked (at most 4000 ticks) and
// counts the ticks the cook spent working at the bench.
func cookTicks(t *testing.T, g *Game, cook, stove entity.EntityID) int {
	t.Helper()
	work := 0
	for i := 0; i < 4000 && g.craft.Bills.Of(stove)[0].Done == 0; i++ {
		if i%100 == 0 {
			g.SetNeeds(cook, needs.Full, needs.Full)
		}
		g.Step(1)
		if j, _ := g.jobs.Get(cook); j.Kind == job.Bill && j.State == job.Do && j.Carry != 0 {
			work++
		}
	}
	if g.craft.Bills.Of(stove)[0].Done != 1 {
		t.Fatal("no meal cooked")
	}
	return work
}

// TestElectricStoveCooksFaster: the electric stove takes the stove's
// recipes and, on power, cooks the same meal 40% faster.
func TestElectricStoveCooksFaster(t *testing.T) {
	t.Parallel()
	g, cook, stove, _ := cookGame(t, construction.Stove, false)
	plain := cookTicks(t, g, cook, stove)
	g, cook, stove, _ = cookGame(t, construction.ElectricStove, true)
	if st := g.appendBillViews(nil, stove)[0].Status; st == BillNoPower {
		t.Fatal("a powered electric stove shows no power")
	}
	fast := cookTicks(t, g, cook, stove)
	t.Logf("simple meal: %d work ticks at a stove, %d at an electric stove", plain, fast)
	if fast*100 > plain*100*100/(100+craft.ElectricSpeedPct)+100 || fast*100 < plain*100*100/(100+craft.ElectricSpeedPct)-100 {
		t.Fatalf("electric stove %d work ticks, stove %d: want 40%% faster", fast, plain)
	}
}

// TestElectricStoveNeedsPower: with no power the electric stove's bill
// shows "no power" and nobody cooks there.
func TestElectricStoveNeedsPower(t *testing.T) {
	t.Parallel()
	g, cook, stove, _ := cookGame(t, construction.ElectricStove, false)
	for i := 0; i < 20; i++ {
		g.SetNeeds(cook, needs.Full, needs.Full)
		g.Step(100)
		if j, _ := g.jobs.Get(cook); j.Kind == job.Bill {
			t.Fatal("a colonist took a bill at an unpowered electric stove")
		}
	}
	if st := g.appendBillViews(nil, stove)[0].Status; st != BillNoPower {
		t.Fatalf("bill status %q, want %q", st, BillNoPower)
	}
}

// TestElectricStovePowerCutMidBill: when the power goes while the cook
// works, the bill job fails as blocked (the bench is put off a while), the
// ingredients are dropped and the bill reads "no power".
func TestElectricStovePowerCutMidBill(t *testing.T) {
	t.Parallel()
	g, cook, stove, gen := cookGame(t, construction.ElectricStove, true)
	working := false
	for i := 0; i < 3000 && !working; i++ {
		if i%100 == 0 {
			g.SetNeeds(cook, needs.Full, needs.Full)
		}
		g.Step(1)
		j, _ := g.jobs.Get(cook)
		working = j.Kind == job.Bill && j.State == job.Do && j.Carry != 0
	}
	if !working {
		t.Fatal("the cook never started the bill")
	}
	g.setFuel(gen, 0) // the generator runs dry
	failed := false
	for i := 0; i < 2*powerInterval && !failed; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(cook)
		failed = j.Kind == job.Bill && j.State == job.Fail && j.Fail == job.Blocked
	}
	if !failed {
		t.Fatal("the bill did not fail as blocked when the power went")
	}
	g.Step(1)
	if !g.postponedFor(cook, job.Bill, stove, 0, 0) {
		t.Fatal("the unpowered stove is not put off")
	}
	if st := g.appendBillViews(nil, stove)[0].Status; st != BillNoPower || g.craft.Bills.Of(stove)[0].Done != 0 {
		t.Fatalf("bill %+v, status %q", g.craft.Bills.Of(stove)[0], st)
	}
	if n := g.stockOf(item.Berries); n != 6 {
		t.Fatalf("%d berries, want all 6", n)
	}
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Carrier != 0 {
			t.Fatalf("%+v still carried: the ingredients must be dropped", st)
		}
	}
}

// researchGame is a researcher and a research bench of kind k at (4, 4)
// (on power when powered), studying Electricity in a fresh colony.
func researchGame(t *testing.T, k construction.Kind, powered bool) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	g.ResetResearch()
	buildNow(t, g, k, benchStuff(k), construction.South, 4, 4)
	if powered {
		onPower(t, g, 4, 4, 7, 3)
	}
	ada, _ := g.SpawnNamedPawn(10, 8, "Ada")
	g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Electricity)})
	g.Step(1)
	return g, ada
}

// researchPerTick steps g until the researcher has studied n ticks at
// the bench (at most 6000 ticks) and returns the points gained.
func researchPerTick(t *testing.T, g *Game, ada entity.EntityID, n int) int {
	t.Helper()
	studied := 0
	for i := 0; i < 6000 && studied < n; i++ {
		if i%100 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Research && j.State == job.Do {
			studied++
		}
	}
	if studied < n {
		t.Fatalf("studied %d ticks, want %d", studied, n)
	}
	return g.research[research.Electricity]
}

// TestElectricResearchBench: on power the electric research bench is a
// research bench that studies 40% faster; without power nobody uses it.
func TestElectricResearchBench(t *testing.T) {
	t.Parallel()
	g, ada := researchGame(t, construction.ResearchBench, false)
	plain := researchPerTick(t, g, ada, 500)
	g, ada = researchGame(t, construction.ElectricResearchBench, true)
	fast := researchPerTick(t, g, ada, 500)
	t.Logf("500 ticks of study: %d points at a research bench, %d at an electric one", plain, fast)
	if fast*100 < plain*(100+craft.ElectricSpeedPct)-2*100 || fast*100 > plain*(100+craft.ElectricSpeedPct)+2*100 {
		t.Fatalf("electric research bench %d points, plain %d: want 40%% more", fast, plain)
	}
	g, ada = researchGame(t, construction.ElectricResearchBench, false)
	for i := 0; i < 20; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(100)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Research {
			t.Fatal("a colonist studies at an unpowered electric research bench")
		}
	}
}
