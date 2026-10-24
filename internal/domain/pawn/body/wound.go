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
)

// scarChance is the permille chance a closed cut, bite, scratch or burn
// leaves a permanent scar.
const scarChance = 100

// scarDmg is the residual damage a scar keeps.
const scarDmg = 1

// Wound is damage on one part; Permanent wounds are scars.
type Wound struct {
	Part      Part
	Dmg       int
	Kind      WoundKind
	Tended    bool
	At        uint64
	Permanent bool
}

// Body is one humanlike's wounds, missing parts (a bit per Part) and
// installed implants (saved). A part replaced by an implant stays Missing;
// the implant stands in for it.
type Body struct {
	Wounds   []Wound
	Missing  uint32
	Implants [NumParts]item.Kind
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
	if b.isMissing(p) || dmg <= 0 {
		return
	}
	if b.partDmg(p)+dmg >= defs[p].MaxHP {
		b.destroy(p)
		return
	}
	b.Wounds = append(b.Wounds, Wound{Part: p, Dmg: dmg, Kind: kind, At: now})
}

// destroy removes p and everything under it, with their wounds and implants.
func (b *Body) destroy(p Part) {
	for q := Part(0); q < NumParts; q++ {
		if under(q, p) {
			b.Missing |= bit(q)
			b.Implants[q] = 0
		}
	}
	kept := b.Wounds[:0]
	for _, w := range b.Wounds {
		if !under(w.Part, p) {
			kept = append(kept, w)
		}
	}
	b.Wounds = kept
}

// VitalLost reports a vital part (brain, heart, torso) destroyed.
func (b *Body) VitalLost() bool {
	for p := Part(0); p < NumParts; p++ {
		if defs[p].Vital && b.isMissing(p) {
			return true
		}
	}
	return false
}

// Heal heals amount of damage, smallest open wound first; scarred reports a
// closed wound that left a permanent scar.
func (b *Body) Heal(roll func(int) int, amount int) (scarred bool) {
	for amount > 0 {
		idx := -1
		for i, w := range b.Wounds {
			if w.Permanent {
				continue
			}
			if idx < 0 || w.Dmg < b.Wounds[idx].Dmg {
				idx = i
			}
		}
		if idx < 0 {
			return scarred
		}
		w := &b.Wounds[idx]
		if w.Dmg > amount {
			w.Dmg -= amount
			return scarred
		}
		amount -= w.Dmg
		w.Dmg = 0
		switch w.Kind {
		case Cut, Bite, Scratch, Burn:
			if roll(1000) < scarChance {
				w.Dmg, w.Permanent, w.Tended = scarDmg, true, false
				scarred = true
				continue
			}
		}
		b.Wounds = append(b.Wounds[:idx], b.Wounds[idx+1:]...)
	}
	return scarred
}

// Tend marks every open wound tended and returns how many it changed.
func (b *Body) Tend() int {
	n := 0
	for i := range b.Wounds {
		if !b.Wounds[i].Permanent && !b.Wounds[i].Tended {
			b.Wounds[i].Tended = true
			n++
		}
	}
	return n
}

// replaced reports p covered by an implant on itself or an ancestor.
func (b *Body) replaced(p Part) bool {
	for q := Part(0); q < NumParts; q++ {
		if b.Implants[q].IsImplant() && under(p, q) {
			return true
		}
	}
	return false
}

// Damage is the total wound damage plus the value of lost parts (it feeds
// health.Record.Injury, 0..100, downed at 70). Losing a vital part reports
// at least KillDamage.
func (b *Body) Damage() int {
	n := 0
	for _, w := range b.Wounds {
		n += w.Dmg
	}
	for p := Part(0); p < NumParts; p++ {
		if !b.isMissing(p) || b.replaced(p) {
			continue
		}
		if q := defs[p].Parent; q != p && b.isMissing(q) {
			continue // counted with its parent
		}
		n += defs[p].MaxHP
	}
	if b.VitalLost() && n < KillDamage {
		return KillDamage
	}
	return n
}
