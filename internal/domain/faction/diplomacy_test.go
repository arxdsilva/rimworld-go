// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import "testing"

func TestGiftGoodwillMeasuredCurve(t *testing.T) {
	f := Faction{Type: Settlers}
	if got := GiftGoodwill(f, 334); got != 8 {
		t.Fatalf("334 silver lump at neutral goodwill: %+d, want +8", got)
	}
	first := GiftGoodwill(f, 167)
	f.Goodwill += first
	second := GiftGoodwill(f, 167)
	if first+second != 7 {
		t.Fatalf("split 167 silver gifts: %+d and %+d, want total +7", first, second)
	}
	f = Faction{Type: Settlers, Goodwill: -100}
	if got := GiftGoodwill(f, 6770); got != 175 {
		t.Fatalf("6770 silver from -100 goodwill: %+d, want +175", got)
	}
	for _, pair := range []struct{ silver, want int }{{-5, 0}, {0, 0}, {39, 0}} {
		if got := GiftGoodwill(Faction{Type: Settlers}, pair.silver); got != pair.want {
			t.Fatalf("silver %d: %+d, want %+d", pair.silver, got, pair.want)
		}
	}
	if GiftGoodwill(Faction{Type: Freebooters}, 50000) != 0 {
		t.Fatal("fixed hostile faction accepted goodwill")
	}
}

func TestAidNeedsFriendly(t *testing.T) {
	f := Faction{Type: Settlers, Relation: Friendly, Goodwill: 50}
	if ok, why := CanRequestAid(f, Cooldowns{}, 0); !ok || why != "" {
		t.Fatal(why)
	}
	for _, f := range []Faction{
		{Type: Settlers, Relation: Neutral, Goodwill: 50}, {Type: Settlers, Relation: Hostile, Goodwill: 50},
		{Type: Settlers, Relation: Friendly, Goodwill: AidCost - 1}, {Type: Settlers, Relation: Friendly, Goodwill: 50, Defeated: true},
	} {
		if ok, why := CanRequestAid(f, Cooldowns{}, 0); ok || why == "" {
			t.Fatalf("%+v allowed", f)
		}
	}
}

func TestRequestCooldown(t *testing.T) {
	f := Faction{Type: Settlers, Relation: Friendly, Goodwill: 75}
	cd := Cooldowns{AidUntil: 100, CaravanUntil: 100}
	if ok, why := CanRequestCaravan(f, cd, 99); ok || why != ReasonCoolingOff {
		t.Fatal("cooling")
	}
	if ok, _ := CanRequestCaravan(f, cd, 100); !ok {
		t.Fatal("cooldown over")
	}
	if ok, _ := CanRequestCaravan(Faction{Type: Settlers, Goodwill: -1}, Cooldowns{}, 0); ok {
		t.Fatal("needs goodwill >= 0")
	}
	if ok, why := CanRequestCaravan(Faction{Type: Settlers, Relation: Hostile}, Cooldowns{}, 0); ok || why != ReasonHostile {
		t.Fatal("hostile")
	}
	if ok, why := CanRequestCaravan(Faction{Type: Settlers, Defeated: true}, Cooldowns{}, 0); ok || why != ReasonGone {
		t.Fatal("gone")
	}
	fr := Faction{Type: Settlers, Relation: Friendly, Goodwill: 50}
	if ok, why := CanRequestAid(fr, Cooldowns{AidUntil: 10}, 5); ok || why != ReasonCoolingOff {
		t.Fatal("aid cooling")
	}
}

func TestTradeGoodwillCarriesValueAndRespectsFixedRelations(t *testing.T) {
	f := Faction{Type: Settlers, Relation: Neutral, Goodwill: 10}
	if got := TradeGoodwill(&f, 300, "first deal"); got != 0 || f.TradeRemainder != 300 || f.Goodwill != 10 {
		t.Fatalf("first trade: %+v, gained %d", f, got)
	}
	if got := TradeGoodwill(&f, 700, "second deal"); got != 2 || f.Goodwill != 12 || f.TradeRemainder != 0 {
		t.Fatalf("second trade: %+v, gained %d", f, got)
	}
	f = Faction{Type: Freebooters, Relation: Hostile, Goodwill: -100}
	if got := TradeGoodwill(&f, 4000, "blocked"); got != 0 || f.TradeRemainder != 0 {
		t.Fatal("fixed faction gained goodwill from trade")
	}
}

func TestGiftCostAndActualCurveShareBoundary(t *testing.T) {
	for _, gw := range []int{-30, 0, 20, 65, 75, 95} {
		f := Faction{Type: Settlers, Goodwill: gw}
		cost := GiftSilverFor(f, gw+1)
		if cost < 1 || GiftGoodwill(f, cost) < 1 || GiftGoodwill(f, cost-1) != 0 {
			t.Fatalf("goodwill %d: cost %d does not match gift outcome", gw, cost)
		}
	}
}
