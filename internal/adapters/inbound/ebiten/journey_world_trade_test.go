// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/planet"
)

// WG-22.5 world journeys: the Territory line of the tile card before any
// caravan exists, and Trade from the world view into the settlement's
// trade dialog (and its refusal at a hostile one).

// visitableSite is the nearest settlement a caravan is received at.
func visitableSite(j *journey, p *planet.Planet) game.WorldSiteView {
	j.t.Helper()
	var best game.WorldSiteView
	bd := 1 << 30
	home := j.g.WorldMap().Tile
	var sc planet.HopScratch
	for _, s := range j.g.WorldSites() {
		if !s.Visitable {
			continue
		}
		if d := p.HopsWith(&sc, home, s.Tile, 200); d >= 0 && d < bd {
			best, bd = s, d
		}
	}
	if bd == 1<<30 {
		j.t.Fatalf("no visitable settlement in %+v", j.g.WorldSites())
	}
	return best
}

// ownedBareTile is a tile a people's territory covers that holds no
// settlement, with that people's name, worked out from the planet the way
// the game charts it.
func ownedBareTile(j *journey, p *planet.Planet) (int, string) {
	j.t.Helper()
	live := map[int]bool{}
	for _, s := range j.g.WorldSites() {
		live[s.Tile] = true
	}
	var terr faction.Territory
	planet.BuildTerritory(p, &terr, func(t int) bool { return live[t] })
	home := j.g.WorldMap().Tile
	for t := 0; t < p.TileCount(); t++ {
		o := terr.Owner(t)
		if o < 0 || live[t] || t == home {
			continue
		}
		for _, f := range j.g.Factions() {
			if f.ID == o && f.Name != "" {
				return t, f.Name
			}
		}
	}
	j.t.Fatal("no bare tile inside a people's territory")
	return 0, ""
}

// worldText is every text run the world view draws this frame, joined.
func worldText(t *testing.T, j *journey) string {
	t.Helper()
	var parts []string
	for _, e := range drawnText(t, j.a, j.a.sw, j.a.sh) {
		parts = append(parts, e.Text)
	}
	return strings.Join(parts, "\n")
}

// TestJourneyWorldSettlementTrade: the tile card names the owner before any
// caravan exists; a caravan standing at a settlement trades from the world
// view (TradeSettlement, then the trade dialog opens and Cancel closes it);
// at a hostile settlement the Trade gizmo is disabled with the game's reason
// and sends nothing.
func TestJourneyWorldSettlementTrade(t *testing.T) {
	j, p := worldColony(t)

	// 1. Territory line, before any caravan.
	if len(j.g.WorldMap().Caravans) != 0 {
		t.Fatalf("caravans before the journey: %+v", j.g.WorldMap().Caravans)
	}
	worldOpen(j)
	tile, owner := ownedBareTile(j, p)
	j.a.world.Show("tile:" + strconv.Itoa(tile))
	j.update(worldFly)
	if got := j.a.world.Selected(); got != "tile:"+strconv.Itoa(tile) {
		t.Fatalf("selected %q, want tile %d", got, tile)
	}
	// The Territory row sits right under the title: no scrolling needed.
	txt := worldText(t, j)
	if !strings.Contains(txt, "Territory") || !strings.Contains(txt, owner) {
		t.Fatalf("tile %d card lacks the Territory line naming %q:\n%s", tile, owner, txt)
	}
	if strings.Contains(txt, "Unclaimed") {
		t.Fatalf("an owned tile shows Unclaimed:\n%s", txt)
	}

	// 2. A caravan at the nearest visitable settlement.
	s := visitableSite(j, p)
	c := sendCaravan(j, j.g.Snapshot()[0].ID, s.Tile)

	// 3. Select it and press Trade.
	key := "caravan:" + strconv.FormatUint(c.ID, 10)
	j.a.world.Show(key)
	j.update(worldFly)
	j.reset()
	if !j.clickID("world.act.trade") {
		w, _ := j.a.uiSt().Find("world.act.trade")
		t.Fatalf("Trade is disabled for the caravan at %s: %q", s.Name, w.Reason)
	}
	got, ok := j.lastCmd(command.TradeSettlement)
	if !ok || got.Target != c.ID {
		t.Fatalf("TradeSettlement %+v sent=%v, want Target %d", got, ok, c.ID)
	}

	// 4. The session opens: read model and dialog.
	j.g.Step(2)
	j.update(4)
	tv, ok := j.g.TradeView()
	if !ok {
		t.Fatal("no TradeView after TradeSettlement")
	}
	if tv.Trader != s.Name {
		t.Fatalf("Trader %q, want the settlement's name %q", tv.Trader, s.Name)
	}
	if j.a.uiModalTop() != dialogs.TradeModalID {
		t.Fatalf("modal %q, want the trade dialog", j.a.uiModalTop())
	}

	// 5. Cancel closes it.
	j.reset()
	if !j.clickID("modal." + dialogs.TradeModalID + ".cancel") {
		t.Fatal("Cancel is disabled")
	}
	if _, sent := j.lastCmd(command.CloseTrade); !sent {
		t.Fatal("Cancel sent no CloseTrade")
	}
	j.g.Step(2)
	j.update(2)
	if j.a.uiModalTop() == dialogs.TradeModalID {
		t.Fatal("the trade dialog is still open after Cancel")
	}
	if _, open := j.g.TradeView(); open {
		t.Fatal("the session is still open after CloseTrade")
	}
}

// TestJourneyWorldSettlementTradeHostile: a caravan at a hostile settlement
// cannot trade: Trade is disabled with the game's refusal and sends nothing.
func TestJourneyWorldSettlementTradeHostile(t *testing.T) {
	j, p := worldColony(t)
	s, _ := nearestSite(j, p, true)
	c := sendCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
	worldOpen(j)
	j.a.world.Show("caravan:" + strconv.FormatUint(c.ID, 10))
	j.update(worldFly)
	j.reset()

	w, found := j.a.uiSt().Find("world.act.trade")
	if !found || !w.Disabled {
		t.Fatalf("Trade at a hostile settlement: found %v, widget %+v, want it disabled", found, w)
	}
	if w.Reason != tradeview.ReasonNoTrade {
		t.Fatalf("reason %q, want the game's %q", w.Reason, tradeview.ReasonNoTrade)
	}
	if j.clickID("world.act.trade") {
		t.Fatal("a disabled Trade took the click")
	}
	if _, sent := j.lastCmd(command.TradeSettlement); sent {
		t.Fatal("the refused Trade sent TradeSettlement")
	}
	j.g.Step(2)
	if _, open := j.g.TradeView(); open {
		t.Fatal("a trade session opened at a hostile settlement")
	}
}
