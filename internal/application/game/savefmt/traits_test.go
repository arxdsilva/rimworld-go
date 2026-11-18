// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

func traitBook(keys ...string) trait.Book {
	core := trait.Core()
	list, all := core.All(), core.Keys()
	for _, k := range keys {
		list = append(list, trait.Def{Key: k, Label: k})
		all = append(all, k)
	}
	return trait.NewBook(list, all)
}

// RemapTraits moves bios', colonists' and other pawns' trait IDs by key,
// and takes a trait whose key the run lacks off the pawn.
func TestRemapTraits(t *testing.T) {
	saved := traitBook("a.one", "b.two")
	one, _ := saved.IDOf("a.one")
	two, _ := saved.IDOf("b.two")
	keys := Keys{trait.Class: saved.Keys()}
	mk := func() pawngen.Pawn { return pawngen.Pawn{Traits: []trait.ID{trait.Sunny, one, two}} }

	run := traitBook("b.two") // a.one is gone, b.two moved down
	moved, _ := run.IDOf("b.two")
	bios := map[entity.EntityID]pawngen.Pawn{1: mk()}
	colonists := []pawngen.Pawn{mk()}
	other := mk()
	keys.RemapTraits(run, bios, colonists, &other, nil)
	want := []trait.ID{trait.Sunny, moved}
	for name, got := range map[string][]trait.ID{"bio": bios[1].Traits, "colonist": colonists[0].Traits, "other": other.Traits} {
		if !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}

	// The saved slice is not shared with the remapped pawn's.
	p := mk()
	q := p.RemappedTraits(keys.TraitIDs(run))
	if p.Traits[1] != one || len(q.Traits) != 2 {
		t.Errorf("remap changed its input: %v -> %v", p.Traits, q.Traits)
	}

	// A save before trait keys keeps its IDs.
	old := mk()
	Keys{}.RemapTraits(run, nil, []pawngen.Pawn{old})
	if !slices.Equal(old.Traits, []trait.ID{trait.Sunny, one, two}) {
		t.Errorf("old save changed: %v", old.Traits)
	}
}
