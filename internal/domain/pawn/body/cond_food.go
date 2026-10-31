package body

import "rimworld-go/internal/domain/sim/clock"

// Food poisoning (WS-M): a badly cooked meal can sicken whoever eats it.
// It runs by time, not by a race, like the anaesthetic: initial for the
// first FoodPoisonInitialHours, major to FoodPoisonMajorHours, recovering
// to FoodPoisonHours, then gone. It never kills, builds no immunity and
// takes no tending. wiki: Food poisoning (FOOD-4, FOOD-6): initial 0-4 h,
// major 4-20 h, recovering 20-24 h; consciousness x0.6 / x0.5, moving
// x0.8 / x0.5, manipulation x0.9 / x0.8, blood filtration x0.95 / x0.85,
// pain +20 % / +40 % and eating speed x0.5 / x0.3 (initial and
// recovering alike; the capacities as offsets on a full capacity, the
// engine's only kind); a vomit every 0.3 / 0.2 / 0.4 days on average. A
// synth stomach (the bionic one) halves the chance to catch it
// (FoodPoisonChance; wiki: Food poisoning, Bionic stomach).
//
// The game steps it hourly (StepFoodPoisoning), setting Stage from the
// hours since it was caught and Sev to the share still to run (1000 → 0)
// for the severity bar, kept inside the stage's band so the condition race
// (which re-derives the stage from the severity and moves nothing, with no
// daily rate) agrees. Stages only ever move down with time, so it never
// flaps. What the game reads beyond the capacities and pain: VomitEvery
// per stage, FoodPoisonEatSpeed and FoodPoisonRestStage, the stage that
// sends a colonist to bed (the major one; up again once recovering).
var condsFood = []CondEntry{{CondFoodPoisoning, CondDef{
	Label:      "food poisoning",
	NoImmunity: true,
	Stages: []Stage{
		{At: 0, Label: "recovering", PainAdd: 200, Mods: foodMild},
		{At: foodMajorAt, Label: "major", PainAdd: 400, Mods: CapMods{ // wiki: Food poisoning
			off(Consciousness, -500), off(Moving, -500), off(Manipulation, -200), off(BloodFiltration, -150)}},
		{At: foodInitialAt, Label: "initial", PainAdd: 200, Mods: foodMild},
	},
}}}

// foodMild is the initial and recovering stages' capacities. wiki: Food
// poisoning.
var foodMild = CapMods{off(Consciousness, -400), off(Moving, -200), off(Manipulation, -100), off(BloodFiltration, -50)}

// Food poisoning timing (hours), the severities its stages sit at (the
// share of the day still to run), and the stages.
const (
	FoodPoisonInitialHours = 4
	FoodPoisonMajorHours   = 20
	FoodPoisonHours        = 24
	foodMajorAt            = 167 // 4 of 24 h left
	foodInitialAt          = 834 // 20 of 24 h left
	FoodStageRecovering    = 0
	FoodStageMajor         = 1
	FoodStageInitial       = 2
	// FoodPoisonRestStage is the stage that sends a colonist to bed.
	FoodPoisonRestStage = FoodStageMajor
)

// foodVomitEvery is each stage's mean hours between vomits (0.4, 0.2
// and 0.3 days, rounded to hours).
var foodVomitEvery = [...]int32{FoodStageRecovering: 10, FoodStageMajor: 5, FoodStageInitial: 7}

// FoodVomitEvery is the mean hours between vomits at food-poisoning stage
// s (0: none).
func FoodVomitEvery(s uint8) int32 {
	if int(s) >= len(foodVomitEvery) {
		return 0
	}
	return foodVomitEvery[s]
}

// foodEatSpeed is each stage's eating speed (permille). wiki: Food
// poisoning (x50 % initial and recovering, x30 % major).
var foodEatSpeed = [...]int32{FoodStageRecovering: 500, FoodStageMajor: 300, FoodStageInitial: 500}

// FoodPoisonEatSpeed is body b's eating speed factor (permille) from food
// poisoning: 1000 without it (or without a body). A free function: Body
// is at its method budget.
func FoodPoisonEatSpeed(b *Body) int32 {
	c, ok := b.Poisoned()
	if !ok || int(c.Stage) >= len(foodEatSpeed) {
		return 1000
	}
	return foodEatSpeed[c.Stage]
}

// FoodPoisonChance is the factor (permille) on body b's chance to catch
// food poisoning: a fitted synth stomach halves it (wiki: Food poisoning,
// Bionic stomach x50 %); 1000 otherwise or without a body.
func FoodPoisonChance(b *Body) int32 {
	if b != nil && b.GutWormImmune() { // the synth stomach test
		return 500
	}
	return 1000
}

// Poisoned reports whether the body has food poisoning, and returns it.
func (b *Body) Poisoned() (Condition, bool) {
	if b == nil {
		return Condition{}, false
	}
	c, ok := b.Cond(CondFoodPoisoning)
	if !ok {
		return Condition{}, false
	}
	return *c, true
}

// FoodPoisonRest reports b's food poisoning at its major stage: the
// colonist goes to bed (bed rest: in at major, out at recovering).
func FoodPoisonRest(b *Body) bool { c, ok := b.Poisoned(); return ok && c.Stage == FoodPoisonRestStage }

// FoodPoisonSick reports b's food poisoning not yet recovering: a patient
// in bed stays there until it is (FoodPoisonRest's hysteresis).
func FoodPoisonSick(b *Body) bool { c, ok := b.Poisoned(); return ok && c.Stage >= FoodPoisonRestStage }

// Poison gives the body fresh food poisoning at tick now; it reports
// false when it already has some (a second bad meal changes nothing).
func (b *Body) Poison(now uint64) bool {
	return b.AddCond(Condition{Kind: CondFoodPoisoning, Part: Torso, Sev: 1000, Since: now, Stage: FoodStageInitial})
}

// StepFoodPoisoning brings food poisoning up to tick now: its stage by
// the hours since it was caught, its severity the share left to run, and
// gone after FoodPoisonHours. It reports whether anything changed.
func (b *Body) StepFoodPoisoning(now uint64) bool {
	c, ok := b.Cond(CondFoodPoisoning)
	if !ok {
		return false
	}
	var el uint64
	if now > c.Since {
		el = now - c.Since
	}
	const hour = uint64(clock.TicksPerHour)
	if el >= FoodPoisonHours*hour {
		b.RemoveCond(CondFoodPoisoning)
		return true
	}
	stage := uint8(FoodStageInitial)
	switch {
	case el >= FoodPoisonMajorHours*hour:
		stage = FoodStageRecovering
	case el >= FoodPoisonInitialHours*hour:
		stage = FoodStageMajor
	}
	sev := int32(1000 - el*1000/(FoodPoisonHours*hour))
	switch stage {
	case FoodStageInitial:
		sev = max(sev, foodInitialAt)
	case FoodStageMajor:
		sev = min(max(sev, foodMajorAt), foodInitialAt-1)
	default:
		sev = min(max(sev, 1), foodMajorAt-1)
	}
	changed := c.Stage != stage || c.Sev != sev
	c.Stage, c.Sev = stage, sev
	return changed
}
