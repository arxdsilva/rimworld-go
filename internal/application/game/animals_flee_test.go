package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// fleeEpisode steps ticks one at a time and counts id's Flee<->Wander
// switches. Every time it stops fleeing it must be clear of the threat
// at from (beyond the release radius) with its run finished.
func fleeEpisode(t *testing.T, g *Game, id entity.EntityID, from func() Position, ticks int) (switches int, fled bool) {
	t.Helper()
	release := animal.DefOf(animal.Dustbuck).FleeRadius + animal.FleeRelease
	prev := animal.Wander
	for i := 0; i < ticks; i++ {
		g.Step(1)
		a, _ := g.animals.Get(id)
		if a.Mode == prev {
			continue
		}
		switches++
		if a.Mode == animal.Flee {
			fled = true
		}
		if prev == animal.Flee {
			p, f := pos(g, id), from()
			if d := weapon.Distance(p.X, p.Y, f.X, f.Y); d <= release {
				t.Fatalf("tick %d: stopped fleeing %d tiles from the threat (release %d), path %d/%d",
					i, d, release, a.PathI, len(a.Path))
			}
		}
		prev = a.Mode
	}
	return switches, fled
}

func TestFleeingDustbuckRunsToItsGoal(t *testing.T) {
	g := New(Config{Seed: 5, Width: 40, Height: 20})
	watcher := draftedPawn(t, g, 10, 10) // stands still
	g.SetNeeds(watcher, needs.Full, needs.Full)
	buck, _ := g.SpawnAnimal("dustbuck", 14, 10)
	switches, fled := fleeEpisode(t, g, buck, func() Position { return pos(g, watcher) }, 600)
	if !fled {
		t.Fatal("a dustbuck 4 tiles from a colonist should flee")
	}
	if switches > 2 {
		t.Fatalf("flee/graze flapping: %d mode switches in 600 ticks", switches)
	}
}

func TestTameAnimalFleesWithoutFlapping(t *testing.T) {
	g := New(Config{Seed: 5, Width: 40, Height: 20})
	wolf, _ := g.SpawnAnimal("ridgewolf", 8, 10)
	pin := func() { // a sated wild wolf that just stands there
		a, _ := g.animals.Get(wolf)
		a.Hunger, a.NextThink, a.Path, a.PathI = 0, 1<<40, nil, 0
		g.animals.Set(wolf, a)
	}
	pin()
	buck, _ := g.SpawnAnimal("dustbuck", 12, 10)
	g.TameAnimal(buck, "Clover")
	switches, fled := fleeEpisode(t, g, buck, func() Position { pin(); return pos(g, wolf) }, 600)
	if !fled {
		t.Fatal("a tame grazer 4 tiles from a wild wolf should flee")
	}
	if switches > 2 {
		t.Fatalf("flee/graze flapping: %d mode switches in 600 ticks", switches)
	}
}

func TestEnragedAnimalDoesNotFlee(t *testing.T) {
	g := New(Config{Seed: 5, Width: 30, Height: 12})
	hunter, _ := g.SpawnNamedPawn(10, 6, "Hunter")
	g.SetNeeds(hunter, needs.Full, needs.Full)
	buck, _ := g.SpawnAnimal("dustbuck", 14, 6)
	a, _ := g.animals.Get(buck)
	a.Manhunter, a.Mode, a.Target = uint64(g.clock.Now())+animal.ManhunterTicks, animal.Attack, hunter
	g.animals.Set(buck, a)
	for i := 0; i < 5; i++ {
		g.hurt(buck, 1, hunter)
		if a, _ := g.animals.Get(buck); a.Mode == animal.Flee || a.FleeUntil != 0 {
			t.Fatalf("hit %d: an enraged dustbuck should keep attacking, got %+v", i, a)
		}
		g.Step(1)
	}
	if hasLog(g, buck, "Fled") {
		t.Fatalf("an enraged dustbuck never flees: %q", logTexts(g, buck))
	}
}
