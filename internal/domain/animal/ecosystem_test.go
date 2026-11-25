// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import "testing"

func TestEcosystemBudgetAtNativeMapScales(t *testing.T) {
	for _, tc := range []struct{ density, size, want int }{
		{370, 100, 370},
		{370, 250, 2312},
		{370, 500, 9250},
		{280, 250, 1750},
		{650, 250, 4062},
		{10, 250, 62},
		{0, 250, 0},
	} {
		if got := EcosystemCapacity(tc.density, tc.size*tc.size); got != tc.want {
			t.Fatalf("density %.2f, %dx%d: %d, want %d hundredths", float64(tc.density)/100, tc.size, tc.size, got, tc.want)
		}
	}
	for _, tc := range []struct {
		species Species
		want    int
	}{
		{Rat, 20}, {Deer, 120}, {Elephant, 400}, {Alpaca, 100}, {GrizzlyBear, 215},
	} {
		if got := EcosystemWeight(tc.species); got != tc.want {
			t.Fatalf("original adult-size cost for %v = %d, want %d", tc.species, got, tc.want)
		}
	}
}

func TestEcosystemResidualAndHabitatWork(t *testing.T) {
	for _, capacity := range []int{2312, 9250} {
		for _, fail := range []bool{false, true} {
			calls := 0
			StartEcosystem(capacity, func(int) int { return 0 }, func(keep func(Species) bool) (Species, bool) {
				if keep(Deer) {
					return Deer, true
				}
				return 0, false
			}, func(s Species, room int) int {
				calls++
				if fail {
					return 0
				}
				return EcosystemWeight(s)
			})
			if fail && calls > StartHerdsMax+32 {
				t.Fatalf("no habitat exhausted %d placement attempts", calls)
			}
			if !fail && calls != capacity*StartCapPct/100/EcosystemWeight(Deer) {
				t.Fatalf("unexpected placed group count %d", calls)
			}
		}
	}
}
