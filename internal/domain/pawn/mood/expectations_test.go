// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import "testing"

func TestExpectationsWealthBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wealth int
		kind   Kind
		value  int
	}{
		{-10, ExpectationsExtremelyLow, 30},
		{0, ExpectationsExtremelyLow, 30},
		{14_999, ExpectationsExtremelyLow, 30},
		{15_000, ExpectationsVeryLow, 24},
		{30_999, ExpectationsVeryLow, 24},
		{31_000, ExpectationsLow, 18},
		{80_999, ExpectationsLow, 18},
		{81_000, ExpectationsModerate, 12},
		{181_999, ExpectationsModerate, 12},
		{182_000, ExpectationsHigh, 6},
		{307_999, ExpectationsHigh, 6},
		{308_000, ExpectationsSkyHigh, 0},
		{1_000_000_000, ExpectationsSkyHigh, 0},
	} {
		if k := ExpectationsForWealth(tc.wealth); k != tc.kind {
			t.Errorf("wealth %d: kind %d, want %d", tc.wealth, k, tc.kind)
		}
		if d := DefOf(tc.kind); d.Value != tc.value || d.Duration != 0 || d.Label == "" {
			t.Errorf("wealth %d: def %+v", tc.wealth, d)
		}
	}
	if Base != 32 {
		t.Fatalf("mood base %d, want 32", Base)
	}
}
