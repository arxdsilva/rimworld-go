package body

// Toxic buildup (FOOD-6, wiki: Toxic buildup, Toxic fallout). Under the
// open sky in toxic fallout a pawn gains ToxicPerDay of severity a day,
// times its toxic resistance (an animal takes half: ToxicResistAnimal;
// insects and machines are immune, the caller's rule); out of it the
// severity falls ToxicRecoverPerDay a day. The stages lower
// consciousness; from moderate the pawn vomits now and then and may
// develop dementia or a carcinoma, which stay. Full severity kills (the
// game steps it hourly: StepToxic; death and downing are the game's).
// It builds no immunity and takes no tending.
var condsToxic = []CondEntry{{CondToxicBuildup, CondDef{
	Label:      "toxic buildup",
	NoImmunity: true,
	Stages: []Stage{
		{At: 0, Label: "faint"}, // original label: the wiki shows nothing below 4 %
		{At: 40, Label: "initial", Mods: CapMods{off(Consciousness, -50)}},
		{At: 200, Label: "minor", Mods: CapMods{off(Consciousness, -100)}},
		{At: 400, Label: "moderate", Mods: CapMods{off(Consciousness, -150)}},
		{At: 600, Label: "serious", Mods: CapMods{off(Consciousness, -250)}},
		{At: 800, Label: "extreme", Mods: CapMods{capMax(Consciousness, 100)}},
	},
}}}

// Toxic buildup rates, permille of severity a day, and the animals'
// resistance, permille (wiki: Toxic buildup: 40 % a day under fallout, 8
// % a day recovering, animals 50 % toxic resistance).
const (
	ToxicPerDay        = 400
	ToxicRecoverPerDay = 80
	ToxicResistAnimal  = 500
)

// toxicMTB is each stage's mean days between a vomit, a dementia onset
// and a carcinoma onset (0: none). wiki: Toxic buildup.
var toxicMTB = [...][3]int{{}, {}, {}, {5, 146, 438}, {1, 37, 111}, {0, 13, 39}}

// toxicVomitExtreme: the extreme stage vomits every half day (the table
// holds whole days). wiki: Toxic buildup (MTB 0.5 days).
const toxicVomitExtremeHours = 12

// ToxicBuildup is body b's toxic buildup severity (permille; 0: none or
// no body). A free function: Body is at its method budget.
func ToxicBuildup(b *Body) int32 {
	if b == nil {
		return 0
	}
	if c, ok := b.Cond(CondToxicBuildup); ok {
		return c.Sev
	}
	return 0
}

// ToxicStep is what an hour of toxic buildup did: it came, went or
// changed stage; it killed; a vomit; a dementia or carcinoma onset is
// due (the game starts them, aging.go).
type ToxicStep struct {
	Changed, Dead, Vomit, Dementia, Carcinoma bool
}

// StepToxic advances body b's toxic buildup by the hour of index h:
// exposed, it sets in and rises by ToxicPerDay less resist (permille);
// otherwise it falls by ToxicRecoverPerDay and is gone at 0. Each hour
// moves severity by the rate's share of that hour, exactly over a run of
// hours. roll(n) is in [0, n) and draws the stage's chances.
func StepToxic(b *Body, exposed bool, resist int32, h uint64, roll func(int) int) ToxicStep {
	var out ToxicStep
	c, ok := b.Cond(CondToxicBuildup)
	if !ok {
		if !exposed || resist >= 1000 {
			return out
		}
		b.AddCond(Condition{Kind: CondToxicBuildup, Part: Torso})
		c, _ = b.Cond(CondToxicBuildup)
		out.Changed = true
	}
	r := uint64(ToxicRecoverPerDay)
	if exposed {
		r = uint64(ToxicPerDay * (1000 - min(max(resist, 0), 1000)) / 1000)
	}
	step := int32((h+1)*r/24 - h*r/24)
	if !exposed {
		step = -step
	}
	c.Sev = min(max(c.Sev+step, 0), 1000)
	if exposed && c.Sev <= 0 {
		c.Sev = 1 // set in this hour: never a zero condition
	}
	if c.Sev <= 0 {
		b.RemoveCond(CondToxicBuildup)
		out.Changed = true
		return out
	}
	stage := DefOfCond(CondToxicBuildup).StageAt(c.Sev)
	out.Changed = out.Changed || stage != c.Stage
	c.Stage = stage
	if c.Sev >= 1000 {
		out.Dead = true
		return out
	}
	mtb := toxicMTB[stage]
	vomit := mtb[0] * 24
	if stage == uint8(len(toxicMTB)-1) {
		vomit = toxicVomitExtremeHours
	}
	out.Vomit = vomit > 0 && roll(vomit) == 0
	out.Dementia = mtb[1] > 0 && roll(mtb[1]*24) == 0
	out.Carcinoma = mtb[2] > 0 && roll(mtb[2]*24) == 0
	return out
}
