package body

import "rimworld-go/internal/domain/combat/damage"

// woundOf is the wound each damage form leaves. wiki: Injury (bullets
// leave gunshots, points and arrows stabs, blunt blows and crushing
// bruises; flame and burns burn; a bomb's shrapnel cuts). EMP and stun
// leave none (Bruise is never dealt for them: the caller skips them).
var woundOf = [damage.NumForms]WoundKind{
	damage.FormBlunt: Bruise, damage.FormCut: Cut, damage.FormStab: Stab, damage.FormBullet: Gunshot,
	damage.FormArrow: Stab, damage.FormScratch: Scratch, damage.FormBite: Bite, damage.FormCrush: Bruise,
	damage.FormBurn: Burn, damage.FormFlame: Burn, damage.FormBomb: Cut, damage.FormEMP: Bruise, damage.FormStun: Bruise,
}

// WoundOf is the wound form f leaves (Bruise for an unknown form).
func WoundOf(f damage.Form) WoundKind {
	if f >= damage.NumForms {
		return Bruise
	}
	return woundOf[f]
}

// Strike wounds b's part p with a hit of dmg of kind (CBT-1). A hit p's hit
// points can hold wounds it; one that overkills it destroys it (with
// what lies under it) at damage.DestroyChance of the overkill, in
// permille of p's max hit points, through window lo..hi, rolled with roll
// only when the chance is neither none nor certain; otherwise p is left
// at 1 hit point and the rest of the hit is lost. A missing part takes
// nothing. It reports p destroyed. wiki: Injury
func Strike(b *Body, p Part, dmg int, kind WoundKind, lo, hi int, roll func(int) int, now uint64) bool {
	if p >= b.numParts() || b.isMissing(p) || dmg <= 0 {
		return false
	}
	mx := b.maxHP(p)
	left := mx - b.partDmg(p)
	if dmg < left {
		b.hurtPart(p, dmg, kind, now)
		return false
	}
	c := damage.DestroyChance((dmg-left)*1000/max(mx, 1), lo, hi)
	if c >= 1000 || c > 0 && roll(1000) < c {
		b.hurtPart(p, left, kind, now)
		return true
	}
	b.hurtPart(p, left-1, kind, now)
	return false
}
