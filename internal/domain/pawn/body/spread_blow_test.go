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

// withinBodies are animal-plan bodies for the SpreadBlowWithin tests, by
// seed: the animal anatomies at three scales, some with a lost part (one
// whose loss does not kill), some wounded up to close to killAt.
func withinBodies(seed uint64) Body {
	b := Body{Anatomy: Anatomy(1 + seed%uint64(NumAnatomies-1)), Scale: []int32{300, 700, 1700}[seed%3]}
	if seed%2 == 0 {
		if p := Part(seed/2) % b.numParts(); p != b.Root() && !b.LossKills(p) {
			b.Missing |= 1 << p
		}
	}
	if pct := int(seed / 5 % 5); pct > 0 {
		Spread(&b, killAt(&b)*(10+pct*21)/120, Cut, splitPCG(seed+500), 0)
	}
	return b
}

// cloneBody is a copy of b sharing no slice with it.
func cloneBody(b Body) Body {
	b.Wounds = slices.Clone(b.Wounds)
	b.Conds = slices.Clone(b.Conds)
	b.Fitted = slices.Clone(b.Fitted)
	return b
}

// TestSpreadBlowWithinMatchesBodyRule: with room = killAt - lossDamage,
// SpreadBlowWithin is SpreadBlow on an animal plan's body: the same
// return, body and draws, on every split form, the early stops included.
func TestSpreadBlowWithinMatchesBodyRule(t *testing.T) {
	t.Parallel()
	n, stopped, quad := 0, 0, 0
	for seed := uint64(1); seed <= 200; seed++ {
		b0 := withinBodies(seed)
		room := killAt(&b0) - lossDamage(&b0)
		if room <= 0 {
			continue // dead by the body's rule already: room must be > 0
		}
		for s := SplitCut; s < numSplits; s++ {
			for _, dmg := range []int{8, 30} {
				a, b := cloneBody(b0), cloneBody(b0)
				var na, nb []int
				want := SpreadBlow(&a, dmg, Cut, s, splitTap(splitPCG(seed), &na), 3)
				got := SpreadBlowWithin(&b, dmg, Cut, s, room, splitTap(splitPCG(seed), &nb), 3)
				if got != want || !reflect.DeepEqual(a, b) || !slices.Equal(na, nb) {
					t.Fatalf("seed %d split %d dmg %d room %d: %d %v %v, SpreadBlow %d %v %v", seed, s, dmg, room, got, b.Wounds, nb, want, a.Wounds, na)
				}
				n++
				if bodyDead(&a) {
					stopped++
					if a.Anatomy == Quadruped {
						quad++
					}
				}
			}
		}
	}
	if n < 1000 || stopped == 0 || quad == 0 {
		t.Fatalf("%d blows compared, %d stopped by the body rule (%d quadruped)", n, stopped, quad)
	}
}

// TestSpreadBlowWithinRoom: a room above the body rule's deals the shares
// SpreadBlow stops before (and the spill), raising Damage by its return; a
// room below stops earlier, but never before the first share.
func TestSpreadBlowWithinRoom(t *testing.T) {
	t.Parallel()
	past := 0
	for seed := uint64(1); seed <= 200; seed++ {
		b0 := withinBodies(seed)
		room := killAt(&b0) - lossDamage(&b0)
		if room <= 0 {
			continue
		}
		for s := SplitCut; s < numSplits; s++ {
			a, b := cloneBody(b0), cloneBody(b0)
			stop := SpreadBlow(&a, 30, Cut, s, splitPCG(seed), 3)
			before := b.Damage()
			got := SpreadBlowWithin(&b, 30, Cut, s, room+1000, splitPCG(seed), 3)
			if got < stop || b.Damage()-before > got {
				t.Fatalf("seed %d split %d: a larger room took %d (Damage +%d), the body rule %d", seed, s, got, b.Damage()-before, stop)
			}
			if bodyDead(&a) && got > stop {
				past++
			}
		}
	}
	if past == 0 {
		t.Fatal("no blow dealt past the body rule's stop")
	}
	// PickPart 416 is the heart: shares Heart 21, LungL 21, Torso 21 (63)
	for _, c := range []struct {
		room, want int
		dmgs       map[Part]int
	}{
		{1, 21, map[Part]int{Heart: 21}}, // the first share is always dealt
		{21, 21, map[Part]int{Heart: 21}},
		{22, 42, map[Part]int{Heart: 21, LungL: 21}},
		{63, 63, map[Part]int{Heart: 21, LungL: 21, Torso: 21}},
		{1000, 63, map[Part]int{Heart: 21, LungL: 21, Torso: 21}},
	} {
		var b Body
		if got := SpreadBlowWithin(&b, 30, Cut, SplitCut, c.room, (&splitRec{vals: []int{416, 30, 296}}).roll, 1); got != c.want || b.Damage() != c.want || !maps.Equal(splitDmgs(&b), c.dmgs) {
			t.Errorf("room %d: returned %d, Damage %d, wounds %v; want %d %v", c.room, got, b.Damage(), splitDmgs(&b), c.want, c.dmgs)
		}
	}
	// the dying body SpreadBlow stops at 21 (TestSpreadBlowVitalAndTotal):
	// with room the arm's share and the spill are dealt too
	dying := Body{Wounds: []Wound{{Part: LegL, Dmg: 39}, {Part: LegR, Dmg: 39}, {Part: Brain, Dmg: 12}}}
	if got := SpreadBlowWithin(&dying, 30, Cut, SplitCut, 1000, (&splitRec{vals: []int{120, 30, 150}}).roll, 1); got <= 21 || splitDmgs(&dying)[ArmL] == 0 || dying.Damage() != 90+got {
		t.Errorf("dying with room: returned %d, Damage %d, wounds %v", got, dying.Damage(), splitDmgs(&dying))
	}
	// no split or no damage: Spread exactly, returning dmg
	for _, dmg := range []int{-3, 0, 17} {
		for _, s := range []Split{SplitNone, SplitCut} {
			if s == SplitCut && dmg > 0 {
				continue
			}
			a, b := Body{Anatomy: Quadruped, Scale: 300}, Body{Anatomy: Quadruped, Scale: 300}
			var na, nb []int
			Spread(&a, dmg, Cut, splitTap(splitPCG(9), &na), 1)
			if got := SpreadBlowWithin(&b, dmg, Cut, s, 5, splitTap(splitPCG(9), &nb), 1); got != dmg || !reflect.DeepEqual(a, b) || !slices.Equal(na, nb) {
				t.Fatalf("split %d dmg %d: %d %v, Spread %v", s, dmg, got, b.Wounds, a.Wounds)
			}
		}
	}
}
