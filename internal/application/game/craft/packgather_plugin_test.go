// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package craft

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
)

// bookHost is the fake host with a run's item book that holds a plugin item.
type bookHost struct {
	*fakeHost
	book item.Book
}

func (h bookHost) Items() item.Book { return h.book }

// TestPackGatherPluginMass: a plugin item's pack fit and "one of each" check
// read the book's grams (700 g here), not a core kind's.
func TestPackGatherPluginMass(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs := make([]item.Def, 0, item.CoreKinds+1)
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "Widget", Mass: 700, Value: 30, Stack: 50})
	book := item.NewBook(defs, append(core.Keys(), "demo.widget"))
	k := item.Kind(item.CoreKinds)

	f := newFake()
	f.on(gameplay.HaulAllIngredients)
	f.names[pawn] = "Ann"
	h := bookHost{f, book}
	s := &Shop{h: h}

	f.room = 1500
	if got := s.fits(pawn, k); got != 2 {
		t.Fatalf("fits %d plugin units in 1500 g, want 2", got)
	}
	if !s.packable(pawn, []pick{{kind: k}}) {
		t.Fatal("a 700 g unit fits 1500 g")
	}
	f.room = 699
	if s.fits(pawn, k) != 0 || s.packable(pawn, []pick{{kind: k}}) {
		t.Fatal("a 700 g unit does not fit 699 g")
	}
}
