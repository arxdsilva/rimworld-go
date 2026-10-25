package features

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/randx"
)

// Placement numbers of the ruins pass.
const (
	ruinStartDist  = 20.0 // cells from the landing spot
	ruinFeatureGap = 12.0 // cells from other ruins and vents
	ruinTries      = 60   // candidate positions per ruin
	ruinShapeTries = 3    // shapes tried per ruin
)

// Ruins is the "ruins" pass (MAPGEN-2): writes Ruins, Blocked.
// Small surface ruins only: broken wall runs and roofless sheds.
type Ruins struct{}

// Name is the step's stream and label key.
func (Ruins) Name() string { return "ruins" }

// shape is a ruin's layout relative to its top-left cell.
type shape struct {
	kind  mapgen.RuinKind
	w, h  int
	cells [][2]int // wall piece cells
	door  int      // index in cells forced missing (-1: none)
}

// ruinCount draws how many ruins the map gets.
func ruinCount(c *mapgen.Ctx, rn *randx.Rand) int {
	n := rn.IntN(4) // 0..3
	switch c.Cfg.Site.Biome {
	case biome.IceSheet, biome.SeaIce, biome.ExtremeDesert:
		n = rn.IntN(2) // 0..1
	}
	if c.Cfg.Site.Settled {
		n++
	}
	return n
}

// Run is the pass.
func (Ruins) Run(c *mapgen.Ctx) error {
	rn := c.Rand("ruins")
	n := ruinCount(c, rn)
	for k := 0; k < n; k++ {
		placeRuin(c, rn)
	}
	return nil
}

func newWallRun(rn *randx.Rand) shape {
	n := 5 + rn.IntN(9) // 5..13 cells
	s := shape{kind: mapgen.WallRun, door: -1}
	if rn.IntN(2) == 0 { // straight
		s.w, s.h = n, 1
		for x := 0; x < n; x++ {
			s.cells = append(s.cells, [2]int{x, 0})
		}
	} else { // one bend
		a := 2 + rn.IntN(n-2) // horizontal arm 2..n-1
		b := n + 1 - a
		s.w, s.h = a, b
		for x := 0; x < a; x++ {
			s.cells = append(s.cells, [2]int{x, 0})
		}
		for y := 1; y < b; y++ {
			s.cells = append(s.cells, [2]int{a - 1, y})
		}
		if rn.IntN(2) == 0 { // mirror x
			for i := range s.cells {
				s.cells[i][0] = a - 1 - s.cells[i][0]
			}
		}
		if rn.IntN(2) == 0 { // mirror y
			for i := range s.cells {
				s.cells[i][1] = b - 1 - s.cells[i][1]
			}
		}
	}
	if rn.IntN(2) == 0 && s.h == 1 { // vertical straight run
		s.w, s.h = 1, s.w
		for i := range s.cells {
			s.cells[i] = [2]int{0, s.cells[i][0]}
		}
	}
	return s
}

func newShed(rn *randx.Rand) shape {
	w, h := 4+rn.IntN(5), 4+rn.IntN(4) // 4..8 x 4..7
	s := shape{kind: mapgen.Shed, w: w, h: h}
	var doors []int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x != 0 && y != 0 && x != w-1 && y != h-1 {
				continue
			}
			corner := (x == 0 || x == w-1) && (y == 0 || y == h-1)
			if !corner {
				doors = append(doors, len(s.cells))
			}
			s.cells = append(s.cells, [2]int{x, y})
		}
	}
	s.door = doors[rn.IntN(len(doors))]
	return s
}

// localStone is the stone at the ruin (centre first, then anywhere in it).
func localStone(c *mapgen.Ctx, b rect) geology.Stone {
	if s := c.F.Stone[c.Idx((b.x0+b.x1)/2, (b.y0+b.y1)/2)]; s != geology.None {
		return s
	}
	for y := b.y0; y <= b.y1; y++ {
		for x := b.x0; x <= b.x1; x++ {
			if s := c.F.Stone[c.Idx(x, y)]; s != geology.None {
				return s
			}
		}
	}
	return geology.None
}

// fits reports the footprint plus a 1-cell margin placeable at b.
func fits(c *mapgen.Ctx, b rect, ruins, vents []rect) bool {
	if b.x0 < 1 || b.y0 < 1 || b.x1 > c.W-2 || b.y1 > c.H-2 {
		return false
	}
	for y := b.y0 - 1; y <= b.y1+1; y++ {
		for x := b.x0 - 1; x <= b.x1+1; x++ {
			if !openGround(c, x, y) || c.F.River[c.Idx(x, y)] {
				return false
			}
		}
	}
	st := c.Start()
	if b.gap(rect{st.X, st.Y, st.X, st.Y}) < ruinStartDist {
		return false
	}
	return farFrom(b, ruins, ruinFeatureGap) && farFrom(b, vents, ruinFeatureGap)
}

func placeRuin(c *mapgen.Ctx, rn *randx.Rand) {
	for s := 0; s < ruinShapeTries; s++ {
		var sh shape
		if rn.IntN(2) == 0 {
			sh = newWallRun(rn)
		} else {
			sh = newShed(rn)
		}
		ruins, vents := existing(c)
		rangeX, rangeY := c.W-sh.w-1, c.H-sh.h-1 // x0 in [1, rangeX]
		if rangeX < 1 || rangeY < 1 {
			continue
		}
		for t := 0; t < ruinTries; t++ {
			x0, y0 := 1+rn.IntN(rangeX), 1+rn.IntN(rangeY)
			b := rect{x0, y0, x0 + sh.w - 1, y0 + sh.h - 1}
			if fits(c, b, ruins, vents) {
				build(c, rn, sh, b)
				return
			}
		}
	}
}

// build fills one ruin of shape sh at b and marks its cells blocked.
func build(c *mapgen.Ctx, rn *randx.Rand, sh shape, b rect) {
	stone := localStone(c, b)
	stuff := func() item.Kind {
		if stone != geology.None && rn.IntN(2) == 0 {
			return item.BlocksOf(stone)
		}
		return item.Wood
	}
	wallStuff := stuff()
	ru := mapgen.Ruin{Kind: sh.kind, X0: b.x0, Y0: b.y0, X1: b.x1, Y1: b.y1}
	taken := map[[2]int]bool{} // cells already used by this ruin (lookup only)
	mark := func(x, y int) {
		taken[[2]int{x, y}] = true
		c.F.Blocked[c.Idx(x, y)] = true
	}

	// Walls: choose which pieces are missing.
	n := len(sh.cells)
	lo, hi := 0.20, 0.40
	if sh.kind == mapgen.Shed {
		lo, hi = 0.30, 0.55
	}
	minMiss, maxMiss := ceilShare(n, lo), floorShare(n, hi)
	miss := clampInt(ceilShare(n, lo+(hi-lo)*float64(rn.IntN(101))/100), minMiss, maxMiss)
	if sh.door >= 0 {
		miss = max(miss, 1)
	}
	order := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if i != sh.door {
			order = append(order, i)
		}
	}
	shuffle(rn, order)
	gone := make([]bool, n)
	left := miss
	if sh.door >= 0 {
		gone[sh.door] = true
		left--
	}
	for _, i := range order {
		if left <= 0 {
			break
		}
		gone[i] = true
		left--
	}
	for i, cell := range sh.cells {
		if gone[i] {
			continue
		}
		x, y := b.x0+cell[0], b.y0+cell[1]
		ru.Walls = append(ru.Walls, mapgen.RuinWall{X: x, Y: y, Stuff: wallStuff, DamagePct: 35 + rn.IntN(46)})
		mark(x, y)
	}

	if sh.kind == mapgen.Shed {
		buildShedInterior(c, rn, &ru, sh, b, stone, stuff(), taken, mark)
	}
	c.R.Ruins = append(c.R.Ruins, ru)
}

func buildShedInterior(c *mapgen.Ctx, rn *randx.Rand, ru *mapgen.Ruin, sh shape, b rect,
	stone geology.Stone, floorStuff item.Kind, taken map[[2]int]bool, mark func(x, y int)) {
	var inner [][2]int
	for y := b.y0 + 1; y < b.y1; y++ {
		for x := b.x0 + 1; x < b.x1; x++ {
			inner = append(inner, [2]int{x, y})
		}
	}
	// Floor on 40..80% of the interior.
	ni := len(inner)
	nf := clampInt(ceilShare(ni, 0.40+0.40*float64(rn.IntN(101))/100), ceilShare(ni, 0.40), floorShare(ni, 0.80))
	pool := append([][2]int(nil), inner...)
	shuffle(rn, pool)
	for _, p := range pool[:nf] {
		ru.Floors = append(ru.Floors, mapgen.RuinFloor{X: p[0], Y: p[1], Stuff: floorStuff})
		mark(p[0], p[1])
	}
	// Loot: 1..3 stacks of distinct kinds on distinct interior cells.
	shuffle(rn, pool)
	kinds := make([]int, len(lootTable))
	for i := range kinds {
		kinds[i] = i
	}
	shuffle(rn, kinds)
	nl := min(1+rn.IntN(3), len(pool))
	for i := 0; i < nl; i++ {
		e := lootTable[kinds[i]]
		p := pool[i]
		ru.Loot = append(ru.Loot, mapgen.Loot{X: p[0], Y: p[1], Kind: e.kind, Qty: e.lo + rn.IntN(e.hi-e.lo+1)})
		taken[p] = true
		c.F.Blocked[c.Idx(p[0], p[1])] = true
	}
	if stone == geology.None {
		return
	}
	// Rubble: 1..4 chunks in or around the shed on cells nothing else uses.
	var free [][2]int
	for y := b.y0 - 1; y <= b.y1+1; y++ {
		for x := b.x0 - 1; x <= b.x1+1; x++ {
			if !taken[[2]int{x, y}] {
				free = append(free, [2]int{x, y})
			}
		}
	}
	shuffle(rn, free)
	nr := min(1+rn.IntN(4), len(free))
	for _, p := range free[:nr] {
		ru.Rubble = append(ru.Rubble, mapgen.ChunkSpawn{X: p[0], Y: p[1], Stone: stone})
		mark(p[0], p[1])
	}
}
