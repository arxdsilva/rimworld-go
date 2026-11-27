// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package devtools

import "testing"

// TestWindfallEntriesInDebugMenu: ore meteor, supply drop and sky song are
// each a debug entry that fires its incident key.
func TestWindfallEntriesInDebugMenu(t *testing.T) {
	t.Parallel()
	f := &fakeShell{layout: devLayouts()["idle 1280x720"]}
	f.env = Env{On: true}
	acts := New(f).Actions(1, 1)
	for label, key := range map[string]string{
		"Event: ore meteor": "ore_meteor", "Event: supply drop": "supply_drop", "Event: sky song": "sky_song",
	} {
		found := false
		for _, a := range acts {
			if a.Label != label {
				continue
			}
			found = true
			log := &fireLog{}
			if msg := a.Run(log); len(log.keys) != 1 || log.keys[0] != key || msg == "" {
				t.Errorf("%s: fired %v, msg %q", label, log.keys, msg)
			}
		}
		if !found {
			t.Errorf("no %q entry", label)
		}
	}
}
