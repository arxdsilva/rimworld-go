package animal

// Breeding and growth (FOOD-6, ENG-003 extended to food: published
// RimWorld wiki numbers, original names). Each species takes the infobox
// of the vanilla animal in its role (husbandry.go names the pairs):
// gestation period, offspring per birth, juvenile age, maturity age and
// life expectancy. A year is 60 days; ages are rounded to whole days.
// Egg layers carry no gestation: their young come from eggs
// (husbandry.go eggRows). The wiki gives only the litter range and its
// average, so a litter is rolled evenly over the range (original tuning).
// The megaspider's page (the hive brute's role) was not fetched: it takes
// the megascarab's ages (original tuning). Mechanoids keep the size
// defaults (they never breed or grow).

// breedRow is a species' published life numbers: gestation in hundredths
// of a day (0: lays eggs), litter range, juvenile and adult ages in days,
// life expectancy in years.
type breedRow struct {
	gest, litMin, litMax, juvenile, adult, years int
}

// wiki: each species page (Base stats: Gestation period, Offspring per
// birth, Juvenile age, Maturity age, Life expectancy).
var breedRows = [numSpecies]breedRow{
	Deer: {566, 1, 1, 6, 20, 15}, TimberWolf: {1000, 1, 3, 12, 30, 12},
	Alpaca: {666, 1, 1, 12, 20, 15}, Bison: {666, 1, 1, 15, 20, 15},
	Emberbuck: {666, 1, 1, 12, 20, 15}, Capybara: {566, 1, 2, 12, 20, 12},
	Caribou: {666, 1, 1, 12, 18, 18}, Cow: {666, 1, 1, 12, 20, 22},
	Donkey: {666, 1, 1, 15, 20, 24}, Dromedary: {666, 1, 1, 15, 20, 45},
	Elephant: {1332, 1, 1, 18, 40, 50}, Elk: {666, 1, 1, 12, 20, 18},
	Gazelle: {566, 1, 1, 6, 13, 12}, Goat: {561, 1, 1, 6, 16, 12},
	Horse: {666, 1, 1, 15, 20, 30}, Ibex: {566, 1, 1, 6, 13, 15},
	GroundSloth: {1332, 1, 1, 20, 40, 20}, Woolhorn: {666, 1, 1, 15, 20, 15},
	Pig: {566, 1, 2, 6, 20, 12}, Rhinoceros: {1332, 1, 1, 18, 33, 45},
	Sheep: {566, 1, 2, 15, 20, 12}, Yak: {666, 1, 1, 12, 20, 22},
	Timbergnaw: {850, 1, 3, 6, 20, 12}, ArcticFox: {1000, 1, 3, 6, 20, 9},
	ArcticWolf: {1000, 1, 3, 12, 30, 12}, CinderRat: {566, 1, 3, 6, 13, 8},
	Cat: {1000, 1, 3, 9, 24, 10}, Chinchilla: {566, 1, 3, 6, 13, 9},
	Cougar: {1000, 1, 1, 15, 30, 13}, FennecFox: {1000, 1, 3, 6, 20, 9},
	GrizzlyBear: {1000, 1, 1, 12, 30, 22}, GuineaPig: {666, 1, 3, 9, 12, 4},
	Hare: {566, 1, 2, 6, 13, 8}, Husky: {1000, 1, 3, 15, 30, 12},
	Labrador: {1000, 1, 3, 15, 30, 12}, Lynx: {1000, 1, 1, 12, 30, 9},
	Monkey: {666, 1, 1, 12, 16, 30}, Panther: {1000, 1, 1, 15, 30, 13},
	PolarBear: {1000, 1, 1, 12, 30, 22}, Raccoon: {566, 1, 2, 6, 13, 8},
	Rat: {566, 1, 3, 6, 13, 8}, RedFox: {1000, 1, 3, 6, 20, 9},
	Snowhare: {566, 1, 2, 6, 13, 8}, Squirrel: {566, 1, 2, 7, 13, 8},
	DireWolf: {1000, 1, 3, 12, 30, 15}, WildBoar: {566, 1, 2, 6, 20, 12},
	YorkshireTerrier: {1000, 1, 3, 6, 18, 15},
	// egg layers (wiki: each species page; the clutch is in eggRows)
	Cassowary: {0, 1, 1, 6, 13, 45}, Chicken: {0, 1, 1, 7, 12, 6},
	Duck: {0, 1, 1, 7, 12, 6}, Emu: {0, 1, 1, 6, 13, 45},
	Goose: {0, 1, 1, 7, 12, 8}, Ostrich: {0, 1, 1, 6, 13, 45},
	Turkey: {0, 1, 1, 7, 12, 9}, Cobra: {0, 1, 1, 9, 30, 20},
	Iguana: {0, 1, 1, 6, 13, 12}, Tortoise: {0, 1, 1, 9, 13, 180},
	// hive insects never breed (wiki: Megascarab, Spelopede)
	HiveBeetle: {0, 1, 1, 2, 24, 10}, HiveCrawler: {0, 1, 1, 2, 12, 6},
	HiveBrute:   {0, 1, 1, 2, 24, 10}, // original tuning: the megascarab's
	Glimmerstag: {2000, 1, 1, 45, 80, 220},
}

// daysPerYear is a RimWorld year (wiki: Time).
const daysPerYear = 60

// breedLife puts s's published numbers over the size defaults l.
func breedLife(s Species, l Life) Life {
	r := breedRows[s]
	if r.adult == 0 {
		return l // a mechanoid
	}
	l.GestCenti, l.LitterMin, l.LitterMax = r.gest, r.litMin, r.litMax
	l.JuvenileDays, l.AdultDays, l.LifeDays = r.juvenile, r.adult, r.years*daysPerYear
	return l
}

// Juvenile reports an animal of s, days old, past the baby stage (adults
// included).
func Juvenile(s Species, days int) bool { return days >= LifeOf(s).JuvenileDays }
