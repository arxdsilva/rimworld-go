// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scripting

import "testing"

// Every event has a distinct script name that parses back to it; the
// zero value and values past the last have none.
func TestEventKindRoundTrip(t *testing.T) {
	want := []string{"quadrum", "year", "research_done", "recipe_done", "map_generated"}
	ks := EventKinds()
	if len(ks) != len(want) {
		t.Fatalf("EventKinds() = %v, want %d kinds", ks, len(want))
	}
	for i, k := range ks {
		if k != EventKind(i+1) || k.String() != want[i] {
			t.Errorf("EventKinds()[%d] = %d %q, want %d %q", i, k, k.String(), i+1, want[i])
		}
		if got, ok := ParseEventKind(k.String()); !ok || got != k {
			t.Errorf("ParseEventKind(%q) = %d, %v, want %d", k.String(), got, ok, k)
		}
	}
	if EventKind(0).String() != "" || EventKind(len(want)+1).String() != "" {
		t.Error("an unknown event has a name")
	}
	for _, s := range []string{"", "Quadrum", "on_event", "nope"} {
		if _, ok := ParseEventKind(s); ok {
			t.Errorf("ParseEventKind(%q) found an event", s)
		}
	}
}

// The bounds are part of the plugin contract.
func TestExtConstsPinned(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"MaxQueryRows", MaxQueryRows, 256},
		{"MaxPanelLines", MaxPanelLines, 12},
		{"MaxActions", MaxActions, 4},
		{"OrderEvery", OrderEvery, 10},
		{"PawnEvery", PawnEvery, 60},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// The Hook table still names the same six hooks in the same order: the
// extension hooks are host-level names, never Hook values.
func TestHookTableUnchanged(t *testing.T) {
	want := []string{"on_hour", "on_day", "on_incident", "on_colonist_died", "on_pawns", "on_pawn"}
	var got []string
	for _, h := range Hooks() {
		got = append(got, h.String())
	}
	if len(got) != len(want) {
		t.Fatalf("Hooks() names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Hooks() names = %v, want %v", got, want)
		}
	}
	if Hook(len(want)+1).String() != "" || Hook(255).String() != "" {
		t.Error("a hook past the table has a name")
	}
}
