package cover

import "testing"

// TestAngleFactor: the wiki's bands, measured at the target between the
// shooter and the cover. wiki: Cover
func TestAngleFactor(t *testing.T) {
	t.Parallel()
	// target (0, 0), cover east (1, 0); the shooter moves round
	for _, c := range []struct{ sx, sy, want int }{
		{10, 0, 1000}, {10, 2, 1000}, // 0 and 11 deg
		{10, 4, 800},                       // 21.8 deg
		{10, 6, 600},                       // 31 deg
		{10, 10, 400},                      // 45 deg
		{10, 15, 200},                      // 56.3 deg
		{10, 30, 0},                        // 71.6 deg
		{0, 10, 0}, {-10, 0, 0}, {0, 0, 0}, // beside, behind, on the target
	} {
		if got := AngleFactor(c.sx, c.sy, 0, 0, 1, 0); got != c.want {
			t.Errorf("shooter (%d,%d): %d, want %d", c.sx, c.sy, got, c.want)
		}
	}
	if RangeFactor(2, 0, 1, 0) != FrontFactor || RangeFactor(3, 0, 1, 0) != NearFactor || RangeFactor(4, 0, 1, 0) != 1000 {
		t.Fatal("point-blank partial cover")
	}
}

func TestEffectiveCombines(t *testing.T) {
	t.Parallel()
	g := grid{{11, 10}: Sandbags}
	if p, k, x, y := Effective(g, 20, 10, 10, 10); p != 550 || k != Sandbags || x != 11 || y != 10 {
		t.Fatalf("straight on: %d %v (%d,%d)", p, k, x, y)
	}
	if p, _, _, _ := Effective(g, 0, 10, 10, 10); p != 0 {
		t.Fatal("behind the target: no cover")
	}
	// a diagonal shot across two walls of sandbags at 45 deg: each holds 40 %
	g[[2]int{10, 11}] = Sandbags
	if p, k, _, _ := Effective(g, 20, 20, 10, 10); p != 1000-(1000-220)*(1000-220)/1000 || k != Sandbags {
		t.Fatalf("two covers at an angle: %d", p)
	}
	if p, k, _, _ := Effective(g, 10, 10, 10, 10); p != 0 || k != None {
		t.Fatal("same tile")
	}
	// the shooter standing on a cover tile does not hide the target
	if p, _, _, _ := Effective(grid{{11, 10}: Wall}, 11, 10, 10, 10); p != 0 {
		t.Fatalf("the shooter's own tile: %d", p)
	}
}
