// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"math"
	"strconv"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestDecodeOfferQtyBounds(t *testing.T) {
	t.Parallel()
	b := item.Book{}
	k := strconv.Itoa(int(item.Club)) + ":"
	cases := []struct {
		name string
		q    int
		want error
	}{
		{"MinInt", math.MinInt, errOfferLine},
		{"MaxInt", math.MaxInt, errOfferLine},
		{"over", MaxOfferQty + 1, errOfferLine},
		{"under", -(MaxOfferQty + 1), errOfferLine},
		{"zero", 0, errOfferLine},
		{"minus one", -1, nil},
		{"one", 1, nil},
		{"max", MaxOfferQty, nil},
		{"-max", -MaxOfferQty, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeOffer(b, k+strconv.Itoa(c.q))
			if err != c.want {
				t.Fatalf("q=%d: err %v, want %v", c.q, err, c.want)
			}
		})
	}
	if _, err := DecodeOffer(b, k+"99999999999999999999999"); err != errOfferSyntax {
		t.Fatalf("Atoi range error: %v", err)
	}
}

func TestSettleBadQty(t *testing.T) {
	t.Parallel()
	col := map[item.Kind]int{item.Silver: 99999, item.Club: 1}
	for _, q := range []int{math.MinInt, math.MaxInt, MaxOfferQty + 1, -(MaxOfferQty + 1)} {
		if r := Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: q}}, col, stock(), Terms{}, false); r.Reject != RejectBadQty {
			t.Errorf("q=%d: %v", q, r.Reject)
		}
	}
	two := []Delta{{Kind: item.Club, Qty: MaxOfferQty}, {Kind: item.Club, Qty: MaxOfferQty}}
	if r := Settle(item.Book{}, two, col, stock(), Terms{}, false); r.Reject != RejectBadQty {
		t.Errorf("overflowing sum: %v", r.Reject)
	}
	neg := []Delta{{Kind: item.Club, Qty: -MaxOfferQty}, {Kind: item.Club, Qty: -MaxOfferQty}}
	if r := Settle(item.Book{}, neg, col, stock(), Terms{}, true); r.Reject != RejectBadQty {
		t.Errorf("overflowing gift sum: %v", r.Reject)
	}
	if RejectBadQty.String() != "impossible quantity" {
		t.Errorf("text %q", RejectBadQty.String())
	}
}

func TestSettleSellBoundedByStock(t *testing.T) {
	t.Parallel()
	col := map[item.Kind]int{item.Club: 1}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: -1}}, col, stock(), Terms{}, false); !r.OK() {
		t.Fatalf("-1 with stock 1: %v", r.Reject)
	}
	if r := Settle(item.Book{}, []Delta{{Kind: item.Club, Qty: -2}}, col, stock(), Terms{}, false); r.Reject != RejectOverStock {
		t.Fatalf("-2 with stock 1: %v", r.Reject)
	}
}

// TestGiftValueBoundedNoOverflow: the largest accepted gift stays far
// below MaxInt64 at the book's highest value.
func TestGiftValueBoundedNoOverflow(t *testing.T) {
	t.Parallel()
	b := item.Book{}
	best := 0
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		best = max(best, b.Value(k))
	}
	v := GiftValue(b, []Delta{{Kind: item.Club, Qty: -MaxOfferQty}}, Terms{})
	if v < 0 || v > MaxOfferQty*max(best, 1) || MaxOfferQty*int64(max(best, 1))*100 > math.MaxInt64/1000 {
		t.Fatalf("gift value %d (best %d)", v, best)
	}
}

func FuzzDecodeSettle(f *testing.F) {
	for _, s := range []string{"", "1:1", "1:-1,2:5", "1:1000000", "1:-1000001", "1:9223372036854775807", "1:-9223372036854775808"} {
		f.Add(s)
	}
	col := map[item.Kind]int{item.Silver: 1000, item.Club: 5}
	f.Fuzz(func(t *testing.T, s string) {
		b := item.Book{}
		offer, err := DecodeOffer(b, s)
		if err != nil {
			return
		}
		for _, gift := range []bool{false, true} {
			r := Settle(b, offer, col, stock(), Terms{}, gift)
			for _, l := range append(append([]Line{}, r.Buy...), r.Sell...) {
				if l.Qty > MaxOfferQty || l.Qty < -MaxOfferQty {
					t.Fatalf("line %+v beyond bound", l)
				}
			}
		}
	})
}
