// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import "rimworld-go/internal/domain/economy/item"

// ItemClass is the item class name in plugin files.
const ItemClass = item.Class

// itemClass is the item class (PLG-026).
// CONTRACT STUB (wave 0): the items agent adds the codec (Encode/Decode,
// as class_recipe.go does); until then a plugin's item defs are refused.
func itemClass() class {
	return class{
		name: ItemClass,
		core: func() []Def { return nil },
		decode: func(cat *Catalog, defs []Def) []Problem {
			if cat != nil {
				cat.Items = item.Core()
			}
			var probs []Problem
			for _, d := range defs {
				probs = append(probs, Problem{Plugin: d.Plugin, File: d.File, Def: d.Ref(), Msg: "item definitions are not supported yet"})
			}
			return probs
		},
	}
}
