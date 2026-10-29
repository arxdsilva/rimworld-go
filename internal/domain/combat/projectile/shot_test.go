package projectile

import (
	"testing"

	"rimworld-go/internal/domain/combat/weapon"
)

// walk flies s to its end, returning the cells entered per tick.
func walk(s *Shot) [][][2]int {
	var out [][][2]int
	for !s.Arrived() {
		var tick [][2]int
		for n := s.Fly(); n > 0; n-- {
			x, y, _ := s.Step()
			tick = append(tick, [2]int{x, y})
		}
		out = append(out, tick)
	}
	return out
}

func TestShotWalksItsLine(t *testing.T) {
	t.Parallel()
	var s Shot
	s.Launch(0, 0, 6, 3, 2000) // two cells a tick
	ticks := walk(&s)
	if len(ticks) != 3 || s.X != 6 || s.Y != 3 || s.Cell != 6 {
		t.Fatalf("%v at %d,%d", ticks, s.X, s.Y)
	}
	want := [][2]int{{1, 0}, {2, 1}, {3, 1}, {4, 2}, {5, 2}, {6, 3}} // Sight.Line's cells (err = dx-dy)
	i := 0
	for _, tk := range ticks {
		for _, c := range tk {
			if c != want[i] {
				t.Fatalf("cell %d = %v want %v", i, c, want[i])
			}
			i++
		}
	}
}

func TestShotSpeedAndView(t *testing.T) {
	t.Parallel()
	var s Shot
	s.Launch(10, 5, 0, 5, weapon.DefOf(weapon.Rifle).Speed)
	s.Weapon = weapon.Rifle
	n := s.Fly()
	if n != 1 {
		t.Fatalf("1166 milli-cells enter one cell, got %d", n)
	}
	x, y, last := s.Step()
	if x != 9 || y != 5 || last {
		t.Fatal("first step")
	}
	v := s.View()
	if v.X != 10000-1166 || v.Y != 5000 || v.VX != -1166 || v.VY != 0 || v.Weapon != "rifle" {
		t.Fatalf("view %+v", v)
	}
	if n := s.Fly(); n != 1 { // 2332 -> cell 2
		t.Fatalf("second tick %d", n)
	}
	var z Shot
	z.Launch(3, 3, 3, 3, 0)
	if !z.Arrived() || z.Speed != 1 || z.View().X != 3000 {
		t.Fatal("zero-length shot")
	}
}

func TestShotLineMatchesSightOrder(t *testing.T) {
	t.Parallel()
	// every octant: the walk ends on the destination after Chebyshev steps
	for _, d := range [][2]int{{7, 2}, {-7, 2}, {7, -2}, {-7, -2}, {2, 7}, {-2, 7}, {2, -7}, {-2, -7}, {5, 5}, {0, -4}} {
		var s Shot
		s.Launch(20, 20, 20+d[0], 20+d[1], 999)
		walk(&s)
		if s.X != 20+d[0] || s.Y != 20+d[1] || s.Cell != max(abs(d[0]), abs(d[1])) {
			t.Fatalf("%v ends at %d,%d after %d", d, s.X, s.Y, s.Cell)
		}
	}
}

func TestPruneKeepsRecentTraces(t *testing.T) {
	t.Parallel()
	ts := []Trace{{Tick: 100}, {Tick: 100 + Lifetime}, {Tick: 90, Stage: Landed}}
	got := Prune(ts, 100+Lifetime)
	if len(got) != 1 || got[0].Tick != 100+Lifetime {
		t.Fatalf("pruned %+v", got)
	}
}

func TestShotBlast(t *testing.T) {
	t.Parallel()
	if !(&Shot{Weapon: weapon.FragGrenade}).Blast() || (&Shot{Weapon: weapon.Rifle}).Blast() || (&Shot{Weapon: 250}).Blast() {
		t.Fatal("only explosives burst")
	}
}
