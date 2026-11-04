package sheet

import (
	"sort"
	"strconv"
	"strings"

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
func StandingLine(p game.PawnView, b game.BioView, c dossierview.Card) Line {
	rel := RelationOf(p, c)
	faction := c.Faction
	if faction == "" {
		faction = p.GuestFaction
	}
	adj := ""
	switch rel {
	case dossierview.RelationFoe:
		adj = "Hostile"
	case dossierview.RelationHeld:
		adj = "Prisoner"
	case dossierview.RelationAlly:
		adj = "Allied"
	case dossierview.RelationNone:
		adj = "Drifter"
		if faction != "" {
			adj = "Visitor"
		}
	}
	kind := c.Role
	if kind == "" {
		kind = p.Title
	}
	text := joinNonEmpty(", ", adj, kind)
	if text == "" {
		text = "Colonist"
	}
	if faction != "" && rel != dossierview.RelationOwn {
		text += " of " + faction
	}
	return Line{Text: text, Tone: RelationTone(rel), Tab: TabBio, Tip: JoinTip(BioTip(p, b, c))}
}

// AnimalKind is the creature's kind in words: "Wild predator", "Mech unit",
// "Tame hunter", "Farm animal".
func AnimalKind(a game.AnimalView) string {
	sp, known := animal.ParseSpecies(a.Species)
	insect := known && animal.AnatomyOf(sp) == body.Insect
	switch {
	case a.Mech:
		if a.Hostile || !a.Tame {
			return "Hostile mech unit"
		}
		return "Mech unit"
	case !a.Tame:
		switch {
		case a.Manhunter:
			return "Manhunter"
		case known && animal.DefOf(sp).Predator:
			return "Wild predator"
		case insect:
			return "Wild insect"
		}
		return "Wild creature"
	case a.Hunts:
		return "Tame hunter"
	case insect:
		return "Tame insect"
	case a.Product != "":
		return "Farm animal"
	case a.Herd != 0:
		return "Herd animal"
	}
	return "Tame animal"
}

// AnimalStandingLine is StandingLine for a creature.
func AnimalStandingLine(a game.AnimalView, c dossierview.Card) Line {
	rel := AnimalRelation(a)
	text := AnimalKind(a)
	if c.Faction != "" {
		text += " of " + c.Faction
	}
	return Line{Text: text, Tone: RelationTone(rel), Tab: TabBio, Tip: JoinTip(AnimalTip(a))}
}

// ageText reads an age: "age 31, birthday today", "age 31 and 1 day".
func ageText(years, dayOfYear int) string {
	a := "age " + strconv.Itoa(years)
	switch {
	case dayOfYear <= 0 && years > 0:
		return a + ", birthday today"
	case dayOfYear == 1:
		return a + " and 1 day"
	case dayOfYear > 1:
		return a + " and " + strconv.Itoa(dayOfYear) + " days"
	}
	return a
}

// IdentityLine is a person's sex and age.
func IdentityLine(p game.PawnView, b game.BioView, c dossierview.Card) Line {
	text := joinNonEmpty(", ", capFirst(p.Sex), ageText(p.AgeYears, c.AgeDayOfYear))
	if p.Sex == "" {
		text = capFirst(ageText(p.AgeYears, c.AgeDayOfYear))
	}
	return Line{Text: text, Tone: RelationTone(RelationOf(p, c)), Tab: TabBio, Tip: JoinTip(BioTip(p, b, c))}
}

// AnimalIdentityLine is a creature's sex, kind and age: "Female arctic fox,
// age 1 and 12 days". The word for the kind is left out for people only.
func AnimalIdentityLine(a game.AnimalView, c dossierview.Card) Line {
	sex := "Male"
	if a.Female {
		sex = "Female"
	}
	years, doy := a.AgeDays/clock.DaysPerYear, a.AgeDays%clock.DaysPerYear
	age := ageText(years, doy)
	if years == 0 {
		age = "age " + strconv.Itoa(doy) + " days"
		if doy == 1 {
			age = "age 1 day"
		}
	}
	text := sex + " " + a.Label + ", " + age
	return Line{Text: text, Tone: RelationTone(AnimalRelation(a)), Tab: TabBio, Tip: JoinTip(AnimalTip(a))}
}

// BodyLine is the worst thing wrong with a body, first match wins:
// dead, bleeding out within a day, untended wounds, life-threatening
// conditions, other conditions, downed, then how much health is left.
func BodyLine(p game.PawnView, h game.HealthView) Line {
	text, tone := bodyState(p, h)
	return Line{Text: text, Tone: tone, Tab: TabHealth, Tip: JoinTip(HealthTip(h))}
}

func bodyState(p game.PawnView, h game.HealthView) (string, Tone) {
	if p.Status == "dead" || p.Embodied {
		return "Dead", ToneCritical
	}
	if p.Bleeding && p.BleedDeathHours < clock.HoursPerDay {
		if p.BleedDeathHours <= 0 {
			return "Bleeding (death within the hour)", ToneCritical
		}
		return "Bleeding (death in " + hoursText(p.BleedDeathHours) + ")", ToneCritical
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
		return untendedText(p, due, rows)
	}
	if len(losing) > 0 || (!rows && p.InfectionLosing) {
		if len(losing) == 1 {
			return losing[0].Label + " (life-threatening)", ToneCritical
		}
		if len(losing) == 0 {
			return "Infection (life-threatening)", ToneCritical
		}
		return strconv.Itoa(len(losing)) + " life-threatening conditions", ToneCritical
	}
	if len(other) > 0 || (!rows && p.Disease != "") {
		return affectedText(p, other), ToneNeutral
	}
	switch {
	case p.Status == "downed":
		return "Downed", ToneWarning
	case p.Health < HealthCritical:
		return "Gravely hurt", ToneCritical
	case p.Health < HealthPoor:
		return "Poor health", ToneWarning
	case p.Health > HealthGood:
		return "Healthy", ToneGood
	}
	return "Stable", ToneNeutral
}

func untendedText(p game.PawnView, due []game.ConditionView, rows bool) (string, Tone) {
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
		text = due[0].Label + " needs tending"
	case n == 1:
		text = "A wound needs tending"
	default:
		text = strconv.Itoa(n) + " wounds need tending"
	}
	tone := ToneWarning
	if nl > 0 {
		tone = ToneCritical
		switch {
		case n == 1:
			text += ", life-threatening"
		case nl == 1:
			text += ", one life-threatening"
		default:
			text += ", " + strconv.Itoa(nl) + " life-threatening"
		}
	}
	return text, tone
}

func affectedText(p game.PawnView, other []game.ConditionView) string {
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
	return strconv.Itoa(len(other)) + " conditions"
}

// AnimalBodyLine is BodyLine for a creature, which has only a hit-point
// percentage and a downed flag.
func AnimalBodyLine(a game.AnimalView) Line {
	var text string
	var tone Tone
	switch {
	case a.Downed:
		text, tone = "Downed", ToneWarning
	case a.Health < HealthCritical:
		text, tone = "Gravely hurt", ToneCritical
	case a.Health < HealthPoor:
		text, tone = "Poor health", ToneWarning
	case a.Health > HealthGood:
		text, tone = "Healthy", ToneGood
	default:
		text, tone = "Stable", ToneNeutral
	}
	return Line{Text: text, Tone: tone}
}

// MindLine is the state of the mind, first match wins: stunned, staggered, a
// break under way, a break close, inspiration, then how the mood reads.
func MindLine(p game.PawnView, c dossierview.Card) Line {
	text, tone := mindState(p, c)
	return Line{Text: text, Tone: tone, Tab: TabNeeds, Tip: JoinTip(MindTip(p, c))}
}

func mindState(p game.PawnView, c dossierview.Card) (string, Tone) {
	switch {
	case c.StunTicks > 0:
		return "Stunned for " + strconv.Itoa(secondsOf(c.StunTicks)) + " s", ToneCritical
	case c.StaggerTicks > 0:
		return "Staggered for " + strconv.Itoa(secondsOf(c.StaggerTicks)) + " s", ToneWarning
	case p.Staggered:
		return "Staggered", ToneWarning
	case p.MentalBreak != "":
		tone := ToneWarning
		if c.BreakTier == mentalbreak.Extreme {
			tone = ToneCritical
		}
		return "Mental break: " + p.MentalBreak, tone
	}
	switch mentalbreak.TierAt(p.Mood, mentalbreak.Thresholds(c.BreakAt)) {
	case mentalbreak.Extreme:
		return "Near collapse", ToneCritical
	case mentalbreak.Major:
		return "Near a serious break", ToneWarning
	case mentalbreak.Minor:
		return "Close to a minor break", ToneWarning
	}
	if p.Inspiration != "" {
		return "Inspired: " + p.Inspiration + ", " + ticksText(p.InspirationTicks) + " left", ToneTop
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
func AnimalMindLine(a game.AnimalView, c dossierview.Card) Line {
	switch {
	case c.StunTicks > 0:
		return Line{Text: "Stunned for " + strconv.Itoa(secondsOf(c.StunTicks)) + " s", Tone: ToneCritical}
	case a.Stunned:
		return Line{Text: "Stunned", Tone: ToneCritical}
	case c.StaggerTicks > 0:
		return Line{Text: "Staggered for " + strconv.Itoa(secondsOf(c.StaggerTicks)) + " s", Tone: ToneWarning}
	case a.Staggered:
		return Line{Text: "Staggered", Tone: ToneWarning}
	case a.Manhunter:
		return Line{Text: "Hunting colonists", Tone: ToneCritical}
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
