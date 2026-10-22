package faction

import (
	"strings"

	"rimworld-go/pkg/randx"
)

// Original word lists for faction and settlement names (NG-1 §3.4). All
// invented or plain English; nothing here comes from any other game.

var adjectives = []string{
	"Ashen", "Amber", "Hollow", "Salt", "Rust", "Silent", "Gilded", "Pale", "Crooked", "Iron",
	"Hidden", "Weathered", "Burnt", "Quiet", "Bitter", "Sallow", "Frosted", "Wandering", "Lone", "Brazen",
	"Tarnished", "Verdant", "Ragged", "Distant", "Copper", "Slate", "Cobalt", "Umber", "Ivory", "Scarlet",
	"Ochre", "Dusky", "Hardy", "Patient", "Restless", "Humble", "Stubborn", "Gaunt", "Mossy", "Windward",
	"Northern", "Southern", "Eastern", "Western", "Thorned", "Hushed", "Brittle", "Cinder", "Dim", "Faded",
	"Fallow", "Granite", "Harrowed", "Jagged", "Keen", "Lantern", "Marsh", "Narrow", "Open", "Proud",
	"Sunken", "Tangled",
}

var nouns = []string{
	"Heron", "Wren", "Badger", "Otter", "Ember", "Thistle", "Marten", "Falcon", "Alder", "Birch",
	"Flint", "Raven", "Stag", "Vole", "Willow", "Hawthorn", "Lynx", "Moth", "Fennel", "Sparrow",
	"Hare", "Cairn", "Torch", "Lark", "Briar", "Rook", "Harrier", "Newt", "Bramble", "Ferret",
	"Gorse", "Plover", "Sedge", "Quill", "Salmon", "Anvil", "Spindle", "Kettle", "Mantle", "Tern",
	"Shrike", "Pike", "Cinder", "Adder", "Teasel", "Yarrow", "Rowan", "Heath", "Dunlin", "Mallow",
	"Cormorant", "Pine", "Nettle", "Ibis", "Wicker", "Sable", "Starling", "Tinder", "Hornet", "Burr",
}

// bodyParts make clan compounds ("Heron-Eye Clan").
var bodyParts = []string{"Eye", "Foot", "Wing", "Claw", "Tooth", "Hand", "Tail", "Crest", "Song", "Feather", "Heart", "Back"}

// Place names are built from two syllable tables: 30 openings and 22
// endings give 660 distinct names.
var placeStarts = []string{
	"Ash", "Bel", "Cor", "Dun", "El", "Fen", "Gar", "Hal", "Ire", "Jor",
	"Kel", "Lin", "Mar", "Nor", "Oak", "Pen", "Quor", "Rav", "Sel", "Thorn",
	"Ul", "Vel", "Wyn", "Yar", "Zen", "Brae", "Cal", "Dov", "Eld", "Fal",
}

var placeEnds = []string{
	"wick", "mere", "stead", "ford", "haven", "dale", "holm", "vale", "rest", "moor",
	"by", "ton", "gate", "reach", "fall", "wood", "bury", "marsh", "cliff", "water",
	"field", "hollow",
}

var femaleNames = []string{
	"Ansel", "Brenna", "Cora", "Dalia", "Edda", "Fenna", "Greta", "Hedra", "Isolde", "Jessa",
	"Kerra", "Liora", "Maren", "Nessa", "Odile", "Petra", "Quilla", "Rhona", "Sabra", "Tamsin",
	"Una", "Vesna", "Wilma", "Ysolde", "Zora", "Alma", "Brisa", "Cleo", "Dorrit", "Ilka",
}

var maleNames = []string{
	"Aldric", "Brand", "Corwin", "Dunstan", "Emmet", "Fergal", "Garrick", "Hollis", "Ivor", "Jasper",
	"Kellan", "Lorcan", "Merrick", "Nolan", "Osric", "Perrin", "Quentin", "Rafe", "Stellan", "Tobin",
	"Ulric", "Vance", "Warrick", "Yorick", "Zane", "Anselm", "Anders", "Cedric", "Dorian", "Ewan",
}

var surnames = []string{
	"Ashby", "Bellweather", "Crane", "Dunmore", "Ellery", "Farrow", "Gillespie", "Harrow", "Ingram", "Jessop",
	"Kettering", "Larkin", "Marlowe", "Nightingale", "Orchard", "Pemberton", "Quayle", "Rushworth", "Sutter", "Thackeray",
	"Underhill", "Varley", "Whitlock", "Yarrow-Hale", "Zeller", "Alder", "Brackett", "Coldwell", "Dray", "Eastwood",
}

func pick(rng *randx.Rand, list []string) string { return list[rng.IntN(len(list))] }

// PlaceCapacity is how many distinct place names the syllable tables make.
func PlaceCapacity() int { return len(placeStarts) * len(placeEnds) }

// placeName composes one place name.
func placeName(rng *randx.Rand) string {
	return pick(rng, placeStarts) + pick(rng, placeEnds)
}

// uniquePlace draws a place name not in used, marking it used. Past
// capacity it numbers the name ("Ashwick II").
func uniquePlace(rng *randx.Rand, used map[string]bool) string {
	for try := 0; try < 80; try++ {
		if n := placeName(rng); !used[n] {
			used[n] = true
			return n
		}
	}
	base := placeName(rng)
	for i := 2; ; i++ {
		if n := base + " " + roman(i); !used[n] {
			used[n] = true
			return n
		}
	}
}

func roman(n int) string {
	vals := []struct {
		v int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}}
	var b strings.Builder
	for _, e := range vals {
		for n >= e.v {
			b.WriteString(e.s)
			n -= e.v
		}
	}
	return b.String()
}

// factionName builds a name for type t from its grammar (§3.4).
func factionName(t Type, rng *randx.Rand) string {
	alt := rng.IntN(2) == 0
	switch t {
	case Settlers:
		if alt {
			return "the " + pick(rng, adjectives) + " Compact"
		}
		return placeName(rng) + " Commonwealth"
	case Clans:
		if alt {
			if rng.IntN(100) < 40 {
				return pick(rng, nouns) + "-" + pick(rng, bodyParts) + " Clan"
			}
			return pick(rng, nouns) + " Clan"
		}
		return "People of the " + placeName(rng)
	case Drifters:
		if alt {
			return "the " + pick(rng, adjectives) + " Drift"
		}
		return placeName(rng) + " Wanderers"
	case Freebooters:
		if alt {
			return "the " + pick(rng, nouns) + " Reavers"
		}
		return pick(rng, adjectives) + " Knives"
	case Swarm:
		return "The Swarm"
	}
	return ""
}

// leaderFor makes a leader for def d.
func leaderFor(d Def, rng *randx.Rand) Leader {
	if len(d.Leaders) == 0 {
		return Leader{}
	}
	title := pick(rng, d.Leaders)
	female := rng.IntN(2) == 0
	switch {
	case strings.HasSuffix(title, "mother"):
		female = true
	case strings.HasSuffix(title, "father"):
		female = false
	}
	given := pick(rng, maleNames)
	if female {
		given = pick(rng, femaleNames)
	}
	return Leader{Name: given + " " + pick(rng, surnames), Title: title}
}
