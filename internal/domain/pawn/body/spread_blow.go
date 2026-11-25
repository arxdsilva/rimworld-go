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
// spill, and it returns what Damage rose by. An owner whose death is
// counted outside the body (an animal's Injury) uses SpreadBlowWithin.
func SpreadBlow(b *Body, dmg int, kind WoundKind, s Split, intn func(int) int, now uint64) int {
	return spreadBlow(b, dmg, kind, s, 0, true, intn, now)
}

// SpreadBlowWithin uses the owner's hit points, not the anatomical
// aggregate (an animal's MaxHP - Injury). Each newly placed share adds
// its accepted damage, including a part's last HP if destroyed. Earlier
// wounds removed with a lost ancestor are not new damage or spill.
// Stop after the share or spill placement reaching room; do not count
// unplaced damage as lethal. Whole-share semantics and the bounded
// root fallback remain as in Spread. Unsplit blows keep the existing
// Spread contract. The native owner supplies room > 0.
func SpreadBlowWithin(b *Body, dmg int, kind WoundKind, s Split, room int, intn func(int) int, now uint64) int {
	return spreadBlow(b, dmg, kind, s, room, false, intn, now)
}

// spreadBlow shares the native part selection and plan. Body-owned
// callers retain their anatomical Damage contract; owners with scalar
// HP use the separate accounting of newly applied shares.
func spreadBlow(b *Body, dmg int, kind WoundKind, s Split, room int, byBody bool, intn func(int) int, now uint64) int {
	if !splits(s) || dmg <= 0 {
		Spread(b, dmg, kind, intn, now)
		return dmg
	}
	p, ok := b.PickPart(intn)
	if !ok {
		if !byBody {
			return spreadWithinRemainder(b, dmg, 0, room, kind, intn, now)
		}
		Spread(b, dmg, kind, intn, now)
		return dmg
	}
	if b.hasReplacement(p) && byBody {
		if left := dealSpread(b, p, dmg, kind, now); left > 0 {
			Spread(b, left, kind, intn, now)
		}
		return dmg
	}
	var sh shares
	if b.hasReplacement(p) {
		sh.add(p, dmg)
	} else {
		sh = planSplit(b, p, dmg, s, intn)
	}
	if !byBody {
		return dealWithin(b, sh, room, kind, intn, now)
	}
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

// dealWithin accounts for actual placements, keeping pending spill
// separate from the owner's injury. An exhausted owner receives no more.
func dealWithin(b *Body, sh shares, room int, kind WoundKind, intn func(int) int, now uint64) int {
	applied, spill := 0, 0
	for _, x := range sh.s[:sh.n] {
		hit := applyRoomShare(b, x.part, x.dmg, kind, now)
		applied += hit
		spill += x.dmg - hit
		if applied >= room {
			return applied
		}
	}
	return spreadWithinRemainder(b, spill, applied, room, kind, intn, now)
}

// applyRoomShare returns the new damage physically accepted by this
// part, before any historical child wounds disappear through destruction.
func applyRoomShare(b *Body, p Part, dmg int, kind WoundKind, now uint64) int {
	if dmg <= 0 || p >= b.numParts() || b.isMissing(p) && !b.hasReplacement(p) {
		return 0
	}
	hp := remainingHP(b, p)
	if dmg >= hp && b.LossKills(p) {
		hp--
	}
	hit := min(dmg, max(0, hp))
	b.hurtPart(p, hit, kind, now)
	return hit
}

func spreadWithinRemainder(b *Body, left, applied, room int, kind WoundKind, intn func(int) int, now uint64) int {
	for tries := 0; left > 0 && tries < MaxSpill; tries++ {
		p, ok := b.PickPart(intn)
		if !ok {
			break
		}
		hit := applyRoomShare(b, p, left, kind, now)
		applied += hit
		left -= hit
		if applied >= room {
			return applied
		}
	}
	if left > 0 {
		b.Wounds = append(b.Wounds, Wound{Part: b.Root(), Dmg: left, Kind: kind, At: now, Size: int32(left)})
		applied += left
	}
	return applied
}
