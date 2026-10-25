// Package mapgen is the field-based colony-map generator for landing sites
// (MAPGEN-2, .cursor/PLAY013_PLAN.md). It builds a map in ordered passes
// (steps): continuous fields first (elevation, fertility, wetness, stone
// material), then landforms (rock, caves, water), then the discrete map
// (terrain by precedence, ore, debris, roads, the landing spot, ruins,
// vents, plants), then validation with local repair. Every step draws from
// its own named random stream, so changing one pass never shifts another.
//
// It is pure, deterministic and headless: no wall clock, no global RNG, no
// Go map iteration. The legacy generator (package generation) stays frozen
// for old maps and tests. All numbers are our own.
package mapgen

import (
	"errors"
	"math"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

// Version is the generator version saved with a colony (Setup.GenVersion):
// regenerating a map's debug fields needs the version that made it.
const Version = 2

// versionSalt keeps this version's streams apart from any other's.
const versionSalt uint64 = 0x6d617067656e0002 // "mapgen" v2

// Far is the distance stored where a feature does not exist (no river, no
// coast).
const Far = float32(math.MaxFloat32)

// Point is a cell.
type Point struct{ X, Y int }

// PointF is a point in cell units (cell centres at +0.5).
type PointF struct{ X, Y float64 }

// Config is one map to generate.
type Config struct {
	// Seed is the colony's setup seed.
	Seed uint64
	// W, H is the map size in cells.
	W, H int
	// Site is the world tile (biome, climate, hills, river, coast, lakes,
	// caves, stones, roads and the world context).
	Site site.Site
	// Biome is the site biome's definition (biome.DefOf(Site.Biome));
	// tests may pass a custom one.
	Biome biome.Def
	// Debug keeps the intermediate Fields in the Result.
	Debug bool
}

// Water is a cell's water class, written by the water steps.
type Water uint8

const (
	Dry        Water = iota
	Deep             // deep water: impassable
	Shallow          // shallow water: wadeable (fords, shelves)
	Beach            // the coast's beach band (sand, or ice when cold)
	Bank             // a river's bank band (gravel, sand or mud by noise)
	Floodplain       // fertile land beside a river (rich soil)
	Shore            // a lake's ring (marsh or mud)
)

// Wet reports actual water (deep or shallow).
func (w Water) Wet() bool { return w == Deep || w == Shallow }

// Fields is the continuous model and the shared masks the steps build on.
// Every slice is row-major W×H and allocated before the first step.
type Fields struct {
	// Elevation is in [0, 1] (rank-normalised: a quantile is a share of
	// the map). MountainBias is the directional mountain mass in [0, 1].
	Elevation, MountainBias []float32
	// Fertility and Wetness are in [0, 1].
	Fertility, Wetness []float32
	// VegDensity is the vegetation target in [0, 1]; ChunkDensity the
	// debris density (relative, 1 = typical).
	VegDensity, ChunkDensity []float32
	// RiverDist and CoastDist are cells to the river centreline and to the
	// shore (Far when there is none).
	RiverDist, CoastDist []float32
	// StartScore is the landing-spot suitability in [0, 1] (debug view).
	StartScore []float32
	// RockTau, RoughTau and GravelTau are the elevation thresholds of rock
	// walls, rough stone ground and gravel (RockTau > RoughTau > GravelTau).
	RockTau, RoughTau, GravelTau float32
	// Stone is the material-field winner of every cell.
	Stone []geology.Stone
	// Rock is the rock-wall mask: provisional after "rockmask" (caves and
	// water cut it), final after "rocks".
	Rock []bool
	// Cave marks cave tunnel floor (open, under rock).
	Cave []bool
	// RockDepth is the Chebyshev distance from a rock cell to the nearest
	// open cell, capped at 255 (0 = open), final after "rocks".
	RockDepth []uint8
	// Water is each cell's water class; River, Coast and Lake mark the
	// cells of each body (water and its bands).
	Water              []Water
	River, Coast, Lake []bool
	// RiverLine is the river's centreline (empty without a river).
	RiverLine []PointF
	// Road marks road cells; Blocked marks cells a feature took (road,
	// ruin, vent, debris chunk), so later steps leave them alone.
	Road, Blocked []bool
}

// PlantSpawn is one wild plant placed at generation. Growth is in growth
// ticks (plant.Def.GrowTicks is fully grown).
type PlantSpawn struct {
	X, Y    int
	Species plant.Species
	Growth  int
}

// ChunkSpawn is one stone chunk lying on the ground.
type ChunkSpawn struct {
	X, Y  int
	Stone geology.Stone
}

// RuinKind is a ruin's design.
type RuinKind uint8

const (
	WallRun RuinKind = iota // a broken length of wall
	Shed                    // a roofless, broken rectangle with an old floor
)

// RuinWall is one standing wall piece.
type RuinWall struct {
	X, Y      int
	Stuff     item.Kind // item.Wood or a stone's blocks
	DamagePct int       // share of its hit points already lost
}

// RuinFloor is one old floor tile.
type RuinFloor struct {
	X, Y  int
	Stuff item.Kind
}

// Loot is an item stack left in a ruin.
type Loot struct {
	X, Y int
	Kind item.Kind
	Qty  int
}

// Ruin is one small surface ruin.
type Ruin struct {
	Kind RuinKind
	// Bounds (inclusive) of the ruin's footprint.
	X0, Y0, X1, Y1 int
	Walls          []RuinWall
	Floors         []RuinFloor
	Loot           []Loot
	// Rubble are stone chunks scattered in and around the ruin.
	Rubble []ChunkSpawn
}

// Metrics are the map's quality measurements (debug tool, profile tests);
// they never fail a map on their own.
type Metrics struct {
	RockShare        float64 // rock-wall cells / all cells
	LargestRockShare float64 // largest rock component / all rock
	TinyRockBodies   int     // rock components of 5 cells or fewer
	MeanPatchSize    float64 // mean 4-connected terrain patch size
	OpenShare        float64 // walkable dry cells / all cells
	VegQuadrants     [4]float64
	TreeClusters     int     // tree groups (trees within 2 cells joined) of 6+
	RiverSinuosity   float64 // centreline length / end-to-end distance
	StoneShare       [8]float64
	StartReach       float64 // walkable dry land reachable from Start
	Repairs          []string
}

// Result is a generated map.
type Result struct {
	W, H    int
	Terrain terrain.Map
	// Geology is the stone of every rock and stone-ground cell, and the ore.
	Geology geology.Map
	// Roof is the natural roof: Natural over rock and roofed caves, plus
	// NaturalThick under deep rock.
	Roof   []roof.Cell
	Plants []PlantSpawn
	// Grass marks soft cells that start with grass.
	Grass  []bool
	Chunks []ChunkSpawn
	Ruins  []Ruin
	// Vents are the top-left cells of 2x2 steam vents.
	Vents []Point
	// Roads are the smoothed road centrelines (road cells are already
	// Track terrain).
	Roads [][]PointF
	// Start is the landing spot.
	Start   Point
	Metrics Metrics
	// Fields is the intermediate model, kept only with Config.Debug.
	Fields *Fields
}

// ErrSize is returned for an empty map.
var ErrSize = errors.New("mapgen: map size must be positive")

// Generate builds the whole map.
func Generate(cfg Config) (Result, error) {
	b, err := NewBuilder(cfg)
	if err != nil {
		return Result{}, err
	}
	for {
		if _, _, done, err := b.Step(); err != nil {
			return Result{}, err
		} else if done {
			return b.Result(), nil
		}
	}
}

// Builder runs the pipeline one step at a time (the loading screen).
type Builder struct {
	ctx  *Ctx
	next int
	res  Result
	done bool
}

// NewBuilder prepares cfg's map.
func NewBuilder(cfg Config) (*Builder, error) {
	if cfg.W <= 0 || cfg.H <= 0 {
		return nil, ErrSize
	}
	return &Builder{ctx: newCtx(cfg)}, nil
}

// Step runs the next step. label is the player-facing phase of the step
// that will run next ("" once done) and frac the share done.
func (b *Builder) Step() (label string, frac float32, done bool, err error) {
	if b.done {
		return "", 1, true, nil
	}
	if b.next < len(pipeline) {
		if err := pipeline[b.next].Run(b.ctx); err != nil {
			return "", 0, false, err
		}
		b.next++
	}
	if b.next >= len(pipeline) {
		b.done = true
		b.res = *b.ctx.R
		b.res.W, b.res.H = b.ctx.W, b.ctx.H
		if b.ctx.Cfg.Debug {
			b.res.Fields = b.ctx.F
		}
		return "", 1, true, nil
	}
	return Label(pipeline[b.next].Name()), float32(b.next) / float32(len(pipeline)), false, nil
}

// Result is the finished map (zero until Step reports done).
func (b *Builder) Result() Result { return b.res }

// Steps is the number of pipeline steps.
func Steps() int { return len(pipeline) }
