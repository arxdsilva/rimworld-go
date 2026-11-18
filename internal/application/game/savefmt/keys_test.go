// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import "testing"

func TestKeysRemap(t *testing.T) {
	now := map[string]int{"core.a": 1, "core.b": 2, "p.x": 4}
	look := func(k string) (int, bool) { i, ok := now[k]; return i, ok }
	m := Keys{"backstory": {"core.a", "core.b", "q.gone", "p.x"}}.Remap("backstory", look)
	for in, want := range map[int]int{0: 0, 1: 1, 2: 2, 3: 0, 4: 4, 5: 0} {
		if got := m(in); got != want {
			t.Errorf("Remap(%d) = %d, want %d", in, got, want)
		}
	}
	old := Keys(nil).Remap("backstory", look) // a save from before PLG-044
	if old(3) != 3 || old(0) != 0 {
		t.Fatal("a save without keys must keep its indexes")
	}
}
