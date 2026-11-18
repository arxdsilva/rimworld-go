// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
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
