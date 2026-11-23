// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"maps"
	"reflect"
	"slices"
	"testing"
)

// TestSpreadBlowNone: with no split, an unknown split or no damage it is
// Spread exactly: the same draws and wounds, returning dmg.
func TestSpreadBlowNone(t *testing.T) {
	t.Parallel()
	start := []Body{
		{},
		{Wounds: []Wound{{Part: Heart, Dmg: 24, Kind: Cut}, {Part: ArmL, Dmg: 30, Kind: Cut}}},
		{Anatomy: Quadruped, Scale: 300},
		{Missing: ^uint32(0)},
	}
	for i, s0 := range start {
		for _, s := range []Split{SplitNone, Split(9), SplitCut, SplitBlunt} {
			for _, dmg := range []int{-2, 0, 15, 90} {
				if (s == SplitCut || s == SplitBlunt) && dmg > 0 && s0.Missing == 0 {
					continue // a real split: TestSpreadBlowVitalAndTotal
				}
				a := s0
				a.Wounds = slices.Clone(s0.Wounds)
				b := s0
				b.Wounds = slices.Clone(s0.Wounds)
				var na, nb []int
				Spread(&a, dmg, Cut, splitTap(splitPCG(uint64(dmg+10)), &na), 1)
				got := SpreadBlow(&b, dmg, Cut, s, splitTap(splitPCG(uint64(dmg+10)), &nb), 1)
				if got != dmg || !reflect.DeepEqual(a, b) || !slices.Equal(na, nb) {
					t.Fatalf("body %d split %d dmg %d: %d %v %v, Spread %v %v", i, s, dmg, got, b.Wounds, nb, a.Wounds, na)
				}
			}
		}
	}
}

// TestSpreadBlowVitalAndTotal: an unplaced split blow never takes a part
// whose loss would kill, raises Damage by exactly what it returns, and
// returns the split total plus any bonus.
func TestSpreadBlowVitalAndTotal(t *testing.T) {
	t.Parallel()
	plans := []Body{{}, {Anatomy: Quadruped}, {Anatomy: Quadruped, Scale: 300}}
	for seed := uint64(1); seed <= 200; seed++ {
		b := plans[seed%3]
		roll := splitPCG(seed)
		for blow := 0; !bodyDead(&b) && blow < 200; blow++ {
			s := SplitCut
			if blow%2 == 1 {
				s = SplitBlunt
			}
			var keep []Part
			for q := Part(0); q < b.numParts(); q++ {
				if !b.isMissing(q) && b.LossKills(q) {
					keep = append(keep, q)
				}
			}
			before := b.Damage()
			got := SpreadBlow(&b, 30, Cut, s, roll, uint64(blow))
			if d := b.Damage() - before; d != got {
				t.Fatalf("seed %d blow %d: Damage rose %d, returned %d", seed, blow, d, got)
			}
			if !bodyDead(&b) && (got < 30 || got > 30*178/100+30) {
				t.Fatalf("seed %d blow %d: total %d", seed, blow, got)
			}
			for _, q := range keep {
				if b.isMissing(q) {
					t.Fatalf("seed %d blow %d: %v lost", seed, blow, q)
				}
			}
		}
		if !bodyDead(&b) {
			t.Fatalf("seed %d: never died (%d)", seed, b.Damage())
		}
	}
	cases := []struct {
		name  string
		b     Body
		s     Split
		vals  []int
		total int
		want  map[Part]int
	}{
		// PickPart 416 is the heart; k = 2, LungL; Torso's bonus 21
		{"cut bonus", Body{}, SplitCut, []int{416, 30, 296}, 63, map[Part]int{Heart: 21, LungL: 21, Torso: 21}},
		// PickPart 120 is the torso; gate, heart, keep 80, extra 20
		{"blunt", Body{}, SplitBlunt, []int{120, 0, 0, 0, 0}, 36, map[Part]int{Torso: 24, Heart: 12}},
		// the arm is lost to its share (+5), the hand's share spills: 37
		// left, 24 on the brain (MaxHP - 1), the rest on the root
		{"spill", Body{Wounds: []Wound{{Part: ArmL, Dmg: 30, Kind: Cut}}}, SplitCut, []int{526, 30, 296}, 42, map[Part]int{Brain: 24, Torso: 13}},
		// the heart at MaxHP - 1 takes nothing: its 21 spills to the brain
		{"vital", Body{Wounds: []Wound{{Part: Heart, Dmg: 24, Kind: Cut}}}, SplitCut, []int{120, 30, 40}, 42, map[Part]int{Torso: 21, Heart: 24, Brain: 21}},
	}
	// a share that kills stops the blow: 90 + the torso's 21, so the
	// arm's share and the spill are not dealt; it returns the rise
	dying := Body{Wounds: []Wound{{Part: LegL, Dmg: 39}, {Part: LegR, Dmg: 39}, {Part: Brain, Dmg: 12}}}
	if got := SpreadBlow(&dying, 30, Cut, SplitCut, (&splitRec{vals: []int{120, 30, 150}}).roll, 1); got != 21 || splitDmgs(&dying)[ArmL] != 0 || dying.Damage() != 111 {
		t.Errorf("dying: returned %d, wounds %v", got, splitDmgs(&dying))
	}
	for _, c := range cases {
		b := c.b
		before := b.Damage()
		got := SpreadBlow(&b, 30, Cut, c.s, (&splitRec{vals: c.vals}).roll, 1)
		if got != c.total || b.Damage()-before != c.total || !maps.Equal(splitDmgs(&b), c.want) {
			t.Errorf("%s: returned %d, Damage +%d, wounds %v; want %d %v", c.name, got, b.Damage()-before, splitDmgs(&b), c.total, c.want)
		}
	}
}
