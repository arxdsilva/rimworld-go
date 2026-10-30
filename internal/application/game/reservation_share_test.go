package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// RES-1: reservations the RimWorld way. A pawn claims only the part of a
// stack it takes (portions, a haul load, a bill's ingredients, a dose),
// so pawns share stacks; a bench's work spot is claimed as a tile; a
// target that leaves the map drops every claim on it.

// jobStarts counts, per pawn, how often its job turned into one of kinds
// while g steps n ticks (or until done): a pawn that flaps between two
// activities restarts the job again and again.
func jobStarts(g *Game, ids []entity.EntityID, n int, done func() bool, kinds ...job.Kind) map[entity.EntityID]int {
	starts, last := map[entity.EntityID]int{}, map[entity.EntityID]job.Record{}
	for _, id := range ids {
		last[id], _ = g.jobs.Get(id)
	}
	for i := 0; i < n && !done(); i++ {
		g.Step(1)
		for _, id := range ids {
			j, _ := g.jobs.Get(id)
			was := last[id]
			for _, k := range kinds {
				if j.Kind == k && j.Active() && (was.Kind != k || !was.Active()) {
					starts[id]++
				}
			}
			last[id] = j
		}
	}
	return starts
}

// TestTwoEatersShareAMealStack (no-flap): two hungry colonists claim
// their own portions of one meal stack, both eat from it, and neither
// restarts its meal.
func TestTwoEatersShareAMealStack(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	meal, _ := g.SpawnStack(item.Meal, 10, 8, 4)
	a, _ := g.SpawnNamedPawn(3, 4, "Ada")
	b, _ := g.SpawnNamedPawn(13, 4, "Bo")
	for _, id := range []entity.EntityID{a, b} {
		g.SetNeeds(id, needs.CriticalHunger-500, needs.Full)
		if !g.seekFood(id) {
			t.Fatalf("pawn %d sees no food: a partly claimed stack still feeds others", id)
		}
		g.assignEat(id)
	}
	ja, _ := g.jobs.Get(a)
	jb, _ := g.jobs.Get(b)
	if ja.Kind != job.Eat || jb.Kind != job.Eat || ja.Target != meal || jb.Target != meal {
		t.Fatalf("both should eat from the one stack: %+v / %+v", ja, jb)
	}
	want := portionsFor(needs.CriticalHunger-500, item.Meal)
	if free := g.reserve.Free(meal, 10); free != 10-2*want {
		t.Fatalf("free %d, want %d (each claims %d portions)", free, 10-2*want, want)
	}
	if _, whole := g.reserve.EntityOwner(meal); !whole || g.reserve.TryEntity(a, meal) {
		t.Fatal("a shared stack reads as taken and can't be held whole")
	}
	ate := func() bool { return g.ateAt[a] && g.ateAt[b] }
	starts := jobStarts(g, []entity.EntityID{a, b}, 3000, ate, job.Eat)
	if !ate() {
		t.Fatal("both should have eaten")
	}
	if starts[a] > 0 || starts[b] > 0 {
		t.Fatalf("eat jobs restarted (flapping): %v", starts)
	}
	if st, ok := g.stacks.Get(meal); ok && st.Qty > 8 {
		t.Fatalf("meal stack %+v after two meals", st)
	}
}

// TestTwoCraftersDrawWoodFromOneStack (no-flap): two crafters at two
// benches each claim the 6 wood their carpet takes from one stack of 20,
// and both bills finish without either job restarting.
func TestTwoCraftersDrawWoodFromOneStack(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	a, _ := g.SpawnNamedPawn(2, 8, "Ada")
	b, _ := g.SpawnNamedPawn(20, 8, "Bo")
	buildNow(t, g, construction.Workbench, item.Wood, construction.South, 5, 2)
	buildNow(t, g, construction.Workbench, item.Wood, construction.South, 15, 2)
	for _, x := range []int{5, 15} {
		g.Enqueue(command.Command{Kind: command.AddBill, X: x, Y: 2, Recipe: uint8(recipe.MakeWoodCarpet), Mode: uint8(recipe.DoCount), Count: 1})
	}
	wood, _ := g.SpawnStack(item.Wood, 20, 10, 8)
	g.Step(1)
	if !g.craft.Shop.Assign(a, crafterMask()) || !g.craft.Shop.Assign(b, crafterMask()) {
		t.Fatal("both crafters should start a bill")
	}
	ja, _ := g.jobs.Get(a)
	jb, _ := g.jobs.Get(b)
	if ja.Target != wood || jb.Target != wood || ja.Dest == jb.Dest {
		t.Fatalf("both should take from the one stack at their own bench: %+v / %+v", ja, jb)
	}
	if free := g.reserve.Free(wood, 20); free != 8 {
		t.Fatalf("free %d, want 20 - 6 - 6", free)
	}
	made := func() bool { return g.stockOf(item.CarpetWood) >= 2 }
	g.SetNeeds(a, needs.Full, needs.Full)
	g.SetNeeds(b, needs.Full, needs.Full)
	if starts := jobStarts(g, []entity.EntityID{a, b}, 4000, made, job.Bill, job.HaulIngredients); starts[a] > 0 || starts[b] > 0 {
		t.Fatalf("a bill job restarted (flapping): %v", starts)
	}
	if !made() || g.stockOf(item.Wood) != 8 {
		t.Fatalf("carpets %d, wood left %d; want 2 and 8", g.stockOf(item.CarpetWood), g.stockOf(item.Wood))
	}
}

// TestGoneTargetsReleaseTheirClaims: an eaten or destroyed item, a removed
// building, plant or animal, and a dead pawn drop every claim on them.
func TestGoneTargetsReleaseTheirClaims(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	a, _ := g.SpawnNamedPawn(1, 1, "Ada")
	b, _ := g.SpawnNamedPawn(2, 1, "Bo")
	meal, _ := g.SpawnStack(item.Meal, 4, 5, 5)
	bench := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 10, 2)
	tree, err := g.spawnPlant(plant.Tree, 15, 6, 0)
	if err != nil {
		t.Fatal(err)
	}
	g.reserve.TryCount(a, meal, 1, 4)
	g.reserve.TryCount(b, meal, 2, 4)
	g.reserve.TryEntity(a, bench)
	g.reserve.TryEntity(b, tree)
	g.reserve.TryEntity(a, b) // a doctor holding a patient
	g.deleteItem(meal)
	g.removeBuilding(bench)
	g.removePlant(tree)
	g.dieBy(b, 0, 0)
	for _, id := range []entity.EntityID{meal, bench, tree, b} {
		if o, held := g.reserve.EntityOwner(id); held {
			t.Errorf("target %d still held by %d", id, o)
		}
	}
	if len(g.reserve.Export()) != 0 {
		t.Fatalf("claims left: %+v", g.reserve.Export())
	}
}

// TestBenchesSharingASpotSeatOneWorker: two workbenches whose work spots
// are the same cell take one crafter at a time; the second finds the
// spot held and works neither.
func TestBenchesSharingASpotSeatOneWorker(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	a, _ := g.SpawnNamedPawn(1, 9, "Ada")
	b, _ := g.SpawnNamedPawn(18, 9, "Bo")
	buildNow(t, g, construction.Workbench, item.Wood, construction.South, 8, 2)
	_, spot, _ := construction.Footprint(construction.Workbench, construction.South, 8, 2)
	placed := false
	for y := spot[1] + 1; y <= spot[1]+3 && !placed; y++ {
		for x := 5; x <= 11 && !placed; x++ {
			if _, s, ok := construction.Footprint(construction.Workbench, construction.North, x, y); ok && s == spot {
				buildNow(t, g, construction.Workbench, item.Wood, construction.North, x, y)
				g.Enqueue(command.Command{Kind: command.AddBill, X: x, Y: y, Recipe: uint8(recipe.MakeWoodCarpet), Mode: uint8(recipe.Forever)})
				placed = true
			}
		}
	}
	if !placed {
		t.Fatal("no north bench shares the spot")
	}
	g.Enqueue(command.Command{Kind: command.AddBill, X: 8, Y: 2, Recipe: uint8(recipe.MakeWoodCarpet), Mode: uint8(recipe.Forever)})
	g.SpawnStack(item.Wood, 30, 2, 9)
	g.SpawnStack(item.Wood, 30, 17, 9)
	g.Step(1)
	if !g.craft.Shop.Assign(a, crafterMask()) {
		t.Fatal("the first crafter should start")
	}
	if o, held := g.reserve.TileOwner(spot[0], spot[1]); !held || o != a {
		t.Fatalf("work spot %v held by %d (%v), want the first crafter", spot, o, held)
	}
	if g.craft.Shop.Assign(b, crafterMask()) {
		jb, _ := g.jobs.Get(b)
		t.Fatalf("a second crafter took a bench on the same spot: %+v", jb)
	}
}

// TestSleeperSkipsACarriersBed: a bed a carrier has reserved for a patient
// (care/carrybed.go) is not taken by a colonist looking for a bed.
func TestSleeperSkipsACarriersBed(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	carrier, _ := g.SpawnNamedPawn(1, 1, "Ada")
	sleeper, _ := g.SpawnNamedPawn(2, 1, "Bo")
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 5)
	if !g.reserve.TryEntity(carrier, bed) {
		t.Fatal("carrier claim")
	}
	if g.rest.Beds.SleepIn(sleeper) {
		j, _ := g.jobs.Get(sleeper)
		t.Fatalf("the sleeper took the carrier's bed: %+v", j)
	}
	g.reserve.ReleaseOwner(carrier)
	if !g.rest.Beds.SleepIn(sleeper) {
		t.Fatal("a free bed should be taken")
	}
}

// TestBillTakesFromItsSourceStockpile: a bill set to take ingredients
// from one stockpile uses only stacks inside it, though wood lies nearer
// outside, and shows the source on its view.
func TestBillTakesFromItsSourceStockpile(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	zoneCmd(g, command.DesignateStockpile, 16, 8, 18, 10, 0)
	g.Step(1)
	zid, ok := g.zones.At(17, 9)
	if !ok {
		t.Fatal("no stockpile")
	}
	src := uint64(zid)
	near, _ := g.SpawnStack(item.Wood, 20, 5, 5)
	far, _ := g.SpawnStack(item.Wood, 20, 17, 9)
	g.Enqueue(command.Command{Kind: command.SetStorageSource, X: 5, Y: 2, Index: 0, Zone: src})
	g.Step(1)
	if bl := g.craft.Bills.Of(bench)[0]; bl.Source != src {
		t.Fatalf("bill source %d, want %d", bl.Source, src)
	}
	if !g.craft.Shop.Assign(ada, crafterMask()) {
		t.Fatal("the bill should start from the stockpile")
	}
	if j, _ := g.jobs.Get(ada); j.Target != far || j.Target == near {
		t.Fatalf("job %+v takes from %d, want the stockpile's stack %d", j, j.Target, far)
	}
	views := g.craft.Bills.AppendViews(nil, bench)
	if views[0].Source != src || views[0].SourceName == "" {
		t.Fatalf("bill view source %d %q", views[0].Source, views[0].SourceName)
	}
	g.interrupt(ada, job.Cancelled)
	g.Enqueue(command.Command{Kind: command.SetStorageSource, X: 5, Y: 2, Index: 0})
	g.Step(1)
	g.interrupt(ada, job.Cancelled)
	if !g.craft.Shop.Assign(ada, crafterMask()) {
		t.Fatal("the bill should start from anywhere")
	}
	if j, _ := g.jobs.Get(ada); j.Target != near {
		t.Fatalf("with no source the nearest stack: %+v", j)
	}
}

// TestTwoHaulersSharePartialPiles: the pile a hauler's load takes only
// part of keeps its rest free, and a second hauler plans exactly that
// rest, never more than the pile holds.
func TestTwoHaulersSharePartialPiles(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 8})
	pile, _ := g.SpawnStack(item.Wood, 50, 3, 3)
	zoneCmd(g, command.DesignateStockpile, 20, 2, 21, 3, 0)
	a := entity.EntityID(1 << 40)
	b, _ := g.SpawnNamedPawn(1, 4, "Bo")
	if !g.reserve.TryCount(a, pile, 30, 50) { // a stands for a hauler on its way: no job releases it
		t.Fatal("first claim")
	}
	jb := haulNow(t, g, b)
	if jb.Target != pile || g.reserve.Free(pile, 50) != 0 {
		t.Fatalf("second hauler %+v, free %d: want the 20 left", jb, g.reserve.Free(pile, 50))
	}
	for i := 0; i < 200; i++ {
		g.Step(5)
		if j, _ := g.jobs.Get(b); j.Carry != 0 {
			st, _ := g.stacks.Get(j.Carry)
			if left, _ := g.stacks.Get(pile); st.Qty != 20 || left.Qty != 30 {
				t.Fatalf("carried %d, left %d: want 20 taken and 30 kept for the first", st.Qty, left.Qty)
			}
			return
		}
	}
	t.Fatal("the second hauler never picked up")
}
