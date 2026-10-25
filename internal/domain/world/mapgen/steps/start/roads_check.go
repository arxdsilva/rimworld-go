package start

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/steps/roads"
	"rimworld-go/internal/domain/world/terrain"
)

// overlay holds the cells validation treats apart from their terrain:
// bridges (walkable over water) and vault footprints (sealed walls).
type overlay struct {
	bridge, vault []bool
}

func newOverlay(c *mapgen.Ctx) overlay {
	n := c.W * c.H
	o := overlay{bridge: make([]bool, n), vault: make([]bool, n)}
	for _, b := range c.R.Bridges {
		if c.In(b.X, b.Y) {
			o.bridge[c.Idx(b.X, b.Y)] = true
		}
	}
	for _, v := range c.R.Vaults {
		for y := max(0, v.Y0); y <= min(c.H-1, v.Y1); y++ {
			for x := max(0, v.X0); x <= min(c.W-1, v.X1); x++ {
				o.vault[c.Idx(x, y)] = true
			}
		}
	}
	return o
}

// walk reports a cell a pawn can stand on: walkable terrain or a bridge,
// never a vault (its walls seal it).
func (o overlay) walk(c *mapgen.Ctx, i int) bool {
	return !o.vault[i] && (c.R.Terrain.Cells[i].Walkable() || o.bridge[i])
}

// land is dry walkable ground outside the vaults.
func (o overlay) land(c *mapgen.Ctx, i int) bool { return !o.vault[i] && land(c.R.Terrain.Cells[i]) }

// road reports a passable road cell: a bridge, or Track or a road cell on
// dry walkable ground.
func (o overlay) road(c *mapgen.Ctx, i int) bool {
	if o.bridge[i] {
		return true
	}
	t := c.R.Terrain.Cells[i]
	return (c.F.Road[i] || t == terrain.Track) && land(t) && !o.vault[i]
}

// roadDist is the 4-connected step count over road cells from p (-1 where
// unreached; all -1 when p is not a road cell).
func roadDist(c *mapgen.Ctx, o overlay, p mapgen.Point) []int32 {
	n := c.W * c.H
	src, pass := make([]bool, n), make([]bool, n)
	for i := range pass {
		pass[i] = o.road(c, i)
	}
	if c.In(p.X, p.Y) && pass[c.Idx(p.X, p.Y)] {
		src[c.Idx(p.X, p.Y)] = true
	}
	return mapgen.BFSDist(c.W, c.H, src, pass, false)
}

func joined(c *mapgen.Ctx, d []int32, p mapgen.Point) bool {
	return c.In(p.X, p.Y) && d[c.Idx(p.X, p.Y)] >= 0
}

// BrokenRoads lists the roads (Fields.RoadEnds indices) whose edge point
// no longer joins their junction over road cells, without repairing them.
func BrokenRoads(c *mapgen.Ctx) []int {
	var out []int
	o := newOverlay(c)
	for k, e := range c.F.RoadEnds {
		if !joined(c, roadDist(c, o, e[0]), e[1]) {
			out = append(out, k)
		}
	}
	return out
}

// roadsConnected checks every road still joins its edge point to its
// junction over road cells (after the start repairs and carving). A broken
// road is re-routed from its farthest connected cell with the roads pass's
// own Route and Lay; if no way exists its dangling piece is reverted to
// plain ground and the road dropped from RoadEnds and Roads.
func roadsConnected(c *mapgen.Ctx) []string {
	if len(c.F.RoadEnds) == 0 {
		return nil
	}
	var log []string
	o := newOverlay(c)
	for i := range c.F.Road { // road marks on cells that are no road any more
		if c.F.Road[i] && !o.road(c, i) {
			c.F.Road[i] = false
		}
	}
	keep := make([]bool, len(c.F.RoadEnds))
	for k, e := range c.F.RoadEnds {
		keep[k] = true
		d := roadDist(c, o, e[0])
		if joined(c, d, e[1]) {
			continue
		}
		from := e[0]
		far := int32(-1)
		for i, v := range d {
			if v > far {
				far, from = v, mapgen.Point{X: i % c.W, Y: i / c.W}
			}
		}
		if p := roads.Route(c, from, e[1]); p != nil {
			roads.Lay(c, p)
			o = newOverlay(c)
			if d = roadDist(c, o, e[0]); joined(c, d, e[1]) {
				log = append(log, fmt.Sprintf("road %d re-routed", k+1))
				continue
			}
		}
		n := revertRoad(c, o, d)
		o = newOverlay(c)
		keep[k] = false
		log = append(log, fmt.Sprintf("road %d cannot reach its junction: %d dangling cells reverted to ground", k+1, n))
	}
	if slices.Contains(keep, false) {
		sameLen := len(c.R.Roads) == len(c.F.RoadEnds)
		var ends [][2]mapgen.Point
		var lines [][]mapgen.PointF
		for k, ok := range keep {
			if !ok {
				continue
			}
			ends = append(ends, c.F.RoadEnds[k])
			if sameLen {
				lines = append(lines, c.R.Roads[k])
			}
		}
		c.F.RoadEnds = ends
		if sameLen {
			c.R.Roads = lines
		}
	}
	return log
}

// revertRoad turns the road cells reached in d back into plain ground:
// bridges are removed (the water stays), Track becomes the most common
// plain ground around it (soil if none). It returns the cells reverted.
func revertRoad(c *mapgen.Ctx, o overlay, d []int32) int {
	n := 0
	drop := make([]bool, c.W*c.H)
	for i, v := range d {
		if v < 0 {
			continue
		}
		n++
		c.F.Road[i], c.F.Blocked[i] = false, false
		if o.bridge[i] {
			drop[i] = true
			continue
		}
		if c.R.Terrain.Cells[i] == terrain.Track {
			c.R.Terrain.Cells[i] = plainGround(c, i)
		}
	}
	c.R.Bridges = slices.DeleteFunc(c.R.Bridges, func(b mapgen.Point) bool {
		return c.In(b.X, b.Y) && drop[c.Idx(b.X, b.Y)]
	})
	return n
}

// plainGround is the most common plain ground among cell i's neighbours
// (ties to soil, then the lower terrain id), or soil.
func plainGround(c *mapgen.Ctx, i int) terrain.Type {
	var cnt [256]int
	x, y := c.XY(i)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx == 0 && dy == 0) || !c.In(x+dx, y+dy) {
				continue
			}
			switch t := c.R.Terrain.At(x+dx, y+dy); {
			case t == terrain.Track, t == terrain.Gravel, terrain.IsWater(t), terrain.IsRockWall(t), !t.Walkable():
			default:
				cnt[t]++
			}
		}
	}
	best := terrain.Soil
	for t, v := range cnt {
		if v > 0 && v > cnt[best] {
			best = terrain.Type(t)
		}
	}
	return best
}
