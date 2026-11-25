// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Regression coverage for paused market polling.
package worldview

import (
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/caravan"
	"testing"
)

type leadCountingMarketGame struct {
	*fakeGame
	marketReads int
}

func (g *leadCountingMarketGame) WorldMap() game.WorldMapView {
	g.marketReads++
	return g.fakeGame.WorldMap()
}
func TestPausedMarketDoesNotPollFullWorld(t *testing.T) {
	g := &leadCountingMarketGame{fakeGame: colonyGame(t)}
	c := &g.world.Caravans[0]
	c.State, c.Tile = caravan.Visiting.Label(), g.sites[1].Tile
	m := tabview.MarketView{CaravanID: c.ID, Negotiator: 3, Tile: c.Tile, Quote: 123, CaravanSilver: 100, TraderSilver: 100, Rows: []tabview.MarketRow{{Kind: item.Cloth.String(), ItemKind: item.Cloth, Label: "Cloth", StockQty: 8, BuyC: 100, WillBuy: true}}}
	g.world.Markets = []tabview.MarketView{m}
	sh, v := newFake(g)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	marketDialog{v}.open(m)
	frame(sh, v, -1, -1, inputframe.Raw{})
	sh.kit.RunModals(nil)
	base := g.marketReads
	for range 5 {
		frame(sh, v, -1, -1, inputframe.Raw{})
		sh.kit.RunModals(nil)
	}
	t.Logf("full WorldMap reads in five unchanged paused modal frames: %d", g.marketReads-base)
	if g.marketReads != base {
		t.Fatalf("paused unchanged modal bypasses refresh cadence: %d full reads", g.marketReads-base)
	}
}
