// Package start holds the landing-spot pass ("start") and the final
// validation pass ("validate") of MAPGEN-2 (.cursor/PLAY013_PLAN.md §4).
package start

import (
	"fmt"
	"math"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
)

// Start scoring and repair (our numbers).
const (
	coarseStep  = 4  // candidate spacing of the coarse pass
	refineWin   = 16 // the best coarse candidate's window, scored cell by cell
	edgeMargin  = 25 // cells from every edge (side/edgeDiv on small maps)
	edgeDiv     = 5
	openRadius  = 12   // open buildable land is counted within this radius
	openNeed    = 0.70 // ... and must be at least this share
	fertRadius  = 20   // fertile ground (the wood and food proxy)
	stoneRadius = 40   // rock or chunks to quarry
	stoneEnough = 30   // stone cells for a full stone score
	hazardRad   = 4    // no deep water or rock this close
	roadNear    = 25   // full road score within this distance ...
	roadFar     = 50   // ... fading to none here
	rockRepair  = 6    // a failing spot has its rock cut back this far
	waterRepair = 4    // and its water dried this far
	chunkClear  = 12   // chunks this close to the start are removed

	wOpen, wFert, wStone, wCentre, wRoad = 0.40, 0.20, 0.15, 0.15, 0.10
)

// Start is the "start" pass: it scores landing spots (StartScore), picks
// the best (Start) and, when even the best breaks a hard rule, repairs its
// surroundings locally. Chunks near the start are cleared.
type Start struct{}

// Name is the step's stream and label key.
func (Start) Name() string { return "start" }

// Run is the pass.
func (Start) Run(c *mapgen.Ctx) error {
	sc := newScorer(c)
	r := c.Rand("start")
	mx, my := min(edgeMargin, c.W/edgeDiv), min(edgeMargin, c.H/edgeDiv)
	x0, x1, y0, y1 := mx, c.W-1-mx, my, c.H-1-my

	clear(c.F.StartScore)
	best := cand{score: -1}
	consider := func(x, y int) cand {
		k := cand{x: x, y: y, tie: r.Float64()}
		k.score, k.pass = sc.eval(x, y)
		if k.better(best) {
			best = k
		}
		return k
	}
	// coarse pass: every coarseStep cells, each value spread to its block
	for y := y0; y <= y1; y += coarseStep {
		for x := x0; x <= x1; x += coarseStep {
			k := consider(x, y)
			for by := max(0, y-coarseStep/2); by < min(c.H, y+coarseStep/2); by++ {
				for bx := max(0, x-coarseStep/2); bx < min(c.W, x+coarseStep/2); bx++ {
					c.F.StartScore[c.Idx(bx, by)] = float32(k.score)
				}
			}
		}
	}
	// refinement: the window around the best coarse candidate, cell by cell
	bx, by := best.x, best.y
	for y := max(y0, by-refineWin/2); y <= min(y1, by+refineWin/2-1); y++ {
		for x := max(x0, bx-refineWin/2); x <= min(x1, bx+refineWin/2-1); x++ {
			k := consider(x, y)
			c.F.StartScore[c.Idx(x, y)] = float32(k.score)
		}
	}
	c.R.Start = mapgen.Point{X: best.x, Y: best.y}

	if !best.pass {
		if n := openRock(c, c.R.Start, rockRepair); n > 0 {
			c.R.Metrics.Repairs = append(c.R.Metrics.Repairs, fmt.Sprintf("start: %d rock cells cut to stone ground", n))
		}
	}
	if n := dryWater(c, c.R.Start, waterRepair); n > 0 {
		c.R.Metrics.Repairs = append(c.R.Metrics.Repairs, fmt.Sprintf("start: %d water cells dried to mud", n))
	}
	clearChunks(c, c.R.Start, chunkClear)
	return nil
}

// cand is a scored candidate; tie is its seeded tie-break key.
type cand struct {
	x, y       int
	score, tie float64
	pass       bool
}

func (k cand) better(o cand) bool {
	if k.score != o.score {
		return k.score > o.score
	}
	return k.tie > o.tie
}

// scorer holds the row prefix sums the disk counts read.
type scorer struct {
	c                         *mapgen.Ctx
	open, fert, stone, hazard rowSums
	reach                     []bool  // walkable cells joined to a map edge
	roadDist                  []int32 // Chebyshev cells to a road (nil: no road)
	cx, cy, maxCentre         float64
	spOpen, spFert, spStone   []int // disk row half-widths per radius
	spHazard                  []int
}

func newScorer(c *mapgen.Ctx) *scorer {
	n := c.W * c.H
	open, fert, stone, hazard := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	anyRoad := false
	for i, t := range c.R.Terrain.Cells {
		rock := c.F.Rock[i] || terrain.IsRockWall(t)
		if buildable(c, i) {
			open[i] = 1
		}
		if t.Walkable() && !terrain.IsWater(t) && t.Fertility() > 0 {
			fert[i] = float64(c.F.Fertility[i])
		}
		if rock {
			stone[i] = 1
		}
		if rock || t == terrain.DeepWater {
			hazard[i] = 1
		}
		anyRoad = anyRoad || c.F.Road[i]
	}
	for _, ch := range c.R.Chunks {
		if c.In(ch.X, ch.Y) {
			stone[c.Idx(ch.X, ch.Y)] = 1
		}
	}
	s := &scorer{
		c: c, open: newRowSums(c.W, c.H, open), fert: newRowSums(c.W, c.H, fert),
		stone: newRowSums(c.W, c.H, stone), hazard: newRowSums(c.W, c.H, hazard),
		reach: edgeReach(c),
		cx:    float64(c.W-1) / 2, cy: float64(c.H-1) / 2,
		spOpen: diskSpan(openRadius), spFert: diskSpan(fertRadius),
		spStone: diskSpan(stoneRadius), spHazard: diskSpan(hazardRad),
	}
	s.maxCentre = math.Max(1, math.Hypot(s.cx, s.cy))
	if anyRoad {
		s.roadDist = mapgen.BFSDist(c.W, c.H, c.F.Road, nil, true)
	}
	return s
}

// buildable is open buildable land: walkable, dry, firm, unroofed and free.
func buildable(c *mapgen.Ctx, i int) bool {
	t := c.R.Terrain.Cells[i]
	if !t.Walkable() || terrain.IsWater(t) || c.F.Rock[i] || c.F.Blocked[i] || c.F.Cave[i] || c.R.Roof[i] != 0 {
		return false
	}
	switch t {
	case terrain.Marsh, terrain.Mud, terrain.Track:
		return false
	}
	return true
}

// eval scores (x, y) in [0, 1]: passing spots land in [0.5, 1], failing
// ones in [0, 0.5), so any passing spot beats every failing one.
func (s *scorer) eval(x, y int) (float64, bool) {
	c := s.c
	openN, cells := s.open.disk(x, y, s.spOpen)
	openShare := openN / max(1, cells)
	fertSum, fcells := s.fert.disk(x, y, s.spFert)
	fert := fertSum / max(1, fcells)
	stoneN, _ := s.stone.disk(x, y, s.spStone)
	stone := math.Min(1, stoneN/stoneEnough)
	haz, _ := s.hazard.disk(x, y, s.spHazard)
	centre := 1 - math.Hypot(float64(x)-s.cx, float64(y)-s.cy)/s.maxCentre

	raw := wOpen*openShare + wFert*fert + wStone*stone + wCentre*centre
	if s.roadDist != nil {
		road := 0.0
		if d := s.roadDist[c.Idx(x, y)]; d >= 0 {
			road = 1 - mapgen.Smoothstep(roadNear, roadFar, float64(d))
		}
		raw += wRoad * road
	} else {
		raw /= 1 - wRoad
	}
	pass := openShare >= openNeed && haz == 0 && s.reach[c.Idx(x, y)]
	score := 0.5 * mapgen.Clamp01(raw)
	if pass {
		score += 0.5
	}
	return math.Min(score, 1), pass
}

// diskSpan is the half-width of each disk row (dy = -r..r).
func diskSpan(r int) []int {
	sp := make([]int, 2*r+1)
	for dy := -r; dy <= r; dy++ {
		sp[dy+r] = int(math.Sqrt(float64(r*r - dy*dy)))
	}
	return sp
}

// rowSums is a per-row prefix sum: disk sums cost one subtraction per row.
type rowSums struct {
	w, h int
	p    []float64 // (w+1) per row
}

func newRowSums(w, h int, v []float64) rowSums {
	p := make([]float64, (w+1)*h)
	for y := range h {
		row := p[y*(w+1):]
		for x := range w {
			row[x+1] = row[x] + v[y*w+x]
		}
	}
	return rowSums{w: w, h: h, p: p}
}

// disk sums the disk around (x, y) with row half-widths span, clipped to
// the map; cells is the in-map cell count.
func (s rowSums) disk(x, y int, span []int) (sum, cells float64) {
	r := len(span) / 2
	for dy := -r; dy <= r; dy++ {
		yy := y + dy
		if yy < 0 || yy >= s.h {
			continue
		}
		a, b := max(0, x-span[dy+r]), min(s.w-1, x+span[dy+r])
		if a > b {
			continue
		}
		row := s.p[yy*(s.w+1):]
		sum += row[b+1] - row[a]
		cells += float64(b - a + 1)
	}
	return sum, cells
}

// edgeReach marks the walkable cells joined (4-connected) to a map edge.
func edgeReach(c *mapgen.Ctx) []bool {
	walk := make([]bool, c.W*c.H)
	src := make([]bool, c.W*c.H)
	for i, t := range c.R.Terrain.Cells {
		walk[i] = t.Walkable()
		x, y := c.XY(i)
		src[i] = walk[i] && (x == 0 || y == 0 || x == c.W-1 || y == c.H-1)
	}
	d := mapgen.BFSDist(c.W, c.H, src, walk, false)
	out := make([]bool, len(d))
	for i, v := range d {
		out[i] = v >= 0
	}
	return out
}

// disk calls f for every in-map cell within Euclidean radius r of p.
func disk(c *mapgen.Ctx, p mapgen.Point, r int, f func(i int)) {
	for y := max(0, p.Y-r); y <= min(c.H-1, p.Y+r); y++ {
		for x := max(0, p.X-r); x <= min(c.W-1, p.X+r); x++ {
			if dx, dy := x-p.X, y-p.Y; dx*dx+dy*dy <= r*r {
				f(c.Idx(x, y))
			}
		}
	}
}

// openRock cuts every rock cell within r of p to rough stone ground.
func openRock(c *mapgen.Ctx, p mapgen.Point, r int) int {
	n := 0
	disk(c, p, r, func(i int) {
		if c.F.Rock[i] || terrain.IsRockWall(c.R.Terrain.Cells[i]) {
			cutRock(c, i)
			n++
		}
	})
	return n
}

// cutRock turns a rock cell into rough stone ground, keeping its stone.
func cutRock(c *mapgen.Ctx, i int) {
	c.R.Terrain.Cells[i] = terrain.Stone
	c.F.Rock[i] = false
	c.F.RockDepth[i] = 0
	if c.R.Geology.Stone[i] == geology.None {
		c.R.Geology.Stone[i] = stoneAt(c, i)
	}
	c.R.Geology.Ore[i] = geology.NoOre
	c.R.Roof[i] = 0
}

// dryWater turns every water cell within r of p into mud.
func dryWater(c *mapgen.Ctx, p mapgen.Point, r int) int {
	n := 0
	disk(c, p, r, func(i int) {
		if terrain.IsWater(c.R.Terrain.Cells[i]) || c.F.Water[i].Wet() {
			c.R.Terrain.Cells[i] = terrain.Mud
			c.F.Water[i] = mapgen.Dry
			c.R.Geology.Stone[i], c.R.Geology.Ore[i], c.R.Roof[i] = geology.None, geology.NoOre, 0
			n++
		}
	})
	return n
}

// clearChunks removes the chunks within r of p and frees their cells.
func clearChunks(c *mapgen.Ctx, p mapgen.Point, r int) {
	kept := c.R.Chunks[:0]
	for _, ch := range c.R.Chunks {
		if dx, dy := ch.X-p.X, ch.Y-p.Y; dx*dx+dy*dy > r*r {
			kept = append(kept, ch)
			continue
		}
		if c.In(ch.X, ch.Y) && !c.F.Road[c.Idx(ch.X, ch.Y)] {
			c.F.Blocked[c.Idx(ch.X, ch.Y)] = false
		}
	}
	c.R.Chunks = kept
}

// stoneAt is the stone a rock or stone-ground cell should carry: the
// material field, else the site's first stone, else granite.
func stoneAt(c *mapgen.Ctx, i int) geology.Stone {
	if s := c.F.Stone[i]; s != geology.None {
		return s
	}
	for _, s := range c.Cfg.Site.Stones {
		if s != geology.None {
			return s
		}
	}
	return geology.Granite
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
