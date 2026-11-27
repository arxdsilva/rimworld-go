// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Each caller of the thought book writes through the run's book: with a
// patched book the memory it makes reads the patched value.

func patchValues(t *testing.T, g *Game, vals map[mood.Kind]int) {
	t.Helper()
	edit := map[mood.Kind]func(*mood.Tuning){}
	for k, v := range vals {
		edit[k] = setValue("patched", v)
	}
	g.sk.content.Thoughts = tunedThoughts(t, edit)
}

func wantThought(t *testing.T, g *Game, id uint64, k mood.Kind, want int) {
	t.Helper()
	for _, th := range g.thoughts(entity.EntityID(id)) {
		if th.Kind == k {
			if th.Value != want {
				t.Fatalf("%v = %d, want %d", k, th.Value, want)
			}
			return
		}
	}
	t.Fatalf("no %v thought", k)
}

// Bond state thoughts (master, not master) and a bond memory (died).
func TestBondThoughtsReadTheBook(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 10)
	bo, _ := g.SpawnNamedPawn(8, 6, "Bo")
	patchValues(t, g, map[mood.Kind]int{mood.BondedMaster: 9, mood.BondedNotMaster: -8, mood.BondedDied: -7})
	a, _ := g.animals.Get(dog)
	a.Bond, a.Master = uint64(ada), uint64(ada)
	g.animals.Set(dog, a)
	wantThought(t, g, uint64(ada), mood.BondedMaster, 9)
	a.Master = uint64(bo)
	g.animals.Set(dog, a)
	wantThought(t, g, uint64(ada), mood.BondedNotMaster, -8)
	g.onAnimalDied(dog, 0)
	wantThought(t, g, uint64(ada), mood.BondedDied, -7)
}

// Gathering and rest hosts remember with the book.
func TestHostMemoriesReadTheBook(t *testing.T) {
	t.Parallel()
	g, ada, _ := bondYard(t, 10)
	bo, _ := g.SpawnNamedPawn(8, 6, "Bo")
	patchValues(t, g, map[mood.Kind]int{mood.AttendedParty: 3, mood.Lovin: 4})
	gatherHearts{g}.Remember(ada, mood.AttendedParty, bo, uint64(g.clock.Now()))
	wantThought(t, g, uint64(ada), mood.AttendedParty, 3)
	restEvents{g}.RememberWith(ada, mood.Lovin, bo)
	wantThought(t, g, uint64(ada), mood.Lovin, 4)
}

// A harvested organ is known to the other colonists at the patched value.
func TestOrganHarvestKnowledgeReadsTheBook(t *testing.T) {
	t.Parallel()
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	patchValues(t, g, map[mood.Kind]int{mood.KnowOrganHarvested: -9})
	g.organHarvested(ada, rid, body.KidneyL)
	wantThought(t, g, uint64(bo), mood.KnowOrganHarvested, -9)
}

// An affair and a turned-down proposal leave the patched memories.
func TestRomanceMemoriesReadTheBook(t *testing.T) {
	t.Parallel()
	g, a, b := couple(t)
	patchValues(t, g, map[mood.Kind]int{mood.CheatedOn: -11, mood.ProposalRejected: -13})
	cara, _ := g.SpawnNamedPawn(8, 5, "Cara")
	g.ensurePerson(cara)
	g.cheatedOn(a, b, cara, socNow(g))
	wantThought(t, g, uint64(a), mood.CheatedOn, -11)

	g.setPair(a, b, social.Pair{Kind: social.PairLover})
	for range 4 {
		g.addOpinionMemory(b, a, social.MemInsulted, socNow(g))
	}
	if !g.propose(a, b, socNow(g)) {
		t.Fatal("proposal not resolved")
	}
	wantThought(t, g, uint64(a), mood.ProposalRejected, -13)
}
