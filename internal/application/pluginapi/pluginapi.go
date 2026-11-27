// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package pluginapi is the versioned contract between plugins and the
// host: the plugin API version a plugin.json declares ("api", separate
// from the game version) and the closed set of capabilities it may ask
// for ("capabilities"). Names are parsed here once, so the rest of the
// game only ever sees a Set; an unknown name can never become a
// Capability. It is a leaf: standard library only.
package pluginapi

// The plugin API versions this host runs: a plugin declaring an API
// outside [MinVersion, Version] does not load.
const (
	Version    = 1
	MinVersion = 1
)

// Capability is one named power a plugin declares and the host grants.
// The zero value is no capability: Set.Has(0) is false.
type Capability uint8

// The capabilities, in canonical order.
const (
	Query     Capability = iota + 1 // ctx reads: tick, day, hour, pawns, count, rand…
	Commands                        // ctx.message, ctx.letter, ctx.spawn
	Events                          // the clock, incident and cadence hooks, on_event
	World                           // ctx.animals, ctx.buildings, ctx.map, ctx.world
	Defs                            // register_defs
	Validate                        // validate_defs, validate_command
	UI                              // panel, on_action
	Incidents                       // run_incident
	Jobs                            // ctx.order
	Stats                           // stat_mod defs
	Health                          // stat_mod on injured or a bio condition
	Recipes                         // recipe_done, ctx.place in it
	Worldgen                        // map_generated, ctx.place in it
)

// names are the capabilities' plugin.json names, by value.
var names = [...]string{
	Query: "query", Commands: "commands", Events: "events", World: "world", Defs: "defs",
	Validate: "validate", UI: "ui", Incidents: "incidents", Jobs: "jobs", Stats: "stats",
	Health: "health", Recipes: "recipes", Worldgen: "worldgen",
}

// String is the capability's plugin.json name; "" for no capability.
func (c Capability) String() string {
	if int(c) >= len(names) {
		return ""
	}
	return names[c]
}

// Set is a set of capabilities, one bit per Capability.
type Set uint32

// Has reports that c is in s; never for the zero Capability.
func (s Set) Has(c Capability) bool {
	return c != 0 && int(c) < len(names) && s&(1<<c) != 0
}

// Names is s's capability names in canonical order.
func (s Set) Names() []string {
	var out []string
	for c := Query; int(c) < len(names); c++ {
		if s.Has(c) {
			out = append(out, c.String())
		}
	}
	return out
}

// Parse maps plugin.json names to a Set, reporting the names that are no
// capability and those listed twice (each once, in first-seen order).
func Parse(in []string) (s Set, unknown, dup []string) {
	seen := map[string]bool{}
	reported := map[string]bool{}
	for _, n := range in {
		c, ok := byName(n)
		switch {
		case !ok:
			if !reported[n] {
				reported[n] = true
				unknown = append(unknown, n)
			}
			continue
		case seen[n]:
			if !reported[n] {
				reported[n] = true
				dup = append(dup, n)
			}
			continue
		}
		seen[n] = true
		s |= 1 << c
	}
	return s, unknown, dup
}

func byName(n string) (Capability, bool) {
	for c := Query; int(c) < len(names); c++ {
		if names[c] == n {
			return c, true
		}
	}
	return 0, false
}

// Legacy is the set of a manifest without "capabilities": what scripts
// could do before capabilities existed.
func Legacy() Set { return 1<<Query | 1<<Commands | 1<<Events }
