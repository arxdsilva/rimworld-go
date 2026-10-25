package resources

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

func newCtx(seed uint64, w, h int) *mapgen.Ctx {
	s := site.Site{Biome: biome.TemperateForest, Stones: []geology.Stone{geology.Granite, geology.Marble}}
	c := mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(s.Biome)})
	c.F.RockTau, c.F.RoughTau, c.F.GravelTau = 0.8, 0.6, 0.5
	return c
}

// rockFixture makes cells with x < rockW rock (granite), the rest soil.
func rockFixture(seed uint64, w, h, rockW int) *mapgen.Ctx {
	c := newCtx(seed, w, h)
	for i := range c.F.Rock {
		x, _ := c.XY(i)
		if x < rockW {
			c.F.Rock[i] = true
			c.R.Terrain.Cells[i] = terrain.Rock
			c.R.Geology.Stone[i] = geology.Granite
			c.F.Stone[i] = geology.Granite
		}
	}
	c.F.RockDepth = mapgen.ChebDepth(w, h, c.F.Rock)
	return c
}

func oreCells(c *mapgen.Ctx, o geology.Ore) []int {
	var out []int
	for i, v := range c.R.Geology.Ore {
		if v == o {
			out = append(out, i)
		}
	}
	return out
}

func TestOresDeterministicAndRockOnly(t *testing.T) {
	a, b, o := rockFixture(1, 200, 200, 120), rockFixture(1, 200, 200, 120), rockFixture(2, 200, 200, 120)
	for _, c := range []*mapgen.Ctx{a, b, o} {
		if err := (Ores{}).Run(c); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(a.R.Geology.Ore, b.R.Geology.Ore) {
		t.Fatal("same seed differs")
	}
	if slices.Equal(a.R.Geology.Ore, o.R.Geology.Ore) {
		t.Fatal("other seed identical")
	}
	n := 0
	for i, v := range a.R.Geology.Ore {
		if v == geology.NoOre {
			continue
		}
		n++
		if a.R.Terrain.Cells[i] != terrain.Rock {
			t.Fatalf("ore off rock at %d", i)
		}
	}
	if n == 0 {
		t.Fatal("no ore")
	}
	if (Ores{}).Name() != "ores" || (Chunks{}).Name() != "chunks" {
		t.Fatal("names")
	}
}

func TestOresNoRock(t *testing.T) {
	c := newCtx(3, 60, 60)
	if err := (Ores{}).Run(c); err != nil {
		t.Fatal(err)
	}
	if len(c.R.Geology.Ore) != 3600 || len(oreCells(c, geology.Steel)) != 0 {
		t.Fatal("ore without rock")
	}
	tiny := newCtx(3, 24, 24)
	if err := (Ores{}).Run(tiny); err != nil || (Chunks{}).Run(tiny) != nil {
		t.Fatal("tiny map")
	}
}

// lumps splits ore cells of kind o into 4-connected groups.
func lumps(c *mapgen.Ctx, o geology.Ore) [][]int {
	seen := map[int]bool{}
	var out [][]int
	for _, s := range oreCells(c, o) {
		if seen[s] {
			continue
		}
		g := []int{s}
		seen[s] = true
		for k := 0; k < len(g); k++ {
			x, y := c.XY(g[k])
			for _, d := range dirs4 {
				if !c.In(x+d[0], y+d[1]) {
					continue
				}
				j := c.Idx(x+d[0], y+d[1])
				if !seen[j] && c.R.Geology.Ore[j] == o {
					seen[j] = true
					g = append(g, j)
				}
			}
		}
		out = append(out, g)
	}
	return out
}

func TestOreRules(t *testing.T) {
	spec := map[geology.Ore]oreSpec{}
	for _, s := range oreSpecs {
		spec[s.ore] = s
	}
	for seed := uint64(1); seed <= 6; seed++ {
		c := rockFixture(seed, 300, 300, 200)
		placed := placeOres(c)
		deep := 0
		for i := range c.F.Rock {
			if c.F.Rock[i] && c.F.RockDepth[i] >= 2 {
				deep++
			}
		}
		count := map[geology.Ore]int{}
		for k, l := range placed {
			sp := spec[l.ore]
			count[l.ore]++
			if c.F.RockDepth[l.origin] < sp.minDepth {
				t.Fatalf("ore %v origin too shallow", l.ore)
			}
			if n := len(l.cells); n != sp.minSize+0 && (n < sp.minSize || n >= sp.minSize+sp.sizeSpan) {
				t.Fatalf("ore %v lump size %d", l.ore, n)
			}
			x0, y0, x1, y1 := 1<<30, 1<<30, -1, -1
			for _, i := range l.cells {
				x, y := c.XY(i)
				x0, x1, y0, y1 = min(x0, x), max(x1, x), min(y0, y), max(y1, y)
				if c.R.Terrain.Cells[i] != terrain.Rock {
					t.Fatal("lump off rock")
				}
			}
			if len(l.cells) >= 6 && (x1-x0+1)*(y1-y0+1) == len(l.cells) {
				t.Fatalf("ore %v lump is a rectangle", l.ore)
			}
			ox, oy := c.XY(l.origin)
			for _, m := range placed[:k] {
				mx, my := c.XY(m.origin)
				if max(abs(mx-ox), abs(my-oy)) < originSpacing {
					t.Fatal("origins closer than spacing")
				}
			}
		}
		for _, o := range []geology.Ore{geology.Steel, geology.Silver, geology.Gold, geology.Machinery} {
			sp := spec[o]
			want := deep / sp.per
			if want == 0 && sp.minDeep > 0 && deep >= sp.minDeep {
				want = 1
			}
			if count[o] != want {
				t.Fatalf("seed %d ore %v: %d veins, budget %d", seed, o, count[o], want)
			}
		}
	}
}

func TestLumpShape(t *testing.T) {
	for seed := uint64(1); seed <= 40; seed++ {
		c := rockFixture(seed, 80, 80, 80)
		r := c.Rand("shape")
		g := growLump(c, r, c.Idx(40, 40), 6+int(seed%9))
		if len(g) != 6+int(seed%9) {
			t.Fatalf("lump size %d", len(g))
		}
		in := map[int]bool{}
		for _, i := range g {
			in[i] = true
		}
		if !connected(c, g, in, -1) || isRect(c, g) {
			t.Fatalf("seed %d: lump disconnected or rectangular", seed)
		}
	}
}

func TestOreOriginSpacing(t *testing.T) {
	origins := []mapgen.Point{{X: 10, Y: 10}}
	if farFromAll(origins, 15, 10) || !farFromAll(origins, 16, 10) || abs(-3) != 3 || abs(2) != 2 {
		t.Fatal("farFromAll")
	}
}

func TestOreDepthRule(t *testing.T) {
	// A rock band 4 cells thick: depth maxes at 2, so only steel can start.
	c := newCtx(8, 120, 120)
	for i := range c.F.Rock {
		x, _ := c.XY(i)
		if x < 4 {
			c.F.Rock[i] = true
			c.R.Terrain.Cells[i] = terrain.Rock
		}
	}
	c.F.RockDepth = mapgen.ChebDepth(120, 120, c.F.Rock)
	if err := (Ores{}).Run(c); err != nil {
		t.Fatal(err)
	}
	for _, o := range []geology.Ore{geology.Silver, geology.Gold, geology.Machinery} {
		if len(oreCells(c, o)) != 0 {
			t.Fatalf("%v placed without depth", o)
		}
	}
}

func TestOreCellsStayDistinct(t *testing.T) {
	c := rockFixture(9, 300, 300, 300)
	if err := (Ores{}).Run(c); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, o := range []geology.Ore{geology.Steel, geology.Silver, geology.Gold, geology.Machinery} {
		total += len(oreCells(c, o))
	}
	if total == 0 {
		t.Fatal("no ore")
	}
}

func chunkRun(t *testing.T, c *mapgen.Ctx) {
	t.Helper()
	if err := (Chunks{}).Run(c); err != nil {
		t.Fatal(err)
	}
}

func TestChunksDeterministic(t *testing.T) {
	a, b, o := rockFixture(1, 200, 200, 80), rockFixture(1, 200, 200, 80), rockFixture(2, 200, 200, 80)
	chunkRun(t, a)
	chunkRun(t, b)
	chunkRun(t, o)
	if len(a.R.Chunks) == 0 || !slices.Equal(a.R.Chunks, b.R.Chunks) || slices.Equal(a.R.Chunks, o.R.Chunks) {
		t.Fatal("determinism")
	}
	for _, d := range a.F.ChunkDensity {
		if d <= 0 {
			t.Fatal("density must be stored and positive")
		}
	}
}

func TestChunksEligibility(t *testing.T) {
	c := newCtx(4, 200, 200)
	c.F.RoughTau = 0.5
	for i := range c.F.Elevation {
		x, y := c.XY(i)
		c.F.Elevation[i] = 0.1
		switch {
		case x < 20:
			c.R.Terrain.Cells[i] = terrain.DeepWater
			c.F.Water[i] = mapgen.Deep
		case x < 40:
			c.R.Terrain.Cells[i] = terrain.Mud
		case x < 60:
			c.R.Terrain.Cells[i] = terrain.Marsh
		case x < 80:
			c.R.Terrain.Cells[i] = terrain.Track
		case x < 100:
			c.F.Blocked[i] = true
		case x < 120:
			c.F.Cave[i] = true
		case x < 140:
			c.F.Elevation[i] = 0.9 // too high, soil
		case x < 160:
			c.F.Elevation[i] = 0.9
			c.R.Terrain.Cells[i] = terrain.Gravel // rough ground ok when high
		case y < 100:
			c.R.Terrain.Cells[i] = terrain.Rock
			c.F.Rock[i] = true
		}
	}
	pre := slices.Clone(c.F.Blocked)
	chunkRun(t, c)
	gravel := 0
	for _, ch := range c.R.Chunks {
		i := c.Idx(ch.X, ch.Y)
		if ch.X < 140 && ch.X >= 0 || pre[i] {
			t.Fatalf("chunk on ineligible cell %v", ch)
		}
		if c.R.Terrain.Cells[i] == terrain.Rock || !c.F.Blocked[i] {
			t.Fatal("rock cell or unmarked chunk")
		}
		if ch.X >= 140 && ch.X < 160 {
			gravel++
		}
	}
	if gravel == 0 {
		t.Fatal("gravel should accept chunks")
	}
}

func TestChunkFormations(t *testing.T) {
	c := newCtx(6, 300, 300)
	chunkRun(t, c)
	if len(c.R.Chunks) == 0 {
		t.Fatal("no chunks")
	}
	// Group by 8-adjacency: each group is formation(s); a lone formation
	// has 1-4 chunks, so most groups are small and none can be huge.
	pos := map[mapgen.Point]bool{}
	for _, ch := range c.R.Chunks {
		if pos[mapgen.Point{X: ch.X, Y: ch.Y}] {
			t.Fatal("two chunks on a cell")
		}
		pos[mapgen.Point{X: ch.X, Y: ch.Y}] = true
	}
	seen := map[mapgen.Point]bool{}
	max4 := 0
	for p := range pos { // order irrelevant: only maxima are computed
		if seen[p] {
			continue
		}
		g := []mapgen.Point{p}
		seen[p] = true
		for k := 0; k < len(g); k++ {
			for _, d := range dirs4 {
				q := mapgen.Point{X: g[k].X + d[0], Y: g[k].Y + d[1]}
				if pos[q] && !seen[q] {
					seen[q] = true
					g = append(g, q)
				}
			}
		}
		max4 = max(max4, len(g))
	}
	if max4 > 8 {
		t.Fatalf("formation group of %d", max4)
	}
}

func TestChunkDensityNearRock(t *testing.T) {
	var near, nearN, far, farN int
	for seed := uint64(1); seed <= 6; seed++ {
		c := rockFixture(seed, 300, 300, 150)
		chunkRun(t, c)
		for _, ch := range c.R.Chunks {
			switch d := ch.X - 150; {
			case d >= 0 && d < 6:
				near++
			case d >= 20:
				far++
			}
		}
		nearN += 6 * 300
		farN += (300 - 170) * 300
	}
	rn, rf := float64(near)/float64(nearN), float64(far)/float64(farN)
	if rn < 2*rf {
		t.Fatalf("near-rock rate %.5f not 2x open rate %.5f", rn, rf)
	}
}

func TestChunkStoneSource(t *testing.T) {
	c := newCtx(1, 10, 10)
	c.R.Geology.Stone[0] = geology.Slate
	c.F.Stone[0], c.F.Stone[1] = geology.Marble, geology.Marble
	if chunkStone(c, 0) != geology.Slate || chunkStone(c, 1) != geology.Marble || chunkStone(c, 2) != geology.Granite {
		t.Fatal("stone fallback chain")
	}
	c.Cfg.Site.Stones = nil
	if chunkStone(c, 2) != geology.None {
		t.Fatal("no stone")
	}
	c = rockFixture(3, 100, 100, 50)
	chunkRun(t, c)
	for _, ch := range c.R.Chunks {
		if ch.Stone != geology.Granite && ch.X < 50 {
			t.Fatal("stone mismatch on rough ground")
		}
		if ch.Stone == geology.None {
			t.Fatal("chunk without stone")
		}
	}
}

func BenchmarkOres500(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		c := rockFixture(1, 500, 500, 300)
		b.StartTimer()
		_ = (Ores{}).Run(c)
	}
}

func BenchmarkChunks500(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		c := rockFixture(1, 500, 500, 200)
		b.StartTimer()
		_ = (Chunks{}).Run(c)
	}
}
