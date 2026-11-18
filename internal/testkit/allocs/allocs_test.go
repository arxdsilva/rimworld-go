// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package allocs

import (
	"math"
	"testing"
)

// A call that allocates nothing measures 0.
func TestFewestOfNothingIsZero(t *testing.T) { // not parallel: AllocsPerRun
	n := 0
	if got := Fewest(10, func() { n++ }); got != 0 {
		t.Fatalf("a call that allocates nothing measures %v", got)
	}
}

// Extra allocations on some calls only (a refill, a growth) do not count:
// the fewest is the allocation every call makes.
func TestFewestIgnoresOccasionalAllocations(t *testing.T) { // not parallel: AllocsPerRun
	keep := make([][]byte, 0, 32)
	calls, extra := 0, 0
	f := func() {
		calls++
		keep = append(keep[:0], make([]byte, 1)) // one of its own, every call
		if calls%3 == 0 {
			extra++
			for range 30 { // a burst of refills now and then
				keep = append(keep, make([]byte, 1))
			}
		}
	}
	if got := Fewest(20, f); got != 1 {
		t.Fatalf("one allocation per call plus occasional ones measures %v, want 1", got)
	}
	if extra == 0 || extra == calls {
		t.Fatalf("%d of %d calls took the extra allocations; want some, not all", extra, calls)
	}
	if got := testing.AllocsPerRun(30, f); got <= 1 {
		t.Fatalf("a 30-call average measures %v: the extras should show in it", got)
	}
}

// No call at all measures +Inf, never 0: an empty sample proves nothing.
func TestFewestOfNoCallsIsInf(t *testing.T) {
	t.Parallel()
	if got := Fewest(0, func() {}); !math.IsInf(got, 1) {
		t.Fatalf("no samples measures %v, want +Inf", got)
	}
}
