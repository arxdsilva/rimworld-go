package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// malnourish gives pawn id malnutrition at severity sev (FOOD-1 tests),
// at the slowest rolled rate.
func malnourish(g *Game, id entity.EntityID, sev int32) {
	v, _ := g.vital.Get(id)
	b := g.ensureBody(id, &v)
	if c, ok := b.Cond(body.CondMalnutrition); ok {
		c.Sev = sev
	} else {
		b.AddCond(body.Condition{Kind: body.CondMalnutrition, Part: body.Torso, Sev: sev, Rate: body.MalnutritionRateMin})
	}
	g.bodyChanged(id, b, &v)
}

// thoughtValue is pawn id's thought of kind k (ok false: none).
func thoughtValue(g *Game, id entity.EntityID, k mood.Kind) (int, bool) {
	for _, th := range g.thoughts(id) {
		if th.Kind == k {
			return th.Value, true
		}
	}
	return 0, false
}

// TestHungerThoughtBands (FOOD-1, wiki: Saturation): Hungry -6 below 25 %,
// Ravenously hungry -12 below 12.5 %, Malnourished at 0 % by the
// malnutrition stage (-20 ... -44).
func TestHungerThoughtBands(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	for _, c := range []struct {
		hunger int
		sev    int32
		kind   mood.Kind
		value  int
	}{
		{needs.HungryAt, 0, mood.Hungry, -6}, {needs.RavenousAt, 0, mood.Ravenous, -12},
		{0, 0, mood.Malnourished, -20}, {0, 450, mood.Malnourished, -32}, {0, 850, mood.Malnourished, -44},
	} {
		g.SetNeeds(id, c.hunger, needs.Full)
		if c.sev > 0 {
			malnourish(g, id, c.sev)
		}
		if v, ok := thoughtValue(g, id, c.kind); !ok || v != c.value {
			t.Errorf("food %d, malnutrition %d: %v thought %d (present %v), want %d", c.hunger, c.sev, c.kind, v, ok, c.value)
		}
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	for _, k := range []mood.Kind{mood.Hungry, mood.Ravenous, mood.Malnourished} {
		if _, ok := thoughtValue(g, id, k); ok {
			t.Errorf("a fed colonist thinks %v", k)
		}
	}
}

// TestStarvingColonistMalnutritionToDeath (FOOD-1, wiki: Malnutrition): a
// colonist with no food stays up at 0 %, its malnutrition rises through
// the stages over about two days (shown as PawnView.Malnutrition) and kills
// it at full severity.
func TestStarvingColonistMalnutritionToDeath(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 10, Height: 6})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, 0, needs.Full)
	stages, start := map[uint8]bool{}, g.clock.Now()
	for g.VitalOf(id).Status != health.Dead && g.clock.Now()-start < 4*clock.TicksPerDay {
		g.Step(clock.TicksPerHour)
		if c, ok := g.bodies[id].Cond(body.CondMalnutrition); ok {
			stages[c.Stage] = true
			if p := pawn(t, g, id); p.Malnutrition != int(c.Sev)/10 {
				t.Fatalf("PawnView.Malnutrition %d, severity %d", p.Malnutrition, c.Sev)
			}
		}
		g.SetNeeds(id, 0, needs.Full) // kept awake and starving
	}
	days := float64(g.clock.Now()-start) / clock.TicksPerDay
	if v := g.VitalOf(id); v.Status != health.Dead || v.Cause != health.CauseNoFood {
		t.Fatalf("after %.2f days: %+v", days, v)
	}
	if days < 1.7 || days > 2.9 || len(stages) != 5 {
		t.Fatalf("died after %.2f days through stages %v, want about 2 days and all five", days, stages)
	}
}

// TestMalnourishedEatsACorpse (FOOD-1, wiki: Malnutrition): with nothing
// else, a malnourished colonist eats an animal's carcass; a merely hungry
// one leaves it.
func TestMalnourishedEatsACorpse(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	sid, _ := g.SpawnStack(item.CorpseAnimal, 1, 8, 2) // a deer (species 0)
	g.SetNeeds(id, needs.HungryAt, needs.Full)
	if g.seekFood(id) {
		t.Fatal("a hungry colonist does not count a carcass as food")
	}
	if best, _, _, _ := g.bestFood(id, id); best != 0 {
		t.Fatal("a hungry colonist picks no carcass")
	}
	g.SetNeeds(id, 0, needs.Full)
	malnourish(g, id, 100)
	if best, _, _, _ := g.bestFood(id, id); best != sid || !g.seekFood(id) {
		t.Fatalf("a malnourished colonist eats the carcass: best %d", best)
	}
	g.Step(TicksPerCell*20 + needs.EatWorkTicks + 50)
	if n, _ := g.need.Get(id); n.Hunger < min(needs.Full, animal.CarcassFood(animal.Deer))-300 {
		t.Fatalf("hunger %d after eating a deer (%d)", n.Hunger, animal.CarcassFood(animal.Deer))
	}
	if _, ok := g.stacks.Get(sid); ok {
		t.Fatal("the carcass is eaten")
	}
}

// TestColonistEatsAboutTwoMealsADay (FOOD-1, no flap): with plenty of
// meals a colonist eats at 30 % and fills up, so 1.6 nutrition a day is
// about two meals, not a nibble every few hours.
func TestColonistEatsAboutTwoMealsADay(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SpawnStack(item.Meal, 20, 8, 3)
	g.SetNeeds(id, needs.Full, needs.Full)
	starts := jobStarts(g, []entity.EntityID{id}, 2*clock.TicksPerDay, func() bool { return false }, job.Eat)
	if n := starts[id]; n < 2 || n > 5 {
		t.Fatalf("%d meals in two days, want 3-4 (1.6 nutrition a day at 0.9 a meal)", n)
	}
}

// TestSimpleMealTakesTenRawUnits (FOOD-1, wiki: Meals): 0.5 nutrition of
// raw food, ten units, make a simple meal; nine are not enough.
func TestSimpleMealTakesTenRawUnits(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Stove, construction.South, 5, 2)
	g.FuelCampfire(5, 2) // FOOD-4: the stove burns wood
	g.SpawnStack(item.Berries, 9, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 1})
	cook := func() {
		for range 20 {
			g.SetNeeds(id, needs.Full, needs.Full)
			g.Step(TicksPerCell * 10)
		}
	}
	cook()
	if w := g.World(); w.FoodByKind["meal"] != 0 || w.FoodByKind["berries"] != 9 {
		t.Fatalf("nine berries cooked: %v", w.FoodByKind)
	}
	g.SpawnStack(item.Berries, 1, 2, 4)
	cook()
	if w := g.World(); w.FoodByKind["meal"] != 1 || w.FoodByKind["berries"] != 0 {
		t.Fatalf("ten berries: %v", w.FoodByKind)
	}
}

// TestStarvationSaveMigration (FOOD-1, save v7): an older save's colonist
// down from hunger gets up on load; a current save keeps malnutrition and
// the hunger band, and plays on identically.
func TestStarvationSaveMigration(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.down(id, health.CauseNoFood)
	g.SetNeeds(id, 0, needs.Full)
	if l := loadAsVersion(t, g, 6); l.VitalOf(id).Status != health.Alive {
		t.Fatalf("a v6 save's starving colonist stays down: %+v", l.VitalOf(id))
	}
	if l := loadAsVersion(t, g, SaveVersion); l.VitalOf(id).Status != health.Downed {
		t.Fatal("a current save keeps its record")
	}
	h := New(Config{Seed: 2, Width: 12, Height: 6})
	ada, _ := h.SpawnNamedPawn(2, 2, "Ada")
	h.SetNeeds(ada, 0, needs.Full)
	h.Step(3 * clock.TicksPerHour)
	l, _ := saveLoadInfo(t, h)
	if body.Malnutrition(l.bodies[ada]) != body.Malnutrition(h.bodies[ada]) || body.Malnutrition(l.bodies[ada]) <= 0 {
		t.Fatalf("malnutrition %d after the round trip, %d before", body.Malnutrition(l.bodies[ada]), body.Malnutrition(h.bodies[ada]))
	}
	if a, b := l.need.Ref(ada), h.need.Ref(ada); *a != *b {
		t.Fatalf("needs %+v after the round trip, %+v before", *a, *b)
	}
	h.Step(3 * clock.TicksPerHour)
	l.Step(3 * clock.TicksPerHour)
	if l.StateHash() != h.StateHash() {
		t.Fatal("the loaded game plays differently")
	}
}
