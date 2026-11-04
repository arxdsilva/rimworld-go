// Package roster is who a colonist is, as the screens read it: the identity
// read models built from a generated pawn, the character-creation pool that
// exists before any Game (NG-1 §2.9, §5.4), and the Health tab's text. It is
// pure and holds no simulation state; the game package names its types for
// the adapters (ng1_types.go).
package roster

import (
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
)

// Bio read-model construction shared by character creation
// (CandidatePool.Views) and the colony's Bio tab (Game.Bio) (NG-1 §2.9,
// §5.4).

// SkillView is one skill row.
type SkillView struct {
	Key, Label string
	Level      int
	LevelLabel string
	Passion    int // 0 none, 1 minor, 2 major
	Incapable  bool
	XPPct      int
	SpeedPct   int
	Affects    string // work it drives
}

// TraitView is one trait chip.
type TraitView struct {
	Key, Label, Desc, Effects string
}

// BioView is a colonist's identity (pawn Bio tab, character creation).
type BioView struct {
	First, Nick, Last string
	Sex, Build        string
	AgeYears          int
	Childhood         string
	ChildhoodDesc     string
	Adulthood         string
	AdulthoodDesc     string
	Traits            []TraitView
	Skills            []SkillView
	Incapable         []string // "Violence (backstory: Convoy mechanic)"
	Conditions        []string
	Look              look.Look
}

// CandidateView is a character-creation candidate.
type CandidateView struct {
	BioView
	Selected bool
	Index    int
}

// TeamReport is the Selected colonists' combined skills and gaps.
type TeamReport struct {
	Best     []TeamSkill
	Warnings []string
}

// TeamSkill is a skill's best level among the Selected and who has it.
type TeamSkill struct {
	Key, Label string
	Level      int
	Who        string
}

// noBackstory is the title shown for a missing backstory (teenagers have
// no adulthood; legacy pawns have neither) and an incapable skill's level.
const noBackstory = "—"

// skillAffects is the work each skill drives and its other effects
// (NG-1 §3.5). hasWork is false for skills that drive no work type, whose
// SpeedPct stays 0.
var skillAffects = [skill.NumSkills]struct {
	text    string
	hasWork bool
}{
	skill.Shooting:     {"Hunting (ranged); ranged hit chance", true},
	skill.Melee:        {"Hunting (melee weapon); melee hit chance", true},
	skill.Construction: {"Construction (build, deconstruct, repair, roof, smooth)", true},
	skill.Mining:       {"Mining; ore and chunk yield", true},
	skill.Cooking:      {"Cooking (stove and campfire bills, butchering); butcher yield", true},
	skill.Plants:       {"Growing (sow, harvest, cut, cut grass); harvest yield", true},
	skill.Animals:      {"Handling (tame, milk, shear, collect); tame chance, product yield", true},
	skill.Crafting:     {"Crafting (workbench, tailoring, stonecutting)", true},
	skill.Medicine:     {"Doctoring (tend, rescue); heal amount, infection chance", true},
	skill.Social:       {"Chat mood bonus; tame chance", false},
	skill.Intellectual: {"Research", true},
}

// BioOf builds p's BioView: names, sex/build/age, backstories with their
// descriptions filled for p's nickname and pronouns, traits, all eleven
// skills, incapabilities with their reasons, conditions and look. It is
// pure: the same pawn always gives the same view.
func BioOf(p pawngen.Pawn) BioView {
	name := p.Nick
	if name == "" {
		name = p.First
	}
	sex := p.Look.Sex
	v := BioView{
		First: p.First, Nick: p.Nick, Last: p.Last,
		Sex:      sex.Label(),
		Build:    p.Look.Build.Label(),
		AgeYears: p.AgeYears(),
		Look:     p.Look,
	}
	v.Childhood, v.ChildhoodDesc = backstoryText(p.Childhood, sex, name)
	v.Adulthood, v.AdulthoodDesc = backstoryText(p.Adulthood, sex, name)

	for _, t := range p.Traits {
		d := trait.DefOf(t)
		if d.Key == "" {
			continue
		}
		v.Traits = append(v.Traits, TraitView{
			Key: d.Key, Label: d.Label,
			Desc:    backstory.Fill(d.Desc, sex, name),
			Effects: d.Effects(),
		})
	}

	inc := p.Incapable()
	workSpeed := int64(pawngen.Stats(p)[stat.WorkSpeed])
	v.Skills = make([]SkillView, skill.NumSkills)
	for id := skill.ID(0); id < skill.NumSkills; id++ {
		s := p.Skills[id]
		sv := SkillView{
			Key: id.Key(), Label: id.Label(),
			Affects: skillAffects[id].text,
		}
		if inc.DisablesSkill(id) {
			sv.Incapable = true
			sv.LevelLabel = noBackstory
		} else {
			sv.Level = int(s.Level)
			sv.LevelLabel = skill.LevelLabel(s.Level)
			sv.Passion = int(s.Passion)
			sv.XPPct = s.XPPct()
			if skillAffects[id].hasWork {
				// Level speed × WorkSpeed stat × trait/condition skill speed.
				sv.SpeedPct = int(int64(skill.SpeedPermil(s.Level)) * workSpeed *
					int64(pawngen.SkillWorkPct(p, id)) / 1_000_000)
			}
		}
		v.Skills[id] = sv
	}

	v.Incapable = p.IncapableReasons()
	for _, c := range p.Conditions {
		if c.Label() == "" {
			continue
		}
		v.Conditions = append(v.Conditions, c.Label()+": "+c.Desc())
	}
	return v
}

// backstoryText is a backstory's title and filled description, or
// ("—", "") when there is none.
func backstoryText(id backstory.ID, sex look.Sex, name string) (title, desc string) {
	d := backstory.DefOf(id)
	if id == 0 || d.Title == "" {
		return noBackstory, ""
	}
	return d.Title, backstory.Fill(d.Desc, sex, name)
}
