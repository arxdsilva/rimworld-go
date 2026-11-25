// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package attack

import "testing"

func TestShooterAndSizeFactors(t *testing.T) {
	t.Parallel()
	if ShooterFactor(PerfectShooter, 30) != 1000 || ShooterFactor(ShooterPerTile(10), 0) != 1000 {
		t.Fatal("perfect shooter / no distance")
	}
	// 0.97^12 = 0.6938
	if got := ShooterFactor(ShooterPerTile(10), 12); got < 692 || got > 694 {
		t.Fatalf("skill 10 at 12: %d", got)
	}
	for in, want := range map[Permille]Permille{100: 500, 500: 500, 1000: 1000, 1800: 1800, 4000: 2000} {
		if SizeFactor(in) != want {
			t.Errorf("SizeFactor(%d) = %d", in, SizeFactor(in))
		}
	}
}

func TestReportAimAndTotal(t *testing.T) {
	t.Parallel()
	r := Report{Band: 800, Shooter: 1000, Weather: 1000, Size: 1000, Light: 1000}
	if r.Aim() != 800 || r.Total() != 800 {
		t.Fatalf("neutral %d %d", r.Aim(), r.Total())
	}
	r.Cover = 550
	if r.Total() != 800*450/1000 {
		t.Fatalf("cover %d", r.Total())
	}
	r.Weather, r.Size = FogFactor, 500
	if r.Aim() != 200 {
		t.Fatalf("fog and size %d", r.Aim())
	}
	// SHT-002: Aim is the pure product, clamped; no flat term after range
	if hi := (Report{Band: 1000, Shooter: 1000, Weather: 2000, Size: 2000, Light: 1000}); hi.Aim() != 1000 {
		t.Fatalf("product clamps high: %d", hi.Aim())
	}
	if lo := (Report{Band: 800, Shooter: 1000, Weather: -500, Size: 1000, Light: 1000}); lo.Aim() != 0 {
		t.Fatalf("negative factor clamps low: %d", lo.Aim())
	}
	if p := (Report{Band: 900, Shooter: 800, Weather: 500, Size: 1500, Light: 800}); p.Aim() != 900*800/1000*500/1000*1500/1000*800/1000 {
		t.Fatalf("pure product: %d", p.Aim())
	}
	if m := (Report{Melee: true, Band: 850, Cover: 900}); m.Total() != 850 {
		t.Fatal("melee ignores cover")
	}
	// Report matches HitChance's product without cover
	b := DefaultBands(650, 800, 900, 800)
	rr := Report{Band: b.Accuracy(20), Shooter: ShooterFactor(ShooterPerTile(8), 20), Weather: 1000, Size: 1000, Light: 1000}
	if hc := HitChance(b, 20, ShooterPerTile(8), 0, 1000, 1000, 1000, 1000); rr.Aim() < hc-1 || rr.Aim() > hc+1 {
		t.Fatalf("aim %d vs HitChance %d", rr.Aim(), hc)
	}
}

func TestMissRadius(t *testing.T) {
	t.Parallel()
	for aim, want := range map[Permille]int{1000: 1, 999: 1, 220: 2, 110: 4, 90: 5, 70: 6, 40: 8, 20: 10, 0: 10} {
		if got := MissRadius(aim); got != want {
			t.Errorf("MissRadius(%d) = %d want %d", aim, got, want)
		}
	}
	for a := Permille(0); a < 1000; a += 7 {
		if MissRadius(a) < MissRadius(a+7) {
			t.Fatalf("miss radius grows with aim at %d", a)
		}
	}
}

func TestInterceptChance(t *testing.T) {
	t.Parallel()
	for size, want := range map[Permille]Permille{1000: 400, 50: 40, 3000: 800, 500: 200} {
		if InterceptChance(size) != want {
			t.Errorf("InterceptChance(%d) = %d", size, InterceptChance(size))
		}
	}
}

// Interception is 0 up to 5 tiles from the muzzle, full from 12 tiles and
// linear in the distance between (wiki: Combat#Interception; no formula given).
func TestDistanceInterceptFactor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dx, dy int
		want   Permille
	}{
		{0, 0, 0}, {5, 0, 0}, {3, 4, 0}, {4, 4, 93}, {6, 0, 142}, {8, 0, 428}, {12, 0, 1000}, {20, 3, 1000}, {-9, 0, 571},
	} {
		if got := DistanceInterceptFactor(tc.dx, tc.dy); got != tc.want {
			t.Errorf("DistanceInterceptFactor(%d,%d) = %d, want %d", tc.dx, tc.dy, got, tc.want)
		}
	}
}

func TestDownedInterceptChance(t *testing.T) {
	t.Parallel()
	for chance, want := range map[Permille]Permille{0: 0, 40: 4, 171: 17, 400: 40} {
		if got := DownedInterceptChance(chance); got != want {
			t.Errorf("DownedInterceptChance(%d) = %d, want %d", chance, got, want)
		}
	}
}

func TestFriendlyFireChance(t *testing.T) {
	t.Parallel()
	for percent, want := range map[int]Permille{-1: 0, 0: 0, 40: 160, 100: 400, 101: 400} {
		if got := FriendlyFireChance(400, percent); got != want {
			t.Errorf("FriendlyFireChance(400, %d) = %d, want %d", percent, got, want)
		}
	}
}

// A non-standing target is hit 8.5x as often within 3 tiles and half as
// often from 5 tiles: wiki: Downed (+750% execution bonus).
func TestDownedFactorLocalTable(t *testing.T) {
	t.Parallel()
	for distance, want := range map[int]Permille{0: 8500, 3: 8500, 4: 1000, 5: 500, 40: 500} {
		if got := DownedFactor(distance); got != want {
			t.Errorf("DownedFactor(%d) = %d, want %d", distance, got, want)
		}
	}
}

func TestDownedTargetReportAccuracy(t *testing.T) {
	t.Parallel()
	r := Report{Band: 100, Shooter: 1000, Weather: 1000, Size: 1000, Light: 1000, Downed: DownedFactor(3)}
	for distance, want := range map[int]Permille{3: 850, 4: 100, 5: 50} {
		r.Dist, r.Downed = distance, DownedFactor(distance)
		if got := r.Aim(); got != want {
			t.Errorf("downed aim at %d tiles = %d, want %d", distance, got, want)
		}
	}
	r.Band, r.Dist, r.Downed = 200, 3, DownedFactor(3)
	if got := r.Aim(); got != 1000 {
		t.Fatalf("execution bonus clamps aim at 1000, got %d", got)
	}
	if got := (Report{Melee: true, Band: 400, Downed: DownedFactor(3), Dist: 3}).Total(); got != 400 {
		t.Fatalf("downed ranged bonus changed melee chance to %d", got)
	}
}
