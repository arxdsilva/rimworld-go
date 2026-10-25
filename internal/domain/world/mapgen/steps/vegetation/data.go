package vegetation

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
)

// group is a plant's layer in the hierarchy: each has its own spatial
// field and spacing rule.
type group uint8

const (
	gTree   group = iota // woody, plantable trees: groves
	gShrub               // bushes, shrubs, reeds, cacti: thickets
	gGround              // low ground plants: patches, no spacing
	numGroups
)

// groupOf classifies a species from its definition: a low ground plant is
// ground; a trunked plant a colonist may plant as a tree (tree, pine,
// jungle tree) is a tree; every other wild plant (berry bush, shrub, reeds,
// cactus) is a shrub.
func groupOf(s plant.Species) group {
	d := plant.DefOf(s)
	switch {
	case d.Ground:
		return gGround
	case d.Trunk && d.Sowable:
		return gTree
	}
	return gShrub
}

// speciesTune is how a species spreads: a plant within ClusterR cells
// (Chebyshev) of its own kind makes another one (1 + ClusterW) times as
// likely there; it prefers Wetness in [WetLo, WetHi].
type speciesTune struct {
	ClusterR     int
	ClusterW     float64
	WetLo, WetHi float64
}

// biomeVeg is a biome's vegetation tuning (our numbers).
type biomeVeg struct {
	// Density scales the VegDensity field (0..1]: the debug view's lushness
	// and the grass shaping; plant totals come from the biome's permille.
	Density float64
	// TreeCell, ShrubCell and GroundCell are each group's field cell size
	// at 250×250 (grows with the map's Scale).
	TreeCell, ShrubCell, GroundCell float64
	// Clearing is the share of the land left as open clearings.
	Clearing float64
	// WetPref leans the vegetation toward wet (+) or dry (−) ground, −1..1.
	WetPref float64
	// Grass replaces the biome's Grass permille when > 0.
	Grass int
	// Tune overrides a species' default spread in this biome.
	Tune map[plant.Species]speciesTune
}

// defaultTune is each wild species' spread.
var defaultTune = map[plant.Species]speciesTune{
	plant.Tree:        {ClusterR: 4, ClusterW: 3, WetLo: 0.15, WetHi: 0.85},
	plant.Pine:        {ClusterR: 4, ClusterW: 3, WetLo: 0.1, WetHi: 0.85},
	plant.JungleTree:  {ClusterR: 4, ClusterW: 3, WetLo: 0.2, WetHi: 1},
	plant.BerryBush:   {ClusterR: 3, ClusterW: 4, WetLo: 0.2, WetHi: 0.8},
	plant.Shrub:       {ClusterR: 3, ClusterW: 1.5, WetLo: 0, WetHi: 0.6},
	plant.Reeds:       {ClusterR: 3, ClusterW: 3, WetLo: 0.72, WetHi: 1},
	plant.Cactus:      {ClusterR: 3, ClusterW: 1, WetLo: 0, WetHi: 0.35},
	plant.Bramble:     {ClusterR: 3, ClusterW: 3, WetLo: 0.1, WetHi: 0.8},
	plant.Moss:        {ClusterR: 3, ClusterW: 2, WetLo: 0.4, WetHi: 1},
	plant.Agave:       {ClusterR: 3, ClusterW: 1.5, WetLo: 0, WetHi: 0.4},
	plant.BallCactus:  {ClusterR: 2, ClusterW: 2, WetLo: 0, WetHi: 0.4},
	plant.ElephantEar: {ClusterR: 3, ClusterW: 3, WetLo: 0.45, WetHi: 1},
	plant.FireLily:    {ClusterR: 3, ClusterW: 2.5, WetLo: 0.2, WetHi: 0.9},
	plant.Dandelion:   {ClusterR: 3, ClusterW: 2.5, WetLo: 0.15, WetHi: 0.75},
}

// fallbackTune is the spread of a species without a tune.
var fallbackTune = speciesTune{ClusterR: 3, ClusterW: 2, WetLo: 0, WetHi: 1}

// vegData is every biome's tuning.
var vegData = [biome.NumBiomes]biomeVeg{
	// the legacy grass rule seeded a fifth of soil; the field map uses a
	// meadow share instead
	biome.TemperateForest: {Density: 0.85, TreeCell: 35, ShrubCell: 10, GroundCell: 16, Clearing: 0.22, Grass: 750},
	biome.BorealForest:    {Density: 0.8, TreeCell: 38, ShrubCell: 10, GroundCell: 18, Clearing: 0.18},
	// lone, stunted pines on the open plain
	biome.Tundra: {Density: 0.3, TreeCell: 30, ShrubCell: 8, GroundCell: 20, Clearing: 0.35,
		Tune: map[plant.Species]speciesTune{plant.Pine: {ClusterR: 3, ClusterW: 0.5, WetLo: 0.1, WetHi: 0.85}}},
	biome.ColdBog:            {Density: 0.6, TreeCell: 30, ShrubCell: 9, GroundCell: 16, Clearing: 0.2, WetPref: 0.6},
	biome.IceSheet:           {Density: 0.05, TreeCell: 30, ShrubCell: 10, GroundCell: 20, Clearing: 0.5},
	biome.SeaIce:             {Density: 0.02, TreeCell: 30, ShrubCell: 10, GroundCell: 20, Clearing: 0.5},
	biome.TemperateSwamp:     {Density: 0.8, TreeCell: 30, ShrubCell: 9, GroundCell: 14, Clearing: 0.18, WetPref: 0.6},
	biome.TropicalSwamp:      {Density: 0.85, TreeCell: 30, ShrubCell: 9, GroundCell: 14, Clearing: 0.15, WetPref: 0.6},
	biome.TropicalRainforest: {Density: 1, TreeCell: 40, ShrubCell: 10, GroundCell: 16, Clearing: 0.12, WetPref: 0.2},
	// dry lands: plants gather where water collects
	biome.AridShrubland: {Density: 0.35, TreeCell: 30, ShrubCell: 12, GroundCell: 20, Clearing: 0.3, WetPref: 0.3},
	biome.Desert:        {Density: 0.2, TreeCell: 30, ShrubCell: 14, GroundCell: 20, Clearing: 0.35, WetPref: 0.4},
	biome.ExtremeDesert: {Density: 0.1, TreeCell: 30, ShrubCell: 16, GroundCell: 20, Clearing: 0.4, WetPref: 0.5},
}

// fallbackData is the tuning of a biome outside the table (a test's
// custom definition).
var fallbackData = biomeVeg{Density: 0.5, TreeCell: 35, ShrubCell: 10, GroundCell: 16, Clearing: 0.2}

// dataOf is biome b's tuning.
func dataOf(b biome.ID) biomeVeg {
	if b >= biome.NumBiomes {
		return fallbackData
	}
	return vegData[b]
}

// tuneOf is species s's spread in biome b.
func tuneOf(b biome.ID, s plant.Species) speciesTune {
	if t, ok := dataOf(b).Tune[s]; ok {
		return t
	}
	if t, ok := defaultTune[s]; ok {
		return t
	}
	return fallbackTune
}

// grassPermil is the share of grass ground (permille) that starts with
// grass in biome d.
func grassPermil(d biome.Def) int {
	if g := dataOf(d.ID).Grass; g > 0 {
		return g
	}
	return d.Grass
}
