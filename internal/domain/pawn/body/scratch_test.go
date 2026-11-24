// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"math/bits"
	"slices"
	"testing"
)

// scratchBrute is the scratch list of p by a plain scan of the table.
func scratchBrute(b *Body, p Part) uint32 {
	t := b.table()
	par := t[p].Parent
	var m uint32
	for q := range t {
		q := Part(q)
		if b.isMissing(q) {
			continue
		}
		if q == p || q == par || t[q].Parent == p || t[q].Parent == par {
			m |= bit(q)
		}
	}
	return m
}

func scratchTotal(sh shares) int {
	n := 0
	for _, x := range sh.s[:sh.n] {
		n += x.dmg
	}
	return n
}

func scratchParts(sh shares) []Part {
	var out []Part
	for _, x := range sh.s[:sh.n] {
		out = append(out, x.part)
	}
	return out
}

// scratchPick finds the roll that makes PickPart return want.
func scratchPick(t *testing.T, b *Body, want Part) int {
	t.Helper()
	for r := range 1000 {
		if p, ok := b.PickPart(func(int) int { return r }); ok && p == want {
			return r
		}
	}
	t.Fatalf("no roll picks part %d", want)
	return 0
}

// TestScratchList: the list is p, its children, its parent and the
// parent's children (a root once), for outer parts of every plan, with and
// without a missing neighbour.
func TestScratchList(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a Anatomy
		p Part
	}{{Humanlike, HandL}, {Humanlike, Torso}, {Humanlike, Nose}, {Quadruped, QHead}} {
		b := &Body{Anatomy: c.a}
		if got, want := scratchList(b, c.p), scratchBrute(b, c.p); got != want {
			t.Errorf("anatomy %d part %d: list %b, want %b", c.a, c.p, got, want)
		}
	}
	for a := Humanlike; a < NumAnatomies; a++ {
		for _, miss := range []uint32{0, 0x2a, 0x155} {
			b := &Body{Anatomy: a, Missing: miss}
			for p := Part(0); p < Part(len(b.table())); p++ {
				if inside(b, p) || b.isMissing(p) {
					continue
				}
				if got, want := scratchList(b, p), scratchBrute(b, p); got != want {
					t.Errorf("anatomy %d miss %x part %d: list %b, want %b", a, miss, p, got, want)
				}
			}
		}
	}
}

// TestScratchOuterTwoDistinct: an outer hit with 3 or more candidates
// plans 2 distinct shares of 67 % each: 35 + 35 at 53, 67 + 67 at 100.
func TestScratchOuterTwoDistinct(t *testing.T) {
	t.Parallel()
	b := &Body{}
	l := scratchList(b, Torso)
	c := bits.OnesCount32(l)
	if c < 3 {
		t.Fatalf("torso list has %d parts", c)
	}
	for _, w := range []struct{ dmg, each, total int }{{53, 35, 70}, {100, 67, 134}} {
		sh := planScratch(b, Torso, w.dmg, splitPCG(7))
		if sh.n != 2 || sh.s[0].part == sh.s[1].part {
			t.Fatalf("dmg %d: shares %v", w.dmg, scratchParts(sh))
		}
		for _, x := range sh.s[:sh.n] {
			if x.dmg != w.each || l&bit(x.part) == 0 {
				t.Errorf("dmg %d: share %+v, want %d inside the list", w.dmg, x, w.each)
			}
		}
		if got := scratchTotal(sh); got != w.total {
			t.Errorf("dmg %d: total %d, want %d", w.dmg, got, w.total)
		}
	}
}

// TestScratchInside: an inside first part splits dmg over its chain, the
// remainder on the first, and draws nothing.
func TestScratchInside(t *testing.T) {
	t.Parallel()
	b := &Body{}
	sh := planScratch(b, Heart, 53, splitNever)
	if sh.n != 2 || sh.s[0] != (share{Heart, 27}) || sh.s[1] != (share{Torso, 26}) {
		t.Fatalf("heart at 53: %+v", sh.s[:sh.n])
	}
}

// TestScratchSmall: 1 or 2 candidates draw nothing; a body whose
// neighbours are all gone puts the full dmg on p.
func TestScratchSmall(t *testing.T) {
	t.Parallel()
	b := &Body{}
	l := scratchList(b, Torso)
	// Leave Torso alone: every other part missing.
	alone := &Body{Missing: ^uint32(0) & (1<<uint(len(b.table())) - 1) &^ bit(Torso)}
	sh := planScratch(alone, Torso, 40, splitNever)
	if sh.n != 1 || sh.s[0] != (share{Torso, 40}) {
		t.Fatalf("alone: %+v", sh.s[:sh.n])
	}
	// Torso and exactly one other part.
	other := Part(bits.TrailingZeros32(l &^ bit(Torso)))
	two := &Body{Missing: alone.Missing &^ bit(other)}
	sh = planScratch(two, Torso, 53, splitNever)
	if sh.n != 2 || sh.s[0] != (share{Torso, 35}) || sh.s[1] != (share{other, 35}) {
		t.Fatalf("two: %+v", sh.s[:sh.n])
	}
}

// TestScratchDraws: the draws are the first pick at bound c, then the
// second at c-1; every list member is a reachable first pick, and rolls
// out of range are clamped.
func TestScratchDraws(t *testing.T) {
	t.Parallel()
	b := &Body{}
	l := scratchList(b, Torso)
	c := bits.OnesCount32(l)
	var seen uint32
	for k := range c {
		r := &splitRec{vals: []int{k, 0}}
		sh := planScratch(b, Torso, 50, r.roll)
		if !slices.Equal(r.ns, []int{c, c - 1}) {
			t.Fatalf("bounds %v, want [%d %d]", r.ns, c, c-1)
		}
		if sh.s[0].part != scratchNth(l, k) {
			t.Errorf("k=%d first %d", k, sh.s[0].part)
		}
		seen |= bit(sh.s[0].part)
	}
	if seen != l {
		t.Errorf("first picks reach %b, want %b", seen, l)
	}
	// Second pick: the k-th of the rest.
	r := &splitRec{vals: []int{0, c - 2}}
	sh := planScratch(b, Torso, 50, r.roll)
	if sh.s[1].part != scratchNth(l&^bit(sh.s[0].part), c-2) {
		t.Errorf("second pick %d", sh.s[1].part)
	}
	// Clamping.
	r = &splitRec{vals: []int{-5, 99}}
	sh = planScratch(b, Torso, 50, r.roll)
	if sh.n != 2 || sh.s[0].part == sh.s[1].part {
		t.Errorf("clamped: %v", scratchParts(sh))
	}
	for _, x := range sh.s[:sh.n] {
		if l&bit(x.part) == 0 {
			t.Errorf("clamped part %d outside list", x.part)
		}
	}
	// A first roll past the end is clamped into the list too.
	r = &splitRec{vals: []int{99, 0}}
	sh = planScratch(b, Torso, 50, r.roll)
	if sh.n != 2 || sh.s[0].part == sh.s[1].part {
		t.Errorf("high first roll: %v", scratchParts(sh))
	}
	for _, x := range sh.s[:sh.n] {
		if l&bit(x.part) == 0 {
			t.Errorf("high first roll: part %d outside list", x.part)
		}
	}
	// An invalid part falls back to one full share.
	if sh := planScratch(b, Part(60), 9, splitNever); sh.n != 1 || sh.s[0] != (share{60, 9}) {
		t.Errorf("invalid part: %+v", sh.s[:sh.n])
	}
}

// TestScratchStrikeBlowDraws: the plan's two draws come before the
// overkill draws.
func TestScratchStrikeBlowDraws(t *testing.T) {
	t.Parallel()
	b := &Body{}
	c := bits.OnesCount32(scratchList(b, Torso))
	var ns []int
	StrikeBlow(b, Torso, 40, Scratch, SplitScratch, 400, 1000, splitTap(splitPCG(3), &ns), 1)
	if len(ns) < 2 || ns[0] != c || ns[1] != c-1 {
		t.Fatalf("draws %v, want %d then %d first", ns, c, c-1)
	}
}

// TestScratchChain: synthetic chains split evenly, the remainder on the
// first, zero shares skipped.
func TestScratchChain(t *testing.T) {
	t.Parallel()
	var ps [maxShares]Part
	for i := range ps {
		ps[i] = Part(i + 1)
	}
	for _, c := range []struct {
		n, dmg int
		want   []int
	}{
		{3, 53, []int{19, 17, 17}}, {2, 53, []int{27, 26}}, {8, 53, []int{11, 6, 6, 6, 6, 6, 6, 6}},
		{3, 2, []int{2}}, {0, 53, nil}, {3, 0, nil}, {20, 16, []int{2, 2, 2, 2, 2, 2, 2, 2}},
	} {
		sh := scratchChain(&ps, c.n, c.dmg)
		var got []int
		for _, x := range sh.s[:sh.n] {
			got = append(got, x.dmg)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("n=%d dmg=%d: %v, want %v", c.n, c.dmg, got, c.want)
		}
		if sh.n > 0 && sh.s[0].part != ps[0] {
			t.Errorf("n=%d: first part %d", c.n, sh.s[0].part)
		}
	}
}

// TestScratchSpreadBlowTotal: an unplaced scratch on the big quadruped
// deals exactly the plan's total.
func TestScratchSpreadBlowTotal(t *testing.T) {
	t.Parallel()
	b := splitBigQuad()
	r := scratchPick(t, &b, QTorso)
	n := 0
	roll := func(int) int {
		n++
		if n == 1 {
			return r
		}
		return 0
	}
	if got := SpreadBlow(&b, 53, Scratch, SplitScratch, roll, 1); got != 70 {
		t.Fatalf("total %d, want 70", got)
	}
	if got := bits.OnesCount32(splitWounded(&b)); got != 2 {
		t.Errorf("%d parts wounded", got)
	}
}

// TestScratchSpreadBlowStop: with a vital part gone a humanlike body is
// dead after the first share, so the second is not dealt; an animal body
// is not, so both are.
func TestScratchSpreadBlowStop(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		b       Body
		first   Part
		wounded int
	}{
		{Body{Missing: bit(Brain)}, Torso, 1},
		{Body{Anatomy: Quadruped, Scale: 1000, Missing: bit(QBrain)}, QTorso, 2},
	} {
		r := scratchPick(t, &c.b, c.first)
		n := 0
		roll := func(int) int {
			n++
			if n == 1 {
				return r
			}
			return 0
		}
		SpreadBlow(&c.b, 40, Scratch, SplitScratch, roll, 1)
		if got := bits.OnesCount32(splitWounded(&c.b)); got != c.wounded {
			t.Errorf("anatomy %d: %d parts wounded, want %d", c.b.Anatomy, got, c.wounded)
		}
	}
}

// TestScratchAllocs: planning allocates nothing.
func TestScratchAllocs(t *testing.T) { // not parallel: AllocsPerRun
	b := &Body{}
	var sink shares
	roll := func(int) int { return 1 }
	n := testing.AllocsPerRun(200, func() {
		sink = planScratch(b, Torso, 53, roll)
		sink = planScratch(b, Heart, 53, roll)
	})
	if n != 0 || sink.n == 0 {
		t.Fatalf("%v allocs", n)
	}
}
