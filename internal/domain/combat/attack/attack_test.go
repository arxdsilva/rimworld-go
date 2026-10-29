package attack

import "testing"

var rifle = DefaultBands(600, 800, 500, 200)

func TestAccuracyBreakpoints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		d    int
		want Permille
	}{
		{0, 600}, {3, 600}, {12, 800}, {25, 500}, {40, 200}, {99, 200},
		{7, 600 + 200*4/9}, // 4 of 9 tiles from touch to short
		{18, 800 - 300*6/13},
	} {
		if got := rifle.Accuracy(tc.d); got != tc.want {
			t.Errorf("Accuracy(%d)=%d want %d", tc.d, got, tc.want)
		}
	}
	if !(Bands{}).Zero() || rifle.Zero() {
		t.Fatal("Zero")
	}
}

func TestShooterPerTile(t *testing.T) {
	t.Parallel()
	for skill, want := range map[int]int{
		-30: 700_000, -20: 700_000, -10: 800_000, 0: 890_000, 5: 945_000, 10: 970_000,
		15: 981_667, 20: 990_000, 30: 996_500, 40: 998_000, 60: 999_000, 99: 999_000,
		2: 890_000 + 55_000*2/5,
	} {
		if got := ShooterPerTile(skill); got != want {
			t.Errorf("skill %d: %d want %d", skill, got, want)
		}
	}
}

func TestHitChance(t *testing.T) {
	t.Parallel()
	n, pf := Permille(1000), PerfectShooter
	hc := func(d, pt, cov, size, light, weather, smoke int) Permille {
		return HitChance(rifle, d, pt, cov, size, light, weather, smoke)
	}
	if got := hc(12, pf, 0, n, n, n, n); got != 800 {
		t.Fatalf("neutral = %d", got)
	}
	if got := hc(12, pf, 550, n, n, n, n); got != 800*450/1000 {
		t.Fatalf("sandbags = %d", got)
	}
	if got := hc(12, pf, 0, n, n, FogFactor, n); got != 400 {
		t.Fatalf("fog = %d", got)
	}
	if got := hc(12, pf, 0, n, n, n, SmokeFactor); got != 240 {
		t.Fatalf("smoke = %d", got)
	}
	// skill 10 at 12 tiles: 0.97^12 = 0.6938 -> 800 * 0.6938 = 555.
	if got := hc(12, ShooterPerTile(10), 0, n, n, n, n); got < 553 || got > 556 {
		t.Fatalf("skill 10 = %d", got)
	}
	if hc(12, ShooterPerTile(0), 0, n, n, n, n) >= hc(12, ShooterPerTile(10), 0, n, n, n, n) {
		t.Fatal("skill must help")
	}
	if got := hc(12, pf, 0, 2000, n, n, n); got != 1000 {
		t.Fatalf("clamped high = %d", got)
	}
	if got := hc(12, pf, 1000, n, n, n, n); got != 0 {
		t.Fatalf("full cover = %d", got)
	}
	if got := hc(12, pf, 2000, n, n, n, n); got != 0 {
		t.Fatalf("cover clamped = %d", got)
	}
	if got := hc(12, pf, 0, 500, 800, 900, n); got != 800*900/1000*500/1000*800/1000 {
		t.Fatalf("product = %d", got)
	}
	if hc(20, 940_000, 550, 700, n, 950, n) != hc(20, 940_000, 550, 700, n, 950, n) {
		t.Fatal("determinism")
	}
	if hc(0, ShooterPerTile(0), 0, n, n, n, n) != 600 {
		t.Fatal("dist 0 has no decay")
	}
}

func TestBurstAndCycle(t *testing.T) {
	t.Parallel()
	if (Burst{}).Count() != 1 || (Burst{}).Duration() != 0 {
		t.Fatal("zero burst is one shot")
	}
	b := Burst{Shots: 3, BetweenTicks: 6}
	if b.Count() != 3 || b.Duration() != 12 {
		t.Fatal("burst")
	}
	if Cycle(Timing{Warmup: 30, Cooldown: 90}, b) != 132 {
		t.Fatal("cycle")
	}
}
