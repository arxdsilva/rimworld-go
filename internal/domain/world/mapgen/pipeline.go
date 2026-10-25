package mapgen

// pipeline is the step order (MAPGEN-2 §1.1). Each step only reads what
// the steps before it wrote.
var pipeline = []Step{
	elevationStep{},  // Elevation, MountainBias
	fertilityStep{},  // Fertility
	wetnessStep{},    // Wetness (base)
	materialStep{},   // Stone
	rockmaskStep{},   // RockTau/RoughTau/GravelTau, provisional Rock
	cavesStep{},      // Cave
	coastStep{},      // Coast, CoastDist, Water
	riverStep{},      // River, RiverDist, RiverLine, Water; cuts Rock
	lakesStep{},      // Lake, Water
	rocksStep{},      // final Rock, RockDepth, Roof; finishes Wetness
	terrainStep{},    // Terrain, Geology.Stone
	oresStep{},       // Geology.Ore
	chunksStep{},     // ChunkDensity, Chunks, Blocked
	roadsStep{},      // Road, Roads, Track terrain, Blocked
	startStep{},      // StartScore, Start (local repair)
	ruinsStep{},      // Ruins, Blocked
	ventsStep{},      // Vents, Blocked
	vegetationStep{}, // VegDensity, Plants, Grass
	validateStep{},   // repairs, Metrics
}

// labels are the loading screen's phase names per step.
var labels = map[string]string{
	"elevation": "Raising the land", "fertility": "Raising the land", "wetness": "Raising the land",
	"material": "Laying stone", "rockmask": "Laying stone", "caves": "Carving caves",
	"coast": "Rivers and shores", "river": "Rivers and shores", "lakes": "Rivers and shores",
	"rocks": "Laying stone", "terrain": "Laying stone", "ores": "Ores and debris", "chunks": "Ores and debris",
	"roads": "Roads and ruins", "start": "Choosing the landing spot", "ruins": "Roads and ruins",
	"vents": "Roads and ruins", "vegetation": "Planting", "validate": "Checking the map",
}

// Label is a step's player-facing phase name.
func Label(step string) string {
	if l, ok := labels[step]; ok {
		return l
	}
	return "Shaping the land"
}

// StepNames lists the pipeline's step names in order.
func StepNames() []string {
	out := make([]string, len(pipeline))
	for i, s := range pipeline {
		out[i] = s.Name()
	}
	return out
}
