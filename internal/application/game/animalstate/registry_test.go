// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalstate

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
)

func TestRegistryPreservesIndependentLiveGroups(t *testing.T) {
	r := New()
	original := animal.State{Species: animal.Husky, Tame: true, Name: "Courier",
		Food: 3000, Rest: 1200, NextThink: 42, GoalX: 8,
		Path:    []pathing.Point{{X: 4, Y: 5}},
		Trained: [3]uint8{3, 2, 1}, Pregnant: 1234, Mated: 2}
	r.Set(41, original)
	view := r.Ref(41)
	if view == nil || r.Len() != 1 {
		t.Fatal("animal absent from live registry")
	}
	identity, vitals, intent, motion := view.Identity, view.Vitals, view.Intent, view.Motion
	threat, feeding, handling := view.Threat, view.Feeding, view.Handling
	reproduction, medical := view.Reproduction, view.Medical
	for _, id := range []entity.EntityID{17, 52, 6, 91} {
		r.Set(id, animal.State{Species: animal.Deer, Food: 1000})
	}
	for _, id := range []entity.EntityID{17, 52, 6, 91} {
		r.Delete(id)
	}
	if got := r.Ref(41); got != view ||
		got.Identity != identity || got.Vitals != vitals ||
		got.Intent != intent || got.Motion != motion ||
		got.Threat != threat || got.Feeding != feeding ||
		got.Handling != handling || got.Reproduction != reproduction ||
		got.Medical != medical {
		t.Fatal("independent stores invalidated a live component reference")
	}
	got, ok := r.Get(41)
	if !ok || !reflect.DeepEqual(got, original) {
		t.Fatalf("initial component assembly failed: %+v", got)
	}
	view.Name = "Mended"
	view.Food = 2030
	view.NextThink = 96
	view.GoalX = 11
	view.Mated = 1
	view.Trained[1] = 3
	live, _ := r.Get(41)
	if live.Name != "Mended" || live.Food != 2030 || live.NextThink != 96 ||
		live.GoalX != 11 || live.Mated != 1 || live.Trained[1] != 3 {
		t.Fatalf("live component mutation was not authoritative: %+v", live)
	}
	live.Name = "Detached"
	live.Food = 0
	if view.Name != "Mended" || view.Food != 2030 {
		t.Fatal("a detached State write changed live component values")
	}
	r.Set(41, live)
	if r.Ref(41) != view || view.Name != "Detached" || view.Food != 0 {
		t.Fatal("Set failed to update independently owned groups in place")
	}
	r.Delete(41)
	if r.Has(41) || r.Ref(41) != nil || r.Len() != 0 {
		t.Fatal("component group survived removal")
	}
	for _, count := range []int{r.identities.Len(), r.vitals.Len(), r.intents.Len(),
		r.motions.Len(), r.threats.Len(), r.feeding.Len(), r.handling.Len(),
		r.reproduction.Len(), r.medical.Len()} {
		if count != 0 {
			t.Fatal("deleted animal retained a group store entry")
		}
	}
}

func TestRegistryGetSetMatchesLegacyStore(t *testing.T) {
	legacy := component.NewStore[animal.State]()
	live := New()
	for i := 0; i < 240; i++ {
		id := entity.EntityID(i*7 + 2)
		state := animal.State{Species: animal.Husky, Born: uint64(i + 100),
			Mode: animal.Wander, Hunger: uint64(i), Food: i + 1, Rest: 1000 + i,
			Mated: i % 3, Master: uint64(i % 5), NextThink: uint64(i * 13)}
		legacy.Set(id, state)
		live.Set(id, state)
	}
	for tick := uint64(0); tick < 120; tick++ {
		legacy.All(func(id entity.EntityID, state animal.State) {
			state.Rest++
			state.NextThink += 2
			legacy.Set(id, state)
		})
		live.All(func(id entity.EntityID, state animal.State) {
			state.Rest++
			state.NextThink += 2
			live.Set(id, state)
		})
	}
	legacy.All(func(id entity.EntityID, state animal.State) {
		got, ok := live.Get(id)
		if !ok || !reflect.DeepEqual(got, state) {
			t.Fatalf("animal %d diverged: new %+v old %+v", id, got, state)
		}
	})
	if got := Export(live); len(got) != legacy.Len() {
		t.Fatalf("wrong live export count: %d", len(got))
	}
}

func BenchmarkAnimalRegistryAccess(b *testing.B) {
	legacy := component.NewStore[animal.State]()
	grouped := New()
	for i := range 1000 {
		id := entity.EntityID(i + 1)
		state := animal.State{Species: animal.Husky, Food: 5000, Rest: 5000, NextThink: uint64(i)}
		legacy.Set(id, state)
		grouped.Set(id, state)
	}
	b.Run("legacy-get-set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for id := entity.EntityID(1); id <= 1000; id++ {
				state, _ := legacy.Get(id)
				state.Food++
				legacy.Set(id, state)
			}
		}
	})
	b.Run("grouped-get-set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for id := entity.EntityID(1); id <= 1000; id++ {
				state, _ := grouped.Get(id)
				state.Food++
				grouped.Set(id, state)
			}
		}
	})
	b.Run("grouped-ref", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for id := entity.EntityID(1); id <= 1000; id++ {
				grouped.Ref(id).Food++
			}
		}
	})
}

func TestAnyTameWithoutTameAnimalsDoesNotAllocate(t *testing.T) {
	r := New()
	for id := entity.EntityID(1); id <= 10; id++ {
		r.Set(id, animal.State{Species: animal.Deer})
	}
	keep := func(_ entity.EntityID, _ *animal.State) bool { return true }
	if r.AnyTame(keep) {
		t.Fatal("wild animal admitted")
	}
	if n := testing.AllocsPerRun(100, func() { r.AnyTame(keep) }); n != 0 {
		t.Fatalf("wild-only registry allocated %g scratch records per call", n)
	}
	r.Ref(4).Tame = true
	var got entity.EntityID
	if !r.AnyTame(func(id entity.EntityID, a *animal.State) bool { got = id; return a.Tame }) || got != 4 {
		t.Fatalf("tame predicate did not receive the real tame animal: %d", got)
	}
}
