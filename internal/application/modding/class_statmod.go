// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/domain/pawn/stat/statmod"
)

// StatModClass is the stat_mod class name in plugin files.
const StatModClass = "stat_mod"

// Stat mod field limits: what a def may hold. Mul is permille.
const (
	smMaxAdd = 100000 // |add|: many defs sum without nearing int32
	smMaxMul = 10000  // permille, at most x10; 0 is unset, so a def never zeroes a stat
	smMaxKey = 60     // runes, of a trait or condition key
)

// capsByPlugin is the capability set each loaded plugin is granted.
type capsByPlugin map[string]pluginapi.Set

// statModClass is the stat_mod class: declarative stat modifiers, which
// only a plugin with the stats capability may add, and conditions on health
// or bio facts only with health. Core has no stat_mod defs. caps names what
// each plugin is granted (a plugin not in it has nothing).
func statModClass(caps capsByPlugin) class {
	return class{
		name: StatModClass,
		core: func() []Def { return nil },
		decode: func(cat *Catalog, defs []Def) []Problem {
			b, probs := DecodeStatMods(defs, caps)
			if cat != nil {
				cat.StatMods = b
			}
			return probs
		},
	}
}

// DecodeStatMods types the stat_mod defs (abstract ones already dropped) in
// order into a book. A def from a plugin without the stats capability is an
// error and is left out, as is one with a condition or injured but no
// health capability; any non-integer value or unknown field (a numeric
// threshold, say) is an error. Typing is strict: a bad def is left out.
func DecodeStatMods(defs []Def, caps capsByPlugin) (statmod.Book, []Problem) {
	var b statmod.Book
	var probs []Problem
	for _, d := range defs {
		sd := smDecoder{bsDecoder: bsDecoder{def: d}, caps: caps[d.Plugin]}
		v := sd.decode()
		if len(sd.probs) > 0 {
			probs = append(probs, sd.probs...)
			continue
		}
		b.Defs = append(b.Defs, v)
	}
	return b, probs
}

// smDecoder types one stat_mod def.
type smDecoder struct {
	bsDecoder
	caps pluginapi.Set
}

func (sd *smDecoder) decode() statmod.Def {
	var d statmod.Def
	def := sd.def
	switch {
	case def.Class != StatModClass:
		sd.fail("", "not a %s def", StatModClass)
		return d
	case def.Plugin == Core:
		sd.fail("", "the game has no %s defs of its own", StatModClass)
		return d
	case !sd.caps.Has(pluginapi.Stats):
		sd.fail("", "needs the %q capability, which the plugin does not declare", pluginapi.Stats)
		return d
	}
	d.Key = def.Key
	for _, k := range rcSorted(def.Body) {
		sd.field(&d, k, def.Body[k])
	}
	if _, ok := def.Body["stat"]; !ok {
		sd.fail("stat", "required")
	}
	return d
}

func (sd *smDecoder) field(d *statmod.Def, k string, v any) {
	switch k {
	case "stat":
		s := sd.text(k, v, 1, smMaxKey)
		id, ok := statmod.StatByKey(s)
		if !ok && s != "" {
			sd.fail(k, "unknown stat %q", s)
		}
		d.Stat = id
	case "add":
		d.Add = int32(sd.num(k, v, -smMaxAdd, smMaxAdd)) // #nosec G115 -- bounded by smMaxAdd
	case "mul":
		d.Mul = int32(sd.num(k, v, 0, smMaxMul)) // #nosec G115 -- bounded by smMaxMul
	case "trait":
		d.When.Trait = sd.text(k, v, 1, smMaxKey)
	case "condition":
		d.When.Condition = sd.text(k, v, 1, smMaxKey)
		sd.needHealth(k)
	case "injured":
		b, ok := v.(bool)
		if !ok {
			sd.fail(k, "want a boolean, got %s", bsJSONType(v))
		}
		d.When.Injured = b
		sd.needHealth(k)
	default:
		sd.fail(k, "unknown field")
	}
}

// needHealth fails field k when the plugin lacks the health capability.
func (sd *smDecoder) needHealth(k string) {
	if !sd.caps.Has(pluginapi.Health) {
		sd.fail(k, "needs the %q capability, which the plugin does not declare", pluginapi.Health)
	}
}
