package ground

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// typeSlots is the size of a per-terrain-type counter (above the number
// of terrain types).
const typeSlots = 256

// Terrain-rule numbers (plan §2.6), all our own.
const (
	coldMean       = -20  // Climate.YearMean below -2 °C: icy beaches
	shoreMarshWet  = 0.5  // a lake ring is marsh at or above, mud below
	puddleWet      = 0.85 // cave floors this wet may hold shallow water
	puddleCell     = 6.0
	puddleCut      = 0.6
	caveGravelCell = 9.0
	caveGravelCut  = 0.6
	bankCell       = 6.0
	bankWetPull    = 0.35 // wetness pushes a bank cell toward mud
	bankMudCut     = 0.58
	bankSandCut    = 0.44
	bankIceCut     = 0.5
	cleanupPasses  = 8
)

// defaultGround is the mix of a biome without one (the legacy temperate
// forest): mostly soil, some rich soil, a little gravel.
var defaultGround = biome.Ground{Soil: 820, Rich: 100, Gravel: 80}

// Terrain is the "terrain" pass (MAPGEN-2): writes Terrain by precedence
// (§2.6), cleans up small soft-ground patches, then sets Geology.Stone on
// every rock and stone-ground cell (invariant I2). Rough-floor cave
// puddles also mark F.Water Shallow, so water terrain always has a wet
// field (I6).
type Terrain struct{}

// Name is the step's stream and label key.
func (Terrain) Name() string { return "terrain" }

// Run is the pass.
func (Terrain) Run(c *mapgen.Ctx) error {
	e := newEnv(c)
	t := c.R.Terrain.Cells
	var rest []int32
	for i := range t {
		ty, ok := e.fixed(i)
		if !ok {
			rest = append(rest, int32(i))
			continue
		}
		t[i] = ty
		if ty == terrain.ShallowWater && !c.F.Water[i].Wet() {
			c.F.Water[i] = mapgen.Shallow // a cave puddle
		}
	}
	left, counts := e.applyPatches(patchesOf(c.Cfg.Biome.ID), rest)
	groundMix(c, groundOf(c.Cfg.Biome), len(rest), left, counts)
	cleanup(c.W, c.H, t)
	setGeology(c)
	return nil
}

// env is the per-run classification context.
type env struct {
	c *mapgen.Ctx
	// cold: icy beaches. icy: a glacial biome (ice is most of its ground):
	// banks, floodplains and shores freeze too. seaIce: all ground is ice.
	cold, icy, seaIce   bool
	roughTau, gravelTau float32
	puddle, caveGravel  uint64
	bank                uint64
}

func newEnv(c *mapgen.Ctx) *env {
	d := c.Cfg.Biome
	g := d.Ground
	total := g.Soil + g.Rich + g.Gravel + g.Sand + g.Marsh + g.Ice
	return &env{
		c:          c,
		cold:       d.Climate.YearMean < coldMean,
		icy:        total > 0 && 2*g.Ice > total,
		seaIce:     d.ID == biome.SeaIce || total > 0 && g.Ice == total,
		roughTau:   c.F.RoughTau,
		gravelTau:  c.F.GravelTau,
		puddle:     c.NoiseSeed("terrain/puddle"),
		caveGravel: c.NoiseSeed("terrain/cavegravel"),
		bank:       c.NoiseSeed("terrain/bank"),
	}
}

// fixed applies precedence rules 1-5 to cell i: water, rock wall, cave
// floor, water edges, elevation bands. ok is false when the biome patches
// and the ground mix decide.
func (e *env) fixed(i int) (terrain.Type, bool) {
	f := e.c.F
	switch f.Water[i] {
	case mapgen.Deep:
		return terrain.DeepWater, true
	case mapgen.Shallow:
		return terrain.ShallowWater, true
	}
	if f.Rock[i] {
		return terrain.Rock, true
	}
	if e.seaIce {
		return terrain.Ice, true
	}
	x, y := e.c.XY(i)
	fx, fy := float64(x)+0.5, float64(y)+0.5
	wet := f.Wetness[i]
	if f.Cave[i] {
		if wet >= puddleWet && noise2.FBm(e.puddle, fx, fy, 2, puddleCell, 0.5) >= puddleCut {
			return terrain.ShallowWater, true
		}
		if noise2.FBm(e.caveGravel, fx, fy, 2, caveGravelCell, 0.5) >= caveGravelCut {
			return terrain.Gravel, true
		}
		return terrain.Stone, true
	}
	switch f.Water[i] {
	case mapgen.Beach:
		if e.cold {
			return terrain.Ice, true
		}
		return terrain.Sand, true
	case mapgen.Bank:
		n := noise2.FBm(e.bank, fx, fy, 2, bankCell, 0.5) + bankWetPull*(float64(wet)-0.5)
		switch {
		case e.icy && n >= bankIceCut:
			return terrain.Ice, true
		case e.icy:
			return terrain.Gravel, true
		case n >= bankMudCut:
			return terrain.Mud, true
		case n < bankSandCut:
			return terrain.Sand, true
		}
		return terrain.Gravel, true
	case mapgen.Floodplain:
		if e.icy {
			return terrain.Gravel, true
		}
		return terrain.RichSoil, true
	case mapgen.Shore:
		switch {
		case e.icy:
			return terrain.Ice, true
		case wet >= shoreMarshWet:
			return terrain.Marsh, true
		}
		return terrain.Mud, true
	}
	el := f.Elevation[i]
	if e.roughTau > 0 && el >= e.roughTau {
		return terrain.Stone, true
	}
	if e.gravelTau > 0 && el >= e.gravelTau {
		return terrain.Gravel, true
	}
	return 0, false
}

// groundOf is a biome's ground mix, or the default when it has none.
func groundOf(d biome.Def) biome.Ground {
	g := d.Ground
	if g.Soil+g.Rich+g.Gravel+g.Sand+g.Marsh+g.Ice <= 0 {
		return defaultGround
	}
	return g
}

// groundMix is rule 7 (and 8): the left cells take the mix by fertility
// rank, least fertile first (ice, sand, gravel, marsh, soil, rich soil),
// like the legacy rankGround. Each type's share is of all n cells that
// reached the patches, minus what the patches already laid, so the map
// keeps the biome's mix overall.
func groundMix(c *mapgen.Ctx, g biome.Ground, n int, left []int32, patched [typeSlots]int) {
	if len(left) == 0 {
		return
	}
	bands := [...]struct {
		t terrain.Type
		w int
	}{
		{terrain.Ice, g.Ice}, {terrain.Sand, g.Sand}, {terrain.Gravel, g.Gravel},
		{terrain.Marsh, g.Marsh}, {terrain.Soil, g.Soil}, {terrain.RichSoil, g.Rich},
	}
	total := 0
	for _, b := range bands {
		total += max(0, b.w)
	}
	var want [len(bands)]float64
	sum := 0.0
	for k, b := range bands {
		want[k] = max(0, float64(max(0, b.w))/float64(total)*float64(n)-float64(patched[b.t]))
		sum += want[k]
	}
	if sum <= 0 { // the patches overshot every type: the plain mix
		for k, b := range bands {
			want[k] = float64(max(0, b.w))
			sum += want[k]
		}
	}
	fert := c.F.Fertility
	order := slices.Clone(left)
	slices.SortFunc(order, func(a, b int32) int {
		if d := cmp.Compare(fert[a], fert[b]); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})
	t := c.R.Terrain.Cells
	band, upTo := 0, want[0]
	scale := float64(len(order)) / sum
	for rank, i := range order {
		for band < len(bands)-1 && float64(rank) >= upTo*scale {
			band++
			upTo += want[band]
		}
		t[i] = bands[band].t
	}
}

// minPatch is a soft-ground type's smallest allowed patch (0: not soft
// ground, never changed by cleanup).
func minPatch(t terrain.Type) int {
	switch t {
	case terrain.Sand, terrain.RichSoil, terrain.Soil:
		return 6
	case terrain.Marsh:
		return 8
	case terrain.Gravel:
		return 4
	case terrain.Mud:
		return 5
	}
	return 0
}

// mergeTarget reports a type a too-small soft patch may merge into: soft
// ground, rough stone ground or ice (never water or rock).
func mergeTarget(t terrain.Type) bool {
	return minPatch(t) > 0 || t == terrain.Stone || t == terrain.Ice
}

// typeComponents labels the 4-connected same-type components of cells.
func typeComponents(w, h int, cells []terrain.Type) (label []int32, sizes []int, types []terrain.Type) {
	return sameComponents(w, h, cells)
}

// sameComponents labels the 4-connected components of equal values
// (numbered in row-major order of their first cell) with each one's size
// and value.
func sameComponents[T ~uint8](w, h int, cells []T) (label []int32, sizes []int, vals []T) {
	label = make([]int32, w*h)
	for i := range label {
		label[i] = -1
	}
	var queue []int32
	for s := range label {
		if label[s] >= 0 {
			continue
		}
		k, v := int32(len(sizes)), cells[s]
		label[s] = k
		queue = append(queue[:0], int32(s))
		for head := 0; head < len(queue); head++ {
			i := int(queue[head])
			x := i % w
			if x > 0 && label[i-1] < 0 && cells[i-1] == v {
				label[i-1] = k
				queue = append(queue, int32(i-1))
			}
			if x < w-1 && label[i+1] < 0 && cells[i+1] == v {
				label[i+1] = k
				queue = append(queue, int32(i+1))
			}
			if i >= w && label[i-w] < 0 && cells[i-w] == v {
				label[i-w] = k
				queue = append(queue, int32(i-w))
			}
			if i+w < w*h && label[i+w] < 0 && cells[i+w] == v {
				label[i+w] = k
				queue = append(queue, int32(i+w))
			}
		}
		sizes = append(sizes, len(queue))
		vals = append(vals, v)
	}
	return label, sizes, vals
}

// cleanup merges every soft-ground patch smaller than its type's minimum
// into the type most of its border touches (soft ground, stone ground or
// ice; ties to the lower type), smallest patches first. Water, rock, stone
// ground and ice are never changed; a small patch boxed in by them stays.
func cleanup(w, h int, cells []terrain.Type) {
	mergeSmall(w, h, cells, func(t terrain.Type, size int) bool { return size < minPatch(t) }, mergeTarget, cleanupPasses)
}

// mergeSmall repeatedly gives every component with small(value, size) the
// value most of its border cells hold among those with target(value)
// (ties to the lower value), smallest components first, until nothing
// changes or passes run out. A component with no target neighbour stays.
func mergeSmall[T ~uint8](w, h int, cells []T, small func(T, int) bool, target func(T) bool, passes int) {
	for pass := 0; pass < passes; pass++ {
		label, sizes, vals := sameComponents(w, h, cells)
		var todo []int32
		for k, s := range sizes {
			if small(vals[k], s) {
				todo = append(todo, int32(k))
			}
		}
		if len(todo) == 0 {
			return
		}
		slices.SortFunc(todo, func(a, b int32) int {
			if d := cmp.Compare(sizes[a], sizes[b]); d != 0 {
				return d
			}
			return cmp.Compare(a, b)
		})
		slot := make([]int32, len(sizes))
		for k := range slot {
			slot[k] = -1
		}
		for n, k := range todo {
			slot[k] = int32(n)
		}
		members := make([][]int32, len(todo))
		for i, k := range label {
			if n := slot[k]; n >= 0 {
				members[n] = append(members[n], int32(i))
			}
		}
		changed := false
		for n, k := range todo {
			var votes [256]int
			for _, i32 := range members[n] {
				i := int(i32)
				x, y := i%w, i/w
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					if j := ny*w + nx; label[j] != k && target(cells[j]) {
						votes[cells[j]]++
					}
				}
			}
			best := -1
			for v, cnt := range votes {
				if cnt > 0 && (best < 0 || cnt > votes[best]) {
					best = v
				}
			}
			if best < 0 || T(best) == cells[members[n][0]] {
				continue
			}
			for _, i := range members[n] {
				cells[i] = T(best)
			}
			changed = true
		}
		if !changed {
			return
		}
	}
}

// setGeology writes Geology.Stone: the material field's stone on every rock
// and stone-ground cell (the first site stone, or the map's commonest,
// where the field has none), None elsewhere (I2).
func setGeology(c *mapgen.Ctx) {
	fallback := geology.Granite
	if s := siteStones(c); len(s) > 0 {
		fallback = s[0]
	} else {
		var n [typeSlots]int
		for _, s := range c.F.Stone {
			n[s]++
		}
		best := 0
		for s, k := range n {
			if s != int(geology.None) && k > best {
				fallback, best = geology.Stone(s), k
			}
		}
	}
	t, st := c.R.Terrain.Cells, c.R.Geology.Stone
	for i, ty := range t {
		switch {
		case ty != terrain.Rock && ty != terrain.Stone:
			st[i] = geology.None
		case c.F.Stone[i] != geology.None:
			st[i] = c.F.Stone[i]
		default:
			st[i] = fallback
		}
	}
}
