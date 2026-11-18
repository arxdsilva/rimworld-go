// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

import "testing"

// Every family set lists keys that parse back to it (PLG-025).
func TestStuffKeysRoundTrip(t *testing.T) {
	t.Parallel()
	for c := StuffCategory(0); c <= AllStuff; c++ {
		var back StuffCategory
		for _, k := range c.Keys() {
			bit, ok := ParseStuff(k)
			if !ok {
				t.Fatalf("ParseStuff(%q) failed", k)
			}
			back |= bit
		}
		if back != c {
			t.Fatalf("%b: keys %v parse to %b", c, c.Keys(), back)
		}
	}
	if _, ok := ParseStuff("glass"); ok {
		t.Fatal("unknown family parsed")
	}
}
