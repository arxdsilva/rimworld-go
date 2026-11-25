// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/pkg/randx"
)

func standaloneWorldLinkFixture(t *testing.T, side int) *Game {
	t.Helper()
	g := NewColony(Config{Seed: 42, Width: side, Height: side})
	// Old standalone checkpoints have no NG1 faction save section.
	g, _ = saveLoadInfo(t, g)
	g.DevAddFactions()
	g.ng.factions[0].Goodwill = 58
	g.ng.factions[0].Relation = faction.Friendly
	g.ng.factions[0].History = []int16{-8, 11, 58}
	g.ng.factions[0].Events = []string{"historical goodwill"}
	if _, err := g.SpawnStack(item.Silver, 350, side/2, side/2); err != nil {
		t.Fatal(err)
	}
	return g
}

func standaloneWorldParams() planet.Params {
	return planet.Params{SeedText: "42", Coverage: 30, Rain: 2, Temp: 2,
		Pop: 1, Factions: faction.DefaultSlots(), Version: planet.CurrentVersion}
}

func TestStandaloneColonyRetainsItsStreamSeed(t *testing.T) {
	for _, seed := range []uint64{1, 2, 42, 99} {
		g := New(Config{Seed: seed, Width: 40, Height: 40})
		if g.worldSeed() != seed {
			t.Fatalf("seed %d: runtime uses seed %d", seed, g.worldSeed())
		}
		want := randx.New(seed ^ 0x90E12)
		for i := 0; i < 8; i++ {
			if got, expected := g.stream(streamPower).IntN(10000), want.IntN(10000); got != expected {
				t.Fatalf("seed %d draw %d: got %d, want %d", seed, i, got, expected)
			}
		}
	}
}

func TestStandalonePlanetLinkPreservesHistoricalMap(t *testing.T) {
	for _, side := range []int{64, 250} {
		t.Run(fmt.Sprint(side), func(t *testing.T) {
			g := standaloneWorldLinkFixture(t, side)
			oldTick, oldPawns, oldItems := g.Tick(), slices.Clone(g.pawnIDs), slices.Clone(g.itemIDs)
			oldFactions := cloneFactionList(g.ng.factions)
			if g.colonyTile() >= 0 || g.hasSetup() {
				t.Fatal("fixture already has a world")
			}
			changed, err := AttachPlanet(g, standaloneWorldParams())
			if err != nil || !changed {
				t.Fatalf("attach planet: changed=%t err=%v", changed, err)
			}
			if g.Width() != side || g.Height() != side || g.Tick() != oldTick ||
				g.colonyTile() < 0 || g.hasSetup() || len(g.WorldSites()) == 0 ||
				!slices.Equal(g.pawnIDs, oldPawns) || !slices.Equal(g.itemIDs, oldItems) {
				t.Fatal("world attachment changed map dimensions, residents, assets or clock")
			}
			if got := g.ng.factions[0]; got.Name != oldFactions[0].Name ||
				got.Goodwill != 58 || got.Relation != faction.Friendly ||
				!reflect.DeepEqual(got.History, oldFactions[0].History) ||
				!reflect.DeepEqual(got.Events, oldFactions[0].Events) {
				t.Fatal("historical faction diplomacy changed")
			}
			params, ok := g.WorldParams()
			if !ok || !reflect.DeepEqual(params, standaloneWorldParams()) {
				t.Fatal("world view did not expose the attached planet")
			}
			var checkpoint bytes.Buffer
			if err := g.Save(&checkpoint, "historical"); err != nil {
				t.Fatal(err)
			}
			reloaded, _, err := Load(bytes.NewReader(checkpoint.Bytes()))
			if err != nil || reloaded.StateHash() != g.StateHash() ||
				reloaded.colonyTile() != g.colonyTile() ||
				len(reloaded.WorldSites()) != len(g.WorldSites()) ||
				reloaded.Width() != side || reloaded.Height() != side ||
				!slices.Equal(reloaded.pawnIDs, oldPawns) ||
				!slices.Equal(reloaded.itemIDs, oldItems) {
				t.Fatalf("world-link save roundtrip diverged: %v", err)
			}
			if changed, err := AttachPlanet(reloaded, standaloneWorldParams()); err != nil || changed {
				t.Fatalf("world was relinked on second call: changed=%t err=%v", changed, err)
			}
			if reloaded.StateHash() != g.StateHash() {
				t.Fatal("idempotent attach changed the saved state")
			}
		})
	}
}

func TestStandalonePlanetLinkRejectsIncompatibleFaction(t *testing.T) {
	g := standaloneWorldLinkFixture(t, 64)
	initial := g.StateHash()
	if _, err := AttachPlanet(g, planet.Params{}); err == nil || g.StateHash() != initial {
		t.Fatal("missing world settings changed the original save")
	}
	g.ng.factions[0].Type = faction.Freebooters
	initial = g.StateHash()
	if _, err := AttachPlanet(g, standaloneWorldParams()); err == nil || g.StateHash() != initial {
		t.Fatal("incompatible faction type was silently rebound")
	}
}

func TestStandalonePlanetLinkFormsCaravan(t *testing.T) {
	g := standaloneWorldLinkFixture(t, 64)
	if _, err := AttachPlanet(g, standaloneWorldParams()); err != nil {
		t.Fatal(err)
	}
	p := g.worldPlanet()
	if p == nil {
		t.Fatal("attached globe cannot be reconstructed")
	}
	dest := -1
	for _, adjacent := range p.Neighbours(g.colonyTile()) {
		if len(p.Route(g.colonyTile(), int(adjacent))) >= 2 {
			dest = int(adjacent)
			break
		}
	}
	if dest < 0 {
		t.Fatal("linked world has no reachable adjacent tile")
	}
	g.Enqueue(command.Command{Kind: command.FormCaravan,
		Name: fmt.Sprint(g.pawnIDs[0]), Count: dest})
	g.ApplyPending()
	if g.worldSt == nil || len(g.worldSt.caravans) == 0 {
		t.Fatal("attached world did not accept native caravan formation")
	}
}

// Later goodwill changes must survive save/load after the planet is bound.
func TestStandaloneLinkSavesLiveDiplomacy(t *testing.T) {
	t.Parallel()
	g := standaloneWorldLinkFixture(t, 64)
	if changed, err := AttachPlanet(g, standaloneWorldParams()); err != nil || !changed {
		t.Fatalf("attach: changed=%v err=%v", changed, err)
	}
	params, ok := g.WorldParams()
	if !ok {
		t.Fatal("linked world lacks settings")
	}
	params.Factions[0].Count++
	actual, _ := g.WorldParams()
	if !reflect.DeepEqual(actual, standaloneWorldParams()) {
		t.Fatal("WorldParams exposed mutable world settings")
	}
	g.ng.factions[0].Goodwill = 79
	g.ng.factions[0].History = append(g.ng.factions[0].History, 79)
	var buf bytes.Buffer
	if err := g.Save(&buf, "changed-diplomacy"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StateHash() != g.StateHash() || loaded.ng.factions[0].Goodwill != 79 ||
		!reflect.DeepEqual(loaded.ng.factions[0].History, g.ng.factions[0].History) {
		t.Fatal("updated diplomacy did not survive world-link round trip")
	}
}

func TestStandaloneLinkRefusesUnplacedWorldMap(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 42, Width: 64, Height: 64})
	g.worldS().atlas.Add(maps.Site, 5, 0)
	before := g.StateHash()
	changed, err := AttachPlanet(g, standaloneWorldParams())
	if err == nil || changed || g.StateHash() != before {
		t.Fatalf("active world accepted link: changed=%v err=%v", changed, err)
	}
}
