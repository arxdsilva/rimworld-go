// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

import (
	"rimworld-go/internal/domain/economy/quality"
	"testing"
)

func TestConditionRangeDefaultsAndExactBounds(t *testing.T) {
	st := Stack{Kind: Club, Stuff: Steel, Qty: 1, Quality: quality.Stored(quality.Good), Wear: 30}
	r := ConditionRange{}.WithHP(70, 70).WithQuality(int(quality.Good), int(quality.Good))
	if !r.Takes(st) || r.Takes(Stack{Kind: Club, Stuff: Steel, Qty: 1, Quality: st.Quality, Wear: 31}) || r.Takes(Stack{Kind: Club, Stuff: Steel, Qty: 1, Quality: quality.Stored(quality.Excellent), Wear: 30}) {
		t.Fatal("exact range not respected")
	}
	if !r.Takes(Stack{Kind: Wood, Qty: 5}) {
		t.Fatal("raw unconditioned good filtered")
	}
	if r.Takes(Stack{Kind: Club, Stuff: Steel, Qty: 1, Wear: 30}) {
		t.Fatal("legacy quality should be treated as Normal")
	}
	if r = r.WithHP(-10, 150).WithQuality(6, 0); r != (ConditionRange{}) {
		t.Fatalf("unrestricted range not zero: %+v", r)
	}
	if _, hi := r.WithHP(0, 0).HP(); hi != 0 {
		t.Fatal("zero-percent upper bound lost")
	}
}
