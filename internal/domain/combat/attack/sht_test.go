// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package attack

import (
	"math/rand"
	"testing"
)

func TestShooterPerTileWikiPoints(t *testing.T) {
	t.Parallel()
	want := map[int]int{
		-20: 700_000, -10: 800_000, -6: 830_000, -4: 850_000, -2: 870_000, 0: 890_000,
		2: 930_000, 4: 940_000, 6: 950_000, 8: 960_000, 10: 970_000, 12: 975_000,
		14: 980_000, 16: 983_333, 18: 986_667, 20: 990_000, 22: 992_500, 26: 995_000,
		30: 996_500, 40: 998_000, 60: 999_000,
		1: 910_000, -5: 840_000, 3: 935_000, 5: 945_000,
	}
	for s, w := range want {
		if got := ShooterPerTile(s); got != w {
			t.Errorf("score %d: %d, want %d", s, got, w)
		}
	}
}

func TestManipAndSightPoints(t *testing.T) {
	t.Parallel()
	for m, w := range map[int]int{1000: 0, 1500: 0, 500: -4, 0: -8} {
		if got := ManipPoints(m); got != w {
			t.Errorf("manip %d: %d, want %d", m, got, w)
		}
	}
	for s, w := range map[int]int{2500: 12, 2000: 12, 1000: 0, 500: -6} {
		if got := SightPoints(s); got != w {
			t.Errorf("sight %d: %d, want %d", s, got, w)
		}
	}
}

func TestDownedFactor(t *testing.T) {
	t.Parallel()
	for d, w := range map[int]Permille{-3: 8500, 0: 8500, 1: 8500, 3: 8500, 4: 1000, 5: 500, 40: 500} {
		if got := DownedFactor(d); got != w {
			t.Errorf("dist %d: %d, want %d", d, got, w)
		}
	}
}

func TestDownedInterceptConst(t *testing.T) {
	t.Parallel()
	if got := InterceptChance(1000) * DownedIntercept / 1000; got != 100 {
		t.Fatalf("downed intercept at 1000: %d, want 100", got)
	}
}

func TestChainMatchesTotal(t *testing.T) {
	t.Parallel()
	check := func(t *testing.T, r Report) {
		t.Helper()
		c := r.Chain()
		if c[StepCover] != r.Total() {
			t.Fatalf("%+v: cover %d != Total %d", r, c[StepCover], r.Total())
		}
		if !r.Melee && c[StepClamp] != r.Aim() {
			t.Fatalf("%+v: clamp %d != Aim %d", r, c[StepClamp], r.Aim())
		}
		n := r
		n.Downed = 1000
		z := r
		z.Downed = 0
		if n.Chain() != z.Chain() {
			t.Fatalf("%+v: Downed 0 != 1000", r)
		}
	}
	t.Run("table", func(t *testing.T) {
		t.Parallel()
		for _, r := range []Report{
			{Band: 900, Shooter: 950, Weather: 1000, Size: 1000, Light: 1000},
			{Band: 900, Shooter: 1000, Weather: 1000, Size: 1000, Light: 1000, Downed: 8500, Cover: 500},
			{Band: 500, Shooter: 1000, Weather: -5, Size: 1000, Light: 1000},
			{Band: 800, Melee: true, Cover: 900},
		} {
			check(t, r)
		}
	})
	t.Run("sweep", func(t *testing.T) {
		t.Parallel()
		rng := rand.New(rand.NewSource(42))
		for range 10_000 {
			check(t, Report{
				Band: Permille(rng.Intn(1200)), Shooter: Permille(rng.Intn(1500)),
				Weather: Permille(rng.Intn(1300) - 100), Size: Permille(rng.Intn(2100)),
				Light: Permille(rng.Intn(1300)), Cover: Permille(rng.Intn(1200) - 100),
				Melee: rng.Intn(10) == 0,
			})
		}
	})
	t.Run("downed clamps", func(t *testing.T) {
		t.Parallel()
		r := Report{Band: 900, Shooter: 1000, Weather: 1000, Size: 1000, Light: 1000, Downed: 8500}
		c := r.Chain()
		if c[StepDowned] != 7650 || c[StepClamp] != 1000 || c[StepCover] != 1000 {
			t.Fatalf("chain %v", c)
		}
	})
	t.Run("steps", func(t *testing.T) {
		t.Parallel()
		r := Report{Band: 800, Shooter: 500, Weather: 500, Size: 1000, Light: 1000, Cover: 250}
		want := [NumSteps]Permille{800, 400, 200, 200, 200, 200, 200, 150}
		if got := r.Chain(); got != want {
			t.Fatalf("chain %v, want %v", got, want)
		}
	})
}
