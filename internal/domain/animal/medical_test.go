// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
	"testing"
)

// The public human curve is evaluated on birthdays; animal lifespan
// normalization is retained native tuning.
func TestAsthmaBirthdayIncidence(t *testing.T) {
	for _, tc := range []struct{ age, life, chance int }{
		{0, 100, 0},
		{10, 100, 240},
		{20, 100, 480},
		{25, 100, 720},
		{30, 100, 960},
		{40, 100, 1152},
		{50, 100, 1344},
		{90, 100, 1344},
	} {
		if got := body.AsthmaBirthdayChance(tc.age, tc.life); got != tc.chance {
			t.Fatalf("age %d/%d: chance %d, want %d", tc.age, tc.life, got, tc.chance)
		}
	}

	now := uint64(3000 * clock.TicksPerDay)
	ageDays := LifeOf(Cow).LifeDays / (2 * clock.DaysPerYear) * clock.DaysPerYear
	if ageDays < clock.DaysPerYear {
		t.Fatal("expected an adult cow with at least one birthday")
	}
	newCow := func(age int) State {
		return State{Species: Cow, Born: BornAt(now, uint64(age)*clock.TicksPerDay)}
	}
	success := newCow(ageDays)
	draws := 0
	if !RollAsthmaBirthday(&success, now, func(n int) int {
		draws++
		if n != 1_000_000 {
			t.Fatalf("chance scale: %d", n)
		}
		return 0
	}) || draws != 1 || len(success.Body.Conds) != 2 {
		t.Fatalf("one successful birthday must affect both lungs: draws=%d, body=%+v", draws, success.Body)
	}
	if RollAsthmaBirthday(&success, now, func(n int) int { draws++; return 0 }) {
		t.Fatal("same asthma incident re-applied")
	}

	failed := newCow(ageDays)
	draws = 0
	if RollAsthmaBirthday(&failed, now, func(n int) int { draws++; return n - 1 }) ||
		failed.Body != nil || draws != 1 {
		t.Fatalf("failed birthday roll created illness: %+v, draws=%d", failed.Body, draws)
	}

	notBirthday := newCow(ageDays - 1)
	draws = 0
	if RollAsthmaBirthday(&notBirthday, now, func(int) int { draws++; return 0 }) ||
		notBirthday.Body != nil || draws != 0 {
		t.Fatal("illness checked between birthdays")
	}
	legacy := State{Species: Cow, Born: 0}
	if RollAsthmaBirthday(&legacy, now, func(int) int { t.Fatal("legacy unknown age rolled"); return 0 }) {
		t.Fatal("legacy animal with unknown birthday developed age-related asthma")
	}
	early := newCow(0)
	if RollAsthmaBirthday(&early, now, func(int) int { draws++; return 0 }) {
		t.Fatal("newborn had an asthma birthday")
	}
}

func TestAsthmaAnatomyBirthday(t *testing.T) {
	now := uint64(3000 * clock.TicksPerDay)
	for _, tc := range []struct {
		species Species
		lungs   int
	}{
		{Cow, 2}, {Chicken, 2}, {Cobra, 1}, {Iguana, 2}, {HiveBeetle, 0}, {Harrow, 0},
	} {
		age := LifeOf(tc.species).LifeDays / (2 * clock.DaysPerYear) * clock.DaysPerYear
		if age < clock.DaysPerYear {
			continue
		}
		a := State{Species: tc.species, Born: BornAt(now, uint64(age)*clock.TicksPerDay)}
		got := RollAsthmaBirthday(&a, now, func(int) int { return 0 })
		if got != (tc.lungs > 0) {
			t.Errorf("%s onset %t; lungs=%d", tc.species, got, tc.lungs)
		}
		if tc.lungs > 0 && len(a.Body.Conds) != tc.lungs {
			t.Errorf("%s conditions=%d, lungs=%d", tc.species, len(a.Body.Conds), tc.lungs)
		}
	}
}

// Severe asthma affects manipulation through consciousness, alongside
// locomotion and rest restoration.
func TestBilateralAsthmaBodyCapacities(t *testing.T) {
	a := State{Species: Cow}
	if !DevelopAsthma(&a, 1) {
		t.Fatal("no animal asthma")
	}
	for i := range a.Body.Conds {
		a.Body.Conds[i].Sev = 500
		a.Body.Conds[i].Stage = 2
	}
	cases := []struct {
		cap  body.Capacity
		want int32
	}{
		{body.Breathing, 500},
		{body.Consciousness, 900},
		{body.Moving, 810},
		{body.Manipulation, 900},
	}
	for _, tc := range cases {
		if got := a.Body.Capacity(tc.cap); got != tc.want {
			t.Errorf("%v capacity=%d, want %d", tc.cap, got, tc.want)
		}
	}
	if got := body.RestRateMultiplier(a.Body); got != 850 {
		t.Errorf("asthma rest multiplier=%d, want 850", got)
	}
}
