package recipe

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestInputKindsMatchPlanner: for every stateless ingredient, Kinds lists
// exactly the kinds the planner's per-stack test (AcceptsStack) takes.
func TestInputKindsMatchPlanner(t *testing.T) {
	t.Parallel()
	for _, d := range All() {
		for _, in := range d.AppendInputs(nil) {
			ks := in.Kinds()
			if !slices.IsSorted(ks) {
				t.Fatalf("%s: kinds not in kind order: %v", d.Name, ks)
			}
			if in.AnyDryable || in.Smelt || in.SmeltWear || in.Wreck {
				continue // judged per stack: the planner's state checks decide
			}
			for k := item.Kind(0); k.String() != "unknown"; k++ {
				if got, want := slices.Contains(ks, k), in.AcceptsStack(item.Stack{Kind: k, Qty: 1}); got != want {
					t.Fatalf("%s: kind %s listed %v, planner takes it %v", d.Name, k, got, want)
				}
			}
		}
	}
	club := DefOf(Club).Input
	if ks := club.Kinds(); !slices.Contains(ks, item.Wood) || !slices.Contains(ks, item.Steel) || slices.Contains(ks, item.Cloth) {
		t.Fatalf("club kinds %v", ks)
	}
	hops := Input{Kind: item.Hops, Hops: true, Qty: 1}
	if ks := hops.Kinds(); !slices.Equal(ks, []item.Kind{item.Brewgrain, item.Hops}) {
		t.Fatalf("hops kinds %v", ks)
	}
}
