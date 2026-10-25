// Package pipeline runs the MAPGEN-2 steps in order (.cursor/PLAY013_PLAN.md
// §1.1): the composition of package mapgen's passes.
package pipeline

import (
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/steps/features"
	"rimworld-go/internal/domain/world/mapgen/steps/ground"
	"rimworld-go/internal/domain/world/mapgen/steps/landform"
	"rimworld-go/internal/domain/world/mapgen/steps/resources"
	"rimworld-go/internal/domain/world/mapgen/steps/roads"
	"rimworld-go/internal/domain/world/mapgen/steps/start"
	"rimworld-go/internal/domain/world/mapgen/steps/vegetation"
	"rimworld-go/internal/domain/world/mapgen/steps/water"
)

// steps is the order. Each step only reads what the steps before it wrote.
var steps = []mapgen.Step{
	landform.Elevation{},    // Elevation, MountainBias
	ground.Fertility{},      // Fertility
	ground.Material{},       // Stone
	landform.RockMask{},     // RockTau/RoughTau/GravelTau, provisional Rock
	landform.Caves{},        // Cave
	water.Coast{},           // Coast, CoastDist, Water
	water.River{},           // River, RiverDist, RiverLine, Water; cuts Rock
	water.Lakes{},           // Lake, Water
	water.Wetness{},         // Wetness
	landform.Rocks{},        // final Rock, RockDepth, Roof
	ground.Terrain{},        // Terrain, Geology.Stone
	resources.Ores{},        // Geology.Ore
	resources.Chunks{},      // ChunkDensity, Chunks, Blocked
	roads.Roads{},           // Road, Roads, Track terrain, Blocked
	start.Start{},           // StartScore, Start (local repair)
	features.Ruins{},        // Ruins, Blocked
	features.Vents{},        // Vents, Blocked
	vegetation.Vegetation{}, // VegDensity, Plants, Grass
	start.Validate{},        // repairs, Metrics
}

// labels are the loading screen's phase names per step.
var labels = map[string]string{
	"elevation": "Raising the land", "fertility": "Raising the land", "wetness": "Rivers and shores",
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

// StepNames lists the step names in order.
func StepNames() []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Name()
	}
	return out
}

// Steps is the number of steps.
func Steps() int { return len(steps) }

// Generate builds the whole map.
func Generate(cfg mapgen.Config) (mapgen.Result, error) {
	b, err := NewBuilder(cfg)
	if err != nil {
		return mapgen.Result{}, err
	}
	for {
		if _, _, done, err := b.Step(); err != nil {
			return mapgen.Result{}, err
		} else if done {
			return b.Result(), nil
		}
	}
}

// Builder runs the steps one at a time (the loading screen).
type Builder struct {
	ctx  *mapgen.Ctx
	next int
	res  mapgen.Result
	done bool
}

// NewBuilder prepares cfg's map.
func NewBuilder(cfg mapgen.Config) (*Builder, error) {
	if cfg.W <= 0 || cfg.H <= 0 {
		return nil, mapgen.ErrSize
	}
	return &Builder{ctx: mapgen.NewCtx(cfg)}, nil
}

// Step runs the next step. label is the phase of the step that runs next
// ("" once done) and frac the share done.
func (b *Builder) Step() (label string, frac float32, done bool, err error) {
	if b.done {
		return "", 1, true, nil
	}
	if b.next < len(steps) {
		if err := steps[b.next].Run(b.ctx); err != nil {
			return "", 0, false, err
		}
		b.next++
	}
	if b.next >= len(steps) {
		b.done, b.res = true, b.ctx.Finish()
		return "", 1, true, nil
	}
	return Label(steps[b.next].Name()), float32(b.next) / float32(len(steps)), false, nil
}

// Result is the finished map (zero until Step reports done).
func (b *Builder) Result() mapgen.Result { return b.res }
