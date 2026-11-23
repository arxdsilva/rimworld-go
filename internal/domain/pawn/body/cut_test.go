// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"slices"
	"testing"
)

// cutMask is the set of parts ps.
func cutMask(ps ...Part) uint32 {
	var m uint32
	for _, p := range ps {
		m |= bit(p)
	}
	return m
}

// TestCutCount: one roll of 100 gives 1 part under 30, 2 under 75, 3
// under 95, else 4 (wiki: Damage Types, Cut: 30/45/20/5 %).
func TestCutCount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ roll, k int }{
		{0, 1}, {29, 1}, {30, 2}, {74, 2}, {75, 3}, {94, 3}, {95, 4}, {99, 4}, {-1, 1}, {100, 4},
	} {
		r := &splitRec{vals: []int{c.roll}}
		if got := cutCount(r.roll); got != c.k || !slices.Equal(r.ns, []int{100}) {
			t.Errorf("roll %d: k %d (draws %v), want %d", c.roll, got, r.ns, c.k)
		}
	}
}

// TestCutTotal: the total is dmg x (1+1.4)/(k+1.4) x k, rounded.
func TestCutTotal(t *testing.T) {
	t.Parallel()
	for k, want := range map[int][2]int{1: {100, 10}, 2: {141, 14}, 3: {164, 16}, 4: {178, 18}} {
		if got := cutTotal(100, k); got != want[0] {
			t.Errorf("100 at k %d: %d, want %d", k, got, want[0])
		}
		if got := cutTotal(10, k); got != want[1] {
			t.Errorf("10 at k %d: %d, want %d", k, got, want[1])
		}
	}
}

// TestCutAdjacency: the extras are the first part's parent and children,
// plus its inside siblings when it is inside itself; present parts only.
func TestCutAdjacency(t *testing.T) {
	t.Parallel()
	var b Body
	cases := []struct {
		p    Part
		want uint32
	}{
		{ArmL, cutMask(Torso, HandL)},
		{HandL, cutMask(ArmL)},
		{Heart, cutMask(Torso, LungL, LungR, Stomach, KidneyL, KidneyR, Liver, Spine)},
		{Torso, cutMask(Brain, Heart, LungL, LungR, Stomach, ArmL, ArmR, LegL, LegR, KidneyL, KidneyR, Liver, Spine)},
		{Brain, cutMask(Torso, EyeL, EyeR, EarL, EarR, Nose, Jaw)},
	}
	for _, c := range cases {
		if got := cutCandidates(&b, c.p); got != c.want {
			t.Errorf("%v: %b, want %b", c.p, got, c.want)
		}
	}
	if got := cutCandidates(&Body{Missing: bit(HandL) | bit(LungR)}, ArmL); got != cutMask(Torso) {
		t.Errorf("missing hand: %b", got)
	}
	q := Body{Anatomy: Quadruped}
	if got := cutCandidates(&q, QBrain); got != cutMask(QHead) {
		t.Errorf("quadruped brain: %b", got)
	}
	if got := cutCandidates(&q, QHeart); got != cutMask(QTorso, QLungL, QLungR, QStomach, QLiver, QKidneyL, QKidneyR, QSpine) {
		t.Errorf("quadruped heart: %b", got)
	}
	// k = 4 on ArmL: only Torso and HandL can join, so 3 parts at k = 3
	r := &splitRec{vals: []int{95, 0, 0}}
	StrikeBlow(&b, ArmL, 10, Cut, SplitCut, 0, 100, r.roll, 1)
	if !slices.Equal(r.ns, []int{100, 321, 25}) {
		t.Fatalf("draws %v", r.ns)
	}
	if got := splitDmgs(&b); len(got) != 3 || got[ArmL] != 6 || got[Torso] != 5 || got[HandL] != 5 {
		t.Fatalf("ArmL k=4: %v", got)
	}
	// every candidate missing: the root alone, k shrinks to 1, no pick drawn
	all := Body{Missing: ^bit(Torso)}
	r = &splitRec{vals: []int{99}}
	StrikeBlow(&all, Torso, 10, Cut, SplitCut, 0, 100, r.roll, 1)
	if got := splitDmgs(&all); !slices.Equal(r.ns, []int{100}) || len(got) != 1 || got[Torso] != 10 {
		t.Fatalf("no candidates: draws %v wounds %v", r.ns, got)
	}
	if _, ok := pickByCoverage(&b, 0, splitNever); ok {
		t.Fatal("picked from nothing")
	}
}

// TestCutSingleNeverInner: a one-part cut on an inner part lands on its
// nearest outer ancestor instead (no re-roll).
func TestCutSingleNeverInner(t *testing.T) {
	t.Parallel()
	var b Body
	r := &splitRec{vals: []int{0}}
	StrikeBlow(&b, Heart, 10, Cut, SplitCut, 0, 100, r.roll, 1)
	if got := splitDmgs(&b); !slices.Equal(r.ns, []int{100}) || len(got) != 1 || got[Torso] != 10 {
		t.Fatalf("heart single: draws %v wounds %v", r.ns, got)
	}
	q := Body{Anatomy: Quadruped}
	StrikeBlow(&q, QBrain, 5, Cut, SplitCut, 0, 100, (&splitRec{}).roll, 1)
	if got := splitDmgs(&q); len(got) != 1 || got[QHead] != 5 {
		t.Fatalf("quadruped brain single: %v", got)
	}
	var c Body
	StrikeBlow(&c, ArmL, 10, Cut, SplitCut, 0, 100, (&splitRec{}).roll, 1)
	if got := splitDmgs(&c); len(got) != 1 || got[ArmL] != 10 {
		t.Fatalf("outer single: %v", got)
	}
}

// TestCutParentBonus: an inner, non-bone target whose present parent is
// not a target gives that parent one bonus wound of the first such share.
func TestCutParentBonus(t *testing.T) {
	t.Parallel()
	var b Body
	r := &splitRec{vals: []int{30, 296}} // k = 2: Heart, then LungL
	StrikeBlow(&b, Heart, 10, Cut, SplitCut, 0, 100, r.roll, 1)
	got := splitDmgs(&b)
	torso := 0
	for _, w := range b.Wounds {
		if w.Part == Torso {
			torso++
		}
	}
	if got[Heart] != 7 || got[LungL] != 7 || got[Torso] != 7 || torso != 1 || len(got) != 3 {
		t.Fatalf("heart and lung: %v (%d torso wounds)", got, torso)
	}
	bonus := func(b *Body, ss ...share) []share {
		var sh shares
		for _, s := range ss {
			sh.add(s.part, s.dmg)
		}
		cutBonuses(b, &sh)
		return sh.s[len(ss):sh.n]
	}
	var fresh Body
	if got := bonus(&fresh, share{Spine, 5}, share{ArmL, 5}); len(got) != 0 {
		t.Errorf("spine is a bone: %v", got)
	}
	if got := bonus(&fresh, share{LungL, 6}, share{ArmL, 5}); !slices.Equal(got, []share{{Torso, 6}}) {
		t.Errorf("lung: %v", got)
	}
	if got := bonus(&fresh, share{Heart, 6}, share{LungL, 5}); !slices.Equal(got, []share{{Torso, 6}}) {
		t.Errorf("the first target's share, remainder included: %v", got)
	}
	if got := bonus(&fresh, share{Torso, 6}, share{LungL, 5}); len(got) != 0 {
		t.Errorf("parent already a target: %v", got)
	}
	if got := bonus(&fresh, share{ArmL, 4}, share{LungL, 3}, share{Heart, 3}); !slices.Equal(got, []share{{Torso, 3}}) {
		t.Errorf("one bonus a parent, the first share: %v", got)
	}
	if got := bonus(&Body{Missing: bit(Torso)}, share{LungL, 6}); len(got) != 0 {
		t.Errorf("missing parent: %v", got)
	}
	q := Body{Anatomy: Quadruped}
	if got := bonus(&q, share{QBrain, 4}, share{QLungL, 3}); !slices.Equal(got, []share{{QHead, 4}, {QTorso, 3}}) {
		t.Errorf("quadruped: %v", got)
	}
}
