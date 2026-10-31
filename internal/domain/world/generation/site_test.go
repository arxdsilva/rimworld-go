package generation

import (
	"fmt"
	"hash/fnv"
	"math"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// mapHash pins a generated map: terrain, then every plant.
func mapHash(res Result) uint64 {
	h := fnv.New64a()
	for _, c := range res.Terrain.Cells {
		h.Write([]byte{byte(c)})
	}
	for _, p := range res.Plants {
		fmt.Fprintf(h, "%d,%d,%d,%d;", p.X, p.Y, p.Species, p.Growth)
	}
	return h.Sum64()
}

// goldenLegacy is GenerateBiome at 96×80 before landing sites existed.
// Recompute only for a deliberate change to legacy generation.
//
// Re-pinned for PLAY-010 (boreal, cold bog, rainforest, desert, extreme
// desert): the biomes' Plants lists gained the wild ground plants (moss,
// bramble, agave, ...) and the rainforest's jungle trees and berries were
// thinned to make room. A field diff against the previous commit showed
// the terrain hash, start clearing and cave layer unchanged in every
// entry; only Plants changed (new species, and the old species' cells
// shifted with the extra rolls). Temperate and sea ice hold.
// Re-pinned (FOOD-2, 2026-10-30): plants spawn with a share of their
// grow time, and FOOD-2 moved every crop, tree and bush to the wiki grow
// days (trees about 30 days instead of 2), so the generated growth values
// change; terrain and plant placement are unchanged.
var goldenLegacy = map[string]uint64{
	"temperate_forest/1":     0x9f1e581aa0799aba,
	"temperate_forest/42":    0x863a4b3d37f62c54,
	"boreal_forest/1":        0xb008c16dce5f0701,
	"cold_bog/42":            0xebd63693187d33c2,
	"sea_ice/1":              0x9e4f0547fb0fc925,
	"tropical_rainforest/42": 0xbca3c8b58f3fa295,
	"desert/1":               0xce20b477c9fe040d,
	"extreme_desert/42":      0x0402a074568c490e,
}

func TestZeroSiteMatchesLegacy(t *testing.T) {
	t.Parallel()
	for _, d := range biome.All() {
		for _, seed := range []uint64{1, 42} {
			got := GenerateSite(seed, 96, 80, d, site.Site{})
			if !reflect.DeepEqual(got, GenerateBiome(seed, 96, 80, d)) {
				t.Fatalf("%s/%d: the zero site must be GenerateBiome", d.Key, seed)
			}
			key := fmt.Sprintf("%s/%d", d.Key, seed)
			if want, ok := goldenLegacy[key]; ok && mapHash(got) != want {
				t.Errorf("%s: legacy map hash %#x, want %#x", key, mapHash(got), want)
			}
		}
	}
	if !reflect.DeepEqual(GenerateSite(7, 48, 40, biome.DefOf(biome.TemperateForest), site.Site{}), Generate(7, 48, 40)) {
		t.Fatal("a zero temperate site is the original generator")
	}
}

// TestFeaturelessSiteKeepsLegacyGround: a site that only names its stones
// (default hills, no water, caves or road) keeps the legacy terrain and
// plants whenever the safety pass had nothing to carve.
//
// PLAY-010: the temperate forest's landing sites also grow brambles and
// dandelions, which the legacy generator (GenerateBiome's temperate map)
// never had. So the legacy equality is checked on the biome without its
// ground plants (exactly the pre-PLAY-010 rows), and the full biome keeps
// the legacy terrain and rolls the same plant stream.
func TestFeaturelessSiteKeepsLegacyGround(t *testing.T) {
	t.Parallel()
	s := site.Site{Stones: []geology.Stone{geology.Granite, geology.Slate}}
	for _, b := range []biome.ID{biome.TemperateForest, biome.AridShrubland, biome.BorealForest} {
		full := biome.DefOf(b)
		old := withoutGroundPlants(full)
		for seed := uint64(1); seed <= 6; seed++ {
			res, g := generateSite(seed, 96, 80, old, s)
			if g.carved > 0 {
				continue
			}
			if !reflect.DeepEqual(res, GenerateBiome(seed, 96, 80, old)) {
				t.Fatalf("%s/%d: featureless site differs from the legacy map", b, seed)
			}
			got, _ := generateSite(seed, 96, 80, full, s)
			if !reflect.DeepEqual(got.Terrain, res.Terrain) {
				t.Fatalf("%s/%d: ground plants changed the terrain", b, seed)
			}
			if want := rollPlants(seed, 96, 80, full); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s/%d: featureless site plants differ from the biome's roll", b, seed)
			}
		}
	}
}

// withoutGroundPlants is d with the PLAY-010 ground plants dropped.
func withoutGroundPlants(d biome.Def) biome.Def {
	var ps []biome.PlantDensity
	for _, pd := range d.Plants {
		if !plant.DefOf(pd.Species).Ground {
			ps = append(ps, pd)
		}
	}
	d.Plants = ps
	return d
}

// rollPlants is the default-hills map of b with b's plant list rolled on
// it, the legacy generator's terrain for a legacy biome.
func rollPlants(seed uint64, w, h int, b biome.Def) Result {
	r := randx.New(seed ^ mapSalt)
	res := newResult(w, h)
	baseTerrain(r, &res, b, hillsTable[site.HillsDefault], nil)
	spawnPlants(r, &res, b)
	return res
}

func count(m terrain.Map, keep func(terrain.Type) bool) int {
	n := 0
	for _, c := range m.Cells {
		if keep(c) {
			n++
		}
	}
	return n
}

func isType(ts ...terrain.Type) func(terrain.Type) bool {
	return func(t terrain.Type) bool {
		for _, x := range ts {
			if t == x {
				return true
			}
		}
		return false
	}
}

// waterComponent is the 8-connected water reached from (x, y).
func waterComponent(m terrain.Map, x, y int) []bool {
	seen := make([]bool, m.W*m.H)
	if !terrain.IsWater(m.At(x, y)) {
		return seen
	}
	stack := []int{y*m.W + x}
	seen[y*m.W+x] = true
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		cx, cy := c%m.W, c/m.W
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := cx+dx, cy+dy
				if nx < 0 || ny < 0 || nx >= m.W || ny >= m.H {
					continue
				}
				if n := ny*m.W + nx; !seen[n] && terrain.IsWater(m.Cells[n]) {
					seen[n] = true
					stack = append(stack, n)
				}
			}
		}
	}
	return seen
}

func cell(p [2]float64) (int, int) { return int(math.Round(p[0])), int(math.Round(p[1])) }

func TestRiverEdgeToEdgeAllSizes(t *testing.T) {
	t.Parallel()
	b := biome.DefOf(biome.TemperateForest)
	for _, size := range []site.RiverSize{site.Creek, site.River, site.LargeRiver} {
		for _, io := range [][2]site.Dir8{{0, 4}, {2, 6}, {1, 5}, {0, 2}, {7, 3}, {4, 4}, {1, 1}} {
			for seed := uint64(1); seed <= 4; seed++ {
				s := site.Site{Hills: site.SmallHills, River: site.RiverPath{Size: size, In: io[0], Out: io[1]}}
				res, g := generateSite(seed, 160, 140, b, s)
				m := res.Terrain
				name := fmt.Sprintf("%s %v seed %d", size.Label(), io, seed)
				ix, iy := cell(g.river.in)
				ox, oy := cell(g.river.out)
				onEdge := func(x, y int) bool { return x == 0 || y == 0 || x == m.W-1 || y == m.H-1 }
				if !onEdge(ix, iy) || !onEdge(ox, oy) {
					t.Fatalf("%s: river runs %v..%v, not edge to edge", name, g.river.in, g.river.out)
				}
				comp := waterComponent(m, ix, iy)
				if !comp[oy*m.W+ox] {
					t.Fatalf("%s: the water entering at %d,%d never reaches %d,%d", name, ix, iy, ox, oy)
				}
				for _, p := range g.river.path {
					if x, y := cell(p); !comp[y*m.W+x] {
						t.Fatalf("%s: the river's course is dry at %d,%d (%s)", name, x, y, m.At(x, y))
					}
				}
				deep := count(m, isType(terrain.DeepWater))
				if size == site.Creek && deep != 0 {
					t.Fatalf("%s: a creek is all shallow, got %d deep tiles", name, deep)
				}
				if size != site.Creek && deep == 0 {
					t.Fatalf("%s: no deep core", name)
				}
				if count(m, isType(terrain.Mud)) == 0 {
					t.Fatalf("%s: no mud banks", name)
				}
			}
		}
	}
}

// riverWidthAt measures the water run across a vertical river at row y.
func TestRiverWidths(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		size       site.RiverSize
		width, dep int
	}{{site.Creek, 3, 0}, {site.River, 9, 3}, {site.LargeRiver, 15, 7}} {
		s := site.Site{Hills: site.Flat, River: site.RiverPath{Size: c.size, In: 0, Out: 4}}
		res, g := generateSite(3, 200, 200, biome.DefOf(biome.TemperateForest), s)
		widths, deeps := map[int]int{}, map[int]int{}
		for k := 10; k < len(g.river.path)-10; k += 7 {
			x, y := cell(g.river.path[k])
			w, d := 0, 0
			for dx := -12; dx <= 12; dx++ {
				switch res.Terrain.At(x+dx, y) {
				case terrain.ShallowWater:
					w++
				case terrain.DeepWater:
					w++
					d++
				}
			}
			widths[w]++
			deeps[d]++
		}
		// the most common run across a mostly north-south river is about its width
		best, bestN := 0, 0
		for w, n := range widths {
			if n > bestN {
				best, bestN = w, n
			}
		}
		if best < c.width || best > c.width+4 {
			t.Errorf("%s: typical width %d, want ~%d (%v)", c.size.Label(), best, c.width, widths)
		}
		if c.dep == 0 && len(deeps) != 1 {
			t.Errorf("%s: deep water in a creek %v", c.size.Label(), deeps)
		}
	}
}

func TestNoWaterAtStart(t *testing.T) {
	t.Parallel()
	for _, s := range siteCombos() {
		for seed := uint64(1); seed <= 6; seed++ {
			res := GenerateSite(seed, 120, 100, biome.DefOf(biome.TemperateSwamp), s)
			cx, cy := 60, 50
			for y := cy - WaterGuard; y <= cy+WaterGuard; y++ {
				for x := cx - WaterGuard; x <= cx+WaterGuard; x++ {
					if terrain.IsWater(res.Terrain.At(x, y)) {
						t.Fatalf("%+v seed %d: water at %d,%d near the start", s, seed, x, y)
					}
				}
			}
			for y := res.ClearY0; y <= res.ClearY1; y++ {
				for x := res.ClearX0; x <= res.ClearX1; x++ {
					if !res.Terrain.At(x, y).Walkable() {
						t.Fatalf("%+v seed %d: start blocked at %d,%d", s, seed, x, y)
					}
				}
			}
		}
	}
}

// siteCombos are the hills × water combinations the safety tests sweep.
func siteCombos() []site.Site {
	var out []site.Site
	rivers := []site.RiverPath{{}, {Size: site.Creek, In: 0, Out: 4}, {Size: site.River, In: 2, Out: 7},
		{Size: site.LargeRiver, In: 6, Out: 2}, {Size: site.LargeRiver, In: 4, Out: 4}}
	coasts := []site.Dir8Set{0, 1 << 0, 1<<2 | 1<<4, 1 << 5}
	for hi, hills := range []site.Hills{site.Flat, site.SmallHills, site.LargeHills, site.Mountainous} {
		for ri, rv := range rivers {
			c := coasts[(hi+ri)%len(coasts)]
			out = append(out, site.Site{Hills: hills, River: rv, Coast: c, Lakes: uint8((hi + ri) % 4),
				Caves: ri%2 == 1, Road: site.Dir8Set(1 << ((hi + ri) % 8))})
		}
	}
	return out
}

// reachShare floods 4-connected from the centre over walkable tiles: the
// permille of walkable land reached, and whether a map edge was reached.
func reachShare(m terrain.Map) (int, bool) {
	cx, cy := m.W/2, m.H/2
	seen := make([]bool, m.W*m.H)
	stack := []int{cy*m.W + cx}
	seen[cy*m.W+cx] = true
	got, edge := 0, false
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := c%m.W, c/m.W
		if land(m.Cells[c]) {
			got++
		}
		edge = edge || x == 0 || y == 0 || x == m.W-1 || y == m.H-1
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= m.W || ny >= m.H {
				continue
			}
			if n := ny*m.W + nx; !seen[n] && m.Cells[n].Walkable() {
				seen[n] = true
				stack = append(stack, n)
			}
		}
	}
	total := count(m, land)
	return got * 1000 / max(1, total), edge
}

func TestReachableSoftGround70(t *testing.T) {
	t.Parallel()
	combos := siteCombos()
	biomes := []biome.ID{biome.TemperateForest, biome.TemperateSwamp, biome.Desert, biome.BorealForest}
	worst := 1000
	for seed := uint64(1); seed <= 50; seed++ {
		s := combos[int(seed)%len(combos)]
		s2 := combos[(int(seed)*7+3)%len(combos)]
		for _, ss := range []site.Site{s, s2} {
			b := biome.DefOf(biomes[int(seed)%len(biomes)])
			res := GenerateSite(seed, 120, 100, b, ss)
			share, edge := reachShare(res.Terrain)
			if !edge || share < ReachShare {
				t.Fatalf("seed %d %s %+v: reached %d permille of the land (edge %v)", seed, b.Key, ss, share, edge)
			}
			worst = min(worst, share)
		}
	}
	t.Logf("worst reach %d permille", worst)
}

// baseOnly is a site's terrain before any pass, as a siteGen.
func baseOnly(seed uint64, w, h int, b biome.Def, s site.Site) *siteGen {
	r := randx.New(seed ^ mapSalt)
	res := newResult(w, h)
	var ridge func(x, y int) float64
	if hillsOf(s.Hills).ridge {
		ridge = ridgeField(seed, w, h)
	}
	baseTerrain(r, &res, b, hillsOf(s.Hills), ridge)
	return &siteGen{seed: seed, w: w, h: h, cx: w / 2, cy: h / 2, b: b, s: s, m: res.Terrain, fresh: make([]bool, w*h)}
}

func TestCavesOnlyInRock(t *testing.T) {
	t.Parallel()
	b := biome.DefOf(biome.TemperateForest)
	s := site.Site{Hills: site.Mountainous, Caves: true}
	for seed := uint64(1); seed <= 8; seed++ {
		g := baseOnly(seed, 140, 120, b, s)
		before := slices.Clone(g.m.Cells)
		g.caves()
		carved := 0
		for i, after := range g.m.Cells {
			x, y := i%140, i/140
			if before[i] == after {
				if g.cave != nil && g.cave[i] {
					t.Fatalf("seed %d: %d,%d marked cave but not carved", seed, x, y)
				}
				continue
			}
			if before[i] != terrain.Rock || after != terrain.Stone || !g.cave[i] {
				t.Fatalf("seed %d: caves changed %s to %s at %d,%d", seed, before[i], after, x, y)
			}
			if g.startDist(x, y) <= caveGuard {
				t.Fatalf("seed %d: cave carved near the start at %d,%d", seed, x, y)
			}
			carved++
		}
		if carved < 40 {
			t.Fatalf("seed %d: only %d cave tiles on a mountain", seed, carved)
		}
		// the full map keeps the tunnels as rough stone, flagged for a roof
		res := GenerateSite(seed, 140, 120, b, s)
		for i, c := range res.Cave {
			if c && (before[i] != terrain.Rock || res.Terrain.Cells[i] != terrain.Stone) {
				t.Fatalf("seed %d: cave flag on %s (was %s)", seed, res.Terrain.Cells[i], before[i])
			}
		}
		if GenerateSite(seed, 140, 120, b, site.Site{Hills: site.Mountainous}).Cave != nil {
			t.Fatalf("seed %d: cave layer without caves", seed)
		}
	}
}

func TestRockShareRisesWithHills(t *testing.T) {
	t.Parallel()
	b := biome.DefOf(biome.TemperateForest)
	levels := []site.Hills{site.Flat, site.SmallHills, site.LargeHills, site.Mountainous}
	prev := -1
	for _, hl := range levels {
		rock := 0
		for seed := uint64(1); seed <= 6; seed++ {
			res := GenerateSite(seed, 150, 150, b, site.Site{Hills: hl})
			rock += count(res.Terrain, isType(terrain.Rock))
		}
		if rock <= prev {
			t.Fatalf("%s: %d rock tiles, not more than the flatter level's %d", hl.Label(), rock, prev)
		}
		prev = rock
	}
}

func TestMountainRidgeAvoidsStart(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 20; seed++ {
		f := ridgeField(seed, 200, 160)
		for y := 0; y < 160; y++ {
			for x := 0; x < 200; x++ {
				if f(x, y) > 0 && math.Hypot(float64(x-100), float64(y-80)) < ridgeGap {
					t.Fatalf("seed %d: ridge at %d,%d inside the start gap", seed, x, y)
				}
			}
		}
	}
}

func TestCoastDepthBands(t *testing.T) {
	t.Parallel()
	b := biome.DefOf(biome.TemperateForest)
	const w, h = 160, 120
	for seed := uint64(1); seed <= 6; seed++ {
		for _, edge := range []site.Dir8{0, 2, 4, 6} {
			res := GenerateSite(seed, w, h, b, site.Site{Hills: site.Flat, Coast: 1 << edge})
			m := res.Terrain
			lines, size := w, float64(h)
			if edge == 2 || edge == 6 {
				lines, size = h, float64(w)
			}
			for k := 0; k < lines; k++ {
				at := func(d int) terrain.Type { // d tiles in from the edge on line k
					switch edge {
					case 0:
						return m.At(k, d)
					case 4:
						return m.At(k, h-1-d)
					case 2:
						return m.At(w-1-d, k)
					}
					return m.At(d, k)
				}
				d := 0
				for ; at(d) == terrain.DeepWater; d++ {
				}
				deep := d
				if lo, hi := coastDeepFrac*size-coastJitter, coastDeepFrac*size+coastJitter+1; float64(deep) < lo || float64(deep) > hi {
					t.Fatalf("seed %d edge %d line %d: %d deep tiles, want %.0f..%.0f", seed, edge, k, deep, lo, hi)
				}
				for ; at(d) == terrain.ShallowWater; d++ {
				}
				if d-deep != coastShallow {
					t.Fatalf("seed %d edge %d line %d: %d shallow tiles, want %d", seed, edge, k, d-deep, coastShallow)
				}
				sh := d
				for ; at(d) == terrain.Sand; d++ {
				}
				if bw := d - sh; bw < 3 || bw > 5 {
					t.Fatalf("seed %d edge %d line %d: beach %d tiles, want 3..5", seed, edge, k, bw)
				}
				for ; d < int(size); d++ {
					if terrain.IsWater(at(d)) {
						t.Fatalf("seed %d edge %d line %d: water inland at %d", seed, edge, k, d)
					}
				}
			}
		}
	}
}

func TestLakeRings(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		b    biome.ID
		ring terrain.Type
	}{{biome.TemperateSwamp, terrain.Marsh}, {biome.Desert, terrain.Mud}, {biome.TemperateForest, terrain.Marsh}} {
		for seed := uint64(1); seed <= 6; seed++ {
			res, g := generateSite(seed, 140, 120, biome.DefOf(c.b), site.Site{Hills: site.Flat, Lakes: 3})
			m := res.Terrain
			if len(g.lakes) == 0 || count(m, terrain.IsWater) == 0 {
				t.Fatalf("%s seed %d: no lake", c.b, seed)
			}
			if count(m, isType(terrain.DeepWater)) == 0 {
				t.Fatalf("%s seed %d: lakes without deep water", c.b, seed)
			}
			for i, tt := range m.Cells {
				if !terrain.IsWater(tt) {
					continue
				}
				x, y := i%m.W, i/m.W
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < 0 || ny < 0 || nx >= m.W || ny >= m.H {
						continue
					}
					n := m.At(nx, ny)
					if !terrain.IsWater(n) && n != c.ring && n != terrain.Rock && n != terrain.Stone {
						t.Fatalf("%s seed %d: lake shore at %d,%d is %s, want %s", c.b, seed, nx, ny, n, c.ring)
					}
				}
			}
		}
	}
}

func TestFordsEveryInterval(t *testing.T) {
	t.Parallel()
	b := biome.DefOf(biome.TemperateForest)
	for _, size := range []site.RiverSize{site.River, site.LargeRiver} {
		for seed := uint64(1); seed <= 6; seed++ {
			s := site.Site{Hills: site.Flat, River: site.RiverPath{Size: size, In: 6, Out: 2}}
			res, g := generateSite(seed, 240, 200, b, s)
			rv := g.river
			total := rv.arc[len(rv.arc)-1]
			if len(rv.fords) < int(total)/(fordEvery+fordJitter) {
				t.Fatalf("%s seed %d: %d fords over %.0f tiles", size.Label(), seed, len(rv.fords), total)
			}
			prev := 0.0
			for _, f := range append(append([]float64(nil), rv.fords...), total) {
				if gap := f - prev; gap > fordEvery+fordJitter {
					t.Fatalf("%s seed %d: %.0f tiles without a ford", size.Label(), seed, gap)
				}
				prev = f
			}
			for k, f := range rv.fords {
				if k > 0 && f-rv.fords[k-1] < fordEvery-fordJitter {
					t.Fatalf("%s seed %d: fords %.0f and %.0f too close", size.Label(), seed, rv.fords[k-1], f)
				}
				for i, a := range rv.arc { // the centre line at the ford wades
					if math.Abs(a-f) <= 1 {
						if x, y := cell(rv.path[i]); res.Terrain.At(x, y) != terrain.ShallowWater {
							t.Fatalf("%s seed %d: ford at %.0f is %s at %d,%d", size.Label(), seed, f, res.Terrain.At(x, y), x, y)
						}
					}
				}
			}
			// away from fords the centre line is deep
			deep := 0
			for i, a := range rv.arc {
				near := false
				for _, f := range rv.fords {
					near = near || math.Abs(a-f) <= 3
				}
				if x, y := cell(rv.path[i]); !near && res.Terrain.At(x, y) == terrain.DeepWater {
					deep++
				}
			}
			if deep*2 < len(rv.arc) {
				t.Fatalf("%s seed %d: only %d of %d centre points deep", size.Label(), seed, deep, len(rv.arc))
			}
		}
	}
}

func TestWetGroundBySiteWater(t *testing.T) {
	t.Parallel()
	s := site.Site{Hills: site.Flat, River: site.RiverPath{Size: site.River, In: 0, Out: 4}}
	plain := site.Site{Hills: site.Flat}
	for _, c := range []struct {
		b       biome.ID
		from    func(terrain.Type) bool
		to      terrain.Type
		reach   int
		without terrain.Type
	}{
		{biome.TemperateSwamp, soft, terrain.Marsh, 4, 0},
		{biome.TemperateForest, isType(terrain.Soil), terrain.RichSoil, 3, terrain.Marsh},
	} {
		res, g := generateSite(5, 160, 140, biome.DefOf(c.b), s)
		base, _ := generateSite(5, 160, 140, biome.DefOf(c.b), plain)
		var fresh [][2]int
		for i, f := range g.fresh {
			if f {
				fresh = append(fresh, [2]int{i % 160, i / 160})
			}
		}
		changed := 0
		for i, tt := range res.Terrain.Cells {
			if !c.from(base.Terrain.Cells[i]) || tt != c.to {
				continue
			}
			x, y := i%160, i/160
			near := 1 << 30
			for _, f := range fresh {
				near = min(near, max(abs(x-f[0]), abs(y-f[1])))
			}
			if near > c.reach {
				t.Fatalf("%s: %s turned %s %d tiles from water", c.b, base.Terrain.Cells[i], tt, near)
			}
			changed++
		}
		if changed < 20 {
			t.Fatalf("%s: only %d tiles turned %s by the water", c.b, changed, c.to)
		}
		if c.without != 0 && count(res.Terrain, isType(c.without)) != 0 {
			t.Fatalf("%s: no %s in a biome without it", c.b, c.without)
		}
	}
}

func TestRoadTrackReachesStart(t *testing.T) {
	t.Parallel()
	res, g := generateSite(4, 120, 100, biome.DefOf(biome.AridShrubland), site.Site{Hills: site.Flat, Road: 1<<0 | 1<<3})
	m := res.Terrain
	if m.At(60, 0) != terrain.Track && m.At(61, 0) != terrain.Track {
		t.Fatal("the north road starts at the edge")
	}
	if m.At(119, 99) != terrain.Track {
		t.Fatal("the south-east road starts in the corner")
	}
	if m.At(60, 50-StartRadius-1) != terrain.Track || m.At(60, 50-StartRadius+1) == terrain.Track {
		t.Fatal("the road runs to the start and stops at StartRadius")
	}
	for i, c := range m.Cells {
		if c == terrain.Track && g.startDist(i%m.W, i/m.W) < StartRadius {
			t.Fatalf("track inside the start at %d", i)
		}
	}
}

func TestSitePlantsOffWaterAndDeterministic(t *testing.T) {
	t.Parallel()
	s := site.Site{Hills: site.LargeHills, River: site.RiverPath{Size: site.River, In: 1, Out: 5}, Lakes: 2,
		Coast: 1 << 6, Caves: true, Road: 1 << 4, Stones: []geology.Stone{geology.Marble, geology.Granite}}
	for _, b := range []biome.ID{biome.TemperateForest, biome.ColdBog, biome.TropicalSwamp} {
		a := GenerateSite(9, 150, 130, biome.DefOf(b), s)
		if !reflect.DeepEqual(a, GenerateSite(9, 150, 130, biome.DefOf(b), s)) {
			t.Fatalf("%s: same seed, different site map", b)
		}
		reeds := false
		for _, p := range a.Plants {
			tt := a.Terrain.At(p.X, p.Y)
			if terrain.IsWater(tt) || tt.Fertility() == 0 || tt == terrain.Track {
				t.Fatalf("%s: %s on %s", b, p.Species, tt)
			}
			reeds = reeds || tt == terrain.Marsh && p.Species.String() == "reeds"
		}
		if b == biome.ColdBog && !reeds {
			t.Fatal("cold bog grows reeds on its marsh")
		}
	}
}

func TestSmallAndOddMapsDoNotPanic(t *testing.T) {
	t.Parallel()
	all := site.Site{Hills: site.Mountainous, River: site.RiverPath{Size: site.LargeRiver, In: 3, Out: 3}, Lakes: 3,
		Coast: 0xff, Caves: true, Road: 0xff, Stones: []geology.Stone{geology.Slate}}
	for _, wh := range [][2]int{{0, 0}, {1, 1}, {12, 9}, {48, 40}, {40, 160}} {
		_ = GenerateSite(3, wh[0], wh[1], biome.DefOf(biome.TemperateForest), all)
		_ = GenerateSite(3, wh[0], wh[1], biome.DefOf(biome.SeaIce), site.Site{Hills: 99, River: site.RiverPath{Size: 9}})
	}
}

func BenchmarkGenerateSite500(b *testing.B) {
	s := site.Site{Hills: site.LargeHills, River: site.RiverPath{Size: site.LargeRiver, In: 0, Out: 3}, Lakes: 3,
		Coast: 1 << 6, Caves: true, Road: 1 << 2, Stones: []geology.Stone{geology.Granite, geology.Limestone}}
	d := biome.DefOf(biome.TemperateSwamp)
	for i := 0; i < b.N; i++ {
		res := GenerateSite(uint64(i+1), 500, 500, d, s)
		_ = geology.GenerateWith(uint64(i+1), res.Terrain, s.Stones)
	}
}
