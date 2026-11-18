// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package allocs measures a call's own allocations for tests that compare
// a draw with Ebitengine's own call (no allocation of the caller's over it).
//
// Each Ebitengine draw takes a scratch slice from a sync.Pool twice
// (Image.invokeUsageCallbacks, for the target and the source). The race
// build drops 1 Put in 4 (sync.Pool.Put), and the next Get's New allocates
// 2, so an average over many calls (testing.AllocsPerRun(n) for n > 1)
// carries a random share of refills and rounds either way: the same code
// measures 3 one run and 4 the next. One call at a time the count is exact,
// and a refill, a slice growth or a background allocation only adds, so
// the fewest over many single calls is the call's own cost. An allocation
// of the caller's is in every call, so the fewest still shows it.
package allocs

import (
	"math"
	"testing"
)

// Fewest is the fewest allocations of n single calls of f, each measured
// by testing.AllocsPerRun(1, f) (a warm-up call, then the counted one).
// Pick n so that some call meets no refill: with odds p per call that one
// does, all n missing has odds (1-p)^n. Not for parallel tests (AllocsPerRun).
func Fewest(n int, f func()) float64 {
	least := math.Inf(1)
	for range n {
		least = min(least, testing.AllocsPerRun(1, f))
	}
	return least
}
