// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/vault"
)

func hiddenAnimalView(t *testing.T, g *Game, id entity.EntityID, typ faction.Type, dormant bool) {
	t.Helper()
	d := faction.DefOf(typ)
	var found bool
	for _, v := range g.World().Animals {
		if v.ID != id {
			continue
		}
		found = true
		if v.Faction != d.Label || v.Emblem != d.Emblem() || v.Dormant != dormant {
			t.Fatalf("animal %d view %+v, want faction %s", id, v, d.Label)
		}
	}
	if !found {
		t.Fatalf("animal %d vanished from snapshot", id)
	}
	card, ok := g.Dossier(id)
	if !ok || card.Relation != dossierview.RelationFoe ||
		card.Faction != d.Label || card.FactionEmblem != d.Emblem() {
		t.Fatalf("animal %d dossier %+v (ok=%v)", id, card, ok)
	}
}

func TestHiddenFactionCreatureViewsAndCheckpoint(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 40, Height: 30})
	_, _ = g.SpawnNamedPawn(20, 15, "Ada")
	mech, err := g.SpawnAnimal("quill", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	wild, err := g.SpawnAnimal("deer", 14, 11)
	if err != nil {
		t.Fatal(err)
	}
	dormant := g.spawnDormant(animal.HiveBrute, 15, 12)
	g.HiveSwarm()
	hive := entity.EntityID(0)
	for _, id := range g.animalIDs {
		if id == dormant {
			continue
		}
		if a, ok := g.animals.Get(id); ok && animal.DefOf(a.Species).Hive {
			hive = id
			break
		}
	}
	if hive == 0 {
		t.Fatal("actual infestation did not spawn an insect")
	}
	check := func(g *Game) {
		hiddenAnimalView(t, g, mech, faction.MechHive, false)
		hiddenAnimalView(t, g, hive, faction.Swarm, false)
		hiddenAnimalView(t, g, dormant, faction.Swarm, true)
		for _, v := range g.World().Animals {
			if v.ID == wild && (v.Faction != "" || v.Emblem != "") {
				t.Fatalf("wildlife acquired foreign faction: %+v", v)
			}
		}
		if card, ok := g.Dossier(wild); !ok || card.Relation != dossierview.RelationWild ||
			card.Faction != "" {
			t.Fatalf("ordinary wildlife dossier %+v (ok=%v)", card, ok)
		}
	}
	check(g)
	h := saveLoad(t, g)
	check(h)
	if g.StateHash() != h.StateHash() {
		t.Fatal("hidden creatures' restored state differs")
	}
}

func TestHiddenFactionAncientOriginCaptureAndRecruit(t *testing.T) {
	t.Parallel()
	g, _, raiders, _ := openOne(t, vault.Hostile)
	if len(raiders) == 0 {
		t.Fatal("vault spawned no ancient defender")
	}
	ancient := raiders[0]
	origin := faction.DefOf(faction.AncientHostile)
	check := func(g *Game) {
		p := g.personOf(ancient)
		if p == nil || p.Faction != 0 || p.Origin != faction.AncientHostile {
			t.Fatalf("ancient origin missing: %+v", p)
		}
		card, ok := g.Dossier(ancient)
		if !ok || card.Faction != origin.Label || card.FactionEmblem != origin.Emblem() {
			t.Fatalf("ancient dossier %+v (ok=%v)", card, ok)
		}
	}
	check(g)
	h := saveLoad(t, g)
	check(h)

	captive, ada, bo, rid, _ := prisonGame(t)
	captive.personOf(rid).Origin = faction.AncientHostile
	capture(t, captive, ada, bo, rid)
	if p := captive.personOf(rid); p.Origin != faction.AncientHostile {
		t.Fatalf("captured ancient origin lost: %+v", p)
	}
	captive = saveLoad(t, captive)
	if p := captive.personOf(rid); p.Origin != faction.AncientHostile {
		t.Fatalf("restored captive origin lost: %+v", p)
	}
	captive.prison.Drop(rid) // prison.Chat removes the held record before Recruit.
	captive.enlist(rid)
	if p := captive.personOf(rid); p.Origin != faction.Player || p.Where != whereColony {
		t.Fatalf("recruited captive kept former allegiance: %+v", p)
	}
	if card, ok := captive.Dossier(rid); !ok || card.Relation != dossierview.RelationOwn ||
		card.Faction != "" || card.FactionEmblem == "" {
		t.Fatalf("recruited dossier %+v (ok=%v)", card, ok)
	}
}
