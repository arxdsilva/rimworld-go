package pipeline

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/steps/start"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

// Profile tests (.cursor/PLAY013_PLAN.md §4): each landform profile must
// land in its range on every one of profileSeeds seeds, and every map must
// be sound and playable.

const profileSeeds = 30

type profile struct {
	name  string
	size  int
	site  site.Site
	check func(m mapgen.Metrics) error
}

func siteCfg(seed uint64, size int, s site.Site) mapgen.Config {
	return mapgen.Config{Seed: seed, W: size, H: size, Site: s, Biome: biome.DefOf(s.Biome), Debug: true}
}

var stones = []geology.Stone{geology.Granite, geology.Marble}

var profiles = []profile{
	{"mountainous", 200, site.Site{Biome: biome.TemperateForest, Hills: site.Mountainous, Stones: stones},
		func(m mapgen.Metrics) error {
			if m.LargestRockShare < 0.55 {
				return fmt.Errorf("largest rock body %.2f of all rock, want ≥ 0.55", m.LargestRockShare)
			}
			return nil
		}},
	{"flat", 200, site.Site{Biome: biome.TemperateForest, Hills: site.Flat, Stones: stones},
		func(m mapgen.Metrics) error {
			if m.RockShare > 0.08 || m.OpenShare < 0.7 {
				return fmt.Errorf("rock %.3f (≤ 0.08), open %.3f (≥ 0.7)", m.RockShare, m.OpenShare)
			}
			return nil
		}},
	{"forest", 250, site.Site{Biome: biome.TemperateForest, Hills: site.SmallHills, Stones: stones},
		func(m mapgen.Metrics) error {
			if m.TreeClusters < 4 {
				return fmt.Errorf("%d tree clusters, want ≥ 4", m.TreeClusters)
			}
			return nil
		}},
	// every feature at once: only the common checks apply
	{"mixed", 200, site.Site{Biome: biome.TemperateForest, Hills: site.LargeHills, Stones: stones,
		River: site.RiverPath{Size: site.River, In: 0, Out: 4}, Coast: 1 << 2, Lakes: 2, Caves: true,
		Road: 1<<6 | 1<<0, Settled: true, Rain: 900}, nil},
	// roads from four sides over a river, lakes and a coast: every road
	// stays connected, crossing water only on bridges (MAPGEN-3)
	{"crossings", 200, site.Site{Biome: biome.TemperateForest, Hills: site.SmallHills, Stones: stones,
		River: site.RiverPath{Size: site.River, In: 6, Out: 2}, Coast: 1 << 4, Lakes: 3,
		Road: 1<<0 | 1<<2 | 1<<4 | 1<<6, Rain: 900}, nil},
}

func TestProfiles(t *testing.T) {
	for _, p := range profiles {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			errs := make([]error, profileSeeds)
			ms := make([]mapgen.Metrics, profileSeeds)
			var wg sync.WaitGroup
			sem := make(chan struct{}, max(1, runtime.GOMAXPROCS(0)/2))
			for k := range profileSeeds {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					ms[k], errs[k] = checkMap(siteCfg(uint64(k+1), p.size, p.site), p.check)
				}()
			}
			wg.Wait()
			logRanges(t, ms)
			for k, err := range errs {
				if err != nil {
					t.Errorf("seed %d: %v", k+1, err)
				}
			}
		})
	}
}

// logRanges logs each metric's range over the seeds (go test -v), for
// tuning.
func logRanges(t *testing.T, ms []mapgen.Metrics) {
	get := []struct {
		name string
		f    func(m mapgen.Metrics) float64
	}{
		{"rock", func(m mapgen.Metrics) float64 { return m.RockShare }},
		{"largestRock", func(m mapgen.Metrics) float64 { return m.LargestRockShare }},
		{"tinyRock", func(m mapgen.Metrics) float64 { return float64(m.TinyRockBodies) }},
		{"patch", func(m mapgen.Metrics) float64 { return m.MeanPatchSize }},
		{"open", func(m mapgen.Metrics) float64 { return m.OpenShare }},
		{"groves", func(m mapgen.Metrics) float64 { return float64(m.TreeClusters) }},
		{"sinuosity", func(m mapgen.Metrics) float64 { return m.RiverSinuosity }},
		{"reach", func(m mapgen.Metrics) float64 { return m.StartReach }},
		{"repairs", func(m mapgen.Metrics) float64 { return float64(len(m.Repairs)) }},
	}
	for _, g := range get {
		lo, hi := g.f(ms[0]), g.f(ms[0])
		for _, m := range ms[1:] {
			lo, hi = min(lo, g.f(m)), max(hi, g.f(m))
		}
		t.Logf("%-12s %8.3f .. %8.3f", g.name, lo, hi)
	}
}

// checkMap generates cfg's map and runs the common checks plus check.
func checkMap(cfg mapgen.Config, check func(mapgen.Metrics) error) (mapgen.Metrics, error) {
	r, err := Generate(cfg)
	if err != nil {
		return mapgen.Metrics{}, err
	}
	m := r.Metrics
	err = commonChecks(cfg, r)
	if err == nil && check != nil {
		err = check(m)
	}
	return m, err
}

// commonChecks hold on every map: reach, invariants, edge margin and a dry
// start.
func commonChecks(cfg mapgen.Config, r mapgen.Result) error {
	m := r.Metrics
	if m.StartReach < 0.7 {
		return fmt.Errorf("start reaches %.2f of the land, want ≥ 0.7", m.StartReach)
	}
	c := &mapgen.Ctx{Cfg: cfg, W: r.W, H: r.H, F: r.Fields, R: &r}
	if v := start.Violations(c); len(v) > 0 {
		return fmt.Errorf("invariants broken: %v", v)
	}
	if err := roadChecks(c); err != nil {
		return err
	}
	mx, my := min(25, r.W/5), min(25, r.H/5)
	if p := r.Start; p.X < mx || p.Y < my || r.W-1-p.X < mx || r.H-1-p.Y < my {
		return fmt.Errorf("start %v inside the %d/%d edge margin", p, mx, my)
	}
	for y := r.Start.Y - 4; y <= r.Start.Y+4; y++ {
		for x := r.Start.X - 4; x <= r.Start.X+4; x++ {
			dx, dy := x-r.Start.X, y-r.Start.Y
			if dx*dx+dy*dy <= 16 && terrain.IsWater(r.Terrain.At(x, y)) {
				return fmt.Errorf("water at %d,%d within 4 of start %v", x, y, r.Start)
			}
		}
	}
	return nil
}

func TestGenerateDeterministic(t *testing.T) {
	for _, p := range profiles {
		cfg := siteCfg(7, 120, p.site)
		a, err := Generate(cfg)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := Generate(cfg)
		for _, f := range []struct {
			name string
			x, y any
		}{
			{"terrain", a.Terrain, b.Terrain}, {"plants", a.Plants, b.Plants}, {"chunks", a.Chunks, b.Chunks},
			{"ruins", a.Ruins, b.Ruins}, {"vents", a.Vents, b.Vents}, {"roof", a.Roof, b.Roof},
			{"geology", a.Geology, b.Geology}, {"grass", a.Grass, b.Grass}, {"start", a.Start, b.Start},
			{"metrics", a.Metrics, b.Metrics},
		} {
			if !reflect.DeepEqual(f.x, f.y) {
				t.Fatalf("%s: %s differs between two runs of one seed", p.name, f.name)
			}
		}
		other, _ := Generate(siteCfg(8, 120, p.site))
		if reflect.DeepEqual(a.Terrain, other.Terrain) && reflect.DeepEqual(a.Plants, other.Plants) {
			t.Fatalf("%s: seeds 7 and 8 made the same map", p.name)
		}
	}
}

// Budgets: ≤ 2 s per 500² map, ≤ 0.6 s per 250² map, on one core.
func BenchmarkGenerate500(b *testing.B) { benchGenerate(b, 500) }

func BenchmarkGenerate250(b *testing.B) { benchGenerate(b, 250) }

func benchGenerate(b *testing.B, size int) {
	s := profiles[3].site
	seed := uint64(0)
	for b.Loop() {
		seed++
		cfg := siteCfg(seed, size, s)
		cfg.Debug = false
		if _, err := Generate(cfg); err != nil {
			b.Fatal(err)
		}
	}
}

// roadChecks (MAPGEN-3): every road joins its edge to its junction, none
// was given up, and every road cell over water is a bridge over
// bridgeable, non-sea terrain.
func roadChecks(c *mapgen.Ctx) error {
	r := c.R
	if b := start.BrokenRoads(c); len(b) > 0 {
		return fmt.Errorf("roads %v not connected (%v)", b, r.Metrics.Repairs)
	}
	for _, rep := range r.Metrics.Repairs {
		if strings.Contains(rep, "reverted") || strings.Contains(rep, "cannot reach") {
			return fmt.Errorf("road given up: %s", rep)
		}
	}
	if len(c.F.RoadEnds) != len(r.Roads) {
		return fmt.Errorf("%d road ends for %d roads", len(c.F.RoadEnds), len(r.Roads))
	}
	bridge := make([]bool, r.W*r.H)
	for _, b := range r.Bridges {
		i := b.Y*r.W + b.X
		bridge[i] = true
		if t := r.Terrain.Cells[i]; !terrain.Bridgeable(t) || r.Ocean[i] || !c.F.Road[i] {
			return fmt.Errorf("bridge %v on %v (ocean %v, road %v)", b, t, r.Ocean[i], c.F.Road[i])
		}
	}
	for i, t := range r.Terrain.Cells {
		if c.F.Road[i] && terrain.IsWater(t) && !bridge[i] {
			return fmt.Errorf("road over water without a bridge at %d,%d", i%r.W, i/r.W)
		}
	}
	return nil
}

// TestOceanMarksTheSea (MAPGEN-3): Result.Ocean is the kept sea, the coast
// water, and lies only on water.
func TestOceanMarksTheSea(t *testing.T) {
	for _, p := range []profile{profiles[4], profiles[0]} {
		r, err := Generate(siteCfg(3, 120, p.site))
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for i, o := range r.Ocean {
			wet := terrain.IsWater(r.Terrain.Cells[i])
			if o {
				n++
				if !wet {
					t.Fatalf("%s: ocean on %v", p.name, r.Terrain.Cells[i])
				}
			}
			if r.Fields.Coast[i] && r.Fields.Water[i].Wet() && wet && !o {
				t.Fatalf("%s: coast water at %d,%d not marked ocean", p.name, i%r.W, i/r.W)
			}
		}
		if (p.site.Coast != 0) != (n > 0) {
			t.Fatalf("%s: %d ocean cells", p.name, n)
		}
	}
}
