package haul

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// TestGatherMealRule (CSE-012): a pile of tainted meals joins a clean load
// only when the colony's rule lets meals mix, and a refused pile is skipped,
// so the hauler ends its walk instead of retrying it.
func TestGatherMealRule(t *testing.T) {
	for _, mix := range []bool{false, true} {
		f := newFake()
		f.rule = item.MealRule{Mix: mix}
		tainted := item.Stack{Kind: item.Meal, Qty: 4, X: 3}
		tainted.AddIngredient(item.HumanMeat, 0)
		f.stacks[2] = tainted
		j := job.Record{Kind: job.Haul, Carry: carried, Count: 10, Pickups: []entity.EntityID{2, 4}}
		g := NewGather(f)
		got := g.Next(1, &j)
		if got != mix {
			t.Fatalf("mix %v: Next = %v, want %v", mix, got, mix)
		}
		if !mix {
			if _, still := f.stacks[2]; !still || f.stacks[carried].Qty != 4 {
				t.Fatal("a refused pile stays where it is, the load unchanged")
			}
			continue
		}
		f.px, f.py = 3, 0
		g.Arrive(1, &j)
		if f.stacks[carried].Qty != 8 {
			t.Fatalf("mixing allowed: load %d, want 8", f.stacks[carried].Qty)
		}
	}
}
