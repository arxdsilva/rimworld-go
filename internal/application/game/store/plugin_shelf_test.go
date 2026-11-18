// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package store

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
)

// bellBook is core's kinds plus one plugin good filed under components.
func bellBook() (item.Book, item.Kind) {
	core := item.Core()
	defs, all := make([]item.Def, 0, item.CoreKinds+1), core.Keys()
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "bell", Path: "Manufactured/Components", Mass: 1000, Value: 40, Stack: 25})
	return item.NewBook(defs, append(all, "t.bell")), item.Kind(item.CoreKinds)
}

// takes reports the zone at (x, y) lists the plugin kind among its kinds,
// as the filter tree reads it.
func takes(t *testing.T, w *world, x, y int) bool {
	return zoneAt(t, w, x, y).EffectiveIn(w.book).Has(item.Kind(item.CoreKinds))
}

func zoneAt(t *testing.T, w *world, x, y int) zone.Zone {
	t.Helper()
	id, ok := w.zs.At(x, y)
	if !ok {
		t.Fatalf("no zone at %d,%d", x, y)
	}
	z, _ := w.zs.Zone(id)
	return z
}

// TestShelfLayAdmitsPlugins: a built shelf allows the run's plugin kind,
// an egg box and a bookcase do not, and a shelf laid again over its old
// cells keeps the filter it had (PLG-026).
func TestShelfLayAdmitsPlugins(t *testing.T) {
	t.Parallel()
	w := newWorld()
	book, bell := bellBook()
	w.book = book
	w.build(construction.Shelf, 3, 3)
	w.build(construction.EggBox, 10, 3)
	w.build(construction.Bookcase, 16, 3)
	if !takes(t, w, 3, 3) {
		t.Fatal("the shelf refuses the plugin kind")
	}
	if takes(t, w, 10, 3) || takes(t, w, 16, 3) {
		t.Fatalf("egg box %v bookcase %v", takes(t, w, 10, 3), takes(t, w, 16, 3))
	}
	// a player-edited shelf keeps its filter when laid again (a move)
	id, _ := w.zs.At(3, 3)
	w.zs.Update(id, func(z *zone.Zone) { z.SetAllowedIn(zone.Filter{}.With(bell, true), false, book) })
	cells, _, _ := construction.Footprint(construction.Shelf, construction.South, 3, 3)
	Buildings{}.Lay(w.zs, w, construction.Shelf, cells)
	if takes(t, w, 3, 3) {
		t.Fatal("a re-laid shelf took its filter back")
	}
	// LayMissing lays a plugin-admitting zone for a building without one
	b := w.build(construction.Shelf, 22, 3)
	w.zs.RemoveTiles([][2]int{{22, 3}})
	Buildings{}.LayMissing(w.zs, w)
	if !takes(t, w, 22, 3) || b == 0 {
		t.Fatal("LayMissing's shelf refuses the plugin kind")
	}
}
