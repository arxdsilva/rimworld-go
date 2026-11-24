// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "rimworld-go/internal/domain/combat/damage"

// Split is the multi-part rule a blow's damage form gives it (CBT 3.4):
// none, or a plan that spreads the blow over several parts (planSplit:
// cut.go, blunt.go, scratch.go, stab.go, bullet.go; Poke and Ranged stab are
// stab.go's rules). The zero value
// splits nothing, so a blow that names no form keeps Strike's and
// Spread's single-part rule.
type Split uint8

const (
	SplitNone Split = iota
	SplitCut
	SplitBlunt
	SplitScratch
	SplitStab
	SplitBullet
	SplitPoke
	SplitRangedStab
	numSplits // sentinel: the first value that is not a split
)

// maxShares bounds a split blow's shares: a cut has up to 4 targets and 4
// parent bonuses, a blunt hit 2, a scratch 2 or its outward chain, a stab,
// poke or ranged stab its outward chain, a bullet 2.
const maxShares = 8

// splitOf is each form's split. wiki: Damage Types:
//   - Cut: 1 to 4 adjacent parts, plus a bonus on an inner part's parent.
//   - Blunt: a 40 % chance of an inner strike under the part.
//   - Scratch: 2 parts of the target, its children, its parent and the
//     parent's children, 67 % each; on an inner target, an even split over
//     it and its parents up to the first outer one.
//   - Stab: a 60 % chance to strike an inner child (never the brain); an
//     inner target and its inner parents take 40 %, the outer parent 75 %.
//   - Bullet: an inner target's parent takes the full damage as a bonus.
//   - Poke: a 40 % chance to strike an inner child, otherwise as Stab
//     (original tuning: the brain is barred as for Stab).
//   - Ranged stab: as Stab, but it can reach the brain.
//
// Bite and Arrow have no multi-part rule on the wiki and wound one part,
// as does every other form: a bomb's shrapnel and a crushing weight
// included, though they cut and bruise (WoundOf).
var splitOf = [damage.NumForms]Split{
	damage.FormCut:        SplitCut,
	damage.FormBlunt:      SplitBlunt,
	damage.FormScratch:    SplitScratch,
	damage.FormStab:       SplitStab,
	damage.FormBullet:     SplitBullet,
	damage.FormPoke:       SplitPoke,
	damage.FormRangedStab: SplitRangedStab,
}

// share is the damage one part takes from a split blow.
type share struct {
	part Part
	dmg  int
}

// shares is a split blow's targets in dealing order, held in place (at
// most maxShares), so splitting allocates nothing.
type shares struct {
	s [maxShares]share
	n int
}

// SplitOf is form f's split (SplitNone for an unknown form).
func SplitOf(f damage.Form) Split {
	if f >= damage.NumForms {
		return SplitNone
	}
	return splitOf[f]
}

// killAt is the Damage at which b is dead: KillDamage scaled by the
// body's Scale (MED-13: an animal's body is scaled to its hit points, and
// the game kills it at its MaxHP), KillDamage for a human (Scale 0 or
// 1000).
func killAt(b *Body) int {
	if b.Scale <= 0 || b.Scale == 1000 {
		return KillDamage
	}
	return max((KillDamage*int(b.Scale)+500)/1000, 1)
}

// bodyDead reports b dead as the game kills its owner. A humanlike body
// (a pawn's) dies at KillDamage or with a vital part lost: Damage reports
// at least KillDamage for that (the game's Injury >= MaxHP or VitalLost).
// An animal plan's body dies only when its wounds and lost parts reach
// killAt (the animal's MaxHP: Injury >= MaxHP); a lost vital part or an
// organ failure does not kill an animal, so it does not stop a blow.
func bodyDead(b *Body) bool {
	if planIndex(b) == Humanlike {
		return b.Damage() >= killAt(b)
	}
	return lossDamage(b) >= killAt(b)
}

// splits reports s a multi-part rule (planSplit has a plan for it).
func splits(s Split) bool { return s > SplitNone && s < numSplits }

// pctOf is pct % of dmg, rounded down but at least 1 for any damage, and
// 0 for none. original tuning: the wiki gives no rounding; the floor
// matches planBlunt's, and the minimum keeps a small blow's share alive.
func pctOf(dmg, pct int) int {
	if dmg <= 0 {
		return 0
	}
	return max(dmg*pct/100, 1)
}

func (sh *shares) add(p Part, dmg int) {
	if sh.n < len(sh.s) {
		sh.s[sh.n] = share{p, dmg}
		sh.n++
	}
}

// planSplit is the shares a blow of dmg with split s first striking p
// deals, drawing with roll in the rule's order: one share of dmg on p
// for no split.
func planSplit(b *Body, p Part, dmg int, s Split, roll func(int) int) shares {
	switch s {
	case SplitCut:
		return planCut(b, p, dmg, roll)
	case SplitBlunt:
		return planBlunt(b, p, dmg, roll)
	case SplitScratch:
		return planScratch(b, p, dmg, roll)
	case SplitStab, SplitPoke, SplitRangedStab:
		return planStab(b, p, dmg, stabRules[s], roll)
	case SplitBullet:
		return planBullet(b, p, dmg)
	}
	var sh shares
	sh.add(p, dmg)
	return sh
}

// dealPlaced deals a placed blow's share after the first: a part whose
// loss would kill (LossKills, judged now, as the earlier shares left the
// body) takes at most what leaves it 1 hit point and the rest is lost;
// any other part is struck as Strike strikes it, its overkill judged on
// its own share. A missing part takes nothing.
func dealPlaced(b *Body, p Part, dmg int, kind WoundKind, lo, hi int, roll func(int) int, now uint64) {
	if p >= b.numParts() || b.isMissing(p) || dmg <= 0 {
		return
	}
	if room := b.maxHP(p) - b.partDmg(p) - 1; dmg > room && b.LossKills(p) {
		b.hurtPart(p, room, kind, now) // hurtPart takes nothing for room <= 0
		return
	}
	Strike(b, p, dmg, kind, lo, hi, roll, now)
}

// dealSpread deals an unplaced blow's share as Spread deals a pick: a
// part whose loss would kill takes at most what leaves it 1 hit point.
// It returns what Damage did not rise by (a missing part's share, a
// capped share, what a lost part could not hold), for the caller to
// spill.
func dealSpread(b *Body, p Part, dmg int, kind WoundKind, now uint64) int {
	if p >= b.numParts() || b.isMissing(p) || dmg <= 0 {
		return max(dmg, 0)
	}
	hit := dmg
	if room := b.maxHP(p) - b.partDmg(p) - 1; hit > room && b.LossKills(p) {
		hit = max(room, 0)
	}
	before := b.Damage()
	b.hurtPart(p, hit, kind, now)
	return dmg - (b.Damage() - before)
}

// StrikeBlow wounds b's part p with a placed blow of dmg of kind and split
// s (CBT 3.4). No split is Strike exactly. A split blow first draws its
// plan (planSplit), then deals the plan's first share through Strike
// (overkill in window lo..hi may destroy its part, a vital part too, as
// CBT-1 does), and every other share in order by dealPlaced: the
// overkill draws come last. Once a share leaves the body dead (bodyDead:
// as the game kills a pawn or an animal) the shares after it are not
// dealt. An invalid or missing part, or no damage, takes nothing and
// draws nothing.
//
// It reports the first share's part destroyed by that share (Strike's
// report) and nothing else. That part is p unless the plan moved the
// blow (a stab retargeted inside, a scratch that did not pick p). When a
// later share destroys an ancestor of the first share's part (HandL
// first, ArmL an extra) the first part goes with it, yet the report stays
// false.
func StrikeBlow(b *Body, p Part, dmg int, kind WoundKind, s Split, lo, hi int, roll func(int) int, now uint64) bool {
	if p >= b.numParts() || b.isMissing(p) || dmg <= 0 {
		return false
	}
	if !splits(s) {
		return Strike(b, p, dmg, kind, lo, hi, roll, now)
	}
	sh := planSplit(b, p, dmg, s, roll)
	gone := Strike(b, sh.s[0].part, sh.s[0].dmg, kind, lo, hi, roll, now)
	for _, x := range sh.s[1:sh.n] {
		if bodyDead(b) {
			break
		}
		dealPlaced(b, x.part, x.dmg, kind, lo, hi, roll, now)
	}
	return gone
}
