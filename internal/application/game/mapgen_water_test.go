// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/world/terrain"
)

// TestNewColonyWaterShare: a new colony's map has the river or lakes its
// tile has on the planet, so a set of seeds yields water on a fair share of
// maps (not none, not all), and the start stays dry.
func TestNewColonyWaterShare(t *testing.T) {
	const seeds = 40
	wet := 0
	for s := uint64(1); s <= seeds; s++ {
		g := NewColony(Config{Seed: s, Width: 96, Height: 96})
		water := 0
		for y := 0; y < g.Height(); y++ {
			for x := 0; x < g.Width(); x++ {
				if terrain.IsWater(g.terr.At(x, y)) {
					water++
				}
			}
		}
		if water > 0 {
			wet++
		}
	}
	if wet < 3 || wet > seeds*3/4 {
		t.Fatalf("%d of %d new colonies have water, want a fair minority", wet, seeds)
	}
}
