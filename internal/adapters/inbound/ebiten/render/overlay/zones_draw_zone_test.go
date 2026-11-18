// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestZoneLabelTextPrefersTheName(t *testing.T) {
	t.Parallel()
	z := game.ZoneView{Kind: game.ZoneGrow, Crop: "wheat"}
	if got := ZoneLabelText(z); got != "Wheat" {
		t.Fatalf("unnamed label %q", got)
	}
	z.Name = "Back forty"
	if got := ZoneLabelText(z); got != "Back forty" {
		t.Fatalf("named label %q", got)
	}
}
