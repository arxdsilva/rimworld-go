package start

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
)

func validate(t testing.TB, c *mapgen.Ctx) {
	t.Helper()
	if err := (Validate{}).Run(c); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCarvesThroughRockWall(t *testing.T) {
	c := fixture(1, 100, 100)
	for y := range 100 {
		for x := 40; x <= 44; x++ {
			setRock(c, x, y)
		}
	}
	c.R.Start = mapgen.Point{X: 70, Y: 50}
	if r := startReach(c); r > 0.6 {
		t.Fatalf("fixture reach %v should start below 70%%", r)
	}
	validate(t, c)
	if c.R.Metrics.StartReach < 0.7 {
		t.Fatalf("reach %v after carving", c.R.Metrics.StartReach)
	}
	opened := 0
	for y := range 100 {
		for x := 40; x <= 44; x++ {
			if c.R.Terrain.At(x, y) == terrain.Stone {
				opened++
				if c.F.Rock[c.Idx(x, y)] || c.R.Roof[c.Idx(x, y)] != 0 {
					t.Fatalf("carved cell %d,%d still rock or roofed", x, y)
				}
			}
		}
	}
	if opened == 0 || opened > 5*5 {
		t.Fatalf("%d wall cells opened, want a narrow crossing", opened)
	}
	if !hasRepair(c, "reach") {
		t.Fatalf("carving not logged: %v", c.R.Metrics.Repairs)
	}
	if v := Violations(c); len(v) > 0 {
		t.Fatalf("violations after validate: %v", v)
	}
}

func TestValidateCarvesOutOfEnclosure(t *testing.T) {
	// the start sits in a walled pocket: carving reaches the edge too.
	c := fixture(2, 60, 60)
	for y := range 60 {
		for x := range 60 {
			if d := max(abs(x-30), abs(y-30)); d >= 8 && d <= 10 {
				setRock(c, x, y)
			}
		}
	}
	setWater(c, 30, 45, mapgen.Deep)
	c.R.Start = mapgen.Point{X: 30, Y: 30}
	validate(t, c)
	if c.R.Metrics.StartReach < 0.7 {
		t.Fatalf("reach %v", c.R.Metrics.StartReach)
	}
}

func TestValidateOpensUnwalkableStart(t *testing.T) {
	c := fixture(3, 30, 30)
	setRock(c, 15, 15)
	c.R.Start = mapgen.Point{X: 15, Y: 15}
	validate(t, c)
	if !c.R.Terrain.At(15, 15).Walkable() || c.R.Metrics.StartReach < 0.99 {
		t.Fatalf("start cell %v reach %v", c.R.Terrain.At(15, 15), c.R.Metrics.StartReach)
	}
}

func TestValidateRepairsInvariants(t *testing.T) {
	cases := []struct {
		inv    string
		break_ func(c *mapgen.Ctx)
		check  func(t *testing.T, c *mapgen.Ctx)
	}{
		{"I1", func(c *mapgen.Ctx) { setRock(c, 5, 5); c.F.Rock[c.Idx(5, 5)] = false; c.F.Rock[c.Idx(8, 8)] = true },
			func(t *testing.T, c *mapgen.Ctx) {
				if !c.F.Rock[c.Idx(5, 5)] || c.F.Rock[c.Idx(8, 8)] {
					t.Fatal("rock mask not synced to terrain")
				}
			}},
		{"I2", func(c *mapgen.Ctx) {
			setRock(c, 5, 5)
			c.R.Geology.Stone[c.Idx(5, 5)] = geology.None
			c.R.Geology.Stone[c.Idx(8, 8)] = geology.Marble
		}, func(t *testing.T, c *mapgen.Ctx) {
			if c.R.Geology.Stone[c.Idx(5, 5)] != geology.Granite || c.R.Geology.Stone[c.Idx(8, 8)] != geology.None {
				t.Fatal("stone not repaired")
			}
		}},
		{"I3", func(c *mapgen.Ctx) { c.R.Geology.Ore[c.Idx(8, 8)] = geology.Gold }, func(t *testing.T, c *mapgen.Ctx) {
			if c.R.Geology.Ore[c.Idx(8, 8)] != geology.NoOre {
				t.Fatal("ore off rock kept")
			}
		}},
		{"I4", func(c *mapgen.Ctx) {
			c.R.Roof[c.Idx(8, 8)] = roof.Natural
			c.R.Roof[c.Idx(9, 9)], c.F.Cave[c.Idx(9, 9)] = roof.Natural, true
		}, func(t *testing.T, c *mapgen.Ctx) {
			if c.R.Roof[c.Idx(8, 8)] != 0 || c.R.Roof[c.Idx(9, 9)] == 0 {
				t.Fatal("open-ground roof kept or cave roof dropped")
			}
		}},
		{"I5", func(c *mapgen.Ctx) {
			setWater(c, 10, 10, mapgen.Deep)
			setRock(c, 11, 11)
			c.R.Terrain.Cells[c.Idx(12, 12)] = terrain.Track
			c.R.Chunks = []mapgen.ChunkSpawn{{X: 10, Y: 10}, {X: 3, Y: 3}, {X: 3, Y: 3}, {X: 6, Y: 6}, {X: -1, Y: 2}}
			c.R.Plants = []mapgen.PlantSpawn{{X: 12, Y: 12, Species: plant.Tree}, {X: 3, Y: 3}, {X: 25, Y: 25}, {X: 25, Y: 25}}
			c.R.Vents = []mapgen.Point{{X: 5, Y: 5}, {X: 30, Y: 30}}
			c.R.Ruins = []mapgen.Ruin{{Walls: []mapgen.RuinWall{{X: 11, Y: 11, Stuff: item.Wood}, {X: 20, Y: 5, Stuff: item.Wood}},
				Floors: []mapgen.RuinFloor{{X: 20, Y: 6}}, Loot: []mapgen.Loot{{X: 20, Y: 6, Kind: item.Steel, Qty: 10}}},
				{Walls: []mapgen.RuinWall{{X: 11, Y: 11}}}}
			c.F.Blocked[c.Idx(3, 3)] = true
		}, func(t *testing.T, c *mapgen.Ctx) {
			if want := []mapgen.ChunkSpawn{{X: 3, Y: 3}, {X: 6, Y: 6}}; !slices.Equal(c.R.Chunks, want) {
				t.Fatalf("chunks %v: want only the first valid one, %v", c.R.Chunks, want)
			}
			if want := []mapgen.PlantSpawn{{X: 25, Y: 25}}; !slices.Equal(c.R.Plants, want) {
				t.Fatalf("plants %v", c.R.Plants)
			}
			if want := []mapgen.Point{{X: 30, Y: 30}}; !slices.Equal(c.R.Vents, want) {
				t.Fatalf("vents %v: the one over the chunk at 6,6 goes", c.R.Vents)
			}
			if len(c.R.Ruins) != 1 || len(c.R.Ruins[0].Walls) != 1 || len(c.R.Ruins[0].Floors) != 1 || len(c.R.Ruins[0].Loot) != 1 {
				t.Fatalf("ruins %+v: the wall on rock and the empty ruin go, loot on its own floor stays", c.R.Ruins)
			}
			for _, q := range [][2]int{{3, 3}, {6, 6}, {30, 30}, {31, 31}, {20, 5}, {20, 6}} {
				if !c.F.Blocked[c.Idx(q[0], q[1])] {
					t.Fatalf("feature cell %v not blocked", q)
				}
			}
			if c.F.Blocked[c.Idx(5, 5)] || c.F.Blocked[c.Idx(25, 25)] {
				t.Fatal("dropped vent's or plant's cell left blocked")
			}
		}},
		{"I6", func(c *mapgen.Ctx) { c.R.Terrain.Cells[c.Idx(8, 8)] = terrain.ShallowWater }, func(t *testing.T, c *mapgen.Ctx) {
			if c.F.Water[c.Idx(8, 8)] != mapgen.Shallow {
				t.Fatal("water class not synced")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.inv, func(t *testing.T) {
			c := fixture(4, 40, 40)
			c.R.Start = mapgen.Point{X: 20, Y: 20}
			if v := Violations(c); len(v) > 0 {
				t.Fatalf("clean fixture reports %v", v)
			}
			tc.break_(c)
			if v := Violations(c); !slices.ContainsFunc(v, func(s string) bool { return strings.HasPrefix(s, tc.inv) }) {
				t.Fatalf("violation not detected: %v", v)
			}
			validate(t, c)
			if v := Violations(c); len(v) > 0 {
				t.Fatalf("violations after validate: %v", v)
			}
			if !hasRepair(c, tc.inv) {
				t.Fatalf("repair not logged: %v", c.R.Metrics.Repairs)
			}
			tc.check(t, c)
		})
	}
}

func TestValidateRiverContinuity(t *testing.T) {
	c := fixture(5, 100, 100)
	for x := range 100 {
		for y := 49; y <= 51; y++ {
			if x < 30 || x > 35 {
				setWater(c, x, y, mapgen.Shallow)
				c.F.River[c.Idx(x, y)] = true
			}
		}
	}
	for x := 0; x <= 100; x += 10 {
		c.F.RiverLine = append(c.F.RiverLine, mapgen.PointF{X: float64(x), Y: 50.5})
	}
	// a ford road and the start's dried pad are left alone
	c.R.Terrain.Cells[c.Idx(60, 50)] = terrain.Track
	c.R.Terrain.Cells[c.Idx(80, 50)], c.F.Water[c.Idx(80, 50)] = terrain.Mud, mapgen.Dry
	c.R.Start = mapgen.Point{X: 80, Y: 53}
	c.R.Plants = []mapgen.PlantSpawn{{X: 32, Y: 50}}
	validate(t, c)
	for x := range 100 {
		i := c.Idx(x, 50)
		switch {
		case x == 60:
			if c.R.Terrain.Cells[i] != terrain.Track {
				t.Fatal("ford overwritten")
			}
		case x == 80:
			if c.R.Terrain.Cells[i] != terrain.Mud {
				t.Fatal("start pad flooded again")
			}
		case !terrain.IsWater(c.R.Terrain.Cells[i]) || !c.F.Water[i].Wet() || !c.F.River[i]:
			t.Fatalf("river gap at %d,50: %v", x, c.R.Terrain.Cells[i])
		}
	}
	if len(c.R.Plants) != 0 || !hasRepair(c, "river") {
		t.Fatalf("plants %v repairs %v", c.R.Plants, c.R.Metrics.Repairs)
	}
}

func TestValidateOceanConnectivity(t *testing.T) {
	c := fixture(6, 80, 80)
	mark := func(x, y int, w mapgen.Water) {
		setWater(c, x, y, w)
		c.F.Coast[c.Idx(x, y)] = true
	}
	for y := range 10 {
		for x := range 80 {
			mark(x, y, mapgen.Deep)
		}
	}
	for y := 40; y <= 42; y++ {
		for x := 40; x <= 42; x++ {
			mark(x, y, mapgen.Deep)
		}
	}
	setWater(c, 60, 60, mapgen.Deep) // a lake cell: not ocean, kept
	c.R.Start = mapgen.Point{X: 20, Y: 40}
	validate(t, c)
	if terrain.IsWater(c.R.Terrain.At(41, 41)) || c.F.Water[c.Idx(41, 41)].Wet() {
		t.Fatal("stray ocean pocket kept")
	}
	if !terrain.IsWater(c.R.Terrain.At(5, 5)) || !terrain.IsWater(c.R.Terrain.At(60, 60)) {
		t.Fatal("edge ocean or lake dropped")
	}
	if !hasRepair(c, "ocean") {
		t.Fatalf("repairs %v", c.R.Metrics.Repairs)
	}
}

func TestValidateKeepsEarlierRepairsAndIsDeterministic(t *testing.T) {
	a, b := noiseFixture(3, 120, 90), noiseFixture(3, 120, 90)
	for _, c := range []*mapgen.Ctx{a, b} {
		c.R.Metrics.Repairs = []string{"start: test"}
		run(t, c)
		validate(t, c)
	}
	if !hasRepair(a, "start: test") {
		t.Fatal("earlier repairs dropped")
	}
	if !slices.Equal(a.R.Terrain.Cells, b.R.Terrain.Cells) || !slices.Equal(a.R.Plants, b.R.Plants) ||
		!slices.Equal(a.R.Chunks, b.R.Chunks) || a.R.Start != b.R.Start || !slices.Equal(a.R.Metrics.Repairs, b.R.Metrics.Repairs) {
		t.Fatal("validate is not deterministic")
	}
}

func TestStartThenValidateOnNoiseMaps(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		for _, sz := range [][2]int{{24, 24}, {80, 120}, {200, 200}} {
			c := noiseFixture(seed, sz[0], sz[1])
			p := run(t, c)
			validate(t, c)
			if v := Violations(c); len(v) > 0 {
				t.Fatalf("seed %d %v: %v", seed, sz, v)
			}
			if c.R.Metrics.StartReach < 0.7 {
				t.Fatalf("seed %d %v: reach %v", seed, sz, c.R.Metrics.StartReach)
			}
			within(c, p, 4, func(x, y, i int) {
				if terrain.IsWater(c.R.Terrain.Cells[i]) || terrain.IsRockWall(c.R.Terrain.Cells[i]) {
					t.Fatalf("seed %d %v: %v at %d,%d near start %v", seed, sz, c.R.Terrain.Cells[i], x, y, p)
				}
			})
		}
	}
}

func TestValidateTinyMaps(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 7}, {24, 24}} {
		c := fixture(1, sz[0], sz[1])
		run(t, c)
		validate(t, c)
		if v := Violations(c); len(v) > 0 {
			t.Fatalf("%v: %v", sz, v)
		}
	}
	// all rock: the start is opened and carving digs out
	c := fixture(2, 30, 30)
	for i := range c.R.Terrain.Cells {
		x, y := c.XY(i)
		setRock(c, x, y)
	}
	run(t, c)
	validate(t, c)
	if v := Violations(c); len(v) > 0 || c.R.Metrics.StartReach < 0.7 {
		t.Fatalf("all-rock map: %v reach %v", v, c.R.Metrics.StartReach)
	}
}

func BenchmarkStartValidate500(b *testing.B) {
	base := noiseFixture(1, 500, 500)
	for b.Loop() {
		b.StopTimer()
		c := cloneCtx(base)
		b.StartTimer()
		if err := (Start{}).Run(c); err != nil {
			b.Fatal(err)
		}
		if err := (Validate{}).Run(c); err != nil {
			b.Fatal(err)
		}
	}
}

func cloneCtx(c *mapgen.Ctx) *mapgen.Ctx {
	n := fixture(c.Cfg.Seed, c.W, c.H)
	f, r := *c.F, *c.R
	f.Rock, f.Water, f.Road, f.Blocked, f.Cave = slices.Clone(f.Rock), slices.Clone(f.Water), slices.Clone(f.Road), slices.Clone(f.Blocked), slices.Clone(f.Cave)
	f.Lake, f.River, f.Coast, f.StartScore = slices.Clone(f.Lake), slices.Clone(f.River), slices.Clone(f.Coast), slices.Clone(f.StartScore)
	f.RockDepth = slices.Clone(f.RockDepth)
	r.Terrain.Cells, r.Roof = slices.Clone(r.Terrain.Cells), slices.Clone(r.Roof)
	r.Geology.Stone, r.Geology.Ore = slices.Clone(r.Geology.Stone), slices.Clone(r.Geology.Ore)
	r.Chunks, r.Plants = slices.Clone(r.Chunks), slices.Clone(r.Plants)
	r.Metrics.Repairs = nil
	*n.F, *n.R = f, r
	return n
}
