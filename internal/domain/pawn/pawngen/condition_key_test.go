// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package pawngen

import "testing"

// TestConditionKeysPinned: every starting condition has its stable,
// namespaced key (stat_mod conditions match it), none and unknown have
// none, and no two conditions share a key.
func TestConditionKeysPinned(t *testing.T) {
	t.Parallel()
	want := map[Condition]string{
		NoCondition: "", OldFracture: "core.old_fracture", Nearsighted: "core.nearsighted",
		HandTremor: "core.hand_tremor", Exhausted: "core.exhausted", numConditions: "", 200: "",
	}
	for c, k := range want {
		if got := c.Key(); got != k {
			t.Errorf("Condition(%d).Key() = %q, want %q", c, got, k)
		}
	}
	seen := map[string]Condition{}
	for c := NoCondition + 1; c < numConditions; c++ {
		k := c.Key()
		if k == "" {
			t.Errorf("Condition(%d) has no key", c)
		}
		if o, dup := seen[k]; dup {
			t.Errorf("Conditions %d and %d share key %q", o, c, k)
		}
		seen[k] = c
	}
}
