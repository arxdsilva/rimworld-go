package game

import (
	"strings"

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

// bioOf builds p's BioView: names, sex/build/age, backstories with their
// descriptions filled for p's nickname and pronouns, traits, all eleven
// skills, incapabilities with their reasons, conditions and look. It is
// pure: the same pawn always gives the same view.
func bioOf(p pawngen.Pawn) BioView {
	name := p.Nick
	if name == "" {
		name = p.First
	}
	sex := p.Look.Sex
	v := BioView{
		First: p.First, Nick: p.Nick, Last: p.Last,
		Sex:      sexLabel(sex),
		Build:    buildLabel(p.Look.Build),
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
			sv.XPPct = xpPct(s)
			if skillAffects[id].hasWork {
				// Level speed × WorkSpeed stat × trait/condition skill speed.
				sv.SpeedPct = int(int64(skill.SpeedPermil(s.Level)) * workSpeed *
					int64(pawngen.SkillWorkPct(p, id)) / 1_000_000)
			}
		}
		v.Skills[id] = sv
	}

	v.Incapable = incapableReasons(p)
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

// incapableReasons lists each incapability with what causes it:
// "Violence (backstory: Convoy mechanic)", or several causes joined:
// "Violence (backstory: Convoy mechanic, trait: Gentle)". Bits go in
// backstory.Bits order; causes go childhood, adulthood, then traits.
func incapableReasons(p pawngen.Pawn) []string {
	all := p.Incapable()
	if all == 0 {
		return nil
	}
	type cause struct {
		bits backstory.Bits
		text string
	}
	var causes []cause
	for _, id := range []backstory.ID{p.Childhood, p.Adulthood} {
		if d := backstory.DefOf(id); id != 0 && d.Incapable != 0 {
			causes = append(causes, cause{d.Incapable, "backstory: " + d.Title})
		}
	}
	for _, t := range p.Traits {
		if d := trait.DefOf(t); d.Incapable != 0 {
			causes = append(causes, cause{d.Incapable, "trait: " + d.Label})
		}
	}
	var out []string
	for b := backstory.Bits(1); b&backstory.AllBits != 0; b <<= 1 {
		if all&b == 0 {
			continue
		}
		var why []string
		for _, c := range causes {
			if c.bits&b != 0 {
				why = append(why, c.text)
			}
		}
		out = append(out, b.Labels()[0]+" ("+strings.Join(why, ", ")+")")
	}
	return out
}

// xpPct is the progress toward the next level in percent (0..100).
func xpPct(s skill.Skill) int {
	c := skill.Cost(s.Level)
	if c <= 0 || s.XP <= 0 {
		return 0
	}
	return int(min(int64(s.XP)*100/int64(c), 100))
}

func sexLabel(s look.Sex) string {
	if s == look.Female {
		return "Female"
	}
	return "Male"
}

func buildLabel(b look.Build) string {
	switch b {
	case look.Thin:
		return "Thin"
	case look.Heavy:
		return "Heavy"
	}
	return "Average"
}
