package sheet

import (
	"sort"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
)

// Inspector tab names a line's Tab can hold.
const (
	TabBio      = "bio"
	TabHealth   = "health"
	TabNeeds    = "needs"
	TabGear     = "gear"
	TabWork     = "work"
	TabSocial   = "social"
	TabPrisoner = "prisoner"
	TabTraining = "training"
)

// Health cuts, percent of hit points.
const (
	HealthCritical = 30
	HealthPoor     = 60
	HealthGood     = 90
)

// Mood cuts for the mind line.
const (
	MoodGood = 65
	MoodTop  = 90
)

// RelationOf is how the colony sees a person. The pawn's own flags win over
// the card for the sides they name; a visitor never reads as the colony's.
func RelationOf(p game.PawnView, c dossierview.Card) dossierview.Relation {
	switch {
	case p.Hostile:
		return dossierview.RelationFoe
	case p.Prisoner:
		return dossierview.RelationHeld
	case p.Ally:
		return dossierview.RelationAlly
	case p.Visitor && c.Relation == dossierview.RelationOwn:
		return dossierview.RelationNone
	}
	return c.Relation
}

// AnimalRelation is how the colony sees a creature.
func AnimalRelation(a game.AnimalView) dossierview.Relation {
	switch {
	case a.Tame:
		return dossierview.RelationOwn
	case a.Hostile || a.Manhunter || (a.Mech && !a.Tame):
		return dossierview.RelationFoe
	}
	return dossierview.RelationWild
}

// StandingLine says who a person is to the colony and what they are:
// "Prisoner, Convoy mechanic of Ashen Court".
func StandingLine(tr lang.Tr, p game.PawnView, b game.BioView, c dossierview.Card) Line {
	rel := RelationOf(p, c)
	faction := c.Faction
	if faction == "" {
		faction = p.GuestFaction
	}
	adj := ""
	switch rel {
	case dossierview.RelationFoe:
		adj = tr.T("Hostile")
	case dossierview.RelationHeld:
		adj = tr.T("Prisoner")
	case dossierview.RelationAlly:
		adj = tr.T("Allied")
	case dossierview.RelationNone:
		adj = tr.T("Drifter")
		if faction != "" {
			adj = tr.T("Visitor")
		}
	}
	kind := c.Role
	if kind == "" {
		kind = p.Title
	}
	text := joinNonEmpty(", ", adj, kind)
	if text == "" {
		text = tr.T("Colonist")
	}
	if faction != "" && rel != dossierview.RelationOwn {
		text = lang.Tf(tr, "%s of %s", text, faction)
	}
	return Line{Text: text, Tone: RelationTone(rel), Tab: TabBio, Tip: JoinTip(BioTip(tr, p, b, c))}
}

// AnimalKind is the creature's kind in words: "Wild predator", "Mech unit",
// "Tame hunter", "Farm animal".
func AnimalKind(tr lang.Tr, a game.AnimalView) string {
	sp, known := animal.ParseSpecies(a.Species)
	insect := known && animal.AnatomyOf(sp) == body.Insect
	switch {
	case a.Mech:
		if a.Hostile || !a.Tame {
			return tr.T("Hostile mech unit")
		}
		return tr.T("Mech unit")
	case !a.Tame:
		switch {
		case a.Manhunter:
			return tr.T("Manhunter")
		case known && animal.DefOf(sp).Predator:
			return tr.T("Wild predator")
		case insect:
			return tr.T("Wild insect")
		}
		return tr.T("Wild creature")
	case a.Hunts:
		return tr.T("Tame hunter")
	case insect:
		return tr.T("Tame insect")
	case a.Product != "":
		return tr.T("Farm animal")
	case a.Herd != 0:
		return tr.T("Herd animal")
	}
	return tr.T("Tame animal")
}

// AnimalStandingLine is StandingLine for a creature.
func AnimalStandingLine(tr lang.Tr, a game.AnimalView, c dossierview.Card) Line {
	rel := AnimalRelation(a)
	text := AnimalKind(tr, a)
	if c.Faction != "" {
		text = lang.Tf(tr, "%s of %s", text, c.Faction)
	}
	return Line{Text: text, Tone: RelationTone(rel), Tab: TabBio, Tip: JoinTip(AnimalTip(tr, a))}
}

// ageText reads an age: "age 31, birthday today", "age 31 and 1 day".
func ageText(tr lang.Tr, years, dayOfYear int) string {
	switch {
	case dayOfYear <= 0 && years > 0:
		return lang.Tf(tr, "age %d, birthday today", years)
	case dayOfYear == 1:
		return lang.Tf(tr, "age %d and 1 day", years)
	case dayOfYear > 1:
		return lang.Tf(tr, "age %d and %d days", years, dayOfYear)
	}
	return lang.Tf(tr, "age %d", years)
}

// IdentityLine is a person's sex and age.
func IdentityLine(tr lang.Tr, p game.PawnView, b game.BioView, c dossierview.Card) Line {
	text := joinNonEmpty(", ", capFirst(p.Sex), ageText(tr, p.AgeYears, c.AgeDayOfYear))
	if p.Sex == "" {
		text = capFirst(ageText(tr, p.AgeYears, c.AgeDayOfYear))
	}
	return Line{Text: text, Tone: RelationTone(RelationOf(p, c)), Tab: TabBio, Tip: JoinTip(BioTip(tr, p, b, c))}
}

// AnimalIdentityLine is a creature's sex, kind and age: "Female arctic fox,
// age 1 and 12 days". The word for the kind is left out for people only.
func AnimalIdentityLine(tr lang.Tr, a game.AnimalView, c dossierview.Card) Line {
	sex := tr.T("Male")
	if a.Female {
		sex = tr.T("Female")
	}
	years, doy := a.AgeDays/clock.DaysPerYear, a.AgeDays%clock.DaysPerYear
	age := ageText(tr, years, doy)
	if years == 0 {
		age = lang.Tf(tr, "age %d days", doy)
		if doy == 1 {
			age = tr.T("age 1 day")
		}
	}
	text := lang.Tf(tr, "%s %s, %s", sex, a.Label, age)
	return Line{Text: text, Tone: RelationTone(AnimalRelation(a)), Tab: TabBio, Tip: JoinTip(AnimalTip(tr, a))}
}

// BodyLine is the worst thing wrong with a body, first match wins:
// dead, bleeding out within a day, untended wounds, life-threatening
// conditions, other conditions, downed, then how much health is left.
func BodyLine(tr lang.Tr, p game.PawnView, h game.HealthView) Line {
	text, tone := bodyState(tr, p, h)
	return Line{Text: text, Tone: tone, Tab: TabHealth, Tip: JoinTip(HealthTip(tr, h))}
}

func bodyState(tr lang.Tr, p game.PawnView, h game.HealthView) (string, Tone) {
	if p.Status == "dead" || p.Embodied {
		return tr.T("Dead"), ToneCritical
	}
	if p.Bleeding && p.BleedDeathHours < clock.HoursPerDay {
		if p.BleedDeathHours <= 0 {
			return tr.T("Bleeding (death within the hour)"), ToneCritical
		}
		return lang.Tf(tr, "Bleeding (death in %s)", hoursText(tr, p.BleedDeathHours)), ToneCritical
	}
	var due, losing, other []game.ConditionView
	for _, cv := range h.Conditions {
		switch {
		case cv.Due:
			due = append(due, cv)
		case cv.Losing:
			losing = append(losing, cv)
		default:
			other = append(other, cv)
		}
	}
	rows := len(h.Conditions) > 0
	if len(due) > 0 || (!rows && p.Wounded) {
		return untendedText(tr, p, due, rows)
	}
	if len(losing) > 0 || (!rows && p.InfectionLosing) {
		if len(losing) == 1 {
			return lang.Tf(tr, "%s (life-threatening)", losing[0].Label), ToneCritical
		}
		if len(losing) == 0 {
			return tr.T("Infection (life-threatening)"), ToneCritical
		}
		return lang.Tf(tr, "%d life-threatening conditions", len(losing)), ToneCritical
	}
	if len(other) > 0 || (!rows && p.Disease != "") {
		return affectedText(tr, p, other), ToneNeutral
	}
	switch {
	case p.Status == "downed":
		return tr.T("Downed"), ToneWarning
	case p.Health < HealthCritical:
		return tr.T("Gravely hurt"), ToneCritical
	case p.Health < HealthPoor:
		return tr.T("Poor health"), ToneWarning
	case p.Health > HealthGood:
		return tr.T("Healthy"), ToneGood
	}
	return tr.T("Stable"), ToneNeutral
}

func untendedText(tr lang.Tr, p game.PawnView, due []game.ConditionView, rows bool) (string, Tone) {
	n, nl := len(due), 0
	for _, cv := range due {
		if cv.Losing {
			nl++
		}
	}
	if !rows {
		n = 1
		if p.InfectionLosing {
			nl = 1
		}
	}
	var text string
	switch {
	case n == 1 && rows:
		text = lang.Tf(tr, "%s needs tending", due[0].Label)
	case n == 1:
		text = tr.T("A wound needs tending")
	default:
		text = lang.Tf(tr, "%d wounds need tending", n)
	}
	tone := ToneWarning
	if nl > 0 {
		tone = ToneCritical
		switch {
		case n == 1:
			text = lang.Tf(tr, "%s, life-threatening", text)
		case nl == 1:
			text = lang.Tf(tr, "%s, one life-threatening", text)
		default:
			text = lang.Tf(tr, "%s, %d life-threatening", text, nl)
		}
	}
	return text, tone
}

func affectedText(tr lang.Tr, p game.PawnView, other []game.ConditionView) string {
	if len(other) == 0 {
		return capFirst(p.Disease)
	}
	sort.SliceStable(other, func(i, j int) bool { return other[i].SevPct > other[j].SevPct })
	switch len(other) {
	case 1:
		return other[0].Label
	case 2:
		return other[0].Label + ", " + lowerFirst(other[1].Label)
	}
	return lang.Tf(tr, "%d conditions", len(other))
}

// AnimalBodyLine is BodyLine for a creature, which has only a hit-point
// percentage and a downed flag.
func AnimalBodyLine(tr lang.Tr, a game.AnimalView) Line {
	var text string
	var tone Tone
	switch {
	case a.Downed:
		text, tone = tr.T("Downed"), ToneWarning
	case a.Health < HealthCritical:
		text, tone = tr.T("Gravely hurt"), ToneCritical
	case a.Health < HealthPoor:
		text, tone = tr.T("Poor health"), ToneWarning
	case a.Health > HealthGood:
		text, tone = tr.T("Healthy"), ToneGood
	default:
		text, tone = tr.T("Stable"), ToneNeutral
	}
	return Line{Text: text, Tone: tone}
}

// MindLine is the state of the mind, first match wins: stunned, staggered, a
// break under way, a break close, inspiration, then how the mood reads.
func MindLine(tr lang.Tr, p game.PawnView, c dossierview.Card) Line {
	text, tone := mindState(tr, p, c)
	return Line{Text: text, Tone: tone, Tab: TabNeeds, Tip: JoinTip(MindTip(tr, p, c))}
}

func mindState(tr lang.Tr, p game.PawnView, c dossierview.Card) (string, Tone) {
	switch {
	case c.StunTicks > 0:
		return lang.Tf(tr, "Stunned for %d s", secondsOf(c.StunTicks)), ToneCritical
	case c.StaggerTicks > 0:
		return lang.Tf(tr, "Staggered for %d s", secondsOf(c.StaggerTicks)), ToneWarning
	case p.Staggered:
		return tr.T("Staggered"), ToneWarning
	case p.MentalBreak != "":
		tone := ToneWarning
		if c.BreakTier == mentalbreak.Extreme {
			tone = ToneCritical
		}
		return lang.Tf(tr, "Mental break: %s", p.MentalBreak), tone
	}
	switch mentalbreak.TierAt(p.Mood, mentalbreak.Thresholds(c.BreakAt)) {
	case mentalbreak.Extreme:
		return tr.T("Near collapse"), ToneCritical
	case mentalbreak.Major:
		return tr.T("Near a serious break"), ToneWarning
	case mentalbreak.Minor:
		return tr.T("Close to a minor break"), ToneWarning
	}
	if p.Inspiration != "" {
		return lang.Tf(tr, "Inspired: %s, %s left", p.Inspiration, ticksText(tr, p.InspirationTicks)), ToneTop
	}
	word := capFirst(mood.Label(p.Mood))
	switch {
	case p.Mood > MoodTop:
		return word, ToneTop
	case p.Mood > MoodGood:
		return word, ToneGood
	case p.Mood < mood.LowMood:
		return word, ToneWarning
	}
	return word, ToneNeutral
}

// AnimalMindLine is the mind state of a creature: stunned, staggered or
// hunting colonists; otherwise nothing to say.
func AnimalMindLine(tr lang.Tr, a game.AnimalView, c dossierview.Card) Line {
	switch {
	case c.StunTicks > 0:
		return Line{Text: lang.Tf(tr, "Stunned for %d s", secondsOf(c.StunTicks)), Tone: ToneCritical}
	case a.Stunned:
		return Line{Text: tr.T("Stunned"), Tone: ToneCritical}
	case c.StaggerTicks > 0:
		return Line{Text: lang.Tf(tr, "Staggered for %d s", secondsOf(c.StaggerTicks)), Tone: ToneWarning}
	case a.Staggered:
		return Line{Text: tr.T("Staggered"), Tone: ToneWarning}
	case a.Manhunter:
		return Line{Text: tr.T("Hunting colonists"), Tone: ToneCritical}
	}
	return Line{}
}

// WarnsInWords reports whether a line's text names the state its warning or
// critical tone stands for (spec §59). It is a guard for tests and for lines
// built from other text.
func WarnsInWords(l Line) bool {
	if l.Tone != ToneCritical && l.Tone != ToneWarning {
		return true
	}
	t := strings.ToLower(l.Text)
	for _, w := range []string{"dead", "bleed", "tend", "life-threat", "downed", "hurt", "poor", "break", "collapse",
		"stunned", "staggered", "fleeing", "hostile", "prisoner", "resistance", "miserable", "unhappy", "captive",
		"hunting"} {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}
