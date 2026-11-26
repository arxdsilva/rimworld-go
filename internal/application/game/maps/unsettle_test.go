// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// unsettleParams is a small real world (the game tests' caravan world).
var unsettleParams = planet.Params{SeedText: "caravan", Coverage: 30, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()}

// lossHost is a Host for destroy: the factions, its own planet, the day
// and the SettlementLost calls.
type lossHost struct {
	Host
	p    *planet.Planet
	facs []faction.Faction
	lost [][3]int
}

func (h *lossHost) Factions() []faction.Faction { return h.facs }
func (h *lossHost) Planet() *planet.Planet      { return h.p }
func (h *lossHost) Day() int                    { return 1 }
func (h *lossHost) SettlementLost(fi, t int, defeated bool) {
	d := 0
	if defeated {
		d = 1
	}
	h.lost = append(h.lost, [3]int{fi, t, d})
}

// cloneFacs is a deep enough copy of fs for destroy and Prune.
func cloneFacs(fs []faction.Faction) []faction.Faction {
	out := slices.Clone(fs)
	for i := range out {
		out[i].Settlements = slices.Clone(out[i].Settlements)
		out[i].SettlementNames = slices.Clone(out[i].SettlementNames)
	}
	return out
}

// sameAsFresh fails unless p's own territory equals one built afresh
// from p's settlements now.
func sameAsFresh(t *testing.T, p *planet.Planet) {
	t.Helper()
	var x faction.Territory
	planet.BuildTerritory(p, &x, nil)
	for i := range p.TileCount() {
		if planet.TerritoryOwner(p, i) != x.Owner(i) {
			t.Fatalf("tile %d: owner %d, a fresh build says %d", i, planet.TerritoryOwner(p, i), x.Owner(i))
		}
	}
}

// TestUnsettleOnDestroyAndPrune: destroy and Prune take settlements off
// the planet through planet.Unsettle, so its territory is rebuilt (once
// for Prune), matches a fresh build and a fork taken before agrees; a
// Prune with nothing to drop leaves the version alone.
func TestUnsettleOnDestroyAndPrune(t *testing.T) {
	t.Parallel()
	t.Run("destroy", func(t *testing.T) {
		t.Parallel()
		p := planet.Build(unsettleParams)
		h := &lossHost{p: p, facs: cloneFacs(p.Factions)}
		fi, tile := -1, -1
		for i, f := range h.facs {
			if n := len(f.Settlements); f.Type.Human() && n > 0 && p.Settlement[f.Settlements[n-1]] >= 0 {
				fi, tile = i, f.Settlements[n-1]
				break
			}
		}
		if fi < 0 {
			t.Fatal("no human faction with a settlement")
		}
		last := len(h.facs[fi].Settlements) == 1
		fork, v0 := p.Fork(), planet.TerritoryVersion(p)
		if v0 == 0 || planet.TerritoryOwner(p, tile) != fi {
			t.Fatalf("version %d, owner %d", v0, planet.TerritoryOwner(p, tile))
		}
		if name := (Lifecycle{Of(&[]Caravan{}, &Atlas{}, h)}).destroy(fi, tile); (name != "") != last {
			t.Fatalf("defeated %q, last settlement %v", name, last)
		}
		if p.Settlement[tile] >= 0 || planet.TerritoryVersion(p) != v0+1 {
			t.Fatalf("settlement %d, version %d (was %d)", p.Settlement[tile], planet.TerritoryVersion(p), v0)
		}
		sameAsFresh(t, p)
		for i := range p.TileCount() {
			if planet.TerritoryOwner(fork, i) != planet.TerritoryOwner(p, i) || fork.Settlement[i] != p.Settlement[i] {
				t.Fatalf("the fork disagrees at tile %d", i)
			}
		}
		d := 0
		if last {
			d = 1
		}
		if !slices.Equal(h.lost, [][3]int{{fi, tile, d}}) {
			t.Fatalf("SettlementLost calls %v", h.lost)
		}
	})
	t.Run("prune", func(t *testing.T) {
		t.Parallel()
		p := planet.Build(unsettleParams)
		fs := cloneFacs(p.Factions)
		v0 := planet.TerritoryVersion(p)
		Prune(p, fs)
		if planet.TerritoryVersion(p) != v0 {
			t.Fatal("a Prune with nothing to drop rebuilt the territory")
		}
		var gone []int
		for i := range fs {
			if fs[i].Type.Human() && len(fs[i].Settlements) > 0 && len(gone) < 2 {
				gone = append(gone, fs[i].Settlements[0])
				fs[i].Settlements = fs[i].Settlements[1:]
			}
		}
		if len(gone) < 2 {
			t.Fatal("too few factions to prune")
		}
		Prune(p, fs)
		for _, tile := range gone {
			if p.Settlement[tile] >= 0 {
				t.Fatalf("tile %d still settled", tile)
			}
		}
		if planet.TerritoryVersion(p) != v0+1 {
			t.Fatalf("version %d after Prune, want %d (one rebuild)", planet.TerritoryVersion(p), v0+1)
		}
		sameAsFresh(t, p)
	})
}
