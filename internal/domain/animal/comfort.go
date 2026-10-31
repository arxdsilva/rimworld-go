package animal

// Comfortable temperature ranges (TEMP-3), whole degrees C, one row per
// species (the roster's temperature column, kept in its own table so the
// art mirror's roster rows stay as they are). wiki: each vanilla species'
// page (comfortable min / max). The invented species take the numbers of
// the vanilla animal in their role (original mapping): ground sloth as
// the megasloth, woolhorn as the muffalo, timbergnaw as the alphabeaver,
// cinder rat as the boomrat, emberbuck as the boomalope, dire wolf as the
// warg, glimmerstag as the thrumbo, and the hive beetle, brute and
// crawler as the megascarab, megaspider and spelopede. Machines feel no
// air (no row).
var comfort = [numSpecies][2]int8{
	Deer: {-30, 40}, TimberWolf: {-40, 40}, Alpaca: {-20, 45}, Bison: {-55, 45}, Emberbuck: {-15, 40},
	Capybara: {-10, 50}, Caribou: {-50, 40}, Cow: {-10, 40}, Donkey: {-25, 45}, Dromedary: {-22, 60},
	Elephant: {-12, 50}, Elk: {-50, 40}, Gazelle: {-10, 40}, Goat: {-20, 40}, Horse: {-35, 45},
	Ibex: {-30, 40}, GroundSloth: {-55, 40}, Woolhorn: {-55, 45}, Pig: {-5, 40}, Rhinoceros: {-8, 40},
	Sheep: {-20, 45}, Yak: {-45, 40}, Timbergnaw: {-40, 40}, ArcticFox: {-50, 40}, ArcticWolf: {-55, 40},
	CinderRat: {-15, 40}, Cat: {-25, 40}, Chinchilla: {-15, 55}, Cougar: {-25, 40}, FennecFox: {-30, 40},
	GrizzlyBear: {-40, 40}, GuineaPig: {-15, 55}, Hare: {-30, 40}, Husky: {-50, 40}, Labrador: {-30, 40},
	Lynx: {-50, 40}, Monkey: {-5, 50}, Panther: {-8, 50}, PolarBear: {-55, 40}, Raccoon: {-30, 40},
	Rat: {-30, 40}, RedFox: {-35, 40}, Snowhare: {-55, 40}, Squirrel: {-35, 40}, DireWolf: {-40, 40},
	WildBoar: {-23, 40}, YorkshireTerrier: {-15, 40}, Cassowary: {-8, 50}, Chicken: {-10, 40}, Duck: {-10, 40},
	Emu: {-8, 50}, Goose: {-10, 40}, Ostrich: {-8, 50}, Turkey: {-8, 40}, Cobra: {0, 60},
	Iguana: {0, 60}, Tortoise: {0, 50}, HiveBeetle: {0, 60}, HiveBrute: {-40, 60}, HiveCrawler: {-25, 60},
	Glimmerstag: {-65, 50},
}

// Comfort is species s's comfortable range in tenths of a degree C;
// false for a machine (or an unknown species), which takes no exposure.
func Comfort(s Species) (lo, hi int, ok bool) {
	if s >= numSpecies || IsMech(s) {
		return 0, 0, false
	}
	c := comfort[s]
	return int(c[0]) * 10, int(c[1]) * 10, true
}

// Slows reports a species that suffers hypothermic slowdown instead of
// hypothermia: the hive insects (wiki: Hypothermia), which never die of
// it and never frostbite.
func Slows(s Species) bool { return s < numSpecies && roster[s].Behaviour == Hive }
