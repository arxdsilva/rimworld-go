// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

// A plugin good is a row of the storage tree under its category, by its
// label, toggled by its key (PLG-026); core's tree has no such row.
func TestTreeRowsListPluginItems(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs := make([]item.Def, 0, item.CoreKinds+1)
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	items := item.NewBook(append(defs, item.Def{Label: "trail bell", Path: "Manufactured/Components"}),
		append(core.Keys(), "frontier.trail_bell"))
	z := game.ZoneView{ID: 3, Kind: "stockpile", Filter: []string{"frontier.trail_bell"}}
	found := false
	for _, r := range treeRowsIn(z, map[string]bool{}, "trail", items) {
		if r.path == "frontier.trail_bell" {
			found = r.label == "trail bell" && r.on && r.id == "storage.filter.frontier.trail_bell"
		}
	}
	if !found {
		t.Fatal("the search does not list the plugin good, checked, by its label")
	}
	for _, r := range treeRows(z, map[string]bool{}, "trail") {
		if r.path == "frontier.trail_bell" {
			t.Fatal("core's tree lists a plugin kind")
		}
	}
}
