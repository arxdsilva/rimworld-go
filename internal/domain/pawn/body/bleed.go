package body

// Bleeding and blood loss (MED-01). Every open cut, bite, scratch or
// surgical wound bleeds in proportion to its damage until it is tended;
// the bleeding fills BloodLoss (permille of the body's blood), which only
// drains back once nothing bleeds. Blood loss lowers consciousness in
// stages and kills at 1000.

// Bleed and blood loss constants, in permille.
const (
	// BleedPerHP is how much blood one point of open wound damage loses
	// per day: a 10 damage cut bleeds 600 (60 % a day). wiki: Injury
	// (bleeding = damage × body size factor; 6 % per HP for a human).
	BleedPerHP = 60
	// HeartBleedMul multiplies a wound on the heart. wiki: Injury.
	HeartBleedMul = 5
	// StumpBleedMul: a part torn off bleeds as a wound of twice its
	// MaxHP until tended. wiki: Injury.
	StumpBleedMul = 2
	// BloodRegenPerDay is the blood a body makes back per day while
	// nothing bleeds. wiki: Blood loss.
	BloodRegenPerDay = 333
	// BloodLossDeath kills. wiki: Blood loss.
	BloodLossDeath = 1000
	// bloodStageBand is how far blood loss must fall below a stage's
	// threshold to leave it again (hysteresis; original tuning).
	bloodStageBand = 50
	// BloodStageExtreme is the last stage: consciousness capped.
	BloodStageExtreme = 4
	// extremeConsciousnessCap caps consciousness at the extreme stage.
	// wiki: Blood loss.
	extremeConsciousnessCap = 100
)

// bloodStages are the blood loss each stage starts at and what it takes
// off consciousness (minor, moderate, severe, extreme). wiki: Blood loss.
var bloodStages = [BloodStageExtreme]struct{ At, Cons int32 }{
	{150, 100}, {300, 200}, {450, 400}, {600, 400},
}

// bloodStageLabels name the stages for the Health tab (index 0: none).
var bloodStageLabels = [BloodStageExtreme + 1]string{"", "minor", "moderate", "severe", "extreme"}

// BloodStageLabel names stage s ("" for none or an unknown stage).
func BloodStageLabel(s uint8) string {
	if int(s) >= len(bloodStageLabels) {
		return ""
	}
	return bloodStageLabels[s]
}

// bleeds reports a wound kind that bleeds while open (bruises and burns
// never do). wiki: Injury.
func bleeds(k WoundKind) bool {
	switch k {
	case Cut, Bite, Scratch, Surgical, Stab, Gunshot:
		return true
	}
	return false
}

// BleedRate is how fast w bleeds, permille of blood per day: 0 once
// tended, for a scar, and for bruises and burns.
func BleedRate(w Wound) int32 { return bleedRateAt(w, Heart) }

// bleedRate is BleedRate on this body: its plan's heart bleeds faster.
func (b *Body) bleedRate(w Wound) int32 { return bleedRateAt(w, planOf(b.Anatomy).heart) }

// bleedRateAt is BleedRate with heart the part that bleeds faster.
func bleedRateAt(w Wound, heart Part) int32 {
	if w.Tended || w.Permanent || w.Dmg <= 0 || !bleeds(w.Kind) {
		return 0
	}
	r := int32(w.Dmg) * BleedPerHP
	if w.Part == heart {
		r *= HeartBleedMul
	}
	return r
}

// stumpBleed is how fast a part torn off bleeds until tended (permille
// per day).
func (b *Body) stumpBleed(p Part) int32 { return int32(b.maxHP(p)) * StumpBleedMul * BleedPerHP }

// BleedPerDay is the body's total bleeding, permille of blood per day:
// its open wounds and fresh stumps, scaled by blood pumping (MED-05).
func (b *Body) BleedPerDay() int32 {
	var n int32
	for _, w := range b.Wounds {
		n += b.bleedRate(w)
	}
	if b.Fresh != 0 {
		for p := Part(0); p < b.numParts(); p++ {
			if b.Fresh&bit(p) != 0 {
				n += b.stumpBleed(p)
			}
		}
	}
	return b.pumped(n)
}

// pumped scales bleeding n by blood pumping (MED-05): a weak heart
// bleeds slower; a pump heart no faster than a healthy one (original
// tuning: the plan's "pumping scales bleed"). A positive rate never
// rounds down to 0.
func (b *Body) pumped(n int32) int32 {
	if n <= 0 {
		return n
	}
	pump := b.pumping(DefOfCond)
	if pump >= 1000 {
		return n
	}
	return max(n*pump/1000, 1)
}

// PartBleedPerDay is the bleeding of p: its wounds, or its stump while
// fresh (permille per day).
func (b *Body) PartBleedPerDay(p Part) int32 {
	if p >= b.numParts() {
		return 0
	}
	var n int32
	for _, w := range b.Wounds {
		if w.Part == p {
			n += b.bleedRate(w)
		}
	}
	if b.Fresh&bit(p) != 0 {
		n += b.stumpBleed(p)
	}
	return b.pumped(n)
}

// FreshStump reports p torn off and not yet tended.
func (b *Body) FreshStump(p Part) bool { return p < b.numParts() && b.Fresh&bit(p) != 0 }

// perHour is a daily permille rate over one hour, rounded to nearest and
// never below 1 for a positive rate (so the slowest bleed still counts).
func perHour(perDay int32) int32 {
	if perDay <= 0 {
		return 0
	}
	return max((perDay+12)/24, 1)
}

// StepBlood advances blood loss by hours: each hour adds the bleeding's
// share, or, while nothing bleeds, drains the regeneration's share. Loss
// stays within 0..BloodLossDeath, and the stage follows it.
func (b *Body) StepBlood(hours int) {
	for range hours {
		if bleed := b.BleedPerDay(); bleed > 0 {
			b.BloodLoss = min(b.BloodLoss+perHour(bleed), BloodLossDeath)
		} else if b.BloodLoss > 0 {
			b.BloodLoss = max(b.BloodLoss-perHour(BloodRegenPerDay), 0)
		}
		b.BloodStage = BloodStage(b.BloodLoss, b.BloodStage)
	}
}

// BloodStage is the blood loss stage for loss, given the previous stage
// prev: a stage is entered at its threshold and left only once loss falls
// bloodStageBand below it, so loss hovering at a threshold does not flap.
func BloodStage(loss int32, prev uint8) uint8 {
	s := min(prev, BloodStageExtreme)
	for s < BloodStageExtreme && loss >= bloodStages[s].At {
		s++
	}
	for s > 0 && loss < bloodStages[s-1].At-bloodStageBand {
		s--
	}
	return s
}

// bloodStage is the body's current blood loss stage.
func (b *Body) bloodStage() uint8 { return BloodStage(b.BloodLoss, b.BloodStage) }

// bloodConsciousness applies the blood loss stage to consciousness v.
func (b *Body) bloodConsciousness(v int32) int32 {
	s := b.bloodStage()
	if s == 0 {
		return v
	}
	v -= bloodStages[s-1].Cons
	if s >= BloodStageExtreme {
		v = min(v, extremeConsciousnessCap)
	}
	return max(v, 0)
}

// BleedDeathHours is how many hours of bleeding at the current rate are
// left before blood loss kills (-1: not bleeding).
func (b *Body) BleedDeathHours() int {
	bleed := b.BleedPerDay()
	if bleed <= 0 {
		return -1
	}
	left := max(BloodLossDeath-b.BloodLoss, 0)
	h := perHour(bleed)
	return int((left + h - 1) / h)
}
