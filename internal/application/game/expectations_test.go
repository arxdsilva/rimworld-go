// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/wealth"
	"rimworld-go/internal/domain/pawn/mood"
)

func TestColonyExpectationsFollowWealth(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 16, Width: 15, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	tests := []struct {
		wealth int
		kind   mood.Kind
		value  int
	}{
		{0, mood.ExpectationsExtremelyLow, 30},
		{14_999, mood.ExpectationsExtremelyLow, 30},
		{15_000, mood.ExpectationsVeryLow, 24},
		{31_000, mood.ExpectationsLow, 18},
		{81_000, mood.ExpectationsModerate, 12},
		{182_000, mood.ExpectationsHigh, 6},
		{308_000, mood.ExpectationsSkyHigh, 0},
	}
	var base int
	for i, tc := range tests {
		g.wealth.Import(&wealth.Save{Counted: true, Value: tc.wealth})
		found := 0
		for _, thought := range g.thoughts(id) {
			if thought.Kind >= mood.ExpectationsExtremelyLow && thought.Kind <= mood.ExpectationsSkyHigh {
				if thought.Kind != tc.kind || thought.Value != tc.value {
					t.Fatalf("wealth %d: expectation %+v", tc.wealth, thought)
				}
				found++
			}
		}
		if found != 1 {
			t.Fatalf("wealth %d: got %d expectation lines", tc.wealth, found)
		}
		if i == 0 {
			base = g.thoughtLevel(id) - tc.value
		}
		if got := g.thoughtLevel(id); got != min(max(base+tc.value, 0), 100) {
			t.Fatalf("wealth %d: target %d, want %d", tc.wealth, got, base+tc.value)
		}
	}
}
