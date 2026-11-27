// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dose

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

func never(int) int { return 999 }

// Drugs.Thoughts reads the given book: a patched drunk thought shows its
// patched line and value, core's shows core's, and a colonist with no drug
// state has none.
func TestThoughtsReadTheGivenBook(t *testing.T) {
	t.Parallel()
	var s drug.State
	s.Ingest(item.Beer, never, 0)
	s.Ingest(item.Beer, never, 0)
	states := map[entity.EntityID]*drug.State{1: &s}
	d := Drugs{states: &states}

	core := mood.Core()
	list := make([]mood.Tuning, core.Len())
	for i := range list {
		def := core.DefOf(mood.Kind(i))
		list[i] = mood.Tuning{Label: def.Label, Value: def.Value, Duration: def.Duration, StackLimit: def.StackLimit, StackFactor: def.StackFactor}
	}
	list[mood.Drunk].Label, list[mood.Drunk].Value = "sozzled", -9
	patched, ok := mood.NewBook(list)
	if !ok {
		t.Fatal("NewBook refused a full list")
	}

	if ts := d.Thoughts(patched, 1); len(ts) != 1 || ts[0].Kind != mood.Drunk || ts[0].Label != "sozzled" || ts[0].Value != -9 {
		t.Fatalf("patched thoughts = %+v", ts)
	}
	want := core.DefOf(mood.Drunk)
	if ts := d.Thoughts(core, 1); len(ts) != 1 || ts[0].Label != want.Label || ts[0].Value != want.Value {
		t.Fatalf("core thoughts = %+v, want %+v", ts, want)
	}
	if ts := d.Thoughts(patched, 2); ts != nil {
		t.Fatalf("no state gave %+v", ts)
	}
}
