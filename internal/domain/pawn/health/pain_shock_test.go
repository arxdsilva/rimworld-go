// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package health

import "testing"

// TestPainShockConstants pins wiki: Pain (shock at 80 %); CBT 3.4 leaves it.
func TestPainShockConstants(t *testing.T) {
	if PainShock != 800 || PainShockBand != 50 {
		t.Fatalf("PainShock %d band %d", PainShock, PainShockBand)
	}
}
