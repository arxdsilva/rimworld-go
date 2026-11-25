// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package visit

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/entity"
)

func verifyManifest(t *testing.T, stock trade.Stock, cargo []Cargo) {
	t.Helper()
	actual := make(map[trade.Goods]int)
	silver := 0
	for _, hold := range cargo {
		silver += hold.Silver
		for _, l := range hold.Lines {
			actual[l.Goods()] += l.Qty
			if l.Qty <= 0 {
				t.Fatalf("empty cargo line: %+v", l)
			}
		}
	}
	expected := make(map[trade.Goods]int)
	for _, l := range stock.Lines {
		expected[l.Goods()] += l.Qty
	}
	if !reflect.DeepEqual(actual, expected) || silver != stock.Silver {
		t.Fatalf("stock %+v, cargo %+v: quantities %v vs %v, silver %d vs %d",
			stock, cargo, actual, expected, silver, stock.Silver)
	}
}

func TestSettleCargoPreservesOwnersWithDuplicateKindsAndSilver(t *testing.T) {
	stock := trade.Stock{Silver: 100}
	stock.Add(item.Wood, 10)
	stock.Add(item.Steel, 10)
	stock.Add(item.Components, 8)
	wood, steel, comps := trade.Line{Kind: item.Wood}, trade.Line{Kind: item.Steel}, trade.Line{Kind: item.Components}
	cargo := []Cargo{
		{Owner: 10, Lines: []trade.Line{{Kind: item.Wood, Qty: 3}, {Kind: item.Wood, Qty: 2},
			{Kind: item.Steel, Qty: 4}}, Silver: 40},
		{Owner: 20, Lines: []trade.Line{{Kind: item.Wood, Qty: 5}, {Kind: item.Steel, Qty: 2},
			{Kind: item.Components, Qty: 7}}, Silver: 60},
		{Owner: 30, Lines: []trade.Line{{Kind: item.Steel, Qty: 4}, {Kind: item.Components, Qty: 1}}},
	}
	for i := range cargo {
		slices.SortFunc(cargo[i].Lines, func(a, b trade.Line) int { return a.Goods().Compare(b.Goods()) })
	}
	verifyManifest(t, stock, cargo)
	first := trade.Result{Buy: []trade.Line{{Kind: wood.Kind, Qty: 6}},
		Sell: []trade.Line{{Kind: item.Corn, Qty: 3}}, Silver: 60}
	if !SettleCargo(cargo, 10, first) {
		t.Fatal("first partial trade refused")
	}
	stock.AddGoods(wood.Goods(), -6)
	stock.Add(item.Corn, 3)
	stock.Silver -= 60
	verifyManifest(t, stock, cargo)
	if cargo[1].Lines[0].Kind != item.Wood || cargo[1].Lines[0].Qty != 4 ||
		cargo[2].Lines[0].Kind != item.Steel || cargo[2].Lines[0].Qty != 4 ||
		cargo[0].Silver != 0 || cargo[1].Silver != 40 {
		t.Fatalf("first sale moved unrelated stock: %+v", cargo)
	}

	second := trade.Result{Buy: []trade.Line{{Kind: steel.Kind, Qty: 5}, {Kind: comps.Kind, Qty: 3}},
		Sell: []trade.Line{{Kind: item.Corn, Qty: 2}}, Silver: -15}
	if !SettleCargo(cargo, 10, second) {
		t.Fatal("second partial trade refused")
	}
	stock.AddGoods(steel.Goods(), -5)
	stock.AddGoods(comps.Goods(), -3)
	stock.Add(item.Corn, 2)
	stock.Silver += 15
	verifyManifest(t, stock, cargo)
	if cargo[2].Lines[0].Kind != item.Steel || cargo[2].Lines[0].Qty != 4 {
		t.Fatalf("other carrier's steel moved: %+v", cargo[2])
	}
	if !SettleCargo(cargo, 10, trade.Result{Sell: []trade.Line{{Kind: item.Silver, Qty: 9}}}) {
		t.Fatal("silver gift refused")
	}
	stock.Silver += 9
	verifyManifest(t, stock, cargo)

	expected := slices.Clone(cargo[1].Lines)
	silverBefore := cargo[1].Silver
	var dropped []trade.Line
	var silver int
	cargo, dropped, silver = LoseCargo(&stock, cargo, 20)
	if !reflect.DeepEqual(dropped, expected) || silver != silverBefore {
		t.Fatalf("pack loss dropped %+v silver %d, expected %+v silver %d", dropped, silver, expected, silverBefore)
	}
	verifyManifest(t, stock, cargo)
	cargo, dropped, silver = LoseCargo(&stock, cargo, 20)
	if len(dropped) != 0 || silver != 0 {
		t.Fatal("duplicate pack death dropped its stock again")
	}
	verifyManifest(t, stock, cargo)
}

func TestSettleCargoKeepsCratedGoodsByBuilding(t *testing.T) {
	st := trade.Stock{Silver: 20}
	a, b := trade.Goods{Kind: item.MinifiedBuilding, Building: 1}, trade.Goods{Kind: item.MinifiedBuilding, Building: 2}
	st.AddGoods(a, 2)
	st.AddGoods(b, 3)
	cargo := []Cargo{
		{Owner: 10, Lines: []trade.Line{{Kind: item.MinifiedBuilding, Building: 1, Qty: 2}}, Silver: 20},
		{Owner: 20, Lines: []trade.Line{{Kind: item.MinifiedBuilding, Building: 2, Qty: 3}}},
	}
	if !SettleCargo(cargo, 10, trade.Result{Buy: []trade.Line{{Kind: item.MinifiedBuilding, Building: 1, Qty: 2}}}) {
		t.Fatal("crate purchase refused")
	}
	st.AddGoods(a, -2)
	verifyManifest(t, st, cargo)
	if len(cargo[1].Lines) != 1 || cargo[1].Lines[0].Goods() != b || cargo[1].Lines[0].Qty != 3 {
		t.Fatalf("other crate was reassigned: %+v", cargo)
	}
}

func TestSettleCargoRefusesMissingGoodsWithoutMutation(t *testing.T) {
	cargo := []Cargo{{Owner: 10, Lines: []trade.Line{{Kind: item.Wood, Qty: 2}}, Silver: 3},
		{Owner: 20, Lines: []trade.Line{{Kind: item.Steel, Qty: 8}}}}
	before := slices.Clone(cargo)
	for i := range before {
		before[i].Lines = slices.Clone(cargo[i].Lines)
	}
	cases := []trade.Result{
		{Buy: []trade.Line{{Kind: item.Wood, Qty: 3}}, Sell: []trade.Line{{Kind: item.Corn, Qty: 5}}},
		{Silver: 4},
	}
	for _, result := range cases {
		if SettleCargo(cargo, entity.EntityID(10), result) || !reflect.DeepEqual(cargo, before) {
			t.Fatalf("invalid trade mutated cargo: %+v", cargo)
		}
	}
}
