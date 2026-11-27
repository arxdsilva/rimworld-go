// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"slices"

	"rimworld-go/internal/application/pluginapi"
)

// ScriptDefs is the load-time side of plugins' scripts, which Build asks
// while it collects defs (the script host implements it).
type ScriptDefs interface {
	// Register is p's register_defs() results as defs files of p, shaped
	// like any defs file ({"<class>": [ {def} ]}), and their problems.
	Register(p Plugin) ([]File, []Problem)
	// Validate is plugin validator's validate_defs(defs) problems.
	Validate(validator string, defs []Def) []Problem
}

// register is p with its script-registered defs files added to a copy of
// its own (they travel the same way as the JSON ones), and the problems.
func register(sd ScriptDefs, p Plugin, caps pluginapi.Set) (Plugin, []Problem) {
	if sd == nil || !(caps.Has(pluginapi.Defs) || caps.Has(pluginapi.Validate)) {
		return p, nil
	}
	// A validate-only plugin is asked too: the host loads its scripts in
	// Register, and validate_defs needs them. Its files count only with defs.
	files, probs := sd.Register(p)
	if caps.Has(pluginapi.Defs) {
		p.Defs = append(slices.Clone(p.Defs), files...)
	}
	return p, probs
}

// validate runs each validating plugin's validate_defs over defs, in load
// order, and returns the defs left and the problems. An error naming a
// plugin's own def drops it (later validators do not see it); on a core
// def it is reissued as a warning, because the game's own is kept; a
// problem naming no known def is a warning.
func validate(sd ScriptDefs, order []Placed, caps capsByPlugin, defs []Def) ([]Def, []Problem) {
	if sd == nil {
		return defs, nil
	}
	var out []Problem
	for _, placed := range order {
		if placed.ID == Core || !caps[placed.ID].Has(pluginapi.Validate) {
			continue
		}
		var dropped []string
		for _, p := range sd.Validate(placed.ID, slices.Clone(defs)) {
			if p.Plugin == "" {
				p.Plugin = placed.ID
			}
			if p.Warn || p.Def == "" {
				out = append(out, p)
				continue
			}
			i := slices.IndexFunc(defs, func(d Def) bool { return d.Ref() == p.Def })
			switch {
			case i < 0:
				p.Warn, p.Msg = true, p.Msg+" (names a def that does not exist)"
			case defs[i].Plugin == Core:
				p.Warn, p.Msg = true, p.Msg+" (the game's own definition is kept)"
			default:
				dropped = append(dropped, p.Def)
				p.Msg += " (the definition is left out)"
			}
			out = append(out, p)
		}
		if len(dropped) > 0 {
			defs = slices.DeleteFunc(slices.Clone(defs), func(d Def) bool { return slices.Contains(dropped, d.Ref()) })
		}
	}
	return defs, out
}
