package body

import "rimworld-go/internal/domain/economy/item"

// Capacity is a bodily ability derived from the parts (append only).
type Capacity uint8

const (
	Consciousness Capacity = iota
	Moving
	Manipulation
	Sight
	Hearing
	Talking
	Breathing
	NumCaps
)

// Implant efficiencies in permille.
const (
	PegLegEff         = 600
	ProstheticLegEff  = 850
	ProstheticArmEff  = 850
	WoodenHandEff     = 500
	DentureEff        = 800
	HearingTrumpetEff = 600
	ProstheticFootEff = 750
)

// ImplantEfficiency is the permille a kind restores (0: not an implant).
func ImplantEfficiency(k item.Kind) int32 {
	switch k {
	case item.PegLeg:
		return PegLegEff
	case item.ProstheticLeg:
		return ProstheticLegEff
	case item.ProstheticArm:
		return ProstheticArmEff
	case item.WoodenHand:
		return WoodenHandEff
	case item.Denture:
		return DentureEff
	case item.HearingTrumpet:
		return HearingTrumpetEff
	case item.ProstheticFoot:
		return ProstheticFootEff
	}
	return 0
}

// eff is the permille a part works at.
func (b *Body) eff(p Part) int32 {
	if b.isMissing(p) {
		if b.fitted(p) {
			return ImplantEfficiency(b.Implant(p))
		}
		if b.replaced(p) {
			return 1000 // part of a prosthetic limb
		}
		return 0
	}
	max := defs[p].MaxHP
	d := b.partDmg(p)
	if d > max {
		d = max
	}
	return int32((max - d) * 1000 / max)
}

func avg(a, b int32) int32 { return (a + b) / 2 }

// Capacity is capacity c in permille (1000: healthy), with the current
// stage of every condition folded in.
func (b *Body) Capacity(c Capacity) int32 { return b.capacity(c, DefOfCond) }

// capacity is Capacity with condition definitions from look.
func (b *Body) capacity(c Capacity, look condLookup) int32 {
	cons := b.consciousness(look)
	var v int32
	switch c {
	case Consciousness:
		return cons
	case Moving:
		v = min32(avg(b.eff(LegL)*b.eff(FootL)/1000, b.eff(LegR)*b.eff(FootR)/1000), cons)
	case Manipulation:
		v = min32(avg(b.eff(ArmL)*b.eff(HandL)/1000, b.eff(ArmR)*b.eff(HandR)/1000), cons)
	case Sight:
		v = avg(b.eff(EyeL), b.eff(EyeR))
	case Hearing:
		v = avg(b.eff(EarL), b.eff(EarR))
	case Talking:
		v = b.eff(Jaw) * cons / 1000
	case Breathing:
		v = avg(b.eff(LungL), b.eff(LungR))
	default:
		v = 1000
	}
	return b.condCap(c, v, look)
}

// consciousness is brain function less half the pain, less the blood
// loss stage (capped at the extreme stage), then the conditions.
func (b *Body) consciousness(look condLookup) int32 {
	v := b.eff(Brain) - b.pain(look)/2
	if v < 0 {
		v = 0
	}
	v = b.bloodConsciousness(v) // bleed.go (MED-01)
	return b.condCap(Consciousness, v, look)
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// Pain per point of wound damage, in tenths of a permille: 12.5 ‰ per HP
// whether the wound is open or tended (tending stops bleeding, not pain).
// wiki: Injury. A fresh stump hurts as a wound of the part's MaxHP until
// tended (original tuning on the wiki's fresh missing part); a scar keeps
// 1 ‰ per HP (original tuning).
const (
	painPerHPx10  = 125
	scarPainPerHP = 1
)

// Pain is the body's pain in permille: 12.5 per point of open or tended
// wound damage, 1 per point of scar; conditions add their current stage's
// pain.
func (b *Body) Pain() int32 { return b.pain(DefOfCond) }

// pain is Pain with condition definitions from look.
func (b *Body) pain(look condLookup) int32 {
	var hurt, scars int32
	for _, w := range b.Wounds {
		if w.Permanent {
			scars += int32(w.Dmg)
			continue
		}
		hurt += int32(w.Dmg)
	}
	if b.Fresh != 0 {
		for p := Part(0); p < NumParts; p++ {
			if b.Fresh&bit(p) != 0 {
				hurt += int32(defs[p].MaxHP)
			}
		}
	}
	n := hurt*painPerHPx10/10 + scars*scarPainPerHP
	n += b.condPain(look)
	if n > 1000 {
		return 1000
	}
	if n < 0 {
		return 0
	}
	return n
}
