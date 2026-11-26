// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/faction"
)

func TestGiftPhysicalOfferCodec(t *testing.T) {
	gift := []Delta{
		{Kind: item.Club, Qty: 1, StackID: 15},
		{Kind: item.Club, Qty: -1, StackID: 11},
		{Kind: item.Club, Qty: -1, StackID: 12},
		{Kind: item.Silver, Qty: -40},
	}
	encoded := EncodeOffer(gift)
	decoded, err := DecodeOffer(item.Book{}, encoded)
	if err != nil || !reflect.DeepEqual(decoded, gift) {
		t.Fatalf("codec %q -> %+v, %v", encoded, decoded, err)
	}
	for _, bad := range []string{
		"7@0:-1", "7@bad:-1", "7@-1:-1", "7@:1",
		"7@42:0", "7@42:-1,7@42:-1", "7@42:-1,7@42:-2",
	} {
		if _, err := DecodeOffer(item.Book{}, bad); err == nil {
			t.Errorf("invalid selected offer accepted: %q", bad)
		}
	}
}

func TestPlanGiftActualItemAndNoDoubleRemoval(t *testing.T) {
	plain := item.Stack{Kind: item.Wood, Qty: 30}
	old := item.Stack{Kind: item.Club, Qty: 1, Stuff: item.Wood, Quality: quality.Stored(quality.Poor), Wear: 40}
	fine := item.Stack{Kind: item.Club, Qty: 1, Stuff: item.Gold, Quality: quality.Stored(quality.Excellent), Wear: 0}
	stock := []GiftStack{{ID: 12, Stack: fine}, {ID: 50, Stack: plain}, {ID: 11, Stack: old}}
	if !GiftIndividual(old) || !GiftIndividual(fine) || GiftIndividual(plain) {
		t.Fatal("incorrect gift item/group classification")
	}
	if GiftStackCenti(item.Book{}, old, GiftVisitor, 0) == GiftStackCenti(item.Book{}, fine, GiftVisitor, 0) {
		t.Fatal("item condition and material did not affect gift quote")
	}
	offer := []Delta{{Kind: item.Club, Qty: -1, StackID: 12}, {Kind: item.Wood, Qty: -20}}
	plan, ok := PlanGift(offer, stock)
	if !ok || len(plan) != 2 || plan[0].ID != 12 || plan[1].ID != 50 ||
		plan[0].Stack != fine || plan[1].Qty != 20 {
		t.Fatalf("selected gift plan %+v, ok=%v", plan, ok)
	}
	want := (GiftStackCenti(item.Book{}, fine, GiftVisitor, 0) + 20*GiftStackCenti(item.Book{}, plain, GiftVisitor, 0)) / 100
	if got, ok := GiftValueAllocations(item.Book{}, plan, Terms{}); !ok || got != want {
		t.Fatalf("actual gift value %d, want %d", got, want)
	}
	// Legacy kind-only gift takes the first eligible item by ID.
	legacy, ok := PlanGift([]Delta{{Kind: item.Club, Qty: -1}}, stock)
	if !ok || len(legacy) != 1 || legacy[0].ID != 11 {
		t.Fatalf("legacy kind gift %+v, ok=%t", legacy, ok)
	}
	both, ok := PlanGift([]Delta{{Kind: item.Club, Qty: -1}, {Kind: item.Club, Qty: -1, StackID: 12}}, stock)
	if !ok || len(both) != 2 || both[0].ID != 12 || both[1].ID != 11 {
		t.Fatalf("mixed gift over-consumed or changed requested IDs: %+v, ok=%t", both, ok)
	}
	for name, in := range map[string][]Delta{
		"stale ID":       {{Kind: item.Club, Qty: -1, StackID: 999}},
		"wrong kind":     {{Kind: item.Wood, Qty: -1, StackID: 11}},
		"wrong crate":    {{Kind: item.Club, Qty: -1, Building: 1, StackID: 11}},
		"over quantity":  {{Kind: item.Club, Qty: -2, StackID: 11}},
		"same ID twice":  {{Kind: item.Club, Qty: -1, StackID: 11}, {Kind: item.Club, Qty: -1, StackID: 11}},
		"double spend":   {{Kind: item.Club, Qty: -2}, {Kind: item.Club, Qty: -1, StackID: 12}},
		"bad positive":   {{Kind: item.Club, Qty: 1, StackID: 11}},
		"generic excess": {{Kind: item.Wood, Qty: -31}},
	} {
		if plan, ok := PlanGift(in, stock); ok {
			t.Errorf("%s accepted as %+v", name, plan)
		}
	}
	// The pure resolver does not mutate the physical stock.
	if stock[0].Stack != fine || stock[1].Stack != plain || stock[2].Stack != old {
		t.Fatal("valuation mutated physical inventory")
	}
}

func TestGiftWithStocksValidatesAndPricesSpecificPhysicalItem(t *testing.T) {
	good := item.Stack{Kind: item.Longsword, Qty: 1, Stuff: item.Gold, Quality: quality.Stored(quality.Excellent)}
	stock := []GiftStack{{ID: 44, Stack: good}}
	offer := []Delta{{Kind: item.Longsword, Qty: -1, StackID: 44}}
	terms := Terms{FactionType: faction.Settlers, Social: 8}
	res, plan, valid := SettlePhysical(item.Book{}, offer, map[item.Kind]int{item.Longsword: 1}, Stock{}, terms, true, stock)
	if !valid || !res.OK() || len(plan) != 1 || plan[0].ID != 44 {
		t.Fatalf("physical settlement refused: result=%+v plan=%+v ok=%t", res, plan, valid)
	}
	want := faction.GiftGoodwill(faction.Faction{Type: faction.Settlers}, GiftStackCenti(item.Book{}, good, GiftVisitor, 8)/100)
	if res.Goodwill != want {
		t.Fatalf("physical gift goodwill %d, want %d", res.Goodwill, want)
	}
	if _, _, ok := SettlePhysical(item.Book{}, offer, map[item.Kind]int{item.Longsword: 1}, Stock{}, terms, true, nil); ok {
		t.Fatal("stale physical ID was accepted")
	}
	paid, chosen, ok := SettlePhysical(item.Book{}, offer, map[item.Kind]int{item.Longsword: 1},
		Stock{Kind: Quartermaster, Silver: 2000}, terms, false, stock)
	if !ok || !paid.OK() || len(chosen) != 1 || len(paid.Sell) != 1 ||
		len(paid.Sell[0].Lots) != 1 || paid.Silver <= 0 {
		t.Fatalf("physical paid sale failed: result=%+v plan=%+v valid=%t", paid, chosen, ok)
	}
}
