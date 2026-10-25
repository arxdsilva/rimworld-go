package features

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

// fixture is an open soil map with granite everywhere and the start in a corner.
func fixture(seed uint64, w, h int, st site.Site) *mapgen.Ctx {
	c := mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: st})
	for i := range c.F.Stone {
		c.F.Stone[i] = geology.Granite
	}
	c.R.Start = mapgen.Point{X: 5, Y: 5}
	return c
}

func runAll(t *testing.T, c *mapgen.Ctx) {
	t.Helper()
	for _, s := range []mapgen.Step{Ruins{}, Vents{}} {
		if err := s.Run(c); err != nil {
			t.Fatal(err)
		}
	}
}

func ruinRect(r mapgen.Ruin) rect { return rect{r.X0, r.Y0, r.X1, r.Y1} }

func TestNames(t *testing.T) {
	if (Ruins{}).Name() != "ruins" || (Vents{}).Name() != "vents" {
		t.Fatal("names changed")
	}
}

func TestDeterminism(t *testing.T) {
	st := site.Site{Hills: site.LargeHills, Settled: true}
	a, b := fixture(1, 200, 200, st), fixture(1, 200, 200, st)
	runAll(t, a)
	runAll(t, b)
	if len(a.R.Ruins) == 0 || len(a.R.Vents) == 0 {
		t.Fatalf("expected features, got %d ruins %d vents", len(a.R.Ruins), len(a.R.Vents))
	}
	if !equalRuns(a, b) {
		t.Fatal("same seed differs")
	}
	different := false
	for s := uint64(2); s < 12 && !different; s++ {
		c := fixture(s, 200, 200, st)
		runAll(t, c)
		different = !equalRuns(a, c)
	}
	if !different {
		t.Fatal("other seeds never differ")
	}
}

func equalRuns(a, b *mapgen.Ctx) bool {
	return slices.EqualFunc(a.R.Ruins, b.R.Ruins, func(x, y mapgen.Ruin) bool {
		return x.Kind == y.Kind && x.X0 == y.X0 && x.Y0 == y.Y0 && x.X1 == y.X1 && x.Y1 == y.Y1 &&
			slices.Equal(x.Walls, y.Walls) && slices.Equal(x.Floors, y.Floors) && slices.Equal(x.Loot, y.Loot) && slices.Equal(x.Rubble, y.Rubble)
	}) && slices.Equal(a.R.Vents, b.R.Vents) && slices.Equal(a.F.Blocked, b.F.Blocked)
}

func countRange(t *testing.T, st site.Site, f func(*mapgen.Ctx) int) (lo, hi int) {
	t.Helper()
	lo, hi = 1<<30, -1
	for s := uint64(1); s <= 80; s++ {
		c := fixture(s, 300, 300, st)
		runAll(t, c)
		n := f(c)
		lo, hi = min(lo, n), max(hi, n)
	}
	return lo, hi
}

func ruinsOf(c *mapgen.Ctx) int { return len(c.R.Ruins) }
func ventsOf(c *mapgen.Ctx) int { return len(c.R.Vents) }

func TestRuinCounts(t *testing.T) {
	if lo, hi := countRange(t, site.Site{}, ruinsOf); lo != 0 || hi != 3 {
		t.Fatalf("temperate ruins range %d..%d, want 0..3", lo, hi)
	}
	if lo, hi := countRange(t, site.Site{Settled: true}, ruinsOf); lo != 1 || hi != 4 {
		t.Fatalf("settled ruins range %d..%d, want 1..4", lo, hi)
	}
	for _, b := range []biome.ID{biome.IceSheet, biome.SeaIce, biome.ExtremeDesert} {
		if lo, hi := countRange(t, site.Site{Biome: b}, ruinsOf); lo != 0 || hi != 1 {
			t.Fatalf("biome %v ruins range %d..%d, want 0..1", b, lo, hi)
		}
	}
}

func TestVentCounts(t *testing.T) {
	if lo, hi := countRange(t, site.Site{Hills: site.Flat}, ventsOf); lo != 0 || hi != 2 {
		t.Fatalf("flat vents %d..%d, want 0..2", lo, hi)
	}
	for _, h := range []site.Hills{site.SmallHills, site.LargeHills, site.Mountainous} {
		if lo, hi := countRange(t, site.Site{Hills: h}, ventsOf); lo != 1 || hi != 3 {
			t.Fatalf("hills %v vents %d..%d, want 1..3", h, lo, hi)
		}
	}
	if _, hi := countRange(t, site.Site{Biome: biome.SeaIce, Hills: site.Mountainous}, ventsOf); hi != 0 {
		t.Fatal("vents on sea ice")
	}
}

// scatterTerrain makes a hostile fixture: rock band, river, water, road, cave.
func hostile(seed uint64) *mapgen.Ctx {
	c := fixture(seed, 300, 300, site.Site{Hills: site.Mountainous, Settled: true})
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			i := c.Idx(x, y)
			switch {
			case x >= 100 && x < 130:
				c.F.Rock[i] = true
				c.R.Terrain.Set(x, y, terrain.Rock)
			case x >= 150 && x < 160:
				c.F.Water[i] = mapgen.Deep
				c.R.Terrain.Set(x, y, terrain.DeepWater)
			case x >= 200 && x < 203:
				c.F.Road[i] = true
				c.R.Terrain.Set(x, y, terrain.Track)
			case y >= 250 && x < 100:
				c.F.Cave[i] = true
			}
		}
	}
	return c
}

func TestPlacementRules(t *testing.T) {
	for seed := uint64(1); seed <= 40; seed++ {
		c := hostile(seed)
		runAll(t, c)
		var rects []rect
		var isVent []bool
		for _, r := range c.R.Ruins {
			rects, isVent = append(rects, ruinRect(r)), append(isVent, false)
			b := ruinRect(r)
			for y := b.y0 - 1; y <= b.y1+1; y++ {
				for x := b.x0 - 1; x <= b.x1+1; x++ {
					i := c.Idx(x, y)
					if c.F.Rock[i] || c.F.Water[i].Wet() || c.F.Road[i] || c.F.Cave[i] || c.R.Terrain.At(x, y) == terrain.Track {
						t.Fatalf("seed %d: ruin %v footprint+1 on bad ground at %d,%d", seed, b, x, y)
					}
				}
			}
			if b.gap(rect{5, 5, 5, 5}) < 20 {
				t.Fatalf("seed %d: ruin too near start", seed)
			}
		}
		for _, v := range c.R.Vents {
			rects, isVent = append(rects, rect{v.X, v.Y, v.X + 1, v.Y + 1}), append(isVent, true)
			if (rect{v.X, v.Y, v.X + 1, v.Y + 1}).gap(rect{5, 5, 5, 5}) < 18 {
				t.Fatalf("seed %d: vent too near start", seed)
			}
			for _, p := range [4][2]int{{v.X, v.Y}, {v.X + 1, v.Y}, {v.X, v.Y + 1}, {v.X + 1, v.Y + 1}} {
				i := c.Idx(p[0], p[1])
				if c.F.Rock[i] || c.F.Water[i].Wet() || c.F.Road[i] || c.F.Cave[i] || !c.F.Blocked[i] {
					t.Fatalf("seed %d: vent cell %v bad", seed, p)
				}
			}
		}
		for a := range rects {
			for b := a + 1; b < len(rects); b++ {
				need := 12.0
				if isVent[a] && isVent[b] {
					need = 20
				}
				if g := rects[a].gap(rects[b]); g < need {
					t.Fatalf("seed %d: features %d,%d gap %.1f < %.0f", seed, a, b, g, need)
				}
			}
		}
	}
}

func TestRuinShapes(t *testing.T) {
	sawRun, sawShed, sawBend, sawStone := false, false, false, false
	for seed := uint64(1); seed <= 150; seed++ {
		c := fixture(seed, 300, 300, site.Site{Settled: true})
		_ = (Ruins{}).Run(c)
		used := map[[2]int]bool{}
		use := func(x, y int, what string) {
			if used[[2]int{x, y}] {
				t.Fatalf("seed %d: %s overlaps at %d,%d", seed, what, x, y)
			}
			used[[2]int{x, y}] = true
			if !c.F.Blocked[c.Idx(x, y)] {
				t.Fatalf("seed %d: %s cell %d,%d not blocked", seed, what, x, y)
			}
		}
		for _, r := range c.R.Ruins {
			w, h := r.X1-r.X0+1, r.Y1-r.Y0+1
			for _, wl := range r.Walls {
				use(wl.X, wl.Y, "wall")
				if wl.DamagePct < 35 || wl.DamagePct > 80 {
					t.Fatalf("damage %d", wl.DamagePct)
				}
				if wl.Stuff != item.Wood && wl.Stuff != item.GraniteBlocks {
					t.Fatalf("wall stuff %v", wl.Stuff)
				}
				if wl.Stuff != item.Wood {
					sawStone = true
				}
			}
			switch r.Kind {
			case mapgen.WallRun:
				sawRun = true
				n := w + h - 1
				if w > 1 && h > 1 {
					sawBend = true
				}
				if n < 5 || n > 13 {
					t.Fatalf("run length %d", n)
				}
				missing := float64(n-len(r.Walls)) / float64(n)
				if missing < 0.2-1e-9 || missing > 0.4+1e-9 {
					t.Fatalf("run missing share %.2f (n=%d)", missing, n)
				}
			case mapgen.Shed:
				sawShed = true
				if w < 4 || w > 8 || h < 4 || h > 7 {
					t.Fatalf("shed %dx%d", w, h)
				}
				per := 2*(w+h) - 4
				missing := float64(per-len(r.Walls)) / float64(per)
				if missing < 0.3-1e-9 || missing > 0.55+1e-9 || per == len(r.Walls) {
					t.Fatalf("shed missing share %.2f", missing)
				}
				inner := (w - 2) * (h - 2)
				fs := float64(len(r.Floors)) / float64(inner)
				if fs < 0.4-1e-9 || fs > 0.8+1e-9 {
					t.Fatalf("floor share %.2f", fs)
				}
				for _, f := range r.Floors {
					use(f.X, f.Y, "floor")
					if f.X <= r.X0 || f.X >= r.X1 || f.Y <= r.Y0 || f.Y >= r.Y1 {
						t.Fatal("floor outside interior")
					}
				}
				if len(r.Loot) < 1 || len(r.Loot) > 3 {
					t.Fatalf("loot stacks %d", len(r.Loot))
				}
				for _, l := range r.Loot {
					ok := false
					for _, e := range lootTable {
						if e.kind == l.Kind && l.Qty >= e.lo && l.Qty <= e.hi {
							ok = true
						}
					}
					if !ok {
						t.Fatalf("loot %+v not from table", l)
					}
					// loot may lie on a floor tile, but never on a wall or rubble
					for _, wl := range r.Walls {
						if wl.X == l.X && wl.Y == l.Y {
							t.Fatal("loot on wall")
						}
					}
					if !c.F.Blocked[c.Idx(l.X, l.Y)] {
						t.Fatal("loot not blocked")
					}
				}
				if len(r.Rubble) < 1 || len(r.Rubble) > 4 {
					t.Fatalf("rubble %d", len(r.Rubble))
				}
				for _, rb := range r.Rubble {
					use(rb.X, rb.Y, "rubble")
					if rb.Stone != geology.Granite {
						t.Fatal("rubble stone")
					}
					for _, l := range r.Loot {
						if l.X == rb.X && l.Y == rb.Y {
							t.Fatal("rubble on loot")
						}
					}
				}
			}
		}
	}
	if !sawRun || !sawShed || !sawBend || !sawStone {
		t.Fatalf("variety: run %v shed %v bend %v stone %v", sawRun, sawShed, sawBend, sawStone)
	}
}

func TestNoStoneUsesWood(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		c := fixture(seed, 200, 200, site.Site{Settled: true})
		clear(c.F.Stone)
		_ = (Ruins{}).Run(c)
		for _, r := range c.R.Ruins {
			for _, w := range r.Walls {
				if w.Stuff != item.Wood {
					t.Fatal("stone stuff without stone")
				}
			}
			if len(r.Rubble) != 0 {
				t.Fatal("rubble without stone")
			}
		}
	}
}

func TestVentsPreferRockEdges(t *testing.T) {
	near, total := 0, 0
	for seed := uint64(1); seed <= 60; seed++ {
		c := fixture(seed, 300, 300, site.Site{Hills: site.Mountainous})
		for y := 0; y < 300; y++ {
			for x := 140; x < 160; x++ {
				c.F.Rock[c.Idx(x, y)] = true
				c.R.Terrain.Set(x, y, terrain.Rock)
			}
		}
		_ = (Vents{}).Run(c)
		for _, v := range c.R.Vents {
			total++
			if v.X >= 140-8 && v.X <= 160+8 {
				near++
			}
		}
	}
	// the open cells within 8 of the rock are ~6% of the open ground; weighting must beat that
	if total == 0 || float64(near)/float64(total) < 0.2 {
		t.Fatalf("near-rock share %d/%d", near, total)
	}
}

func TestNothingFitsNoPanic(t *testing.T) {
	st := site.Site{Hills: site.Mountainous, Settled: true}
	c := fixture(3, 120, 120, st)
	for i := range c.F.Rock {
		c.F.Rock[i] = true
	}
	runAll(t, c)
	if len(c.R.Ruins)+len(c.R.Vents) != 0 {
		t.Fatal("placed on rock")
	}
	for _, sz := range [][2]int{{1, 1}, {3, 3}, {10, 10}, {24, 24}, {24, 60}, {60, 24}} {
		for s := uint64(1); s <= 20; s++ {
			c := fixture(s, sz[0], sz[1], st)
			c.R.Start = mapgen.Point{X: sz[0] / 2, Y: sz[1] / 2}
			runAll(t, c)
		}
	}
}

func TestSmallMapRespectsDistances(t *testing.T) {
	for s := uint64(1); s <= 30; s++ {
		c := fixture(s, 60, 60, site.Site{Hills: site.Mountainous, Settled: true})
		runAll(t, c)
		for _, r := range c.R.Ruins {
			if ruinRect(r).gap(rect{5, 5, 5, 5}) < 20 {
				t.Fatal("start distance")
			}
		}
	}
}

func BenchmarkRuins500(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := fixture(uint64(i), 500, 500, site.Site{Settled: true})
		_ = (Ruins{}).Run(c)
	}
}

func BenchmarkVents500(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := fixture(uint64(i), 500, 500, site.Site{Hills: site.Mountainous})
		for x := 240; x < 260; x++ {
			for y := 0; y < 500; y++ {
				c.F.Rock[c.Idx(x, y)] = true
			}
		}
		_ = (Vents{}).Run(c)
	}
}
