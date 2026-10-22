package geology_test

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/generation"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// handMade is a 40×40 rock mass with an open soil clearing, a strip of
// rough stone ground, a smoothed floor, a smoothed wall and a sand patch.
func handMade() terrain.Map {
	m := terrain.NewMap(40, 40)
	for i := range m.Cells {
		m.Cells[i] = terrain.Rock
	}
	for y := 4; y < 10; y++ {
		for x := 4; x < 12; x++ {
			m.Set(x, y, terrain.Soil)
		}
	}
	for x := 0; x < 40; x++ {
		m.Set(x, 20, terrain.Stone)
	}
	m.Set(5, 5, terrain.StoneSmooth)
	m.Set(30, 30, terrain.RockSmooth)
	m.Set(6, 6, terrain.Sand)
	return m
}

// maps is the terrain under test: generated ones and the hand-made one.
func maps() map[string]terrain.Map {
	return map[string]terrain.Map{
		"generated 1":  generation.Generate(1, 80, 60).Terrain,
		"generated 77": generation.Generate(77, 80, 60).Terrain,
		"hand-made":    handMade(),
	}
}

func TestGenerateDeterministic(t *testing.T) {
	t.Parallel()
	for name, tm := range maps() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, b := geology.Generate(5, tm), geology.Generate(5, tm)
			if a.W != tm.W || a.H != tm.H {
				t.Fatalf("size %dx%d, want %dx%d", a.W, a.H, tm.W, tm.H)
			}
			if !slices.Equal(a.Stone, b.Stone) || !slices.Equal(a.Ore, b.Ore) {
				t.Fatal("same seed produced different geology")
			}
			differs := false
			for seed := uint64(6); seed < 12 && !differs; seed++ {
				c := geology.Generate(seed, tm)
				differs = !slices.Equal(a.Stone, c.Stone) || !slices.Equal(a.Ore, c.Ore)
			}
			if !differs {
				t.Fatal("different seeds never produced different geology")
			}
		})
	}
}

func TestStonesOnlyOnStoneAndRock(t *testing.T) {
	t.Parallel()
	for name, tm := range maps() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := geology.Generate(3, tm)
			for y := 0; y < tm.H; y++ {
				for x := 0; x < tm.W; x++ {
					tt, s := tm.At(x, y), g.At(x, y)
					natural := tt == terrain.Rock || tt == terrain.RockSmooth || tt == terrain.Stone || tt == terrain.StoneSmooth
					if natural != (s != geology.None) {
						t.Fatalf("(%d,%d) terrain %v has stone %v", x, y, tt, s)
					}
				}
			}
		})
	}
}

func TestOreOnlyInRock(t *testing.T) {
	t.Parallel()
	for name, tm := range maps() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			found := false
			for seed := uint64(0); seed < 10; seed++ {
				g := geology.Generate(seed, tm)
				for y := 0; y < tm.H; y++ {
					for x := 0; x < tm.W; x++ {
						o := g.OreAt(x, y)
						if o == geology.NoOre {
							continue
						}
						found = true
						if tm.At(x, y) != terrain.Rock {
							t.Fatalf("seed %d: ore %v on %v at (%d,%d)", seed, o, tm.At(x, y), x, y)
						}
					}
				}
			}
			if name == "hand-made" && !found {
				t.Fatal("a 40×40 rock mass never got any ore")
			}
		})
	}
}

func TestSteelVeinOnLargeRockMass(t *testing.T) {
	t.Parallel()
	tm := handMade()
	if deep := len(geology.DeepCells(tm, geology.Depths(tm))); deep < 60 {
		t.Fatalf("fixture has only %d deep cells", deep)
	}
	for seed := uint64(0); seed < 20; seed++ {
		g := geology.Generate(seed, tm)
		if !slices.Contains(g.Ore, geology.Steel) {
			t.Fatalf("seed %d: no steel on a map with ≥60 deep cells", seed)
		}
	}
}

func TestVeinSizesWithinBounds(t *testing.T) {
	t.Parallel()
	tm := handMade()
	deep := len(geology.DeepCells(tm, geology.Depths(tm)))
	for seed := uint64(0); seed < 20; seed++ {
		g := geology.Generate(seed, tm)
		// Veins of one ore may touch, so a component is at most every
		// vein of that ore at its largest.
		for _, comp := range veins(g) {
			if hi := geology.VeinMax(comp.ore); len(comp.cells) > hi*geology.VeinCount(deep, comp.ore) {
				t.Fatalf("seed %d: %v component of %d cells exceeds every vein's max", seed, comp.ore, len(comp.cells))
			}
		}
	}
}

type vein struct {
	ore   geology.Ore
	cells [][2]int
}

// veins splits the ore cells into 4-connected same-ore components.
func veins(g geology.Map) []vein {
	seen := make([]bool, g.W*g.H)
	var out []vein
	for i, o := range g.Ore {
		if o == geology.NoOre || seen[i] {
			continue
		}
		v := vein{ore: o}
		stack := []int{i}
		seen[i] = true
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := c%g.W, c/g.W
			v.cells = append(v.cells, [2]int{x, y})
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if g.OreAt(nx, ny) != o {
					continue
				}
				if n := ny*g.W + nx; !seen[n] {
					seen[n] = true
					stack = append(stack, n)
				}
			}
		}
		out = append(out, v)
	}
	return out
}

func TestVeinSeedsDeep(t *testing.T) {
	t.Parallel()
	for name, tm := range maps() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := geology.Depths(tm)
			for seed := uint64(0); seed < 10; seed++ {
				for _, v := range veins(geology.Generate(seed, tm)) {
					deep := false
					for _, c := range v.cells {
						deep = deep || d[c[1]*tm.W+c[0]] >= 2
					}
					if !deep {
						t.Fatalf("seed %d: %v vein %v has no cell at depth ≥ 2", seed, v.ore, v.cells)
					}
				}
			}
		})
	}
}

func TestDepthIsChebyshevToOpenGround(t *testing.T) {
	t.Parallel()
	// 7×5 rock with one open cell at (1, 2); edges are rock, not open.
	m := terrain.NewMap(7, 5)
	for i := range m.Cells {
		m.Cells[i] = terrain.Rock
	}
	m.Set(1, 2, terrain.Soil)
	m.Set(6, 0, terrain.RockSmooth) // smoothed rock is still rock
	d := geology.Depths(m)
	tests := []struct{ x, y, want int }{
		{1, 2, 0}, {0, 2, 1}, {2, 3, 1}, {0, 0, 2}, {3, 2, 2}, {4, 4, 3}, {6, 0, 5}, {6, 4, 5},
	}
	for _, tc := range tests {
		if got := d[tc.y*m.W+tc.x]; got != tc.want {
			t.Errorf("depth(%d,%d) = %d, want %d", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestDepthAllRockIsDeep(t *testing.T) {
	t.Parallel()
	m := terrain.NewMap(4, 4)
	for i := range m.Cells {
		m.Cells[i] = terrain.Rock
	}
	for i, v := range geology.Depths(m) {
		if v < 2 {
			t.Fatalf("cell %d depth %d on a map with no open ground", i, v)
		}
	}
}

func TestTwoOrThreeStonesPerMap(t *testing.T) {
	t.Parallel()
	counts := map[int]bool{}
	for seed := uint64(0); seed < 64; seed++ {
		picked := geology.PickStones(randx.New(seed))
		if n := len(picked); n < 2 || n > 3 {
			t.Fatalf("seed %d picked %d stones", seed, n)
		}
		seen := map[geology.Stone]bool{}
		for _, s := range picked {
			if s == geology.None || seen[s] {
				t.Fatalf("seed %d picked %v (invalid or repeated)", seed, picked)
			}
			seen[s] = true
		}
		counts[len(picked)] = true
	}
	if !counts[2] || !counts[3] {
		t.Fatalf("stone counts seen %v, want both 2 and 3", counts)
	}
	for name, tm := range maps() {
		for seed := uint64(0); seed < 10; seed++ {
			distinct := map[geology.Stone]bool{}
			for _, s := range geology.Generate(seed, tm).Stone {
				if s != geology.None {
					distinct[s] = true
				}
			}
			if len(distinct) < 1 || len(distinct) > 3 {
				t.Fatalf("%s seed %d: %d distinct stones", name, seed, len(distinct))
			}
		}
	}
}

func TestGenerateEmptyAndOpenMaps(t *testing.T) {
	t.Parallel()
	if g := geology.Generate(1, terrain.NewMap(0, 0)); g.W != 0 || len(g.Stone) != 0 {
		t.Fatalf("empty map: %+v", g)
	}
	g := geology.Generate(1, terrain.NewMap(10, 10)) // all soil
	for i := range g.Stone {
		if g.Stone[i] != geology.None || g.Ore[i] != geology.NoOre {
			t.Fatalf("all-soil map got geology at %d", i)
		}
	}
}

func TestMapAccessorsOOB(t *testing.T) {
	t.Parallel()
	m := geology.NewMap(3, 2)
	m.Set(2, 1, geology.Marble)
	m.SetOre(2, 1, geology.Gold)
	tests := []struct {
		name  string
		x, y  int
		stone geology.Stone
		ore   geology.Ore
	}{
		{"in bounds", 2, 1, geology.Marble, geology.Gold},
		{"empty cell", 0, 0, geology.None, geology.NoOre},
		{"left", -1, 0, geology.None, geology.NoOre},
		{"top", 0, -1, geology.None, geology.NoOre},
		{"right", 3, 0, geology.None, geology.NoOre},
		{"bottom", 0, 2, geology.None, geology.NoOre},
	}
	for _, tc := range tests {
		if s, o := m.At(tc.x, tc.y), m.OreAt(tc.x, tc.y); s != tc.stone || o != tc.ore {
			t.Errorf("%s: got %v/%v, want %v/%v", tc.name, s, o, tc.stone, tc.ore)
		}
	}
	m.Set(-1, 0, geology.Slate)
	m.Set(3, 1, geology.Slate)
	m.SetOre(0, 5, geology.Steel)
	for i := range m.Stone {
		if i != 5 && (m.Stone[i] != geology.None || m.Ore[i] != geology.NoOre) {
			t.Fatalf("out-of-bounds write landed at %d", i)
		}
	}
	var zero geology.Map
	zero.Set(0, 0, geology.Granite)
	zero.SetOre(0, 0, geology.Steel)
	if zero.At(0, 0) != geology.None || zero.OreAt(0, 0) != geology.NoOre {
		t.Fatal("zero map must read empty")
	}
	legacy := geology.Map{W: 2, H: 1, Stone: make([]geology.Stone, 2)} // a save without ore
	legacy.SetOre(1, 0, geology.Steel)
	if legacy.OreAt(1, 0) != geology.NoOre {
		t.Fatal("a map without an ore layer reads NoOre")
	}
}

func TestParseStoneRoundTrip(t *testing.T) {
	t.Parallel()
	if got := geology.Stones(); !slices.Equal(got, []geology.Stone{geology.Sandstone, geology.Granite, geology.Limestone, geology.Slate, geology.Marble}) {
		t.Fatalf("Stones() = %v", got)
	}
	for _, s := range geology.Stones() {
		got, ok := geology.ParseStone(s.String())
		if !ok || got != s {
			t.Errorf("ParseStone(%q) = %v, %v", s.String(), got, ok)
		}
		if s.Label() == "" {
			t.Errorf("%v has no label", s)
		}
	}
	for _, bad := range []string{"none", "", "Granite", "obsidian"} {
		if _, ok := geology.ParseStone(bad); ok {
			t.Errorf("ParseStone(%q) accepted", bad)
		}
	}
	if geology.None.String() != "none" || geology.Stone(200).String() != "none" || geology.Stone(200).Label() != "" {
		t.Fatal("out-of-range stone names")
	}
	if geology.Granite.Label() != "Granite" {
		t.Fatalf("Granite label = %q", geology.Granite.Label())
	}
	wantOre := []struct {
		o           geology.Ore
		name, label string
	}{
		{geology.Steel, "steel", "Compacted steel"},
		{geology.Silver, "silver", "Silver ore"},
		{geology.Gold, "gold", "Gold ore"},
	}
	if !slices.Equal(geology.Ores(), []geology.Ore{geology.Steel, geology.Silver, geology.Gold}) {
		t.Fatalf("Ores() = %v", geology.Ores())
	}
	for _, w := range wantOre {
		if w.o.String() != w.name || w.o.Label() != w.label {
			t.Errorf("%d: %q/%q", w.o, w.o.String(), w.o.Label())
		}
	}
	if geology.Ore(200).String() != "" || geology.Ore(200).Label() != "" {
		t.Fatal("out-of-range ore names")
	}
}

func TestAllOresOnHugeRockMass(t *testing.T) {
	t.Parallel()
	m := terrain.NewMap(120, 120)
	for i := range m.Cells {
		m.Cells[i] = terrain.Rock
	}
	m.Set(60, 60, terrain.Soil)
	deep := len(geology.DeepCells(m, geology.Depths(m)))
	g := geology.Generate(9, m)
	for _, o := range geology.Ores() {
		n := 0
		for _, c := range g.Ore {
			if c == o {
				n++
			}
		}
		if want := geology.VeinCount(deep, o); want == 0 || n == 0 || n > want*geology.VeinMax(o) {
			t.Errorf("%v: %d cells from %d veins (max %d each)", o, n, want, geology.VeinMax(o))
		}
	}
}

func TestVeinCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ore  geology.Ore
		deep int
		want int
	}{
		{geology.Steel, 0, 0},
		{geology.Steel, 59, 0},
		{geology.Steel, 60, 1},
		{geology.Steel, 349, 1},
		{geology.Steel, 700, 2},
		{geology.Silver, 1399, 0},
		{geology.Silver, 2800, 2},
		{geology.Gold, 2799, 0},
		{geology.Gold, 5600, 2},
	}
	for _, tc := range tests {
		if got := geology.VeinCount(tc.deep, tc.ore); got != tc.want {
			t.Errorf("%v at %d deep cells: %d veins, want %d", tc.ore, tc.deep, got, tc.want)
		}
	}
	for _, c := range []struct {
		ore geology.Ore
		max int
	}{{geology.Steel, 14}, {geology.Silver, 9}, {geology.Gold, 6}} {
		if got := geology.VeinMax(c.ore); got != c.max {
			t.Errorf("%v max vein %d, want %d", c.ore, got, c.max)
		}
	}
}
