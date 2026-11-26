// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/roof"
	"testing"
)

// TestInfestationDarkRoofedWarm (EAI-035): an infestation looks over the
// whole map, not only by the rock face: a roofed, dark cave far from home
// gets the hive, the nearest of the spots to home.
func TestInfestationDarkRoofedWarm(t *testing.T) {
	t.Parallel()
	g, _ := def4Game(t, 80, 40, 60, 20)
	roofOver(g, 0, 0, 9, 9) // thin roof cannot seed
	thickCave(g, 20, 5, 25, 9)
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 27, 8)
	g.Step(1)
	if !g.raid.Hives.Infest() {
		t.Fatal("a roofed, dark, warm spot exists: the infestation takes it")
	}
	hives := g.raid.Hives.List()
	if len(hives) != 1 {
		t.Fatalf("one hive: %+v", hives)
	}
	if h := hives[0]; h.X < 20 || h.X > 25 || h.Y < 5 || h.Y > 9 {
		t.Fatalf("the hive rose at (%d,%d), not in the cave nearest home", h.X, h.Y)
	}
	if len(hives[0].Brood) == 0 {
		t.Fatal("the swarm is the hive's brood")
	}
}

// TestInfestationSkipsFreezing: below -17 C no spot is fit, so the
// infestation does not happen and the caller sends a raid instead.
func TestInfestationSkipsFreezing(t *testing.T) {
	t.Parallel()
	g, _ := def4Game(t, 80, 40, 60, 20)
	thickCave(g, 20, 5, 25, 9)
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 27, 8)
	g.Step(1)
	g.climate.YearMean = -600
	if g.raid.Hives.Infest() {
		t.Fatal("a freezing cave is no hive spot")
	}
	if len(g.raid.Hives.List()) != 0 || len(g.animalIDs) != 0 {
		t.Fatal("nothing changed")
	}
	g2, _ := def4Game(t, 40, 30, 20, 15) // no roof at all
	if g2.raid.Hives.Infest() {
		t.Fatal("no roof, no spot")
	}
}

func thickCave(g *Game, x0, y0, x1, y1 int) {
	g.ensureRoofGrid()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			g.setRoof(y*g.grid.Width()+x, roof.Natural|roof.NaturalThick)
		}
	}
}

func TestInfestationColonyDistanceAndSavedCooldown(t *testing.T) {
	g, _ := def4Game(t, 80, 40, 60, 20)
	thickCave(g, 2, 2, 5, 5)
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 60, 20)
	if g.raid.Hives.Infest() {
		t.Fatal("distant cave admitted")
	}
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 8, 8)
	g.Step(1)
	if !g.raid.Hives.Infest() {
		t.Fatal("eligible cave refused")
	}
	h, _ := saveLoadInfo(t, g)
	if h.raid.Hives.Infest() || g.raid.Hives.Infest() {
		t.Fatal("cooldown lost")
	}
	if h.raid.Mechs.Export().InfestAfter != g.raid.Mechs.Export().InfestAfter || g.raid.Mechs.Export().InfestAfter < 20*clock.TicksPerDay {
		t.Fatal("saved deadline mismatch")
	}
}
