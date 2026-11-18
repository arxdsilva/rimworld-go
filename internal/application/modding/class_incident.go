// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import "rimworld-go/internal/domain/incident"

// IncidentClass is the incident class name in plugin files.
const IncidentClass = incident.Class

// incidentClass is the incident class (PLG-026).
// CONTRACT STUB (wave 0): the incidents agent adds the codec (Encode/Decode,
// as class_recipe.go does); until then a plugin's incident defs are refused.
func incidentClass() class {
	return class{
		name: IncidentClass,
		core: func() []Def { return nil },
		decode: func(cat *Catalog, defs []Def) []Problem {
			if cat != nil {
				cat.Incidents = incident.Core()
			}
			var probs []Problem
			for _, d := range defs {
				probs = append(probs, Problem{Plugin: d.Plugin, File: d.File, Def: d.Ref(), Msg: "incident definitions are not supported yet"})
			}
			return probs
		},
	}
}
