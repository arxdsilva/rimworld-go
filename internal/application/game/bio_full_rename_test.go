// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"testing"
)

func TestRenamePawnChangesIdentityAndSurvivesSave(t *testing.T) {
	g := New(Config{Seed: 18, Width: 24, Height: 18})
	id := spawnGen(t, g, 4, 4, genPawn("Mira", nil))
	original, _ := g.personBio(id)
	rename := func(g *Game, id entity.EntityID, raw string) {
		g.Enqueue(command.Command{Kind: command.RenamePawn, Pawn: uint64(id), Name: raw})
		g.ApplyPending()
	}
	rename(g, id, command.RenameName("Ada", "Ash", "O'Neil-Smith"))
	got, ok := g.personBio(id)
	if !ok || got.First != "Ada" || got.Nick != "Ash" || got.Last != "O'Neil-Smith" || g.name(id) != "Ash" {
		t.Fatalf("renamed identity: %+v; display=%q", got, g.name(id))
	}
	h := saveLoad(t, g)
	saved, ok := h.personBio(id)
	if !ok || saved.First != got.First || saved.Nick != got.Nick || saved.Last != got.Last || h.name(id) != "Ash" {
		t.Fatalf("save/load lost renamed identity: %+v display=%q", saved, h.name(id))
	}
	rename(h, id, command.RenameName("", "Other", "No"))
	rename(h, id, "unpacked")
	rename(h, id, command.RenameName("Ada1", "Other", "No"))
	denied, _ := h.personBio(id)
	if !reflect.DeepEqual(denied, saved) {
		t.Fatalf("invalid name changed identity: %+v", denied)
	}
	if original.First == saved.First && original.Nick == saved.Nick && original.Last == saved.Last {
		t.Fatal("name did not change")
	}
}

func TestRenamePawnRejectsForeignAndMissingIdentities(t *testing.T) {
	g, id := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	before := g.name(rid)
	for _, target := range []entity.EntityID{rid, entity.EntityID(999999)} {
		g.Enqueue(command.Command{Kind: command.RenamePawn, Pawn: uint64(target), Name: command.RenameName("Enemy", "M", "No")})
	}
	g.ApplyPending()
	if g.name(rid) != before {
		t.Fatal("raider accepted rename")
	}
	if _, ok := g.personBio(entity.EntityID(999999)); ok {
		t.Fatal("fabricated missing pawn")
	}
	if _, ok := g.personBio(entity.EntityID(id)); !ok {
		t.Fatal("colonist missing bio")
	}
}
