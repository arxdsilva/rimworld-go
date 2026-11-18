// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import "rimworld-go/internal/domain/pawn/trait"

// TraitClass is the trait class name in plugin files.
const TraitClass = trait.Class

// traitClass is the trait class (PLG-026).
// CONTRACT STUB (wave 0): the traits agent adds the codec (Encode/Decode,
// as class_recipe.go does); until then a plugin's trait defs are refused.
func traitClass() class {
	return class{
		name: TraitClass,
		core: func() []Def { return nil },
		decode: func(cat *Catalog, defs []Def) []Problem {
			if cat != nil {
				cat.Traits = trait.Core()
			}
			var probs []Problem
			for _, d := range defs {
				probs = append(probs, Problem{Plugin: d.Plugin, File: d.File, Def: d.Ref(), Msg: "trait definitions are not supported yet"})
			}
			return probs
		},
	}
}
