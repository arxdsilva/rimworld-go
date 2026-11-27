// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package contentid

import (
	"testing"

	"rimworld-go/internal/domain/pawn/mood"
)

// Core's thought book, zero or built, adds nothing to the fingerprint; a
// patched thought changes it.
func TestThoughtsInFingerprint(t *testing.T) {
	t.Parallel()
	core := mood.Core()
	list := make([]mood.Tuning, core.Len())
	for i := range list {
		d := core.DefOf(mood.Kind(i))
		list[i] = mood.Tuning{Label: d.Label, Value: d.Value, Duration: d.Duration, StackLimit: d.StackLimit, StackFactor: d.StackFactor}
	}
	built, ok := mood.NewBook(list)
	if !ok {
		t.Fatal("NewBook refused core's list")
	}
	if Of(nil, Books{Thoughts: core}).Hash != Fingerprint() || Of(nil, Books{Thoughts: built}).Hash != Fingerprint() || Of(nil, Books{}).Hash != Fingerprint() {
		t.Fatal("core's thought book moved the fingerprint")
	}
	list[mood.SleptOutside].Value++
	patched, _ := mood.NewBook(list)
	if Of(nil, Books{Thoughts: patched}).Hash == Fingerprint() {
		t.Fatal("a patched thought left the fingerprint as it was")
	}
	if l := thoughtList(patched); len(l.items) != 1 || l.items[0].key != patched.Key(mood.SleptOutside) {
		t.Fatalf("list = %+v", l)
	}
}
