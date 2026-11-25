// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func checkCaravanManifest(t *testing.T, c *traderCaravan) {
	t.Helper()
	actual, expected := map[trade.Goods]int{}, map[trade.Goods]int{}
	silver := 0
	for _, carrier := range c.Cargo {
		silver += carrier.Silver
		for _, l := range carrier.Lines {
			if l.Qty <= 0 {
				t.Fatalf("empty carrier goods: %+v", l)
			}
			actual[l.Goods()] += l.Qty
		}
	}
	for _, l := range c.Stock.Lines {
		expected[l.Goods()] += l.Qty
	}
	if !reflect.DeepEqual(actual, expected) || silver != c.Stock.Silver {
		t.Fatalf("carriers %v, stock %v, silver %d/%d", actual, expected, silver, c.Stock.Silver)
	}
}

func TestCaravanCargoOwnershipPersistsThroughPaidSaleAndGuardLoss(t *testing.T) {
	g, c := caravanGame(t)
	checkCaravanManifest(t, c)
	neg := openCaravanTrade(t, g, c)
	tradeYard(t, g, neg, 12000, 10)
	var guardID entity.EntityID
	var line trade.Line
	for _, owner := range c.Cargo {
		if owner.Owner != c.Trader && len(owner.Lines) != 0 {
			guardID, line = owner.Owner, owner.Lines[0]
			break
		}
	}
	if guardID == 0 {
		t.Fatal("the trader's guards hold none of the caravan's stock")
	}
	res := trade.Settle(g.sk.content.Items, []trade.Delta{{Kind: line.Kind, Building: line.Building, Qty: 1}},
		g.colonyGoods(false).Counts(), c.Stock, g.termsFor(neg, g.factionByID(c.Faction)), false)
	if !res.OK() {
		t.Fatalf("guard-held goods unavailable to buy: %s", res.Reject)
	}
	g.Enqueue(command.Command{Kind: command.ProposeTrade,
		Name: trade.EncodeOffer([]trade.Delta{{Kind: line.Kind, Building: line.Building, Qty: 1}})})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != "" {
		t.Fatalf("trade refused: %s", v.Reason)
	}
	checkCaravanManifest(t, c)
	for _, owner := range c.Cargo {
		if owner.Owner != guardID {
			continue
		}
		found := false
		for _, l := range owner.Lines {
			if l.Goods() == line.Goods() {
				found = true
				if l.Qty != line.Qty-1 {
					t.Fatalf("guard-held goods %d after buying one of %d", l.Qty, line.Qty)
				}
			}
		}
		if !found && line.Qty != 1 {
			t.Fatal("unrelated guard inventory was reassigned")
		}
	}
	remainingGuard := []visit.Cargo{}
	for _, carrier := range c.Cargo {
		if carrier.Owner == guardID {
			remainingGuard = append(remainingGuard, carrier)
		}
	}
	if len(remainingGuard) != 1 {
		t.Fatal("guard cargo vanished")
	}
	beforeDeath := cloneCaravan(c)
	g.die(guardID, health.CauseInjury)
	g.Step(1)
	checkCaravanManifest(t, c)
	for _, carrier := range c.Cargo {
		if carrier.Owner == guardID {
			t.Fatal("dead guard is still assigned merchandise")
		}
	}
	if c.Stock.Silver != beforeDeath.Stock.Silver-remainingGuard[0].Silver || g.trade.caravan != c {
		t.Fatalf("guard loss: active=%p want=%p silver=%d expected=%d", g.trade.caravan, c, c.Stock.Silver, beforeDeath.Stock.Silver-remainingGuard[0].Silver)
	}
}

func TestCaravanTraderLossAfterPartialSaleDropsOnce(t *testing.T) {
	for _, how := range []string{"killed", "captured"} {
		t.Run(how, func(t *testing.T) {
			g, c := caravanGame(t)
			neg := openCaravanTrade(t, g, c)
			tradeYard(t, g, neg, 12000, 10)
			purchase := c.Stock.Lines[0]
			g.Enqueue(command.Command{Kind: command.ProposeTrade,
				Name: trade.EncodeOffer([]trade.Delta{{Kind: purchase.Kind, Building: purchase.Building, Qty: 1}})})
			g.ApplyPending()
			if v, _ := g.TradeView(); v.Reason != "" {
				t.Fatalf("sale refused: %q", v.Reason)
			}
			checkCaravanManifest(t, c)
			var traderOwned visit.Cargo
			for _, carrier := range c.Cargo {
				if carrier.Owner == c.Trader {
					traderOwned = carrier
				}
			}
			before := totalOf(g, item.Silver)
			goodwill := g.factionByID(c.Faction).Goodwill
			if how == "killed" {
				g.die(c.Trader, health.CauseInjury)
			} else {
				g.makePrisoner(c.Trader, g.pawnIDs[0])
			}
			g.Step(1)
			if g.trade.caravan != nil || g.factionByID(c.Faction).Goodwill != goodwill-traderLostGoodwill {
				t.Fatal("trader removal and faction consequence diverged")
			}
			checkCaravanManifest(t, c)
			if c.Stock.Silver != 0 || totalOf(g, item.Silver) < before+traderOwned.Silver {
				t.Fatalf("trader silver lost or duplicated: stock %d, ground %d", c.Stock.Silver, totalOf(g, item.Silver))
			}
			g.traderVisitorGone(c.Trader)
			checkCaravanManifest(t, c)
			if g.factionByID(c.Faction).Goodwill != goodwill-traderLostGoodwill {
				t.Fatal("duplicate loss charged faction goodwill twice")
			}
		})
	}
}

func TestCaravanCargoSaveRestoresAndMigratesOldLedger(t *testing.T) {
	g, c := caravanGame(t)
	checkCaravanManifest(t, c)
	var buf bytes.Buffer
	if err := g.Save(&buf, "carriers"); err != nil {
		t.Fatal(err)
	}
	recovered, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.trade.caravan == nil || !reflect.DeepEqual(c.Cargo, recovered.trade.caravan.Cargo) {
		t.Fatal("saved owners were lost or reassigned")
	}
	checkCaravanManifest(t, recovered.trade.caravan)

	old := g.exportTrade()
	old.Version = 1
	old.Caravan.Cargo = nil
	g.importTrade(old)
	if g.trade.caravan == nil || len(g.trade.caravan.Cargo) == 0 {
		t.Fatal("old save did not reconstruct its original caravan stock")
	}
	checkCaravanManifest(t, g.trade.caravan)
}

func TestRetiredCaravanSettlesSurvivingGuardLoss(t *testing.T) {
	g, c := caravanGame(t)
	goodwill := g.factionByID(c.Faction).Goodwill
	g.die(c.Trader, health.CauseInjury)
	g.Step(1)
	if g.trade.caravan != nil || g.trade.retired != c {
		t.Fatal("surviving caravan carriers lost their manifest after trader death")
	}
	checkCaravanManifest(t, c)
	var guard visit.Cargo
	for _, owner := range c.Cargo {
		if owner.Owner != c.Trader && len(owner.Lines) > 0 {
			guard = owner
			break
		}
	}
	if guard.Owner == 0 {
		t.Fatal("retired caravan lacks a carrying guard")
	}
	before := cloneStock(c.Stock)
	g.die(guard.Owner, health.CauseInjury)
	g.Step(1)
	checkCaravanManifest(t, c)
	for _, line := range guard.Lines {
		if got := stockQty(c.Stock, line.Kind); got != stockQty(before, line.Kind)-line.Qty {
			t.Fatalf("guard died but %s remains in sale ledger: %d", line.Kind, got)
		}
	}
	if g.factionByID(c.Faction).Goodwill != goodwill-traderLostGoodwill {
		t.Fatal("guard loss charged a second trader goodwill penalty")
	}
	g.traderVisitorGone(guard.Owner)
	checkCaravanManifest(t, c)
	for len(c.Members) > 0 {
		g.traderVisitorGone(c.Members[0])
	}
	// A living pack carrier is not accounted for by human member removal.
	pack := caravanPack(g, c)
	if pack != 0 && g.animals.Has(pack) {
		if caravanByGroup(&g.trade, c.GroupID) != c {
			t.Fatal("retired pack lost its manifest")
		}
		checkCaravanManifest(t, c)
		g.removeAnimal(pack)
	}
	if len(c.Cargo) != 0 {
		t.Fatalf("remaining unaccounted cargo: %+v", c.Cargo)
	}
	(visitGround{g: g}).GroupClosed(c.GroupID)
	if g.trade.retired != nil {
		t.Fatal("retired caravan remained after every carrier and group was accounted for")
	}
}
