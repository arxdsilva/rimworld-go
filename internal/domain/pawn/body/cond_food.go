package body

import "rimworld-go/internal/domain/sim/clock"

// Food poisoning (WS-M): a badly cooked meal can sicken whoever eats it.
// It runs by time, not by a race, like the anaesthetic: initial for the
// first FoodPoisonInitialHours, major to FoodPoisonMajorHours, recovering
// to FoodPoisonHours, then gone. It never kills, builds no immunity and
// takes no tending. Original tuning (not the wiki's).
//
// The game steps it hourly (StepFoodPoisoning), setting Stage from the
// hours since it was caught and Sev to the share still to run (1000 → 0)
// for the severity bar, kept inside the stage's band so the condition race
// (which re-derives the stage from the severity and moves nothing, with no
// daily rate) agrees. Stages only ever move down with time, so it never
// flaps. What the game reads beyond the capacities and pain: VomitEvery
// per stage and FoodPoisonRestStage, the stage that sends a colonist to
// bed (the major one; up again once recovering).
var condsFood = []CondEntry{{CondFoodPoisoning, CondDef{
	Label:      "food poisoning",
	NoImmunity: true,
	Stages: []Stage{
		{At: 0, Label: "recovering", Mods: CapMods{off(Consciousness, -50)}},
		{At: foodMajorAt, Label: "major", PainAdd: 100, Mods: CapMods{
			off(Consciousness, -200), off(Moving, -100), off(Manipulation, -100)}},
		{At: foodInitialAt, Label: "initial", Mods: CapMods{
			off(Consciousness, -100), off(Moving, -50), off(Manipulation, -50)}},
	},
}}}

// Food poisoning timing (hours), the severities its stages sit at, and
// the mean hours between vomits per stage (0: none).
const (
	FoodPoisonInitialHours = 4
	FoodPoisonMajorHours   = 16
	FoodPoisonHours        = 24
	foodMajorAt            = 300
	foodInitialAt          = 800
	FoodStageRecovering    = 0
	FoodStageMajor         = 1
	FoodStageInitial       = 2
	// FoodPoisonRestStage is the stage that sends a colonist to bed.
	FoodPoisonRestStage = FoodStageMajor
)

// foodVomitEvery is each stage's mean hours between vomits.
var foodVomitEvery = [...]int32{FoodStageRecovering: 0, FoodStageMajor: 4, FoodStageInitial: 8}

// FoodVomitEvery is the mean hours between vomits at food-poisoning stage
// s (0: none).
func FoodVomitEvery(s uint8) int32 {
	if int(s) >= len(foodVomitEvery) {
		return 0
	}
	return foodVomitEvery[s]
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
