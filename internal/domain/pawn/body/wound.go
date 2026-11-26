// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "rimworld-go/internal/domain/economy/item"

// WoundKind is how a wound was made (append only, saved).
type WoundKind uint8

const (
	Cut WoundKind = iota
	Bruise
	Bite
	Scratch
	Burn
	Surgical
	// CBT-1 (append only): a point's wound and a gunshot (an arrow
	// stabs), both bleeding as a cut does. wiki: Injury
	Stab
	Gunshot
	// Frostbite (TEMP-3, cond_frostbite.go): cold damage on an extremity;
	// it never bleeds. wiki: Frostbite
	Frostbite
	// Added after the prior save ordinals; caused only on the corpse.
	CorrosiveBurn
)

// scarDmg is the residual damage a scar keeps.
const scarDmg = 1

// Wound is damage on one part; Permanent wounds are scars.
//
// MED-00 appends the medical fields, all zero-safe (an older save and a
// fresh wound read them as zero): TendQ is the tend quality in permille
// (0: untended or tended before quality existed), TendAt the tick it was
// tended, InfectAt the tick its infection roll is due (0: none due) and
// Bleed its cached bleed rate (permille of blood per day).
//
// MED-02 appends HealAcc, the healing accumulated towards the next HP
// (milli-HP × hours a day, tend.go), and Size, the damage the wound was
// made with (scar odds; 0 for wounds older than MED-02).
type Wound struct {
	Part      Part
	Dmg       int
	Kind      WoundKind
	Tended    bool
	At        uint64
	Permanent bool
	TendQ     int16
	TendAt    uint64
	InfectAt  uint64
	Bleed     int32
	HealAcc   int32
	Size      int32
}

// Replacement is an artificial body part. Damage is measured against the
// replaced part's max HP; a zero value means no replacement.
type Replacement struct {
	Kind    item.Kind
	Damage  int16
	HealAcc int32
}

// Body is a body's authoritative health state. Missing tracks lost natural
// tissue; Replacements keep artificial parts damageable independently of
// wounds. Implants are attached to existing parts without replacing them.
type Body struct {
	Wounds       []Wound
	Missing      uint32
	Replacements []Replacement
	Implants     []Implant
	// Fitted is read only by gob when loading pre-v18 animal bodies embedded
	// directly in world saves. Runtime code uses Replacements.
	Fitted     []item.Kind
	BloodLoss  int32
	Conds      []Condition
	BloodStage uint8
	Fresh      uint32
	Anatomy    Anatomy
	Scale      int32
}

// ReplacementAt reports the artificial part fitted at p, if any.
func (b *Body) ReplacementAt(p Part) item.Kind {
	if int(p) < len(b.Replacements) {
		return b.Replacements[p].Kind
	}
	return 0
}

func replacementDamage(b *Body, p Part) int32 {
	if int(p) < len(b.Replacements) {
		return int32(b.Replacements[p].Damage)
	}
	return 0
}

// SetReplacement changes the installed artificial part without surgery checks.
// An existing replacement's damage is reset when a new one is fitted.
func (b *Body) SetReplacement(p Part, k item.Kind) {
	if p >= b.numParts() {
		return
	}
	if k != 0 {
		b.Fresh &^= bit(p)
	}
	if int(p) >= len(b.Replacements) {
		if k == 0 {
			return
		}
		b.Replacements = append(b.Replacements, make([]Replacement, int(p)+1-len(b.Replacements))...)
	}
	b.Replacements[p] = Replacement{Kind: k}
	n := len(b.Replacements)
	for n > 0 && b.Replacements[n-1].Kind == 0 {
		n--
	}
	if n == 0 {
		b.Replacements = nil
	} else {
		b.Replacements = b.Replacements[:n]
	}
}

// ReplacementsFrom converts the pre-v18 part-indexed item array at the save
// boundary. The rest of the simulation operates only on Replacements.
func ReplacementsFrom(a []item.Kind) []Replacement {
	var b Body
	for p, k := range a {
		if k != 0 {
			b.SetReplacement(Part(p), k)
		}
	}
	return b.Replacements
}

func (b *Body) hasReplacement(p Part) bool {
	return b.ReplacementAt(p).IsReplacement()
}

// partDmg is the wound damage on p.
func (b *Body) partDmg(p Part) int {
	n := 0
	for _, w := range b.Wounds {
		if w.Part == p {
			n += w.Dmg
		}
	}
	return n
}

// Hurt deals dmg of kind to a part chosen with roll; it returns the part.
// A part whose damage reaches its MaxHP is destroyed with its children.
func (b *Body) Hurt(roll func(int) int, dmg int, kind WoundKind, now uint64) Part {
	p, ok := b.PickPart(roll)
	if !ok || dmg <= 0 {
		return p
	}
	b.hurtPart(p, dmg, kind, now)
	return p
}

// hurtPart wounds p, destroying it when its damage reaches MaxHP.
func (b *Body) hurtPart(p Part, dmg int, kind WoundKind, now uint64) {
	if dmg <= 0 {
		return
	}
	if b.isMissing(p) {
		if !b.hasReplacement(p) {
			return
		}
		r := &b.Replacements[p]
		if int(r.Damage)+dmg >= b.maxHP(p) {
			b.destroy(p)
		} else {
			r.Damage += int16(dmg)
		}
		return
	}
	if b.partDmg(p)+dmg >= b.maxHP(p) {
		b.destroy(p)
		if kind != Burn { // a burn sears its stump shut
			b.Fresh |= bit(p)
		}
		return
	}
	b.Wounds = append(b.Wounds, Wound{Part: p, Dmg: dmg, Kind: kind, At: now, Size: int32(dmg)})
}

// destroy removes p and everything under it, with their wounds and implants.
func (b *Body) destroy(p Part) {
	for q := Part(0); q < b.numParts(); q++ {
		if b.under(q, p) {
			b.Missing |= bit(q)
			b.Fresh &^= bit(q) // a stump below is part of this loss
			b.SetReplacement(q, 0)
		}
	}
	dropImplantsUnder(b, p)
	kept := b.Wounds[:0]
	for _, w := range b.Wounds {
		if !b.under(w.Part, p) {
			kept = append(kept, w)
		}
	}
	b.Wounds = kept
}

// VitalLost reports a vital part (brain, heart, torso) destroyed with no
// implant standing in (a pump or valve heart keeps the patient alive), or
// an organ failure (MED-05, OrganFailure: a capacity the body cannot live
// without at 0).
func (b *Body) VitalLost() bool {
	t := b.table()
	for p := range t {
		if t[p].Vital && b.isMissing(Part(p)) && !b.hasReplacement(Part(p)) {
			return true
		}
	}
	return b.OrganFailure()
}

// replaced reports p covered by an implant on itself or an ancestor.
func (b *Body) coveredByReplacement(p Part) bool {
	if b.Replacements == nil {
		return false
	}
	for q := Part(0); q < b.numParts(); q++ {
		if b.hasReplacement(q) && b.under(p, q) {
			return true
		}
	}
	return false
}

// Damage is the total wound damage plus the value of lost parts (it feeds
// health.Record.Injury, 0..100, downed at 70). Losing a vital part reports
// at least KillDamage.
func (b *Body) Damage() int {
	n := lossDamage(b)
	if b.VitalLost() && n < KillDamage {
		return KillDamage
	}
	return n
}

// lossDamage is Damage without its vital-loss floor: the wound damage plus
// the hit points of the lost parts not counted with a lost parent. It is a
// free function so Body gains no method (internal/arch/solid_budget.txt).
func lossDamage(b *Body) int {
	n := 0
	for _, w := range b.Wounds {
		n += w.Dmg
	}
	for _, r := range b.Replacements {
		n += int(r.Damage)
	}
	t := b.table()
	for p := Part(0); p < Part(len(t)); p++ {
		if !b.isMissing(p) || b.coveredByReplacement(p) {
			continue
		}
		if q := t[p].Parent; q != p && b.isMissing(q) {
			continue // counted with its parent
		}
		n += b.maxHP(p)
	}
	return n
}
