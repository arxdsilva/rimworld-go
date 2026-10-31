package body

// Temperature exposure (TEMP-3; free functions: Body is at its method
// budget). The game keeps each pawn's and animal's hypothermia and
// heatstroke severities (domain/pawn/exposure, fine fixed point) and sets
// them here as conditions in permille (SetExposure), so their stages
// reach the capacities and pain through the condition machinery: a stage
// past 62 % caps consciousness at 10 %, which downs the pawn. Death at
// full severity is the game's. None of them builds immunity, takes
// tending or races anything (their SevPerDay is 0).
//
// Hypothermic slowdown (wiki: Hypothermia) is what an insect suffers in
// place of hypothermia: the same gain, recovery and stages, but it never
// kills and never frostbites.
//
// Frostbite (wiki: Frostbite) is a wound of kind Frostbite on an
// extremity (StrikeFrostbite): a pawn at hypothermia of at least 37 % in
// air at or below 0 °C can take one, at a chance rising with the severity
// (exposure.FrostbiteChance, original tuning). It hurts 1.25 % per point
// like any wound (the wiki's pain 1.25), never bleeds, festers a quarter
// of the time untended (infection.WoundChance), heals as wounds do and
// scars 1 % of the time (ScarOdds).
var condsFrostbite = []CondEntry{
	{CondHypothermia, CondDef{Label: "hypothermia", NoImmunity: true, Stages: coldStages(true)}},
	{CondHypothermicSlowdown, CondDef{Label: "hypothermic slowdown", NoImmunity: true, Stages: coldStages(true)}},
	{CondHeatstroke, CondDef{Label: "heatstroke", NoImmunity: true, Stages: coldStages(false)}},
}

// coldStages are the exposure stages (wiki: Hypothermia, Heatstroke);
// heatstroke has no manipulation term.
func coldStages(manip bool) []Stage {
	m := func(v int32, mods ...CapMod) CapMods {
		if manip {
			mods = append(mods, off(Manipulation, v))
		}
		return mods
	}
	return []Stage{
		{At: 0, Label: "shivering"}, // original label: nothing shows below 4 %
		{At: 40, Label: "trivial", Mods: m(-80, off(Consciousness, -50))},
		{At: 200, Label: "minor", Mods: m(-200, off(Consciousness, -100), off(Moving, -100))},
		{At: 350, Label: "serious", PainAdd: 150, Mods: m(-500, off(Consciousness, -200), off(Moving, -300))},
		{At: 620, Label: "extreme", PainAdd: 300, Mods: CapMods{capMax(Consciousness, 100)}},
	}
}

// Frostbite thresholds: the hypothermia severity (permille) and the air
// temperature (tenths of a degree C) at or past which frostbite can
// strike (wiki: Hypothermia: 37 %, 0 °C); FrostbiteScar is the permille
// chance a healed frostbite scars (wiki: Frostbite, 1 %); frostbiteMaxDmg
// the most one strike deals (original tuning: 1–4 points, never the
// whole part).
const (
	FrostbiteMinHypothermia = 370
	FrostbiteMaxTemp        = 0
	FrostbiteScar           = 10
	frostbiteMaxDmg         = 4
)

// SetExposure sets b's condition k (CondHypothermia, its slowdown or
// CondHeatstroke) to sev permille: added at the first point, removed at
// 0, its stage following the severity. It reports a change of stage
// (appearing and going included): capacities and pain moved.
func SetExposure(b *Body, k CondKind, sev int32) bool {
	c, ok := b.Cond(k)
	if sev <= 0 {
		return ok && b.RemoveCond(k)
	}
	stage := DefOfCond(k).StageAt(min(sev, 1000))
	if !ok {
		b.AddCond(Condition{Kind: k, Part: b.Root(), Sev: sev, Stage: stage})
		return true
	}
	changed := c.Stage != stage
	c.Sev, c.Stage = min(sev, 1000), stage
	return changed
}

// ExposureSev is b's severity of condition k in permille (0: none or no
// body).
func ExposureSev(b *Body, k CondKind) int32 {
	if b == nil {
		return 0
	}
	if c, ok := b.Cond(k); ok {
		return c.Sev
	}
	return 0
}

// frostParts are each plan's extremities, the parts frostbite can take:
// ears, noses, hands, feet, paws and tails; never an internal organ, and
// an insect none (it slows instead).
var frostParts = [NumAnatomies][]Part{
	Humanlike: {EarL, EarR, Nose, HandL, HandR, FootL, FootR},
	Quadruped: {QEarL, QEarR, QNose, QPawFL, QPawFR, QPawHL, QPawHR, QTail},
	Bird:      {BEarL, BEarR, BFootL, BFootR, BTail},
	Snake:     {STail},
	Lizard:    {LFootFL, LFootFR, LFootHL, LFootHR, LTail},
}

// StrikeFrostbite frostbites one of b's extremities picked with roll
// (roll(n) in [0, n)): 1–frostbiteMaxDmg points, never as many as the
// part has left (frostbite here never takes a part). It reports the part
// and the damage, false when no extremity is left to take it.
func StrikeFrostbite(b *Body, roll func(int) int, now uint64) (Part, int, bool) {
	if b.Anatomy >= NumAnatomies {
		return 0, 0, false
	}
	var open []Part
	for _, p := range frostParts[b.Anatomy] {
		if p < b.numParts() && !b.isMissing(p) && b.maxHP(p)-b.partDmg(p) > 1 {
			open = append(open, p)
		}
	}
	if len(open) == 0 {
		return 0, 0, false
	}
	p := open[roll(len(open))]
	dmg := min(1+roll(frostbiteMaxDmg), b.maxHP(p)-b.partDmg(p)-1)
	b.hurtPart(p, dmg, Frostbite, now)
	return p, dmg, true
}

// FrostbiteParts counts b's parts with an open frostbite (0: none or no
// body).
func FrostbiteParts(b *Body) int {
	if b == nil {
		return 0
	}
	var seen uint32
	n := 0
	for _, w := range b.Wounds {
		if w.Kind == Frostbite && !w.Permanent && seen&bit(w.Part) == 0 {
			seen |= bit(w.Part)
			n++
		}
	}
	return n
}
