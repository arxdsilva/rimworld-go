// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import "rimworld-go/internal/domain/research"

// ResearchClass is the research class name in plugin files.
const ResearchClass = research.Class

// researchClass is the research class (PLG-026).
// CONTRACT STUB (wave 0): the researchs agent adds the codec (Encode/Decode,
// as class_recipe.go does); until then a plugin's research defs are refused.
func researchClass() class {
	return class{
		name: ResearchClass,
		core: func() []Def { return nil },
		decode: func(cat *Catalog, defs []Def) []Problem {
			if cat != nil {
				cat.Research = research.Core()
			}
			var probs []Problem
			for _, d := range defs {
				probs = append(probs, Problem{Plugin: d.Plugin, File: d.File, Def: d.Ref(), Msg: "research definitions are not supported yet"})
			}
			return probs
		},
	}
}
