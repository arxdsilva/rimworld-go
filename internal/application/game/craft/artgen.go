package craft

import (
	"strings"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// The art grammar (PRD-3): a sculpture's title and description grow
// from its seed alone, so a save keeps only the seed. Every word list is
// original to this game.

var (
	artAdjectives = [...]string{"Quiet", "Patient", "Last", "Hollow", "Bright", "Wandering", "Silent", "Small",
		"Gentle", "Restless", "Hidden", "Second", "Crooked", "Faithful", "Distant", "Borrowed"}
	artNouns = [...]string{"Harvest", "Lantern", "Crossing", "Promise", "Winter", "River", "Ember", "Shelter",
		"Return", "Vigil", "Thaw", "Echo", "Hearth", "Orchard", "Bargain", "Threshold"}
	artTimes = [...]string{"Dusk", "First Light", "the Thaw", "Midwinter", "Low Tide", "the Last Bell",
		"Noon", "the Long Night"}
	artFor = [...]string{"the Lost", "Absent Friends", "the Road", "No One", "the Children", "the Harvest",
		"the Wanderers", "Those Who Stayed"}
	artSubjects = [...]string{
		"a heron standing in reeds", "two hands passing a lantern", "an old miner resting on a pick",
		"a fox curled around a seed", "a child counting stars", "a broken wheel sprouting vines",
		"a woman reading to a sleeping dog", "a tower of stacked stones", "a boar chasing a moth",
		"three crows sharing bread", "a ship held fast in ice", "a farmer lifting the first harvest",
		"a wolf watching an empty road", "a drummer at a wedding", "a lamp burning in a storm",
		"a sleeping giant made of hills",
	}
	artDetails = [...]string{
		"The lines are calm and unhurried.", "Rough chisel marks are left on purpose.",
		"It seems to lean toward whoever looks at it.", "The surface is worked smooth as river glass.",
		"Small details reward a close look.", "The proportions are strange but honest.",
		"One side is finished; the other is left raw.", "Light catches it differently every hour.",
	}
	// artCodas is a line by quality, Awful..Legendary.
	artCodas = [...]string{
		"Nothing about it quite agrees with anything else.", "It looks hurried.", "",
		"The work is careful and sure.", "People slow down when they pass it.",
		"It is hard to look away from.", "Visitors will speak of it for years.",
	}
)

// artBits draws successive picks from seed (a xorshift, so nearby seeds
// still read differently).
type artBits uint32

func (b *artBits) pick(n int) int {
	x := uint32(*b)
	if x == 0 {
		x = 0x9e3779b9
	}
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	*b = artBits(x)
	return int(x % uint32(n))
}

// ArtTitle is the title a sculpture of seed seed carries.
func ArtTitle(seed uint32) string {
	b := artBits(seed)
	adj, noun := artAdjectives[b.pick(len(artAdjectives))], artNouns[b.pick(len(artNouns))]
	switch b.pick(4) {
	case 0:
		return noun + " at " + artTimes[b.pick(len(artTimes))]
	case 1:
		return "A " + noun + " for " + artFor[b.pick(len(artFor))]
	case 2:
		return adj + " " + noun + ", Study " + string(rune('2'+b.pick(8)))
	}
	return "The " + adj + " " + noun
}

// ArtDescription is the description of a sculpture of seed seed, kind
// k, material stuff and quality l, carved by maker ("" when unknown).
func ArtDescription(seed uint32, k construction.Kind, stuff item.Kind, l quality.Level, maker string) string {
	b := artBits(seed ^ 0x5bd1e995)
	var sb strings.Builder
	sb.WriteString(article(l.String()))
	sb.WriteString(" ")
	sb.WriteString(l.String())
	if s := artStuff(stuff); s != "" {
		sb.WriteString(" " + s)
	}
	sb.WriteString(" " + construction.DefOf(k).Name + " of ")
	sb.WriteString(artSubjects[b.pick(len(artSubjects))])
	sb.WriteString(". ")
	sb.WriteString(artDetails[b.pick(len(artDetails))])
	if l.Valid() && artCodas[l] != "" {
		sb.WriteString(" " + artCodas[l])
	}
	if maker != "" {
		sb.WriteString(" Carved by " + maker + ".")
	}
	return sb.String()
}

// artStuff names a sculpture's material as an adjective ("granite",
// "steel", "wooden").
func artStuff(k item.Kind) string {
	if k == item.Wood {
		return "wooden"
	}
	if k.StuffCategory() == 0 {
		return ""
	}
	return strings.TrimSuffix(k.String(), " blocks")
}

// article is "An" before a vowel, else "A".
func article(word string) string {
	if word != "" && strings.ContainsRune("aeiouAEIOU", rune(word[0])) {
		return "An"
	}
	return "A"
}
