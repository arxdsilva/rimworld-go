package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

func TestButcheringYieldsLeather(t *testing.T) {
	d := recipe.DefOf(recipe.ButcherAnimal) // leather by species (SIM-019)
	if d.Extra != item.Leather || !d.BySpecies || animal.DefOf(animal.Deer).Leather <= 0 {
		t.Fatalf("butchering should give leather: %+v", d)
	}
	if recipe.DefOf(recipe.MakeParka).Input.Kind != item.Leather {
		t.Fatal("parkas are sewn from leather")
	}
}

func TestColonistDressesForWinterAndSurvives(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 44, 6)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	g.SpawnStack(item.Parka, 1, 6, 3)
	g.SpawnStack(item.Tuque, 1, 8, 3)
	g.Step(600)
	if got := g.Wearing(id); len(got) != 2 {
		t.Fatalf("wearing %v: should have put on the parka and tuque", got)
	}
	// a winter day and night outdoors with no shelter
	for i := 0; i < 24; i++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	if v, _ := g.vital.Get(id); v.Status != health.Alive {
		t.Fatalf("dressed colonist %v (%v): a parka should get it through", v.Status, v.Cause)
	}
}

func TestParkaComesOffInSummer(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 15, 9)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	w := g.apparel[id]
	w[item.Body] = item.Parka
	g.apparel[id] = w
	g.Step(200)
	if got := g.Wearing(id); len(got) != 0 {
		t.Fatalf("still wearing %v in midsummer", got)
	}
	if st, ok := stackOf(g, item.Parka); !ok || st.Qty != 1 {
		t.Fatal("the parka should be left on the ground")
	}
}

func TestWearOrderInMenu(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SpawnStack(item.Duster, 1, 6, 3)
	found := false
	for _, o := range g.OrdersAt(id, 6, 3) {
		if o.Label == "Wear duster" && !o.Disabled {
			found = true
			g.Enqueue(o.Cmd)
		}
	}
	if !found {
		t.Fatal("order menu should offer Wear duster")
	}
	g.Step(300)
	if got := g.Wearing(id); len(got) != 1 || got[0] != "duster" {
		t.Fatalf("wearing %v after the order", got)
	}
	_ = command.Prioritize
}
