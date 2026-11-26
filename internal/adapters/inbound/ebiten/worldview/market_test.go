// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package worldview

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/caravan"
)

func TestMarketDraftAmountsAndGiftMode(t *testing.T) {
	d := &marketDraft{view: tabview.MarketView{
		CaravanID: 7, Negotiator: 3, CaravanSilver: 2, TraderSilver: 40,
		Rows: []tabview.MarketRow{
			{Kind: item.Cloth.String(), Label: "Cloth", CargoQty: 4, StockQty: 6, BuyC: 310, SellC: 140, WillBuy: true, WillSell: true},
			{Kind: item.Wood.String(), Label: "Wood", CargoQty: 7, StockQty: 0, SellC: 30, WillSell: false},
		},
	}, amounts: make([]int, 2)}
	d.step(0, 10)
	if got := d.amounts[0]; got != 6 {
		t.Fatalf("buy clamped to %d, want 6", got)
	}
	if got := d.reason(); got != "The caravan lacks silver." {
		t.Fatalf("unaffordable quote reason %q", got)
	}
	d.step(0, -10)
	if got := d.amounts[0]; got != -4 {
		t.Fatalf("sell clamped to %d, want -4", got)
	}
	d.giftMode(true)
	d.step(0, 4)
	d.step(1, -1)
	if got := d.offer(); len(got) != 1 || got[0] != (trade.Delta{Kind: item.Wood, Qty: -1}) {
		t.Fatalf("gift offer after reducing cloth: %+v", got)
	}
	d.giftMode(false)
	if d.amounts[1] != 0 || d.reason() != "Choose goods to trade." {
		t.Fatalf("gift-only wood remained for paid trade: %+v", d)
	}
}

func TestMarketDraftSilverGiftAndInvalidNegotiator(t *testing.T) {
	g := colonyGame(t)
	c := &g.world.Caravans[0]
	c.State, c.Tile = caravan.Visiting.Label(), g.sites[1].Tile
	g.world.Markets = []tabview.MarketView{{CaravanID: c.ID, Negotiator: 0,
		Tile: c.Tile, CaravanSilver: 9}}
	sh, v := newFake(g)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if gz := gizmoOf(t, v, "trade"); gz.why == "" {
		t.Fatal("an incapacitated negotiator can open the settlement market")
	}
	marketDialog{v}.open(tabview.MarketView{CaravanID: c.ID, Negotiator: 3,
		CaravanSilver: 9})
	if len(sh.kit.Modals) == 0 {
		t.Fatal("gift market did not open")
	}
	frame(sh, v, -1, -1, inputframe.Raw{})
	sh.kit.RunModals(nil)
	w, ok := sh.kit.Find("world.market.gift")
	if !ok || w.Disabled {
		t.Fatal("gift toggle missing")
	}
	w.Click()
	frame(sh, v, -1, -1, inputframe.Raw{})
	sh.kit.RunModals(nil)
	sell, ok := sh.kit.Find("world.market.sell.0")
	if !ok || sell.Disabled {
		t.Fatal("silver gift lacks its give action")
	}
	sell.Click()
	frame(sh, v, -1, -1, inputframe.Raw{})
	sh.kit.RunModals(nil)
	accept := sh.kit.Modals[len(sh.kit.Modals)-1].Buttons[0]
	if why := accept.Disabled(); why != "" {
		t.Fatalf("silver gift blocked: %s", why)
	}
	accept.OnClick()
	if len(sh.cmds) != 1 || !sh.cmds[0].Enabled ||
		sh.cmds[0].Name != trade.EncodeOffer([]trade.Delta{{Kind: item.Silver, Qty: -1}}) {
		t.Fatalf("silver gift command %+v", sh.cmds)
	}
}

func TestWorldMarketGizmoOpensAuthoritativeTradeCommand(t *testing.T) {
	g := colonyGame(t)
	c := &g.world.Caravans[0]
	c.State, c.Tile = caravan.Visiting.Label(), g.sites[1].Tile
	g.world.Markets = []tabview.MarketView{{
		CaravanID: c.ID, Negotiator: 19, Tile: c.Tile,
		CaravanSilver: 100, TraderSilver: 250,
		Rows: []tabview.MarketRow{{
			Kind: item.Cloth.String(), Label: "Cloth", StockQty: 8,
			BuyC: 140, WillBuy: true,
		}},
	}}
	sh, v := newFake(g)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	gizmoOf(t, v, "trade").run()
	if sh.kit.Top() != marketModalID {
		t.Fatalf("trade opened modal %q", sh.kit.Top())
	}
	frame(sh, v, -1, -1, inputframe.Raw{})
	sh.kit.RunModals(nil)
	w, ok := sh.kit.Find("world.market.buy.0")
	if !ok || w.Disabled {
		t.Fatalf("market stock row unavailable: widget %+v, found %v", w, ok)
	}
	w.Click()
	frame(sh, v, -1, -1, inputframe.Raw{})
	sh.kit.RunModals(nil)
	accept := sh.kit.Modals[len(sh.kit.Modals)-1].Buttons[0]
	if why := accept.Disabled(); why != "" {
		t.Fatalf("affordable offer blocked: %s", why)
	}
	accept.OnClick()
	if len(sh.cmds) != 1 {
		t.Fatalf("market sent %d commands", len(sh.cmds))
	}
	cmd := sh.cmds[0]
	want := trade.EncodeOffer([]trade.Delta{{Kind: item.Cloth, Qty: 1}})
	if cmd.Kind != command.WorldCaravanTrade || cmd.Target != c.ID ||
		cmd.Pawn != 19 || cmd.Name != want || cmd.Enabled {
		t.Fatalf("market command %+v, want %q", cmd, want)
	}
}
