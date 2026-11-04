package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
)

// A tidy's job.CleanRoom holds its tiles in Route, encoded as the tile index
// y*w+x, in walking order, RouteI the one being swept. DestX, DestY is that
// tile, TX, TY the spot it is worked from, and WorkLeft, Count its work, as a
// plain job.Clean has them. Each tile is claimed as the pawn heads for it and
// let go after it.

const (
	// maxTidyCells bounds a route: what "unlimited" means (CSE-023, ours).
	maxTidyCells = 64
	// hugeRoomCells is the size above which a room is tidied only around the
	// job (CSE-026), hugeRoomReach how far: a square of 225 cells, near the
	// mod's 200.
	hugeRoomCells = 160
	hugeRoomReach = 7
	// detourFar is how long a walk to the fetch has to be before a detour
	// counts, in path steps standing in for the mod's path cost of 500
	// (ours); detourFarLine is the same in straight tiles. detourRatio is
	// the mod's 0.7, in tenths.
	detourFar     = 40
	detourFarLine = 10
	detourRatio   = 7
)

// tidyCell is a queued tile and its distance from the job.
type tidyCell struct {
	d int
	i int32
}

// detour is CSE-027. A job with a fetch walks start to fetch to target; a tidy
// sends the pawn start to target first. That is unreasonable when the fetch
// is far and the target doesn't lie on the way. The real paths are searched,
// up to three, or straight lines with the rule off; a failed or unaffordable
// search skips the tidy rather than search on.
func (t Tidy) detour(id entity.EntityID, rec job.Record, ax, ay int) bool {
	fx, fy := rec.TX, rec.TY
	if rec.Kind != job.Bill && rec.Kind != job.Eat || fx == ax && fy == ay {
		return false
	}
	px, py := t.r.Pos(id)
	if !t.s.Rules.On(gameplay.FullPathCheck) {
		sf := pathing.Dist(px, py, fx, fy)
		return sf > detourFarLine && 10*pathing.Dist(px, py, ax, ay) < detourRatio*(sf+pathing.Dist(fx, fy, ax, ay))
	}
	sf, ok := t.r.PathLen(id, px, py, fx, fy)
	if !ok {
		return true
	}
	if sf <= detourFar {
		return false
	}
	st, ok := t.r.PathLen(id, px, py, ax, ay)
	if !ok {
		return true
	}
	ft, ok := t.r.PathLen(id, fx, fy, ax, ay)
	return !ok || 10*st < detourRatio*(sf+ft)
}

// list is CSE-026: the queued Home tiles of room, nearest (ax, ay) first, up
// to limit (0: maxTidyCells), that id may take. In a huge room only those near
// the job. One walk of the queue, behind the cooldown and the room's dirt. The
// queue ascends, so a tie stays with the lower index.
func (t Tidy) list(id entity.EntityID, room, cells, ax, ay, limit int, buf []tidyCell) []tidyCell {
	if limit <= 0 || limit > maxTidyCells {
		limit = maxTidyCells
	}
	grid, home, w := t.r.RoomGrid(), t.r.HomeCells(), t.r.Width()
	huge := cells > hugeRoomCells
	n := 0
	for _, i := range t.r.Queue() {
		k := int(i)
		if k >= len(grid) || grid[k] != room || !inHome(home, k) {
			continue
		}
		x, y := k%w, k/w
		d := pathing.Dist(ax, ay, x, y)
		if huge && d > hugeRoomReach || n == limit && d >= buf[n-1].d {
			continue
		}
		if !t.r.CleanFree(id, x, y) {
			continue
		}
		if n < limit {
			n++
		}
		c := n - 1
		for c > 0 && buf[c-1].d > d {
			buf[c] = buf[c-1]
			c--
		}
		buf[c] = tidyCell{d, i}
	}
	return buf[:n]
}

// walkOrder puts cells in greedy nearest-neighbour order from (x, y); ties
// by index.
func walkOrder(cells []tidyCell, x, y, w int) {
	for a := range cells {
		best, bestD := a, -1
		for b := a; b < len(cells); b++ {
			k := int(cells[b].i)
			d := pathing.Dist(x, y, k%w, k/w)
			if bestD < 0 || d < bestD || d == bestD && cells[b].i < cells[best].i {
				best, bestD = b, d
			}
		}
		cells[a], cells[best] = cells[best], cells[a]
		k := int(cells[a].i)
		x, y = k%w, k/w
	}
}

// route is the job.CleanRoom for room around (ax, ay), starting at the first
// tile in reach; ok false when no tile will do.
func (t Tidy) route(id entity.EntityID, room, cells, ax, ay, limit int) (job.Record, bool) {
	var buf [maxTidyCells]tidyCell
	picked := t.list(id, room, cells, ax, ay, limit, buf[:maxTidyCells])
	if len(picked) == 0 {
		return job.Record{}, false
	}
	w := t.r.Width()
	px, py := t.r.Pos(id)
	walkOrder(picked, px, py, w)
	route := make([]entity.EntityID, len(picked))
	for k, c := range picked {
		route[k] = entity.EntityID(c.i)
	}
	for k, c := range picked {
		x, y := int(c.i)%w, int(c.i)/w
		tx, ty, ok := t.r.ReachableSpot(id, x, y)
		if !ok {
			t.r.PostponeTile(id, job.Clean, x, y)
			continue
		}
		ticks := cleanTicks(t.r.Dirt(int(c.i)) + t.r.Stain(int(c.i)))
		return job.Record{Kind: job.CleanRoom, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: ticks, Count: ticks,
			Route: route, RouteI: k}, true
	}
	return job.Record{}, false
}

// Do runs one tick of job.CleanRoom: sweep the tile, then head for the next.
// A tile that turned clean, left Home or that another pawn took is passed by.
func (t Tidy) Do(id entity.EntityID, j job.Record) {
	r := t.r
	i := j.DestY*r.Width() + j.DestX
	dirt, st := r.Dirt(i), r.Stain(i)
	if dirt+st == 0 || !r.InBounds(j.DestX, j.DestY) || !inHome(r.HomeCells(), i) || !r.TryTile(id, j.DestX, j.DestY) {
		t.next(id, j)
		return
	}
	u := r.WorkUnits(id)
	if u == 0 {
		r.SetJob(id, j)
		return
	}
	left := sweep(r, i, j.DestX, j.DestY, dirt, st)
	if j.WorkLeft -= u; left == 0 || j.WorkLeft <= 0 { // what is left stays queued
		t.next(id, j)
		return
	}
	r.SetJob(id, j)
}

// next lets the swept tile go and sets off for the next one worth the walk,
// or ends the tidy. A tile out of reach is put off, like any cleaning.
func (t Tidy) next(id entity.EntityID, j job.Record) {
	r := t.r
	if !t.holdsForResume(id, j.DestX, j.DestY) {
		r.LeaveTile(id, j.DestX, j.DestY)
	}
	w, home := r.Width(), r.HomeCells()
	for k := j.RouteI + 1; k < len(j.Route); k++ {
		i := int(j.Route[k])
		x, y := i%w, i/w
		total := r.Dirt(i) + r.Stain(i)
		if total == 0 || !inHome(home, i) || !r.CleanFree(id, x, y) {
			continue
		}
		tx, ty, ok := r.ReachableSpot(id, x, y)
		if ok {
			if path, found := r.PathFor(id, tx, ty); found {
				r.ClaimTile(id, x, y)
				ticks := cleanTicks(total)
				j.RouteI, j.TX, j.TY, j.DestX, j.DestY, j.WorkLeft, j.Count = k, tx, ty, x, y, ticks, ticks
				j.Path, j.PathI, j.State = path, 0, job.Goto
				if len(path) == 0 {
					j.State = job.Do
				}
				r.SetJob(id, j)
				return
			}
		}
		r.PostponeTile(id, job.Clean, x, y)
	}
	j.State = job.Complete
	r.SetJob(id, j)
}

// holdsForResume: the job waiting behind this tidy claimed (x, y) itself (a
// work spot that was dirty too), so the tile stays claimed.
func (t Tidy) holdsForResume(id entity.EntityID, x, y int) bool {
	rec, ok := t.s.Resume[id]
	return ok && (rec.TX == x && rec.TY == y || rec.DestX == x && rec.DestY == y)
}
