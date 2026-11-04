package sheet

import (
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

// WorkLabel is a work type in words.
func WorkLabel(t work.Type) string {
	switch t {
	case work.Firefight:
		return "Firefighting"
	case work.Doctor:
		return "Doctoring"
	case work.Grow:
		return "Growing"
	case work.Cook:
		return "Cooking"
	case work.Construct:
		return "Construction"
	case work.Hunt:
		return "Hunting"
	case work.Haul:
		return "Hauling"
	case work.Clean:
		return "Cleaning"
	case work.Research:
		return "Research"
	case work.Mine:
		return "Mining"
	case work.Craft:
		return "Crafting"
	case work.Handle:
		return "Animal handling"
	case work.Warden:
		return "Guarding prisoners"
	case work.PlantCut:
		return "Cutting plants"
	}
	return ""
}

// ActivityLine is what a person is doing now. A flight, a raid plan and an
// attack are said ahead of the plain job.
func ActivityLine(p game.PawnView, c dossierview.Card) Line {
	l := Line{Tab: TabWork, Tip: JoinTip(ActivityTip(c))}
	switch {
	case p.Fleeing:
		l.Text, l.Tone = "Fleeing", ToneWarning
	case p.RaidPlan != "":
		l.Text = "Raiding (" + p.RaidPlan + ")"
	case c.AttackTarget != "":
		l.Text = "Attacking " + c.AttackTarget
	case p.Job != "" && p.Job != "idle":
		l.Text = capFirst(jobWords(p.Job))
	case p.Acting != "":
		l.Text = capFirst(p.Acting)
	default:
		l.Text, l.Tone = "Idle", ToneOff
	}
	return l
}

// jobWords is a job line without the state the simulation appends to it
// (job.Record.Line): "wander goto" reads "wander".
func jobWords(line string) string {
	for _, state := range []string{" goto", " do", " complete"} {
		if t, ok := strings.CutSuffix(line, state); ok {
			return t
		}
	}
	return line
}

// AnimalActivityLine is what a creature is doing now.
func AnimalActivityLine(a game.AnimalView, c dossierview.Card) Line {
	l := Line{Tab: TabTraining}
	switch {
	case c.AttackTarget != "":
		l.Text = "Attacking " + c.AttackTarget
	case a.State != "":
		l.Text = capFirst(a.State)
	default:
		l.Text, l.Tone = "Idle", ToneOff
	}
	return l
}

// ActivityTip names the work type and the skills it draws on.
func ActivityTip(c dossierview.Card) []TipLine {
	var out []TipLine
	if c.WorkType >= 0 && c.WorkType < int(work.NumTypes) {
		t := work.Type(c.WorkType)
		out = append(out, TipLine{Text: "Work: " + WorkLabel(t)})
		if id, ok := skill.ForWork(t); ok {
			out = append(out, TipLine{Text: "Draws on " + id.Label()})
		}
	}
	if c.ActiveSkill >= 0 && c.ActiveSkill < int(skill.NumSkills) {
		out = append(out, TipLine{Text: "Practising " + skill.ID(c.ActiveSkill).Label()})
	}
	return out
}

// QueuedLine is the first waiting order and how many wait behind it; empty
// when none waits.
func QueuedLine(p game.PawnView) Line {
	q := p.QueuedOrders
	if len(q) == 0 {
		return Line{}
	}
	text := "Next: " + q[0].Label
	if len(q) > 1 {
		text += " (+" + strconv.Itoa(len(q)-1) + ")"
	}
	return Line{Text: text, Tab: TabWork}
}

// EquippedLine is the restraints when held, else the weapon in hand.
func EquippedLine(p game.PawnView) Line {
	if p.Prisoner {
		return Line{Text: "Held captive", Tone: ToneWarning, Tab: TabGear}
	}
	if p.Weapon == "" || p.Weapon == "fists" {
		return Line{Text: "Unarmed", Tone: ToneOff, Tab: TabGear}
	}
	return Line{Text: "Armed: " + p.Weapon, Tab: TabGear}
}

// CarryingLine is what is held in the arms; empty when nothing is.
func CarryingLine(p game.PawnView) Line {
	switch p.Carrying {
	case "":
		return Line{}
	case "pawn":
		return Line{Text: "Carrying a person", Tab: TabGear}
	}
	return Line{Text: "Carrying " + p.Carrying, Tab: TabGear}
}

// TraitsLine lists the traits; empty when there are none.
func TraitsLine(b game.BioView) Line {
	if len(b.Traits) == 0 {
		return Line{}
	}
	names := make([]string, len(b.Traits))
	for i, t := range b.Traits {
		names[i] = t.Label
	}
	return Line{Text: strings.Join(names, ", "), Tab: TabBio, Tip: JoinTip(TraitsTip(b))}
}

// CaptiveLine is a prisoner's resistance and where they came from; empty for
// anyone who is not held.
func CaptiveLine(p game.PawnView, pr game.PrisonerView, c dossierview.Card) Line {
	if !p.Prisoner {
		return Line{}
	}
	faction := c.Faction
	if faction == "" {
		faction = p.GuestFaction
	}
	text := "Resistance " + strconv.Itoa(pr.Resistance)
	if faction == "" {
		text += ", no faction"
	} else {
		text += ", from " + faction
	}
	return Line{Text: text, Tone: ToneWarning, Tab: TabPrisoner}
}

// KeeperLine is the colonist a creature follows, tinted good when they are
// bonded.
func KeeperLine(a game.AnimalView) Line {
	l := Line{Tab: TabTraining, Tip: JoinTip(AnimalTip(a))}
	switch {
	case a.Master == "":
		l.Text, l.Tone = "No master", ToneOff
	case a.Bond != "":
		l.Text, l.Tone = "Master: "+a.Master+", bonded", ToneGood
	default:
		l.Text = "Master: " + a.Master
	}
	return l
}

// TallyLine is one running count; empty for an unknown key.
func TallyLine(p game.PawnView, def string) Line {
	switch def {
	case "kills":
		return Line{Text: "Kills: " + strconv.Itoa(p.Kills)}
	case "damage":
		return Line{Text: "Damage dealt: " + strconv.Itoa(p.DamageDealt)}
	}
	return Line{}
}

// NeedValue reads one of the other needs off a person: its name and percent.
func NeedValue(p game.PawnView, def string) (label string, pct int, ok bool) {
	switch def {
	case "comfort":
		return "Comfort", p.Comfort, true
	case "beauty":
		return "Beauty", p.Beauty, true
	case "outdoors":
		return "Outdoors", p.Outdoors, true
	}
	return "", 0, false
}

// FormatStat prints a stat's value in its unit. Decimals false rounds a
// percentage to a whole number.
func FormatStat(id stat.ID, v int32, decimals bool) string {
	switch id {
	case stat.MoodOffset, stat.BreakThreshold, stat.ShootAcc, stat.MeleeAcc:
		return signedInt(int(v))
	case stat.PainTolerance:
		return strconv.Itoa(int(v)) + "%"
	case stat.ColdComfort, stat.HeatComfort:
		neg := v < 0
		a := int(v)
		if neg {
			a = -a
		}
		s := strconv.Itoa(a/10) + "." + strconv.Itoa(a%10) + " °C"
		if neg {
			return "-" + s
		}
		return "+" + s
	}
	if decimals {
		return pctText(int(v))
	}
	return strconv.Itoa((int(v)+5)/10) + "%"
}

// StatLine is one stat as "Label: value"; empty when the stat is off (-1) or
// the key is unknown.
func StatLine(c dossierview.Card, def string, decimals bool) Line {
	id, ok := StatByKey(def)
	if !ok || c.Stats[id] == -1 {
		return Line{}
	}
	return Line{Text: id.Label() + ": " + FormatStat(id, c.Stats[id], decimals)}
}
