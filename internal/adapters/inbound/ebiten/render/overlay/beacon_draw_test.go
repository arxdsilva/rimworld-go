// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package overlay

import "testing"

func TestBeaconGlowOnlyWhilePowered(t *testing.T) {
	t.Parallel()
	for _, reduced := range []bool{false, true} {
		lit := false
		for p := uint64(0); p < BeaconBlinkFrames; p++ {
			if BeaconGlowOn(false, p, 3, reduced) {
				t.Fatalf("unpowered beacon glows (present %d, reduced %v)", p, reduced)
			}
			lit = lit || BeaconGlowOn(true, p, 3, reduced)
		}
		if !lit {
			t.Errorf("powered beacon never glows (reduced %v)", reduced)
		}
	}
}
