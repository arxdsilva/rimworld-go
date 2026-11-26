// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package tradeview

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

// label is the core label stand-in the tests pass to Rows.
func label(k item.Kind) string { return "L-" + k.String() }

// TestRowsTable: a quartermaster's stock against the colony's goods. A
// line the trader has sells (WillBuy), a kind it wants it buys
// (WillSell), a crated building sells but is never bought and never
// counts the colony's own crates, an unknown kind and silver show no row,
// and the rows come by category then label with PriceOf's prices.
func TestRowsTable(t *testing.T) {
	t.Parallel()
	b := item.Book{}
	unknown := item.Kind(b.Len() + 5)
	st := trade.Stock{Kind: trade.Quartermaster, Silver: 900, Lines: []trade.Line{
		{Kind: item.Rifle, Qty: 3},
		{Kind: item.MinifiedBuilding, Qty: 1, Building: uint8(construction.Bed)},
		{Kind: unknown, Qty: 4},
	}}
	mine := map[item.Kind]int{item.Silver: 100, item.Steel: 10, item.Gold: 2, item.MinifiedBuilding: 5, item.Rifle: 1}
	terms := trade.Terms{Social: 8, Goodwill: 20}
	rows := Rows(b, mine, st, terms, label)

	bed := trade.Goods{Kind: item.MinifiedBuilding, Building: uint8(construction.Bed)}
	type want struct {
		goods             trade.Goods
		colony, trader    int
		willBuy, willSell bool
	}
	cases := map[string]want{
		"rifle":  {trade.Goods{Kind: item.Rifle}, 1, 3, true, true},
		"steel":  {trade.Goods{Kind: item.Steel}, 10, 0, false, true},
		"gold":   {trade.Goods{Kind: item.Gold}, 2, 0, false, false},
		"crated": {bed, 0, 1, true, false},
	}
	if len(rows) != len(cases) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(cases), rows)
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			i := slices.IndexFunc(rows, func(r Row) bool { return r.Kind == w.goods.Name(b) })
			if i < 0 {
				t.Fatalf("no row %q in %+v", w.goods.Name(b), rows)
			}
			r := rows[i]
			if r.ColonyQty != w.colony || r.TraderQty != w.trader || r.WillBuy != w.willBuy || r.WillSell != w.willSell {
				t.Fatalf("row %+v, want %+v", r, w)
			}
			if buy, sell := trade.PriceOf(b, w.goods, st.Kind, trade.Buy, terms), trade.PriceOf(b, w.goods, st.Kind, trade.Sell, terms); r.BuyC != buy || r.SellC != sell {
				t.Fatalf("prices %d/%d, want %d/%d", r.BuyC, r.SellC, buy, sell)
			}
			if r.Category != b.Category(w.goods.Kind) {
				t.Fatalf("category %q", r.Category)
			}
		})
	}
	if i := slices.IndexFunc(rows, func(r Row) bool { return r.Kind == bed.Name(b) }); rows[i].Label != "Bed (crated)" {
		t.Fatalf("crate label %q", rows[i].Label)
	}
	if i := slices.IndexFunc(rows, func(r Row) bool { return r.Kind == item.Steel.String() }); rows[i].Label != label(item.Steel) {
		t.Fatalf("core label %q", rows[i].Label)
	}
	if !slices.IsSortedFunc(rows, func(a, c Row) int {
		if ra, rc := item.CategoryRank(a.Category), item.CategoryRank(c.Category); ra != rc {
			return ra - rc
		}
		switch {
		case a.Label < c.Label:
			return -1
		case a.Label > c.Label:
			return 1
		}
		return 0
	}) {
		t.Fatalf("rows out of order: %+v", rows)
	}
}

// TestRowsMergesAndEmpty: two stock lines of one goods make one row with
// their sum, a colony with only silver against an empty stock has no
// rows, and the inputs are left alone.
func TestRowsMergesAndEmpty(t *testing.T) {
	t.Parallel()
	b := item.Book{}
	st := trade.Stock{Kind: trade.BulkHauler, Lines: []trade.Line{{Kind: item.Steel, Qty: 2}, {Kind: item.Steel, Qty: 5}}}
	rows := Rows(b, map[item.Kind]int{item.Steel: 1}, st, trade.Terms{}, label)
	if len(rows) != 1 || rows[0].TraderQty != 7 || rows[0].ColonyQty != 1 {
		t.Fatalf("rows %+v", rows)
	}
	if st.Lines[0].Qty != 2 || len(st.Lines) != 2 {
		t.Fatal("Rows changed the stock")
	}
	if rows := Rows(b, map[item.Kind]int{item.Silver: 50}, trade.Stock{}, trade.Terms{}, label); len(rows) != 0 {
		t.Fatalf("silver made rows: %+v", rows)
	}
}

// TestPriceModPct: Social and goodwill take off the markup, capped by the
// floor.
func TestPriceModPct(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		terms trade.Terms
		want  int
	}{
		{"none", trade.Terms{}, 0},
		{"social", trade.Terms{Social: 10}, 10 * trade.SocialStepCenti / 100},
		{"goodwill", trade.Terms{Goodwill: 100}, 100 * trade.BuyGoodwillMilli / 10 / 100},
		{"capped", trade.Terms{Social: 1000, Goodwill: 1000}, trade.BuyMarkupPct - trade.BuyFloorPct},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PriceModPct(c.terms); got != c.want {
				t.Fatalf("PriceModPct(%+v) = %d, want %d", c.terms, got, c.want)
			}
		})
	}
	if PriceModPct(trade.Terms{Social: 10}) == 0 {
		t.Fatal("social 10 earns nothing")
	}
}
