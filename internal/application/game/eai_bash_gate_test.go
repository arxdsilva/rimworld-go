package game

import (
	"testing"

	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/domain/sim/entity"
)

// EAI-001 cost bounds: only foes that matter are broken through to, and a
// breach search that found no way is not run again every tick.

// eaiChaseOnce runs one rage chase step of wild animal id after foe.
func eaiChaseOnce(g *Game, id, foe entity.EntityID) {
	a := animalOf(g, id)
	a.Target, a.Path, a.PathI, a.RetryAt = foe, nil, 0, 0
	rageOf(g).Chase(id, &a, uint64(g.clock.Now()))
	g.animals.Set(id, a)
}

// TestPredatorBashesOnlyForPeopleAndTameAnimals: a wild predator after
// wild prey behind a shut door leaves the door alone; after a colonist or
// a tame animal it breaks through.
func TestPredatorBashesOnlyForPeopleAndTameAnimals(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		foe  func(g *Game, ada entity.EntityID) entity.EntityID
		bash bool
	}{
		{"wild prey", func(g *Game, _ entity.EntityID) entity.EntityID {
			hare, _ := g.SpawnAnimal("hare", 12, 7)
			return hare
		}, false},
		{"tame animal", func(g *Game, _ entity.EntityID) entity.EntityID {
			hare, _ := g.SpawnAnimal("hare", 12, 7)
			a := animalOf(g, hare)
			a.Tame = true
			g.animals.Set(hare, a)
			return hare
		}, true},
		{"colonist", func(_ *Game, ada entity.EntityID) entity.EntityID { return ada }, true},
	} {
		g := New(Config{Seed: 6, Width: 30, Height: 22})
		_, ada := eaiShutIn(t, g)
		foe := c.foe(g, ada)
		if foe == 0 {
			t.Fatalf("%s: no foe", c.name)
		}
		wolf, _ := g.SpawnAnimal("timber_wolf", 13, 18)
		eaiChaseOnce(g, wolf, foe)
		b, held := g.wl.Rage.Bash[wolf]
		if got := held && b.X >= 0; got != c.bash {
			t.Fatalf("%s: bashing %v (%+v), want %v", c.name, got, b, c.bash)
		}
	}
}

// TestBreachSearchFailureRemembered: a manhunter wolf shut out by walls
// alone (it breaks no wall) remembers the failed search and runs no other
// for rage.BreachRetry ticks, showing no bash meanwhile, then searches
// again.
func TestBreachSearchFailureRemembered(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 30, Height: 22})
	bareGround(g)
	eaiBox(t, g, 10, 5, 16, 11) // no door
	draftedPawn(t, g, 13, 8)
	wolf, _ := g.SpawnAnimal("timber_wolf", 13, 18)
	eaiEnrage(g, wolf)
	var first rage.Bash
	for i := 0; i < 2000 && first == (rage.Bash{}); i++ {
		g.Step(1)
		first = g.wl.Rage.Bash[wolf]
	}
	if first.X >= 0 {
		t.Fatalf("the failed breach search should be remembered as no block: %+v", first)
	}
	for uint64(g.clock.Now()) < first.Since+rage.BreachRetry {
		if b := g.wl.Rage.Bash[wolf]; b != first {
			t.Fatalf("tick %d: searched again before the retry: %+v after %+v", g.clock.Now(), b, first)
		}
		if eaiView(g, wolf).Bashing {
			t.Fatal("no block found, yet the inspector shows a bash")
		}
		g.Step(1)
	}
	for i := 0; i < 1000; i++ {
		g.Step(1)
		if b, ok := g.wl.Rage.Bash[wolf]; ok && b.Since > first.Since {
			if b.Since < first.Since+rage.BreachRetry || b.X >= 0 {
				t.Fatalf("the next search %+v after %+v", b, first)
			}
			return
		}
	}
	t.Fatal("the wolf never searched for a way through again")
}
