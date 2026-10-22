package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
)

// Cleaning (SIM-021): colonists with cleaning work sweep dirt and stains
// off Home-area tiles, nearest first. A tile is queued at filth.CleanStart
// and cleaned to 0, so a half-cleaned tile is finished later rather than
// dropped (hysteresis).

const (
	cleanProbes = 16 // reachability probes per assignment, nearest first
	cleanSlack  = 10 // extra work ticks for dirt tracked in meanwhile
)

// assignClean gives colonist id the nearest unreserved, reachable queued
// tile (ties by index). Tiles no longer in Home are dropped from the queue
// on the way.
func (g *Game) assignClean(id entity.EntityID) bool {
	f := &g.filth
	if len(f.cleanQ) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	w := g.grid.Width()
	type cand struct {
		d int
		i int32
	}
	var best [cleanProbes]cand
	n := 0
	kept := f.cleanQ[:0]
	for _, i := range f.cleanQ {
		if !g.inHome(int(i)) {
			continue // painted out of Home: lazily dropped
		}
		kept = append(kept, i)
		x, y := int(i)%w, int(i)/w
		d := pathing.Dist(pos.X, pos.Y, x, y)
		if n == len(best) && d >= best[n-1].d {
			continue // ascending index: an equal distance never wins
		}
		if _, taken := g.reserve.TileOwner(x, y); taken {
			continue
		}
		if g.postponedFor(id, job.Clean, 0, x, y) {
			continue // SIM-027: out of reach lately
		}
		if n < len(best) {
			n++
		}
		k := n - 1
		for k > 0 && best[k-1].d > d {
			best[k] = best[k-1]
			k--
		}
		best[k] = cand{d, i}
	}
	f.cleanQ = kept
	for _, c := range best[:n] {
		x, y := int(c.i)%w, int(c.i)/w
		tx, ty, ok := g.reachableSpot(id, x, y)
		if !ok {
			g.postponeTile(id, job.Clean, x, y)
			continue
		}
		work := (g.filthTotal(int(c.i))+filth.CleanPerTick-1)/filth.CleanPerTick + cleanSlack
		if !g.startJob(id, job.Record{Kind: job.Clean, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: work, Count: work}) {
			return false
		}
		g.reserve.TryTile(id, x, y)
		return true
	}
	return false
}

// doClean sweeps the job's tile: filth.CleanPerTick a work tick, dirt
// before stain, until it is clean.
func (g *Game) doClean(id entity.EntityID, j job.Record) {
	i := j.DestY*g.grid.Width() + j.DestX
	dirt, st := g.dirtAtIdx(i), g.stainAtIdx(i)
	switch {
	case !g.grid.InBounds(j.DestX, j.DestY) || !g.inHome(i):
		j.State, j.Fail = job.Fail, job.TargetGone
	case dirt+st == 0 && j.WorkLeft >= j.Count:
		j.State, j.Fail = job.Fail, job.TargetGone // clean before it was started
	case dirt+st == 0:
		j.State = job.Complete
	}
	if j.State != job.Do {
		g.jobs.Set(id, j)
		return
	}
	if !g.workTick(id) {
		g.jobs.Set(id, j)
		return
	}
	nd, ns := filth.CleanStep(dirt, st)
	g.addDirt(i, nd-dirt)
	if ns != st {
		g.setStainAmount(j.DestX, j.DestY, ns)
	}
	j.WorkLeft--
	if nd+ns == 0 || j.WorkLeft <= 0 {
		j.State = job.Complete // what is left stays queued
	}
	g.jobs.Set(id, j)
}

// wireCleaning switches cleaning on, at the default priority, for every
// colonist of a save made before cleaning existed.
func (g *Game) wireCleaning() {
	for _, id := range g.pawnIDs {
		if g.isRaider(id) {
			continue
		}
		if _, ok := g.labor.Get(id); !ok {
			continue
		}
		if g.prioOf(id)[work.Clean] == 0 {
			g.setPriority(id, work.Clean, work.DefaultPriority)
		}
	}
}
