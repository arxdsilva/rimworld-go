package pawngen

import (
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
)

// ExhaustedRest is the rest need (percent) an Exhausted colonist starts at.
const ExhaustedRest = 20

var condInfo = [numConditions]struct {
	label, desc string
	mods        []stat.Mod
	work        map[skill.ID]int16
}{
	NoCondition: {},
	OldFracture: {"Old leg fracture", "A badly set break that still aches in the cold.",
		[]stat.Mod{{Stat: stat.MoveSpeed, Mul: 920}}, nil},
	Nearsighted: {"Nearsighted", "Anything past arm's length is a blur.",
		[]stat.Mod{{Stat: stat.ShootAcc, Add: -15}}, nil},
	HandTremor: {"Hand tremor", "A faint, constant shake that fine work makes worse.",
		nil, map[skill.ID]int16{skill.Crafting: -10, skill.Medicine: -10}},
	Exhausted: {"Exhausted", "Arrived worn out, with nothing but bare hands.", nil, nil},
}

// Label is the display name of a condition.
func (c Condition) Label() string {
	if c >= numConditions {
		return ""
	}
	return condInfo[c].label
}

// Desc is a one-line description of a condition.
func (c Condition) Desc() string {
	if c >= numConditions {
		return ""
	}
	return condInfo[c].desc
}

// Mods are the condition's stat changes.
func (c Condition) Mods() []stat.Mod {
	if c >= numConditions {
		return nil
	}
	return condInfo[c].mods
}

// bodyMods are the build's stat changes (NG-1 §4.6).
func bodyMods(b look.Build) []stat.Mod {
	switch b {
	case look.Thin:
		return []stat.Mod{{Stat: stat.HungerRate, Mul: 900}, {Stat: stat.ColdComfort, Add: 20},
			{Stat: stat.MoveSpeed, Mul: 1030}, {Stat: stat.MeleeAcc, Add: -5}}
	case look.Heavy:
		return []stat.Mod{{Stat: stat.HungerRate, Mul: 1150}, {Stat: stat.ColdComfort, Add: -30},
			{Stat: stat.MoveSpeed, Mul: 950}, {Stat: stat.MeleeAcc, Add: 5}}
	}
	return nil
}

// ageMods are the age band's stat changes (NG-1 §4.6).
func ageMods(years int) []stat.Mod {
	switch {
	case years < adultAge:
		return []stat.Mod{{Stat: stat.LearnRate, Mul: 1250}, {Stat: stat.WorkSpeed, Mul: 850}}
	case years >= 65:
		return []stat.Mod{{Stat: stat.WorkSpeed, Mul: 850}, {Stat: stat.MoveSpeed, Mul: 900}}
	case years >= 50:
		return []stat.Mod{{Stat: stat.WorkSpeed, Mul: 950}, {Stat: stat.MoveSpeed, Mul: 950}}
	}
	return nil
}

// StatLayers are p's stat layers in the fixed NG-1 order: body, age,
// conditions, traits. The game appends the difficulty layer and calls
// stat.Build(stat.Base(), layers...).
func StatLayers(p Pawn) [][]stat.Mod {
	var cond, traits []stat.Mod
	for _, c := range p.Conditions {
		cond = append(cond, c.Mods()...)
	}
	for _, t := range p.Traits {
		traits = append(traits, trait.DefOf(t).Mods...)
	}
	return [][]stat.Mod{bodyMods(p.Look.Build), ageMods(p.AgeYears()), cond, traits}
}

// Stats is p's stat sheet before difficulty. A Legacy pawn gets exactly
// stat.Base().
func Stats(p Pawn) stat.Sheet { return stat.Build(stat.Base(), StatLayers(p)...) }

// SkillWorkPct is p's work speed for jobs using skill id, in percent of
// normal, from traits (Green thumb) and conditions (Hand tremor). It
// multiplies the WorkSpeed stat and the skill level's speed.
func SkillWorkPct(p Pawn, id skill.ID) int {
	pct := 100
	for _, t := range p.Traits {
		pct += int(trait.DefOf(t).WorkSpeed[id])
	}
	for _, c := range p.Conditions {
		if c < numConditions {
			pct += int(condInfo[c].work[id])
		}
	}
	return max(pct, 0)
}

// LearnPct is p's learning multiplier for skill id from traits, in
// percent (Bookish: 150 for Intellectual). It multiplies LearnRate.
func LearnPct(p Pawn, id skill.ID) int {
	pct := 100
	for _, t := range p.Traits {
		if m, ok := trait.DefOf(t).LearnMul[id]; ok {
			pct = pct * int(m) / 100
		}
	}
	return pct
}
