package sense

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
type Cleaning struct {
	filth  Filth
	ground Ground
	walks  Walks
	labor  Labor
}

// NewCleaning is the component over its roles.
func NewCleaning(f Filth, g Ground, w Walks, l Labor) Cleaning {
	return Cleaning{f, g, w, l}
}

const (
	cleanProbes = 16 // reachability probes per assignment, nearest first
	cleanSlack  = 10 // extra work ticks for dirt tracked in meanwhile
)

func inHome(cells []uint8, i int) bool {
	return i >= 0 && i < len(cells) && cells[i] != 0
}

// Assign gives colonist id the nearest unreserved, reachable queued tile
// (ties by index). Tiles no longer in Home are dropped from the queue on
// the way.
func (c Cleaning) Assign(id entity.EntityID) bool {
	queue := c.filth.Queue()
	if len(queue) == 0 {
		return false
	}
	px, py := c.walks.Pos(id)
	w := c.ground.Width()
	home := c.ground.HomeCells()
	type cand struct {
		d int
		i int32
	}
	var best [cleanProbes]cand
	n := 0
	kept := queue[:0]
	for _, i := range queue {
		if !inHome(home, int(i)) {
			continue // painted out of Home: lazily dropped
		}
		kept = append(kept, i)
		x, y := int(i)%w, int(i)/w
		d := pathing.Dist(px, py, x, y)
		if n == len(best) && d >= best[n-1].d {
			continue // ascending index: an equal distance never wins
		}
		if !c.filth.CleanFree(id, x, y) {
			continue
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
	c.filth.SetQueue(kept)
	for _, b := range best[:n] {
		x, y := int(b.i)%w, int(b.i)/w
		tx, ty, ok := c.walks.ReachableSpot(id, x, y)
		if !ok {
			c.walks.PostponeTile(id, job.Clean, x, y)
			continue
		}
		ticks := cleanTicks(c.filth.Dirt(int(b.i)) + c.filth.Stain(int(b.i)))
		if !c.walks.StartJob(id, job.Record{Kind: job.Clean, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: ticks, Count: ticks}) {
			return false
		}
		c.walks.ClaimTile(id, x, y)
		return true
	}
	return false
}

// Do sweeps the job's tile: filth.CleanPerTick a work tick, dirt before
// stain, until it is clean.
func (c Cleaning) Do(id entity.EntityID, j job.Record) {
	i := j.DestY*c.ground.Width() + j.DestX
	dirt, st := c.filth.Dirt(i), c.filth.Stain(i)
	switch {
	case !c.ground.InBounds(j.DestX, j.DestY) || !inHome(c.ground.HomeCells(), i):
		j.State, j.Fail = job.Fail, job.TargetGone
	case dirt+st == 0 && j.WorkLeft >= j.Count:
		j.State, j.Fail = job.Fail, job.TargetGone // clean before it was started
	case dirt+st == 0:
		j.State = job.Complete
	}
	if j.State != job.Do {
		c.walks.SetJob(id, j)
		return
	}
	u := c.labor.WorkUnits(id)
	if u == 0 {
		c.walks.SetJob(id, j)
		return
	}
	left := sweep(c.filth, i, j.DestX, j.DestY, dirt, st)
	j.WorkLeft -= u
	if left == 0 || j.WorkLeft <= 0 {
		j.State = job.Complete // what is left stays queued
	}
	c.walks.SetJob(id, j)
}

// cleanTicks is the work budget for a tile holding total dirt and stain.
func cleanTicks(total int) int {
	return (total+filth.CleanPerTick-1)/filth.CleanPerTick + cleanSlack
}

// sweep takes one tick's worth off tile i, dirt before stain; left is what
// remains.
func sweep(f Filth, i, x, y, dirt, st int) (left int) {
	nd, ns := filth.CleanStep(dirt, st)
	f.AddDirt(i, nd-dirt)
	if ns != st {
		f.SetStain(x, y, ns)
	}
	return nd + ns
}

// Wire switches cleaning on, at the default priority, for every colonist of
// a save made before cleaning existed.
func (c Cleaning) Wire() {
	for _, id := range c.labor.PawnIDs() {
		if c.labor.IsRaider(id) {
			continue
		}
		if !c.labor.HasLabor(id) {
			continue
		}
		if c.labor.Priorities(id)[work.Clean] == 0 {
			c.labor.SetPriority(id, work.Clean, work.DefaultPriority)
		}
	}
}
