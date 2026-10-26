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
	// MED-05: the heart pumps, the kidneys and the liver filter, the
	// stomach and the liver digest. Any of them (or Breathing) at 0 kills
	// (OrganFailure).
	BloodPumping
	BloodFiltration
	Digestion
	NumCaps
)

// Implant efficiencies in permille. MED-05 fixed the prosthetic arm
// (850 → 500) and the wooden hand (500 → 600) to the wiki and added the
// servo tier.
const (
	PegLegEff         = 600 // wiki: Peg leg
	ProstheticLegEff  = 850 // wiki: Prosthetic leg
	ProstheticArmEff  = 500 // wiki: Prosthetic arm
	WoodenHandEff     = 600 // wiki: Wooden hand
	DentureEff        = 800 // wiki: Denture
	HearingTrumpetEff = 600
	ProstheticFootEff = 750
	ServoEff          = 1250 // wiki: Bionic arm, leg, eye, ear, heart, stomach (servo arm, leg, optic, cochlear, pump heart, synth stomach)
	SpinalBraceEff    = 1000 // wiki: Bionic spine
	ValveHeartEff     = 800  // wiki: Prosthetic heart
)

// Capacity weights (permille): how much of a shortfall in one capacity
// carries into another, RimWorld's weighted capacity impactors.
const (
	// pumpingWeight: blood pumping weighs 0.2 on consciousness and
	// moving. wiki: Blood pumping.
	pumpingWeight = 200
	// filtrationWeight: blood filtration weighs 0.1 on consciousness, so
	// one kidney (filtration 500) costs 50. wiki: Kidney.
	filtrationWeight = 100
	// immunityFiltrationWeight: filtration weighs 0.5 on immunity gain,
	// so one kidney costs 250. wiki: Kidney.
	immunityFiltrationWeight = 500
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
	case item.ServoArm, item.ServoLeg, item.OpticImplant, item.CochlearImplant, item.PumpHeart, item.SynthStomach:
		return ServoEff
	case item.SpinalBrace:
		return SpinalBraceEff
	case item.ValveHeart:
		return ValveHeartEff
	}
	return 0
}

// weighted is v scaled by a shortfall in capacity c of weight w
// (permille): v × (1 − w × (1 − c)), c counted at most at 1000.
func weighted(v, c, w int32) int32 {
	c = max(min(c, 1000), 0)
	return v * (1000 - w*(1000-c)/1000) / 1000
}

// limbs is a limb capacity (moving, manipulation) of raw limb efficiency
// v under consciousness cons: capped at cons, except that what servo
// limbs add above 1000 is kept, scaled by cons.
func limbs(v, cons int32) int32 {
	if v <= 1000 {
		return min32(v, cons)
	}
	return min32(1000, cons) + (v-1000)*cons/1000
}

// pumping is the heart's work with the conditions folded in.
func (b *Body) pumping(look condLookup) int32 {
	return b.condCap(BloodPumping, b.eff(Heart), look)
}

// filtration is the kidneys' (500 each) and the liver's work with the
// conditions folded in. wiki: Blood filtration.
func (b *Body) filtration(look condLookup) int32 {
	return b.condCap(BloodFiltration, avg(b.eff(KidneyL), b.eff(KidneyR))*b.eff(Liver)/1000, look)
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
	var v int32
	switch c {
	case Consciousness:
		return b.consciousness(look)
	case Moving: // the spine carries the legs: without it, no moving
		v = limbs(avg(b.eff(LegL)*b.eff(FootL)/1000, b.eff(LegR)*b.eff(FootR)/1000), b.consciousness(look))
		v = weighted(v*b.eff(Spine)/1000, b.pumping(look), pumpingWeight)
	case Manipulation:
		v = limbs(avg(b.eff(ArmL)*b.eff(HandL)/1000, b.eff(ArmR)*b.eff(HandR)/1000), b.consciousness(look))
	case Sight:
		v = avg(b.eff(EyeL), b.eff(EyeR))
	case Hearing:
		v = avg(b.eff(EarL), b.eff(EarR))
	case Talking:
		v = b.eff(Jaw) * b.consciousness(look) / 1000
	case Breathing:
		v = avg(b.eff(LungL), b.eff(LungR))
	case BloodPumping:
		return b.pumping(look)
	case BloodFiltration:
		return b.filtration(look)
	case Digestion:
		v = b.eff(Stomach) * b.eff(Liver) / 1000
	default:
		v = 1000
	}
	return b.condCap(c, v, look)
}

// consciousness is brain function less half the pain, weighed by blood
// pumping and filtration (MED-05), less the blood loss stage (capped at
// the extreme stage), then the conditions.
func (b *Body) consciousness(look condLookup) int32 {
	v := b.eff(Brain) - b.pain(look)/2
	if v < 0 {
		v = 0
	}
	v = weighted(v, b.pumping(look), pumpingWeight)
	v = weighted(v, b.filtration(look), filtrationWeight)
	v = b.bloodConsciousness(v) // bleed.go (MED-01)
	return b.condCap(Consciousness, v, look)
}

// ImmunityGain is the body's immunity gain factor (permille, 1000:
// healthy): blood filtration weighs 0.5 on it, so one kidney gives 750.
// wiki: Kidney.
func (b *Body) ImmunityGain() int32 {
	return weighted(1000, b.filtration(DefOfCond), immunityFiltrationWeight)
}

// lifeCaps are the capacities whose loss kills: wiki: Blood pumping,
// Blood filtration, Breathing, Digestion.
var lifeCaps = [...]Capacity{BloodPumping, BloodFiltration, Breathing, Digestion}

// OrganFailure reports a capacity the body cannot live without at 0
// (both kidneys, the liver, both lungs or the stomach gone with nothing
// standing in): the pawn dies (health.CauseOrganFailure).
func (b *Body) OrganFailure() bool {
	for _, c := range lifeCaps {
		if b.Capacity(c) <= 0 {
			return true
		}
	}
	return false
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
// pain, and an acting painkiller halves the sum (MED-07).
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
	n = b.painkilled(n) // cond_drug.go (MED-07): a painkiller halves it
	if n > 1000 {
		return 1000
	}
	if n < 0 {
		return 0
	}
	return n
}
