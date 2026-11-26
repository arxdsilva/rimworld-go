// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package expectations

import "testing"

func TestWealthBands(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wealth int
		band   Band
		mood   int
		decay  int
	}{
		{0, ExtremelyLow, 30, 18},
		{14999, ExtremelyLow, 30, 18},
		{15000, VeryLow, 24, 13},
		{30999, VeryLow, 24, 13},
		{31000, Low, 18, 11},
		{80999, Low, 18, 11},
		{81000, Moderate, 12, 10},
		{181999, Moderate, 12, 10},
		{182000, High, 6, 8},
		{307999, High, 6, 8},
		{308000, SkyHigh, 0, 7},
		{1_000_000_000, SkyHigh, 0, 7},
	} {
		band := ForWealth(tc.wealth)
		e := band.Effects()
		if band != tc.band || e.MoodBonus != tc.mood || e.ToleranceDecayPct != tc.decay {
			t.Fatalf("wealth %d: band %d effects %+v", tc.wealth, band, e)
		}
	}
}
