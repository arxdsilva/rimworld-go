package start

import (
	"fmt"
	"math"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
)

// Validation and repair (our numbers).
const (
	reachNeed   = 0.70 // share of dry walkable land reachable from the start
	narrowTries = 8    // carves 3 wide first ...
	carveTries  = 24   // ... then 5 wide, up to this many in all
)

// Validate is the last pass: local repairs (river continuity, ocean
// connectivity, reachability from the start, road connectivity,
// invariants I1–I6 and feature overlap), then the map's Metrics. Every
// repair is logged in Metrics.Repairs. Bridges count as walkable ground;
// vault footprints are sealed (neither walked nor carved to).
type Validate struct{}

// Name is the step's stream and label key.
func (Validate) Name() string { return "validate" }

// Run is the pass.
func (Validate) Run(c *mapgen.Ctx) error {
	var log []string
	add := func(format string, n int) {
		if n > 0 {
			log = append(log, fmt.Sprintf(format, n))
		}
	}
	add("river: %d gap cells refilled", riverGaps(c))
	add("ocean: %d stray water cells dropped", strayOcean(c))
	add("reach: %d crossings carved from the start", carveReach(c))
	log = append(log, roadsConnected(c)...)
	log = append(log, invariants(c, true)...)
	m := metrics(c)
	m.Repairs = append(c.R.Metrics.Repairs, log...)
	c.R.Metrics = m
	return nil
}

// Violations lists the invariant breaches (I1–I6) of a finished map
// without repairing them; empty when the map is sound.
func Violations(c *mapgen.Ctx) []string { return invariants(c, false) }

// riverGaps re-fills the cells along the river centreline that are no
// longer water as shallow water. Road Track and the start's dried pad are
// left alone.
func riverGaps(c *mapgen.Ctx) int {
	pts := c.F.RiverLine
	n, last := 0, -1
	for k := 0; k+1 < len(pts); k++ {
		a, b := pts[k], pts[k+1]
		steps := max(1, int(math.Ceil(2*math.Hypot(b.X-a.X, b.Y-a.Y))))
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			x, y := int(math.Floor(a.X+(b.X-a.X)*t)), int(math.Floor(a.Y+(b.Y-a.Y)*t))
			if !c.In(x, y) {
				continue
			}
			i := c.Idx(x, y)
			if i == last {
				continue
			}
			last = i
			if tt := c.R.Terrain.Cells[i]; terrain.IsWater(tt) || tt == terrain.Track || nearStart(c, x, y, waterRepair) {
				continue
			}
			c.R.Terrain.Cells[i] = terrain.ShallowWater
			c.F.Water[i], c.F.River[i] = mapgen.Shallow, true
			c.F.Rock[i], c.F.RockDepth[i] = false, 0
			c.R.Geology.Stone[i], c.R.Geology.Ore[i], c.R.Roof[i] = geology.None, geology.NoOre, 0
			n++
		}
	}
	return n
}

func nearStart(c *mapgen.Ctx, x, y, r int) bool {
	dx, dy := x-c.R.Start.X, y-c.R.Start.Y
	return dx*dx+dy*dy <= r*r
}

// strayOcean drops coast water cut off from every map edge (water bodies
// are joined over any water, so an estuary through a river still counts),
// and keeps the Ocean mask on water only.
func strayOcean(c *mapgen.Ctx) int {
	defer func() {
		for i, o := range c.R.Ocean {
			if o && i < len(c.R.Terrain.Cells) && !terrain.IsWater(c.R.Terrain.Cells[i]) {
				c.R.Ocean[i] = false
			}
		}
	}()
	water := make([]bool, c.W*c.H)
	anyCoast := false
	for i, t := range c.R.Terrain.Cells {
		water[i] = terrain.IsWater(t)
		anyCoast = anyCoast || (water[i] && c.F.Coast[i])
	}
	if !anyCoast {
		return 0
	}
	label, sizes := mapgen.Components(c.W, c.H, water, false)
	edge := make([]bool, len(sizes))
	for i, l := range label {
		if x, y := c.XY(i); l >= 0 && (x == 0 || y == 0 || x == c.W-1 || y == c.H-1) {
			edge[l] = true
		}
	}
	n := 0
	for i, l := range label {
		if l >= 0 && c.F.Coast[i] && !edge[l] {
			c.R.Terrain.Cells[i] = terrain.Sand
			c.F.Water[i] = mapgen.Beach
			n++
		}
	}
	return n
}

// land is dry walkable ground.
func land(t terrain.Type) bool { return t.Walkable() && !terrain.IsWater(t) }

// carveReach opens crossings until the start reaches a map edge and
// reachNeed of the dry land (the legacy generator's carve, kept): each try
// cuts the cheapest way (fewest blocked cells) to the edge, or to the
// largest unreached land, 3 cells wide and later 5.
func carveReach(c *mapgen.Ctx) int {
	n := 0
	if s := c.Idx(c.R.Start.X, c.R.Start.Y); !c.R.Terrain.Cells[s].Walkable() {
		openCell(c, s)
		n++
	}
	for try := range carveTries {
		reached, edge, ok := reach(c)
		if ok {
			break
		}
		rad := 1
		if try >= narrowTries {
			rad = 2
		}
		if !carve(c, reached, edge, rad) {
			break
		}
		n++
	}
	return n
}

// reach floods (4-connected) from the start over walkable cells and
// bridges.
func reach(c *mapgen.Ctx) (reached []bool, edge, ok bool) {
	reached = make([]bool, c.W*c.H)
	o := newOverlay(c)
	s := c.Idx(c.R.Start.X, c.R.Start.Y)
	if !o.walk(c, s) {
		return reached, false, false
	}
	src := make([]bool, c.W*c.H)
	src[s] = true
	walk := make([]bool, c.W*c.H)
	for i := range walk {
		walk[i] = o.walk(c, i)
	}
	d := mapgen.BFSDist(c.W, c.H, src, walk, false)
	got, total := 0, 0
	for i := range c.R.Terrain.Cells {
		isLand := o.land(c, i)
		if isLand {
			total++
		}
		if d[i] < 0 {
			continue
		}
		reached[i] = true
		if isLand {
			got++
		}
		if x, y := c.XY(i); x == 0 || y == 0 || x == c.W-1 || y == c.H-1 {
			edge = true
		}
	}
	return reached, edge, edge && float64(got) >= reachNeed*float64(total)
}

// carve opens the cheapest crossing from the reached area to the edge
// (when no edge is reached) or to the largest unreached land, rad cells to
// either side of the way. It reports whether anything was opened.
func carve(c *mapgen.Ctx, reached []bool, edge bool, rad int) bool {
	n := c.W * c.H
	o := newOverlay(c)
	target := make([]bool, n)
	if !edge {
		for i := range target {
			x, y := c.XY(i)
			target[i] = x == 0 || y == 0 || x == c.W-1 || y == c.H-1
		}
	} else {
		free := make([]bool, n)
		for i := range free {
			free[i] = !reached[i] && o.walk(c, i)
		}
		label, sizes := mapgen.Components(c.W, c.H, free, false)
		landN := make([]int, len(sizes))
		for i, l := range label {
			if l >= 0 && o.land(c, i) {
				landN[l]++
			}
		}
		best := -1
		for k, v := range landN {
			if v > 0 && (best < 0 || v > landN[best]) {
				best = k
			}
		}
		if best < 0 {
			return false
		}
		for i, l := range label {
			target[i] = int(l) == best
		}
	}
	// 0-1 BFS by cost levels: stepping onto a blocked cell costs 1
	cost := make([]int32, n)
	parent := make([]int32, n)
	var cur []int32
	for i := range cost {
		cost[i], parent[i] = math.MaxInt32, -1
		if reached[i] {
			cost[i] = 0
			cur = append(cur, int32(i))
		}
	}
	end := int32(-1)
	for level := int32(0); len(cur) > 0 && end < 0; level++ {
		var next []int32
		for h := 0; h < len(cur); h++ {
			i := cur[h]
			if cost[i] != level {
				continue
			}
			if target[i] && !reached[i] {
				end = i
				break
			}
			x, y := c.XY(int(i))
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if !c.In(nx, ny) {
					continue
				}
				m := int32(c.Idx(nx, ny))
				if o.vault[m] {
					continue // sealed: never carved
				}
				step := int32(0)
				if !o.walk(c, int(m)) {
					step = 1
				}
				if nc := level + step; nc < cost[m] {
					cost[m], parent[m] = nc, i
					if step == 0 {
						cur = append(cur, m)
					} else {
						next = append(next, m)
					}
				}
			}
		}
		cur = next
	}
	if end < 0 {
		return false
	}
	for p := end; p >= 0 && !reached[p]; p = parent[p] {
		x, y := c.XY(int(p))
		for dy := -rad; dy <= rad; dy++ {
			for dx := -rad; dx <= rad; dx++ {
				if c.In(x+dx, y+dy) {
					openCell(c, c.Idx(x+dx, y+dy))
				}
			}
		}
	}
	return true
}

// openCell makes a blocked cell crossable: deep water wades, rock is cut
// back to rough stone ground.
func openCell(c *mapgen.Ctx, i int) {
	switch t := c.R.Terrain.Cells[i]; {
	case t == terrain.DeepWater:
		c.R.Terrain.Cells[i] = terrain.ShallowWater
		c.F.Water[i] = mapgen.Shallow
	case terrain.IsRockWall(t):
		cutRock(c, i)
	}
}

// invariants checks I1–I6 (mapgen's Fields/Result doc) and, with fix,
// repairs them; the terrain is the authority the fields are synced to.
// Vault cells may carry stone and the natural roof; water under a bridge
// is sound, but a bridge must stand on bridgeable, non-sea terrain (I6),
// and the Ocean mask on water. It returns one line per broken invariant.
func invariants(c *mapgen.Ctx, fix bool) []string {
	var cnt [7]int
	g := &c.R.Geology
	vault := newOverlay(c).vault
	for i, t := range c.R.Terrain.Cells {
		rock := terrain.IsRockWall(t)
		if c.F.Rock[i] != rock { // I1
			cnt[1]++
			if fix {
				c.F.Rock[i] = rock
			}
		}
		needStone := rock || t == terrain.Stone // I2
		if st := g.Stone[i]; vault[i] && !needStone {
			// a vault's floor and ring keep their stone (or none)
		} else if needStone && st == geology.None {
			cnt[2]++
			if fix {
				g.Stone[i] = stoneAt(c, i)
			}
		} else if !needStone && st != geology.None {
			cnt[2]++
			if fix {
				g.Stone[i] = geology.None
			}
		}
		if !rock && g.Ore[i] != geology.NoOre { // I3
			cnt[3]++
			if fix {
				g.Ore[i] = geology.NoOre
			}
		}
		if c.R.Roof[i] != 0 && !rock && !c.F.Cave[i] && !vault[i] { // I4
			cnt[4]++
			if fix {
				c.R.Roof[i] = 0
			}
		}
		if terrain.IsWater(t) && !c.F.Water[i].Wet() { // I6
			cnt[6]++
			if fix {
				c.F.Water[i] = mapgen.Shallow
				if t == terrain.DeepWater {
					c.F.Water[i] = mapgen.Deep
				}
			}
		}
	}
	cnt[6] += bridgesAndSea(c, fix)
	cnt[5] = features(c, fix)
	var out []string
	for k := 1; k <= 6; k++ {
		if cnt[k] == 0 {
			continue
		}
		if fix {
			out = append(out, fmt.Sprintf("I%d: %d repaired", k, cnt[k]))
		} else {
			out = append(out, fmt.Sprintf("I%d: %d broken", k, cnt[k]))
		}
	}
	return out
}

// bridgesAndSea checks the I6 additions: every bridge on the map, listed
// once, over bridgeable terrain that is not the open sea; the Ocean mask
// only on water. With fix, bad bridges are dropped and the mask cleared.
func bridgesAndSea(c *mapgen.Ctx, fix bool) int {
	bad := 0
	for i, o := range c.R.Ocean {
		if o && i < len(c.R.Terrain.Cells) && !terrain.IsWater(c.R.Terrain.Cells[i]) {
			bad++
			if fix {
				c.R.Ocean[i] = false
			}
		}
	}
	seen := make([]bool, c.W*c.H)
	kept := c.R.Bridges[:0:0]
	for _, b := range c.R.Bridges {
		ok := c.In(b.X, b.Y)
		if ok {
			i := c.Idx(b.X, b.Y)
			ok = !seen[i] && terrain.Bridgeable(c.R.Terrain.Cells[i]) && !(i < len(c.R.Ocean) && c.R.Ocean[i])
			seen[i] = true
		}
		if !ok {
			bad++
			continue
		}
		kept = append(kept, b)
	}
	if fix && bad > 0 {
		c.R.Bridges = kept
	}
	return bad
}

// features checks I5: no vault cell, chunk, ruin piece, vent or plant off
// the map, on water, rock or Track, or on another feature (the earlier one
// wins, in the order vaults, chunks, ruins, vents, plants; a ruin's or
// vault's own pieces may share cells), a vault's caskets, loot and
// defender spots inside its ring, and F.Blocked marks exactly the feature
// and road cells. With fix, offenders are dropped (a vault with a bad
// footprint cell whole) and Blocked is rewritten. It returns the number
// of offending features plus unmarked feature cells.
func features(c *mapgen.Ctx, fix bool) int {
	owner := make([]int32, c.W*c.H)
	next, bad := int32(1), 0
	free := func(x, y int, id int32) bool {
		if !c.In(x, y) {
			return false
		}
		i := c.Idx(x, y)
		if t := c.R.Terrain.Cells[i]; terrain.IsWater(t) || terrain.IsRockWall(t) || t == terrain.Track {
			return false
		}
		return owner[i] == 0 || owner[i] == id
	}
	claim := func(x, y int, id int32) bool {
		if !free(x, y, id) {
			bad++
			return false
		}
		owner[c.Idx(x, y)] = id
		return true
	}

	vaults := c.R.Vaults[:0:0]
	for _, v := range c.R.Vaults {
		id := next
		next++
		sound := v.X0 <= v.X1 && v.Y0 <= v.Y1
		for y := v.Y0; sound && y <= v.Y1; y++ {
			for x := v.X0; sound && x <= v.X1; x++ {
				sound = free(x, y, id)
			}
		}
		if !sound {
			bad++
			continue
		}
		for y := v.Y0; y <= v.Y1; y++ {
			for x := v.X0; x <= v.X1; x++ {
				owner[c.Idx(x, y)] = id
			}
		}
		k := v
		k.Caskets, k.Loot, k.DefenderSpots = nil, nil, nil
		for _, cs := range v.Caskets {
			if v.Inside(cs.X, cs.Y) && v.Inside(cs.X, cs.Y+1) {
				k.Caskets = append(k.Caskets, cs)
			} else {
				bad++
			}
		}
		for _, l := range v.Loot {
			if v.Inside(l.X, l.Y) {
				k.Loot = append(k.Loot, l)
			} else {
				bad++
			}
		}
		for _, p := range v.DefenderSpots {
			if v.Inside(p.X, p.Y) {
				k.DefenderSpots = append(k.DefenderSpots, p)
			} else {
				bad++
			}
		}
		vaults = append(vaults, k)
	}
	chunks := c.R.Chunks[:0:0]
	for _, ch := range c.R.Chunks {
		if claim(ch.X, ch.Y, next) {
			chunks = append(chunks, ch)
		}
		next++
	}
	var ruins []mapgen.Ruin
	for _, r := range c.R.Ruins {
		id := next
		next++
		k := r
		k.Walls, k.Floors, k.Loot, k.Rubble = nil, nil, nil, nil
		for _, w := range r.Walls {
			if claim(w.X, w.Y, id) {
				k.Walls = append(k.Walls, w)
			}
		}
		for _, f := range r.Floors {
			if claim(f.X, f.Y, id) {
				k.Floors = append(k.Floors, f)
			}
		}
		for _, l := range r.Loot {
			if claim(l.X, l.Y, id) {
				k.Loot = append(k.Loot, l)
			}
		}
		for _, ch := range r.Rubble {
			if claim(ch.X, ch.Y, id) {
				k.Rubble = append(k.Rubble, ch)
			}
		}
		if len(k.Walls)+len(k.Floors)+len(k.Loot)+len(k.Rubble) > 0 {
			ruins = append(ruins, k)
		}
	}
	vents := c.R.Vents[:0:0]
	for _, v := range c.R.Vents {
		id := next
		next++
		if !free(v.X, v.Y, id) || !free(v.X+1, v.Y, id) || !free(v.X, v.Y+1, id) || !free(v.X+1, v.Y+1, id) {
			bad++
			continue
		}
		for _, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			owner[c.Idx(v.X+d[0], v.Y+d[1])] = id
		}
		vents = append(vents, v)
	}
	plantBase := next
	plants := c.R.Plants[:0:0]
	for _, p := range c.R.Plants {
		if claim(p.X, p.Y, next) {
			plants = append(plants, p)
		}
		next++
	}
	for i, o := range owner {
		want := c.F.Road[i] || (o != 0 && o < plantBase)
		if want && !c.F.Blocked[i] {
			bad++
		}
		if fix {
			c.F.Blocked[i] = want
		}
	}
	if fix {
		c.R.Chunks, c.R.Ruins, c.R.Vents, c.R.Plants = chunks, ruins, vents, plants
		if len(c.R.Vaults) > 0 {
			c.R.Vaults = vaults
		}
	}
	return bad
}
