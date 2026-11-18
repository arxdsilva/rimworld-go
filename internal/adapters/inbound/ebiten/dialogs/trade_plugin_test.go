// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

// TestPluginTradeRowParses: a trade row naming a plugin good by its key
// (game.TradeView) becomes an offer line of that kind through the run's
// book, and the encoded offer decodes to the same deal; a book without
// the plugin leaves the row out instead of sending a wrong kind.
func TestPluginTradeRowParses(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs, keys := make([]item.Def, 0, core.Len()+1), core.Keys()
	for k := range core.Len() {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "copper bell", Path: "Manufactured/Components", Value: 40, TraderStock: 6})
	b := item.NewBook(defs, append(keys, "frontier.bell"))
	bell := item.Kind(item.CoreKinds)
	v := testTradeView()
	v.Rows = append(v.Rows, game.TradeRowView{Kind: "frontier.bell", Label: "copper bell", Category: "Manufactured",
		ColonyQty: 4, TraderQty: 6, BuyC: 5800, SellC: 2200, WillBuy: true, WillSell: true})
	d := newTradeDraft(v)
	d.step(v.Rows[2], 3)  // buy 3 bells
	d.step(v.Rows[0], -2) // sell 2 wood
	offer := d.offer(b)
	want := []trade.Delta{{Kind: item.Wood, Qty: -2}, {Kind: bell, Qty: 3}}
	if len(offer) != 2 || offer[0] != want[0] || offer[1] != want[1] {
		t.Fatalf("offer %+v, want %+v", offer, want)
	}
	got, err := trade.DecodeOffer(b, trade.EncodeOffer(offer))
	if err != nil || len(got) != 2 || got[1] != want[1] {
		t.Fatalf("decoded %+v, %v", got, err)
	}
	if core := d.offer(item.Book{}); len(core) != 1 || core[0] != want[0] {
		t.Fatalf("the core book's offer %+v", core)
	}
}
