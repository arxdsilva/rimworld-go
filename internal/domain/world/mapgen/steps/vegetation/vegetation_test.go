package vegetation

import (
	"math"
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// fixture is a featureless w×h map of biome b: soft ground laid out from
// the biome's Ground mix by a smooth noise rank (as the terrain step
// would), with smooth Fertility and Wetness fields. Nothing is blocked.
func fixture(b biome.ID, seed uint64, w, h int) *mapgen.Ctx {
	s := site.Site{Biome: b}
	c := mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(b)})
	ns := noise2.Seed(seed, "fixture/ground")
	fs, ws := noise2.Seed(seed, "fixture/fert"), noise2.Seed(seed, "fixture/wet")
	vals := make([]float32, w*h)
	for i := range vals {
		x, y := c.XY(i)
		vals[i] = float32(noise2.FBm(ns, float64(x), float64(y), 3, 30, 0.5))
		c.F.Fertility[i] = float32(noise2.FBm(fs, float64(x), float64(y), 3, 40, 0.5))
		c.F.Wetness[i] = float32(noise2.FBm(ws, float64(x), float64(y), 3, 40, 0.5))
	}
	mapgen.RankNormalise(vals)
	g := biome.DefOf(b).Ground
	bands := []struct {
		t terrain.Type
		w int
	}{{terrain.Ice, g.Ice}, {terrain.Sand, g.Sand}, {terrain.Gravel, g.Gravel},
		{terrain.Marsh, g.Marsh}, {terrain.Soil, g.Soil}, {terrain.RichSoil, g.Rich}}
	total := 0
	for _, bd := range bands {
		total += bd.w
	}
	for i, v := range vals {
		t := terrain.Soil
		if total > 0 {
			acc := 0
			for _, bd := range bands {
				acc += bd.w
				if float64(v) <= float64(acc)/float64(total) && bd.w > 0 {
					t = bd.t
					break
				}
			}
		}
		c.R.Terrain.Cells[i] = t
		if t == terrain.Marsh { // marsh is wet ground
			c.F.Wetness[i] = 0.6 + 0.4*c.F.Wetness[i]
		}
	}
	return c
}

func run(t testing.TB, c *mapgen.Ctx) {
	t.Helper()
	if err := (Vegetation{}).Run(c); err != nil {
		t.Fatal(err)
	}
}

func cheb(ax, ay, bx, by int) int { return max(abs(ax-bx), abs(ay-by)) }

// eligibleTest is the test's own reading of the placement rule.
func eligibleTest(c *mapgen.Ctx, i int) bool {
	x, y := c.XY(i)
	t := c.R.Terrain.Cells[i]
	switch {
	case c.F.Blocked[i], c.F.Road[i], c.F.Rock[i], c.F.Cave[i], c.F.Water[i].Wet():
		return false
	case t.Fertility() == 0, terrain.IsWater(t), t == terrain.Ice:
		return false
	case cheb(x, y, c.R.Start.X, c.R.Start.Y) <= startClear:
		return false
	}
	return true
}

func countEligible(c *mapgen.Ctx) int {
	n := 0
	for i := range c.R.Terrain.Cells {
		if eligibleTest(c, i) {
			n++
		}
	}
	return n
}

func TestDeterministic(t *testing.T) {
	t.Parallel()
	a, b := fixture(biome.TemperateForest, 7, 120, 100), fixture(biome.TemperateForest, 7, 120, 100)
	o := reseed(fixture(biome.TemperateForest, 7, 120, 100), 8) // same inputs, other seed
	for _, c := range []*mapgen.Ctx{a, b, o} {
		run(t, c)
	}
	if !slices.Equal(a.R.Plants, b.R.Plants) || !slices.Equal(a.R.Grass, b.R.Grass) ||
		!slices.Equal(a.F.VegDensity, b.F.VegDensity) {
		t.Fatal("same seed differs")
	}
	if len(a.R.Plants) == 0 {
		t.Fatal("no plants")
	}
	if slices.Equal(a.R.Plants, o.R.Plants) {
		t.Fatal("other seed identical")
	}
}

// reseed is c's inputs under another seed.
func reseed(c *mapgen.Ctx, seed uint64) *mapgen.Ctx {
	cfg := c.Cfg
	cfg.Seed = seed
	o := mapgen.NewCtx(cfg)
	copy(o.R.Terrain.Cells, c.R.Terrain.Cells)
	copy(o.F.Fertility, c.F.Fertility)
	copy(o.F.Wetness, c.F.Wetness)
	return o
}

// featureFixture adds every kind of ineligible cell and moves the start.
func featureFixture(seed uint64) *mapgen.Ctx {
	c := fixture(biome.TemperateSwamp, seed, 100, 90)
	c.R.Start = mapgen.Point{X: 30, Y: 40}
	for i := range c.R.Terrain.Cells {
		x, y := c.XY(i)
		switch {
		case x < 10: // rock wall
			c.F.Rock[i] = true
			c.R.Terrain.Cells[i] = terrain.Rock
		case x >= 10 && x < 20 && y < 30: // a cave tunnel on soil (rule, not terrain)
			c.F.Cave[i] = true
		case x >= 90: // a lake
			c.F.Water[i] = mapgen.Deep
			c.R.Terrain.Cells[i] = terrain.DeepWater
		case x >= 85: // its shallow edge
			c.F.Water[i] = mapgen.Shallow
			c.R.Terrain.Cells[i] = terrain.ShallowWater
		case y == 60: // a road
			c.F.Road[i], c.F.Blocked[i] = true, true
			c.R.Terrain.Cells[i] = terrain.Track
		case y == 61 && x%2 == 0: // chunks beside it
			c.F.Blocked[i] = true
		case y > 75 && x < 40:
			c.R.Terrain.Cells[i] = terrain.Ice
		case y > 75 && x < 60:
			c.R.Terrain.Cells[i] = terrain.Stone
		}
	}
	c.R.Plants = append(c.R.Plants, mapgen.PlantSpawn{X: 50, Y: 10, Species: plant.Tree, Growth: 1})
	return c
}

func TestPlantsOnlyOnEligibleCells(t *testing.T) {
	t.Parallel()
	c := featureFixture(3)
	run(t, c)
	seen := map[int]bool{}
	for k, p := range c.R.Plants {
		i := c.Idx(p.X, p.Y)
		if seen[i] {
			t.Fatalf("two plants on (%d,%d)", p.X, p.Y)
		}
		seen[i] = true
		if k == 0 {
			continue // the pre-existing plant
		}
		if !eligibleTest(c, i) {
			t.Fatalf("plant %v on ineligible cell %v blocked=%v", p, c.R.Terrain.Cells[i], c.F.Blocked[i])
		}
	}
	if len(c.R.Plants) < 100 {
		t.Fatalf("only %d plants", len(c.R.Plants))
	}
}

func TestBlockedUntouched(t *testing.T) {
	t.Parallel()
	c := featureFixture(4)
	before := slices.Clone(c.F.Blocked)
	run(t, c)
	if !slices.Equal(before, c.F.Blocked) {
		t.Fatal("vegetation changed Blocked")
	}
}

func TestSpacing(t *testing.T) {
	t.Parallel()
	for _, b := range []biome.ID{biome.TropicalRainforest, biome.TemperateForest, biome.TemperateSwamp} {
		c := fixture(b, 11, 160, 160)
		run(t, c)
		grid := make([]int8, c.W*c.H) // 0 none, else group+1
		for _, p := range c.R.Plants {
			grid[c.Idx(p.X, p.Y)] = int8(groupOf(p.Species)) + 1
		}
		at := func(x, y int) int8 {
			if !c.In(x, y) {
				return 0
			}
			return grid[c.Idx(x, y)]
		}
		for _, p := range c.R.Plants {
			g := groupOf(p.Species)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					n := at(p.X+dx, p.Y+dy)
					if n == 0 || group(n-1) == gGround {
						continue
					}
					orth := dx == 0 || dy == 0
					if g == gTree && group(n-1) == gTree && orth {
						t.Fatalf("%v: tree beside tree at (%d,%d)", b, p.X, p.Y)
					}
					if g == gShrub || group(n-1) == gShrub {
						if g != gGround {
							t.Fatalf("%v: shrub crowded at (%d,%d)", b, p.X, p.Y)
						}
					}
				}
			}
		}
	}
}

func TestSpeciesTotalsMatchPermille(t *testing.T) {
	t.Parallel()
	for _, b := range []biome.ID{biome.TemperateForest, biome.BorealForest, biome.TropicalRainforest,
		biome.AridShrubland, biome.Desert, biome.TemperateSwamp} {
		c := fixture(b, 21, 200, 200)
		e := countEligible(c)
		run(t, c)
		counts := map[plant.Species]int{}
		for _, p := range c.R.Plants {
			counts[p.Species]++
		}
		for _, pd := range biome.DefOf(b).Plants {
			want := float64(pd.Permil) * float64(e) / 1000
			got := float64(counts[pd.Species])
			if math.Abs(got-want) > 0.25*want {
				t.Errorf("%v %s: %v plants, want %.0f ±25%%", b, plant.DefOf(pd.Species).Name, got, want)
			}
		}
	}
}

func treeCells(c *mapgen.Ctx) []mapgen.Point {
	var out []mapgen.Point
	for _, p := range c.R.Plants {
		if groupOf(p.Species) == gTree {
			out = append(out, mapgen.Point{X: p.X, Y: p.Y})
		}
	}
	return out
}

// clarkEvans is the mean nearest-neighbour distance over its expectation
// for the same count spread uniformly at random over area cells.
func clarkEvans(c *mapgen.Ctx, pts []mapgen.Point, area int) float64 {
	grid := make([]bool, c.W*c.H)
	for _, p := range pts {
		grid[c.Idx(p.X, p.Y)] = true
	}
	sum := 0.0
	for _, p := range pts {
		best := math.Inf(1)
		for r := 1; r < 60 && float64(r-1) < best; r++ {
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					if max(abs(dx), abs(dy)) != r || !c.In(p.X+dx, p.Y+dy) || !grid[c.Idx(p.X+dx, p.Y+dy)] {
						continue
					}
					best = min(best, math.Hypot(float64(dx), float64(dy)))
				}
			}
		}
		sum += best
	}
	mean := sum / float64(len(pts))
	return mean / (0.5 / math.Sqrt(float64(len(pts))/float64(area)))
}

func TestForestIsClustered(t *testing.T) {
	t.Parallel()
	for _, seed := range []uint64{31, 32, 33} {
		c := fixture(biome.TemperateForest, seed, 250, 250)
		e := countEligible(c)
		run(t, c)
		trees := treeCells(c)
		if ce := clarkEvans(c, trees, e); ce >= 0.85 {
			t.Errorf("seed %d: Clark-Evans %.2f, want < 0.85", seed, ce)
		}
		// groves: trees within 2 cells joined
		grid := make([]bool, c.W*c.H)
		for _, p := range trees {
			grid[c.Idx(p.X, p.Y)] = true
		}
		label := make([]int, c.W*c.H)
		groves := 0
		for _, p := range trees {
			if label[c.Idx(p.X, p.Y)] != 0 {
				continue
			}
			id := len(trees) + groves + 1
			stack := []mapgen.Point{p}
			label[c.Idx(p.X, p.Y)] = id
			size := 0
			for len(stack) > 0 {
				q := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				size++
				for dy := -2; dy <= 2; dy++ {
					for dx := -2; dx <= 2; dx++ {
						x, y := q.X+dx, q.Y+dy
						if c.In(x, y) && grid[c.Idx(x, y)] && label[c.Idx(x, y)] == 0 {
							label[c.Idx(x, y)] = id
							stack = append(stack, mapgen.Point{X: x, Y: y})
						}
					}
				}
			}
			if size >= 6 {
				groves++
			}
		}
		if groves < 4 {
			t.Errorf("seed %d: %d groves of 6+", seed, groves)
		}
		// clearings: 12×12 windows of eligible land without a tree
		clear := 0
		for y0 := 0; y0+12 <= c.H; y0 += 12 {
			for x0 := 0; x0+12 <= c.W; x0 += 12 {
				ok := true
				for y := y0; y < y0+12 && ok; y++ {
					for x := x0; x < x0+12 && ok; x++ {
						ok = !grid[c.Idx(x, y)] && eligibleTest(c, c.Idx(x, y))
					}
				}
				if ok {
					clear++
				}
			}
		}
		if clear < 3 {
			t.Errorf("seed %d: %d tree-free 12×12 clearings", seed, clear)
		}
	}
}

func TestGrowthBounds(t *testing.T) {
	t.Parallel()
	c := fixture(biome.TropicalRainforest, 5, 150, 150)
	run(t, c)
	lowest := map[group]int{gTree: 100, gShrub: 100, gGround: 100}
	for _, p := range c.R.Plants {
		d := plant.DefOf(p.Species)
		lo := d.GrowTicks * 10 / 100
		if groupOf(p.Species) == gTree {
			lo = d.GrowTicks * 25 / 100
		}
		if p.Growth < lo || p.Growth > d.GrowTicks {
			t.Fatalf("%s growth %d outside [%d, %d]", d.Name, p.Growth, lo, d.GrowTicks)
		}
		lowest[groupOf(p.Species)] = min(lowest[groupOf(p.Species)], p.Growth*100/d.GrowTicks)
	}
	if lowest[gTree] > 35 || lowest[gShrub] > 20 || lowest[gGround] > 20 {
		t.Fatalf("growth range too narrow: %v", lowest)
	}
}

func TestIceSheetIsBare(t *testing.T) {
	t.Parallel()
	c := fixture(biome.IceSheet, 9, 120, 120)
	run(t, c)
	if len(c.R.Plants) != 0 {
		t.Fatalf("%d plants on the ice sheet", len(c.R.Plants))
	}
	for i, g := range c.R.Grass {
		if g && c.R.Terrain.Cells[i] == terrain.Ice {
			t.Fatal("grass on ice")
		}
	}
}

// grassEligibleTest is the test's reading of the grass rule.
func grassEligibleTest(c *mapgen.Ctx, i int) bool {
	t := c.R.Terrain.Cells[i]
	return t.Fertility() > 0 && t != terrain.Mud && t != terrain.Track && !terrain.IsWater(t) &&
		!c.F.Blocked[i] && !c.F.Water[i].Wet()
}

func TestGrassShare(t *testing.T) {
	t.Parallel()
	for _, b := range []biome.ID{biome.TemperateForest, biome.BorealForest, biome.Tundra, biome.TropicalRainforest,
		biome.AridShrubland, biome.Desert, biome.ExtremeDesert, biome.TemperateSwamp, biome.ColdBog} {
		c := fixture(b, 41, 200, 200)
		run(t, c)
		n, g := 0, 0
		for i, has := range c.R.Grass {
			if !grassEligibleTest(c, i) {
				if has {
					t.Fatalf("%v: grass on ineligible %v", b, c.R.Terrain.Cells[i])
				}
				continue
			}
			n++
			if has {
				g++
			}
		}
		want := grassPermil(biome.DefOf(b))
		got := g * 1000 / max(1, n)
		if abs(got-want) > 100 {
			t.Errorf("%v: grass %d‰, want %d‰ ±100", b, got, want)
		}
	}
}

func TestGrassFollowsPlantsAndAvoidsDunes(t *testing.T) {
	t.Parallel()
	c := fixture(biome.Desert, 43, 200, 200)
	run(t, c)
	var sand, sandG, other, otherG int
	for i, has := range c.R.Grass {
		if !grassEligibleTest(c, i) {
			continue
		}
		if c.R.Terrain.Cells[i] == terrain.Sand {
			sand++
			if has {
				sandG++
			}
		} else {
			other++
			if has {
				otherG++
			}
		}
	}
	if sand == 0 || other == 0 || sandG*other*2 > otherG*sand {
		t.Fatalf("desert grass on sand %d/%d vs elsewhere %d/%d", sandG, sand, otherG, other)
	}
	// denser where the vegetation field is high
	f := fixture(biome.BorealForest, 44, 200, 200)
	run(t, f)
	var hiN, hiG, loN, loG int
	q75, q25 := mapgen.Quantile(f.F.VegDensity, 0.75), mapgen.Quantile(f.F.VegDensity, 0.25)
	for i, has := range f.R.Grass {
		if !grassEligibleTest(f, i) {
			continue
		}
		if f.F.VegDensity[i] > q75 {
			hiN++
			if has {
				hiG++
			}
		} else if f.F.VegDensity[i] < q25 {
			loN++
			if has {
				loG++
			}
		}
	}
	if hiG*loN <= loG*hiN {
		t.Fatalf("grass not denser under vegetation: hi %d/%d lo %d/%d", hiG, hiN, loG, loN)
	}
}

func TestWetnessPreference(t *testing.T) {
	t.Parallel()
	c := fixture(biome.TemperateSwamp, 51, 200, 200)
	run(t, c)
	var reed, all float64
	var nr, na int
	for _, p := range c.R.Plants {
		w := float64(c.F.Wetness[c.Idx(p.X, p.Y)])
		all += w
		na++
		if p.Species == plant.Reeds {
			reed += w
			nr++
		}
	}
	if nr == 0 || reed/float64(nr) < all/float64(na)+0.05 {
		t.Fatalf("reeds not on wet ground: %.2f vs %.2f", reed/float64(max(1, nr)), all/float64(na))
	}
	d := fixture(biome.Desert, 52, 200, 200)
	run(t, d)
	var cac float64
	var nc int
	all, na = 0, 0
	for _, p := range d.R.Plants {
		w := float64(d.F.Wetness[d.Idx(p.X, p.Y)])
		all += w
		na++
		if p.Species == plant.Cactus {
			cac += w
			nc++
		}
	}
	if nc == 0 || cac/float64(nc) > all/float64(na)-0.02 {
		t.Fatalf("cactus not on dry ground: %.2f vs %.2f", cac/float64(max(1, nc)), all/float64(na))
	}
}

func TestVegDensityField(t *testing.T) {
	t.Parallel()
	c := featureFixture(6)
	run(t, c)
	var lo, hi float32 = 1, 0
	for i, v := range c.F.VegDensity {
		if v < 0 || v > 1 || math.IsNaN(float64(v)) {
			t.Fatalf("VegDensity %v", v)
		}
		if c.F.Rock[i] || c.F.Water[i].Wet() {
			if v != 0 {
				t.Fatalf("VegDensity %v on rock or water", v)
			}
			continue
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	if hi-lo < 0.3 {
		t.Fatalf("VegDensity flat: %v..%v", lo, hi)
	}
}

func TestTinyAndEmptyMaps(t *testing.T) {
	t.Parallel()
	for _, sz := range [][2]int{{1, 1}, {5, 3}, {24, 24}, {50, 50}} {
		for _, b := range biome.All() {
			c := fixture(b.ID, 1, sz[0], sz[1])
			run(t, c)
			for _, p := range c.R.Plants {
				if !eligibleTest(c, c.Idx(p.X, p.Y)) {
					t.Fatalf("%v %v: plant on ineligible cell", sz, b.Key)
				}
			}
		}
	}
	// all rock: nothing grows, nothing panics
	c := fixture(biome.TropicalRainforest, 2, 60, 60)
	for i := range c.F.Rock {
		c.F.Rock[i] = true
		c.R.Terrain.Cells[i] = terrain.Rock
	}
	run(t, c)
	if len(c.R.Plants) != 0 || slices.Contains(c.R.Grass, true) {
		t.Fatal("plants or grass on solid rock")
	}
	// a custom biome without data or plants
	c = fixture(biome.TemperateForest, 2, 60, 60)
	c.Cfg.Biome = biome.Def{ID: biome.NumBiomes + 3}
	run(t, c)
	if len(c.R.Plants) != 0 {
		t.Fatal("plants without a plant list")
	}
}

func TestDataCoversEveryBiome(t *testing.T) {
	t.Parallel()
	for _, b := range biome.All() {
		d := dataOf(b.ID)
		if d.Density <= 0 || d.Density > 1 || d.TreeCell <= 0 || d.ShrubCell <= 0 || d.GroundCell <= 0 {
			t.Errorf("%s: bad data %+v", b.Key, d)
		}
		for _, pd := range b.Plants {
			tn := tuneOf(b.ID, pd.Species)
			if tn.WetLo < 0 || tn.WetHi > 1 || tn.WetLo >= tn.WetHi || tn.ClusterR < 0 || tn.ClusterW < 0 {
				t.Errorf("%s/%s: bad tune %+v", b.Key, plant.DefOf(pd.Species).Name, tn)
			}
		}
	}
	if grassPermil(biome.DefOf(biome.TemperateForest)) != 750 {
		t.Error("temperate forest grass share")
	}
	for s, want := range map[plant.Species]group{plant.Tree: gTree, plant.Pine: gTree, plant.JungleTree: gTree,
		plant.BerryBush: gShrub, plant.Shrub: gShrub, plant.Reeds: gShrub, plant.Cactus: gShrub,
		plant.Moss: gGround, plant.Agave: gGround, plant.Dandelion: gGround, plant.Bramble: gGround} {
		if groupOf(s) != want {
			t.Errorf("%s: group %d, want %d", plant.DefOf(s).Name, groupOf(s), want)
		}
	}
}

func BenchmarkVegetation500Rainforest(b *testing.B) {
	base := fixture(biome.TropicalRainforest, 99, 500, 500)
	terr := slices.Clone(base.R.Terrain.Cells)
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		c := mapgen.NewCtx(base.Cfg)
		copy(c.R.Terrain.Cells, terr)
		copy(c.F.Fertility, base.F.Fertility)
		copy(c.F.Wetness, base.F.Wetness)
		b.StartTimer()
		run(b, c)
	}
}
