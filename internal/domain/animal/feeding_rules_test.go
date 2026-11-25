// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import "testing"

func TestFeedingRatesAndReach(t *testing.T) {
	for _, tc := range []struct {
		kind FeedKind
		want int
	}{
		{FeedGrass, GrazeTicks}, {FeedTree, TreeTicks},
		{FeedCarcass, CarcassTicks}, {FeedItem, ItemTicks},
		{FeedTrough, ItemTicks},
	} {
		if got := EatTicks(tc.kind); got != tc.want {
			t.Fatalf("food %v takes %d ticks, want %d", tc.kind, got, tc.want)
		}
	}
	if FeedReach(FeedTarget{Kind: FeedGrass}) != 0 ||
		FeedReach(FeedTarget{Kind: FeedCarcass}) != 1 {
		t.Fatal("feeding reach changed")
	}
	const period, rate = 97, 31
	sum := 0
	for tick := uint64(1); tick <= period; tick++ {
		sum += RateStep(tick, rate, period)
	}
	if sum != rate || RateStep(0, rate, period) != 0 {
		t.Fatalf("needs rate sum %d, want %d", sum, rate)
	}
}
