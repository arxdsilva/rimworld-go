// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "math/bits"

// scratchPct is the share of a scratch's damage each struck part takes.
// wiki: Damage Types (Scratch): 67 % of the damage to each of 2 parts.
const scratchPct = 67

// scratchList is the set a scratch first striking the outer part p picks
// from: p, its children, its parent and the parent's children, present
// parts only (wiki: "selected from this list at random"). A root is its own
// parent, so the set stays deduplicated by being a mask. original tuning:
// the wiki does not say whether the inside children count; they are in
// (cutCandidates), as the list says "children".
func scratchList(b *Body, p Part) uint32 {
	return cutCandidates(b, p) | bit(p) | scratchSiblings(b, p)
}

// scratchSiblings is the present parts hanging from p's parent, except the
// parent itself (a root's own self-parent entry).
func scratchSiblings(b *Body, p Part) uint32 {
	t := b.table()
	par := t[p].Parent
	var m uint32
	for q := range t {
		if Part(q) != par && t[q].Parent == par && !b.isMissing(Part(q)) {
			m |= bit(Part(q))
		}
	}
	return m
}

// scratchNth is the k-th (from 0) set bit of mask as a part.
func scratchNth(mask uint32, k int) Part {
	for ; k > 0; k-- {
		mask &= mask - 1
	}
	return Part(bits.TrailingZeros32(mask))
}

// scratchChain splits dmg evenly over the n parts of ps; the first takes
// the remainder. original tuning: the wiki says an inner scratch is split
// over its chain without rounding; a share of 0 is not added.
func scratchChain(ps *[maxShares]Part, n, dmg int) shares {
	var sh shares
	n = min(n, maxShares)
	if n <= 0 || dmg <= 0 {
		return sh
	}
	each, rem := dmg/n, dmg%n
	for i := range n {
		d := each
		if i == 0 {
			d += rem
		}
		if d > 0 {
			sh.add(ps[i], d)
		}
	}
	return sh
}

// planScratch is a scratch of dmg first striking p (wiki: Damage Types,
// Scratch), its shares in dealing order. An inner p splits dmg over its
// outward chain, drawing nothing. An outer p wounds 2 distinct parts of
// scratchList, each for scratchPct % of dmg: with 3 or more candidates two
// uniform draws (bounds c, then c-1 over the rest) pick them; with 2 both
// are hit, p first; with 1 p takes dmg. original tuning: the picks are
// distinct and p itself may go unpicked.
func planScratch(b *Body, p Part, dmg int, roll func(int) int) shares {
	var sh shares
	if int(p) >= len(b.table()) {
		sh.add(p, dmg)
		return sh
	}
	if inside(b, p) {
		ps, n := outwardChain(b, p)
		return scratchChain(&ps, n, dmg)
	}
	l := scratchList(b, p)
	c := bits.OnesCount32(l)
	d := pctOf(dmg, scratchPct)
	switch {
	case c <= 1:
		sh.add(p, dmg)
	case c == 2:
		sh.add(p, d)
		sh.add(scratchNth(l&^bit(p), 0), d)
	default:
		first := min(max(roll(c), 0), c-1)
		a := scratchNth(l, first)
		rest := l &^ bit(a)
		second := min(max(roll(c-1), 0), c-2)
		sh.add(a, d)
		sh.add(scratchNth(rest, second), d)
	}
	return sh
}
