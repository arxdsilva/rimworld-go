// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

func TestPaidSameKindExchangeAndExactStockAttributes(t *testing.T) {
	fine := item.Stack{Kind: item.Longsword, Qty: 1, Quality: quality.Stored(quality.Excellent), Stuff: item.Gold}
	poor := item.Stack{Kind: item.Longsword, Qty: 1, Quality: quality.Stored(quality.Poor), Stuff: item.Wood, Wear: 30}
	stock := Stock{Kind: Quartermaster, Silver: 10000, NextLotID: 4, Lines: []Line{
		{Kind: item.Longsword, Qty: 1, Lots: []Lot{{ID: 4, Stack: fine}}},
	}}
	offer := []Delta{
		{Kind: item.Longsword, Qty: 1, StackID: 4},
		{Kind: item.Longsword, Qty: -1, StackID: 10},
	}
	colony := map[item.Kind]int{item.Silver: 10000, item.Longsword: 1}
	res, plan, ok := SettlePhysical(item.Book{}, offer, colony, stock, Terms{}, false, []GiftStack{{ID: 10, Stack: poor}})
	if !ok || !res.OK() || len(plan) != 1 || plan[0].ID != 10 ||
		len(res.Buy) != 1 || len(res.Sell) != 1 ||
		res.Buy[0].Lots[0].ID != 4 || res.Sell[0].Lots[0].ID != 5 ||
		res.SilverCenti >= 0 {
		t.Fatalf("same-kind, different quality physical exchange: result=%+v plan=%+v valid=%t", res, plan, ok)
	}
	if res.TradedValue(item.Book{}) != item.StackValue(fine)+item.StackValue(poor) {
		t.Fatalf("traded value used kind price instead of actual quality: %+v", res)
	}
	copy := stock
	copy.Lines = CloneLines(stock.Lines)
	if !copy.ApplyTrade(res) {
		t.Fatal("same-kind exchange rejected after successful validation")
	}
	if len(copy.Lines) != 1 || len(copy.Lines[0].Lots) != 1 ||
		copy.Lines[0].Lots[0].ID != 5 || copy.Lines[0].Lots[0].Stack != poor ||
		copy.NextLotID != 5 || len(DropStacks(item.Book{}, res.Buy[0])) != 1 ||
		DropStacks(item.Book{}, res.Buy[0])[0] != fine {
		t.Fatalf("trader lost its new physical inventory or return item: %+v", copy)
	}
	// Stale buy IDs remain invalid even though total stock of the kind is present.
	if _, _, valid := SettlePhysical(item.Book{}, []Delta{{Kind: item.Longsword, Qty: 1, StackID: 4}},
		colony, copy, Terms{}, false, nil); valid {
		t.Fatal("stale purchase ID reused after an exchange")
	}
}

func TestPaidTraderLotsGenericAndSelectedPartialRemainConserved(t *testing.T) {
	high := item.Stack{Kind: item.Parka, Qty: 2, Quality: quality.Stored(quality.Good), Stuff: item.Wool}
	stock := Stock{Kind: Quartermaster, Silver: 10000, NextLotID: 2,
		Lines: []Line{{Kind: item.Parka, Qty: 3, Lots: []Lot{{ID: 2, Stack: high}}}}}
	offer := []Delta{{Kind: item.Parka, Qty: 1, StackID: 2}, {Kind: item.Parka, Qty: 2}}
	res, _, valid := SettlePhysical(item.Book{}, offer, map[item.Kind]int{item.Silver: 20000}, stock, Terms{}, false, nil)
	if !valid || !res.OK() || len(res.Buy) != 1 || res.Buy[0].Qty != 3 ||
		res.Buy[0].OrdinaryQty() != 1 || len(res.Buy[0].Lots) != 1 ||
		res.Buy[0].Lots[0].Stack.Qty != 2 {
		t.Fatalf("mixed generic and named partial buy: %+v valid=%t", res, valid)
	}
	if !stock.ApplyTrade(res) || len(stock.Lines) != 0 {
		t.Fatalf("physical and fungible quantities diverged after mixed purchase: %+v", stock)
	}
	if got := DropStacks(item.Book{}, res.Buy[0]); !reflect.DeepEqual(got, []item.Stack{
		{Kind: item.Parka, Qty: 1, Quality: high.Quality, Stuff: high.Stuff},
		{Kind: item.Parka, Qty: 1, Quality: high.Quality, Stuff: high.Stuff},
		{Kind: item.Parka, Qty: 1},
	}) {
		t.Fatalf("mixed stock restored wrong attributes: %+v", got)
	}
}
