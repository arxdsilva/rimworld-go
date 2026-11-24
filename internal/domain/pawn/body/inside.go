// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "math/bits"

// insideMask is each plan's inside parts, a bit per Part: the organs a
// blow reaches through the part over them (CBT 3.4, split.go). On the
// humanlike plan they are the Inner organs; each animal plan adds its
// brain, which sits under the head (the humanlike brain stands in for the
// head, its face parts hang from it, so it is outer). original tuning:
// the wiki names inner parts without listing them.
//
// solidMask is the inside parts that are bone, which pass no parent bonus
// and take no blunt inner strike: the spines. original tuning: no part is
// called a bone, so only the spine counts as one (the insect's nerve cord
// is soft).
var (
	insideMask = [NumAnatomies]uint32{
		Humanlike: 1<<Heart | 1<<LungL | 1<<LungR | 1<<Stomach | 1<<KidneyL | 1<<KidneyR | 1<<Liver | 1<<Spine,
		Quadruped: 1<<QBrain | 1<<QHeart | 1<<QLungL | 1<<QLungR | 1<<QStomach | 1<<QLiver | 1<<QKidneyL | 1<<QKidneyR | 1<<QSpine,
		Bird:      1<<BBrain | 1<<BHeart | 1<<BLungL | 1<<BLungR | 1<<BStomach | 1<<BLiver | 1<<BKidneyL | 1<<BKidneyR | 1<<BSpine,
		Snake:     1<<SBrain | 1<<SHeart | 1<<SLung | 1<<SStomach | 1<<SLiver | 1<<SKidneyL | 1<<SKidneyR | 1<<SSpine,
		Lizard:    1<<LBrain | 1<<LHeart | 1<<LLungL | 1<<LLungR | 1<<LStomach | 1<<LLiver | 1<<LKidneyL | 1<<LKidneyR | 1<<LSpine,
		Insect:    1<<IBrain | 1<<IHeart | 1<<IStomach | 1<<INerveCord,
	}
	solidMask = [NumAnatomies]uint32{
		Humanlike: 1 << Spine, Quadruped: 1 << QSpine, Bird: 1 << BSpine, Snake: 1 << SSpine, Lizard: 1 << LSpine,
	}
)

// planIndex is b's anatomy as a mask index (humanlike for an unknown
// value, as planOf reads it).
func planIndex(b *Body) Anatomy {
	if b.Anatomy >= NumAnatomies {
		return Humanlike
	}
	return b.Anatomy
}

// inside reports p an inside part of b's plan.
func inside(b *Body, p Part) bool {
	return p < b.numParts() && insideMask[planIndex(b)]&bit(p) != 0
}

// solid reports p a bone of b's plan.
func solid(b *Body, p Part) bool {
	return p < b.numParts() && solidMask[planIndex(b)]&bit(p) != 0
}

// outerOf is p, or, when p is inside, its nearest ancestor that is not.
func outerOf(b *Body, p Part) Part {
	t := b.table()
	for range t {
		if !inside(b, p) || t[p].Parent == p {
			return p
		}
		p = t[p].Parent
	}
	return p
}

// insideChildren is the set of p's present inside children: the inside
// parts q != p of b's plan whose parent is p and that are not missing (a
// head's brain on an animal plan, the torso's organs on a pawn). It walks
// the plan's inside mask, not the whole table.
func insideChildren(b *Body, p Part) uint32 {
	t := b.table()
	var m uint32
	for r := insideMask[planIndex(b)] &^ b.Missing &^ bit(p); r != 0; r &= r - 1 {
		if q := Part(bits.TrailingZeros32(r)); int(q) < len(t) && t[q].Parent == p {
			m |= bit(q)
		}
	}
	return m
}

// outwardChain is p's outward chain (a scratch's or a stab's inner split):
// p, then each parent while the current part is inside, ending with the
// first outer part, which is included. An outer p gives [p], an invalid p
// none (n = 0). Missing parts stay in the chain; dealing skips them. Call
// it as ps, n := outwardChain(b, p) and pass &ps, n on. original tuning:
// the wiki bounds no chain; it stops at maxShares parts and at the plan's
// length (every real chain is one inside part and its outer parent).
func outwardChain(b *Body, p Part) (ps [maxShares]Part, n int) {
	t := b.table()
	if p >= Part(len(t)) {
		return ps, 0
	}
	for n < maxShares && n < len(t) {
		ps[n] = p
		n++
		if !inside(b, p) || t[p].Parent == p {
			break
		}
		p = t[p].Parent
	}
	return ps, n
}
