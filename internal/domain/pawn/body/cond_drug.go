package body

import "rimworld-go/internal/domain/sim/clock"

// Drug conditions (MED-07). Neither races immunity: the game steps them
// hourly itself (StepOverdose, StepPainkiller), so the condition race
// leaves them as they are (no daily rates, no immunity).
//
// Overdose: every dose of a hard drug (the spark tablet) adds 180..350
// severity and a prophyl tablet 80..140 (the game rolls the amount); it
// falls by 1000 a day. At 500 consciousness drops by 500 and the patient
// vomits now and then (mean 4.8 h); at 750 consciousness is capped at 10 %
// (downed) and death strikes on average 3 days in; 1000 kills at once.
// wiki: Overdose. Severity only rises on a dose and only falls between
// doses, so its stage never flaps.
//
// Painkiller: a numbroot tincture halves the body's pain for 12 hours
// (Pain). Original tuning: the wiki gives no numbers for a plain
// painkiller. Its severity is the share of the 12 hours still to run.
var condsDrug = []CondEntry{
	{CondOverdose, CondDef{
		Label:      "overdose",
		NoImmunity: true,
		Stages: []Stage{ // wiki: Overdose
			{At: 0},
			{At: OverdoseMajorAt, Label: "major", Mods: CapMods{{Cap: Consciousness, Off: -500}}},
			{At: OverdoseExtremeAt, Label: "extreme", Mods: CapMods{{Cap: Consciousness, Max: 100, HasMax: true}}},
		},
	}},
	{CondPainkiller, CondDef{
		Label:      "numbroot",
		NoImmunity: true,
		Stages:     []Stage{{At: 0, Label: "pain halved"}},
	}},
}

// Overdose and painkiller tuning.
const (
	OverdoseMajorAt   = 500  // wiki: Overdose
	OverdoseExtremeAt = 750  // wiki: Overdose
	OverdoseDeath     = 1000 // wiki: Overdose
	// OverdoseStageMajor and OverdoseStageExtreme index the stages.
	OverdoseStageMajor   = 1
	OverdoseStageExtreme = 2
	// overdoseDecayDay is how far overdose falls a day; stepped hourly as
	// 1000*h/24 so a day is exactly 1000. wiki: Overdose.
	overdoseDecayDay = 1000
	// OverdoseVomitMTBTenths is the mean time between vomits from the
	// major stage, in tenths of an hour (0.2 days). wiki: Overdose.
	OverdoseVomitMTBTenths = 48
	// OverdoseDeathMTBHours is the mean time to death at the extreme
	// stage (3 days). wiki: Overdose.
	OverdoseDeathMTBHours = 72
	// PainkillerHours is how long a numbroot dose acts; painkillerMul is
	// the pain it leaves (permille). Original tuning.
	PainkillerHours = 12
	painkillerMul   = 500
)

// AddOverdose adds sev (permille) to the body's overdose at tick now and
// returns the new severity; at OverdoseDeath or more the caller kills the
// pawn. A dose only moves the stage up.
func (b *Body) AddOverdose(sev int32, now uint64) int32 {
	if sev <= 0 {
		return b.Overdose()
	}
	d := DefOfCond(CondOverdose)
	if c, ok := b.Cond(CondOverdose); ok {
		c.Sev = min(c.Sev+sev, OverdoseDeath)
		c.Stage = max(c.Stage, d.StageAt(c.Sev))
		return c.Sev
	}
	sev = min(sev, OverdoseDeath)
	b.Conds = append(b.Conds, Condition{Kind: CondOverdose, Part: Torso, Sev: sev, Since: now, Stage: d.StageAt(sev)})
	return sev
}

// Overdose is the body's overdose severity (permille; 0: none).
func (b *Body) Overdose() int32 {
	if c, ok := b.Cond(CondOverdose); ok {
		return c.Sev
	}
	return 0
}

// OverdoseStage is the body's overdose stage (0: none or below major).
func (b *Body) OverdoseStage() uint8 {
	if c, ok := b.Cond(CondOverdose); ok {
		return c.Stage
	}
	return 0
}

// StepOverdose lets an hour of the overdose wear off; hour counts the
// hours since the dose (so 24 steps take exactly overdoseDecayDay). It
// reports whether the stage changed or the overdose ended.
func (b *Body) StepOverdose(hour uint64) bool {
	c, ok := b.Cond(CondOverdose)
	if !ok {
		return false
	}
	h := hour % 24
	dec := int32(overdoseDecayDay*(h+1)/24 - overdoseDecayDay*h/24)
	c.Sev -= dec
	if c.Sev <= 0 {
		b.RemoveCond(CondOverdose)
		return true
	}
	stage := DefOfCond(CondOverdose).StageAt(c.Sev)
	if stage >= c.Stage {
		return false // severity only falls here: the stage only moves down
	}
	c.Stage = stage
	return true
}

// Numb gives the body a fresh painkiller dose at tick now (one still
// acting starts over).
func (b *Body) Numb(now uint64) {
	fresh := Condition{Kind: CondPainkiller, Part: Torso, Sev: 1000, Since: now}
	if c, ok := b.Cond(CondPainkiller); ok {
		*c = fresh
		return
	}
	b.Conds = append(b.Conds, fresh)
}

// Numbed reports a painkiller still acting.
func (b *Body) Numbed() bool {
	_, ok := b.Cond(CondPainkiller)
	return ok
}

// StepPainkiller brings the painkiller up to tick now: its severity the
// share of PainkillerHours left, gone once they have passed. It reports
// whether it wore off.
func (b *Body) StepPainkiller(now uint64) bool {
	c, ok := b.Cond(CondPainkiller)
	if !ok {
		return false
	}
	const span = uint64(PainkillerHours * clock.TicksPerHour)
	var el uint64
	if now > c.Since {
		el = now - c.Since
	}
	if el >= span {
		b.RemoveCond(CondPainkiller)
		return true
	}
	c.Sev = max(int32((span-el)*1000/span), 1)
	return false
}

// painkilled scales pain n by an acting painkiller (Pain).
func (b *Body) painkilled(n int32) int32 {
	if b.Numbed() {
		return n * painkillerMul / 1000
	}
	return n
}
