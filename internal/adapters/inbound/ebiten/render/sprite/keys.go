package sprite

// TwoDigits is n (0..99) as two decimal digits, the sprite-key suffix:
// 7 is "07".
func TwoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// TileHash is the stable per-tile hash behind variant picks.
func TileHash(x, y int) uint64 {
	return uint64(uint32(x))*0x9E3779B97F4A7C15 ^ uint64(uint32(y))*0xBF58476D1CE4E5B9
}

// grassTints colours the grass tufts per biome: lush green, boreal dark
// green, moss olive, dry gold, jungle saturated. Temperate forest keeps
// the original art untinted.
var grassTints = map[string]ColorScale{
	"temperate_forest":    NoCS,
	"temperate_swamp":     {0.95, 1.1, 0.9, 1},   // lush
	"boreal_forest":       {0.68, 0.9, 0.78, 1},  // boreal dark green
	"cold_bog":            {0.86, 0.9, 0.62, 1},  // moss
	"tundra":              {0.88, 0.9, 0.66, 1},  // moss
	"ice_sheet":           {0.8, 0.95, 0.9, 1},   // pale
	"sea_ice":             {0.8, 0.95, 0.9, 1},   // pale
	"arid_shrubland":      {1.22, 1.05, 0.58, 1}, // dry gold
	"desert":              {1.28, 1.02, 0.5, 1},  // dry gold
	"extreme_desert":      {1.3, 1.0, 0.48, 1},   // dry gold
	"tropical_swamp":      {0.78, 1.18, 0.7, 1},  // jungle
	"tropical_rainforest": {0.72, 1.25, 0.62, 1}, // jungle
}

// GrassTint is the biome's grass colour scale (none for an unknown key).
func GrassTint(biomeKey string) ColorScale {
	if cs, ok := grassTints[biomeKey]; ok {
		return cs
	}
	return NoCS
}

// IsTreePlant reports species that are trees: they shake in the wind and
// get the tree lean.
func IsTreePlant(species string) bool {
	return species == "tree" || species == "pine" || species == "jungle_tree"
}

// BiomeItemSprite is the sprite of the new products, "" for other kinds.
func BiomeItemSprite(kind string) string {
	switch kind {
	case "fertile egg":
		return "item/fertile_egg"
	case "ember oil":
		return "item/ember_oil"
	case "wool parka":
		return "item/parka_wool"
	case "wool carpet":
		return "item/carpet_wool_rolled"
	}
	return ""
}
