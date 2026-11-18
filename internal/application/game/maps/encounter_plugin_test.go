// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/caravan"
)

// demandHost is a Host for Fleet.demand: the book, the caravan's goods
// and the demand letter it posts.
type demandHost struct {
	Host
	book   item.Book
	goods  []item.Stack
	letter string
}

func (h *demandHost) Items() item.Book { return h.book }
func (h *demandHost) Cargo() Freight   { return goodsOf{list: h.goods} }
func (h *demandHost) Factions() []faction.Faction {
	return []faction.Faction{{Name: "Ridge clan"}}
}
func (h *demandHost) TileName(t int) string { return fmt.Sprint("tile ", t) }
func (h *demandHost) DemandLetter(_, format string, args ...any) uint64 {
	h.letter = fmt.Sprintf(format, args...)
	return 1
}

// goodsOf is a Freight that only lists goods.
type goodsOf struct {
	Freight
	list []item.Stack
}

func (g goodsOf) Goods(uint64) []item.Stack { return g.list }

// plugBook is core's kinds plus one plugin good worth value; the second
// result is its kind.
func plugBook(value int) (item.Book, item.Kind) {
	core := item.Core()
	defs, keys := make([]item.Def, 0, item.CoreKinds+1), core.Keys()
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "star bell", Path: "Manufactured/Components", Mass: 700, Value: value, Stack: 10})
	return item.NewBook(defs, append(keys, "frontier.star_bell")), item.Kind(item.CoreKinds)
}

// A demand asks for half of the goods worth most by the book's value: a
// plugin good outvalues a bigger pile of steel, and is named by its label.
func TestPluginDemandPicksByValue(t *testing.T) {
	b, bell := plugBook(9000)
	h := &demandHost{book: b, goods: []item.Stack{{Kind: item.Steel, Qty: 100}, {Kind: bell, Qty: 7}}}
	c := &Caravan{C: caravan.Caravan{ID: 3}}
	if !Of(&[]Caravan{*c}, &Atlas{}, h).demand(c, 1, 100) {
		t.Fatal("no demand made")
	}
	if c.Ask.Kind != bell || c.Ask.Qty != 4 {
		t.Fatalf("asked %d of kind %d, want 4 of the plugin good", c.Ask.Qty, c.Ask.Kind)
	}
	if !strings.Contains(h.letter, "4 star bell") {
		t.Fatalf("letter %q does not name the good by label", h.letter)
	}
	// A cheap plugin good loses to core steel.
	b2, bell2 := plugBook(1)
	h2 := &demandHost{book: b2, goods: []item.Stack{{Kind: item.Steel, Qty: 100}, {Kind: bell2, Qty: 2}}}
	c2 := &Caravan{}
	Of(&[]Caravan{}, &Atlas{}, h2).demand(c2, 1, 100)
	if c2.Ask.Kind != item.Steel {
		t.Fatalf("asked kind %d, want steel", c2.Ask.Kind)
	}
}
