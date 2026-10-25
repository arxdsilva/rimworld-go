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
	"rimworld-go/internal/domain/world/vault"
)

// rockMap is a map that is solid rock except a clearing around the landing spot.
func rockMap(seed uint64, w, h int, st site.Site) *mapgen.Ctx {
	c := fixture(seed, w, h, st)
	c.R.Start = mapgen.Point{X: 5, Y: 5}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < 12 && y < 12 {
				continue
			}
			c.F.Rock[c.Idx(x, y)] = true
			c.R.Terrain.Set(x, y, terrain.Rock)
			c.R.Geology.Set(x, y, geology.Granite)
		}
	}
	return c
}

func runVaults(t *testing.T, c *mapgen.Ctx) {
	t.Helper()
	if err := (Vaults{}).Run(c); err != nil {
		t.Fatal(err)
	}
}

func vrect(v mapgen.Vault) rect { return rect{v.X0, v.Y0, v.X1, v.Y1} }

func TestVaultsName(t *testing.T) {
	if (Vaults{}).Name() != "vaults" {
		t.Fatal("name changed")
	}
}

func TestVaultsDeterminism(t *testing.T) {
	a, b := rockMap(1, 400, 400, site.Site{}), rockMap(1, 400, 400, site.Site{})
	runVaults(t, a)
	runVaults(t, b)
	if len(a.R.Vaults) == 0 {
		t.Fatal("no vaults")
	}
	if !slices.EqualFunc(a.R.Vaults, b.R.Vaults, func(x, y mapgen.Vault) bool {
		return x.ID == y.ID && x.X0 == y.X0 && x.Y0 == y.Y0 && x.X1 == y.X1 && x.Y1 == y.Y1 &&
			x.WallStuff == y.WallStuff && slices.Equal(x.Walls, y.Walls) && slices.Equal(x.Floors, y.Floors) &&
			slices.Equal(x.Caskets, y.Caskets) && slices.Equal(x.Loot, y.Loot) && slices.Equal(x.DefenderSpots, y.DefenderSpots)
	}) {
		t.Fatal("same seed differs")
	}
	c := rockMap(2, 400, 400, site.Site{})
	runVaults(t, c)
	if len(c.R.Vaults) > 0 && c.R.Vaults[0].X0 == a.R.Vaults[0].X0 && c.R.Vaults[0].Y0 == a.R.Vaults[0].Y0 {
		t.Fatal("other seed gives the same spot")
	}
}

func TestVaultCountAndIDs(t *testing.T) {
	for _, tc := range []struct{ w, h, want int }{{250, 250, 1}, {200, 200, 1}, {350, 350, 2}, {500, 500, 3}, {700, 700, 3}} {
		c := rockMap(3, tc.w, tc.h, site.Site{})
		runVaults(t, c)
		if len(c.R.Vaults) != tc.want {
			t.Fatalf("%dx%d: %d vaults, want %d", tc.w, tc.h, len(c.R.Vaults), tc.want)
		}
		for i, v := range c.R.Vaults {
			if v.ID != i+1 {
				t.Fatalf("id %d at index %d", v.ID, i)
			}
		}
	}
}

func TestVaultsSeaIceNone(t *testing.T) {
	c := rockMap(1, 400, 400, site.Site{Biome: biome.SeaIce})
	runVaults(t, c)
	if len(c.R.Vaults) != 0 {
		t.Fatal("vault on sea ice")
	}
}

func TestVaultsTinyAndRockPoor(t *testing.T) {
	for _, d := range [][2]int{{1, 1}, {3, 3}, {10, 12}, {16, 16}, {40, 40}} {
		c := rockMap(1, d[0], d[1], site.Site{})
		runVaults(t, c)
	}
	c := fixture(1, 300, 300, site.Site{}) // no rock at all
	runVaults(t, c)
	if len(c.R.Vaults) != 0 {
		t.Fatal("vault without rock")
	}
}

// manyVaults generates vaults over many seeds on a large rock map.
func manyVaults(t *testing.T, seeds int) (cs []*mapgen.Ctx) {
	t.Helper()
	for s := 1; s <= seeds; s++ {
		c := rockMap(uint64(s), 500, 500, site.Site{})
		runVaults(t, c)
		cs = append(cs, c)
	}
	return cs
}

func TestVaultGeometry(t *testing.T) {
	sizes := map[[2]int]bool{}
	for _, c := range manyVaults(t, 40) {
		for _, v := range c.R.Vaults {
			w, h := v.X1-v.X0+1, v.Y1-v.Y0+1
			iw, ih := w-2, h-2
			if (iw-1)%3 != 0 || iw < 4 || iw > 13 || (ih != 7 && ih != 10) {
				t.Fatalf("bad interior %dx%d", iw, ih)
			}
			sizes[[2]int{iw, ih}] = true
			if v.X0 < 1 || v.Y0 < 1 || v.X1 > c.W-2 || v.Y1 > c.H-2 {
				t.Fatalf("margin violated %+v", v)
			}
			// Ring is complete.
			ring := map[mapgen.Point]bool{}
			for _, p := range v.Walls {
				ring[p] = true
			}
			if len(ring) != 2*(w+h)-4 || len(v.Walls) != len(ring) {
				t.Fatalf("ring has %d cells, want %d", len(ring), 2*(w+h)-4)
			}
			for y := v.Y0; y <= v.Y1; y++ {
				for x := v.X0; x <= v.X1; x++ {
					edge := !v.Inside(x, y)
					if edge != ring[mapgen.Point{X: x, Y: y}] {
						t.Fatalf("ring mismatch at %d,%d", x, y)
					}
					i := c.Idx(x, y)
					if c.F.Rock[i] || !c.F.Blocked[i] || c.R.Terrain.At(x, y) != terrain.StoneSmooth || c.R.Geology.At(x, y) != geology.Granite {
						t.Fatalf("cell %d,%d not carved", x, y)
					}
				}
			}
			if len(v.Floors) != iw*ih {
				t.Fatalf("%d floors, want %d", len(v.Floors), iw*ih)
			}
			for _, f := range v.Floors {
				if !v.Inside(f.X, f.Y) || f.Stuff != item.GraniteBlocks {
					t.Fatalf("bad floor %+v", f)
				}
			}
			if v.WallStuff != item.GraniteBlocks && v.WallStuff != item.Steel {
				t.Fatalf("wall stuff %v", v.WallStuff)
			}
			sealedInterior(t, c, v)
		}
	}
	if len(sizes) < 6 {
		t.Fatalf("only %d distinct interior sizes", len(sizes))
	}
}

// sealedInterior flood-fills 4-connected from an interior cell over non-wall
// cells and requires it never leaves the ring.
func sealedInterior(t *testing.T, c *mapgen.Ctx, v mapgen.Vault) {
	t.Helper()
	wall := map[mapgen.Point]bool{}
	for _, p := range v.Walls {
		wall[p] = true
	}
	seen := map[mapgen.Point]bool{}
	q := []mapgen.Point{{X: v.X0 + 1, Y: v.Y0 + 1}}
	seen[q[0]] = true
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := mapgen.Point{X: p.X + d[0], Y: p.Y + d[1]}
			if seen[n] || wall[n] {
				continue
			}
			if !v.Inside(n.X, n.Y) {
				t.Fatalf("interior leaks to %v", n)
			}
			seen[n] = true
			q = append(q, n)
		}
	}
	if len(seen) != len(v.Floors) {
		t.Fatalf("reached %d of %d interior cells", len(seen), len(v.Floors))
	}
}

func TestVaultPlacementRules(t *testing.T) {
	for _, c := range manyVaults(t, 40) {
		st := c.Start()
		for i, v := range c.R.Vaults {
			if vrect(v).cheb(rect{st.X, st.Y, st.X, st.Y}) < 30 {
				t.Fatalf("vault %d too near start", v.ID)
			}
			for _, o := range c.R.Vaults[i+1:] {
				if vrect(v).cheb(vrect(o)) < 12 {
					t.Fatal("vaults too close")
				}
			}
		}
	}
}

func TestVaultEmbeddedAndAvoidsFeatures(t *testing.T) {
	// 40% of the map is rock band; caves, water, road and blocked stripes must be avoided.
	for s := uint64(1); s <= 30; s++ {
		c := hostile(s)
		c.R.Start = mapgen.Point{X: 5, Y: 5}
		before := slices.Clone(c.F.Blocked)
		runVaults(t, c)
		for _, v := range c.R.Vaults {
			rock, total := 0, 0
			for y := v.Y0 - 2; y <= v.Y1+2; y++ {
				for x := v.X0 - 2; x <= v.X1+2; x++ {
					total++
					if !c.In(x, y) {
						continue
					}
					i := c.Idx(x, y)
					inside := x >= v.X0 && x <= v.X1 && y >= v.Y0 && y <= v.Y1
					if inside {
						if c.F.Cave[i] || c.F.Road[i] || c.F.Water[i].Wet() || before[i] {
							t.Fatalf("vault %d overlaps a feature at %d,%d", v.ID, x, y)
						}
						rock++ // carved cells were rock or soil; counted below via the original
					} else if c.F.Rock[i] {
						rock++
					}
				}
			}
			if rock*10 < total*7 {
				t.Fatalf("vault %d embedded %d/%d", v.ID, rock, total)
			}
		}
	}
}

func TestVaultRockPoorFailsGracefully(t *testing.T) {
	c := rockMap(1, 300, 300, site.Site{})
	// Leave only a thin rock lattice: no footprint reaches 70%.
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if x%3 != 0 || y%3 != 0 {
				c.F.Rock[c.Idx(x, y)] = false
			}
		}
	}
	runVaults(t, c)
	if len(c.R.Vaults) != 0 {
		t.Fatal("vault forced on rock-poor map")
	}
}

func TestVaultCaskets(t *testing.T) {
	counts := map[vault.Occupant]int{}
	total := 0
	for _, c := range manyVaults(t, 120) {
		for _, v := range c.R.Vaults {
			perRow := (v.X1 - v.X0 - 1 - 1) / 3
			ih := v.Y1 - v.Y0 - 1
			rows := 1
			if ih == 10 {
				rows = 2
			}
			if len(v.Caskets) != perRow*rows {
				t.Fatalf("%d caskets, want %d", len(v.Caskets), perRow*rows)
			}
			seen := map[mapgen.Point]bool{}
			for _, k := range v.Caskets {
				for dy := 0; dy < 2; dy++ {
					p := mapgen.Point{X: k.X, Y: k.Y + dy}
					if !v.Inside(p.X, p.Y) || seen[p] {
						t.Fatalf("casket cell %v outside or overlapping", p)
					}
					seen[p] = true
				}
				if k.Occupant < vault.Grateful || k.Occupant >= vault.NumOccupants {
					t.Fatalf("occupant %v", k.Occupant)
				}
				counts[k.Occupant]++
				total++
			}
			for _, l := range v.Loot {
				if seen[mapgen.Point{X: l.X, Y: l.Y}] {
					t.Fatal("loot under casket")
				}
			}
			for _, d := range v.DefenderSpots {
				if seen[d] {
					t.Fatal("defender under casket")
				}
			}
		}
	}
	want := map[vault.Occupant]float64{vault.Grateful: 30, vault.Hostile: 25, vault.SleeperAndCrawler: 15, vault.Beetles: 10, vault.Empty: 10, vault.Remains: 10}
	for o, w := range want {
		got := 100 * float64(counts[o]) / float64(total)
		if got < w-7 || got > w+7 {
			t.Fatalf("occupant %v share %.1f%%, want %.0f%%±7 (n=%d)", o, got, w, total)
		}
	}
}

func TestVaultLootAndDefenders(t *testing.T) {
	allowed := map[item.Kind][2]int{
		item.Components: {4, 12}, item.IndustrialMedicine: {3, 8}, item.Silver: {150, 450}, item.Gold: {20, 70},
		item.Rifle: {1, 1}, item.ProstheticLeg: {1, 1}, item.ProstheticArm: {1, 1}, item.GlassEye: {1, 1}, item.SparkTablet: {1, 1},
	}
	rifles, vaults := 0, 0
	for _, c := range manyVaults(t, 100) {
		for _, v := range c.R.Vaults {
			vaults++
			if len(v.Loot) < 3 || len(v.Loot) > 6 || len(v.DefenderSpots) < 2 || len(v.DefenderSpots) > 6 {
				t.Fatalf("loot %d defenders %d", len(v.Loot), len(v.DefenderSpots))
			}
			cells := map[mapgen.Point]bool{}
			kinds := map[item.Kind]bool{}
			for _, l := range v.Loot {
				r, ok := allowed[l.Kind]
				if !ok || l.Qty < r[0] || l.Qty > r[1] {
					t.Fatalf("bad loot %+v", l)
				}
				if kinds[l.Kind] {
					t.Fatalf("duplicate kind %v", l.Kind)
				}
				kinds[l.Kind] = true
				p := mapgen.Point{X: l.X, Y: l.Y}
				if !v.Inside(p.X, p.Y) || cells[p] {
					t.Fatal("loot cell outside or shared")
				}
				cells[p] = true
				if l.Kind == item.Rifle {
					rifles++
				}
			}
			for _, d := range v.DefenderSpots {
				if !v.Inside(d.X, d.Y) || cells[d] {
					t.Fatal("defender cell outside or shared")
				}
				cells[d] = true
			}
		}
	}
	if share := 100 * rifles / vaults; share < 20 || share > 50 {
		t.Fatalf("rifle share %d%%", share)
	}
}

func TestVaultWallStuffMix(t *testing.T) {
	steel, n := 0, 0
	for _, c := range manyVaults(t, 100) {
		for _, v := range c.R.Vaults {
			n++
			if v.WallStuff == item.Steel {
				steel++
			}
		}
	}
	if s := 100 * steel / n; s < 15 || s > 45 {
		t.Fatalf("steel share %d%%", s)
	}
}

func TestVaultStoneFallbackToFields(t *testing.T) {
	c := rockMap(1, 400, 400, site.Site{})
	for i := range c.R.Geology.Stone {
		c.R.Geology.Stone[i] = geology.None
	}
	for i := range c.F.Stone {
		c.F.Stone[i] = geology.Marble
	}
	runVaults(t, c)
	if len(c.R.Vaults) == 0 || c.R.Vaults[0].Floors[0].Stuff != item.MarbleBlocks {
		t.Fatal("stone fallback to Fields.Stone failed")
	}
	for i := range c.F.Stone {
		c.F.Stone[i] = geology.None
	}
	if vaultStone(c, rect{1, 1, 3, 3}) != geology.Granite {
		t.Fatal("default stone")
	}
}

// A vault is cut through whatever ore was there: no ore stays under its
// walls or floor (invariant I3: ore only in rock).
func TestVaultClearsOre(t *testing.T) {
	c := rockMap(3, 200, 200, site.Site{})
	for i := range c.R.Geology.Ore {
		c.R.Geology.Ore[i] = geology.Steel
	}
	runVaults(t, c)
	if len(c.R.Vaults) == 0 {
		t.Fatal("no vault placed")
	}
	for _, v := range c.R.Vaults {
		for y := v.Y0; y <= v.Y1; y++ {
			for x := v.X0; x <= v.X1; x++ {
				if c.R.Geology.OreAt(x, y) != geology.NoOre {
					t.Fatalf("vault %d keeps ore at %d,%d", v.ID, x, y)
				}
			}
		}
	}
}
