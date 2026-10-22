package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
)

// TestSplitLoadKeepsCorpseSpecies: a load split off a stack of elk
// corpses is still elk (the split once dropped the species: deer).
func TestSplitLoadKeepsCorpseSpecies(t *testing.T) {
	st := item.Stack{Kind: item.CorpseAnimal, Qty: 3, X: 4, Y: 5, Rot: 77, Animal: uint8(animal.Elk), DiedAt: 900}
	got := splitLoad(st, 1)
	if got.Qty != 1 || got.Animal != uint8(animal.Elk) || got.DiedAt != 900 || got.Rot != 77 || got.X != 4 {
		t.Fatalf("split load = %+v", got)
	}
}
