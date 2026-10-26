package body

import "rimworld-go/internal/domain/sim/clock"

// Anaesthetic (MED-06): every operation, and the Anesthetize operation on
// its own, leaves the patient under an anaesthetic. It wears off by time,
// not by a race: sedated for the first 6 hours (consciousness at most
// 1 %, so the patient lies down), woozy to 12 hours (consciousness at most
// 70 %, moving and manipulation −20 %), wearing off to 20 hours
// (consciousness at most 90 %), then gone. wiki: Anesthetic.
//
// The game steps it hourly (StepAnesthetic), setting Stage from the hours
// since it was given and Sev to the share still to wear off (1000 → 0) for
// the severity bar. It has no immunity, no tending and no daily severity
// rate, so the condition race leaves it as it is; stages only ever move
// down with time, so it never flaps.
var condsAnesthetic = []CondEntry{{CondAnesthetic, CondDef{
	Label:      "anesthetic",
	NoImmunity: true,
	Stages: []Stage{ // wiki: Anesthetic
		{At: 0, Label: "wearing off", Mods: CapMods{{Cap: Consciousness, Max: 900, HasMax: true}}},
		{At: anesthWoozyAt, Label: "woozy", Mods: CapMods{
			{Cap: Consciousness, Max: 700, HasMax: true}, {Cap: Moving, Off: -200}, {Cap: Manipulation, Off: -200}}},
		{At: anesthSedatedAt, Label: "sedated", Mods: CapMods{{Cap: Consciousness, Max: 10, HasMax: true}}},
	},
}}}

// Anaesthetic timing (hours) and the severities its stages sit at.
const (
	AnesthSedatedHours = 6  // wiki: Anesthetic
	AnesthWoozyHours   = 12 // wiki: Anesthetic
	AnesthHours        = 20 // wiki: Anesthetic
	anesthSedatedAt    = 700
	anesthWoozyAt      = 400
	anesthStageSedated = 2
	anesthStageWoozy   = 1
	anesthStageOff     = 0
)

// Anesthetize puts the body under a fresh anaesthetic at tick now (an
// existing one starts over).
func (b *Body) Anesthetize(now uint64) {
	fresh := Condition{Kind: CondAnesthetic, Part: Torso, Sev: 1000, Since: now, Stage: anesthStageSedated}
	if c, ok := b.Cond(CondAnesthetic); ok {
		*c = fresh
		return
	}
	b.Conds = append(b.Conds, fresh)
}

// Sedated reports the body under the first, knocked-out stage of an
// anaesthetic.
func (b *Body) Sedated() bool {
	c, ok := b.Cond(CondAnesthetic)
	return ok && c.Stage == anesthStageSedated
}

// Anesthetized reports any stage of an anaesthetic still on the body.
func (b *Body) Anesthetized() bool {
	_, ok := b.Cond(CondAnesthetic)
	return ok
}

// StepAnesthetic brings the anaesthetic up to tick now: its stage by the
// hours since it was given, its severity the share left to wear off, and
// gone after AnesthHours. It reports whether anything changed.
func (b *Body) StepAnesthetic(now uint64) bool {
	c, ok := b.Cond(CondAnesthetic)
	if !ok {
		return false
	}
	var el uint64
	if now > c.Since {
		el = now - c.Since
	}
	const hour = uint64(clock.TicksPerHour)
	if el >= AnesthHours*hour {
		b.RemoveCond(CondAnesthetic)
		return true
	}
	stage := uint8(anesthStageSedated)
	switch {
	case el >= AnesthWoozyHours*hour:
		stage = anesthStageOff
	case el >= AnesthSedatedHours*hour:
		stage = anesthStageWoozy
	}
	sev := int32(1000 - el*1000/(AnesthHours*hour))
	sev = stageSev(stage, sev)
	changed := c.Stage != stage || c.Sev != sev
	c.Stage, c.Sev = stage, sev
	return changed
}

// stageSev keeps a severity inside its stage's band, so the condition
// race (which re-derives the stage from the severity) agrees with the
// stage set by time.
func stageSev(stage uint8, sev int32) int32 {
	switch stage {
	case anesthStageSedated:
		return max(sev, anesthSedatedAt)
	case anesthStageWoozy:
		return min(max(sev, anesthWoozyAt), anesthSedatedAt-1)
	}
	return min(max(sev, 1), anesthWoozyAt-1)
}
