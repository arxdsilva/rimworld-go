package body

// Diseases (MED-04). Flu, plague, malaria and sleeping sickness race the
// body's immunity on the shared engine (package infection): severity
// climbs until immunity reaches 1000, then recedes; a tend slows the climb
// by up to TendMax at full quality for TendEvery hours; each kills at full
// severity. Gut worms and muscle parasites build no immunity and never
// kill: they last until the summed quality of their tends reaches
// CureTendSum (3000, i.e. 300 %), one tend every 48 hours. The numbers are
// the wiki's (ENG-003/MED exception), as permille; the names are the
// real-world diseases'.
//
// What a stage does beyond capacities and pain (hunger, rest fall,
// vomiting, which stage sends a patient to bed) is in Disease, below,
// read by the game's disease step.

// diseaseCure is the summed tend quality that cures a parasite (300 %).
const diseaseCure = 3000 // wiki: Gut worms, Muscle parasites

// off is a capacity offset (permille).
func off(c Capacity, v int32) CapMod { return CapMod{Cap: c, Off: v} }

// capMax caps a capacity at v (permille).
func capMax(c Capacity, v int32) CapMod { return CapMod{Cap: c, Max: v, HasMax: true} }

var condsDisease = []CondEntry{
	{CondFlu, CondDef{
		Label:           "flu",
		SevPerDay:       249,  // wiki: Flu (+0.2488/day not immune)
		ImmuneSevPerDay: -495, // wiki: Flu (−0.4947/day once immune)
		ImmPerDay:       239,  // wiki: Flu (+0.2388/day while sick)
		TendMax:         77,   // wiki: Flu (−0.0773/day at 100 % tend quality)
		TendEvery:       12,   // wiki: Flu (a tend every 12 hours)
		Lethal:          true,
		Stages: []Stage{ // wiki: Flu
			{At: 0, Label: "minor", Mods: CapMods{off(Consciousness, -50), off(Manipulation, -50), off(Breathing, -100)}},
			{At: 666, Label: "major", Mods: CapMods{off(Consciousness, -100), off(Manipulation, -100), off(Breathing, -150)}},
			{At: 833, Label: "extreme", PainAdd: 50, Mods: CapMods{off(Consciousness, -150), off(Manipulation, -200), off(Breathing, -200)}},
		},
	}},
	{CondPlague, CondDef{
		Label:           "plague",
		SevPerDay:       666,  // wiki: Plague (+0.666/day not immune)
		ImmuneSevPerDay: -333, // wiki: Plague (−0.333/day once immune)
		ImmPerDay:       522,  // wiki: Plague (+0.5224/day while sick)
		TendMax:         363,  // wiki: Plague (−0.3628/day at 100 % tend quality)
		TendEvery:       15,   // wiki: Plague (a tend every 15 hours)
		Lethal:          true,
		Stages: []Stage{ // wiki: Plague
			{At: 0, Label: "minor", PainAdd: 200, Mods: CapMods{off(Consciousness, -50), off(Manipulation, -50)}},
			{At: 600, Label: "major", PainAdd: 350, Mods: CapMods{off(Consciousness, -200), off(Manipulation, -200)}},
			{At: 800, Label: "extreme", PainAdd: 600, Mods: CapMods{off(Consciousness, -300), off(Manipulation, -300)}},
			{At: 910, Label: "extreme", PainAdd: 850, Mods: CapMods{off(Consciousness, -300), off(Manipulation, -300), off(Breathing, -150)}},
		},
	}},
	{CondMalaria, CondDef{
		Label:           "malaria",
		SevPerDay:       370,  // wiki: Malaria (+0.3702/day not immune)
		ImmuneSevPerDay: -730, // wiki: Malaria (−0.7297/day once immune)
		ImmPerDay:       315,  // wiki: Malaria (+0.3145/day while sick)
		TendMax:         232,  // wiki: Malaria (−0.232/day at 100 % tend quality)
		TendEvery:       12,   // wiki: Malaria (a tend every 12 hours)
		Lethal:          true,
		Stages: []Stage{ // wiki: Malaria
			{At: 0, Label: "minor", Mods: CapMods{off(BloodFiltration, -100), off(Consciousness, -50)}},
			{At: 780, Label: "major", PainAdd: 300, Mods: CapMods{off(BloodFiltration, -200), off(Consciousness, -120), off(Manipulation, -80)}},
			// The wiki gives "loss of consciousness" (flagged unsure) for
			// the extreme stage: consciousness capped at 10 %.
			{At: 910, Label: "extreme", PainAdd: 300, Mods: CapMods{off(BloodFiltration, -220), capMax(Consciousness, 100), off(Manipulation, -100)}},
		},
	}},
	{CondSleepingSickness, CondDef{
		Label:           "sleeping sickness",
		SevPerDay:       120,  // wiki: Sleeping sickness (+0.12/day not immune)
		ImmuneSevPerDay: -176, // wiki: Sleeping sickness (−0.176/day once immune)
		ImmPerDay:       110,  // wiki: Sleeping sickness (+0.11/day while sick)
		TendMax:         70,   // wiki: Sleeping sickness (−0.07/day at 100 % tend quality)
		TendEvery:       32,   // wiki: Sleeping sickness (a tend every 32 hours)
		Lethal:          true,
		Stages: []Stage{ // wiki: Sleeping sickness
			{At: 0, Label: "minor", PainAdd: 20, Mods: CapMods{off(Consciousness, -20), off(Manipulation, -20)}},
			{At: 437, Label: "minor", PainAdd: 30, Mods: CapMods{off(Consciousness, -40), off(Manipulation, -40)}},
			{At: 625, Label: "major", PainAdd: 50, Mods: CapMods{off(Consciousness, -150), off(Manipulation, -60)}},
			{At: 875, Label: "extreme", PainAdd: 100, Mods: CapMods{off(Consciousness, -200), off(Manipulation, -200)}},
			{At: 937, Label: "extreme", PainAdd: 150, Mods: CapMods{capMax(Consciousness, 100), off(Manipulation, -200)}},
		},
	}},
	{CondGutWorms, CondDef{
		Label:       "gut worms",
		TendEvery:   48, // wiki: Gut worms (a tend every 2 days)
		NoImmunity:  true,
		CureTendSum: diseaseCure,
		Stages:      []Stage{{At: 0, PainAdd: 200}}, // wiki: Gut worms (pain +20 %)
	}},
	{CondMuscleParasites, CondDef{
		Label:       "muscle parasites",
		TendEvery:   48, // wiki: Muscle parasites (a tend every 2 days)
		NoImmunity:  true,
		CureTendSum: diseaseCure,
		Stages: []Stage{{At: 0, PainAdd: 200, // wiki: Muscle parasites (pain +20 %, manipulation and moving −30 %)
			Mods: CapMods{off(Manipulation, -300), off(Moving, -300)}}},
	}},
}

// Disease is what a disease does beyond its CondDef: HungerMul and
// RestMul scale the hunger and the awake rest fall (permille, 1000 none);
// VomitEvery is each stage's mean hours between vomits (0: none); Rest is
// the first stage that sends a patient to bed ("major"; 0: none, the
// tends alone do); Onset is the severity it starts at.
type Disease struct {
	Kind       CondKind
	HungerMul  int32
	RestMul    int32
	VomitEvery []int32
	Rest       uint8
	Onset      int32
}

// diseaseOnset is the severity an immunizable disease starts at: the wiki
// gives none (original tuning, as the wound infection's).
const diseaseOnset = 10

// diseases is every disease's extra effects, in kind order.
var diseases = []Disease{
	{Kind: CondFlu, HungerMul: 1000, RestMul: 1000, VomitEvery: []int32{0, 36, 18}, Rest: 1, Onset: diseaseOnset},                     // wiki: Flu (vomit every 1.5 / 0.75 days)
	{Kind: CondPlague, HungerMul: 1000, RestMul: 1000, VomitEvery: []int32{0, 0, 0, 0}, Rest: 1, Onset: diseaseOnset},                 // wiki: Plague
	{Kind: CondMalaria, HungerMul: 1000, RestMul: 1000, VomitEvery: []int32{0, 36, 18}, Rest: 1, Onset: diseaseOnset},                 // wiki: Malaria (vomit every 1.5 / 0.75 days)
	{Kind: CondSleepingSickness, HungerMul: 1000, RestMul: 1000, VomitEvery: []int32{0, 0, 84, 42, 24}, Rest: 2, Onset: diseaseOnset}, // wiki: Sleeping sickness (vomit every 3.5 / 1.75 / 1 days)
	{Kind: CondGutWorms, HungerMul: 2000, RestMul: 1000, VomitEvery: []int32{24}, Onset: 1000},                                        // wiki: Gut worms (hunger +100 %, vomit every day)
	{Kind: CondMuscleParasites, HungerMul: 1000, RestMul: 2000, VomitEvery: []int32{0}, Onset: 1000},                                  // wiki: Muscle parasites (rest fall +100 %)
}

// IsDisease reports k a disease (not a wound infection or another
// condition).
func IsDisease(k CondKind) bool { return k >= CondFlu && k <= CondMuscleParasites }

// DiseaseOf is disease k's extra effects (false: k is no disease).
func DiseaseOf(k CondKind) (Disease, bool) {
	if !IsDisease(k) {
		return Disease{}, false
	}
	return diseases[k-CondFlu], true
}

// VomitEveryAt is the mean hours between vomits at stage s (0: none).
func (d Disease) VomitEveryAt(s uint8) int32 {
	if int(s) >= len(d.VomitEvery) {
		return 0
	}
	return d.VomitEvery[s]
}

// TendCureSev is a parasite's severity shown for its tends so far: full
// with none, falling as the summed quality nears the cure (at least 1
// until cured).
func TendCureSev(tendSum, cure int32) int32 {
	if cure <= 0 {
		return 1000
	}
	return max(1000-min(tendSum, cure)*1000/cure, 1)
}
