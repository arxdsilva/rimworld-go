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
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal products (SIM-026, plan 4.4).

// onlyWork leaves a colonist with just the given work types.
func onlyWork(g *Game, id entity.EntityID, types ...work.Type) {
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(id, wt, 0)
	}
	for _, wt := range types {
		g.setPriority(id, wt, work.DefaultPriority)
	}
}

// collectOnce runs a handler until an animal's product is collected.
func collectOnce(t *testing.T, g *Game, pawn, aid entity.EntityID) {
	t.Helper()
	for i := 0; i < 200; i++ {
		g.Step(20)
		g.SetNeeds(pawn, needs.Full, needs.Full)
		keepFed(g, aid)
		if animalOf(g, aid).Product == 0 {
			return
		}
	}
	j, _ := g.jobs.Get(pawn)
	t.Fatalf("not collected: %+v, job %v", animalOf(g, aid), j.Line())
}

func TestMilkJob(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	onlyWork(g, ada, work.Handle)
	bess := spawnSexed(t, g, "cow", 15, 10, true, true, "Bess")
	bull := spawnSexed(t, g, "cow", 20, 10, false, true, "Bull")
	setAnimal(g, bess, func(a *animal.State) { a.Product = animal.ProductFull })
	setAnimal(g, bull, func(a *animal.State) { a.Product = animal.ProductFull }) // no milk from a bull
	if v := viewOf(g, bess); v.Product != "milk" || v.ProductPct != 100 {
		t.Fatalf("view %+v", v)
	}
	if v := viewOf(g, bull); v.Product != "" {
		t.Fatalf("a bull gives no milk: %+v", v)
	}
	sawHold, sawAction := false, false
	for i := 0; i < 300 && animalOf(g, bess).Product != 0; i++ {
		g.Step(5)
		g.SetNeeds(ada, needs.Full, needs.Full)
		keepFed(g, bess)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Milk {
			sawHold = sawHold || g.beingTamed(bess)
			for _, p := range g.Snapshot() {
				if p.ID == ada && p.Action == ActionMilk && p.Acting == "milk" {
					sawAction = true
				}
			}
		}
	}
	if animalOf(g, bess).Product != 0 {
		t.Fatal("the cow is milked")
	}
	if n := itemsOnGround(g, "milk"); n != animal.LifeOf(animal.Cow).ProductYield || n != 10 {
		t.Fatalf("milk %d, want a large animal's 10", n)
	}
	if !sawHold || !sawAction {
		t.Fatalf("the cow holds still (%v); the handler milks (%v)", sawHold, sawAction)
	}
	g.Step(500)
	if j, _ := g.jobs.Get(ada); j.Kind == job.Milk {
		t.Fatal("no milking a bull")
	}
}

func TestShearJob(t *testing.T) {
	for _, c := range []struct {
		species string
		s       animal.Species
		want    int
	}{
		{"sheep", animal.Sheep, 20},   // small
		{"alpaca", animal.Alpaca, 35}, // medium
		{"bison", animal.Bison, 70},   // large
	} {
		g := New(Config{Seed: 1, Width: 30, Height: 20})
		ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
		onlyWork(g, ada, work.Handle)
		id := spawnSexed(t, g, c.species, 15, 10, false, true, "Woolly")
		setAnimal(g, id, func(a *animal.State) { a.Product = animal.ProductFull })
		if v := viewOf(g, id); v.Product != "wool" {
			t.Fatalf("%s: view %+v", c.species, v)
		}
		collectOnce(t, g, ada, id)
		_, st, ok := eggStack(g, item.Wool)
		if !ok || st.Qty != c.want || animal.Species(st.Animal) != c.s {
			t.Fatalf("%s: sheared %+v, want %d of its wool", c.species, st, c.want)
		}
	}
}

func TestEmberOilCollected(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	onlyWork(g, ada, work.Handle)
	buck := spawnSexed(t, g, "emberbuck", 15, 10, false, true, "Spark")
	setAnimal(g, buck, func(a *animal.State) { a.Product = animal.ProductFull })
	acting := ""
	for i := 0; i < 300 && animalOf(g, buck).Product != 0; i++ {
		g.Step(5)
		g.SetNeeds(ada, needs.Full, needs.Full)
		keepFed(g, buck)
		for _, p := range g.Snapshot() {
			if p.ID == ada && p.Acting != "" {
				acting = p.Acting
			}
		}
	}
	if itemsOnGround(g, "ember oil") != 6 || acting != "collect ember oil" {
		t.Fatalf("ember oil %d, acting %q", itemsOnGround(g, "ember oil"), acting)
	}
}

func TestHungryAnimalsProduceSlower(t *testing.T) {
	if animal.ProductPerDay(animal.Goat, 0) != 0 {
		t.Fatal("a starving animal grows nothing")
	}
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	fed := spawnSexed(t, g, "goat", 10, 10, true, true, "Fed")
	thin := spawnSexed(t, g, "goat", 20, 10, true, true, "Thin")
	young := spawnSexed(t, g, "goat", 25, 10, true, true, "Kid")
	setAnimal(g, young, func(a *animal.State) { a.Born = animal.BornAt(uint64(g.clock.Now()), 0) })
	for i := 0; i < clock.TicksPerDay/100; i++ {
		keepFed(g, fed)
		keepFed(g, young)
		keepFed(g, thin)
		setAnimal(g, thin, func(a *animal.State) { a.Food = animal.ProductFedFood - 1 })
		g.Step(100)
	}
	// milk fills in two days: half a day's worth when hungry
	f, h := animalOf(g, fed).Product, animalOf(g, thin).Product
	if f < 490 || f > 510 || h < 240 || h > 260 {
		t.Fatalf("a day's milk: fed %d, hungry %d", f, h)
	}
	if animalOf(g, young).Product != 0 {
		t.Fatal("the young produce nothing")
	}
	g.Step(clock.TicksPerDay * 2)
	if p := animalOf(g, fed).Product; p != animal.ProductFull {
		t.Fatalf("capped full: %d", p)
	}
}

func TestEmberOilFuelsCampfire(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	if !g.BuildNow("campfire", "wood", "south", 7, 3) {
		t.Fatal("build campfire")
	}
	if _, err := g.SpawnStack(item.EmberOil, 10, 2, 5); err != nil {
		t.Fatal(err)
	}
	g.Step(1500)
	bid := g.layerBldgAt(7, 3, construction.LayerObject)
	if g.fuel[bid] < (CampfireCapacity-1)*TicksPerWood {
		t.Fatalf("campfire fuel = %d after refuelling with oil", g.fuel[bid])
	}
	// a fire takes 7 oil (3 logs' worth each) to fill 20 logs' worth
	if st, _ := stackOf(g, item.EmberOil); st.Qty != 3 {
		t.Fatalf("ember oil left = %d, want 3", st.Qty)
	}
	if fuelTicks(item.EmberOil) != 3*fuelTicks(item.Wood) || fuelTicks(item.Stone) != 0 {
		t.Fatal("one oil burns as long as three logs")
	}
}

func TestWoolParkaRecipe(t *testing.T) {
	for id, want := range map[recipe.ID]struct {
		qty int
		out item.Kind
	}{recipe.KnitWoolParka: {16, item.WoolParka}, recipe.WeaveWoolCarpet: {8, item.CarpetWool}} {
		d := recipe.DefOf(id)
		if d.Bench != construction.TailoringBench || d.Input.Kind != item.Wool || d.Input.Qty != want.qty || d.Output != want.out {
			t.Fatalf("recipe %v = %+v", id, d)
		}
	}
	cold, _ := item.WoolParka.Insulation()
	leather, _ := item.Parka.Insulation()
	if !item.WoolParka.IsApparel() || item.WoolParka.Slot() != item.Parka.Slot() || cold*100 < leather*120 {
		t.Fatal("a wool parka is worn like a parka and 20% warmer")
	}
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	onlyWork(g, ada, work.Craft)
	placeBuilt(t, g, construction.TailoringBench, construction.South, 15, 2)
	g.Enqueue(commandAddBill(15, 2, recipe.KnitWoolParka))
	sid, _ := g.SpawnStack(item.Wool, 16, 8, 8)
	st, _ := g.stacks.Get(sid)
	st.Animal = uint8(animal.Sheep)
	g.stacks.Set(sid, st)
	worn := 0 // the tailor may put it straight on
	for i := 0; i < 100 && itemsOnGround(g, "wool parka")+worn == 0; i++ {
		g.Step(50)
		g.SetNeeds(ada, needs.Full, needs.Full)
		for _, k := range g.Wearing(ada) {
			if k == "wool parka" {
				worn = 1
			}
		}
	}
	if itemsOnGround(g, "wool parka")+worn != 1 || itemsOnGround(g, "wool") != 0 {
		t.Fatalf("knitted: parka %d (+%d worn), wool left %d", itemsOnGround(g, "wool parka"), worn, itemsOnGround(g, "wool"))
	}
}

func TestHandleWorkTypeAndTamingMoved(t *testing.T) {
	try := func(types ...work.Type) bool {
		g := New(Config{Seed: 2, Width: 24, Height: 14})
		ada, _ := g.SpawnNamedPawn(4, 7, "Ada")
		onlyWork(g, ada, types...)
		goat, _ := g.SpawnAnimal("goat", 16, 7)
		g.Enqueue(command.Command{Kind: command.DesignateTame, X: 16, Y: 7, X2: 16, Y2: 7})
		for i := 0; i < 600; i++ { // a failed try waits TameRetryTicks
			g.Step(50)
			g.SetNeeds(ada, needs.Full, needs.Full)
			if animalOf(g, goat).Tame {
				return true
			}
		}
		return false
	}
	if try(work.Hunt) {
		t.Fatal("hunters no longer tame (SIM-026: taming is Handle work)")
	}
	if !try(work.Handle) {
		t.Fatal("a handler with Hunt off tames the marked goat")
	}
	if wt, ok := workTypeOf(command.WorkHandle); !ok || wt != work.Handle {
		t.Fatal("the Handle work command")
	}
}
