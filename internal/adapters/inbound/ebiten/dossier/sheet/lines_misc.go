package sheet

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

// WorkLabel is a work type in words.
func WorkLabel(tr lang.Tr, t work.Type) string {
	switch t {
	case work.Firefight:
		return tr.T("Firefighting")
	case work.Doctor:
		return tr.T("Doctoring")
	case work.Grow:
		return tr.T("Growing")
	case work.Cook:
		return tr.T("Cooking")
	case work.Construct:
		return tr.T("Construction")
	case work.Hunt:
		return tr.T("Hunting")
	case work.Haul:
		return tr.T("Hauling")
	case work.Clean:
		return tr.T("Cleaning")
	case work.Research:
		return tr.T("Research")
	case work.Mine:
		return tr.T("Mining")
	case work.Craft:
		return tr.T("Crafting")
	case work.Handle:
		return tr.T("Animal handling")
	case work.Warden:
		return tr.T("Guarding prisoners")
	case work.PlantCut:
		return tr.T("Cutting plants")
	}
	return ""
}

// ActivityLine is what a person is doing now. A flight, a raid plan and an
// attack are said ahead of the plain job.
func ActivityLine(tr lang.Tr, p game.PawnView, c dossierview.Card) Line {
	l := Line{Tab: TabWork, Tip: JoinTip(ActivityTip(tr, c))}
	switch {
	case p.Fleeing:
		l.Text, l.Tone = tr.T("Fleeing"), ToneWarning
	case p.Advancing && p.Hostile:
		l.Text, l.Tone = tr.T("Advancing on the colony"), ToneWarning
	case p.RaidPlan != "":
		l.Text = lang.Tf(tr, "Raiding (%s)", planWord(tr, p.RaidPlan))
	case c.AttackTarget != "":
		l.Text = lang.Tf(tr, "Attacking %s", c.AttackTarget)
	case p.Job != "" && p.Job != "idle":
		l.Text = capFirst(jobWords(p.Job))
	case p.Acting != "":
		l.Text = capFirst(p.Acting)
	default:
		l.Text, l.Tone = tr.T("Idle"), ToneOff
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
func AnimalActivityLine(tr lang.Tr, a game.AnimalView, c dossierview.Card) Line {
	l := Line{Tab: TabTraining}
	switch {
	case c.AttackTarget != "":
		l.Text = lang.Tf(tr, "Attacking %s", c.AttackTarget)
	case a.State != "":
		l.Text = capFirst(a.State)
	default:
		l.Text, l.Tone = tr.T("Idle"), ToneOff
	}
	return l
}

// ActivityTip names the work type and the skills it draws on.
func ActivityTip(tr lang.Tr, c dossierview.Card) []TipLine {
	var out []TipLine
	if c.WorkType >= 0 && c.WorkType < int(work.NumTypes) {
		t := work.Type(c.WorkType)
		out = append(out, TipLine{Text: lang.Tf(tr, "Work: %s", WorkLabel(tr, t))})
		if id, ok := skill.ForWork(t); ok {
			out = append(out, TipLine{Text: lang.Tf(tr, "Draws on %s", id.Label())})
		}
	}
	if c.ActiveSkill >= 0 && c.ActiveSkill < int(skill.NumSkills) {
		out = append(out, TipLine{Text: lang.Tf(tr, "Practising %s", skill.ID(c.ActiveSkill).Label())})
	}
	return out
}

// QueuedLine is the first waiting order and how many wait behind it; empty
// when none waits.
func QueuedLine(tr lang.Tr, p game.PawnView) Line {
	q := p.QueuedOrders
	if len(q) == 0 {
		return Line{}
	}
	text := lang.Tf(tr, "Next: %s", q[0].Label)
	if len(q) > 1 {
		text += " (+" + strconv.Itoa(len(q)-1) + ")"
	}
	return Line{Text: text, Tab: TabWork}
}

// EquippedLine is the restraints when held, else the weapon in hand.
func EquippedLine(tr lang.Tr, p game.PawnView) Line {
	if p.Prisoner {
		return Line{Text: tr.T("Held captive"), Tone: ToneWarning, Tab: TabGear}
	}
	if p.Weapon == "" || p.Weapon == "fists" {
		return Line{Text: tr.T("Unarmed"), Tone: ToneOff, Tab: TabGear}
	}
	return Line{Text: lang.Tf(tr, "Armed: %s", p.Weapon), Tab: TabGear}
}

// CarryingLine is what is held in the arms; empty when nothing is.
func CarryingLine(tr lang.Tr, p game.PawnView) Line {
	switch p.Carrying {
	case "":
		return Line{}
	case "pawn":
		return Line{Text: tr.T("Carrying a person"), Tab: TabGear}
	}
	return Line{Text: lang.Tf(tr, "Carrying %s", p.Carrying), Tab: TabGear}
}

// TraitsLine lists the traits; empty when there are none.
func TraitsLine(tr lang.Tr, b game.BioView) Line {
	if len(b.Traits) == 0 {
		return Line{}
	}
	names := make([]string, len(b.Traits))
	for i, t := range b.Traits {
		names[i] = t.Label
	}
	return Line{Text: strings.Join(names, ", "), Tab: TabBio, Tip: JoinTip(TraitsTip(tr, b))}
}

// CaptiveLine is a prisoner's resistance and where they came from; empty for
// anyone who is not held.
func CaptiveLine(tr lang.Tr, p game.PawnView, pr game.PrisonerView, c dossierview.Card) Line {
	if !p.Prisoner {
		return Line{}
	}
	faction := c.Faction
	if faction == "" {
		faction = p.GuestFaction
	}
	text := lang.Tf(tr, "Resistance %d", pr.Resistance)
	if faction == "" {
		text = lang.Tf(tr, "%s, no faction", text)
	} else {
		text = lang.Tf(tr, "%s, from %s", text, faction)
	}
	return Line{Text: text, Tone: ToneWarning, Tab: TabPrisoner}
}

// KeeperLine is the colonist a creature follows, tinted good when they are
// bonded.
func KeeperLine(tr lang.Tr, a game.AnimalView) Line {
	l := Line{Tab: TabTraining, Tip: JoinTip(AnimalTip(tr, a))}
	switch {
	case a.Master == "":
		l.Text, l.Tone = tr.T("No master"), ToneOff
	case a.Bond != "":
		l.Text, l.Tone = lang.Tf(tr, "Master: %s, bonded", a.Master), ToneGood
	default:
		l.Text = lang.Tf(tr, "Master: %s", a.Master)
	}
	return l
}

// TallyLine is one running count; empty for an unknown key.
func TallyLine(tr lang.Tr, p game.PawnView, def string) Line {
	switch def {
	case "kills":
		return Line{Text: lang.Tf(tr, "Kills: %d", p.Kills)}
	case "damage":
		return Line{Text: lang.Tf(tr, "Damage dealt: %d", p.DamageDealt)}
	}
	return Line{}
}

// NeedValue reads one of the other needs off a person: its name and percent.
func NeedValue(tr lang.Tr, p game.PawnView, def string) (label string, pct int, ok bool) {
	switch def {
	case "comfort":
		return tr.T("Comfort"), p.Comfort, true
	case "beauty":
		return tr.T("Beauty"), p.Beauty, true
	case "outdoors":
		return tr.T("Outdoors"), p.Outdoors, true
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

// planWord is a raid plan's name in the player's language (EAI-016); a plan
// the sheet does not know keeps the game's word.
func planWord(tr lang.Tr, plan string) string {
	switch plan {
	case "assault":
		return tr.T("assault")
	case "siege":
		return tr.T("siege")
	case "sappers":
		return tr.T("sappers")
	case "staged assault":
		return tr.T("staged assault")
	case "steal":
		return tr.T("steal")
	case "burn":
		return tr.T("burn")
	case "defend":
		return tr.T("defend")
	}
	return plan
}
