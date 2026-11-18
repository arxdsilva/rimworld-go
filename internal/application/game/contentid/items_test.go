// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package contentid

import (
	"testing"

	itemkind "rimworld-go/internal/domain/economy/item"
)

// Core's item book, zero or built, adds nothing to the fingerprint; a
// plugin item or a patched core item changes it (PLG-026).
func TestItemsInFingerprint(t *testing.T) {
	t.Parallel()
	core := itemkind.Core()
	defs := make([]itemkind.Def, 0, itemkind.CoreKinds+1)
	for k := range itemkind.CoreKinds {
		defs = append(defs, core.Def(itemkind.Kind(k)))
	}
	built := itemkind.NewBook(defs, core.Keys())
	if Of(nil, Books{Items: built}).Hash != Fingerprint() || Of(nil, Books{Items: core}).Hash != Fingerprint() {
		t.Fatal("core's item book moved the fingerprint")
	}
	patched := append([]itemkind.Def(nil), defs...)
	patched[itemkind.Wool].Value++
	plugin := append(append([]itemkind.Def(nil), defs...), itemkind.Def{Label: "bell", Path: "Items/Parts", Value: 5})
	a := Of(nil, Books{Items: itemkind.NewBook(patched, core.Keys())}).Hash
	b := Of(nil, Books{Items: itemkind.NewBook(plugin, append(core.Keys(), "x.bell"))}).Hash
	if a == Fingerprint() || b == Fingerprint() || a == b {
		t.Fatal("a patched or plugin item left the fingerprint as it was")
	}
}
