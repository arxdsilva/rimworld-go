package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// FOOD-4 eating: food policies, food choice by optimality, the wiki's
// meal thoughts, cannibalism, fed patients and the days of food.

// setPolicy gives eater id food policy pol.
func setPolicy(g *Game, id entity.EntityID, pol int) {
	g.Enqueue(command.Command{Kind: command.SetFoodPolicy, Pawn: uint64(id), Index: pol})
	g.ApplyPending()
}

// makeCannibal gives colonist id the Cannibal trait.
func makeCannibal(g *Game, id entity.EntityID) {
	if g.sk.bios == nil {
		g.sk.bios = map[entity.EntityID]pawngen.Pawn{}
	}
	b := g.sk.bios[id]
	b.Traits = append(b.Traits, trait.Cannibal)
	g.sk.bios[id] = b
}

// TestFoodPolicyKeepsColonistOffLavish: on Lavish a colonist walks to the
// lavish meal; on Simple it eats the simple meal further off and leaves
// the lavish one, and its view names the policy.
func TestFoodPolicyKeepsColonistOffLavish(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 24, Height: 6})
	ada, _ := g.SpawnNamedPawn(1, 2, "Ada")
	lavish := stack(t, g, item.LavishMeal, 1, 8, 2)
	simple := stack(t, g, item.Meal, 1, 14, 2)
	if best, _, _, _ := g.bestFood(ada, ada); best != lavish {
		t.Fatalf("on Lavish the lavish meal: %d", best)
	}
	setPolicy(g, ada, diet.Simple)
	if best, _, _, _ := g.bestFood(ada, ada); best != simple {
		t.Fatalf("on Simple the simple meal: %d", best)
	}
	if v := pawnView(g, ada); v.FoodPolicy != "Simple" {
		t.Fatalf("view names %q", v.FoodPolicy)
	}
	g.SetNeeds(ada, needs.HungryAt, needs.Full)
	for i := 0; i < 2000 && g.lastMeal[ada] == 0; i++ {
		g.Step(1)
	}
	if g.lastMeal[ada] != item.Meal {
		t.Fatalf("ate %s", g.lastMeal[ada])
	}
	if st, ok := g.stacks.Get(lavish); !ok || st.Qty != 1 {
		t.Fatal("the lavish meal was touched")
	}
	setPolicy(g, ada, diet.Nothing)
	if best, _, _, _ := g.bestFood(ada, ada); best != 0 {
		t.Fatal("Nothing allows nothing")
	}
}

// TestMealMoods (wiki: Food, Meals): fine +5, lavish +12, raw -7 (not
// berries), no thought for a simple meal and none for a table; -3
// without one, never for a pawn fed in bed.
func TestMealMoods(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 12, Height: 6})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	for _, c := range []struct {
		k    item.Kind
		mood mood.Kind
		want int
	}{{item.FineMeal, mood.AteFineMeal, 5}, {item.LavishMeal, mood.AteLavishMeal, 12}, {item.Potatoes, mood.AteRaw, -7},
		{item.Kibble, mood.AteKibble, -12}, {item.NutrientPasteMeal, mood.AtePaste, -4}} {
		g.afterEat(ada, item.Stack{Kind: c.k, Qty: 1}, false)
		if v, ok := memValue(g, ada, c.mood); !ok || v != c.want {
			t.Errorf("%s: %d %v, want %d", c.k, v, ok, c.want)
		}
	}
	if v, ok := memValue(g, ada, mood.AteWithoutTable); !ok || v != -3 {
		t.Fatalf("no table: %d %v", v, ok)
	}
	bo, _ := g.SpawnNamedPawn(6, 2, "Bo")
	g.afterEat(bo, item.Stack{Kind: item.Berries, Qty: 1}, true)
	g.afterEat(bo, item.Stack{Kind: item.Meal, Qty: 1}, true)
	for _, k := range []mood.Kind{mood.AteRaw, mood.AteMeal, mood.AteWithoutTable, mood.AteAtTable} {
		if _, ok := memValue(g, bo, k); ok {
			t.Errorf("berries and a simple meal fed in bed leave no %s", mood.DefOf(k).Label)
		}
	}
}

// TestCannibalism (wiki: Human meat, Traits): raw human flesh -20 and a
// meal of it -15, a cannibal +20 and +15; insect meat -6 raw, -3 cooked;
// cannibals shrug off butchery; the optimality follows.
func TestCannibalism(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 12, Height: 6})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	cal, _ := g.SpawnNamedPawn(6, 2, "Cal")
	makeCannibal(g, cal)
	meal := item.Stack{Kind: item.Meal, Qty: 1}
	meal.Ingredients[0], meal.Ingredients[1] = item.Rice, item.HumanMeat
	for _, id := range []entity.EntityID{ada, cal} {
		g.afterEat(id, item.Stack{Kind: item.HumanMeat, Qty: 1}, true)
		g.afterEat(id, meal, true)
	}
	raw, _ := memValue(g, ada, mood.AteHumanMeat)
	cooked, _ := memValue(g, ada, mood.AteHumanMeatCooked)
	craw, _ := memValue(g, cal, mood.AteHumanMeat)
	ccooked, _ := memValue(g, cal, mood.AteHumanMeatCooked)
	if raw != -20 || cooked != -15 || craw != 20 || ccooked != 15 {
		t.Fatalf("human flesh: %d %d, cannibal %d %d", raw, cooked, craw, ccooked)
	}
	g.afterEat(ada, item.Stack{Kind: item.InsectMeat, Qty: 1}, true)
	if v, _ := memValue(g, ada, mood.AteInsectMeat); v != -6 {
		t.Fatalf("insect meat %d", v)
	}
	craftOutput{g}.ButcheredHuman(ada)
	if _, ok := memValue(g, cal, mood.ObservedButchery); ok {
		t.Fatal("a cannibal minds no butchery")
	}
	if _, ok := memValue(g, ada, mood.ButcheredHuman); !ok {
		t.Fatal("the butcher minds it")
	}
}

// TestFedPatientGetsThoughtsAndPaste: a doctor whose best food for the
// patient is a paste meal fetches one from the dispenser; the patient
// remembers it (no table thought in bed) and its view shows it ate.
func TestFedPatientGetsThoughtsAndPaste(t *testing.T) {
	t.Parallel()
	g, disp, _, doc := pasteKitchen(t, 60) // sto2_test.go: a stocked dispenser, colonist Ada
	onlyWork(g, doc, work.Doctor)
	scheduleAll(g, schedule.Work)
	ann, _ := g.SpawnNamedPawn(20, 12, "Ann")
	g.down(ann, health.CauseInjury)
	g.SetNeeds(ann, care.FeedHungry-100, needs.Full)
	g.SetNeeds(doc, needs.Full, needs.Full)
	setPolicy(g, ann, diet.Paste)
	if food, _, _ := (careMeals{g}).BestFood(doc, ann); food != disp {
		t.Fatal("the paste dispenser is the patient's best food")
	}
	for i := 0; i < 4000 && g.lastMeal[ann] == 0; i++ {
		fed(g, doc)
		g.Step(1)
	}
	if g.lastMeal[ann] != item.NutrientPasteMeal {
		t.Fatalf("the patient ate %s", g.lastMeal[ann])
	}
	if v, ok := memValue(g, ann, mood.AtePaste); !ok || v != -4 {
		t.Fatalf("paste thought %d %v", v, ok)
	}
	if _, ok := memValue(g, ann, mood.AteWithoutTable); ok {
		t.Fatal("fed in bed: no table thought")
	}
}

// TestPatientPolicyRulesTheFeedersChoice: the feeder fetches by the
// patient's policy, not its own.
func TestPatientPolicyRulesTheFeedersChoice(t *testing.T) {
	t.Parallel()
	g, ann, doc := feedGame(t, care.FeedHungry-100, true) // feed_patient_test.go: meals at (16, 5)
	setPolicy(g, ann, diet.Raw)
	if food, _, _ := (careMeals{g}).BestFood(doc, ann); food != 0 {
		t.Fatal("a Raw patient is brought no meal")
	}
	berries := stack(t, g, item.Berries, 10, 18, 5)
	if food, _, _ := (careMeals{g}).BestFood(doc, ann); food != berries {
		t.Fatalf("the berries for the Raw patient: %d", food)
	}
	if food, _, _, _ := g.bestFood(doc, doc); food == berries || food == 0 {
		t.Fatal("the doctor itself would take a meal")
	}
}

// TestRawFoodPoisonRoll (wiki: Ailments): raw food rolls its flat 2 %,
// paste never; over many meals raw food poisons about one in fifty.
func TestRawFoodPoisonRoll(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 12, Height: 6})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	sick := 0
	for i := 0; i < 2000; i++ {
		g.rollFoodPoison(ada, item.Stack{Kind: item.NutrientPasteMeal, Qty: 1})
		if g.poisoned(ada) {
			t.Fatal("paste never poisons")
		}
	}
	for i := 0; i < 2000; i++ {
		g.rollFoodPoison(ada, item.Stack{Kind: item.Potatoes, Qty: 1})
		if g.poisoned(ada) {
			sick++
			g.bodies[ada].RemoveCond(body.CondFoodPoisoning)
		}
	}
	if sick < 20 || sick > 70 { // 2 % of 2000 is 40
		t.Fatalf("raw food poisoned %d of 2000", sick)
	}
}

// TestTameAnimalPolicy: a tame animal's policy keeps it off forbidden
// feed; its view names the policy (Animal feed by default).
func TestTameAnimalPolicy(t *testing.T) {
	t.Parallel()
	for _, pol := range []int{diet.Nothing, diet.AnimalFeed} {
		g := feedField(30, 12)
		dog := hungryTame(t, g, "labrador", 10, 6, "Rex")
		stack(t, g, item.Kibble, 30, 13, 6)
		if v := animalView(t, g, dog); v.FoodPolicy != "Animal feed" {
			t.Fatalf("default policy %q", v.FoodPolicy)
		}
		setPolicy(g, dog, pol)
		a := firstFeed(g, dog, 2*animal.DecideEvery)
		if ate := a.Mode == animal.Feed && a.Feed.Kind == animal.FeedItem; ate != (pol == diet.AnimalFeed) {
			t.Fatalf("policy %d: kibble %v, %+v", pol, ate, a.Feed)
		}
	}
}

// TestFoodDaysAndPolicySave: days of food are the loose human food over
// 1.6 nutrition a colonist a day; policies and choices survive a save.
func TestFoodDaysAndPolicySave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 16, Height: 6})
	ada, _ := g.SpawnNamedPawn(1, 2, "Ada")
	g.SpawnNamedPawn(2, 2, "Bo")
	stack(t, g, item.Meal, 8, 8, 2)  // 7.2 nutrition
	stack(t, g, item.Hay, 50, 10, 2) // never people's
	if d := g.World().FoodDays; d != 72*10/32 {
		t.Fatalf("days %d, want %d", d, 72*10/32)
	}
	setPolicy(g, ada, diet.Fine)
	g.Enqueue(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyCreate, Index: diet.Raw, Name: "Scraps"})
	g.ApplyPending()
	h := saveLoad(t, g)
	w := h.World()
	if len(w.FoodPolicies) != 8 || w.FoodPolicies[7].Name != "Scraps" || pawnView(h, ada).FoodPolicy != "Fine" {
		t.Fatalf("after load: %v, %q", w.FoodPolicies, pawnView(h, ada).FoodPolicy)
	}
}
