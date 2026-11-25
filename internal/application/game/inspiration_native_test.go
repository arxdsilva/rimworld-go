// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

func TestInspiredTimedStatsReachGame(t *testing.T) {
	g := New(Config{Seed: 4, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	baseMove, baseWork := g.statOf(id, stat.MoveSpeed), g.statOf(id, stat.WorkSpeed)
	g.mind.Inspire.Start(id, mind.GoFrenzy)
	if got := g.statOf(id, stat.MoveSpeed); got != baseMove*140/100 || g.paceStep(id, 14) >= 14 {
		t.Fatalf("inspired movement speed %d, base %d", got, baseMove)
	}
	g.mind.Inspire.Start(id, mind.WorkFrenzy)
	if got := g.statOf(id, stat.WorkSpeed); got != baseWork*180/100 {
		t.Fatalf("inspired work speed %d, base %d", got, baseWork)
	}
	units := 0
	for range 10 {
		units += g.workProgress(id, skill.Mining, false)
		g.clock.Advance(1)
	}
	if units != 18 {
		t.Fatalf("inspired worker advanced %d units, want 18", units)
	}
	g.clock.Advance(mind.InspireDuration)
	if got := g.statOf(id, stat.WorkSpeed); got != baseWork {
		t.Fatalf("expired work speed %d, base %d", got, baseWork)
	}
}

func TestShootFrenzyFollowsNativeBodyAccuracyCurve(t *testing.T) {
	g := New(Config{Seed: 41, Width: 12, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 2, "Shooter")
	h := combatSkills{g}
	for _, level := range []uint8{0, 8, 12} {
		setSkill(g, id, skill.Shooting, level)
		before, oldOffset := h.ShotAim(id)
		g.mind.Inspire.Start(id, mind.ShootFrenzy)
		during, newOffset := h.ShotAim(id)
		score := attack.ShootingScore(int(level), int(g.statOf(id, stat.ShootAcc)))
		if during != attack.ShooterPerTile(score) || during <= before || newOffset != oldOffset {
			t.Fatalf("level %d, normal %d/%d, inspired %d/%d", level, before, oldOffset, during, newOffset)
		}
		g.mind.Inspire.Use(id, mind.ShootFrenzy)
		after, afterOffset := h.ShotAim(id)
		if after != before || afterOffset != oldOffset {
			t.Fatalf("inspiration spent yet shot remains %d/%d, want %d/%d", after, afterOffset, before, oldOffset)
		}
	}
}

func TestInspiredSurgeryDoublesChanceAndSpendsOnOperation(t *testing.T) {
	g, patient, doctor := legless(t, 9)
	before, _ := g.surgeryChanceOf(doctor, patient, body.InstallImplant, item.HerbalMedicine)
	g.mind.Inspire.Start(doctor, mind.Surgery)
	during, detail := g.surgeryChanceOf(doctor, patient, body.InstallImplant, item.HerbalMedicine)
	if during != min(before*2, body.MaxSurgeryChance) || len(detail) == 0 {
		t.Fatalf("inspired surgery chance %d, normal %d", during, before)
	}
	addPegLeg(g, patient)
	if !g.mind.Inspire.Has(doctor, mind.Surgery) {
		t.Fatal("queuing an operation spent the inspiration")
	}
	operateUntil(t, g, patient, doctor)
	if g.mind.Inspire.Has(doctor, mind.Surgery) {
		t.Fatal("completed surgery retained the inspiration")
	}
}

func TestInspiredTradeQuotesAndSpendsOnlyAfterSuccessfulDeal(t *testing.T) {
	g, caravan := caravanGame(t)
	neg := openCaravanTrade(t, g, caravan)
	tradeYard(t, g, neg, 900, 10)
	g.mind.Inspire.Start(neg, mind.Trade)
	v, ok := g.TradeView()
	if !ok || len(v.Rows) == 0 {
		t.Fatal("trade quote missing")
	}
	terms := g.termsFor(neg, g.factionByID(caravan.Faction))
	if !terms.Inspired || v.PriceModPct != trade.TradePriceImprovementCenti(terms)/100 {
		t.Fatalf("inspired quote view disagrees with terms: %+v / %+v", terms, v)
	}
	for _, row := range v.Rows {
		k, valid := item.ParseKind(row.Kind)
		if valid && k != item.Silver &&
			row.BuyC != trade.Price(g.sk.content.Items, k, caravan.Kind, trade.Buy, terms) {
			t.Fatalf("inspired buy quote for %v disagrees with settlement", k)
		}
	}
	g.cmdProposeTrade(trade.EncodeOffer([]trade.Delta{{Kind: item.Silver, Qty: -200}}), true)
	g.cmdProposeTrade(trade.EncodeOffer([]trade.Delta{{Kind: item.Rifle, Qty: 9999}}), false)
	if !g.mind.Inspire.Has(neg, mind.Trade) {
		t.Fatal("gift or rejected offer spent trade inspiration")
	}
	buy := caravan.Stock.Lines[0]
	offer := []trade.Delta{{Kind: buy.Kind, Qty: 1}, {Kind: item.Components, Qty: -4}}
	if buy.Kind > item.Components {
		offer[0], offer[1] = offer[1], offer[0]
	}
	before := g.colonyGoods(false)
	silverBefore := totalOf(g, item.Silver)
	res := trade.Settle(g.sk.content.Items, offer, before.Counts(), caravan.Stock, terms, false)
	if !res.OK() {
		t.Fatalf("valid inspired trade was rejected: %v", res.Reject)
	}
	g.cmdProposeTrade(trade.EncodeOffer(offer), false)
	if g.mind.Inspire.Has(neg, mind.Trade) || totalOf(g, item.Silver) != silverBefore+res.Silver {
		t.Fatal("successful trade did not consume inspiration at the quoted price")
	}
}

func TestDeathClearsSavedInspiration(t *testing.T) {
	g := New(Config{Seed: 9, Width: 12, Height: 9})
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	g.mind.Inspire.Start(id, mind.WorkFrenzy)
	g.die(id, health.CauseInjury)
	if _, ok := g.mind.Inspire.Of(id); ok || len(g.mind.Export().Inspire) != 0 {
		t.Fatal("dead colonist retained temporary inspiration in save")
	}
}
