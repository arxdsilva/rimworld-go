// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package plugins

import "testing"

// Problems found while playing (a script turned off, PLG-060) follow the
// loading ones on every read, and never touch the start's slice.
func TestLiveAddsLateProblems(t *testing.T) {
	t.Parallel()
	late := []Problem{}
	v := View{Problems: make([]Problem, 1, 4), Late: func() []Problem { return late }}
	v.Problems[0] = Problem{Plugin: "a", Msg: "bad def", Warn: true}
	if got := v.Live(); len(got.Problems) != 1 || got.Errors() != 0 {
		t.Fatalf("no late problem yet: %+v", got.Problems)
	}
	late = append(late, Problem{Plugin: "b", File: "scripts/x.star", Field: "line 3", Msg: "script turned off"})
	got := v.Live()
	if len(got.Problems) != 2 || got.Errors() != 1 || got.FirstError("b") != "script turned off" {
		t.Fatalf("late problem: %+v", got.Problems)
	}
	if len(v.Problems) != 1 || v.Problems[:2][1].Plugin != "" {
		t.Fatal("Live wrote into the start's problems")
	}
	if (View{}).Live().Problems != nil {
		t.Fatal("no Late: unchanged")
	}
}
