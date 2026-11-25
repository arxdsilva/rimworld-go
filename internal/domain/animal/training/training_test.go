// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package training

import "testing"

func TestStepsAndLearned(t *testing.T) {
	t.Parallel()
	if Needed(Obedience) != 3 || Needed(Guard) != 3 || Needed(Release) != 2 || Needed(NumSkills) != 0 {
		t.Fatal("steps")
	}
	if !Learned(Release, 2) || Learned(Release, 1) || Learned(NumSkills, 9) {
		t.Fatal("learned")
	}
}

func TestStepChance(t *testing.T) {
	t.Parallel()
	if StepChance(0, false) != 100 || StepChance(10, false) != 350 {
		t.Fatal("plain")
	}
	if StepChance(10, true) != 1000 || StepChance(0, true) != 500 {
		t.Fatalf("bonded %d %d", StepChance(10, true), StepChance(0, true))
	}
	if StepChance(-4, false) != 100 || StepChance(99, false) != 600 {
		t.Fatal("clamp")
	}
}

func TestDecayInterval(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wildness int
		interval uint64
	}{
		{-1, DecayMaxTicks}, {0, DecayMaxTicks}, {500, 9 * 60000},
		{1000, DecayMinTicks}, {1500, DecayMinTicks},
	} {
		if got := DecayInterval(tc.wildness); got != tc.interval {
			t.Fatalf("wildness %d: %d, want %d", tc.wildness, got, tc.interval)
		}
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()
	if BondTame != 10 || BondTrain != 7 || BondTend != 4 || BondProximity != 1 {
		t.Fatal("bond")
	}
}
