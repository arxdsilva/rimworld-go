package body

// Malnutrition (FOOD-1, wiki: Malnutrition): at 0 % food a pawn grows
// malnourished. Its severity rises by the pawn's own rate, rolled once
// when it sets in (1.51 to 2.27 % an hour), falls at the same rate once
// the pawn has food again, and kills at full severity (the game steps it
// hourly: StepMalnutrition; death and the downing rule are the game's).
// The stages lower consciousness, raise the hunger rate and set the
// Malnourished thought's mood; wounds don't heal while it lasts
// (HealHour). It builds no immunity and takes no tending.
var condsMalnutrition = []CondEntry{{CondMalnutrition, CondDef{
	Label:      "malnutrition",
	NoImmunity: true,
	Stages: []Stage{
		{At: 0, Label: "trivial", Mods: CapMods{off(Consciousness, -50)}},
		{At: 200, Label: "minor", Mods: CapMods{off(Consciousness, -100)}},
		{At: 400, Label: "moderate", Mods: CapMods{off(Consciousness, -200)}},
		{At: 600, Label: "severe", Mods: CapMods{off(Consciousness, -300)}},
		{At: 800, Label: "extreme", Mods: CapMods{capMax(Consciousness, 100)}},
	},
}}}

// The malnutrition rates (tenths of a permille of severity an hour): a
// pawn's rate is rolled in [MalnutritionRateMin, MalnutritionRateMax].
// wiki: Malnutrition (1.51 % to 2.27 % an hour).
const (
	MalnutritionRateMin = 151
	MalnutritionRateMax = 227
)

// malnourishedMood is the Malnourished thought per stage, and
// malnutritionHunger the hunger rate (permille) per stage. wiki:
// Malnutrition, Saturation.
var (
	malnourishedMood   = [...]int{-20, -26, -32, -38, -44}
	malnutritionHunger = [...]int32{1500, 1600, 1600, 1600, 1600}
)

// Malnutrition is body b's malnutrition severity (permille; 0: none or
// no body). A free function: Body is at its method budget.
func Malnutrition(b *Body) int32 {
	if b == nil {
		return 0
	}
	if c, ok := b.Cond(CondMalnutrition); ok {
		return c.Sev
	}
	return 0
}

// MalnourishedMood is the Malnourished thought's mood for a body with
// malnutrition severity sev (the trivial stage's -20 without any).
func MalnourishedMood(sev int32) int {
	return malnourishedMood[DefOfCond(CondMalnutrition).StageAt(sev)]
}

// HungerMul is the hunger rate (permille) condition c sets: a disease's
// HungerMul, malnutrition's by stage, 1000 for anything else.
func HungerMul(c Condition) int32 {
	if c.Kind == CondMalnutrition && int(c.Stage) < len(malnutritionHunger) {
		return malnutritionHunger[c.Stage]
	}
	if dz, ok := DiseaseOf(c.Kind); ok {
		return dz.HungerMul
	}
	return 1000
}

// StepMalnutrition advances body b's malnutrition by the hour of index
// h: while starving it sets in (its rate from roll) and rises, else it
// falls and is gone at 0. Each hour moves severity by the rate's share of
// that hour, exactly over a run of hours. changed: it came, went or
// changed stage; dead: it reached full severity.
func StepMalnutrition(b *Body, starving bool, h uint64, roll func(int) int) (changed, dead bool) {
	c, ok := b.Cond(CondMalnutrition)
	if !ok {
		if !starving {
			return false, false
		}
		b.AddCond(Condition{Kind: CondMalnutrition, Part: Torso, Rate: MalnutritionRateMin + int32(roll(MalnutritionRateMax-MalnutritionRateMin+1))})
		c, _ = b.Cond(CondMalnutrition)
		changed = true
	}
	r := uint64(max(c.Rate, 1))
	step := int32((h+1)*r/10 - h*r/10)
	if !starving {
		step = -step
	}
	c.Sev = min(max(c.Sev+step, 0), 1000)
	if starving && c.Sev <= 0 {
		c.Sev = 1 // set in this hour: never a zero condition
	}
	if c.Sev <= 0 {
		b.RemoveCond(CondMalnutrition)
		return true, false
	}
	stage := DefOfCond(CondMalnutrition).StageAt(c.Sev)
	changed = changed || stage != c.Stage
	c.Stage = stage
	return changed, c.Sev >= 1000
}
