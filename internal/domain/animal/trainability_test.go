// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import "testing"

// Every species has a deliberate training classification. Leaving the zero
// value would silently grant a new animal a guessed class.
func TestTrainabilityTableCoversEverySpecies(t *testing.T) {
	for _, species := range All() {
		class := TrainabilityOf(species)
		if class < TrainNone || class > TrainAdvanced {
			t.Fatalf("%s has no explicit trainability: %d", species, class)
		}
		r := RosterOf(species)
		if r.Penned && class != TrainNone {
			t.Fatalf("%s is pen livestock but allows training", species)
		}
		if (r.Behaviour == Hive || r.Behaviour == Mechanoid) && class != TrainNone {
			t.Fatalf("%s is an original non-tame hostile but allows training", species)
		}
	}
	if TrainabilityOf(numSpecies) != TrainNone || TrainabilityOf(Species(255)) != TrainNone {
		t.Fatal("out-of-range saved species must remain untrainable")
	}
}

func TestTrainabilityPublishedCoreAndOriginalRoles(t *testing.T) {
	// Curated against the published Core training columns. The additional
	// original creature rows follow their documented vanilla gameplay role.
	// wiki: https://rimworldwiki.com/wiki/List_of_animals
	advanced := map[Species]bool{
		TimberWolf: true, ArcticWolf: true, Elephant: true,
		GroundSloth: true, Cougar: true, GrizzlyBear: true,
		Husky: true, Labrador: true, Panther: true, PolarBear: true,
		Glimmerstag: true,
	}
	intermediate := map[Species]bool{
		ArcticFox: true, FennecFox: true, Lynx: true,
		Monkey: true, RedFox: true, Rhinoceros: true,
		YorkshireTerrier: true, DireWolf: true,
	}
	if len(advanced) != 11 || len(intermediate) != 8 {
		t.Fatal("the complete published/role-aligned trainable set changed")
	}
	for _, species := range All() {
		want := TrainNone
		switch {
		case advanced[species]:
			want = TrainAdvanced
		case intermediate[species]:
			want = TrainIntermediate
		}
		if got := TrainabilityOf(species); got != want {
			t.Errorf("%s: published class %d, got %d", species, want, got)
		}
	}
}
