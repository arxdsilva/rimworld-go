// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package planet

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/faction"
)

func settledWorld(t testing.TB) *Planet {
	t.Helper()
	p := Build(Params{SeedText: "amber-hollow", Coverage: 30, Pop: 1, Factions: faction.DefaultSlots()})
	if len(p.SettlementTiles()) < 3 {
		t.Skip("faction placement produced too few settlements")
	}
	return p
}

func TestTerritoryBuiltWithPlanet(t *testing.T) {
	p, q := settledWorld(t), settledWorld(t)
	owners := p.ext.territory.Owners()
	claimed := 0
	standing := map[int]bool{}
	for f := range p.Factions {
		for _, tile := range p.Factions[f].Settlements {
			standing[f] = standing[f] || p.Settlement[tile] >= 0
		}
	}
	for i, o := range owners {
		if o < 0 {
			continue
		}
		claimed++
		if !p.Land(i) {
			t.Fatalf("water tile %d owned", i)
		}
		if !standing[int(o)] {
			t.Fatalf("tile %d owned by faction %d with no standing settlement", i, o)
		}
		if TerritoryOwner(p, i) != int(o) {
			t.Fatalf("TerritoryOwner(%d) disagrees with Owners", i)
		}
	}
	if claimed == 0 || TerritoryVersion(p) != 1 {
		t.Fatalf("claimed %d tiles, version %d", claimed, TerritoryVersion(p))
	}
	if !slices.Equal(owners, q.ext.territory.Owners()) {
		t.Fatal("two builds of one planet differ")
	}
	total := 0
	for f := range p.Factions {
		tiles := TerritoryTiles(p, f)
		if !slices.IsSorted(tiles) {
			t.Fatalf("faction %d tiles not ascending", f)
		}
		total += len(tiles)
	}
	if total != claimed {
		t.Fatalf("TerritoryTiles total %d, claimed %d", total, claimed)
	}
	// Every settlement tile belongs to its own faction.
	for _, tile := range p.SettlementTiles() {
		if f, _ := p.SettlementFaction(int(tile)); TerritoryOwner(p, int(tile)) != f {
			t.Fatalf("settlement tile %d owned by %d, want %d", tile, TerritoryOwner(p, int(tile)), f)
		}
	}
}

func TestTerritoryUnsettle(t *testing.T) {
	p := settledWorld(t)
	before := slices.Clone(p.ext.territory.Owners())
	v := TerritoryVersion(p)
	tile := int(p.SettlementTiles()[1])
	f, _ := p.SettlementFaction(tile)

	// Unsettled, out-of-range and negative tiles change nothing.
	free := -1
	for i := range p.TileCount() {
		if p.Settlement[i] < 0 && p.Land(i) {
			free = i
			break
		}
	}
	Unsettle(p, free, -1, p.TileCount(), p.TileCount()+5)
	Unsettle(p)
	if TerritoryVersion(p) != v || !slices.Equal(before, p.ext.territory.Owners()) {
		t.Fatal("no-op Unsettle changed the territory")
	}

	Unsettle(p, tile, tile) // the repeat is already unsettled: one rebuild
	if p.Settlement[tile] != -1 {
		t.Fatal("tile still settled")
	}
	if TerritoryVersion(p) != v+1 {
		t.Fatalf("version %d, want %d", TerritoryVersion(p), v+1)
	}
	for i, o := range before {
		now := TerritoryOwner(p, i)
		if int(o) != f && now != int(o) {
			t.Fatalf("tile %d of faction %d moved to %d", i, o, now)
		}
	}
	// A rebuild from scratch agrees, and the seat's own tile left it.
	var x faction.Territory
	BuildTerritory(p, &x, nil)
	if !slices.Equal(x.Owners(), p.ext.territory.Owners()) {
		t.Fatal("Unsettle index differs from a fresh build")
	}
}

func TestTerritoryUnsettleWithoutIndex(t *testing.T) {
	p := settledWorld(t)
	p.ext.territory = nil
	if TerritoryOwner(p, 0) != -1 || TerritoryVersion(p) != 0 || TerritoryTiles(p, 0) != nil {
		t.Fatal("unbuilt planet must read as empty")
	}
	Unsettle(p, int(p.SettlementTiles()[0]))
	if TerritoryVersion(p) != 1 {
		t.Fatal("Unsettle must create the index")
	}
}

func TestTerritoryStandingMatchesUnsettle(t *testing.T) {
	a, b := settledWorld(t), settledWorld(t)
	bOwners := slices.Clone(b.ext.territory.Owners())
	bVersion := TerritoryVersion(b)
	hidden := map[int]bool{}
	var x faction.Territory
	for _, k := range []int{0, 2} {
		tile := int(a.SettlementTiles()[k])
		Unsettle(a, tile)
		hidden[tile] = true
		BuildTerritory(b, &x, func(i int) bool { return !hidden[i] })
		if !slices.Equal(a.ext.territory.Owners(), x.Owners()) {
			t.Fatalf("after hiding %d settlements the standing mirror differs from Unsettle", len(hidden))
		}
		if b.Settlement[tile] < 0 {
			t.Fatal("BuildTerritory cleared a settlement on its planet")
		}
	}
	if !slices.Equal(bOwners, b.ext.territory.Owners()) || TerritoryVersion(b) != bVersion {
		t.Fatal("BuildTerritory into another index changed the planet's own")
	}
}

func BenchmarkPlanetTerritory(b *testing.B) {
	p := settledWorld(b)
	var x faction.Territory
	BuildTerritory(p, &x, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		BuildTerritory(p, &x, nil)
	}
}
