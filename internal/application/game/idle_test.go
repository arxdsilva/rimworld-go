package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

func TestIdleColonistsDontStandStill(t *testing.T) {
	g := New(Config{Seed: 3, Width: 24, Height: 16})
	start := map[uint64]Position{}
	for i, n := range []string{"Ada", "Bo", "Cora"} {
		id, _ := g.SpawnNamedPawn(8+i, 8, n)
		start[uint64(id)] = Position{8 + i, 8}
	}
	moved, idle := 0, map[job.Kind]bool{}
	for step := 0; step < 20; step++ {
		g.Step(150)
		for _, id := range g.pawnIDs {
			g.SetNeeds(id, needs.Full, needs.Full)
			if j, _ := g.jobs.Get(id); j.Kind.IsIdle() {
				idle[j.Kind] = true
			}
		}
	}
	for _, id := range g.pawnIDs {
		if p, _ := g.positions.Get(id); p != start[uint64(id)] {
			moved++
		}
	}
	if moved < 2 || len(idle) == 0 {
		t.Fatalf("moved=%d idle kinds=%v: colonists with nothing to do should wander about", moved, idle)
	}
}

func TestIdleGivesWayToWork(t *testing.T) {
	g := New(Config{Seed: 3, Width: 24, Height: 16})
	id, _ := g.SpawnNamedPawn(8, 8, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	g.Step(200)
	if j, _ := g.jobs.Get(id); !j.Kind.IsIdle() {
		t.Fatalf("job %v: should be idling", j.Kind)
	}
	if _, err := g.SpawnPlantSpecies("tree", 14, 8, 1000000); err != nil {
		t.Fatal(err)
	}
	g.markCut(14, 8, 14, 8, true)
	g.Step(idleRecheck + 1)
	if j, _ := g.jobs.Get(id); j.Kind != job.Cut {
		t.Fatalf("job %v: a designated tree should replace idling", j.Kind)
	}
}

func TestBedtimeBeatsAPrioritizedOrder(t *testing.T) {
	g := New(Config{Seed: 3, Width: 24, Height: 16})
	id, _ := g.SpawnNamedPawn(2, 8, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	jumpTo(g, 0, 19)
	g.Step(clock.TicksPerHour - 400) // 19:50ish
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 22, Y: 14})
	g.Step(2)
	if j, _ := g.jobs.Get(id); j.Kind != job.Move {
		t.Fatalf("job %v: the order should start", j.Kind)
	}
	g.Step(500) // past 20:00, bedtime
	if j, _ := g.jobs.Get(id); j.Kind != job.Sleep {
		t.Fatalf("job %v at %v: bedtime should override the order", j.Kind, g.Date())
	}
}
