// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// Stab constants. wiki: Damage Types (Stab): an outer target has a 60 %
// chance to strike an inner child instead; an inner target and its inner
// parents take 40 %, the outer parent 75 %.
const (
	stabGate      = 60 // wiki: Damage Types (Stab), 60 % chance
	pokeGate      = 40 // wiki: Damage Types (Poke), 40 % chance
	stabGateRolls = 100
	stabInnerPct  = 40 // wiki: Damage Types (Stab), 40 % on inner parts
	stabOuterPct  = 75 // wiki: Damage Types (Stab), 75 % on the outer parent
)

// stabRule is one row of the stab table: the gate chance of a strike
// reaching an inside child, and the mask that bars the plan's brain from
// the stab (all ones bars it, zero lets it through).
type stabRule struct {
	gate     int
	brainBar uint32
}

// stabRules is the stab family by split; every other split's row is unused.
var stabRules = [numSplits]stabRule{
	// wiki: Damage Types (Stab), 60 % chance; the brain is barred (original
	// tuning: the wiki says a stab "will only penetrate as deep as the skull").
	SplitStab: {stabGate, ^uint32(0)},
	// wiki: Damage Types (Poke), 40 % chance; the brain is barred (original
	// tuning: exactly the same as Stab).
	SplitPoke: {pokeGate, ^uint32(0)},
	// wiki: Damage Types (Ranged stab), 60 % chance like Stab, and it can
	// reach the brain: nothing bars it.
	SplitRangedStab: {stabGate, 0},
}

// planStab is a stab of dmg first striking p under rule r (wiki: Damage
// Types, Stab, Poke, Ranged stab), its shares in dealing order. A stab
// first landing on a barred brain becomes a stab of the brain's outer
// ancestor. An outer p with eligible inside children (never a barred
// brain) draws one gate roll and, on a hit, one coverage pick (original
// tuning: which child); a miss, or no child, is one share of dmg on p, with
// no pick. An inside final target takes stabChain over its outward chain.
func planStab(b *Body, p Part, dmg int, r stabRule, roll func(int) int) shares {
	var sh shares
	barred := bit(planOf(planIndex(b)).brain) & r.brainBar
	if bit(p)&barred != 0 {
		p = outerOf(b, p)
	}
	t := p
	if !inside(b, p) {
		if cand := insideChildren(b, p) &^ barred; cand != 0 && roll(stabGateRolls) < r.gate {
			if q, ok := pickByCoverage(b, cand, roll); ok {
				t = q
			}
		}
	}
	if inside(b, t) {
		ps, n := outwardChain(b, t)
		return stabChain(&ps, n, dmg)
	}
	sh.add(t, dmg)
	return sh
}

// stabChain is the shares of a stab striking an inside part: the chain's
// inside members (0..n-2) take stabInnerPct % of dmg each, its last
// member (the first outer ancestor) stabOuterPct %. One member takes dmg.
// original tuning: the percentages are of the post-armor dmg, the sum may
// pass 100 %, and bones are not exempt.
func stabChain(ps *[maxShares]Part, n, dmg int) shares {
	var sh shares
	switch {
	case n <= 0:
	case n == 1:
		sh.add(ps[0], dmg)
	default:
		for _, q := range ps[:n-1] {
			sh.add(q, pctOf(dmg, stabInnerPct))
		}
		sh.add(ps[n-1], pctOf(dmg, stabOuterPct))
	}
	return sh
}
