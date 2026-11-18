// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// TestGoodsKgReadsTheBook: a plugin kind's mass comes from the run's
// book (700 g x 10 = 7 kg); core kinds weigh what they always did.
func TestGoodsKgReadsTheBook(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs, keys := make([]item.Def, 0, core.Len()+1), core.Keys()
	for k := range core.Len() {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "copper bell", Path: "Manufactured/Components", Mass: 700, Value: 40, Stack: 25})
	b := item.NewBook(defs, append(keys, "frontier.bell"))
	if got := GoodsKg(b, "frontier.bell", 10); got != 7 {
		t.Fatalf("plugin kind 700 g x 10 = %d kg, want 7", got)
	}
	if got := GoodsKg(item.Book{}, "frontier.bell", 10); got != 0 {
		t.Fatalf("core book knows frontier.bell: %d kg", got)
	}
	if got, want := GoodsKg(b, "steel", 7), (7*item.Steel.MassGrams()+999)/1000; got != want {
		t.Fatalf("core steel = %d kg, want %d", got, want)
	}
}

// TestFormReasonWeighsPluginGoods: a caravan carrying a heavy plugin kind
// is too heavy for one colonist; a light load is fine.
func TestFormReasonWeighsPluginGoods(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs, keys := make([]item.Def, 0, core.Len()+1), core.Keys()
	for k := range core.Len() {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "anvil", Path: "Manufactured/Components", Mass: 700000, Value: 40, Stack: 5})
	s, dl := newFake()
	s.items = item.NewBook(defs, append(keys, "frontier.anvil"))
	f := &formCaravan{picked: map[entity.EntityID]bool{1: true}, beasts: map[entity.EntityID]bool{}, goods: map[string]int{}, dest: 3}
	if r := dl.formReason(f); r != "" {
		t.Fatalf("empty load: %q", r)
	}
	f.goods["frontier.anvil"] = 1
	if r := dl.formReason(f); r != FormTooHeavy {
		t.Fatalf("700 kg on one colonist: %q, want %q", r, FormTooHeavy)
	}
}
