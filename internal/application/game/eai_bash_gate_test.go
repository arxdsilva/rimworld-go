package game

import (
	"testing"

	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// EAI-001/002 cost bounds: only foes that matter are broken through to, a
// foe with no way to it (even through doors) is marked and not searched
// for again every tick, and a marked foe gives way to a reachable one.

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
// alone (it breaks no wall) marks its foe unreachable and looks for no
// way to it (path or breach) for rage.BreachRetry ticks, standing still
// and showing no bash meanwhile, then looks again; found unreachable a
// second time running, the foe stays marked twice as long.
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
	held := func(mark rage.Bash, wait uint64) {
		t.Helper()
		for uint64(g.clock.Now()) < mark.Since+wait {
			if b := g.wl.Rage.Bash[wolf]; b != mark {
				t.Fatalf("tick %d: searched again before the retry: %+v after %+v", g.clock.Now(), b, mark)
			}
			if eaiView(g, wolf).Bashing {
				t.Fatal("no block found, yet the inspector shows a bash")
			}
			if a := animalOf(g, wolf); a.PathI < len(a.Path) {
				t.Fatalf("tick %d: a path toward a foe marked unreachable", g.clock.Now())
			}
			g.Step(1)
		}
	}
	next := func(mark rage.Bash, wait uint64) rage.Bash {
		t.Helper()
		for i := 0; i < 1000; i++ {
			g.Step(1)
			if b, ok := g.wl.Rage.Bash[wolf]; ok && b.Since > mark.Since {
				if b.Since < mark.Since+wait || b.X >= 0 || b.Y != mark.Y-1 {
					t.Fatalf("the next search %+v after %+v", b, mark)
				}
				return b
			}
		}
		t.Fatal("the wolf never searched for a way through again")
		return rage.Bash{}
	}
	held(first, rage.BreachRetry)
	second := next(first, rage.BreachRetry)
	held(second, 2*rage.BreachRetry)
	next(second, 2*rage.BreachRetry)
}

// TestUnreachableMarkIsTheFoes: a predator's mark on a walled-in colonist
// does not keep it from chasing other prey it can reach.
func TestUnreachableMarkIsTheFoes(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 30, Height: 22})
	bareGround(g)
	eaiBox(t, g, 10, 5, 16, 11) // no door
	ada := draftedPawn(t, g, 13, 8)
	wolf, _ := g.SpawnAnimal("timber_wolf", 13, 18)
	hare, _ := g.SpawnAnimal("hare", 22, 18)
	eaiChaseOnce(g, wolf, ada)
	if !eaiMarked(g, wolf) {
		t.Fatalf("the walled-in colonist should be marked: %+v", g.wl.Rage.Bash[wolf])
	}
	eaiChaseOnce(g, wolf, hare)
	if a := animalOf(g, wolf); a.PathI >= len(a.Path) || eaiMarked(g, wolf) {
		t.Fatalf("the hare is not the marked foe: path %d/%d, mark %+v", a.PathI, len(a.Path), g.wl.Rage.Bash[wolf])
	}
}

// TestUnreachableFoeIsSkipped: a manhunter whose only foe is walled in
// (no door) marks it; a colonist in the open farther off is taken on the
// wolf's next look, and the wolf never goes back to the walled-in one
// while the other stands.
func TestUnreachableFoeIsSkipped(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 50, Height: 20})
	bareGround(g)
	eaiBox(t, g, 10, 7, 14, 13) // no door
	ada := draftedPawn(t, g, 12, 10)
	wolf, _ := g.SpawnAnimal("timber_wolf", 20, 10)
	eaiEnrage(g, wolf)
	for i := 0; i < 300 && !eaiMarked(g, wolf); i++ {
		g.Step(1)
	}
	if !eaiMarked(g, wolf) || animalOf(g, wolf).Target != ada {
		t.Fatalf("the walled-in colonist should be the marked foe: target %d, Ada %d, %+v", animalOf(g, wolf).Target, ada, g.wl.Rage.Bash[wolf])
	}
	bo := draftedPawn(t, g, 40, 10) // reachable, but farther than Ada
	switched := -1
	for i := 0; i < 400; i++ {
		g.Step(1)
		tg := animalOf(g, wolf).Target
		if switched < 0 && tg == bo {
			switched = i
		}
		if switched >= 0 && tg == ada && g.standingPawn(bo) {
			t.Fatalf("tick %d: back to the walled-in foe after switching", i)
		}
	}
	if switched < 0 || switched > animal.DecideEvery {
		t.Fatalf("the wolf should take the reachable colonist on its next look: switched at %d", switched)
	}
}

// eaiMarked reports animal id's foe marked unreachable.
func eaiMarked(g *Game, id entity.EntityID) bool {
	b, ok := g.wl.Rage.Bash[id]
	return ok && b.X < 0
}
