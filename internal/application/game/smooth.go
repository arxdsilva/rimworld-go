package game

// Smoothing (SIM-029): a builder works a natural rock wall into a smoothed
// wall, or rough stone ground into a smooth floor. The stone type stays.
// Marks live in g.smooths (saved); the work runs under Construction after
// frames, repairs and roofs, like mining with its own designation.

import (
	"slices"
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

// SmoothWallWork and SmoothFloorWork are the labour to smooth one tile.
const (
	SmoothWallWork  = 480
	SmoothFloorWork = 240
)

// smoothable reports whether (x, y) can be smoothed now: rough rock with
// no ore, or rough stone ground with no building on it.
func (g *Game) smoothable(x, y int) bool {
	if !g.grid.InBounds(x, y) {
		return false
	}
	switch g.terr.At(x, y) {
	case terrain.Rock:
		return g.geo.OreAt(x, y) == geology.NoOre
	case terrain.Stone:
		return g.layerBldgAt(x, y, construction.LayerFloor) == 0 && g.layerBldgAt(x, y, construction.LayerObject) == 0
	}
	return false
}

// smoothWork is the labour to smooth (x, y).
func (g *Game) smoothWork(x, y int) int {
	if terrain.IsRockWall(g.terr.At(x, y)) {
		return SmoothWallWork
	}
	return SmoothFloorWork
}

// markSmooth (un)marks smoothable tiles in the rectangle.
func (g *Game) markSmooth(x0, y0, x1, y1 int, on bool) {
	minX, minY, maxX, maxY := normalizeRect(x0, y0, x1, y1)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			k := [2]int{x, y}
			if on && g.smoothable(x, y) && !g.fogged(x, y) {
				g.smooths[k] = true
			} else if !on {
				delete(g.smooths, k)
			}
		}
	}
}

// smoothList is the smoothing designations, sorted y then x.
func (g *Game) smoothList() [][2]int {
	out := make([][2]int, 0, len(g.smooths))
	for k := range g.smooths {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][1] != out[j][1] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

// assignSmooth smooths the nearest marked tile, trying the next nearest
// when one can't be started (walled in, postponed: SIM-027), at most
// workTries tiles. Ties go to the earlier tile in smoothList order.
func (g *Game) assignSmooth(id entity.EntityID) bool {
	if len(g.smooths) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	list := g.smoothList()
	var tried [][2]int
	for range workTries {
		bestD := 1 << 30
		var best [2]int
		found := false
		for _, k := range list {
			if _, taken := g.reserve.TileOwner(k[0], k[1]); taken {
				continue
			}
			d := pathing.Dist(pos.X, pos.Y, k[0], k[1])
			if d >= bestD || slices.Contains(tried, k) || !g.smoothable(k[0], k[1]) ||
				g.postponedFor(id, job.Smooth, 0, k[0], k[1]) {
				continue
			}
			best, bestD, found = k, d, true
		}
		if !found {
			return false
		}
		if g.startSmooth(id, best[0], best[1], false) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

// smoothSpot is where a builder stands to smooth (x, y): beside a wall,
// on a floor tile itself.
func (g *Game) smoothSpot(id entity.EntityID, x, y int) (int, int, bool) {
	if terrain.IsRockWall(g.terr.At(x, y)) {
		return g.reachableSpot(id, x, y)
	}
	if g.grid.Walkable(x, y) && g.reachable(id, x, y) {
		return x, y, true
	}
	return 0, 0, false
}

// startSmooth starts smoothing (x, y), reserving the tile.
func (g *Game) startSmooth(id entity.EntityID, x, y int, forced bool) bool {
	tx, ty, ok := g.smoothSpot(id, x, y)
	if !ok {
		g.postponeTile(id, job.Smooth, x, y) // walled in: SIM-027
		return false
	}
	rec := job.Record{Kind: job.Smooth, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: g.smoothWork(x, y), Forced: forced}
	if !g.startJob(id, rec) {
		return false
	}
	g.reserve.TryTile(id, x, y)
	return true
}

// doSmooth advances a smoothing job: rock becomes smoothed rock, rough
// stone a smooth floor, keeping the stone type.
func (g *Game) doSmooth(id entity.EntityID, j job.Record) {
	k := [2]int{j.DestX, j.DestY}
	if !g.smooths[k] || !g.smoothable(j.DestX, j.DestY) {
		delete(g.smooths, k) // unmarked, dug out, built over or already smooth
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skConstruction, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	to := terrain.StoneSmooth
	if g.terr.At(j.DestX, j.DestY) == terrain.Rock {
		to = terrain.RockSmooth
	}
	g.setTerrain(j.DestX, j.DestY, to) // also drops the mark
	delete(g.smooths, k)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// forcedSmooth is the "Smooth wall/floor" order for pawn id at (x, y)
// (orders.go), or why it can't be given.
func (g *Game) forcedSmooth(id entity.EntityID, x, y int, constructOK bool) (job.Record, string) {
	if !g.smoothable(x, y) {
		return job.Record{}, ReasonGone
	}
	if !constructOK {
		return job.Record{}, ReasonNoConstruct
	}
	if owner, taken := g.reserve.TileOwner(x, y); taken && owner != id {
		return job.Record{}, ReasonBusyTarget + g.name(owner)
	}
	tx, ty, ok := g.smoothSpot(id, x, y)
	if !ok {
		return job.Record{}, ReasonUnreachable
	}
	return job.Record{Kind: job.Smooth, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: g.smoothWork(x, y)}, ""
}
