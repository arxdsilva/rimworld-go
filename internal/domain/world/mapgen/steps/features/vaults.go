package features

import (
	"math"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/vault"
	"rimworld-go/pkg/randx"
)

// Placement numbers of the vaults pass.
const (
	vaultStartDist  = 30  // cells (Chebyshev) from the landing spot
	vaultApart      = 12  // cells (Chebyshev) from other vaults
	vaultTries      = 400 // candidate positions per vault
	vaultMaxPerMap  = 3
	vaultRockShare  = 0.70 // rock share of the footprint plus its 2-cell ring
	vaultSteelShare = 30   // percent of vaults walled in steel
)

// Vaults is the "vaults" pass (MAPGEN-3): sealed chambers cut into rock. It
// writes Vaults, Terrain and Geology (interior and ring), clears Rock and sets
// Blocked on every footprint cell. Roof stays natural.
type Vaults struct{}

// Name is the step's stream and label key.
func (Vaults) Name() string { return "vaults" }

// vaultCount is the attempts a map gets: one per 250x250 of area.
func vaultCount(c *mapgen.Ctx) int {
	if c.Cfg.Site.Biome == biome.SeaIce {
		return 0
	}
	n := int(math.Round(float64(c.W*c.H) / (250 * 250)))
	return clampInt(n, 1, vaultMaxPerMap)
}

// Run is the pass.
func (Vaults) Run(c *mapgen.Ctx) error {
	rn := c.Rand("vaults")
	n := vaultCount(c)
	for k := 0; k < n; k++ {
		bays := 1 + rn.IntN(4)
		rows := 1 + rn.IntN(2)
		iw, ih := 3*bays+1, 7
		if rows == 2 {
			ih = 10
		}
		fw, fh := iw+2, ih+2
		rangeX, rangeY := c.W-2-fw+1, c.H-2-fh+1 // x0 in [1, rangeX]
		if rangeX < 1 || rangeY < 1 {
			continue
		}
		for t := 0; t < vaultTries; t++ {
			x0, y0 := 1+rn.IntN(rangeX), 1+rn.IntN(rangeY)
			b := rect{x0, y0, x0 + fw - 1, y0 + fh - 1}
			if vaultFits(c, b) {
				buildVault(c, rn, b, bays, rows)
				break
			}
		}
	}
	return nil
}

// cheb is the Chebyshev gap between two rectangles.
func (a rect) cheb(b rect) int {
	dx := max(0, max(a.x0-b.x1, b.x0-a.x1))
	dy := max(0, max(a.y0-b.y1, b.y0-a.y1))
	return max(dx, dy)
}

// vaultFits reports footprint b a legal site.
func vaultFits(c *mapgen.Ctx, b rect) bool {
	st := c.Start()
	if b.cheb(rect{st.X, st.Y, st.X, st.Y}) < vaultStartDist {
		return false
	}
	for _, v := range c.R.Vaults {
		if b.cheb(rect{v.X0, v.Y0, v.X1, v.Y1}) < vaultApart {
			return false
		}
	}
	if !c.F.Rock[c.Idx((b.x0+b.x1)/2, (b.y0+b.y1)/2)] {
		return false
	}
	rock, total := 0, 0
	for y := b.y0 - 2; y <= b.y1+2; y++ {
		for x := b.x0 - 2; x <= b.x1+2; x++ {
			total++
			if !c.In(x, y) {
				continue
			}
			i := c.Idx(x, y)
			inside := x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1
			if inside && (c.F.Cave[i] || c.F.Road[i] || c.F.Blocked[i] || c.F.Water[i].Wet() || terrain.IsWater(c.R.Terrain.At(x, y))) {
				return false
			}
			if c.F.Rock[i] {
				rock++
			}
		}
	}
	return float64(rock) >= vaultRockShare*float64(total)
}

// vaultStone is the stone of the rock around the vault.
func vaultStone(c *mapgen.Ctx, b rect) geology.Stone {
	for _, pick := range []func(x, y int) geology.Stone{
		func(x, y int) geology.Stone { return c.R.Geology.At(x, y) },
		func(x, y int) geology.Stone { return c.F.Stone[c.Idx(x, y)] },
	} {
		if s := pick((b.x0+b.x1)/2, (b.y0+b.y1)/2); s != geology.None {
			return s
		}
		for y := b.y0; y <= b.y1; y++ {
			for x := b.x0; x <= b.x1; x++ {
				if s := pick(x, y); s != geology.None {
					return s
				}
			}
		}
	}
	return geology.Granite
}

// rollOccupant draws a casket's occupant from the vault table.
func rollOccupant(rn *randx.Rand) vault.Occupant {
	switch r := rn.IntN(100); {
	case r < 30:
		return vault.Grateful
	case r < 55:
		return vault.Hostile
	case r < 70:
		return vault.SleeperAndCrawler
	case r < 80:
		return vault.Beetles
	case r < 90:
		return vault.Empty
	default:
		return vault.Remains
	}
}

// buildVault carves footprint b (ring included) and fills it.
func buildVault(c *mapgen.Ctx, rn *randx.Rand, b rect, bays, rows int) {
	stone := vaultStone(c, b)
	blocks := item.BlocksOf(stone)
	v := mapgen.Vault{ID: len(c.R.Vaults) + 1, X0: b.x0, Y0: b.y0, X1: b.x1, Y1: b.y1, WallStuff: blocks}
	if rn.IntN(100) < vaultSteelShare {
		v.WallStuff = item.Steel
	}
	for y := b.y0; y <= b.y1; y++ {
		for x := b.x0; x <= b.x1; x++ {
			i := c.Idx(x, y)
			c.R.Terrain.Set(x, y, terrain.StoneSmooth)
			c.R.Geology.Set(x, y, stone)
			c.R.Geology.SetOre(x, y, geology.NoOre) // the vault was cut through any ore
			c.F.Rock[i] = false
			c.F.Blocked[i] = true
			if v.Inside(x, y) {
				v.Floors = append(v.Floors, mapgen.RuinFloor{X: x, Y: y, Stuff: blocks})
			} else {
				v.Walls = append(v.Walls, mapgen.Point{X: x, Y: y})
			}
		}
	}
	// Interior origin; caskets stand in bays along the top (and bottom) edge.
	ix, iy := b.x0+1, b.y0+1
	ih := b.y1 - b.y0 - 1
	under := map[[2]int]bool{}
	for r := 0; r < rows; r++ {
		cy := iy
		if r == 1 {
			cy = iy + ih - 2
		}
		for k := 0; k < bays; k++ {
			cx := ix + 3*k + 1
			v.Caskets = append(v.Caskets, mapgen.Casket{X: cx, Y: cy, Occupant: rollOccupant(rn)})
			under[[2]int{cx, cy}], under[[2]int{cx, cy + 1}] = true, true
		}
	}
	// Contents strip: the floor between the casket rows.
	sy0, sy1 := iy+2, iy+ih-1
	if rows == 2 {
		sy1 = iy + ih - 3
	}
	var free []mapgen.Point
	for y := sy0; y <= sy1; y++ {
		for x := ix; x <= b.x1-1; x++ {
			if !under[[2]int{x, y}] {
				free = append(free, mapgen.Point{X: x, Y: y})
			}
		}
	}
	shuffle(rn, free)
	spots := vaultLoot(rn)
	for _, s := range spots {
		p := free[0]
		free = free[1:]
		v.Loot = append(v.Loot, mapgen.Loot{X: p.X, Y: p.Y, Kind: s.kind, Qty: s.lo + rn.IntN(s.hi-s.lo+1)})
	}
	for d, nd := 0, 2+rn.IntN(5); d < nd && len(free) > 0; d++ {
		v.DefenderSpots = append(v.DefenderSpots, free[0])
		free = free[1:]
	}
	c.R.Vaults = append(c.R.Vaults, v)
}

// vaultLootTable is the vault's stack table; vaultSpecials are the rare finds.
var (
	vaultLootBase = []lootEntry{
		{item.Components, 4, 12},
		{item.IndustrialMedicine, 3, 8},
		{item.Silver, 150, 450},
		{item.Gold, 20, 70},
	}
	vaultRifle    = lootEntry{item.Rifle, 1, 1}
	vaultSpecials = []item.Kind{item.ProstheticLeg, item.ProstheticArm, item.GlassEye, item.SparkTablet}
)

// vaultLoot draws 3-6 stacks: a rifle (35%), one rare find (15%), and the
// rest from the base table.
func vaultLoot(rn *randx.Rand) []lootEntry {
	n := 3 + rn.IntN(4)
	var out []lootEntry
	if rn.IntN(100) < 35 {
		out = append(out, vaultRifle)
	}
	if rn.IntN(100) < 15 {
		out = append(out, lootEntry{vaultSpecials[rn.IntN(len(vaultSpecials))], 1, 1})
	}
	base := append([]lootEntry(nil), vaultLootBase...)
	shuffle(rn, base)
	take := min(len(base), max(1, n-len(out)))
	return append(out, base[:take]...)
}
