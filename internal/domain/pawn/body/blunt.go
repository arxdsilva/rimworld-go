// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// A blunt blow strikes inside bluntGate % of the time; the outer part
// keeps bluntKeepMin + roll(bluntKeepSpan) % of the blow and the inner
// part takes the rest plus bluntExtraMin + roll(bluntExtraSpan) % of the
// base. wiki: Damage Types (Blunt: 40 % chance; 10-20 % of the damage
// moved to an inner part, plus 20-35 %).
const (
	bluntGate       = 40
	bluntKeepMin    = 80
	bluntKeepSpan   = 11 // 80..90 %
	bluntExtraMin   = 20
	bluntExtraSpan  = 16 // 20..35 %
	bluntGateRollOf = 100
)

// bluntCandidates is the set of parts a blunt blow on p may strike
// inside: p's present inside children that are not bone (none for an
// arm, a hand or the humanlike brain).
func bluntCandidates(b *Body, p Part) uint32 {
	return insideChildren(b, p) &^ solidMask[planIndex(b)]
}

// planBlunt is a blunt blow of dmg on p (R2). With no candidate under p it
// draws nothing; otherwise it draws the gate, and on a miss the blow is
// p's alone. On the gate it draws the inner part by coverage (original
// tuning, pickByCoverage), the keep
// and the extra: p takes dmg x keep and the inner part the rest plus
// dmg x extra, so the blow deals dmg + dmg x extra in all. original
// tuning: the wiki's 10-20 % and 20-35 % read as the share moved inward
// plus a fresh share of the base.
func planBlunt(b *Body, p Part, dmg int, roll func(int) int) shares {
	var sh shares
	cand := bluntCandidates(b, p)
	if cand == 0 || roll(bluntGateRollOf) >= bluntGate {
		sh.add(p, dmg)
		return sh
	}
	q, ok := pickByCoverage(b, cand, roll)
	if !ok {
		sh.add(p, dmg)
		return sh
	}
	keep := bluntKeepMin + roll(bluntKeepSpan)
	extra := bluntExtraMin + roll(bluntExtraSpan)
	outer := dmg * keep / 100
	sh.add(p, outer)
	sh.add(q, dmg-outer+dmg*extra/100)
	return sh
}
