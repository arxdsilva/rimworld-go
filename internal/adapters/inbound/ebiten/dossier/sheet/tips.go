package sheet

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
)

// TipLine is one line of a tooltip with its tone.
type TipLine struct {
	Text string
	Tone Tone
}

// JoinTip joins lines to plain text, one a line, for callers that do not
// colour them.
func JoinTip(ls []TipLine) string {
	if len(ls) == 0 {
		return ""
	}
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = l.Text
	}
	return strings.Join(parts, "\n")
}

// BioTip is the life story: name and calling, backstories, traits, and what
// the pawn cannot do (subdued).
func BioTip(tr lang.Tr, p game.PawnView, b game.BioView, c dossierview.Card) []TipLine {
	head := p.Name
	if kind := joinNonEmpty(" ", p.Title); kind != "" {
		head = lang.Tf(tr, "%s, %s", head, lowerFirst(kind))
	}
	if c.Faction != "" {
		head = lang.Tf(tr, "%s of %s", head, c.Faction)
	}
	out := []TipLine{{Text: head}}
	if b.Childhood != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Childhood: %s", b.Childhood)})
	}
	if b.Adulthood != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Adulthood: %s", b.Adulthood)})
	}
	if len(b.Traits) > 0 {
		names := make([]string, len(b.Traits))
		for i, t := range b.Traits {
			names[i] = t.Label
		}
		out = append(out, TipLine{Text: lang.Tf(tr, "Traits: %s", strings.Join(names, ", "))})
	}
	for _, inc := range b.Incapable {
		out = append(out, TipLine{Text: lang.Tf(tr, "Cannot do: %s", inc), Tone: ToneOff})
	}
	return out
}

// TraitsTip gives each trait with what it does.
func TraitsTip(_ lang.Tr, b game.BioView) []TipLine {
	var out []TipLine
	for _, t := range b.Traits {
		out = append(out, TipLine{Text: t.Label})
		if t.Desc != "" {
			out = append(out, TipLine{Text: "  " + t.Desc, Tone: ToneNeutral})
		}
		if t.Effects != "" {
			out = append(out, TipLine{Text: "  " + t.Effects, Tone: ToneNeutral})
		}
	}
	return out
}

func permanent(pt game.PartView) bool {
	s := strings.ToLower(pt.Status)
	return pt.Implant != "" || strings.Contains(s, "missing") || strings.Contains(s, "scar") || strings.Contains(s, "artificial")
}

func partTone(pt game.PartView) Tone {
	s := strings.ToLower(pt.Status)
	switch {
	case permanent(pt):
		return ToneNeutral
	case s == "healthy" || s == "ok":
		return ToneGood
	case pt.TendTip != "" && !strings.Contains(strings.ToLower(pt.TendTip), "untended"):
		return ToneWarning
	}
	return ToneCritical
}

// HealthTip lists what is wrong, part by part, then conditions, drugs and
// pain. Tone: permanent and immune neutral, tended and addictions warning,
// anything else bad critical; "Nothing wrong" when there is nothing. Every
// tone is also in the words ("permanent", "immune", "tended", "untended").
func HealthTip(tr lang.Tr, h game.HealthView) []TipLine {
	var out []TipLine
	for _, pt := range h.Parts {
		text := pt.Label + ": " + pt.Status
		if pt.Implant != "" {
			text = lang.Tf(tr, "%s (%s)", text, pt.Implant)
		}
		tone := partTone(pt)
		switch {
		case permanent(pt):
			text = lang.Tf(tr, "%s, permanent", text)
		case tone == ToneCritical:
			text = lang.Tf(tr, "%s, %d HP, untended", text, pt.HP)
		case tone == ToneWarning:
			text = lang.Tf(tr, "%s, tended", text)
		}
		if pt.Bleed > 0 {
			text = lang.Tf(tr, "%s, bleeding %d%% a day", text, pt.Bleed)
		}
		out = append(out, TipLine{Text: text, Tone: tone})
		for _, ln := range strings.Split(pt.TendTip, "\n") {
			if ln = strings.TrimSpace(ln); ln != "" {
				lt := ToneWarning
				if strings.Contains(strings.ToLower(ln), "untended") {
					lt = ToneCritical
				}
				out = append(out, TipLine{Text: "  " + ln, Tone: lt})
			}
		}
	}
	for _, cv := range h.Conditions {
		text, tone := cv.Label+" "+strconv.Itoa(cv.SevPct)+"%", ToneCritical
		switch {
		case cv.ImmPct >= 100:
			text, tone = lang.Tf(tr, "%s, immune", text), ToneNeutral
		case cv.Losing:
			text = lang.Tf(tr, "%s, life-threatening", text)
		case cv.Due:
			text = lang.Tf(tr, "%s, needs tending", text)
		case cv.TendPct > 0:
			text, tone = lang.Tf(tr, "%s, tended %d%%", text, cv.TendPct), ToneWarning
		default:
			text = lang.Tf(tr, "%s, untended", text)
		}
		out = append(out, TipLine{Text: text, Tone: tone})
	}
	for _, d := range h.Drugs {
		out = append(out, TipLine{Text: lang.Tf(tr, "%s dependence %d%%", d.Label, d.Pct), Tone: ToneWarning})
	}
	if h.BloodLossPct > 0 {
		out = append(out, TipLine{Text: lang.Tf(tr, "Blood lost: %d%%", h.BloodLossPct), Tone: ToneWarning})
	}
	if h.Pain > 0 {
		out = append(out, TipLine{Text: lang.Tf(tr, "Pain: %d%%", h.Pain), Tone: ToneNeutral})
	}
	if len(out) == 0 {
		return []TipLine{{Text: tr.T("Nothing wrong"), Tone: ToneOff}}
	}
	return out
}

// MindTip is the mood with every thought, equal thoughts merged with a count,
// the value toned at -10, 0 and 10, and where breaks begin.
func MindTip(tr lang.Tr, p game.PawnView, c dossierview.Card) []TipLine {
	out := []TipLine{{Text: lang.Tf(tr, "Mood: %d%%", p.Mood)}}
	if c.BreakAt.Minor > 0 || c.BreakAt.Major > 0 || c.BreakAt.Extreme > 0 {
		out = append(out, TipLine{Text: lang.Tf(tr, "Breaks start below %d (minor), %d (serious), %d (extreme)",
			c.BreakAt.Minor, c.BreakAt.Major, c.BreakAt.Extreme), Tone: ToneOff})
	}
	type merged struct {
		label string
		total int
		n     int
	}
	var ms []merged
	for _, t := range p.Thoughts {
		found := false
		for i := range ms {
			if ms[i].label == t.Label {
				ms[i].total += t.Value
				ms[i].n++
				found = true
				break
			}
		}
		if !found {
			ms = append(ms, merged{t.Label, t.Value, 1})
		}
	}
	for _, m := range ms {
		text := signedInt(m.total) + " " + m.label
		if m.n > 1 {
			text += " (x" + strconv.Itoa(m.n) + ")"
		}
		out = append(out, TipLine{Text: text, Tone: MoodTipTone(m.total)})
	}
	if len(ms) == 0 {
		out = append(out, TipLine{Text: tr.T("No thoughts"), Tone: ToneOff})
	}
	if p.Inspiration != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Inspired: %s, %s left", p.Inspiration, ticksText(tr, p.InspirationTicks)), Tone: ToneTop})
	}
	return out
}

// FoodTip is what feeds the pawn and how fast it eats and gets hungry.
func FoodTip(tr lang.Tr, diet, policy string, c dossierview.Card) []TipLine {
	var out []TipLine
	if diet != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Diet: %s", diet)})
	}
	if policy != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Food policy: %s", policy)})
	}
	if c.EatSpeedPct > 0 {
		out = append(out, TipLine{Text: lang.Tf(tr, "Eating speed: %d%%", c.EatSpeedPct)})
	}
	if v := c.Stats[stat.HungerRate]; v != 0 {
		out = append(out, TipLine{Text: lang.Tf(tr, "Hunger rate: %s", pctText(int(v)))})
	}
	return out
}

// RestTip is how fast the pawn tires.
func RestTip(tr lang.Tr, c dossierview.Card) []TipLine {
	if v := c.Stats[stat.RestRate]; v != 0 {
		return []TipLine{{Text: lang.Tf(tr, "Rest rate: %s", pctText(int(v)))}}
	}
	return nil
}

// FunTip is what feeds recreation: beauty, comfort and the outdoors.
func FunTip(tr lang.Tr, p game.PawnView) []TipLine {
	return []TipLine{
		{Text: lang.Tf(tr, "Recreation: %d%%", p.Recreation)},
		{Text: lang.Tf(tr, "Beauty around: %d%%", p.Beauty)},
		{Text: lang.Tf(tr, "Comfort: %d%%", p.Comfort)},
		{Text: lang.Tf(tr, "Time outdoors: %d%%", p.Outdoors)},
	}
}

// AnimalTip is a creature's summary: what it is, what taming takes, how its
// training fades, what it eats.
func AnimalTip(tr lang.Tr, a game.AnimalView) []TipLine {
	head := capFirst(a.Label)
	if a.Name != "" {
		head = lang.Tf(tr, "%s, %s", a.Name, a.Label)
	}
	if a.Stage != "" {
		head = lang.Tf(tr, "%s (%s)", head, a.Stage)
	}
	out := []TipLine{{Text: head}}
	if !a.Tame {
		if a.MinHandling > 0 {
			out = append(out, TipLine{Text: lang.Tf(tr, "Needs Animals skill %d to tame", a.MinHandling)})
		}
		if a.Wildness > 0 {
			out = append(out, TipLine{Text: lang.Tf(tr, "Wildness: %d%%", a.Wildness)})
		}
		if a.Revenge > 0 {
			out = append(out, TipLine{Text: lang.Tf(tr, "Turns on whoever hurts it %d%% of the time", a.Revenge), Tone: ToneWarning})
		}
	} else {
		out = append(out, TipLine{Text: lang.Tf(tr, "A trained behaviour fades one step after %s without practice", ticksText(tr, training.DecayTicks))})
	}
	if a.Diet != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Eats: %s", a.Diet)})
	}
	if a.Product != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Gives %s", a.Product)})
	}
	if a.Master != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Master: %s", a.Master)})
	}
	if a.Bond != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Bonded with %s", a.Bond), Tone: ToneGood})
	}
	return out
}

// TrainDesc says in a sentence what a behaviour gives.
func TrainDesc(tr lang.Tr, s training.Skill) string {
	switch s {
	case training.Obedience:
		return tr.T("Comes when called and follows its master's orders.")
	case training.Guard:
		return tr.T("Stands watch over the colony and drives off threats.")
	case training.Release:
		return tr.T("Lets go of what it holds when told to.")
	}
	return ""
}

// TrainTip is a behaviour's description, progress and how it fades.
func TrainTip(tr lang.Tr, a game.AnimalView, t game.AnimalTrainingView) []TipLine {
	return append(AnimalTip(tr, a), trainLines(tr, t)...)
}

// trainLines is what one behaviour teaches and how far along it is.
func trainLines(tr lang.Tr, t game.AnimalTrainingView) []TipLine {
	s := training.Skill(t.Skill)
	out := []TipLine{{Text: t.Label}}
	if d := TrainDesc(tr, s); d != "" {
		out = append(out, TipLine{Text: d, Tone: ToneNeutral})
	}
	out = append(out, TipLine{Text: lang.Tf(tr, "Steps learned: %d of %d", t.Steps, t.Needed)})
	out = append(out, TipLine{Text: lang.Tf(tr, "Fades one step after %s without practice", ticksText(tr, training.DecayTicks)), Tone: ToneOff})
	if !t.On && !t.Learned {
		out = append(out, TipLine{Text: tr.T("Not being trained"), Tone: ToneOff})
	}
	return out
}

func percentLine(label string, permille int) TipLine {
	return TipLine{Text: label + ": " + pctText(permille)}
}

// SkillTip is the skill's description, then what its level does to the work.
// Nothing is listed past the description for a skill the pawn cannot use.
// weapon is the melee skill's weapon line ("" to leave it out). Lines for
// formulas that live in the simulation come from Card.ExtraLines.
func SkillTip(tr lang.Tr, s game.SkillView, c dossierview.Card, weapon string) []TipLine {
	out := []TipLine{{Text: s.Label}}
	if s.Affects != "" {
		out = append(out, TipLine{Text: lang.Tf(tr, "Drives: %s", s.Affects), Tone: ToneNeutral})
	}
	if s.Incapable {
		return append(out, TipLine{Text: tr.T("This pawn cannot do this work"), Tone: ToneOff})
	}
	id, ok := skill.Parse(s.Key)
	speed := s.SpeedPct * 10
	if s.SpeedPct == 0 {
		speed = skill.SpeedPermil(uint8(min(max(s.Level, 0), skill.MaxLevel)))
	}
	if w := c.Stats[stat.WorkSpeed]; w != 0 {
		out = append(out, percentLine(tr.T("Global work speed"), int(w)))
	}
	if !ok {
		return out
	}
	lv := min(max(s.Level, 0), skill.MaxLevel)
	switch id {
	case skill.Animals:
		out = append(out, percentLine(tr.T("Handling speed"), speed), percentLine(tr.T("Gather yield"), animal.GatherYield(lv)),
			percentLine(tr.T("Chance to tame at typical wildness (50%)"), animal.TameChance(lv, 500)))
	case skill.Construction:
		out = append(out, percentLine(tr.T("Building and smoothing speed"), speed))
	case skill.Cooking:
		out = append(out, percentLine(tr.T("Cooking and butchery speed"), speed))
	case skill.Crafting:
		out = append(out, percentLine(tr.T("Crafting and smelting speed"), speed))
	case skill.Intellectual:
		out = append(out, percentLine(tr.T("Research speed"), speed))
	case skill.Medicine:
		q := body.TendQuality(body.TendInput{Skill: lv, Manip: 1000, Sight: 1000, Medicine: item.Food})
		out = append(out, percentLine(tr.T("Doctoring speed"), speed), percentLine(tr.T("Tend quality without medicine"), int(q)))
	case skill.Melee:
		if weapon != "" {
			out = append(out, TipLine{Text: weapon})
		}
		out = append(out, TipLine{Text: lang.Tf(tr, "Melee accuracy: %s", FormatStat(stat.MeleeAcc, c.Stats[stat.MeleeAcc], false))})
	case skill.Mining:
		out = append(out, percentLine(tr.T("Mining and drilling speed"), speed))
	case skill.Plants:
		out = append(out, percentLine(tr.T("Planting and harvesting speed"), speed))
	case skill.Shooting:
		out = append(out, TipLine{Text: lang.Tf(tr, "Shooting accuracy: %s", FormatStat(stat.ShootAcc, c.Stats[stat.ShootAcc], false))},
			percentLine(tr.T("Aim time"), int(c.Stats[stat.AimCooldown])))
	case skill.Social:
		out = append(out, percentLine(tr.T("Negotiation ability"), int(c.Stats[stat.NegotiationAbility])),
			percentLine(tr.T("Trade price improvement"), int(c.Stats[stat.TradePriceImprovement])),
			percentLine(tr.T("Social impact"), int(c.Stats[stat.SocialImpact])))
	}
	for _, x := range c.ExtraLines[id] {
		out = append(out, TipLine{Text: x.Label + ": " + strconv.Itoa(x.Pct) + "%"})
	}
	switch SkillStateOf(s, c) {
	case SkillFull:
		out = append(out, TipLine{Text: tr.T("Learning is capped for today"), Tone: ToneNeutral})
	case SkillActive:
		out = append(out, TipLine{Text: tr.T("In use right now"), Tone: ToneNeutral})
	}
	if v := c.LearnedWeek[id]; v > 0 {
		out = append(out, TipLine{Text: lang.Tf(tr, "Learned this week: %d", v), Tone: ToneNeutral})
	}
	return out
}

// DecayDays is how many days a trained step lasts, for UI that wants a number.
func DecayDays() int { return int(training.DecayTicks / clock.TicksPerDay) }

// WeaponLine is the wielded weapon's damage a hit and a second, for the melee
// skill's tooltip; "" when the stats are empty.
func WeaponLine(tr lang.Tr, p game.PawnView) string {
	w := p.WeaponStats
	cycle := w.Warmup + w.Cooldown
	if w.Name == "" || w.Damage <= 0 || cycle <= 0 {
		return ""
	}
	tenths := w.Damage * max(w.Burst, 1) * clock.TicksPerSecond * 10 / cycle
	return lang.Tf(tr, "%s: %d damage a hit, %d.%d a second", capFirst(w.Name), w.Damage, tenths/10, tenths%10)
}
