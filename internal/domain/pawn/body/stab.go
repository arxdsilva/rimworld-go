// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// Stab constants. wiki: Damage Types (Stab): an outer target has a 60 %
// chance to strike an inner child instead; an inner target and its inner
// parents take 40 %, the outer parent 75 %.
const (
	stabGate      = 60 // wiki: Damage Types (Stab), 60 % chance
	stabGateRolls = 100
	stabInnerPct  = 40 // wiki: Damage Types (Stab), 40 % on inner parts
	stabOuterPct  = 75 // wiki: Damage Types (Stab), 75 % on the outer parent
)

// planStab is a stab of dmg first striking p (wiki: Damage Types, Stab),
// its shares in dealing order. A stab first landing on the plan's brain
// becomes a stab of the brain's outer ancestor (original tuning: the wiki
// says a stab "will only penetrate as deep as the skull"). An outer p
// with eligible inside children (never the brain) draws one gate roll and,
// on a hit, one coverage pick (original tuning: which child); a miss, or
// no child, is one share of dmg on p, with no pick. An inside final
// target takes stabChain over its outward chain.
func planStab(b *Body, p Part, dmg int, roll func(int) int) shares {
	var sh shares
	brain := planOf(planIndex(b)).brain
	if p == brain {
		p = outerOf(b, p)
	}
	t := p
	if !inside(b, p) {
		if cand := insideChildren(b, p) &^ bit(brain); cand != 0 && roll(stabGateRolls) < stabGate {
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
