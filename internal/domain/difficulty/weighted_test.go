// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import (
	"math"
	"rimworld-go/internal/domain/sim/clock"
	"testing"
)

func TestWeightedThreatPublishedCurves(t *testing.T) {
	s := Of(Seasoned)
	s.Adaptation = false
	s.IgnoreWealth = false
	s.TimeEscalation = false
	for _, c := range []struct {
		w, p, a   float64
		day, want int
	}{
		{14000, 0, 0, 40, 35},
		{400000, 1, 0, 40, 2540},
		{700000, 1, 0, 40, 3770},
		{1000000, 1, 0, 40, 4400},
		{400000, 1, 200, 10, 1789},
		{400000, 1, 200, 25, 2173},
		{400000, 1, 200, 40, 2556},
		{4000000, 100, 0, 40, PointsCap},
	} {
		got := ThreatPointsWeighted(s, ThreatBase{c.w, c.p, c.a}, uint64(c.day)*clock.TicksPerDay, 60, 1)
		if got != c.want {
			t.Fatalf("weighted curve %+v: %d want %d", c, got, c.want)
		}
	}
}
func TestWeightedThreatPawnFactors(t *testing.T) {
	for _, c := range []struct {
		age, hp, w float64
		sleep      bool
	}{
		{3, 1, 0, false}, {8, 1, .25, false}, {13, 1, .5, false}, {18, 1, 1, false},
		{50, .5, .675, false}, {30, 1, .3, true}, {30, .5, .2025, true},
	} {
		if got := PawnWeight(c.age, c.hp, c.sleep); math.Abs(got-c.w) > 1e-9 {
			t.Fatalf("%+v weight=%g", c, got)
		}
	}
}
func TestWeightedThreatCustomMode(t *testing.T) {
	s := Of(Seasoned)
	s.IgnoreWealth = true
	s.TimeEscalation = true
	s.EscPerYear = 100
	s.Adaptation = true
	ticks := uint64(80) * clock.TicksPerDay
	b := ThreatBase{Wealth: 1000000, PawnWeight: 2}
	if got, want := ThreatPointsWeighted(s, b, ticks, 80, 1.2), ThreatPoints(s, 0, 2, 1, 20); got != want {
		t.Fatalf("custom %d want %d", got, want)
	}
}

func TestWeightedThreatSeparateScopeArithmetic(t *testing.T) {
	s := Of(Seasoned)
	s.Adaptation = false
	s.IgnoreWealth = false
	s.TimeEscalation = false
	tick := uint64(40) * clock.TicksPerDay
	first := ThreatPointsWeighted(s, ThreatBase{Wealth: 400000, PawnWeight: 1}, tick, 60, 1)
	second := ThreatPointsWeighted(s, ThreatBase{Wealth: 400000, PawnWeight: 1}, tick, 60, 1)
	if got := first + second; got != 5080 {
		t.Fatalf("separate maps: %d, want 5080", got)
	}
	pooled := ThreatPointsWeighted(s, ThreatBase{Wealth: 800000, PawnWeight: 2}, tick, 60, 1)
	if pooled == first+second {
		t.Fatal("pooled wealth must not replace separate scopes")
	}
}

func TestInjuryWeightAnchors(t *testing.T) {
	for _, c := range []struct{ health, want float64 }{{-1, .35}, {0, .35}, {.2, .48}, {.5, .675}, {.8, .87}, {1, 1}, {2, 1}} {
		if got := InjuryWeight(c.health); math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("health %v: got %v, want %v", c.health, got, c.want)
		}
	}
}
