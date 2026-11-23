// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"maps"
	"slices"
	"testing"
)

// TestBluntCandidates: a blunt hit reaches only the present inner,
// non-bone parts right under the struck part.
func TestBluntCandidates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		b    Body
		p    Part
		want uint32
	}{
		{Body{}, Torso, cutMask(Heart, LungL, LungR, Stomach, KidneyL, KidneyR, Liver)},
		{Body{Missing: bit(Heart)}, Torso, cutMask(LungL, LungR, Stomach, KidneyL, KidneyR, Liver)},
		{Body{}, Brain, 0},
		{Body{}, ArmL, 0},
		{Body{}, HandL, 0},
		{Body{}, Heart, 0},
		{Body{Anatomy: Quadruped}, QHead, cutMask(QBrain)},
		{Body{Anatomy: Snake}, SBody, cutMask(SHeart, SLung, SStomach, SLiver, SKidneyL, SKidneyR)},
		{Body{Anatomy: Insect}, IThorax, cutMask(IHeart, IStomach, INerveCord)},
	}
	for _, c := range cases {
		if got := bluntCandidates(&c.b, c.p); got != c.want {
			t.Errorf("%v %v: %b, want %b", c.b.Anatomy, c.p, got, c.want)
		}
	}
}

// TestBluntInnerStrike: on the gate the outer part keeps 80-90 % and the
// inner part takes the rest plus 20-35 % of the base; off it the hit is
// a plain strike.
func TestBluntInnerStrike(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		vals  []int
		dmg   int
		want  map[Part]int
		draws []int
	}{
		{"low draws", []int{0, 0, 0, 0}, 20, map[Part]int{Torso: 16, Heart: 8}, []int{100, 169, 11, 16}},
		{"top draws", []int{39, 168, 10, 15}, 20, map[Part]int{Torso: 18, Liver: 9}, []int{100, 169, 11, 16}},
		{"gate missed", []int{40}, 20, map[Part]int{Torso: 20}, []int{100}},
		{"tiny blow", []int{0, 0, 0, 0}, 1, map[Part]int{Heart: 1}, []int{100, 169, 11, 16}},
	}
	for _, c := range cases {
		var b Body
		r := &splitRec{vals: c.vals}
		StrikeBlow(&b, Torso, c.dmg, Bruise, SplitBlunt, 400, 1000, r.roll, 1)
		if got := splitDmgs(&b); !maps.Equal(got, c.want) || !slices.Equal(r.ns, c.draws) {
			t.Errorf("%s: wounds %v draws %v, want %v %v", c.name, got, r.ns, c.want, c.draws)
		}
	}
	// a blow of 100 on a large quadruped (Scale 3000: torso 150, liver 120)
	// shows every percent: the liver (pick 93 of 145) holds its share
	// uncapped, so the outer and inner parts take keep and 100 - keep +
	// extra exactly
	for _, c := range []struct {
		vals         []int
		outer, inner int
	}{{[]int{0, 93, 0, 0}, 80, 40}, {[]int{39, 93, 10, 15}, 90, 45}} {
		q := Body{Anatomy: Quadruped, Scale: 3000}
		r := &splitRec{vals: c.vals}
		StrikeBlow(&q, QTorso, 100, Bruise, SplitBlunt, 400, 1000, r.roll, 1)
		want := map[Part]int{QTorso: c.outer, QLiver: c.inner}
		if got := splitDmgs(&q); !maps.Equal(got, want) || !slices.Equal(r.ns, []int{100, 145, 11, 16}) {
			t.Errorf("blunt 100 with %v: wounds %v draws %v, want %v", c.vals, got, r.ns, want)
		}
	}
	// the inner share on a vital organ stops at MaxHP - 1
	var b Body
	StrikeBlow(&b, Torso, 100, Bruise, SplitBlunt, 400, 1000, (&splitRec{}).roll, 1)
	if got := splitDmgs(&b); got[Heart] != 24 || got[Torso] != 59 || b.isMissing(Heart) {
		t.Fatalf("vital inner share: %v", got)
	}
}

// TestBluntRate: a blunt blow on the torso strikes inside 40 % of the
// time (wiki: Damage Types, Blunt).
func TestBluntRate(t *testing.T) {
	t.Parallel()
	roll := splitPCG(4040)
	const n = 10000
	in := 0
	for range n {
		var b Body
		StrikeBlow(&b, Torso, 10, Bruise, SplitBlunt, 400, 1000, roll, 1)
		if splitWounded(&b)&^bit(Torso) != 0 {
			in++
		}
	}
	if in*100 < 38*n || in*100 > 42*n {
		t.Fatalf("inner strikes %d of %d, want 40 %%", in, n)
	}
}
