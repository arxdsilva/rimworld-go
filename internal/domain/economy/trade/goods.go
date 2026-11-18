// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"cmp"
	"strings"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// Crated buildings in trade (AX-1, wiki: Minified thing): traders stock
// crated furniture no colony can build (the orbital exotic trader's big
// screen television, telescope and urns; a bulk hauler's urn). A crate is
// one line per building kind, keyed by Goods; it arrives as an
// item.MinifiedBuilding of that kind in its default material, which
// Reinstall places. The colony's own crates are not bought back yet.

// Goods is a trade key: an item kind and, for a crated building, its
// construction kind (0 for any other goods).
type Goods struct {
	Kind     item.Kind
	Building uint8
}

// Goods is line l's key.
func (l Line) Goods() Goods { return Goods{l.Kind, l.Building} }

// Compare orders goods by kind, then building.
func (g Goods) Compare(o Goods) int {
	if c := cmp.Compare(g.Kind, o.Kind); c != 0 {
		return c
	}
	return cmp.Compare(g.Building, o.Building)
}

// crateSep joins a crate's item name and building name in Name.
const crateSep = ":"

// Name is the goods' row key: the item kind's name in the run's book b
// (a plugin kind's key), and for a crate "crated building:<building>"
// (ParseGoods reads it back).
func (g Goods) Name(b item.Book) string {
	if g.Building == 0 {
		return b.Name(g.Kind)
	}
	return g.Kind.String() + crateSep + construction.Kind(g.Building).String()
}

// Label is the goods' display name: "Big screen television (crated)" for
// a crate, a plugin kind's def label, else "" (the caller's core item
// label stands).
func (g Goods) Label(b item.Book) string {
	if g.Building == 0 {
		if b.Plugin(g.Kind) {
			return b.Label(g.Kind)
		}
		return ""
	}
	n := construction.Kind(g.Building).String()
	return strings.ToUpper(n[:1]) + n[1:] + " (crated)"
}

// ParseGoods reads Name's output (a plugin kind's key by the run's book
// b).
func ParseGoods(b item.Book, name string) (Goods, bool) {
	if kn, bn, crate := strings.Cut(name, crateSep); crate {
		k, ok := item.ParseKind(kn)
		bld, okB := construction.Parse(bn)
		if !ok || !okB || k != item.MinifiedBuilding || int(bld) > 255 {
			return Goods{}, false
		}
		return Goods{k, uint8(bld)}, true
	}
	k, ok := b.Parse(name)
	return Goods{Kind: k}, ok
}

// Value is one unit's market value in silver: a crate's CrateValue, else
// its item kind's in the run's book b.
func (g Goods) Value(b item.Book) int {
	if g.Building != 0 {
		return CrateValue(construction.Kind(g.Building))
	}
	return b.Value(g.Kind)
}

// CrateValue is the market value of crated building k when a trader sells
// it (wiki: Megascreen television 1485, Telescope 350; Urn, "20 to 2000
// by material": 200, original tuning in that range); 0 for a building no
// trader sells.
func CrateValue(k construction.Kind) int {
	switch k {
	case construction.BigScreenTV:
		return 1485
	case construction.Telescope:
		return 350
	case construction.Urn:
		return 200
	}
	return 0
}

// CrateStuff is the material a crate of building b arrives in (0 for b 0,
// goods that are no crate).
func CrateStuff(b uint8) item.Kind {
	if b == 0 {
		return 0
	}
	return construction.DefaultStuff(construction.Kind(b))
}

// crateEntry is a crated-building stock candidate: Min..Max crates of
// building kind, present with chance Pct.
type crateEntry struct {
	kind     construction.Kind
	min, max int
	pct      int
}

// cratePools are the crates each trader kind may stock (wiki: Megascreen
// television and Telescope, from exotic orbital traders; Urn, found or
// traded). Counts and chances are original tuning.
var cratePools = [NumKinds][]crateEntry{
	BulkHauler:    {{construction.Urn, 1, 1, 20}},
	CurioMerchant: {{construction.BigScreenTV, 1, 1, 30}, {construction.Telescope, 1, 1, 40}, {construction.Urn, 1, 2, 40}},
}
