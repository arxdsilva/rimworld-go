// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scripting

import "testing"

// Every hook has a distinct function name, and Hooks lists each once in
// value order, the cadence hooks last.
func TestHooksNamed(t *testing.T) {
	want := []string{"on_hour", "on_day", "on_incident", "on_colonist_died", "on_pawns", "on_pawn"}
	hs := Hooks()
	if len(hs) != len(want) {
		t.Fatalf("Hooks() = %v, want %d hooks", hs, len(want))
	}
	for i, h := range hs {
		if h != Hook(i+1) || h.String() != want[i] {
			t.Errorf("Hooks()[%d] = %d %q, want %d %q", i, h, h.String(), i+1, want[i])
		}
	}
	if Hook(0).String() != "" || Hook(7).String() != "" {
		t.Error("an unknown hook has a name")
	}
}
