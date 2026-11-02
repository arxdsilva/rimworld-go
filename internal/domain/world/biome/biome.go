// Package biome is a map's natural setting (SIM-023): its climate, ground,
// wild plants, grass and how much wildlife it carries. The climates reach
// the wiki's summer and winter extremes (TEMP-2, ENG-003); the other
// numbers are our own; biome names are plain geography.
package biome

import (
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
)

// ID is a biome. Saved: append only.
type ID uint8

const (
	TemperateForest ID = iota // the original map; old saves load as this
	BorealForest
	Tundra
	ColdBog
	IceSheet
	SeaIce
	TemperateSwamp
	TropicalSwamp
	TropicalRainforest
	AridShrubland
	Desert
	ExtremeDesert
	NumBiomes
)

// Ground is the soft-ground mix in permille (rock and stone come from the
// map's own noise in every biome).
type Ground struct {
	Soil, Rich, Gravel, Sand, Marsh, Ice int
}

// PlantDensity is how many fertile tiles (permille) start with a wild plant
// of a species.
type PlantDensity struct {
	Species plant.Species
	Permil  int
}

// Def is a biome's definition.
type Def struct {
	ID      ID
	Key     string // "temperate_forest"
	Name    string // "Temperate forest"
	Blurb   string // one line for the landing picker
	Climate temperature.Climate
	// Wetness scales rain and snow, percent of the temperate odds.
	Wetness int
	// Legacy maps keep the original generator (temperate forest): ground,
	// plants and grass exactly as before biomes existed.
	Legacy bool
	Ground Ground
	Plants []PlantDensity
	Grass  int // permille of soft tiles with grass at the start
	Regrow int // days for a grazed or cut grass tile to grow back (0: never)
	// Wildlife is the wild animals the map carries per 10,000 tiles.
	Wildlife int
	// Warmth (tenths of a degree, original) is how warm the land counts
	// for the animals' habitat bands (animal.ClimateBand): the annual
	// means from before TEMP-2, since the wiki's summer and winter
	// extremes no longer tell a desert from a temperate forest.
	Warmth int
	// Difficulty labels the picker card ("", "hard", "extreme").
	Difficulty string

	// Forage is how much wild food foraging finds here, percent (wiki: Biomes).
	Forage int
	// DiseaseMTB is the mean days between colony diseases (wiki: Biomes).
	DiseaseMTB int
	// MoveBase is the world-map movement difficulty, and MoveWinter the
	// same at the depth of winter (wiki: Biomes, Caravan).
	MoveBase, MoveWinter float64
}

// Icon is the picker icon's sprite key.
func (d Def) Icon() string { return "ui_icon/biome_" + d.Key }

// c is a climate whose warmest summer afternoon is summer and coldest
// winter night winter (whole degrees, TEMP-2 wiki: Biomes "temperature
// variation"), with a day swing of swing tenths (original tuning: the
// wiki does not publish one). Its annual mean, halfway, falls inside the
// wiki's "average temperature" span for every biome.
func c(summer, winter, swing int) temperature.Climate {
	return temperature.Climate{YearMean: (summer + winter) * 5, Amplitude: (summer-winter)*5 - swing,
		PeakDay: temperature.Temperate.PeakDay, DaySwing: swing}
}

// Fit is site climate cl brought inside biome b's summer and winter
// extremes (TEMP-2, wiki: Biomes): a wider year loses amplitude (then day
// swing), then the mean moves until both ends fit. A climate already
// inside is unchanged; the zero (legacy) climate stays zero.
func Fit(b ID, cl temperature.Climate) temperature.Climate {
	if cl == (temperature.Climate{}) {
		return cl
	}
	lo, hi := DefOf(b).Climate.Range()
	half := int(hi-lo) / 2
	if over := cl.Amplitude + cl.DaySwing - half; over > 0 {
		cut := min(over, cl.Amplitude)
		cl.Amplitude -= cut
		cl.DaySwing -= over - cut
	}
	span := cl.Amplitude + cl.DaySwing
	cl.YearMean = min(max(cl.YearMean, int(lo)+span), int(hi)-span)
	return cl
}

// Forage, DiseaseMTB, MoveBase and MoveWinter per biome: wiki: Biomes.
var defs = [NumBiomes]Def{
	TemperateForest: {Key: "temperate_forest", Name: "Temperate forest", Warmth: 90, Blurb: "Mild seasons, mixed woods and meadows. A forgiving start.",
		Forage: 100, DiseaseMTB: 50, MoveBase: 1, MoveWinter: 3,
		Climate: temperature.Temperate, Wetness: 100, Legacy: true,
		Plants: []PlantDensity{{plant.Tree, 55}, {plant.BerryBush, 25}, {plant.Bramble, 12}, {plant.Dandelion, 15}}, Regrow: 6, Wildlife: 100},
	BorealForest: {Key: "boreal_forest", Name: "Boreal forest", Warmth: 0, Blurb: "Long snowy winters under dark pine. Short summers to farm in.",
		Forage: 75, DiseaseMTB: 60, MoveBase: 1, MoveWinter: 3,
		Climate: c(25, -35, 60), Wetness: 90, Ground: Ground{Soil: 700, Gravel: 250, Marsh: 50},
		Plants: []PlantDensity{{plant.Pine, 80}, {plant.BerryBush, 10}, {plant.Tree, 10}, {plant.Moss, 30}, {plant.Bramble, 8}}, Grass: 700, Regrow: 9, Wildlife: 80},
	Tundra: {Key: "tundra", Name: "Tundra", Warmth: -80, Blurb: "Open, frozen plains of moss and gravel. Barely a summer.",
		Forage: 50, DiseaseMTB: 80, MoveBase: 1, MoveWinter: 3,
		Climate: c(25, -50, 50), Wetness: 70, Ground: Ground{Soil: 450, Gravel: 550},
		Plants: []PlantDensity{{plant.Pine, 5}, {plant.Shrub, 10}, {plant.Moss, 25}}, Grass: 600, Regrow: 14, Wildlife: 40, Difficulty: "hard"},
	ColdBog: {Key: "cold_bog", Name: "Cold bog", Warmth: 0, Blurb: "Soggy, chilly wetland of pine and reeds. Slow going underfoot.",
		Forage: 50, DiseaseMTB: 45, MoveBase: 4, MoveWinter: 6,
		Climate: c(25, -35, 50), Wetness: 130, Ground: Ground{Soil: 650, Gravel: 50, Marsh: 300},
		Plants: []PlantDensity{{plant.Pine, 40}, {plant.Reeds, 30}, {plant.Moss, 35}}, Grass: 650, Regrow: 9, Wildlife: 60},
	IceSheet: {Key: "ice_sheet", Name: "Ice sheet", Warmth: -250, Blurb: "Endless ice. Nothing grows; food must be hunted or brought.",
		Forage: 0, DiseaseMTB: 90, MoveBase: 1.5, MoveWinter: 3.5,
		Climate: c(10, -70, 40), Wetness: 60, Ground: Ground{Gravel: 50, Ice: 950}, Wildlife: 5, Difficulty: "extreme"},
	SeaIce: {Key: "sea_ice", Name: "Sea ice", Warmth: -280, Blurb: "Frozen sea, no rock, no soil. The hardest place to live.",
		Forage: 0, DiseaseMTB: 90, MoveBase: 1.5, MoveWinter: 3.5,
		Climate: c(10, -60, 40), Wetness: 60, Ground: Ground{Ice: 1000}, Wildlife: 3, Difficulty: "extreme"},
	TemperateSwamp: {Key: "temperate_swamp", Name: "Temperate swamp", Warmth: 120, Blurb: "Mild, wet woodland with marshes and reeds.",
		Forage: 75, DiseaseMTB: 40, MoveBase: 4, MoveWinter: 6,
		Climate: c(35, -25, 50), Wetness: 140, Ground: Ground{Soil: 550, Rich: 100, Marsh: 350},
		Plants: []PlantDensity{{plant.Tree, 60}, {plant.Reeds, 40}, {plant.BerryBush, 15}, {plant.Bramble, 15}, {plant.Moss, 20}}, Grass: 800, Regrow: 5, Wildlife: 90},
	TropicalSwamp: {Key: "tropical_swamp", Name: "Tropical swamp", Warmth: 260, Blurb: "Hot, steaming marshland under jungle canopy.",
		Forage: 75, DiseaseMTB: 30, MoveBase: 4, MoveWinter: 6,
		Climate: c(35, 0, 40), Wetness: 150, Ground: Ground{Soil: 200, Rich: 450, Marsh: 350},
		Plants: []PlantDensity{{plant.JungleTree, 70}, {plant.Reeds, 40}, {plant.ElephantEar, 20}, {plant.Moss, 10}}, Grass: 800, Regrow: 4, Wildlife: 100},
	TropicalRainforest: {Key: "tropical_rainforest", Name: "Tropical rainforest", Warmth: 270, Blurb: "Hot and wet all year. Rich soil and dense jungle.",
		Forage: 100, DiseaseMTB: 35, MoveBase: 2, MoveWinter: 4,
		Climate: c(35, 0, 40), Wetness: 160, Ground: Ground{Soil: 400, Rich: 600},
		// PLAY-010: jungle trees 120 -> 95 and berries 40 -> 25 make room for
		// the 45 permille of ground plants, so the jungle stays ~165 permille.
		Plants: []PlantDensity{{plant.JungleTree, 95}, {plant.BerryBush, 25},
			{plant.ElephantEar, 25}, {plant.FireLily, 10}, {plant.Bramble, 10}}, Grass: 850, Regrow: 4, Wildlife: 120},
	AridShrubland: {Key: "arid_shrubland", Name: "Arid shrubland", Warmth: 220, Blurb: "Hot, dry scrub and sand. Little wood, wide horizons.",
		Forage: 50, DiseaseMTB: 65, MoveBase: 1, MoveWinter: 3,
		Climate: c(35, 0, 90), Wetness: 50, Ground: Ground{Soil: 400, Gravel: 300, Sand: 300},
		Plants: []PlantDensity{{plant.Shrub, 40}, {plant.Cactus, 10}, {plant.Agave, 15}, {plant.BallCactus, 10}}, Grass: 350, Regrow: 10, Wildlife: 70},
	Desert: {Key: "desert", Name: "Desert", Warmth: 300, Blurb: "Sand, cactus and scorching days. Farming is hard work.",
		Forage: 25, DiseaseMTB: 90, MoveBase: 1, MoveWinter: 3,
		Climate: c(35, -25, 110), Wetness: 20, Ground: Ground{Soil: 50, Gravel: 250, Sand: 700},
		// sparse grass that comes back quickly after rain keeps the few
		// grazers alive (a slower regrow starved them by day 5)
		Plants: []PlantDensity{{plant.Cactus, 15}, {plant.Shrub, 30}, {plant.Agave, 10}, {plant.BallCactus, 12}}, Grass: 220, Regrow: 7, Wildlife: 30, Difficulty: "hard"},
	ExtremeDesert: {Key: "extreme_desert", Name: "Extreme desert", Warmth: 380, Blurb: "Dunes and blistering heat. Almost nothing lives here.",
		Forage: 0, DiseaseMTB: 90, MoveBase: 1, MoveWinter: 3,
		Climate: c(35, -5, 120), Wetness: 5, Ground: Ground{Gravel: 150, Sand: 850},
		Plants: []PlantDensity{{plant.Cactus, 4}, {plant.Shrub, 8}, {plant.BallCactus, 5}}, Grass: 60, Regrow: 9, Wildlife: 10, Difficulty: "extreme"},
}

func init() {
	for i := range defs {
		defs[i].ID = ID(i)
	}
}

// DefOf returns biome b's definition (temperate forest for an unknown id).
func DefOf(b ID) Def {
	if b >= NumBiomes {
		return defs[TemperateForest]
	}
	return defs[b]
}

// DiseaseOdds is how often disease strikes in b, in percent of a temperate
// forest (the catalogue's weights are that forest's; WG-05).
func DiseaseOdds(b ID) int {
	return 100 * defs[TemperateForest].DiseaseMTB / DefOf(b).DiseaseMTB
}

// All lists every biome in picker order.
func All() []Def { return append([]Def(nil), defs[:]...) }

// Parse returns the biome with this key.
func Parse(key string) (ID, bool) {
	for _, d := range defs {
		if d.Key == key {
			return d.ID, true
		}
	}
	return 0, false
}

func (b ID) String() string { return DefOf(b).Key }

// Set is a bitset of biomes.
type Set uint16

// Of builds a set.
func Of(ids ...ID) Set {
	var s Set
	for _, id := range ids {
		s |= 1 << id
	}
	return s
}

// Has reports whether b is in the set.
func (s Set) Has(b ID) bool { return s&(1<<b) != 0 }
