// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

func allKinds() []item.Kind {
	var ks []item.Kind
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		ks = append(ks, k)
	}
	return ks
}

func TestPriceBuyAboveSell(t *testing.T) {
	for _, k := range []TraderKind{BulkHauler, Quartermaster, CurioMerchant} {
		if b, s := Price(item.Book{}, item.Rifle, k, Buy, Terms{}), Price(item.Book{}, item.Rifle, k, Sell, Terms{}); b != 16800 || s >= b || s <= 0 {
			t.Fatalf("kind %d rifle buy %d sell %d", k, b, s)
		}
	}
}

func TestNoArbitrageAllKinds(t *testing.T) {
	for _, it := range allKinds() {
		if it.Value() <= 0 || it == item.Silver {
			continue
		}
		for k := TraderKind(0); k < NumKinds; k++ {
			for soc := 0; soc <= 20; soc++ {
				for gw := -100; gw <= 100; gw += 5 {
					tm := Terms{Social: soc, Goodwill: gw}
					b, s := Price(item.Book{}, it, k, Buy, tm), Price(item.Book{}, it, k, Sell, tm)
					if s > b {
						t.Fatalf("%v kind %d social %d gw %d: buy %d sell %d", it, k, soc, gw, b, s)
					}
				}
			}
		}
	}
}

func TestPriceSocialMonotonic(t *testing.T) {
	pb, ps := 1<<30, 0
	for soc := 0; soc <= 20; soc++ {
		b := Price(item.Book{}, item.Rifle, Quartermaster, Buy, Terms{Social: soc})
		s := Price(item.Book{}, item.Rifle, Quartermaster, Sell, Terms{Social: soc})
		if b > pb || s < ps {
			t.Fatalf("social %d: buy %d (prev %d) sell %d (prev %d)", soc, b, pb, s, ps)
		}
		pb, ps = b, s
	}
}

func TestPublishedCoreTradeSocialTable(t *testing.T) {
	for _, tc := range []struct{ social, buy, sell int }{
		{0, 140000, 60000},
		{1, 137900, 60900},
		{10, 119000, 69000},
		{20, 98000, 78000},
	} {
		terms := Terms{Social: tc.social}
		buy := PriceMarket(Goods{Kind: item.Lung}, 1000, BulkHauler, Buy, terms)
		sell := PriceMarket(Goods{Kind: item.Lung}, 1000, BulkHauler, Sell, terms)
		if buy != tc.buy || sell != tc.sell {
			t.Fatalf("social %d: buy/sell %d/%d, want %d/%d", tc.social, buy, sell, tc.buy, tc.sell)
		}
		terms.Goodwill = -100
		if b, s := PriceMarket(Goods{Kind: item.Lung}, 1000, Quartermaster, Buy, terms),
			PriceMarket(Goods{Kind: item.Lung}, 1000, Quartermaster, Sell, terms); b != buy || s != sell {
			t.Fatalf("goodwill changed price: %d/%d", b, s)
		}
	}
	terms := Terms{Social: 20, ImproveCenti: 200}
	if b, s := PriceMarket(Goods{Kind: item.Lung}, 1000, BulkHauler, Buy, terms),
		PriceMarket(Goods{Kind: item.Lung}, 1000, BulkHauler, Sell, terms); b != 95200 || s != 79200 {
		t.Fatalf("faction settlement price %d/%d", b, s)
	}
}

func TestSilverAtPar(t *testing.T) {
	for _, tm := range []Terms{{}, {Social: 20, Goodwill: 100}, {Goodwill: -100}, {Hostile: true}} {
		for _, s := range []Side{Buy, Sell} {
			if p := Price(item.Book{}, item.Silver, CurioMerchant, s, tm); p != 100 {
				t.Fatalf("silver %v = %d", tm, p)
			}
		}
	}
	if Price(item.Book{}, item.Parka, BulkHauler, Buy, Terms{Hostile: true}) != 0 {
		t.Fatal("hostile has no price")
	}
}

func TestTraderStockSpecialtyDoesNotInventSellBonus(t *testing.T) {
	if a, b := Price(item.Book{}, item.Steel, BulkHauler, Sell, Terms{}),
		Price(item.Book{}, item.Steel, CurioMerchant, Sell, Terms{}); a != b {
		t.Fatalf("inventory category changed base price: %d/%d", a, b)
	}
}

func TestGenerateStockDeterministic(t *testing.T) {
	for k := TraderKind(0); k < NumKinds; k++ {
		a := Generate(item.Book{}, k, faction.Industrial, randx.New(7))
		b := Generate(item.Book{}, k, faction.Industrial, randx.New(7))
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("kind %d differs", k)
		}
	}
}

func has(st Stock, it item.Kind) bool {
	for _, l := range st.Lines {
		if l.Kind == it {
			return true
		}
	}
	return false
}

func TestGenerateStockPerKind(t *testing.T) {
	for k := TraderKind(0); k < NumKinds; k++ {
		d := DefOf(k)
		for seed := uint64(0); seed < 40; seed++ {
			for _, tech := range []faction.Tech{faction.Neolithic, faction.Industrial} {
				st := Generate(item.Book{}, k, tech, randx.New(seed))
				if st.Silver < d.SilverMin || st.Silver > d.SilverMax || len(st.Lines) == 0 {
					t.Fatalf("kind %d silver %d lines %d", k, st.Silver, len(st.Lines))
				}
				for i, l := range st.Lines {
					if l.Qty <= 0 || l.Kind == item.Silver || i > 0 && st.Lines[i-1].Goods().Compare(l.Goods()) >= 0 { // AX-1: crates order by building
						t.Fatalf("bad line %+v", l)
					}
				}
				if tech == faction.Neolithic && (has(st, item.Rifle) || has(st, item.Components)) {
					t.Fatal("neolithic carries rifles or components")
				}
			}
		}
	}
	if !has(Generate(item.Book{}, Quartermaster, faction.Industrial, randx.New(1)), item.Rifle) {
		t.Fatal("industrial quartermaster sells rifles")
	}
	if !has(Generate(item.Book{}, CurioMerchant, faction.Spacer, randx.New(1)), item.Gold) {
		t.Fatal("curio sells gold")
	}
	if DefOf(NumKinds).Key != "" || Generate(item.Book{}, NumKinds, faction.Spacer, randx.New(1)).Silver != 0 {
		t.Fatal("unknown kind")
	}
	if !DefOf(CurioMerchant).Orbital || DefOf(BulkHauler).Orbital {
		t.Fatal("orbital flags")
	}
}

func TestWants(t *testing.T) {
	if !Wants(item.Book{}, BulkHauler, item.Wood) || Wants(item.Book{}, BulkHauler, item.Rifle) || Wants(item.Book{}, BulkHauler, item.Silver) ||
		!Wants(item.Book{}, Quartermaster, item.Rifle) || Wants(item.Book{}, Quartermaster, item.Beer) || !Wants(item.Book{}, CurioMerchant, item.Gold) ||
		Wants(item.Book{}, CurioMerchant, item.Wood) || Wants(item.Book{}, NumKinds, item.Wood) || Wants(item.Book{}, BulkHauler, item.CorpseHuman) {
		t.Fatal("wants table")
	}
}

func stock() Stock {
	return Stock{Kind: Quartermaster, Silver: 500, Lines: []Line{{Kind: item.Rifle, Qty: 2}, {Kind: item.Parka, Qty: 3}}}
}

func TestSettleBuyAndSell(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 400, item.Club: 5}
	r := Settle(item.Book{}, []Delta{{Kind: item.Parka, Qty: 1}, {Kind: item.Club, Qty: -2}}, col, stock(), Terms{}, false)
	if !r.OK() {
		t.Fatalf("rejected: %v", r.Reject)
	}
	want := 2*Price(item.Book{}, item.Club, Quartermaster, Sell, Terms{}) - Price(item.Book{}, item.Parka, Quartermaster, Buy, Terms{})
	if r.SilverCenti != want || len(r.Buy) != 1 || len(r.Sell) != 1 {
		t.Fatalf("%+v want %d", r, want)
	}
	// The whole signed net is rounded to nearest whole silver.
	if r.Silver > 0 || r.Silver != RoundedSilver(want) {
		t.Fatalf("silver %d", r.Silver)
	}
	// Colony receives.
	r = Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: -5}}, col, stock(), Terms{}, false)
	if !r.OK() || r.Silver != RoundedSilver(r.SilverCenti) || r.Silver <= 0 {
		t.Fatalf("%+v", r)
	}
}

func TestSettleRejectsUnaffordable(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 10, item.Club: 1000}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Rifle, Qty: 1}}, col, stock(), Terms{}, false); r.Reject != RejectUnaffordable {
		t.Fatalf("colony: %v", r.Reject)
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: -1000}}, col, stock(), Terms{}, false); r.Reject != RejectUnaffordable {
		t.Fatalf("trader: %v", r.Reject)
	}
}

func TestSettleRejectsOverStock(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 99999, item.Club: 1}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Rifle, Qty: 3}}, col, stock(), Terms{}, false); r.Reject != RejectOverStock {
		t.Fatalf("trader stock: %v", r.Reject)
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: -2}}, col, stock(), Terms{}, false); r.Reject != RejectOverStock {
		t.Fatalf("colony stock: %v", r.Reject)
	}
}

func TestSettleRejectsUnwanted(t *testing.T) {
	col := map[item.Kind]int{item.Wood: 50, item.Silver: 50}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Wood, Qty: -5}}, col, stock(), Terms{}, false); r.Reject != RejectUnwanted {
		t.Fatalf("%v", r.Reject)
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Silver, Qty: 5}}, col, stock(), Terms{}, false); r.Reject != RejectUnwanted {
		t.Fatalf("silver line: %v", r.Reject)
	}
}

func TestSettleHostileEmpty(t *testing.T) {
	col := map[item.Kind]int{item.Club: 5}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: -1}}, col, stock(), Terms{Hostile: true}, false); r.Reject != RejectHostile {
		t.Fatal("hostile")
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: 1}, {Kind: item.Club, Qty: -1}}, col, stock(), Terms{}, false); r.Reject != RejectEmpty {
		t.Fatal("empty")
	}
	for _, r := range []Reject{RejectNone, RejectEmpty, RejectHostile, RejectUnaffordable, RejectOverStock, RejectUnwanted} {
		if (r == RejectNone) != (r.String() == "") {
			t.Fatalf("string %d", r)
		}
	}
}

func TestSettleGiftNoSilver(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 300, item.Wood: 500}
	// Gifts need not be wanted and pay nothing back.
	r := Settle(item.Book{}, []Delta{{Kind: item.Silver, Qty: -200}, {Kind: item.Wood, Qty: -100}}, col, stock(), Terms{FactionType: faction.Settlers}, true)
	if !r.OK() || r.SilverCenti != 0 || r.Silver != 0 || r.Goodwill != faction.GiftGoodwill(faction.Faction{Type: faction.Settlers}, 260) || len(r.Sell) != 2 {
		t.Fatalf("%+v", r)
	}
	if GiftValue(item.Book{}, []Delta{{Kind: item.Silver, Qty: -200}, {Kind: item.Wood, Qty: -100}, {Kind: item.Rifle, Qty: 4}}, Terms{FactionType: faction.Settlers}) != 260 {
		t.Fatal("gift value uses Social-dependent selling value for outgoing goods only")
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Rifle, Qty: 1}}, col, stock(), Terms{}, true); r.Reject != RejectUnwanted {
		t.Fatal("cannot take from a gift")
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Silver, Qty: -400}}, col, stock(), Terms{}, true); r.Reject != RejectOverStock {
		t.Fatal("gift over colony silver")
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Bones, Qty: -1}}, map[item.Kind]int{item.Bones: 1}, stock(), Terms{}, true); r.Reject != RejectUnwanted {
		t.Fatal("worthless gift")
	}
}

func TestInspirationImprovesQuoteAndHonorsFloor(t *testing.T) {
	b := item.Core()
	normal := Terms{Social: 8}
	inspired := Terms{Social: 8, Inspired: true}
	buy0 := Price(b, item.Parka, Quartermaster, Buy, normal)
	buy1 := Price(b, item.Parka, Quartermaster, Buy, inspired)
	sell0 := Price(b, item.Parka, Quartermaster, Sell, normal)
	sell1 := Price(b, item.Parka, Quartermaster, Sell, inspired)
	if buy1 >= buy0 || sell1 <= sell0 {
		t.Fatalf("inspired quote unchanged: buy %d -> %d, sell %d -> %d", buy0, buy1, sell0, sell1)
	}
	if got := Price(b, item.Parka, Quartermaster, Buy, Terms{Social: 20, Inspired: true}); got <= 0 {
		t.Fatalf("high-skill inspired quote invalid: %d", got)
	}
}

func TestWholeOfferRoundTripNearestSilver(t *testing.T) {
	for _, tc := range []struct{ cents, silver int }{
		{-3360, -34}, {-3349, -33}, {-50, -1}, {-49, 0},
		{0, 0}, {49, 0}, {50, 1}, {3349, 33}, {3360, 34},
	} {
		if got := RoundedSilver(tc.cents); got != tc.silver {
			t.Fatalf("%d centi -> %d, want %d", tc.cents, got, tc.silver)
		}
	}
}

func TestInspiredCoreTradeTPIWithCap(t *testing.T) {
	ordinary := TradePriceImprovementCenti(Terms{Social: 8})
	inspired := TradePriceImprovementCenti(Terms{Social: 8, Inspired: true})
	if inspired-ordinary != 1800 || inspired != 3000 {
		t.Fatalf("inspired trade TPI %d vs %d", inspired, ordinary)
	}
	if got := TradePriceImprovementCenti(Terms{Social: 20, Inspired: true}); got != MaxTradeImprovementCenti {
		t.Fatalf("inspiration exceeded normal TPI cap: %d", got)
	}
	if got := TradePriceImprovementCenti(Terms{Social: 20, Inspired: true, ImproveCenti: 200}); got != MaxTradeImprovementCenti+200 {
		t.Fatalf("settlement supplement lost: %d", got)
	}
}

func TestCoreSellPriceMultipliersForWeaponsAndCrates(t *testing.T) {
	book := item.Book{}
	base := PriceMarket(Goods{Kind: item.Lung}, 1000, BulkHauler, Sell, Terms{})
	weapon := PriceMarket(Goods{Kind: item.Rifle}, 1000, Quartermaster, Sell, Terms{})
	furniture := PriceMarket(Goods{Kind: item.MinifiedBuilding, Building: uint8(construction.BigScreenTV)},
		1000, CurioMerchant, Sell, Terms{})
	art := PriceMarket(Goods{Kind: item.MinifiedBuilding, Building: uint8(construction.SmallSculpture)},
		1000, CurioMerchant, Sell, Terms{})
	if base != 60000 || weapon != 12000 || furniture != 42000 || art != 66000 {
		t.Fatalf("Core sell multipliers: base=%d weapon=%d furniture=%d art=%d", base, weapon, furniture, art)
	}
	if price := PriceOf(book, Goods{Kind: item.MinifiedBuilding, Building: uint8(construction.BigScreenTV)},
		CurioMerchant, Buy, Terms{}); price != 1485*BuyBasePct {
		t.Fatalf("actual crated television buy price %d", price)
	}
}

func TestTradePricesRejectUnrepresentablePluginValue(t *testing.T) {
	const maxInt = int(^uint(0) >> 1)
	goods := Goods{Kind: item.Lung}
	for _, side := range []Side{Buy, Sell} {
		if got := PriceMarket(goods, maxInt, Quartermaster, side, Terms{}); got != -1 {
			t.Fatalf("side %d accepted impossible market value: %d", side, got)
		}
	}
}
