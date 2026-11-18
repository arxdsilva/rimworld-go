// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package zone

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// zbBook is core plus one plugin good, a copper bell filed under components.
func zbBook() (item.Book, item.Kind) {
	core := item.Core()
	defs := make([]item.Def, 0, item.CoreKinds+1)
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "copper bell", Path: "Manufactured/Components", Mass: 700, Value: 40, Stack: 25})
	return item.NewBook(defs, append(core.Keys(), "frontier.bell")), item.Kind(item.CoreKinds)
}

// A plugin good is stored like components: a stockpile's older switches
// take it, a dump's does not, nor does a hopper's ceiling; the filter tree files it under
// its category by its key and label (PLG-026).
func TestPluginItemStorage(t *testing.T) {
	t.Parallel()
	b, bell := zbBook()
	pile, dump := Zone{Kind: KindStockpile, Food: true, Wood: true, Stone: true}, Zone{Kind: KindDump}
	if !pile.Allows(bell) || dump.Allows(bell) || !UniverseHas(KindStockpile, bell) ||
		UniverseHas(KindHopper, bell) || pile.Allows(item.None) {
		t.Fatal("legacy zones store a plugin good unlike components")
	}
	if pile.Effective().Has(bell) || !pile.EffectiveIn(b).Has(bell) {
		t.Fatal("only the run's book adds the plugin kind to a zone's effective filter")
	}
	i := slices.IndexFunc(TreeIn(b), func(n Node) bool { return n.Leaf && n.Kind == bell })
	if i < 0 || TreeIn(b)[i].Path != "frontier.bell" || TreeIn(b)[i].Label != "copper bell" ||
		slices.ContainsFunc(Tree(), func(n Node) bool { return n.Kind == bell }) {
		t.Fatalf("tree leaf: %d", i)
	}
	for _, name := range []string{Root, "Manufactured", "Manufactured/Components", "frontier.bell"} {
		if f, ok := ResolveIn(name, b); !ok || !f.Has(bell) {
			t.Errorf("ResolveIn(%q) lacks the bell", name)
		}
	}
	if f, _ := Resolve(Root); f.Has(bell) {
		t.Error("core's Root holds a plugin kind")
	}
	raw, _ := ResolveIn("Raw resources", b)
	pile.SetAllowedIn(raw, false, b)
	if !pile.Custom || !pile.Allows(bell) || pile.Allows(item.Steel) {
		t.Fatal("editing a legacy zone dropped the plugin good")
	}
	comps, _ := ResolveIn("Manufactured/Components", b)
	pile.SetAllowedIn(comps, false, b)
	if pile.Allows(bell) || slices.Contains(pile.Filter.NamesIn(b), "frontier.bell") {
		t.Fatal("clearing components kept the bell")
	}
	pile.SetAllowedIn(comps, true, b)
	rows := RowsForProfileIn(KindStockpile, ProfileNone, pile.Filter.NamesIn(b), b)
	if j := slices.IndexFunc(rows, func(r Row) bool { return r.Leaf && r.Path == "frontier.bell" }); j < 0 || !rows[j].On || rows[j].Label != "copper bell" {
		t.Fatalf("rows lack the bell on: %d", j)
	}
	var shelf Zone
	shelf.Kind = KindShelf
	shelf.PasteFromIn(pile, b)
	if !shelf.Allows(bell) {
		t.Fatal("pasting lost the bell")
	}
}
