// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// SpreadBlow deals b an unplaced blow of dmg of kind and split s (an
// animal's, CBT 3.4). With no split, or no damage, it is Spread exactly
// and returns dmg. Otherwise it picks the first part by coverage (one
// intn draw, as Spread does), plans the blow there with the same draws
// as StrikeBlow (planSplit), and deals every share as Spread deals a
// pick: a part whose loss would kill takes at most what leaves it 1 hit
// point, and what no share's part held is spread on (Spread), so Damage
// rises by exactly the blow's total. It returns that total: the sum of
// the plan's shares (a cut's total plus its parent bonuses, a blunt
// blow's dmg plus its extra, a scratch's, stab's or bullet's shares).
//
// Once a share leaves the body dead (bodyDead: an animal's body at its
// Scale's KillDamage, so a large animal lives past 100, and a lost vital
// part or an organ failure does not kill it; a pawn's also on a lost
// vital part) nothing more is dealt, neither the shares after it nor the
// spill, and it returns what Damage rose by.
func SpreadBlow(b *Body, dmg int, kind WoundKind, s Split, intn func(int) int, now uint64) int {
	if !splits(s) || dmg <= 0 {
		Spread(b, dmg, kind, intn, now)
		return dmg
	}
	p, ok := b.PickPart(intn)
	if !ok { // nothing left to pick: Spread lands it on the root, no draw
		Spread(b, dmg, kind, intn, now)
		return dmg
	}
	sh := planSplit(b, p, dmg, s, intn)
	before := b.Damage()
	total, left := 0, 0
	for i, x := range sh.s[:sh.n] {
		if i > 0 && bodyDead(b) {
			return b.Damage() - before
		}
		total += x.dmg
		left += dealSpread(b, x.part, x.dmg, kind, now)
	}
	if bodyDead(b) {
		return b.Damage() - before
	}
	if left > 0 {
		Spread(b, left, kind, intn, now)
	}
	return total
}
