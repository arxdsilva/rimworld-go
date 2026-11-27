// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package statmod is the declarative stat_mod definition class plugins
// add stat modifiers with: a Def adds Add and multiplies by Mul (permille)
// one stat while its condition holds. Conditions are discrete facts only
// (a trait, a bio condition, being injured), so there is no numeric
// threshold and nothing to flap; Book.Layer is the last layer of a pawn's
// stat sheet. Pure and deterministic: integers only, defs in book order.
package statmod

import "rimworld-go/internal/domain/pawn/stat"

// Cond is when a Def applies; the zero Cond always holds.
type Cond struct {
	Trait     string // a trait key the pawn must have ("" any)
	Condition string // a bio condition key the pawn must have ("" any)
	Injured   bool   // the pawn must be injured
}

// Def is one stat_mod definition.
type Def struct {
	Key  string // the def's namespaced key
	Stat stat.ID
	Add  int32 // offset
	Mul  int32 // permille rate, 0 = none
	When Cond
}

// Book is the run's stat_mod defs, in content order.
type Book struct {
	Defs []Def
}

// Facts are what a pawn is, as the conditions read it.
type Facts struct {
	Traits     []string // trait keys
	Conditions []string // bio condition keys
	Injured    bool
}

// Layer is the mods of every def whose condition f meets, in book order;
// nil (no allocation) when none match.
func (b Book) Layer(f Facts) []stat.Mod {
	var out []stat.Mod
	for i := range b.Defs {
		d := &b.Defs[i]
		if !d.When.holds(f) {
			continue
		}
		out = append(out, stat.Mod{Stat: d.Stat, Add: d.Add, Mul: d.Mul})
	}
	return out
}

// holds reports whether every set part of c is met by f.
func (c Cond) holds(f Facts) bool {
	if c.Injured && !f.Injured {
		return false
	}
	return (c.Trait == "" || has(f.Traits, c.Trait)) &&
		(c.Condition == "" || has(f.Conditions, c.Condition))
}

func has(keys []string, k string) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

// StatByKey finds the stat a def names by its key ("work_speed"), over the
// closed key table of the stat package.
func StatByKey(k string) (stat.ID, bool) { return stat.ParseKey(k) }
