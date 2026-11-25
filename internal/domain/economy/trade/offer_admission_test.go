// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"math"
	"rimworld-go/internal/domain/economy/item"
	"strings"
	"testing"
)

func TestNativeOfferRejectsUnboundedCommandBeforeParsing(t *testing.T) {
	book := item.Book{}
	for _, text := range []string{
		strings.Repeat("1:1,", 2048) + "1:1",
		strings.Repeat("1:1,", 17000),
	} {
		if v, err := DecodeOffer(book, text); err != errOfferSyntax || v != nil {
			t.Fatalf("unbounded command len %d: offer=%v err=%v", len(text), v, err)
		}
	}
}

func largeValueBook(value int) item.Book {
	base := item.Core()
	keys := base.Keys()
	defs := make([]item.Def, len(keys))
	for i := range defs {
		defs[i] = item.CoreDef(item.Kind(i))
	}
	defs[item.Club].Value = value
	return item.NewBook(defs, keys)
}

func TestNativeTradeRejectsQuoteOverflowBeforeGoodsMove(t *testing.T) {
	b := largeValueBook(math.MaxInt / 2_000_000)
	st := Stock{Kind: Quartermaster, Silver: math.MaxInt, Lines: []Line{{Kind: item.Club, Qty: MaxOfferQty}}}
	colony := map[item.Kind]int{item.Silver: math.MaxInt, item.Club: MaxOfferQty}
	for _, q := range []int{MaxOfferQty, -MaxOfferQty} {
		r := Settle(b, []Delta{{Kind: item.Club, Qty: q}}, colony, st, Terms{}, false)
		if r.OK() {
			t.Fatalf("overflowed signed centi quote settled: %+v", r)
		}
	}
	impossible := largeValueBook(math.MaxInt / 4)
	if result := Settle(impossible, []Delta{{Kind: item.Club, Qty: 1}}, colony, st, Terms{}, false); result.OK() {
		t.Fatalf("unrepresentable single-unit plugin price settled: %+v", result)
	}
	b = largeValueBook(math.MaxInt / MaxOfferQty * 2)
	gift := []Delta{{Kind: item.Club, Qty: -MaxOfferQty}}
	if got := GiftValue(b, gift, Terms{}); got != 0 {
		t.Fatalf("direct enormous gift value wrapped to %d", got)
	}
	if r := Settle(b, gift, colony, st, Terms{}, true); r.OK() {
		t.Fatalf("overflowing gift settled: %+v", r)
	}
}
