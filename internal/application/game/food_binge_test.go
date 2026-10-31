package game

import (
	"testing"

	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
)

// TestWholeFoodBingeIsBounded runs a food binge for the wiki's longest
// (45 000 ticks, Food binge) next to a big pile of meals: the binger eats
// one meal a sitting (wiki: Maximum To Ingest 1) at most once every
// mind.BingeGap, so the whole binge eats a handful to a few dozen meals,
// never the unbounded stream of a re-issued eat (FOOD-6).
func TestWholeFoodBingeIsBounded(t *testing.T) {
	const longest = 45000
	g := New(Config{Seed: 2, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SpawnStack(item.Meal, 200, 4, 2)
	g.SetNeeds(id, needs.Full, needs.Full)
	g.ForceBreak(id, mentalbreak.FoodBinge)
	g.breaks[id] = breakState{kind: mentalbreak.FoodBinge, until: uint64(g.clock.Now()) + longest}
	sittings, prev := 0, 200
	for i := 0; i < longest+clock.TicksPerHour; i++ {
		g.Step(1)
		st, _ := stackOf(g, item.Meal)
		if st.Qty < prev {
			if prev-st.Qty > 1 {
				t.Fatalf("a sitting ate %d meals, want 1 (Maximum To Ingest)", prev-st.Qty)
			}
			sittings++
			prev = st.Qty
		}
	}
	if g.Breaking(id) != "" {
		t.Fatalf("still %q after the longest binge", g.Breaking(id))
	}
	most := longest/mind.BingeGap + 1
	if sittings < 5 || sittings > most {
		t.Fatalf("a whole binge ate %d meals, want 5..%d", sittings, most)
	}
	t.Logf("a %d-tick binge ate %d meals", longest, sittings)
}

// TestFoodPoisonedPawnEatsSlower: food poisoning's eating factor (x50 %
// in the initial stage; wiki: Food poisoning) stretches a sitting to
// twice the work of a healthy one (FOOD-6).
func TestFoodPoisonedPawnEatsSlower(t *testing.T) {
	sitting := func(poisoned bool) int {
		g := New(Config{Seed: 2, Width: 10, Height: 6})
		id, _ := g.SpawnNamedPawn(2, 2, "Ada")
		meal, _ := g.SpawnStack(item.Meal, 1, 2, 2)
		g.SetNeeds(id, needs.CriticalHunger, needs.Full)
		if v, _ := g.vital.Get(id); poisoned && !g.ensureBody(id, &v).Poison(uint64(g.clock.Now())) {
			t.Fatal("poison")
		}
		g.jobs.Set(id, job.Record{Kind: job.Eat, Target: meal, TX: 2, TY: 2, WorkLeft: needs.EatWorkTicks, Count: 2, State: job.Do})
		for i := 1; i <= 4*needs.EatWorkTicks; i++ {
			g.Step(1)
			if _, ate := g.lastMeal[id]; ate {
				return i
			}
		}
		t.Fatalf("poisoned %v: never finished the meal", poisoned)
		return 0
	}
	healthy, sick := sitting(false), sitting(true)
	if healthy > needs.EatWorkTicks+2 || sick < 2*needs.EatWorkTicks-2 || sick > 2*needs.EatWorkTicks+2 {
		t.Fatalf("a sitting took %d ticks healthy, %d poisoned; want %d and twice that", healthy, sick, needs.EatWorkTicks)
	}
}
