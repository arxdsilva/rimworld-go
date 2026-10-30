package raid

import (
	"testing"

	"rimworld-go/internal/domain/faction"
)

// TestArrivalWeights: wiki map entry, walk-ins 60 of 90 rolls and the pods
// 30 split three ways; a tribe always walks in.
func TestArrivalWeights(t *testing.T) {
	t.Parallel()
	got := map[Arrival]int{}
	for roll := 0; roll < ArrivalRolls; roll++ {
		got[PickArrival(roll, faction.Industrial)]++
		if a := PickArrival(roll, faction.Neolithic); a != WalkInEdge {
			t.Fatalf("roll %d: a tribe arrives by %v", roll, a)
		}
	}
	want := map[Arrival]int{WalkInEdge: 60, DropPodCentre: 10, DropPodEdge: 10, DropPodScatter: 10}
	for a, n := range want {
		if got[a] != n {
			t.Errorf("%v: %d rolls, want %d", a, got[a], n)
		}
	}
}

// TestStrategyBands: the old plan's sappers and siege odds kept, the new
// strategies only from their first day, enclosed asked only for sappers.
func TestStrategyBands(t *testing.T) {
	t.Parallel()
	asked := 0
	enclosed := func() bool { asked++; return true }
	count := func(day int) map[Strategy]int {
		out := map[Strategy]int{}
		for roll := 0; roll < StrategyRolls; roll++ {
			out[PickStrategy(roll, day, enclosed)]++
		}
		return out
	}
	if c := count(1); c[ImmediateAssault] != StrategyRolls || asked != 0 {
		t.Fatalf("day 1: %v (enclosed asked %d)", c, asked)
	}
	c := count(20)
	if c[Sappers] != 50 || c[StageThenAssault] != 15 || c[Steal] != 10 || c[Burn] != 8 || c[ImmediateAssault] != 17 {
		t.Fatalf("day 20, shut in: %v", c)
	}
	open := func() bool { return false }
	if PickStrategy(10, 20, open) != Siege || PickStrategy(40, 20, open) != ImmediateAssault {
		t.Fatal("an open colony faces a siege under 30, else an assault below the stage band")
	}
}

// TestFit: a siege needs industrial mortars and points.
func TestFit(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s    Strategy
		tech faction.Tech
		pts  int
		want Strategy
	}{
		{Siege, faction.Industrial, SiegeMinPoints, Siege},
		{Siege, faction.Industrial, SiegeMinPoints - 1, StageThenAssault},
		{Siege, faction.Neolithic, 5000, StageThenAssault},
		{Burn, faction.Neolithic, 10, Burn},
		{NumStrategies, faction.Spacer, 10, ImmediateAssault},
	} {
		if got := Fit(c.s, c.tech, c.pts); got != c.want {
			t.Errorf("Fit(%v, %v, %d) = %v, want %v", c.s, c.tech, c.pts, got, c.want)
		}
	}
}

// TestFleeShare: wiki 40 % to 70 %, clamped; a group breaks at its share.
func TestFleeShare(t *testing.T) {
	t.Parallel()
	if FleeLoss(-3) != 40 || FleeLoss(0) != 40 || FleeLoss(FleeLossRolls-1) != 70 || FleeLoss(99) != 70 {
		t.Fatal("flee share outside 40..70")
	}
	if Broken(7, 10, 40) || !Broken(6, 10, 40) {
		t.Fatal("a 40 % group breaks at 4 of 10 lost, not 3")
	}
	if Broken(4, 10, 70) || !Broken(3, 10, 70) {
		t.Fatal("a 70 % group breaks at 7 of 10 lost, not 6")
	}
	if !Broken(1, 2, 0) || Broken(2, 2, 0) || Broken(0, 0, 50) {
		t.Fatal("legacy half rule; an empty group never breaks")
	}
}
