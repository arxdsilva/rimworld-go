package mapgen

import (
	"math"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
	"rimworld-go/pkg/randx"
)

// Step is one generation pass. Run reads and writes ctx.F and ctx.R.
type Step interface {
	Name() string
	Run(ctx *Ctx) error
}

// Ctx is the shared state of one generation.
type Ctx struct {
	Cfg  Config
	W, H int
	F    *Fields
	R    *Result
	base uint64
}

// NewCtx allocates every field and result grid for cfg (W, H > 0): the
// pipeline's start, and step tests' fixture.
func NewCtx(cfg Config) *Ctx {
	n := cfg.W * cfg.H
	far := func() []float32 {
		s := make([]float32, n)
		for i := range s {
			s[i] = Far
		}
		return s
	}
	f := &Fields{
		Elevation: make([]float32, n), MountainBias: make([]float32, n),
		Fertility: make([]float32, n), Wetness: make([]float32, n),
		VegDensity: make([]float32, n), ChunkDensity: make([]float32, n),
		RiverDist: far(), CoastDist: far(), StartScore: make([]float32, n),
		Stone: make([]geology.Stone, n), Rock: make([]bool, n), Cave: make([]bool, n),
		RockDepth: make([]uint8, n), Water: make([]Water, n),
		River: make([]bool, n), Coast: make([]bool, n), Lake: make([]bool, n),
		Road: make([]bool, n), Blocked: make([]bool, n),
	}
	r := &Result{
		W: cfg.W, H: cfg.H,
		Terrain: terrain.NewMap(cfg.W, cfg.H),
		Geology: geology.NewMap(cfg.W, cfg.H),
		Roof:    make([]roof.Cell, n),
		Grass:   make([]bool, n),
		Start:   Point{cfg.W / 2, cfg.H / 2},
	}
	return &Ctx{Cfg: cfg, W: cfg.W, H: cfg.H, F: f, R: r, base: cfg.Seed ^ versionSalt}
}

// Rand is step's own random stream: the same for a (seed, step) pair and
// unrelated to every other step's. Call it once per step and keep it.
func (c *Ctx) Rand(step string) *randx.Rand { return randx.Derive(c.base, step) }

// NoiseSeed is the noise2 seed of a named field.
func (c *Ctx) NoiseSeed(name string) uint64 { return noise2.Seed(c.base, name) }

// Scale is the map's linear size relative to a 250×250 map: feature sizes
// that should grow with the map multiply by it.
func (c *Ctx) Scale() float64 { return math.Sqrt(float64(c.W*c.H)) / 250 }

// In reports (x, y) on the map.
func (c *Ctx) In(x, y int) bool { return x >= 0 && y >= 0 && x < c.W && y < c.H }

// Idx is (x, y)'s row-major index.
func (c *Ctx) Idx(x, y int) int { return y*c.W + x }

// XY is index i's cell.
func (c *Ctx) XY(i int) (int, int) { return i % c.W, i / c.W }

// Start is the landing spot chosen so far (the map centre until "start").
func (c *Ctx) Start() Point { return c.R.Start }
