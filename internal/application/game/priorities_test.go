package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
)

// a colonist next to a ripe berry bush and a stack to haul
func choiceGame(t *testing.T) (*Game, uint64) {
	t.Helper()
	g := New(Config{Seed: 3, Width: 20, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 15, Y: 2, X2: 17, Y2: 4})
	g.SpawnStack(item.Wood, 10, 6, 5)
	if _, err := g.SpawnPlantSpecies("berry_bush", 4, 5, 1000000); err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 4, Y: 5, X2: 4, Y2: 5})
	return g, uint64(id)
}

func TestEqualPrioritiesFollowTheDefaultOrder(t *testing.T) {
	g, id := choiceGame(t)
	g.Step(2)
	if j, _ := g.jobs.Get(uint64ToID(id)); j.Kind != job.Haul {
		t.Fatalf("job %v: at equal priority hauling comes before growing", j.Kind)
	}
}

func TestHigherPriorityWorkComesFirst(t *testing.T) {
	g, id := choiceGame(t)
	g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: id, Work: command.WorkGrow, Count: 1})
	g.Step(2)
	if j, _ := g.jobs.Get(uint64ToID(id)); j.Kind != job.Harvest {
		t.Fatalf("job %v: growing at priority 1 beats hauling at 3", j.Kind)
	}
	if p := pawn(t, g, uint64ToID(id)); p.Priorities[2] != 1 || !p.WorkGrow {
		t.Fatalf("view priorities %v", p.Priorities)
	}
	g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: id, Work: command.WorkGrow, Count: 0})
	g.Step(1)
	if p := pawn(t, g, uint64ToID(id)); p.WorkGrow {
		t.Fatal("priority 0 turns the work off")
	}
}

func TestPersonalSchedule(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 10})
	a, _ := g.SpawnNamedPawn(3, 3, "Ada")
	b, _ := g.SpawnNamedPawn(8, 3, "Bo")
	h := g.Date().Hour
	g.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: uint64(b), Index: h, Mode: uint8(schedule.Sleep)})
	g.Step(5)
	if j, _ := g.jobs.Get(b); j.Kind != job.Sleep {
		t.Fatalf("Bo's job %v: his schedule says sleep now", j.Kind)
	}
	if j, _ := g.jobs.Get(a); j.Kind == job.Sleep {
		t.Fatal("Ada's schedule wasn't touched")
	}
	if pawn(t, g, b).Schedule[h] != schedule.Sleep {
		t.Fatal("schedule view")
	}
	// Pawn 0 paints everyone
	g.Enqueue(command.Command{Kind: command.SetSchedule, Index: h, Mode: uint8(schedule.Sleep)})
	g.Step(5)
	if j, _ := g.jobs.Get(a); j.Kind != job.Sleep {
		t.Fatal("colony-wide paint should reach Ada")
	}
}
