// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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

// weighted is v scaled by a shortfall in capacity c of weight w
// (permille): v × (1 − w × (1 − c)), c counted at most at 1000.
func weighted(v, c, w int32) int32 {
	c = max(min(c, 1000), 0)
	return v * (1000 - w*(1000-c)/1000) / 1000
}

// weightedUncapped keeps gains above 100% for Moving. Consciousness
// uses weighted instead because its blood and breath factors cap at 100%.
// wiki: Blood Pumping, Breathing, Moving.
func weightedUncapped(v, c, w int32) int32 {
	c = max(c, 0)
	return v * (1000 + w*(c-1000)/1000) / 1000
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
	if b.Anatomy != Humanlike {
		return b.condCap(BloodPumping, b.planRaw(BloodPumping), look)
	}
	return b.condCap(BloodPumping, b.eff(Heart), look)
}

// filtration is the kidneys' (500 each) and the liver's work with the
// conditions folded in. wiki: Blood filtration.
func (b *Body) filtration(look condLookup) int32 {
	if b.Anatomy != Humanlike {
		return b.condCap(BloodFiltration, b.planRaw(BloodFiltration), look)
	}
	return b.condCap(BloodFiltration, avg(b.eff(KidneyL), b.eff(KidneyR))*b.eff(Liver)/1000, look)
}

// eff is the permille a part works at.
func (b *Body) eff(p Part) int32 {
	if b.isMissing(p) {
		if b.hasReplacement(p) {
			v := ImplantEfficiency(b.ReplacementAt(p))
			return v * (int32(b.maxHP(p)) - replacementDamage(b, p)) / int32(b.maxHP(p))
		}
		if b.coveredByReplacement(p) {
			return 1000
		}
		return 0
	}
	maxHP := b.maxHP(p)
	damage := min(b.partDmg(p), maxHP)
	efficiency := int32((maxHP - damage) * 1000 / maxHP)
	offset := int32(0)
	for _, c := range b.Conds {
		if c.Part != p {
			continue
		}
		switch c.Kind {
		case CondCataract, CondHearingLoss:
			offset -= 500 // wiki: Hediffs (Part Efficiency Offset -50%)
		case CondAsthma:
			if d := b.table()[p]; d.HasCap && d.Cap == Breathing {
				stages := [...]int32{100, 300, 500} // wiki: Asthma
				offset -= stages[min(int(c.Stage), len(stages)-1)]
			}
		case CondAlzheimers:
			if p == Brain {
				stages := [...]int32{50, 100, 150, 200} // wiki: Alzheimer's
				offset -= stages[min(int(c.Stage), len(stages)-1)]
			}
		case CondCarcinoma:
			stages := [...]int32{100, 250, 500, 800, 900} // wiki: Carcinoma
			offset -= stages[min(int(c.Stage), len(stages)-1)]
		}
	}
	return max(0, efficiency+offset)
}

func avg(a, b int32) int32 { return (a + b) / 2 }

// Capacity is capacity c in permille (1000: healthy), with the current
// stage of every condition folded in.
func (b *Body) Capacity(c Capacity) int32 { return b.capacity(c, DefOfCond) }

// capacity is Capacity with condition definitions from look.
func (b *Body) capacity(c Capacity, look condLookup) int32 {
	if b.Anatomy != Humanlike {
		return b.planCapacity(c, look) // anatomy_plan.go (MED-13)
	}
	var v int32
	switch c {
	case Consciousness:
		return b.consciousness(look)
	case Moving: // the spine carries the legs: without it, no moving
		v = limbs(avg(b.eff(LegL)*b.eff(FootL)/1000, b.eff(LegR)*b.eff(FootR)/1000), b.consciousness(look))
		v = weightedUncapped(v*b.eff(Spine)/1000, b.pumping(look), pumpingWeight)
		v = weightedUncapped(v, b.capacity(Breathing, look), 200) // wiki: Breathing, Moving (20% weight)
		// wiki: Power claw (moving -8 percentage points).
		if len(b.Replacements) > int(HandL) && b.Replacements[HandL].Kind == item.ServoClaw {
			v -= 80
		}
		if len(b.Replacements) > int(HandR) && b.Replacements[HandR].Kind == item.ServoClaw {
			v -= 80
		}
		v = max(v, 0)
	case Manipulation:
		v = limbs(avg(b.eff(ArmL)*b.eff(HandL)/1000, b.eff(ArmR)*b.eff(HandR)/1000), b.consciousness(look))
	case Sight: // wiki: Sight (better eye contributes 75%).
		a, c := b.eff(EyeL), b.eff(EyeR)
		v = (3*max(a, c) + min(a, c)) / 4
	case Hearing: // wiki: Hearing (better ear contributes 75%).
		a, c := b.eff(EarL), b.eff(EarR)
		v = (3*max(a, c) + min(a, c)) / 4
	case Talking:
		v = b.eff(Jaw) * b.eff(Tongue) / 1000 * b.consciousness(look) / 1000
	case Breathing:
		v = avg(b.eff(LungL), b.eff(LungR))
	case BloodPumping:
		return b.pumping(look)
	case BloodFiltration:
		return b.filtration(look)
	case Digestion:
		v = avg(b.eff(Stomach), b.eff(Liver)) // wiki: Digestion (50% stomach, 50% liver)
	default:
		v = 1000
	}
	return b.condCap(c, v, look)
}

// consciousness is brain function less half the pain, weighed by blood
// pumping and filtration (MED-05), less the blood loss stage (capped at
// the extreme stage), then the conditions.
func (b *Body) consciousness(look condLookup) int32 {
	brain := Brain
	if b.Anatomy != Humanlike {
		brain = planOf(b.Anatomy).brain
	}
	v := b.eff(brain)
	if HasImplant(b, Brain, item.BlissCircuit) { // wiki: Joywire
		v -= 200
	}
	pain := b.pain(look)
	v = max(0, v) * (1000 - max(0, pain-100)*4/9) / 1000 // wiki: Consciousness, Pain
	v = weighted(v, b.pumping(look), pumpingWeight)
	v = weighted(v, b.capacity(Breathing, look), 200) // wiki: Breathing (20% consciousness)
	v = weighted(v, b.filtration(look), filtrationWeight)
	v = b.bloodConsciousness(v) // bleed.go (MED-01)
	return b.condCap(Consciousness, v, look)
}

// EatingSpeed is the eating-time multiplier in permille.
// wiki: Eating, Eating Speed. Eating has 95% stat weight and
// Manipulation has 30%; food poisoning is a post-factor.
func EatingSpeed(b *Body) int32 {
	if b == nil || b.Anatomy != Humanlike {
		return 1000
	}
	eating := max(int32(100), b.eff(Jaw)*b.eff(Tongue)/1000*b.Capacity(Consciousness)/1000)
	eatingFactor := 1000 + (eating-1000)*950/1000
	manipulationFactor := 1000 + (b.Capacity(Manipulation)-1000)*300/1000
	v := eatingFactor * manipulationFactor / 1000
	v = v * FoodPoisonEatSpeed(b) / 1000
	return max(150, v)
}

// RestRateMultiplier is the organ-dependent part of rest recovery.
// wiki: Rest Rate Multiplier (blood pumping, digestion and breathing each
// contribute 30%; multiplicative factors, no cap at 100%).
func RestRateMultiplier(b *Body) int32 {
	if b == nil {
		return 1000
	}
	v := int32(1000)
	for _, c := range [...]Capacity{BloodPumping, Digestion, Breathing} {
		v = weightedUncapped(v, b.Capacity(c), 300)
	}
	return max(v, 50) // wiki: Rest Rate Multiplier (5% minimum)
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

// Pain is the body's pain in permille: per point of open or tended wound
// damage 12.5 (burn 18.75), per point of scar 6.25 (pain.go; wiki: Injury,
// Pain); conditions add their current stage's
// pain, and an acting painkiller halves the sum (MED-07).
func (b *Body) Pain() int32 { return b.pain(DefOfCond) }

// pain is Pain with condition definitions from look.
func (b *Body) pain(look condLookup) int32 {
	if HasImplant(b, Brain, item.NerveDampener) { // wiki: Painstopper
		return 0
	}
	var hurt, burns, scars int32
	for _, w := range b.Wounds {
		switch {
		case w.Permanent:
			scars += int32(w.Dmg)
		case painRate(w.Kind) == painBurn:
			burns += int32(w.Dmg)
		default:
			hurt += int32(w.Dmg)
		}
	}
	if b.Fresh != 0 {
		for p := Part(0); p < b.numParts(); p++ {
			if b.Fresh&bit(p) != 0 {
				hurt += int32(b.maxHP(p))
			}
		}
	}
	if s := b.Scale; s > 0 && s != 1000 { // MED-13: damage against the body's size
		hurt, burns, scars = hurt*1000/s, burns*1000/s, scars*1000/s
	}
	n := (hurt*painStd + burns*painBurn + scars*painScar) / 100
	n += b.condPain(look)
	n = b.painkilled(n) // cond_drug.go (MED-07)
	if c, ok := b.Cond(CondAnesthetic); ok {
		// wiki: Anesthetic: sedated 0, woozy x80%, wearing off x95% pain.
		switch c.Stage {
		case anesthStageSedated:
			n = 0
		case anesthStageWoozy:
			n = n * 800 / 1000
		default:
			n = n * 950 / 1000
		}
	}
	if n > 1000 {
		return 1000
	}
	if n < 0 {
		return 0
	}
	return n
}
