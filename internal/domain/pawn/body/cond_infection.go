package body

// Wound infection (MED-02): a condition on the part whose wound festered.
// Severity climbs until immunity reaches 1000, then recedes; tending slows
// it by up to TendMax at full quality for TendEvery hours. It kills at
// 1000. The race itself is stepped by package infection. wiki: Infection.
var condsInfection = []CondEntry{{CondInfection, CondDef{
	Label:           "infection",
	SevPerDay:       840,  // wiki: Infection (+0.84/day not immune)
	ImmuneSevPerDay: -700, // wiki: Infection (−0.70/day once immune)
	ImmPerDay:       644,  // wiki: Infection (+0.6441/day while sick)
	TendMax:         530,  // wiki: Infection (−0.53/day at 100 % tend quality)
	TendEvery:       12,   // wiki: Infection (a tend every 12 hours)
	Lethal:          true,
	Stages: []Stage{ // wiki: Infection
		{At: 0, Label: "minor", PainAdd: 50},
		{At: 330, Label: "major", PainAdd: 80},
		{At: 780, Label: "extreme", PainAdd: 120, Mods: CapMods{{Cap: Consciousness, Off: -50}}},
		{At: 870, Label: "extreme", PainAdd: 850, Mods: CapMods{
			{Cap: Consciousness, Max: 100, HasMax: true}, {Cap: Breathing, Off: -50}}},
	},
}}}
