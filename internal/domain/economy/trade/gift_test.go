// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

func TestGiftMethodPublicCoreRates(t *testing.T) {
	for _, tc := range []struct {
		k      item.Kind
		market int
		method GiftMethod
		social int
		centi  int
	}{
		{item.Lung, 1000, GiftVisitor, 0, 60000},
		{item.Lung, 1000, GiftVisitor, 1, 60900},
		{item.Lung, 1000, GiftVisitor, 20, 78000},
		{item.Lung, 1000, GiftPod, 0, 61200},
		{item.Lung, 1000, GiftPod, 20, 61200},
		{item.Lung, 1000, GiftSettlement, 0, 61200},
		{item.Lung, 1000, GiftSettlement, 20, 79200},
		{item.Silver, 1000, GiftVisitor, 20, 100000},
		{item.Silver, 1000, GiftPod, 0, 100000},
		{item.Rifle, 120, GiftVisitor, 0, 1440},
		{item.Rifle, 120, GiftPod, 0, 1469},
		{item.Lung, 0, GiftPod, 0, 0},
	} {
		if got := GiftUnitCenti(tc.k, tc.market, tc.method, tc.social); got != tc.centi {
			t.Fatalf("kind %s method %d social %d: centi %d, want %d", tc.k, tc.method, tc.social, got, tc.centi)
		}
	}
}

func TestGiftCratedFurnitureByMethod(t *testing.T) {
	b := item.Book{}
	g := Goods{Kind: item.MinifiedBuilding, Building: uint8(construction.Telescope)}
	if got := GiftGoodsCenti(b, g, GiftVisitor, 0); got != 14700 {
		t.Fatalf("settlement furniture gift centi %d, want 14700", got)
	}
	if got := GiftGoodsCenti(b, g, GiftPod, 0); got != 21420 {
		t.Fatalf("pod gift centi %d, want 21420", got)
	}
}

func TestGiftSettlementValueAffectsAuthoritativeGoodwill(t *testing.T) {
	b := item.Book{}
	offer := []Delta{{Kind: item.Lung, Qty: -1}}
	colony := map[item.Kind]int{item.Lung: 1}
	st := Stock{Kind: Quartermaster}
	visitor := Terms{Social: 20, FactionType: faction.Settlers}
	settlement := Terms{Social: 20, FactionType: faction.Settlers, ImproveCenti: 200}
	a, z := GiftValue(b, offer, visitor), GiftValue(b, offer, settlement)
	if a != 780 || z != 792 {
		t.Fatalf("selling value of lung visitor %d settlement %d", a, z)
	}
	first := Settle(b, offer, colony, st, visitor, true)
	second := Settle(b, offer, colony, st, settlement, true)
	if !first.OK() || !second.OK() ||
		first.Goodwill != faction.GiftGoodwill(faction.Faction{Type: faction.Settlers}, a) ||
		second.Goodwill != faction.GiftGoodwill(faction.Faction{Type: faction.Settlers}, z) ||
		second.Goodwill < first.Goodwill {
		t.Fatalf("recipient valuation diverged from settlement: visitor %+v settlement %+v", first, second)
	}
}

func TestGrossTradeValueCountsBothSides(t *testing.T) {
	r := Result{Buy: []Line{{Kind: item.Lung, Qty: 1}},
		Sell: []Line{{Kind: item.Wood, Qty: 20}}}
	if got := r.TradedValue(item.Book{}); got != 1000+20*item.Wood.Value() {
		t.Fatalf("gross traded value %d", got)
	}
}
