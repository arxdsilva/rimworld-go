// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// The optimization retains the actual native candidate order, eligibility
// and result buffer, including same-tick roster changes.
func TestNativePresenceMatchesOriginalSearch(t *testing.T) {
	g := New(Config{Seed: 3, Width: 64, Height: 64})
	for i := range 4 {
		if _, err := g.SpawnPawn(5+i, 5); err != nil {
			t.Fatal(err)
		}
	}
	wolf, err := g.SpawnAnimal("timber_wolf", 7, 7)
	if err != nil {
		t.Fatal(err)
	}
	cow, err := g.SpawnAnimal("cow", 9, 8)
	if err != nil {
		t.Fatal(err)
	}
	now := uint64(g.clock.Now())
	predicates := []func(entity.EntityID, *animal.State) bool{
		func(_ entity.EntityID, a *animal.State) bool { return animal.Penned(a) },
		func(_ entity.EntityID, a *animal.State) bool { return herd.Due(a) },
		func(_ entity.EntityID, a *animal.State) bool { return herd.Marked(a) },
		func(id entity.EntityID, a *animal.State) bool { return herd.Ready(herdHost{g}, id, a, now) },
		func(id entity.EntityID, a *animal.State) bool { return g.needsVet(id, a, now) },
		func(id entity.EntityID, a *animal.State) bool { return g.trainable(id, a, now) },
	}
	oldAny := func(p func(entity.EntityID, *animal.State) bool) bool {
		found := false
		g.animals.All(func(id entity.EntityID, a animal.State) {
			if !found {
				found = p(id, &a)
			}
		})
		return found
	}
	oldNear := func(x, y, r int, who combat.Who, buf []combat.Seen) []combat.Seen {
		g.creatures().grid.Each(x, y, r+1, func(e idxEntry) bool {
			if p, ok := g.positions.Get(e.id); ok && e.kind&uint8(who) != 0 && weapon.Distance(x, y, p.X, p.Y) <= r {
				buf = append(buf, combat.Seen{ID: e.id, X: p.X, Y: p.Y})
			}
			return true
		})
		return buf
	}
	compare := func() {
		t.Helper()
		for _, p := range predicates {
			if oldAny(p) != g.animals.AnyTame(p) {
				t.Fatal("tame candidate changed")
			}
		}
		if oldAny(func(_ entity.EntityID, a *animal.State) bool { return a.TameMark && !a.Tame }) != (herdHost{g}).AnyTameMarked() {
			t.Fatal("tame mark changed")
		}
		if oldAny(func(_ entity.EntityID, a *animal.State) bool { return a.Designated && !a.Dead() }) != (combatHunts{g}).AnyMarked() {
			t.Fatal("hunt mark changed")
		}
		for _, who := range []combat.Who{combat.Pawns, combat.Raiders, combat.Animals, combat.Everyone, 0} {
			prefix := []combat.Seen{{ID: 99999, X: 63, Y: 63}}
			want := oldNear(5, 5, 14, who, slices.Clone(prefix))
			got := (combatRosters{g}).Near(5, 5, 14, who, slices.Clone(prefix))
			if !slices.Equal(got, want) {
				t.Fatalf("near(%v)=%v want %v", who, got, want)
			}
		}
	}
	compare()
	a := g.animals.Ref(wolf)
	a.TameMark, a.Designated = true, true
	compare()
	g.TameAnimal(cow, "Bess")
	b := g.animals.Ref(cow)
	b.Downed, b.Designated, b.Food = true, true, 1
	compare()
	spawnRaiderAt(t, g, 6, 7, 0)
	compare()
}
