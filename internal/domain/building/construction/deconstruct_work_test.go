// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

import "testing"

func TestDeconstructRemaining(t *testing.T) {
	t.Parallel()
	unmarked := Building{Kind: Wall}
	if got := DeconstructRemaining(unmarked); got != DeconstructTicks(Wall) {
		t.Fatalf("unmarked wall work = %d, want %d", got, DeconstructTicks(Wall))
	}
	marked := Building{Kind: Wall, Deconstruct: true, WorkLeft: 17}
	if got := DeconstructRemaining(marked); got != 17 {
		t.Fatalf("marked wall work = %d, want 17", got)
	}
	marked.WorkLeft = 0
	if got := DeconstructRemaining(marked); got != 0 {
		t.Fatalf("completed deconstruction work = %d, want zero", got)
	}
}
