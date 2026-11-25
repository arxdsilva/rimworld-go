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
	b.Replacements = slices.Clone(b.Replacements)
	b.Implants = slices.Clone(b.Implants)
	return b
}

// observeRoomShare is a test-only physical accounting oracle. It measures
// surviving part damage, or the remaining HP consumed by a destruction.
// It never treats the removal of an old child wound as new spill.
func observeRoomShare(t *testing.T, b *Body, p Part, dmg int, now uint64) int {
	t.Helper()
	if p >= b.numParts() || b.isMissing(p) || dmg <= 0 {
		return 0
	}
	before := b.partDmg(p)
	capacity := b.PartDef(p).MaxHP - before
	allowed := capacity
	if b.LossKills(p) {
		allowed--
	}
	b.hurtPart(p, min(dmg, max(0, allowed)), Cut, now)
	if b.isMissing(p) {
		return capacity
	}
	return b.partDmg(p) - before
}

// referenceRoom applies the native plan, observing each physical
// placement independently. Record whether legacy aggregate accounting
// is provably equivalent, including its unbounded-after-death spill.
func referenceRoom(t *testing.T, b *Body, dmg int, s Split, room int, roll func(int) int) (int, bool) {
	t.Helper()
	start := lossDamage(b)
	p, ok := b.PickPart(roll)
	if !ok {
		t.Fatal("fixture has no initial part")
	}
	plan := planSplit(b, p, dmg, s, roll)
	placed, pending, planned := 0, 0, 0
	equivalent := true
	observe := func(p Part, n int) int {
		before := b.Damage()
		hit := observeRoomShare(t, b, p, n, 3)
		placed += hit
		if b.Damage()-before != hit || lossDamage(b)-start != placed {
			equivalent = false
		}
		return n - hit
	}
	for _, x := range plan.s[:plan.n] {
		planned += x.dmg
		pending += observe(x.part, x.dmg)
		if placed >= room {
			return placed, equivalent
		}
	}
	for attempt := 0; pending > 0 && attempt < MaxSpill; attempt++ {
		p, ok := b.PickPart(roll)
		if !ok {
			break
		}
		pending = observe(p, pending)
		if placed >= room {
			return placed, equivalent && pending == 0
		}
	}
	if pending > 0 {
		before := b.Damage()
		b.Wounds = append(b.Wounds, Wound{Part: b.Root(), Dmg: pending, Kind: Cut, At: 3, Size: int32(pending)})
		placed += pending
		equivalent = equivalent && b.Damage()-before == pending && lossDamage(b)-start == placed
	}
	if placed != planned {
		t.Fatalf("oracle lost damage: placed %d planned %d", placed, planned)
	}
	return placed, equivalent
}

// TestSpreadBlowWithinMatchesBodyRule retains all upstream seeds, forms
// and damage sizes. Where the anatomical rise equals physical placements
// and the old spill does not continue after death, pin the old return,
// body and draws. In every case also compare the observed-placement oracle.
// This explicitly exercises the accounting divergence rather than ignoring it.
func TestSpreadBlowWithinMatchesBodyRule(t *testing.T) {
	t.Parallel()
	n, stopped, quad, same, different := 0, 0, 0, 0, 0
	for seed := uint64(1); seed <= 200; seed++ {
		b0 := withinBodies(seed)
		room := killAt(&b0) - lossDamage(&b0)
		if room <= 0 {
			continue
		}
		for s := SplitCut; s < numSplits; s++ {
			for _, dmg := range []int{8, 30} {
				a, b, ref := cloneBody(b0), cloneBody(b0), cloneBody(b0)
				var na, nb, nr []int
				legacy := SpreadBlow(&a, dmg, Cut, s, splitTap(splitPCG(seed), &na), 3)
				want, equivalent := referenceRoom(t, &ref, dmg, s, room, splitTap(splitPCG(seed), &nr))
				got := SpreadBlowWithin(&b, dmg, Cut, s, room, splitTap(splitPCG(seed), &nb), 3)
				if got != want || !reflect.DeepEqual(ref, b) || !slices.Equal(nb, nr) {
					t.Fatalf("seed %d split %d dmg %d room %d: actual %d %v %v, reference %d %v %v", seed, s, dmg, room, got, b.Wounds, nb, want, ref.Wounds, nr)
				}
				match := got == legacy && reflect.DeepEqual(a, b) && slices.Equal(na, nb)
				if equivalent && !match {
					t.Fatalf("seed %d split %d dmg %d: equivalent accounting changed legacy body/draws: %d vs %d", seed, s, dmg, got, legacy)
				}
				if match {
					same++
				} else {
					different++
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
	t.Logf("%d comparisons, %d legacy exact, %d corrected divergences, %d body stops (%d quadruped)", n, same, different, stopped, quad)
	if n < 1000 || stopped == 0 || quad == 0 || same < 1000 || different == 0 {
		t.Fatal("insufficient equivalent and divergent split coverage")
	}
}

// TestSpreadBlowWithinRoom: a larger owner room permits the later
// physical placements; a smaller room stops earlier. Preserve the exact
// healthy and unsplit cases and compare all broader cases with the oracle.
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
			stop := SpreadBlowWithin(&a, 30, Cut, s, room, splitPCG(seed), 3)
			ref := cloneBody(b0)
			want, _ := referenceRoom(t, &ref, 30, s, room+1000, splitPCG(seed))
			got := SpreadBlowWithin(&b, 30, Cut, s, room+1000, splitPCG(seed), 3)
			if got < stop || got != want || !reflect.DeepEqual(ref, b) {
				t.Fatalf("seed %d split %d: larger room took %d, oracle %d, smaller room %d", seed, s, got, want, stop)
			}
			if stop >= room && got > stop {
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
