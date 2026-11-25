// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"fmt"
	"strconv"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// TestJourneyWorldSettlementMarketTrade packs a physical item, travels to a
// friendly base and gifts it through the actual World inspector and modal.
// The command is observed at the App boundary and settled by the game.
func TestJourneyWorldSettlementMarketTrade(t *testing.T) {
	journeyWorldSettlementMarketTrade(t, false)
}

func TestJourneyWorldSettlementMarketTradeRejectsStaleDraft(t *testing.T) {
	journeyWorldSettlementMarketTrade(t, true)
}

func journeyWorldSettlementMarketTrade(t *testing.T, stale bool) {
	t.Helper()
	j, planet := worldColony(t)
	site, _ := nearestSite(j, planet, false)
	pawn := j.g.Snapshot()[0].ID
	var goods entity.EntityID
	qty := 1
	if stale {
		qty = 2
	}
	for _, it := range j.g.World().Items {
		kind, ok := item.ParseKind(it.Kind)
		if ok && (kind == item.Wood || kind == item.Steel || kind == item.Silver) &&
			it.Qty >= qty && !it.Carried && !it.Forbidden {
			goods = it.ID
			break
		}
	}
	if goods == 0 {
		t.Fatal("the wizard colony has no physical wood, steel or silver to pack")
	}
	j.g.Enqueue(command.Command{Kind: command.FormCaravan,
		Name: fmt.Sprintf("%d,%d:%d", pawn, goods, qty), Count: site.Tile})
	var cid uint64
	for range 700 {
		j.g.Step(60)
		for _, c := range j.g.WorldMap().Caravans {
			if c.State == caravan.Travelling.Label() {
				cid = c.ID
				break
			}
		}
		if cid != 0 {
			break
		}
	}
	if cid == 0 {
		t.Fatal("the supplied caravan never departed")
	}
	game.DevArrive(j.g, entity.EntityID(cid), site.Tile)
	j.g.Step(2)
	world := j.g.WorldMap()
	if len(world.Markets) != 1 || world.Markets[0].CaravanID != cid {
		t.Fatalf("caravan visit did not open its settlement market: %+v", world.Markets)
	}
	m := world.Markets[0]
	giftRow := -1
	for i, row := range m.GiftRows {
		if row.CargoQty >= qty {
			giftRow = i
			break
		}
	}
	if giftRow < 0 {
		t.Fatalf("the caravan did not carry the physical gift: %+v", m.Rows)
	}
	worldOpen(j)
	worldFind(j, site)
	worldPick(j, "world.obj.caravan."+strconv.FormatUint(cid, 10),
		"caravan:"+strconv.FormatUint(cid, 10))
	j.key("Digit2", 0)
	if j.a.renderer.speed != speedFast {
		t.Fatal("speed key did not set fast speed")
	}
	if !j.clickID("world.act.trade") || j.a.uiModalTop() != "world.market" {
		t.Fatalf("Trade did not open market modal: %q", j.a.uiModalTop())
	}
	if j.a.renderer.speed != speedPaused {
		t.Fatal("settlement modal did not pause")
	}
	if !j.clickID("world.market.gift") {
		t.Fatal("gift mode was unavailable")
	}
	if !j.clickID(fmt.Sprintf("world.market.sell.%d", giftRow)) {
		t.Fatal("the trade modal refused an available physical good")
	}
	if stale {
		// Queue a competing gift ahead of the UI command while the world
		// is paused. The UI's fresh read still sees the original inventory;
		// the owner must reject the second command after the first commits.
		r := m.GiftRows[giftRow]
		j.g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: m.Negotiator, Enabled: true, Name: trade.EncodeOffer([]trade.Delta{{Kind: r.ItemKind, Building: r.Building, Qty: -1, StackID: r.StackID}})})
		if !j.clickID("modal.world.market.accept") {
			t.Fatal("stale draft was not submitted")
		}
		j.update(1)
		current := j.g.WorldMap()
		if current.MarketReply.Reason != "The trade quote changed. Review the current offer." || current.MarketReply.Quote != m.Quote || j.a.uiModalTop() != "world.market" || j.a.renderer.speed != speedPaused {
			t.Fatalf("stale receipt/modal: %+v, %q, %v", current.MarketReply, j.a.uiModalTop(), j.a.renderer.speed)
		}
		n := 0
		for _, row := range current.Markets[0].GiftRows {
			if row.Kind == r.Kind {
				n = row.CargoQty
			}
		}
		if n != r.CargoQty-1 {
			t.Fatalf("stale transaction changed inventory: before=%d after=%d", r.CargoQty, n)
		}
		if !j.clickID("modal.world.market.refresh") {
			t.Fatal("refresh unavailable after refusal")
		}
		m = current.Markets[0]
		for i, row := range m.GiftRows {
			if row.Kind == r.Kind {
				giftRow = i
				break
			}
		}
		// Refresh retains the remaining valid selection; no reselect.
	}
	if !j.clickID("modal.world.market.accept") {
		t.Fatal("the market offer could not be submitted")
	}
	cmd, ok := j.lastCmd(command.WorldCaravanTrade)
	if !ok || cmd.Target != cid || cmd.Pawn != m.Negotiator || !cmd.Enabled || cmd.Quote != m.Quote {
		t.Fatalf("market command %+v (seen %v)", cmd, ok)
	}
	offer, err := trade.DecodeOffer(item.Book{}, cmd.Name)
	if err != nil || len(offer) != 1 || offer[0].Qty != -1 {
		t.Fatalf("wrong physical gift offer %+v, err %v", offer, err)
	}
	j.g.Step(1)
	j.draw(2)
	if j.a.uiModalTop() == "world.market" || j.a.renderer.speed != speedFast {
		t.Fatalf("accepted receipt did not restore speed: modal=%q speed=%v", j.a.uiModalTop(), j.a.renderer.speed)
	}
	after := j.g.WorldMap().Markets
	if len(after) != 1 {
		t.Fatalf("settlement market disappeared after gift: %+v", after)
	}
	for _, row := range after[0].GiftRows {
		if row.Kind == m.GiftRows[giftRow].Kind && row.CargoQty >= m.GiftRows[giftRow].CargoQty {
			t.Fatalf("gift did not remove a physical caravan stack: before %+v, after %+v",
				m.GiftRows[giftRow], row)
		}
	}
}
