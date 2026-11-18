// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package recipe

import (
	"strconv"

	"rimworld-go/internal/domain/economy/item"
)

// smeltRate is the steel a plugin kind gives back: num/den per unit.
type smeltRate struct{ num, den int }

// WithItems is b smelting the run's plugin item kinds too (PLG-026): a
// plugin kind gives back its def's Smelt steel a unit, else SmeltPct of
// the steel b's recipe making it takes per unit made. A plugin kind with
// neither is not smeltable. Core kinds smelt as SmeltYield says.
func (b Book) WithItems(items item.Book) Book {
	b = b.self()
	b.smelt, b.items = nil, items
	for _, k := range items.Plugins() {
		r := smeltRate{num: items.Def(k).Smelt, den: 1}
		if r.num == 0 {
			r = b.madeOfSteel(k)
		}
		if r.num > 0 {
			if b.smelt == nil {
				b.smelt = make([]smeltRate, items.Len())
			}
			b.smelt[k] = r
		}
	}
	return b
}

// Items is the run's item kinds the book was made with (zero: core).
func (b Book) Items() item.Book { return b.items }

// madeOfSteel is SmeltPct of the steel the first recipe making k takes,
// per unit it makes.
func (b Book) madeOfSteel(k item.Kind) smeltRate {
	for _, d := range b.defs {
		if d.Output == k && d.Builds == 0 && !d.Legacy && d.OutQty > 0 {
			if n := d.steelIn(0); n > 0 {
				return smeltRate{num: n * SmeltPct, den: 100 * d.OutQty}
			}
		}
	}
	return smeltRate{}
}

// smeltOf is plugin kind k's rate (zero: not smeltable, or a core kind).
func (b Book) smeltOf(k item.Kind) smeltRate {
	if int(k) < len(b.smelt) {
		return b.smelt[k]
	}
	return smeltRate{}
}

// SmeltYield is the steel smelting stack s gives back in this run: a
// core kind's SmeltYield, a plugin kind's rate times its quantity (at
// least 1).
func (b Book) SmeltYield(s item.Stack) int {
	if r := b.smeltOf(s.Kind); r.num > 0 {
		return max(1, r.num*max(1, s.Qty)/r.den)
	}
	return SmeltYield(s)
}

// Takes is in.Accepts in this run: a smelt input (Smelt, not SmeltWear)
// also takes every smeltable plugin kind.
func (b Book) Takes(in Input, k item.Kind) bool {
	if in.Smelt && b.smeltOf(k).num > 0 {
		return true
	}
	return in.Accepts(k)
}

// TakesStack is in.AcceptsStack in this run (Takes, by the stack).
func (b Book) TakesStack(in Input, s item.Stack) bool {
	if in.Smelt && b.smeltOf(s.Kind).num > 0 {
		return true
	}
	return in.AcceptsStack(s)
}

// InputsLabel is d.InputsLabel in this run: a plugin kind by its label.
func (b Book) InputsLabel(d Def) string {
	ins := d.Inputs
	if len(ins) == 0 {
		ins = []Input{d.Input}
	}
	s := ""
	for i, in := range ins {
		if i > 0 {
			s += " + "
		}
		if b.items.Plugin(in.Kind) && in.Stuff == 0 {
			s += strconv.Itoa(in.Qty) + " " + b.items.Label(in.Kind)
			continue
		}
		s += in.Label()
	}
	return s
}

// DefTakes is d.Accepts in this run: one of d's inputs Takes k. It
// allocates nothing (the bill views ask it for every kind).
func (b Book) DefTakes(d Def, k item.Kind) bool {
	if d.Accepts(k) {
		return true
	}
	if b.smeltOf(k).num == 0 {
		return false
	}
	if len(d.Inputs) == 0 {
		return d.Input.Smelt
	}
	for _, in := range d.Inputs {
		if in.Smelt {
			return true
		}
	}
	return false
}
