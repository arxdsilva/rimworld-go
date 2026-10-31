package body

// Cold conditions (TEMP-0 contract; TEMP-3 applies and steps them, with
// free functions here: Body is at its method budget).
//
// Frostbite (wiki: Hypothermia, Frostbite) strikes an extremity (a finger,
// toe, ear or nose; never an internal organ) of a pawn at hypothermia of
// at least 37 % in air at or below 0 °C, with a chance rising with the
// severity (original tuning: the wiki publishes none). It hurts like a
// wound, 1.25 times as much per point (pain is TEMP-3's), gets infected
// a quarter of the time untreated and leaves a permanent scar 1 % of the
// time.
//
// Hypothermic slowdown (wiki: Hypothermia) is what an insect suffers in
// place of hypothermia: the same gain, recovery and stages, but it never
// kills and never frostbites.
var condsFrostbite = []CondEntry{
	{CondFrostbite, CondDef{
		Label:      "frostbite",
		NoImmunity: true,
		Stages:     []Stage{{At: 0, Label: "frostbitten"}},
	}},
	{CondHypothermicSlowdown, CondDef{
		Label:      "hypothermic slowdown",
		NoImmunity: true,
		Stages: []Stage{ // wiki: Hypothermia (the stages, death left out)
			{At: 0, Label: "shivering"}, // original label: nothing shows below 4 %
			{At: 40, Label: "trivial", Mods: CapMods{off(Consciousness, -50), off(Manipulation, -80)}},
			{At: 200, Label: "minor", Mods: CapMods{off(Consciousness, -100), off(Manipulation, -200), off(Moving, -100)}},
			{At: 350, Label: "serious", PainAdd: 150,
				Mods: CapMods{off(Consciousness, -200), off(Manipulation, -500), off(Moving, -300)}},
			{At: 620, Label: "extreme", PainAdd: 300, Mods: CapMods{capMax(Consciousness, 100)}},
		},
	}},
}

// Frostbite thresholds (TEMP-3): the hypothermia severity (permille) and
// the air temperature (tenths of a degree C) at or past which frostbite
// can strike. wiki: Hypothermia (37 %, 0 °C).
const (
	FrostbiteMinHypothermia = 370
	FrostbiteMaxTemp        = 0
)
