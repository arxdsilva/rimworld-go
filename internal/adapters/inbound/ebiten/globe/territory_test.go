// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package globe

import (
	"image/color"
	"slices"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

var terrTypes = []faction.Type{faction.Settlers, faction.Clans}

// terrFixture picks two land tiles and a water tile and builds an owner slice
// that gives the first land tile faction 1 and an out-of-range owner to the
// second.
func terrFixture(t *testing.T, g *View) (owner []int16, owned, stray, water int) {
	t.Helper()
	owned, stray, water = -1, -1, -1
	for i := 0; i < g.P.TileCount(); i++ {
		switch {
		case isLand(g.P, i) && owned < 0:
			owned = i
		case isLand(g.P, i) && stray < 0:
			stray = i
		case g.P.Water[i] == planet.Ocean && water < 0:
			water = i
		}
	}
	if owned < 0 || stray < 0 || water < 0 {
		t.Fatal("fixture tiles missing")
	}
	owner = make([]int16, g.P.TileCount())
	for i := range owner {
		owner[i] = -1
	}
	owner[owned], owner[stray], owner[water] = 1, 9, 1
	return owner, owned, stray, water
}

func TestGlobeTerritoryTint(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	before := slices.Clone(g.Base)
	owner, owned, stray, water := terrFixture(t, g)
	g.SetTerritory(owner, 1, terrTypes)
	a := Accent(faction.Clans)
	want := MixRGB(before[owned], toRGB(color.RGBA(a)), 0.4)
	if g.Base[owned] != want {
		t.Fatalf("owned tile %v, want %v", g.Base[owned], want)
	}
	if g.Base[stray] != before[stray] || g.Base[water] != before[water] {
		t.Fatal("out-of-range owner or water was tinted")
	}
	for i := range before {
		if i != owned && g.Base[i] != before[i] {
			t.Fatalf("tile %d changed", i)
		}
	}
}

func TestGlobeTerritorySameVersionNoRecolour(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	owner, owned, _, _ := terrFixture(t, g)
	g.SetTerritory(owner, 1, terrTypes)
	tinted := g.Base[owned]
	g.Base[owned] = rgb{9, 9, 9} // a recolour would overwrite this
	g.SetTerritory(owner, 1, terrTypes)
	if g.Base[owned] != (rgb{9, 9, 9}) {
		t.Fatal("same version recoloured")
	}
	g.SetTerritory(owner, 2, terrTypes)
	if g.Base[owned] != tinted {
		t.Fatal("new version did not recolour")
	}
}

func TestGlobeTerritoryOff(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	before := slices.Clone(g.Base)
	owner, owned, _, _ := terrFixture(t, g)
	g.SetTerritory(nil, 0, nil) // off while off: nothing
	g.SetTerritory(owner, 1, terrTypes)
	g.SetMode(ModeTemperature)
	if g.Base[owned] == before[owned] {
		t.Fatal("tint lost on mode switch")
	}
	g.SetMode(ModeBiome)
	if g.Base[owned] == before[owned] {
		t.Fatal("tint lost on return to biome")
	}
	g.SetTerritory(nil, 0, nil)
	if !slices.Equal(g.Base, before) {
		t.Fatal("tint not removed")
	}
}
