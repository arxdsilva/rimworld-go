// Package roads is the "roads" pass (MAPGEN-2, .cursor/PLAY013_PLAN.md §3.3):
// world roads enter the map at edge points and meet at a junction near the
// middle. Each road is routed by A* over a terrain cost grid, pulled
// straight where that costs nothing, smoothed (Chaikin corner cutting) and
// rasterised as a two-cell wide track with soft gravel edges.
package roads

import (
	"container/heap"
	"fmt"
	"math"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// Cell costs in half units (open ground = 2); impassable = -1.
const (
	costOpen    = 2
	costRough   = 3
	costSoft    = 8
	costWater   = 24 // river or lake water, crossed on a bridge
	costBlocked = 12
	costRock    = 50

	diagStep  = 14 // tenths per diagonal step ...
	diagWater = 22 // ... and per diagonal step into unbridged water, so crossings run square to the flow
	thickRock = 7  // rock this deep is cut only when no other way exists

	pullWindow   = 80 // longest straight shortcut tried, in path steps
	smoothRounds = 3
	coreRadius   = 1.0
	edgeRadius   = 1.6
)

// Roads is the "roads" pass: writes Road, RoadEnds, Roads, Bridges, Track
// terrain, Blocked.
type Roads struct{}

// Name is the step's stream and label key.
func (Roads) Name() string { return "roads" }

type router struct {
	c      *mapgen.Ctx
	w, h   int
	cost   []int16
	wet    []bool // unbridged river or lake water (crossings are bridged)
	g      []int32
	prev   []int32
	cut    []bool // rock cells the current road's straight lines cross
	dist   []float32
	touch  []int32
	bridge []bool // cells already in Result.Bridges
	vault  []bool // vault footprints: never crossed
	thick  bool   // the last strict cost fill left thick rock impassable
}

func newRouter(c *mapgen.Ctx) *router {
	n := c.W * c.H
	rt := &router{c: c, w: c.W, h: c.H, cost: make([]int16, n), wet: make([]bool, n), g: make([]int32, n),
		prev: make([]int32, n), cut: make([]bool, n), dist: make([]float32, n),
		bridge: make([]bool, n), vault: make([]bool, n)}
	for i := range rt.dist {
		rt.dist[i] = float32(math.Inf(1))
	}
	for _, p := range c.R.Bridges {
		if c.In(p.X, p.Y) {
			rt.bridge[c.Idx(p.X, p.Y)] = true
		}
	}
	for _, v := range c.R.Vaults {
		for y := max(0, v.Y0); y <= min(c.H-1, v.Y1); y++ {
			for x := max(0, v.X0); x <= min(c.W-1, v.X1); x++ {
				rt.vault[c.Idx(x, y)] = true
			}
		}
	}
	return rt
}

// Run is the pass.
func (Roads) Run(c *mapgen.Ctx) error {
	s := c.Cfg.Site
	if s.Road == 0 || c.W < 2 || c.H < 2 {
		return nil
	}
	rt := newRouter(c)
	r := c.Rand("roads")
	rt.fillCost(true)
	junction := rt.junction()
	n := 0
	for d := site.Dir8(0); d < 8; d++ {
		if !s.Road.Has(d) {
			continue
		}
		u := 2*r.Float64() - 1 // always drawn: a road's shift never depends on others
		start, ok := rt.endpoint(d, u)
		if !ok {
			continue
		}
		n++
		path := rt.route(int32(start), int32(junction))
		if path == nil {
			c.R.Metrics.Repairs = append(c.R.Metrics.Repairs, fmt.Sprintf("roads: road %d (direction %d) cannot reach the junction", n, d))
			continue
		}
		c.R.Roads = append(c.R.Roads, rt.lay(path))
		c.F.RoadEnds = append(c.F.RoadEnds, [2]mapgen.Point{rt.pt(int32(start)), rt.pt(int32(junction))})
	}
	return nil
}

// Route is the road A* from one cell to another over the map as it is now:
// rock is cut, river and lake water bridged, the open sea and vaults never
// crossed. Thick rock is cut only when there is no other way. It returns
// the cells from from to to (8-connected), or nil when no way exists.
func Route(c *mapgen.Ctx, from, to mapgen.Point) []mapgen.Point {
	if !c.In(from.X, from.Y) || !c.In(to.X, to.Y) {
		return nil
	}
	rt := newRouter(c)
	path := rt.route(int32(c.Idx(from.X, from.Y)), int32(c.Idx(to.X, to.Y)))
	if path == nil {
		return nil
	}
	out := make([]mapgen.Point, len(path))
	for k, i := range path {
		out[k] = rt.pt(i)
	}
	return out
}

// Lay lays a routed path down as road: water crossings straightened into
// 2-wide wooden bridges between dry banks, the rest smoothed and
// rasterised as Track with soft gravel edges. Off-map cells are ignored.
func Lay(c *mapgen.Ctx, path []mapgen.Point) {
	cells := make([]int32, 0, len(path))
	for _, p := range path {
		if c.In(p.X, p.Y) {
			cells = append(cells, int32(c.Idx(p.X, p.Y)))
		}
	}
	if len(cells) == 0 {
		return
	}
	rt := newRouter(c)
	rt.fillCost(false)
	rt.lay(cells)
}

func (rt *router) pt(i int32) mapgen.Point { return mapgen.Point{X: int(i) % rt.w, Y: int(i) / rt.w} }

// route is A* with thick rock impassable, then, if that fails, with every
// rock cut at the rock cost.
func (rt *router) route(a, b int32) []int32 {
	rt.fillCost(true)
	if p := rt.astar(int(a), int(b)); p != nil || !rt.thick {
		return p
	}
	rt.fillCost(false)
	return rt.astar(int(a), int(b))
}

func (rt *router) isRock(i int) bool {
	c := rt.c
	return c.F.Rock[i] || terrain.IsRockWall(c.R.Terrain.At(i%rt.w, i/rt.w))
}

// isOcean reports open-sea water (the coast), which is never crossed.
func (rt *router) isOcean(i int) bool {
	c := rt.c
	return (i < len(c.R.Ocean) && c.R.Ocean[i]) || (c.F.Coast[i] && c.F.Water[i].Wet())
}

func (rt *router) isWater(i int) bool {
	c := rt.c
	return c.F.Water[i].Wet() || terrain.IsWater(c.R.Terrain.At(i%rt.w, i/rt.w))
}

// fillCost prices every cell; strict leaves thick rock impassable.
func (rt *router) fillCost(strict bool) {
	c := rt.c
	rt.thick = false
	for i := range rt.cost {
		var k int16
		t := c.R.Terrain.At(i%rt.w, i/rt.w)
		water := rt.isWater(i)
		rt.wet[i] = water && !c.F.Road[i]
		switch {
		case rt.vault[i] || (water && rt.isOcean(i)):
			k = -1
		case rt.isRock(i):
			k = costRock
			if strict && c.F.RockDepth[i] >= thickRock {
				k, rt.thick = -1, true
			}
		case c.F.Road[i]:
			k = costOpen
		case c.F.Blocked[i]:
			k = costBlocked
		case water:
			k = costWater
		case t == terrain.Marsh || t == terrain.Mud || t == terrain.Sand:
			k = costSoft
		case t == terrain.Gravel || t == terrain.Stone || t == terrain.Ice:
			k = costRough
		default:
			k = costOpen
		}
		rt.cost[i] = k
	}
}

// junction is the best walkable dry cell near the map centre, preferring
// open soil and cells away from rock.
func (rt *router) junction() int {
	c := rt.c
	cx, cy := rt.w/2, rt.h/2
	rad := max(2, int(math.Round(20*c.Scale())))
	rad = min(rad, min(rt.w, rt.h)/2)
	best, bestScore := -1, math.Inf(1)
	for y := cy - rad; y <= cy+rad; y++ {
		for x := cx - rad; x <= cx+rad; x++ {
			if !c.In(x, y) {
				continue
			}
			i := c.Idx(x, y)
			if rt.isRock(i) || rt.isWater(i) || c.F.Blocked[i] {
				continue
			}
			dx, dy := float64(x-cx), float64(y-cy)
			dd := math.Hypot(dx, dy)
			if dd > float64(rad) {
				continue
			}
			sc := 0.5 * dd / float64(rad)
			switch c.R.Terrain.At(x, y) {
			case terrain.Soil, terrain.RichSoil, terrain.Track:
			case terrain.Gravel, terrain.Stone:
				sc += 0.3
			default:
				sc += 1
			}
			for _, o := range [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
				if c.In(x+o[0], y+o[1]) && rt.isRock(c.Idx(x+o[0], y+o[1])) {
					sc += 0.15
				}
			}
			if sc < bestScore {
				best, bestScore = i, sc
			}
		}
	}
	if best < 0 {
		best = c.Idx(cx, cy)
	}
	return best
}

// endpoint is the edge cell where road direction d leaves the map, shifted
// along the edge by u*15% of its length, then moved to the nearest dry cell
// outside any vault.
func (rt *router) endpoint(d site.Dir8, u float64) (int, bool) {
	w, h := rt.w, rt.h
	alongX := func(frac float64, y int) (int, int, bool, int) { // horizontal edge
		return int(math.Round((frac + 0.15*u) * float64(w-1))), y, true, w
	}
	alongY := func(frac float64, x int) (int, int, bool, int) { // vertical edge
		return x, int(math.Round((frac + 0.15*u) * float64(h-1))), false, h
	}
	var x, y int
	var horiz bool
	var length int
	switch d {
	case 0:
		x, y, horiz, length = alongX(0.5, 0)
	case 1:
		x, y, horiz, length = alongX(0.85, 0)
	case 2:
		x, y, horiz, length = alongY(0.5, w-1)
	case 3:
		x, y, horiz, length = alongX(0.85, h-1)
	case 4:
		x, y, horiz, length = alongX(0.5, h-1)
	case 5:
		x, y, horiz, length = alongX(0.15, h-1)
	case 6:
		x, y, horiz, length = alongY(0.5, 0)
	default:
		x, y, horiz, length = alongX(0.15, 0)
	}
	pos := x
	if !horiz {
		pos = y
	}
	pos = min(max(pos, 0), length-1)
	for off := 0; off < length; off++ {
		for _, p := range [2]int{pos + off, pos - off} {
			if p < 0 || p >= length {
				continue
			}
			cx, cy := p, y
			if !horiz {
				cx, cy = x, p
			}
			if i := cy*w + cx; !rt.isWater(i) && !rt.vault[i] {
				return i, true
			}
		}
	}
	return 0, false
}

type node struct{ f, g, i int32 }
type pq []node

func (q pq) Len() int { return len(q) }
func (q pq) Less(a, b int) bool {
	if q[a].f != q[b].f {
		return q[a].f < q[b].f
	}
	if q[a].g != q[b].g {
		return q[a].g > q[b].g
	}
	return q[a].i < q[b].i
}
func (q pq) Swap(a, b int) { q[a], q[b] = q[b], q[a] }
func (q *pq) Push(x any)   { *q = append(*q, x.(node)) }
func (q *pq) Pop() any {
	o := *q
	v := o[len(o)-1]
	*q = o[:len(o)-1]
	return v
}

var nbrs = [8][3]int{{0, -1, 10}, {1, 0, 10}, {0, 1, 10}, {-1, 0, 10}, {1, -1, diagStep}, {1, 1, diagStep}, {-1, 1, diagStep}, {-1, -1, diagStep}}

// astar routes from cell a to cell b (8-connected) and returns the cells.
func (rt *router) astar(a, b int) []int32 {
	w, h := rt.w, rt.h
	const inf = int32(math.MaxInt32)
	for i := range rt.g {
		rt.g[i] = inf
		rt.prev[i] = -1
	}
	bx, by := b%w, b/w
	hf := func(i int) int32 {
		dx, dy := i%w-bx, i/w-by
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		return int32(2 * (10*max(dx, dy) + 4*min(dx, dy)))
	}
	q := &pq{}
	rt.g[a] = 0
	heap.Push(q, node{hf(a), 0, int32(a)})
	for q.Len() > 0 {
		cur := heap.Pop(q).(node)
		i := int(cur.i)
		if cur.g > rt.g[i] {
			continue
		}
		if i == b {
			var path []int32
			for k := int32(i); k >= 0; k = rt.prev[k] {
				path = append(path, k)
			}
			for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
				path[l], path[r] = path[r], path[l]
			}
			return path
		}
		x, y := i%w, i/w
		for _, nb := range nbrs {
			nx, ny := x+nb[0], y+nb[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			j := ny*w + nx
			k := rt.cost[j]
			if k < 0 {
				continue
			}
			step := int32(nb[2])
			if step == diagStep {
				if rt.cost[y*w+nx] < 0 || rt.cost[ny*w+x] < 0 {
					continue // no squeezing diagonally between impassable cells
				}
				if rt.wet[j] {
					step = diagWater
				}
			}
			ng := cur.g + int32(k)*step
			if ng < rt.g[j] {
				rt.g[j] = ng
				rt.prev[j] = int32(i)
				heap.Push(q, node{ng + hf(j), ng, int32(j)})
			}
		}
	}
	return nil
}

// line walks the Bresenham cells from a (excluded) to b; it returns their
// summed cost, or -1 if one is impassable or unbridged water, or a
// diagonal step squeezes between impassable cells. out receives the cells.
func (rt *router) line(a, b int, out []int32) (int, []int32) {
	w := rt.w
	x0, y0, x1, y1 := a%w, a/w, b%w, b/w
	dx, dy := x1-x0, y1-y0
	sx, sy := 1, 1
	if dx < 0 {
		dx, sx = -dx, -1
	}
	if dy < 0 {
		dy, sy = -dy, -1
	}
	err := dx - dy
	sum := 0
	out = out[:0]
	for x0 != x1 || y0 != y1 {
		e2 := 2 * err
		px, py := x0, y0
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
		k := rt.cost[y0*w+x0]
		if k < 0 || rt.wet[y0*w+x0] {
			return -1, out
		}
		if x0 != px && y0 != py && (rt.cost[py*w+x0] < 0 || rt.cost[y0*w+px] < 0) {
			return -1, out
		}
		sum += int(k)
		out = append(out, int32(y0*w+x0))
	}
	return sum, out
}

// smooth pulls the raw path straight where that is no dearer, then rounds
// its corners; it returns the centreline in cell units.
func (rt *router) smooth(path []int32) []mapgen.PointF {
	w := rt.w
	pre := make([]int, len(path))
	for k := 1; k < len(path); k++ {
		pre[k] = pre[k-1] + int(rt.cost[path[k]])
	}
	mark := func(cells []int32) {
		for _, i := range cells {
			if rt.isRock(int(i)) {
				rt.cut[i] = true
			}
		}
	}
	mark(path[:1])
	verts := []int32{path[0]}
	var scratch, best []int32
	for i := 0; i < len(path)-1; {
		j := min(len(path)-1, i+pullWindow)
		accepted := i + 1
		best = append(best[:0], path[i+1])
		for ; j > i+1; j-- {
			sum, cells := rt.line(int(path[i]), int(path[j]), scratch)
			scratch = cells
			if sum >= 0 && sum <= pre[j]-pre[i] {
				accepted = j
				best = append(best[:0], cells...)
				break
			}
		}
		mark(best)
		verts = append(verts, path[accepted])
		i = accepted
	}
	pts := make([]mapgen.PointF, 0, len(verts))
	for _, v := range verts {
		p := mapgen.PointF{X: float64(int(v)%w) + 0.5, Y: float64(int(v)/w) + 0.5}
		if m := len(pts); m >= 2 && collinear(pts[m-2], pts[m-1], p) {
			pts[m-1] = p
			continue
		}
		pts = append(pts, p)
	}
	for r := 0; r < smoothRounds && len(pts) > 2; r++ {
		next := chaikin(pts)
		if !rt.curveOK(next) {
			break
		}
		pts = next
	}
	return pts
}

func collinear(a, b, c mapgen.PointF) bool {
	return math.Abs((b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X)) < 1e-9
}

// chaikin cuts every corner at 1/4 and 3/4 of its segments, keeping both
// ends fixed.
func chaikin(p []mapgen.PointF) []mapgen.PointF {
	out := []mapgen.PointF{p[0]}
	last := len(p) - 2
	for i := 0; i <= last; i++ {
		a, b := p[i], p[i+1]
		if i > 0 {
			out = append(out, mapgen.PointF{X: 0.75*a.X + 0.25*b.X, Y: 0.75*a.Y + 0.25*b.Y})
		}
		if i < last {
			out = append(out, mapgen.PointF{X: 0.25*a.X + 0.75*b.X, Y: 0.25*a.Y + 0.75*b.Y})
		}
	}
	return append(out, p[len(p)-1])
}

// curveOK reports whether the centreline stays off impassable cells,
// unbridged water and rock the straight lines did not already cut.
func (rt *router) curveOK(p []mapgen.PointF) bool {
	for i := 0; i+1 < len(p); i++ {
		a, b := p[i], p[i+1]
		steps := int(math.Ceil(math.Hypot(b.X-a.X, b.Y-a.Y)*2)) + 1
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			x, y := int(math.Floor(a.X+(b.X-a.X)*t)), int(math.Floor(a.Y+(b.Y-a.Y)*t))
			if x < 0 || y < 0 || x >= rt.w || y >= rt.h {
				return false
			}
			k := y*rt.w + x
			if rt.cost[k] < 0 || rt.wet[k] || (rt.isRock(k) && !rt.cut[k]) {
				return false
			}
		}
	}
	return true
}

func segDist(px, py float64, a, b mapgen.PointF) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((px-a.X)*dx+(py-a.Y)*dy)/l2))
	}
	return math.Hypot(px-(a.X+t*dx), py-(a.Y+t*dy))
}

// rasterise lays centrelines down as track with soft edges. Water is
// skipped, unless bridgeWater, when water within the core radius becomes
// bridge (a crossing no straight strip fitted). changed collects the cells
// whose ground changed.
func (rt *router) rasterise(lines [][]mapgen.PointF, bridgeWater bool, changed map[int]bool) {
	c := rt.c
	w, h := rt.w, rt.h
	rt.touch = rt.touch[:0]
	pad := edgeRadius + 0.5
	one := func(a, b mapgen.PointF) {
		x0 := max(0, int(math.Floor(math.Min(a.X, b.X)-pad)))
		x1 := min(w-1, int(math.Ceil(math.Max(a.X, b.X)+pad)))
		y0 := max(0, int(math.Floor(math.Min(a.Y, b.Y)-pad)))
		y1 := min(h-1, int(math.Ceil(math.Max(a.Y, b.Y)+pad)))
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				d := float32(segDist(float64(x)+0.5, float64(y)+0.5, a, b))
				if d > edgeRadius {
					continue
				}
				i := y*w + x
				if math.IsInf(float64(rt.dist[i]), 1) {
					rt.touch = append(rt.touch, int32(i))
				}
				if d < rt.dist[i] {
					rt.dist[i] = d
				}
			}
		}
	}
	for _, line := range lines {
		if len(line) == 1 {
			one(line[0], line[0])
		}
		for i := 0; i+1 < len(line); i++ {
			one(line[i], line[i+1])
		}
	}
	// Rock the straight lines cut, and its neighbours, may be cut wide.
	near := func(i int) bool {
		x, y := i%w, i/w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if c.In(x+dx, y+dy) && rt.cut[(y+dy)*w+x+dx] {
					return true
				}
			}
		}
		return false
	}
	seed := c.NoiseSeed("roads/edge")
	for _, ti := range rt.touch {
		i := int(ti)
		d := float64(rt.dist[i])
		rt.dist[i] = float32(math.Inf(1))
		x, y := i%w, i/w
		rock := rt.isRock(i)
		core := d <= coreRadius
		water := rt.isWater(i)
		switch {
		case rt.cost[i] < 0 && !rock:
			continue // the sea, a vault
		case rock && (!core || !near(i)):
			continue
		case water && (!core || !bridgeWater):
			continue
		}
		if c.F.Road[i] {
			continue // an earlier road's track is never downgraded
		}
		if water {
			rt.addBridge(i)
			continue
		}
		asTrack := core || noise2.Gradient(seed, float64(x), float64(y), 3) > 0.5
		rt.ground(i, asTrack)
		changed[i] = true
	}
}

// ground turns cell i into Track (a road cell) or a gravel edge, cutting
// rock and clearing stone and ore.
func (rt *router) ground(i int, asTrack bool) {
	c := rt.c
	x, y := i%rt.w, i/rt.w
	if rt.isRock(i) {
		c.F.Rock[i] = false
		c.R.Roof[i] = roof.Cell(0)
		c.F.RockDepth[i] = 0
	}
	c.R.Geology.Stone[i] = geology.None
	c.R.Geology.Ore[i] = geology.NoOre
	if asTrack {
		c.R.Terrain.Set(x, y, terrain.Track)
		c.F.Road[i] = true
	} else {
		c.R.Terrain.Set(x, y, terrain.Gravel)
	}
	c.F.Blocked[i] = true
}

// addBridge makes cell i an old wooden bridge: a road cell over
// unchanged water, marsh or mud.
func (rt *router) addBridge(i int) {
	c := rt.c
	c.F.Road[i], c.F.Blocked[i] = true, true
	rt.wet[i] = false
	if !rt.bridge[i] {
		rt.bridge[i] = true
		c.R.Bridges = append(c.R.Bridges, rt.pt(int32(i)))
	}
}

// dropChunks removes the chunks on changed cells.
func (rt *router) dropChunks(changed map[int]bool) {
	c := rt.c
	if len(changed) == 0 {
		return
	}
	keep := c.R.Chunks[:0:0]
	for _, ch := range c.R.Chunks {
		if !changed[ch.Y*rt.w+ch.X] {
			keep = append(keep, ch)
		}
	}
	c.R.Chunks = keep
}
