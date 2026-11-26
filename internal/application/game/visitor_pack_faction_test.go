// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

func packSpawn(t *testing.T, g *Game, s animal.Species, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnAnimal(s.String(), x, y)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func caravanPackFaction(t *testing.T, g *Game, pack entity.EntityID) (*faction.Faction, int) {
	t.Helper()
	gr, ok := visitsOf(g).GroupOf(pack)
	if !ok || gr.Faction <= 0 {
		t.Fatalf("no foreign visit group for pack %d", pack)
	}
	f := g.factionOf(gr.Faction)
	if f == nil {
		t.Fatalf("unknown faction reference %d", gr.Faction)
	}
	return f, gr.Faction
}

func TestCaravanPackFactionReadModelAndReplay(t *testing.T) {
	g, c := caravanGame(t)
	pack := caravanPack(g, c)
	if pack == 0 {
		t.Fatal("caravan has no animal")
	}
	f, _ := caravanPackFaction(t, g, pack)
	if a, _ := g.animals.Get(pack); a.Tame {
		t.Fatal("visiting pack animal must not become colony tame livestock")
	}
	card, ok := g.Dossier(pack)
	if !ok || card.Faction != f.Name || card.FactionEmblem != factionView(*f).Emblem || card.Relation != dossierview.RelationNone {
		t.Fatalf("foreign pack dossier %+v, want neutral group %s", card, f.Name)
	}
	viewed := false
	for _, av := range g.animalViews() {
		if av.ID == pack {
			viewed = av.Faction == f.Name && av.Emblem == factionView(*f).Emblem && !av.Tame
		}
	}
	if !viewed {
		t.Fatal("animal world view lost foreign pack faction")
	}
	h := saveLoad(t, g)
	card2, ok := h.Dossier(pack)
	if !ok || card2.Faction != card.Faction || card2.Relation != card.Relation {
		t.Fatalf("foreign faction identity lost after save: %+v", card2)
	}
	g.Step(30)
	h.Step(30)
	if g.StateHash() != h.StateHash() {
		t.Fatal("foreign pack faction diverged after save/replay")
	}
}

func packFactionScene(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g, _ := def4Game(t, 80, 45, 72, 40)
	g.ng.factions = []faction.Faction{
		{ID: 0, Name: "The Bay", Type: faction.Settlers, Relation: faction.Neutral},
		{ID: 1, Name: "The Ridge", Type: faction.Clans, Relation: faction.Hostile},
	}
	pack := packSpawn(t, g, animal.Donkey, 11, 10)
	visitsOf(g).NewGroup(visit.KindCaravan, 1, []visit.Member{{ID: pack, Role: visit.RolePackAnimal}},
		uint64(g.clock.Now()), uint64(g.clock.Now())+1000)
	return g, pack
}

func TestCaravanPackMachineTargetsForeignAnimal(t *testing.T) {
	t.Parallel()
	g, pack := packFactionScene(t)
	mech := packSpawn(t, g, animal.Harrow, 12, 10)
	now := uint64(g.clock.Now())
	candidates := g.combat.Tactics.RaidFoes(mech, 12, 10, 5)
	found := false
	for _, candidate := range candidates {
		found = found || candidate.ID == pack
	}
	if !found {
		t.Fatalf("machine ignored foreign pack among foes: %+v", candidates)
	}
	a, _ := g.animals.Get(mech)
	for i := uint64(0); i < animal.DecideEvery && a.Target == 0; i++ {
		g.raid.Mechs.Think(mech, &a, now+i)
	}
	if a.Target != pack {
		t.Fatalf("machine target %d, want foreign pack %d", a.Target, pack)
	}
}

func TestCaravanPackHumanWarsAndPeace(t *testing.T) {
	t.Parallel()
	g, pack := packFactionScene(t)
	ga := visitArrivals{g}
	makeRaider := func(ref int, x int) entity.EntityID {
		id := ga.NewPerson(ref, &g.ng.factions[ref-1])
		ga.Place(id, ref, x, 10, uint64(g.clock.Now()))
		r := g.raiders[id]
		r.Leaving = false
		g.raiders[id] = r
		g.setWhere(id, whereRaider)
		return id
	}
	allied := makeRaider(1, 12)
	enemy := makeRaider(2, 13)
	nearPack := func(id entity.EntityID) bool {
		for _, s := range g.combat.Tactics.RaidFoes(id, 12, 10, 5) {
			if s.ID == pack {
				return true
			}
		}
		return false
	}
	if nearPack(allied) || nearPack(enemy) {
		t.Fatal("peaceful human factions targeted a foreign caravan pack")
	}
	g.ng.factions[0].Enemies = []int{1}
	g.ng.factions[1].Enemies = []int{0}
	if nearPack(allied) || !nearPack(enemy) {
		t.Fatal("same-faction raider friendly fire or warring raider failed to target caravan pack")
	}
}

func TestCaravanPackManhunterTargetsAnimal(t *testing.T) {
	t.Parallel()
	g, pack := packFactionScene(t)
	wolf := packSpawn(t, g, animal.TimberWolf, 12, 10)
	a, _ := g.animals.Get(wolf)
	a.Manhunter, a.Mode = 1<<60, animal.Attack
	g.thinkAnimal(wolf, &a, uint64(g.clock.Now()))
	if a.Target != pack {
		t.Fatalf("manhunter chose %d, want pack %d", a.Target, pack)
	}
}

func TestCaravanPackHostilityAndDeparture(t *testing.T) {
	g, c := caravanGame(t)
	pack := caravanPack(g, c)
	f, _ := caravanPackFaction(t, g, pack)
	now := uint64(g.clock.Now())
	if g.combat.Turrets.Targetable(pack, now) {
		t.Fatal("neutral pack targeted by colony turret")
	}
	f.Relation = faction.Hostile
	if !g.combat.Turrets.Targetable(pack, now) {
		t.Fatal("hostile foreign pack ignored by colony turret")
	}
	if card, ok := g.Dossier(pack); !ok || card.Relation != dossierview.RelationFoe {
		t.Fatalf("hostile pack dossier: %+v", card)
	}
	for i := range g.worldSt.visitGroups {
		if g.worldSt.visitGroups[i].ID != c.GroupID {
			continue
		}
		for j := range g.worldSt.visitGroups[i].Members {
			m := &g.worldSt.visitGroups[i].Members[j]
			if m.ID == pack {
				m.Departed = true
			}
		}
	}
	if g.combat.Turrets.Targetable(pack, now) {
		t.Fatal("departed pack still hostile to colony")
	}
	if card, ok := g.Dossier(pack); !ok || card.Relation != dossierview.RelationWild {
		t.Fatalf("departed pack retained foreign faction: %+v", card)
	}
}

// Actual combat uses the existing machine attack and damage path.
// The pack's faction metadata only supplies the target eligibility.
func TestCaravanPackCrossFactionDamage(t *testing.T) {
	t.Parallel()
	g, pack := packFactionScene(t)
	mech := packSpawn(t, g, animal.Harrow, 12, 10)
	initial, _ := g.animals.Get(pack)
	hit := false
	for i := 0; i < 900 && g.animals.Has(pack); i++ {
		g.Step(1)
		for _, shot := range g.traces {
			if shot.Attacker == mech && shot.Target == pack && shot.Damage > 0 {
				hit = true
				break
			}
		}
		if hit {
			break
		}
	}
	if !hit {
		t.Fatalf("active mech %d never dealt damage to foreign caravan animal %d (start HP %d)", mech, pack, initial.HP())
	}
}

// An incapacitated carrier triggers the group's cumulative loss rule.
// Its trade stock remains owned by the caravan until actual removal.
func TestCaravanPackDownedLossRetreatKeepsCargo(t *testing.T) {
	g, c := caravanGame(t)
	pack := caravanPack(g, c)
	if pack == 0 {
		t.Fatal("no physical carrier")
	}
	a, _ := g.animals.Get(pack)
	a.Downed = true
	g.animals.Set(pack, a)
	silver, cargoN, itemsN := c.Stock.Silver, len(c.Cargo), len(g.itemIDs)
	visitsOf(g).TickGroups(uint64(g.clock.Now()))
	gr, ok := visitsOf(g).GroupOf(pack)
	if !ok || gr.Phase != visit.PhaseLeaving || gr.Reason != visit.LeaveLoss {
		t.Fatalf("downed carrier did not prompt caravan retreat: %+v", gr)
	}
	if !g.animals.Has(pack) || c.Stock.Silver != silver || len(c.Cargo) != cargoN || len(g.itemIDs) != itemsN {
		t.Fatal("incapacitation prematurely removed the carrier or forfeited its cargo")
	}
}
