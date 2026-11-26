// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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

// ShootingScore is the ShootingAccuracyPawn stat before its curve: skill plus
// (min(Manipulation,100%)-100%)x8 and (min(Sight,200%)-100%)x12 plus points.
func TestShootingScoreBodyCapacitiesAndBonus(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ manipulation, sight, bonus, want int }{
		{0, 1000, 0, 2}, {500, 1000, 0, 6}, {1000, 1000, 0, 10}, {1500, 1000, 0, 10},
		{1000, 0, 0, -2}, {1000, 500, 0, 4}, {1000, 1500, 0, 16},
		{1000, 2000, 0, 22}, {1000, 2500, 0, 22}, {500, 1500, 0, 12},
		{1000, 1000, 12, 22}, {1000, 1000, -10, 0},
	} {
		if got := ShootingScore(10, ManipPoints(tc.manipulation)+SightPoints(tc.sight)+tc.bonus); got != tc.want {
			t.Errorf("ShootingScore(10, %d, %d, %d) = %d want %d", tc.manipulation, tc.sight, tc.bonus, got, tc.want)
		}
	}
}

func TestShooterPerTile(t *testing.T) {
	t.Parallel()
	for skill, want := range map[int]int{
		-30: 700_000, -20: 700_000, -10: 800_000, -6: 830_000, -5: 840_000, -4: 850_000,
		-2: 870_000, 0: 890_000, 1: 910_000, 2: 930_000, 3: 935_000, 4: 940_000,
		5: 945_000, 6: 950_000, 8: 960_000, 10: 970_000, 12: 975_000, 14: 980_000,
		15: 981_666, 16: 983_333, 17: 984_999, 18: 986_666, 19: 988_333, 20: 990_000,
		22: 992_500, 24: 993_750, 26: 995_000, 28: 995_750,
		30: 996_500, 40: 998_000, 60: 999_000, 99: 999_000,
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
	if first, again := hc(20, 940_000, 550, 700, n, 950, n), hc(20, 940_000, 550, 700, n, 950, n); first != again {
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

func TestShootingScore(t *testing.T) {
	levels := []int{0, 10, 20}
	points := []int{-5, 0, 5}
	want := [3][3]int{
		// Re-pinned for the full published curve (SHT-004): -5 is 84.0 (was
		// 84.5), 15 is 981_666 (integer interpolation 14..16) and 25 is
		// 994_375 (22..26).
		{840_000, 890_000, 945_000},
		{945_000, 970_000, 981_666},
		{981_666, 990_000, 994_375},
	}
	for i, l := range levels {
		for j, p := range points {
			if got := ShooterPerTile(ShootingScore(l, p)); got != want[i][j] {
				t.Errorf("level %d points %d: got %d, want %d", l, p, got, want[i][j])
			}
		}
	}
	if s := ShootingScore(0, -30); s != -30 || ShooterPerTile(s) != 700_000 {
		t.Errorf("score -30: %d -> %d", s, ShooterPerTile(s))
	}
}
