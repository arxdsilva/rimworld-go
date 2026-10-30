package haul

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func cands(cs ...Cand) []Cand { return cs }

func TestPlanChainsNearestFirst(t *testing.T) {
	first := Cand{ID: 1, X: 0, Y: 0, Qty: 12}
	// a row of piles, listed out of order: the chain walks the row
	got := Plan(nil, first, cands(
		Cand{ID: 4, X: 3, Y: 0, Qty: 12},
		Cand{ID: 2, X: 1, Y: 0, Qty: 12},
		Cand{ID: 3, X: 2, Y: 0, Qty: 12},
	), 100, Radius)
	want := []Pick{{2, 12}, {3, 12}, {4, 12}}
	if !slices.Equal(got, want) {
		t.Fatalf("plan = %v, want %v", got, want)
	}
}

func TestPlanCapacityTakesTheLastPileInPart(t *testing.T) {
	first := Cand{ID: 1, Qty: 12}
	var cs []Cand
	for i := range 11 {
		cs = append(cs, Cand{ID: entity.EntityID(i + 2), X: i + 1, Qty: 12})
	}
	got := Plan(nil, first, cs, 100, Radius)
	sum := 12
	for _, p := range got {
		sum += p.Take
	}
	if sum != 100 || len(got) != 8 || got[7] != (Pick{9, 4}) {
		t.Fatalf("plan = %v (load %d), want 7 whole piles and 4 of the ninth: 100", got, sum)
	}
	if got := Plan(nil, Cand{ID: 1, Qty: 120}, cs, 100, Radius); len(got) != 0 {
		t.Fatalf("a first pile over capacity gathers nothing: %v", got)
	}
}

func TestPlanRadiusCutsTheChain(t *testing.T) {
	first := Cand{ID: 1, Qty: 10}
	got := Plan(nil, first, cands(
		Cand{ID: 2, X: 8, Y: 8, Qty: 10},  // just in reach (Chebyshev 8)
		Cand{ID: 3, X: 17, Y: 8, Qty: 10}, // 9 from the last pile
		Cand{ID: 4, X: 0, Y: 30, Qty: 10}, // far from everything
	), 100, Radius)
	if !slices.Equal(got, []Pick{{2, 10}}) {
		t.Fatalf("plan = %v, want only the pile in reach", got)
	}
}

func TestPlanTiesGoToTheLowerID(t *testing.T) {
	first := Cand{ID: 1, X: 5, Y: 5, Qty: 90}
	got := Plan(nil, first, cands(
		Cand{ID: 9, X: 6, Y: 5, Qty: 10},
		Cand{ID: 7, X: 4, Y: 5, Qty: 10},
		Cand{ID: 8, X: 5, Y: 6, Qty: 10},
	), 100, Radius)
	if !slices.Equal(got, []Pick{{7, 10}}) {
		t.Fatalf("plan = %v, want the lower id of three at one distance", got)
	}
}

func TestPlanIsDeterministicAndSkipsFirst(t *testing.T) {
	base := cands(
		Cand{ID: 1, X: 0, Y: 0, Qty: 5}, // the first pile itself, listed too
		Cand{ID: 5, X: 2, Y: 2, Qty: 5},
		Cand{ID: 3, X: 1, Y: 2, Qty: 5},
		Cand{ID: 6, X: 3, Y: 1, Qty: 0}, // empty: never taken
		Cand{ID: 2, X: 2, Y: 1, Qty: 5},
	)
	first := Cand{ID: 1, Qty: 5}
	want := Plan(nil, first, slices.Clone(base), 100, Radius)
	for range 5 {
		rev := slices.Clone(base)
		slices.Reverse(rev)
		if got := Plan(make([]Pick, 0, 8), first, rev, 100, Radius); !slices.Equal(got, want) {
			t.Fatalf("plan over another order = %v, want %v", got, want)
		}
	}
	for _, p := range want {
		if p.ID == 1 || p.ID == 6 {
			t.Fatalf("plan took the first or an empty pile: %v", want)
		}
	}
}

func TestPickupsAndClaimAll(t *testing.T) {
	if Pickups(nil) != nil {
		t.Fatal("no picks, no pickups")
	}
	pu := Pickups([]Pick{{4, 12}, {9, 3}})
	if !slices.Equal(pu, []entity.EntityID{4, 12, 9, 3}) {
		t.Fatalf("pickups = %v", pu)
	}
	c := fakeClaims{taken: map[entity.EntityID]bool{}}
	if !ClaimAll(c, 1, pu) || !c.taken[4] || !c.taken[9] || c.taken[12] {
		t.Fatalf("claims %v, want the pile ids only", c.taken)
	}
	c.refuse = 9
	if ClaimAll(c, 1, pu) {
		t.Fatal("a taken pile fails the claim")
	}
}

type fakeClaims struct {
	taken  map[entity.EntityID]bool
	refuse entity.EntityID
}

func (f fakeClaims) TryEntity(_, target entity.EntityID) bool {
	if target == f.refuse {
		return false
	}
	f.taken[target] = true
	return true
}
