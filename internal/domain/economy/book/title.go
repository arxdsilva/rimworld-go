package book

import (
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
	"rimworld-go/pkg/randx"
)

// Titles: every word list below is this game's own (ENG-003: no vanilla
// title or text is copied). A title is picked from the seed's low 16
// bits, so a book keeps its title wherever it goes.

var (
	novelAdj = []string{"Quiet", "Last", "Burning", "Hollow", "Silver", "Distant", "Broken", "Patient", "Crooked", "Bright",
		"Salt", "Long", "Hidden", "Iron", "Drowned", "Gentle"}
	novelNoun = []string{"Harbor", "Orchard", "Winter", "Lantern", "Road", "Tide", "Garden", "Bell", "Mountain", "River",
		"Ember", "Signal", "Field", "Comet", "Well", "Crown"}
	novelName = []string{"Mara", "Tobin", "Esk", "Delphine", "Oren", "Saffi", "Ivo", "Laurel"}
	novelForm = []string{"The %a %n", "%n of the %a %m", "A %n for %p", "%p and the %n", "The %n Keeper", "Under the %a %n",
		"%p's %a %n", "After the %n"}

	textIntro = []string{"A Primer of", "Practical", "Notes on", "Foundations of", "Lessons in", "A Field Manual of",
		"Plain Talk on", "The Working Guide to"}
	textTopic = [skill.NumSkills]string{"Marksmanship", "Close Combat", "Building", "Rock and Ore", "the Kitchen",
		"Growing Things", "Animal Keeping", "Handiwork", "Healing", "Persuasion", "Reasoning"}

	schemIntro = []string{"Plans for", "Drawings of", "Specifications for", "Working Notes on", "A Design for",
		"Sketches toward"}
)

// Title is the title of a book of kind k with seed s ("" for no book).
func Title(k item.Kind, s uint32) string {
	h := uint64(s & 0xffff)
	pick := func(l []string) string {
		h = randx.SplitMix64(h)
		return l[h%uint64(len(l))]
	}
	switch k {
	case item.Novel:
		f := pick(novelForm)
		a, n, m, p := pick(novelAdj), pick(novelNoun), pick(novelNoun), pick(novelName)
		return strings.NewReplacer("%a", a, "%n", n, "%m", m, "%p", p).Replace(f)
	case item.Textbook:
		var topics []string
		for _, sk := range Skills(k, s) {
			topics = append(topics, textTopic[sk])
		}
		if len(topics) == 0 {
			topics = append(topics, textTopic[skill.Intellectual])
		}
		return pick(textIntro) + " " + strings.Join(topics, " and ")
	case item.Schematic:
		var names []string
		for _, p := range Projects(k, s) {
			names = append(names, research.DefOf(p).Label)
		}
		if len(names) == 0 {
			names = append(names, "Unknown Machines")
		}
		return pick(schemIntro) + " " + strings.Join(names, " and ")
	}
	return ""
}

// Teaches is "Construction, Mining" for a textbook's skills or a
// schematic's projects ("" for a novel).
func Teaches(k item.Kind, s uint32) string {
	var out []string
	for _, sk := range Skills(k, s) {
		out = append(out, sk.Label())
	}
	for _, p := range Projects(k, s) {
		out = append(out, research.DefOf(p).Label)
	}
	return strings.Join(out, ", ")
}
