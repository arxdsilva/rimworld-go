// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

// RateStep returns this tick's share of a periodic needs rate. Differences
// of cumulative integer units preserve the exact total each period.
func RateStep(now uint64, per, period int) int {
	if now == 0 {
		return 0
	}
	p, q := uint64(per), uint64(period)
	return int(now*p/q - (now-1)*p/q)
}

// EatTicks is the duration of one meal for each food target type.
func EatTicks(k FeedKind) int {
	switch k {
	case FeedTree:
		return TreeTicks
	case FeedCarcass:
		return CarcassTicks
	case FeedItem, FeedTrough:
		return ItemTicks
	}
	return GrazeTicks
}

// FeedReach is the allowed distance from a meal to stand while eating.
func FeedReach(t FeedTarget) int {
	if t.Kind == FeedGrass {
		return 0
	}
	return 1
}
