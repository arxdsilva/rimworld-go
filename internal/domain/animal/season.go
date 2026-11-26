// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"strconv"
	"strings"
)

// Season weights in permille (SIM-024, our tuning; moved from the game's
// wildlife.go in FOOD-6): a habitat's own season draws more of a species,
// the opposite one fewer; an any-climate species is drawn at
// weightAnyHab all year.
const (
	weightFavoured = 3000
	weightNormal   = 1000
	weightShunned  = 300
	weightAnyHab   = 600
)

// habitatWeight is how welcome one habitat is in season.
func habitatWeight(h Habitat, season string) int {
	switch {
	case h == HabCold && season == "Winter", h == HabHot && season == "Summer":
		return weightFavoured
	case h == HabCold && season == "Summer", h == HabHot && season == "Winter":
		return weightShunned
	}
	return weightNormal
}

// SeasonWeight is how much season favours species s (permille): the mean
// over its habitats, an any-climate species at 0.6.
func SeasonWeight(s Species, season string) int {
	r := RosterOf(s)
	if r.Habitat == HabAny {
		return weightAnyHab
	}
	w, n := 0, 0
	for _, h := range []Habitat{HabTemperate, HabCold, HabHot} {
		if r.Habitat&h != 0 {
			w += habitatWeight(h, season)
			n++
		}
	}
	if n == 0 {
		return weightNormal
	}
	return w / n
}

// Plural is a species' plural display name ("timber wolves").
func Plural(s Species) string {
	l := s.Label()
	switch {
	case strings.HasSuffix(l, "wolf"):
		return l[:len(l)-1] + "ves"
	case strings.HasSuffix(l, "goose"):
		return l[:len(l)-4] + "eese"
	case strings.HasSuffix(l, "x"), strings.HasSuffix(l, "ch"):
		return l + "es"
	case strings.HasSuffix(l, "deer"), strings.HasSuffix(l, "sheep"), strings.HasSuffix(l, "bison"),
		strings.HasSuffix(l, "elk"), strings.HasSuffix(l, "caribou"):
		return l
	case strings.HasSuffix(l, "y") && !strings.HasSuffix(l, "ey") && !strings.HasSuffix(l, "ay"):
		return l[:len(l)-1] + "ies"
	case strings.HasSuffix(l, "s"):
		return l + "es"
	}
	return l + "s"
}

// Bands of a biome's yearly mean (tenths of a degree C) for climate fit
// (our tuning; moved from the game's incidents.go in FOOD-6).
const (
	coldMeanMax = 20  // at or below: a cold land
	hotMeanMin  = 200 // at or above: a hot land
)

// ClimateBand is the habitat a yearly mean temperature calls for.
func ClimateBand(mean int) Habitat {
	switch {
	case mean <= coldMeanMax:
		return HabCold
	case mean >= hotMeanMin:
		return HabHot
	}
	return HabTemperate
}

// YoungName is n young of s for a log line ("2 goat kids").
func YoungName(s Species, n int) string {
	w := YoungWord(s)
	if n != 1 {
		switch {
		case w == "young":
		case strings.HasSuffix(w, "calf"):
			w = w[:len(w)-1] + "ves"
		default:
			w += "s"
		}
	}
	return s.Label() + " " + w
}

// YoungPhrase is n young of s in a sentence ("a goat kid", "2 goat
// kids").
func YoungPhrase(s Species, n int) string {
	switch {
	case n != 1:
		return strconv.Itoa(n) + " " + YoungName(s, n)
	case strings.ContainsRune("aeiou", rune(s.Label()[0])):
		return "an " + YoungName(s, 1)
	}
	return "a " + YoungName(s, 1)
}

// YoungSentence capitalizes the first word of a young-animal birth notice.
func YoungSentence(s Species, n int) string {
	phrase := YoungPhrase(s, n)
	if phrase == "" {
		return phrase
	}
	return strings.ToUpper(phrase[:1]) + phrase[1:]
}

// CountsWild: an animal counted against capacity. Tame animals, leavers
// (herd crossers among them) and event animals are not: manhunters, hive
// insects, legendaries and tree-eaters, which only come with events.
func CountsWild(a State, now uint64) bool {
	if a.Tame || a.Leaving || a.Enraged(now) || a.Dead() {
		return false
	}
	d := DefOf(a.Species)
	return !d.Hive && !d.Legendary && !d.Mech && EcologyOf(a.Species).Diet != Dendrovore
}
