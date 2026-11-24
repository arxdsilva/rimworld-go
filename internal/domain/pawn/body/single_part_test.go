// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/combat/damage"
)

// The wiki's Damage Types page has no Bite or Arrow section, so neither
// form splits: a bite or arrow wounds the one part it strikes (original
// tuning, as the existing single-part behaviour).
var singlePartForms = []damage.Form{damage.FormBite, damage.FormArrow}

func TestSinglePartSplitOf(t *testing.T) {
	t.Parallel()
	for _, f := range singlePartForms {
		if SplitOf(f) != SplitNone {
			t.Fatalf("form %v splits", f)
		}
	}
}

// TestSinglePartStrikeBlow: StrikeBlow with the form's split is Strike, in
// state and in draws.
func TestSinglePartStrikeBlow(t *testing.T) {
	t.Parallel()
	for _, f := range singlePartForms {
		lo, hi := f.Overkill()
		for _, p := range []Part{Torso, Heart, Spine, ArmL} {
			for seed := uint64(1); seed <= 20; seed++ {
				var a, c Body
				var na, nc []int
				ga := StrikeBlow(&a, p, 40, WoundOf(f), SplitOf(f), lo, hi, splitTap(splitPCG(seed), &na), 1)
				gc := Strike(&c, p, 40, WoundOf(f), lo, hi, splitTap(splitPCG(seed), &nc), 1)
				if ga != gc || !reflect.DeepEqual(a, c) || !reflect.DeepEqual(na, nc) {
					t.Fatalf("form %v part %v seed %d differs", f, p, seed)
				}
			}
		}
	}
}

// TestSinglePartSpreadBlow: SpreadBlow with the form's split is Spread.
func TestSinglePartSpreadBlow(t *testing.T) {
	t.Parallel()
	for _, f := range singlePartForms {
		for seed := uint64(1); seed <= 50; seed++ {
			var a, c Body
			SpreadBlow(&a, 40, WoundOf(f), SplitOf(f), splitPCG(seed), 1)
			Spread(&c, 40, WoundOf(f), splitPCG(seed), 1)
			if !reflect.DeepEqual(a, c) {
				t.Fatalf("form %v seed %d differs", f, seed)
			}
		}
	}
}

// TestSinglePartArrowHeart: an arrow into Heart wounds Heart only.
func TestSinglePartArrowHeart(t *testing.T) {
	t.Parallel()
	b := Body{}
	StrikeBlow(&b, Heart, 10, WoundOf(damage.FormArrow), SplitOf(damage.FormArrow), 0, 0, splitNever, 1)
	if got := splitWounded(&b); got != bit(Heart) {
		t.Fatalf("wounded mask %b", got)
	}
}
