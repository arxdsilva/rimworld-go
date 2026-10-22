// Package biome is a map's natural setting (SIM-023): its climate, ground,
// wild plants, grass and how much wildlife it carries. The numbers are our
// own; biome names are plain geography.
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
	// Difficulty labels the picker card ("", "hard", "extreme").
	Difficulty string
}

// Icon is the picker icon's sprite key.
func (d Def) Icon() string { return "ui_icon/biome_" + d.Key }

func c(mean, amp, swing int) temperature.Climate {
	return temperature.Climate{YearMean: mean, Amplitude: amp, PeakDay: temperature.Temperate.PeakDay, DaySwing: swing}
}

var defs = [NumBiomes]Def{
	TemperateForest: {Key: "temperate_forest", Name: "Temperate forest", Blurb: "Mild seasons, mixed woods and meadows. A forgiving start.",
		Climate: temperature.Temperate, Wetness: 100, Legacy: true,
		Plants: []PlantDensity{{plant.Tree, 55}, {plant.BerryBush, 25}}, Regrow: 6, Wildlife: 100},
	BorealForest: {Key: "boreal_forest", Name: "Boreal forest", Blurb: "Long snowy winters under dark pine. Short summers to farm in.",
		Climate: c(0, 180, 60), Wetness: 90, Ground: Ground{Soil: 700, Gravel: 250, Marsh: 50},
		Plants: []PlantDensity{{plant.Pine, 80}, {plant.BerryBush, 10}, {plant.Tree, 10}}, Grass: 700, Regrow: 9, Wildlife: 80},
	Tundra: {Key: "tundra", Name: "Tundra", Blurb: "Open, frozen plains of moss and gravel. Barely a summer.",
		Climate: c(-80, 180, 50), Wetness: 70, Ground: Ground{Soil: 450, Gravel: 550},
		Plants: []PlantDensity{{plant.Pine, 5}, {plant.Shrub, 10}}, Grass: 600, Regrow: 14, Wildlife: 40, Difficulty: "hard"},
	ColdBog: {Key: "cold_bog", Name: "Cold bog", Blurb: "Soggy, chilly wetland of pine and reeds. Slow going underfoot.",
		Climate: c(0, 160, 50), Wetness: 130, Ground: Ground{Soil: 650, Gravel: 50, Marsh: 300},
		Plants: []PlantDensity{{plant.Pine, 40}, {plant.Reeds, 30}}, Grass: 650, Regrow: 9, Wildlife: 60},
	IceSheet: {Key: "ice_sheet", Name: "Ice sheet", Blurb: "Endless ice. Nothing grows; food must be hunted or brought.",
		Climate: c(-250, 120, 40), Wetness: 60, Ground: Ground{Gravel: 50, Ice: 950}, Wildlife: 5, Difficulty: "extreme"},
	SeaIce: {Key: "sea_ice", Name: "Sea ice", Blurb: "Frozen sea, no rock, no soil. The hardest place to live.",
		Climate: c(-280, 120, 40), Wetness: 60, Ground: Ground{Ice: 1000}, Wildlife: 3, Difficulty: "extreme"},
	TemperateSwamp: {Key: "temperate_swamp", Name: "Temperate swamp", Blurb: "Mild, wet woodland with marshes and reeds.",
		Climate: c(120, 120, 50), Wetness: 140, Ground: Ground{Soil: 550, Rich: 100, Marsh: 350},
		Plants: []PlantDensity{{plant.Tree, 60}, {plant.Reeds, 40}, {plant.BerryBush, 15}}, Grass: 800, Regrow: 5, Wildlife: 90},
	TropicalSwamp: {Key: "tropical_swamp", Name: "Tropical swamp", Blurb: "Hot, steaming marshland under jungle canopy.",
		Climate: c(260, 40, 40), Wetness: 150, Ground: Ground{Soil: 200, Rich: 450, Marsh: 350},
		Plants: []PlantDensity{{plant.JungleTree, 70}, {plant.Reeds, 40}}, Grass: 800, Regrow: 4, Wildlife: 100},
	TropicalRainforest: {Key: "tropical_rainforest", Name: "Tropical rainforest", Blurb: "Hot and wet all year. Rich soil and dense jungle.",
		Climate: c(270, 30, 40), Wetness: 160, Ground: Ground{Soil: 400, Rich: 600},
		Plants: []PlantDensity{{plant.JungleTree, 120}, {plant.BerryBush, 40}}, Grass: 850, Regrow: 4, Wildlife: 120},
	AridShrubland: {Key: "arid_shrubland", Name: "Arid shrubland", Blurb: "Hot, dry scrub and sand. Little wood, wide horizons.",
		Climate: c(220, 100, 90), Wetness: 50, Ground: Ground{Soil: 400, Gravel: 300, Sand: 300},
		Plants: []PlantDensity{{plant.Shrub, 40}, {plant.Cactus, 10}}, Grass: 350, Regrow: 10, Wildlife: 70},
	Desert: {Key: "desert", Name: "Desert", Blurb: "Sand, cactus and scorching days. Farming is hard work.",
		Climate: c(300, 90, 110), Wetness: 20, Ground: Ground{Soil: 50, Gravel: 250, Sand: 700},
		// sparse grass that comes back quickly after rain keeps the few
		// grazers alive (a slower regrow starved them by day 5)
		Plants: []PlantDensity{{plant.Cactus, 15}, {plant.Shrub, 30}}, Grass: 220, Regrow: 7, Wildlife: 30, Difficulty: "hard"},
	ExtremeDesert: {Key: "extreme_desert", Name: "Extreme desert", Blurb: "Dunes and blistering heat. Almost nothing lives here.",
		Climate: c(380, 80, 120), Wetness: 5, Ground: Ground{Gravel: 150, Sand: 850},
		Plants: []PlantDensity{{plant.Cactus, 4}, {plant.Shrub, 8}}, Grass: 60, Regrow: 9, Wildlife: 10, Difficulty: "extreme"},
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
