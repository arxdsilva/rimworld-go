// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package research

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	proj "rimworld-go/internal/domain/research"
)

// TestFinishTellsTheHostOnce: both ways a project completes (bench work
// and the dev finish) tell the host its key once (the plugins'
// research_done); picking and working before the end tell nothing.
func TestFinishTellsTheHostOnce(t *testing.T) {
	t.Parallel()
	f := newFake()
	l := New(f)
	bench := f.add(construction.ElectricResearchBench, 0, 0)
	l.Pick(proj.Bowcraft)
	for i := 0; i < 400*2500 && l.Set; i++ {
		f.now++
		l.Work(1, bench)
		if l.Set && len(f.finished) != 0 {
			t.Fatalf("told %v before the end", f.finished)
		}
	}
	l.Pick(proj.Traps)
	l.DevFinish()
	l.DevFinish() // nothing picked: no second word
	want := []string{f.book.Key(proj.Bowcraft), f.book.Key(proj.Traps)}
	if want[0] == "" || !slices.Equal(f.finished, want) {
		t.Fatalf("finished %q, want %q", f.finished, want)
	}
}
