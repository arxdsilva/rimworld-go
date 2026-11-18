// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package devtools

import "rimworld-go/internal/application/game"

// IncidentSource lists the run's plugin incidents (PLG-026): the game
// answers it, read only.
type IncidentSource interface {
	PluginIncidents() []game.PluginIncident
}

// pluginIncidentActions are the debug menu's entries for the plugins'
// incidents, one per incident under its own label, fired through
// Tools.FireIncident (none without plugin incidents).
func (p *Panel) pluginIncidentActions(fire func(key, name string) Action) []Action {
	src, ok := p.sh.Env().Game.(IncidentSource)
	if !ok {
		return nil
	}
	var out []Action
	for _, in := range src.PluginIncidents() {
		out = append(out, fire(in.Key, in.Label+" (like "+in.Like+")"))
	}
	return out
}
