package ground

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// newCtx is a featureless fixture: no rock, water, caves or elevation
// bands (the taus stay 0, which the terrain step reads as "no bands").
func newCtx(seed uint64, w, h int, b biome.ID, stones ...geology.Stone) *mapgen.Ctx {
	s := site.Site{Biome: b, Stones: stones, Climate: biome.DefOf(b).Climate}
	return mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(b)})
}

// fillWet stands in for the wetness step: a smooth field in [0, 1].
func fillWet(c *mapgen.Ctx) {
	s := noise2.Seed(99, "test/wet")
	for i := range c.F.Wetness {
		x, y := c.XY(i)
		c.F.Wetness[i] = float32(noise2.FBm(s, float64(x), float64(y), 3, 40, 0.5))
	}
}

// runAll runs the three ground steps on a fixture.
func runAll(t testing.TB, c *mapgen.Ctx) {
	t.Helper()
	for _, s := range []mapgen.Step{Fertility{}, Material{}, Terrain{}} {
		if err := s.Run(c); err != nil {
			t.Fatalf("%s: %v", s.Name(), err)
		}
	}
}

func TestStepNames(t *testing.T) {
	if (Fertility{}).Name() != "fertility" || (Material{}).Name() != "material" || (Terrain{}).Name() != "terrain" {
		t.Fatal("step names are the contract")
	}
}

// ---- fertility ----

func TestFertilityRankedAndSmooth(t *testing.T) {
	c := newCtx(1, 200, 200, biome.TemperateForest)
	if err := (Fertility{}).Run(c); err != nil {
		t.Fatal(err)
	}
	f := c.F.Fertility
	if slices.Min(f) != 0 || slices.Max(f) != 1 {
		t.Fatalf("fertility not rank-normalised: [%v, %v]", slices.Min(f), slices.Max(f))
	}
	// rank-normalised: the median is ~0.5
	if q := mapgen.Quantile(f, 0.5); q < 0.49 || q > 0.51 {
		t.Fatalf("median %v", q)
	}
	// smooth: neighbouring cells differ little on average
	sum, n := 0.0, 0
	for y := 0; y < c.H; y++ {
		for x := 1; x < c.W; x++ {
			d := float64(f[c.Idx(x, y)] - f[c.Idx(x-1, y)])
			sum += max(d, -d)
			n++
		}
	}
	if m := sum / float64(n); m > 0.03 {
		t.Fatalf("fertility too noisy: mean neighbour diff %.4f", m)
	}
	// medium-scale patches: the fertile quarter forms a handful of large
	// blobs, not hundreds of specks
	top := make([]bool, len(f))
	for i, v := range f {
		top[i] = v >= 0.75
	}
	_, sizes := mapgen.Components(c.W, c.H, top, false)
	big := 0
	for _, s := range sizes {
		if s >= 100 {
			big += s
		}
	}
	if big*10 < len(f)/4*8 {
		t.Fatalf("fertile quarter is speckled: %d of %d cells in blobs ≥ 100", big, len(f)/4)
	}
}

func TestFertilityDeterministic(t *testing.T) {
	a, b, o := newCtx(5, 80, 60, biome.Desert), newCtx(5, 80, 60, biome.Desert), newCtx(6, 80, 60, biome.Desert)
	for _, c := range []*mapgen.Ctx{a, b, o} {
		if err := (Fertility{}).Run(c); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(a.F.Fertility, b.F.Fertility) {
		t.Fatal("same seed, different fertility")
	}
	if slices.Equal(a.F.Fertility, o.F.Fertility) {
		t.Fatal("other seed, same fertility")
	}
}

// ---- material ----

func stoneSet(s []geology.Stone) []geology.Stone {
	var out []geology.Stone
	for _, v := range s {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}

func coherence(c *mapgen.Ctx) float64 {
	same := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			s, ok := c.F.Stone[c.Idx(x, y)], true
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if nx, ny := x+d[0], y+d[1]; c.In(nx, ny) && c.F.Stone[c.Idx(nx, ny)] != s {
					ok = false
				}
			}
			if ok {
				same++
			}
		}
	}
	return float64(same) / float64(c.W*c.H)
}

func TestMaterialSiteStones(t *testing.T) {
	both := false
	for seed := uint64(1); seed <= 6; seed++ {
		c := newCtx(seed, 200, 200, biome.TemperateForest, geology.Granite, geology.None, geology.Granite, geology.Marble)
		if err := (Material{}).Run(c); err != nil {
			t.Fatal(err)
		}
		set := stoneSet(c.F.Stone)
		for _, s := range set {
			if s != geology.Granite && s != geology.Marble {
				t.Fatalf("seed %d: stone %v not in the site list", seed, s)
			}
		}
		if len(set) == 2 {
			both = true
		}
		if k := coherence(c); k < 0.95 {
			t.Fatalf("seed %d: stone regions speckled, coherence %.3f", seed, k)
		}
	}
	if !both {
		t.Fatal("two site stones never both appear")
	}
}

func TestMaterialPicksStonesWhenSiteHasNone(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		c := newCtx(seed, 200, 200, biome.Tundra)
		if err := (Material{}).Run(c); err != nil {
			t.Fatal(err)
		}
		set := stoneSet(c.F.Stone)
		if len(set) < 1 || len(set) > 3 || slices.Contains(set, geology.None) {
			t.Fatalf("seed %d: stones %v", seed, set)
		}
		if k := coherence(c); k < 0.95 {
			t.Fatalf("seed %d: coherence %.3f", seed, k)
		}
		// broad regions only: no stone pocket below 2% of the map
		_, sizes, _ := sameComponents(c.W, c.H, c.F.Stone)
		if len(sizes) > 8 || slices.Min(sizes) < c.W*c.H/50 {
			t.Fatalf("seed %d: stone regions %v", seed, sizes)
		}
	}
}

func TestMaterialSingleStoneAndDeterminism(t *testing.T) {
	c := newCtx(3, 50, 40, biome.Desert, geology.Slate)
	if err := (Material{}).Run(c); err != nil {
		t.Fatal(err)
	}
	if set := stoneSet(c.F.Stone); !slices.Equal(set, []geology.Stone{geology.Slate}) {
		t.Fatalf("single stone map has %v", set)
	}
	a, b, o := newCtx(4, 120, 90, biome.Desert), newCtx(4, 120, 90, biome.Desert), newCtx(40, 120, 90, biome.Desert)
	for _, c := range []*mapgen.Ctx{a, b, o} {
		if err := (Material{}).Run(c); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(a.F.Stone, b.F.Stone) {
		t.Fatal("same seed, different stones")
	}
	if slices.Equal(a.F.Stone, o.F.Stone) {
		t.Fatal("other seed, same stones")
	}
}

// ---- terrain precedence ----

func TestTerrainPrecedence(t *testing.T) {
	c := newCtx(1, 10, 10, biome.TemperateForest, geology.Granite)
	f := c.F
	f.RockTau, f.RoughTau, f.GravelTau = 0.95, 0.9, 0.8
	set := func(i int, w mapgen.Water, rock, cave bool, elev float32) {
		f.Water[i], f.Rock[i], f.Cave[i], f.Elevation[i] = w, rock, cave, elev
	}
	cases := []struct {
		w    mapgen.Water
		rock bool
		cave bool
		elev float32
		want terrain.Type
	}{
		{mapgen.Deep, true, true, 1, terrain.DeepWater},
		{mapgen.Shallow, true, false, 1, terrain.ShallowWater},
		{mapgen.Beach, true, true, 1, terrain.Rock},
		{mapgen.Floodplain, false, true, 1, terrain.Stone}, // cave floor (dry: no puddle, see below)
		{mapgen.Beach, false, false, 1, terrain.Sand},
		{mapgen.Floodplain, false, false, 0.92, terrain.RichSoil},
		{mapgen.Dry, false, false, 0.92, terrain.Stone},
		{mapgen.Dry, false, false, 0.85, terrain.Gravel},
	}
	e := newEnv(c)
	for k, tc := range cases {
		set(k, tc.w, tc.rock, tc.cave, tc.elev)
		f.Wetness[k] = 0
		got, ok := e.fixed(k)
		if tc.w == mapgen.Floodplain && tc.cave {
			// the cave floor may carry a gravel patch by noise
			if !ok || got != terrain.Stone && got != terrain.Gravel {
				t.Fatalf("case %d: cave floor got %v", k, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Fatalf("case %d: got %v (%v), want %v", k, got, ok, tc.want)
		}
	}
	// below every band, no water feature: not fixed (patches and mix decide)
	set(20, mapgen.Dry, false, false, 0.5)
	if _, ok := e.fixed(20); ok {
		t.Fatal("plain ground decided by the fixed rules")
	}
	// the shore: marsh when wet, mud when not
	set(21, mapgen.Shore, false, false, 0)
	f.Wetness[21] = 0.7
	if got, _ := e.fixed(21); got != terrain.Marsh {
		t.Fatalf("wet shore %v", got)
	}
	f.Wetness[21] = 0.2
	if got, _ := e.fixed(21); got != terrain.Mud {
		t.Fatalf("dry shore %v", got)
	}
	// zero taus (no rockmask yet): no elevation bands
	c.F.RoughTau, c.F.GravelTau = 0, 0
	e = newEnv(c)
	set(22, mapgen.Dry, false, false, 1)
	if _, ok := e.fixed(22); ok {
		t.Fatal("zero taus must disable the elevation bands")
	}
}

func TestBeachIceWhenCold(t *testing.T) {
	for _, b := range []biome.ID{biome.Tundra, biome.IceSheet} {
		c := newCtx(1, 4, 4, b)
		c.F.Water[0] = mapgen.Beach
		if got, _ := newEnv(c).fixed(0); got != terrain.Ice {
			t.Fatalf("%v beach: %v", b, got)
		}
	}
	c := newCtx(1, 4, 4, biome.BorealForest) // 0 °C mean: not below -2 °C
	c.F.Water[0] = mapgen.Beach
	if got, _ := newEnv(c).fixed(0); got != terrain.Sand {
		t.Fatalf("boreal beach: %v", got)
	}
}

func TestBankTypes(t *testing.T) {
	for _, b := range []biome.ID{biome.TemperateForest, biome.IceSheet} {
		c := newCtx(2, 120, 120, b)
		fillWet(c)
		e := newEnv(c)
		seen := map[terrain.Type]int{}
		for i := range c.F.Water {
			c.F.Water[i] = mapgen.Bank
			got, ok := e.fixed(i)
			if !ok {
				t.Fatal("bank not fixed")
			}
			seen[got]++
		}
		allowed := []terrain.Type{terrain.Gravel, terrain.Sand, terrain.Mud}
		if b == biome.IceSheet {
			allowed = []terrain.Type{terrain.Gravel, terrain.Ice}
		}
		for ty := range seen {
			if !slices.Contains(allowed, ty) {
				t.Fatalf("%v bank has %v", b, ty)
			}
		}
		if len(seen) < 2 {
			t.Fatalf("%v bank is one type: %v", b, seen)
		}
	}
}

// ---- terrain over whole maps ----

// banded adds every water band, a rock block and a cave, the way the
// earlier steps would.
func banded(c *mapgen.Ctx) {
	f := c.F
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			i := c.Idx(x, y)
			switch {
			case x < 6:
				f.Water[i] = mapgen.Deep
			case x < 9:
				f.Water[i] = mapgen.Shallow
			case x < 14:
				f.Water[i] = mapgen.Beach
			case y >= 40 && y < 44 && x >= 20:
				f.Water[i] = mapgen.Shallow
			case (y >= 37 && y < 40 || y >= 44 && y < 47) && x >= 20:
				f.Water[i] = mapgen.Bank
			case (y >= 33 && y < 37 || y >= 47 && y < 51) && x >= 20:
				f.Water[i] = mapgen.Floodplain
			case x >= 60 && x < 75 && y >= 60 && y < 75:
				f.Water[i] = mapgen.Shore
				if x >= 63 && x < 72 && y >= 63 && y < 72 {
					f.Water[i] = mapgen.Deep
				}
			case x >= 120 && y >= 120:
				f.Rock[i] = !(x >= 130 && x < 170 && y >= 140 && y < 150)
				f.Cave[i] = !f.Rock[i]
			}
		}
	}
}

// forbidden lists the types a biome may never show.
func forbidden(b biome.ID) []terrain.Type {
	switch b {
	case biome.IceSheet:
		return []terrain.Type{terrain.Sand, terrain.Marsh, terrain.Mud, terrain.Soil, terrain.RichSoil}
	case biome.SeaIce:
		return []terrain.Type{terrain.Sand, terrain.Marsh, terrain.Mud, terrain.Soil, terrain.RichSoil, terrain.Gravel, terrain.Stone}
	case biome.Tundra:
		return nil // cold but not glacial: icy beaches, sandy banks
	}
	return []terrain.Type{terrain.Ice}
}

func TestTerrainPlausiblePerBiome(t *testing.T) {
	for _, d := range biome.All() {
		c := newCtx(11, 200, 200, d.ID, geology.Granite, geology.Slate)
		fillWet(c)
		banded(c)
		c.F.RockTau, c.F.RoughTau, c.F.GravelTau = 0.99, 0.97, 0.94
		runAll(t, c)
		for i, ty := range c.R.Terrain.Cells {
			if slices.Contains(forbidden(d.ID), ty) {
				t.Fatalf("%s: %v at %v", d.Key, ty, i)
			}
			if terrain.IsWater(ty) && !c.F.Water[i].Wet() {
				t.Fatalf("%s: water terrain on dry field (I6) at %d", d.Key, i)
			}
			if (ty == terrain.Rock) != c.F.Rock[i] {
				t.Fatalf("%s: I1 broken at %d", d.Key, i)
			}
		}
		checkI2(t, c, d.Key)
		checkCleanup(t, c, d.Key)
	}
}

func checkI2(t *testing.T, c *mapgen.Ctx, label string) {
	t.Helper()
	for i, ty := range c.R.Terrain.Cells {
		hard := ty == terrain.Rock || ty == terrain.Stone
		if hard != (c.R.Geology.Stone[i] != geology.None) {
			t.Fatalf("%s: I2 broken at %d: %v with stone %v", label, i, ty, c.R.Geology.Stone[i])
		}
	}
}

// checkCleanup: every soft patch below its minimum is boxed in by ground
// cleanup must not change (water, rock).
func checkCleanup(t *testing.T, c *mapgen.Ctx, label string) {
	t.Helper()
	cells := c.R.Terrain.Cells
	lab, sizes, _ := typeComponents(c.W, c.H, cells)
	for i, ty := range cells {
		m := minPatch(ty)
		if m == 0 || sizes[lab[i]] >= m {
			continue
		}
		x, y := c.XY(i)
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if !c.In(nx, ny) {
				continue
			}
			if n := cells[c.Idx(nx, ny)]; n != ty && mergeTarget(n) {
				t.Fatalf("%s: %v patch of %d cells (min %d) at %d,%d next to %v", label, ty, sizes[lab[i]], m, x, y, n)
			}
		}
	}
}

func TestGroundMixShares(t *testing.T) {
	for _, d := range biome.All() {
		c := newCtx(21, 200, 200, d.ID)
		fillWet(c)
		runAll(t, c)
		g := groundOf(d)
		total := g.Soil + g.Rich + g.Gravel + g.Sand + g.Marsh + g.Ice
		want := map[terrain.Type]float64{
			terrain.Soil: float64(g.Soil) / float64(total), terrain.RichSoil: float64(g.Rich) / float64(total),
			terrain.Gravel: float64(g.Gravel) / float64(total), terrain.Sand: float64(g.Sand) / float64(total),
			terrain.Marsh: float64(g.Marsh) / float64(total), terrain.Ice: float64(g.Ice) / float64(total),
		}
		got := map[terrain.Type]float64{}
		for _, ty := range c.R.Terrain.Cells {
			got[ty] += 1 / float64(len(c.R.Terrain.Cells))
		}
		for ty, w := range want {
			if dlt := got[ty] - w; dlt > 0.10 || dlt < -0.10 {
				t.Fatalf("%s: %v share %.3f, mix %.3f", d.Key, ty, got[ty], w)
			}
		}
		checkCleanup(t, c, d.Key)
	}
}

func TestTemperateForestDefaultGround(t *testing.T) {
	g := groundOf(biome.DefOf(biome.TemperateForest))
	if g.Soil < 700 || g.Rich == 0 || g.Gravel == 0 || g.Ice != 0 || g.Sand != 0 {
		t.Fatalf("default ground %+v", g)
	}
	if groundOf(biome.DefOf(biome.Desert)) != biome.DefOf(biome.Desert).Ground {
		t.Fatal("a biome's own mix is kept")
	}
}

func TestTerrainDeterministic(t *testing.T) {
	mk := func(seed uint64) *mapgen.Ctx {
		c := newCtx(seed, 200, 200, biome.TemperateSwamp)
		fillWet(c)
		banded(c)
		runAll(t, c)
		return c
	}
	a, b, o := mk(8), mk(8), mk(9)
	if !slices.Equal(a.R.Terrain.Cells, b.R.Terrain.Cells) || !slices.Equal(a.R.Geology.Stone, b.R.Geology.Stone) {
		t.Fatal("same seed, different terrain")
	}
	if slices.Equal(a.R.Terrain.Cells, o.R.Terrain.Cells) {
		t.Fatal("other seed, same terrain")
	}
}

func TestGeologyFallbackStone(t *testing.T) {
	// F.Stone left None (no material step): the first site stone is used
	c := newCtx(1, 30, 30, biome.TemperateForest, geology.None, geology.Limestone, geology.Granite)
	for i := 0; i < 100; i++ {
		c.F.Rock[i] = true
	}
	if err := (Terrain{}).Run(c); err != nil {
		t.Fatal(err)
	}
	checkI2(t, c, "fallback")
	if c.R.Geology.Stone[0] != geology.Limestone {
		t.Fatalf("fallback stone %v", c.R.Geology.Stone[0])
	}
	// no site stone either: still a real stone
	c = newCtx(1, 30, 30, biome.TemperateForest)
	c.F.Rock[0] = true
	if err := (Terrain{}).Run(c); err != nil {
		t.Fatal(err)
	}
	checkI2(t, c, "no stones")
	// stale geology from before is cleared
	c.R.Geology.Stone[500] = geology.Marble
	if err := (Terrain{}).Run(c); err != nil {
		t.Fatal(err)
	}
	checkI2(t, c, "stale")
}

func TestSeaIceIsAllIce(t *testing.T) {
	c := newCtx(3, 100, 100, biome.SeaIce)
	fillWet(c)
	c.F.RockTau, c.F.RoughTau, c.F.GravelTau = 0.9, 0.8, 0.7
	for i := range c.F.Elevation {
		c.F.Elevation[i] = float32(i%100) / 100
		if i%7 == 0 {
			c.F.Water[i] = mapgen.Beach
		}
		if i < 300 {
			c.F.Cave[i] = true
		}
	}
	runAll(t, c)
	for i, ty := range c.R.Terrain.Cells {
		if ty != terrain.Ice || c.R.Geology.Stone[i] != geology.None {
			t.Fatalf("sea ice cell %d: %v, stone %v", i, ty, c.R.Geology.Stone[i])
		}
	}
}

func TestCaveFloor(t *testing.T) {
	c := newCtx(5, 120, 120, biome.TemperateForest, geology.Marble)
	for i := range c.F.Cave {
		c.F.Cave[i] = true
		c.F.Wetness[i] = 0.2
	}
	runAll(t, c)
	n := map[terrain.Type]int{}
	for _, ty := range c.R.Terrain.Cells {
		n[ty]++
	}
	if n[terrain.Stone] < len(c.F.Cave)/2 || n[terrain.Gravel] == 0 || len(n) != 2 {
		t.Fatalf("dry cave floor mix %v", n)
	}
	checkI2(t, c, "cave")
	// soaked caves get a few puddles, marked as shallow water in the field
	c = newCtx(5, 120, 120, biome.TemperateForest, geology.Marble)
	for i := range c.F.Cave {
		c.F.Cave[i] = true
		c.F.Wetness[i] = 0.95
	}
	runAll(t, c)
	p := 0
	for i, ty := range c.R.Terrain.Cells {
		if ty == terrain.ShallowWater {
			p++
			if c.F.Water[i] != mapgen.Shallow {
				t.Fatal("cave puddle without a Shallow field (I6)")
			}
		}
	}
	if p == 0 || p > len(c.F.Cave)/3 {
		t.Fatalf("soaked cave puddles: %d of %d", p, len(c.F.Cave))
	}
}

// ---- patches ----

func TestPatchTableCoversEveryBiome(t *testing.T) {
	for _, d := range biome.All() {
		ps := patchesOf(d.ID)
		if len(ps) == 0 {
			t.Fatalf("%s has no patches", d.Key)
		}
		for _, p := range ps {
			if p.MinSize < 1 || p.FertMin > p.FertMax || p.WetMin > p.WetMax || p.NoiseCell < 0 {
				t.Fatalf("%s: bad patch %+v", d.Key, p)
			}
			if minPatch(p.Terrain) == 0 && p.Terrain != terrain.Ice {
				t.Fatalf("%s: patch of %v", d.Key, p.Terrain)
			}
		}
	}
	if patchesOf(biome.NumBiomes+3) != nil {
		t.Fatal("unknown biome has patches")
	}
}

func TestPatchesDropSmallComponents(t *testing.T) {
	c := newCtx(1, 40, 40, biome.TemperateForest)
	// a patch that covers wet cells only: two blobs, 3 and 30 cells
	ps := []patch{{Terrain: terrain.Marsh, FertMax: 1, WetMin: 0.5, WetMax: 1, MinSize: 8}}
	for _, i := range []int{c.Idx(1, 1), c.Idx(2, 1), c.Idx(3, 1)} {
		c.F.Wetness[i] = 1
	}
	for y := 10; y < 15; y++ {
		for x := 10; x < 16; x++ {
			c.F.Wetness[c.Idx(x, y)] = 1
		}
	}
	rest := make([]int32, c.W*c.H)
	for i := range rest {
		rest[i] = int32(i)
	}
	e := newEnv(c)
	left, counts := e.applyPatches(ps, rest)
	if counts[terrain.Marsh] != 30 || len(left) != c.W*c.H-30 {
		t.Fatalf("marsh %d, left %d", counts[terrain.Marsh], len(left))
	}
	if c.R.Terrain.Cells[c.Idx(1, 1)] == terrain.Marsh || c.R.Terrain.Cells[c.Idx(12, 12)] != terrain.Marsh {
		t.Fatal("small patch kept or big patch dropped")
	}
}

// ---- cleanup ----

func TestCleanupMergesSmallSoftPatchesOnly(t *testing.T) {
	w, h := 12, 12
	cells := make([]terrain.Type, w*h)
	for i := range cells {
		cells[i] = terrain.Soil
	}
	at := func(x, y int) int { return y*w + x }
	cells[at(2, 2)] = terrain.Sand
	cells[at(3, 2)] = terrain.Sand
	cells[at(6, 6)] = terrain.ShallowWater
	cells[at(8, 2)] = terrain.Rock
	cells[at(2, 8)] = terrain.Stone
	cells[at(8, 8)] = terrain.Ice
	cleanup(w, h, cells)
	if cells[at(2, 2)] != terrain.Soil || cells[at(3, 2)] != terrain.Soil {
		t.Fatal("2-cell sand patch kept")
	}
	for _, k := range []struct {
		x, y int
		t    terrain.Type
	}{{6, 6, terrain.ShallowWater}, {8, 2, terrain.Rock}, {2, 8, terrain.Stone}, {8, 8, terrain.Ice}} {
		if cells[at(k.x, k.y)] != k.t {
			t.Fatalf("cleanup changed %v at %d,%d", k.t, k.x, k.y)
		}
	}
	// a small soft patch boxed in by water stays
	cells = []terrain.Type{
		terrain.DeepWater, terrain.DeepWater, terrain.DeepWater,
		terrain.DeepWater, terrain.Marsh, terrain.DeepWater,
		terrain.DeepWater, terrain.DeepWater, terrain.DeepWater,
	}
	cleanup(3, 3, cells)
	if cells[4] != terrain.Marsh {
		t.Fatal("boxed-in marsh changed")
	}
	// two small patches touching each other end as one type
	cells = []terrain.Type{terrain.Rock, terrain.Sand, terrain.Marsh, terrain.Rock}
	cleanup(4, 1, cells)
	if cells[1] != cells[2] {
		t.Fatalf("touching small patches: %v", cells)
	}
}

func TestMinPatch(t *testing.T) {
	want := map[terrain.Type]int{terrain.Sand: 6, terrain.Marsh: 8, terrain.RichSoil: 6, terrain.Gravel: 4, terrain.Mud: 5, terrain.Soil: 6}
	for ty, m := range want {
		if minPatch(ty) != m {
			t.Fatalf("%v min %d", ty, minPatch(ty))
		}
	}
	for _, ty := range []terrain.Type{terrain.Rock, terrain.Stone, terrain.Ice, terrain.DeepWater, terrain.ShallowWater, terrain.Track} {
		if minPatch(ty) != 0 {
			t.Fatalf("%v is not soft ground", ty)
		}
	}
}

// ---- small maps ----

func TestTinyMaps(t *testing.T) {
	for _, d := range biome.All() {
		for _, sz := range [][2]int{{1, 1}, {2, 3}, {24, 24}, {50, 7}} {
			c := newCtx(1, sz[0], sz[1], d.ID)
			fillWet(c)
			if sz[0] > 1 {
				c.F.Rock[0] = true
				c.F.Water[len(c.F.Water)-1] = mapgen.Beach
			}
			runAll(t, c)
			checkI2(t, c, d.Key)
			for _, s := range c.F.Stone {
				if s == geology.None {
					t.Fatalf("%s %v: material left a None cell", d.Key, sz)
				}
			}
		}
	}
}

// ---- benchmarks ----

func benchStep(b *testing.B, s mapgen.Step, prep func(*mapgen.Ctx)) {
	for b.Loop() {
		b.StopTimer()
		c := newCtx(7, 500, 500, biome.TemperateSwamp, geology.Granite, geology.Marble, geology.Slate)
		prep(c)
		b.StartTimer()
		if err := s.Run(c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFertility500(b *testing.B) { benchStep(b, Fertility{}, func(*mapgen.Ctx) {}) }
func BenchmarkMaterial500(b *testing.B)  { benchStep(b, Material{}, func(*mapgen.Ctx) {}) }
func BenchmarkTerrain500(b *testing.B) {
	benchStep(b, Terrain{}, func(c *mapgen.Ctx) {
		fillWet(c)
		banded(c)
		c.F.RockTau, c.F.RoughTau, c.F.GravelTau = 0.99, 0.97, 0.94
		_ = (Fertility{}).Run(c)
		_ = (Material{}).Run(c)
	})
}
