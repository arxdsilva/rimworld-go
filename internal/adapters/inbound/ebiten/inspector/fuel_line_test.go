// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inspector

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
)

// TestFuelLineShowsLogs: a wood burner's pane says its logs against the
// tank (wiki: Fueled stove holds 50); other burners say nothing of logs.
func TestFuelLineShowsLogs(t *testing.T) {
	t.Parallel()
	tr := lang.English{}
	b := game.BuildingView{Kind: "stove", Fuel: true, FuelPct: 24, FuelLogs: 12, FuelCapLogs: 50, Refuelable: true, AutoRefuel: true}
	if got := fuelLine(tr, b); got != "12 / 50 wood" {
		t.Errorf("stove line %q", got)
	}
	b.Burning = true
	if got := fuelLine(tr, b); got != "Burning, 12 / 50 wood" {
		t.Errorf("burning stove line %q", got)
	}
	b.FuelCapLogs = 0
	if got := fuelLine(tr, b); got != "Burning" {
		t.Errorf("chemfuel burner line %q", got)
	}
}
