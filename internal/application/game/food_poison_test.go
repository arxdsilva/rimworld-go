package game

import (
	"bytes"
	"encoding/gob"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
)

// setCooking sets id's Cooking level.
func setCooking(g *Game, id entity.EntityID, lvl int) {
	(*g.sk.skills[id])[skill.Cooking].Level = uint8(lvl)
}

// medicalHours runs only the medical phase for hours whole hours (the map
// stands still).
func medicalHours(g *Game, hours int) {
	for h := 0; h < hours; h++ {
		now := uint64(g.clock.Now())
		g.clock.Advance(int(clock.TicksPerHour - now%clock.TicksPerHour))
		g.phaseMedical()
	}
}

func TestPoisonChanceFallsWithSkill(t *testing.T) {
	t.Parallel()
	if poisonBase(0) != poisonUnskilled || poisonBase(20) != poisonMaster {
		t.Fatalf("base at 0 = %d, at 20 = %d", poisonBase(0), poisonBase(20))
	}
	for lvl := 1; lvl <= 20; lvl++ {
		if poisonBase(lvl) > poisonBase(lvl-1) {
			t.Fatalf("base rose from %d to %d at level %d", poisonBase(lvl-1), poisonBase(lvl), lvl)
		}
	}
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(1, 1, "Cook")
	var last int16 = 1001
	for _, lvl := range []int{0, 5, 10, 15, 20} {
		setCooking(g, id, lvl)
		p := g.mealPoison(id, construction.Stove, 5, 5) // outdoors: ×1.5
		if p >= last || p <= 0 {
			t.Fatalf("level %d poison %d‰ (previous %d‰): must fall with skill", lvl, p, last)
		}
		last = p
	}
	setCooking(g, id, 20)
	if p := g.mealPoison(id, construction.Campfire, 5, 5); p != poisonCampfire {
		t.Fatalf("a master's campfire meal %d‰, want the floor %d‰", p, poisonCampfire)
	}
}

func TestFilthyKitchenPoisonsMore(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	id, _ := g.SpawnNamedPawn(4, 4, "Cook")
	setCooking(g, id, 6)
	outdoors := g.mealPoison(id, construction.Stove, 20, 10)
	clean := g.mealPoison(id, construction.Stove, 4, 4)
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			g.addDirt(tileIdx(g, x, y), filth.Full)
		}
	}
	filthy := g.mealPoison(id, construction.Stove, 4, 4)
	base := poisonBase(6)
	if int(clean) != base*kitchenClean/1000 || int(filthy) != base*kitchenFilthy/1000 || int(outdoors) != base*kitchenOutdoors/1000 {
		t.Fatalf("base %d‰: clean %d‰, filthy %d‰, outdoors %d‰", base, clean, filthy, outdoors)
	}
	if !(clean < outdoors && outdoors < filthy) {
		t.Fatalf("clean %d‰ < outdoors %d‰ < filthy %d‰ fails", clean, outdoors, filthy)
	}
}

// TestCookedMealCarriesPoison: a stove meal leaves the bench with its
// cook's chance, and the item inspector's view shows it.
func TestCookedMealCarriesPoison(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	setCooking(g, id, 0)
	placeBuilt(t, g, construction.Stove, construction.South, 5, 2)
	g.SpawnStack(item.Berries, 6, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 1})
	for i := 0; i < 40 && g.World().FoodByKind["meal"] == 0; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	found := false
	for _, it := range g.World().Items {
		if it.Kind == "meal" {
			found = true
			if want := int(g.mealPoison(id, construction.Stove, it.X, it.Y)); it.PoisonPermille != want {
				t.Fatalf("meal poison %d‰, want %d‰", it.PoisonPermille, want)
			}
		} else if it.PoisonPermille != 0 {
			t.Fatalf("%s carries poison %d‰", it.Kind, it.PoisonPermille)
		}
	}
	if !found {
		t.Fatal("no meal was cooked")
	}
}

func TestFoodPoisoningRunsItsCourse(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(4, 4, "Pat")
	g.rollFoodPoison(id, item.Stack{Kind: item.Meal, Qty: 1, Poison: 1000})
	if !g.poisoned(id) || !g.indexedPatient(id) || !g.feelsSick(id) {
		t.Fatalf("a sure meal did not poison: poisoned %v, patient %v", g.poisoned(id), g.indexedPatient(id))
	}
	full := g.capacity(id, body.Consciousness)
	stages := map[uint8]bool{}
	prev := uint8(body.FoodStageInitial)
	hours := 0
	for g.poisoned(id) && hours < 48 {
		c, _ := g.bodies[id].Poisoned()
		if c.Stage > prev {
			t.Fatalf("stage went back up from %d to %d at hour %d", prev, c.Stage, hours)
		}
		prev = c.Stage
		stages[c.Stage] = true
		if c.Stage == body.FoodStageMajor && g.capacity(id, body.Consciousness) > full-150 {
			t.Fatalf("major stage consciousness %d (well %d)", g.capacity(id, body.Consciousness), full)
		}
		medicalHours(g, 1)
		hours++
	}
	if hours < body.FoodPoisonHours-1 || hours > body.FoodPoisonHours+1 {
		t.Fatalf("food poisoning lasted %d hours, want about %d", hours, body.FoodPoisonHours)
	}
	if len(stages) != 3 {
		t.Fatalf("stages seen %v, want initial, major and recovering", stages)
	}
	if v, _ := g.vital.Get(id); v.Status == health.Dead || g.feelsSick(id) {
		t.Fatalf("after: %v, sick %v", v.Status, g.feelsSick(id))
	}
}

func TestPoisonSurvivesStackMerge(t *testing.T) {
	t.Parallel()
	if p := item.MergePoison(1, 40, 3, 20); p != 25 {
		t.Fatalf("MergePoison(1×40, 3×20) = %d, want 25", p)
	}
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	g.dropMeal(item.Meal, 1, 5, 5, 40)
	g.dropMeal(item.Meal, 3, 5, 5, 20)
	var meals []item.Stack
	for _, sid := range g.itemIDs {
		if st, ok := g.stacks.Get(sid); ok && st.Kind == item.Meal {
			meals = append(meals, st)
		}
	}
	if len(meals) != 1 || meals[0].Qty != 4 || meals[0].Poison != 25 {
		t.Fatalf("merged meals %+v, want one stack of 4 at 25‰", meals)
	}
	if l := splitLoad(meals[0], 2); l.Poison != 25 {
		t.Fatalf("a hauled load keeps %d‰, want 25‰", l.Poison)
	}
}

func TestLegacyStacksNotPoisoned(t *testing.T) {
	t.Parallel()
	// a stack saved before the field existed decodes with no poison
	type oldStack struct {
		Kind item.Kind
		Qty  int
		X, Y int
		Rot  int
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(oldStack{Kind: item.Meal, Qty: 3, X: 2, Y: 2}); err != nil {
		t.Fatal(err)
	}
	var st item.Stack
	if err := gob.NewDecoder(&buf).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Kind != item.Meal || st.Qty != 3 || st.Poison != 0 {
		t.Fatalf("legacy stack %+v", st)
	}
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(4, 4, "Pat")
	ref := New(Config{Seed: 1, Width: 12, Height: 8})
	for i := 0; i < 2000; i++ {
		g.rollFoodPoison(id, st)
	}
	if g.poisoned(id) {
		t.Fatal("a legacy meal poisoned")
	}
	if a, b := g.stream(streamMedical).IntN(1<<30), ref.stream(streamMedical).IntN(1<<30); a != b {
		t.Fatal("eating a zero-poison meal drew on the medical stream")
	}
}

func TestFoodPoisoningBedRestNoFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id, _ := g.SpawnNamedPawn(4, 4, "Pat")
	g.SpawnNamedPawn(12, 6, "Doc")
	buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
	v, _ := g.vital.Get(id)
	b := g.ensureBody(id, &v)
	b.Poison(uint64(g.clock.Now()))
	if g.needsBedRest(id) {
		t.Fatal("the initial stage needs no bed")
	}
	transitions, was := 0, false
	for h := 0; h < body.FoodPoisonHours+2; h++ {
		now := uint64(g.clock.Now())
		g.clock.Advance(int(clock.TicksPerHour - now%clock.TicksPerHour))
		b.StepFoodPoisoning(uint64(g.clock.Now()))
		for i := 0; i < 4; i++ { // a few looks an hour, as phaseAI does every tick
			g.updateBedRest(id)
		}
		on := g.onBedRest(id)
		c, still := b.Poisoned()
		switch {
		case still && c.Stage == body.FoodStageMajor && !on:
			t.Fatalf("hour %d: major food poisoning, not in bed", h)
		case still && c.Stage == body.FoodStageRecovering && on:
			t.Fatalf("hour %d: recovering, still kept in bed", h)
		}
		if on != was {
			transitions++
			was = on
		}
	}
	if transitions != 2 {
		t.Fatalf("bed rest switched %d times, want in once and out once", transitions)
	}
	b.Poison(uint64(g.clock.Now()) - body.FoodPoisonInitialHours*clock.TicksPerHour)
	b.StepFoodPoisoning(uint64(g.clock.Now()))
	if !g.needsBedRest(id) {
		t.Fatal("a fresh major stage sends to bed")
	}
	if n := kindChanges(g, id, 300); n > 3 {
		t.Fatalf("job kind changed %d times in 300 ticks", n)
	}
}
