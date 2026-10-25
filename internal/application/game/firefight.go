package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
)

// Firefighting (PLAY-015 §5.2): colonists with the Firefight work type
// (the first work column, on by default) beat out fires from a cell
// beside them, fireBeat size every fireBeatTicks, whatever their skills.
// A fire is theirs to fight when it burns inside Home AND inside their
// allowed area (an unrestricted colonist: Home only). A drafted colonist
// can be ordered to Extinguish any fire (a forced order ignores Home and
// the area). Fire state is fire.go's: this file reads it with fireAt and
// eachFire and puts fires out with douseFire.

const (
	// fireBeatTicks is how long one beat takes; fireBeat what it takes
	// off the fire's size.
	fireBeatTicks = 30
	fireBeat      = 10
	// fireProbes: the nearest eligible fires tried per assignment.
	fireProbes = 8
)

// fireEligible reports whether colonist id fights the fire at (x, y) on
// its own: inside Home and inside its allowed area.
func (g *Game) fireEligible(id entity.EntityID, x, y int) bool {
	if !g.grid.InBounds(x, y) || !g.inHome(y*g.grid.Width()+x) {
		return false
	}
	return g.inArea(id, x, y)
}

// assignFightFire sends colonist id to the nearest eligible fire it can
// stand beside (ties: the active set's order).
func (g *Game) assignFightFire(id entity.EntityID) bool {
	if !g.anyFire() {
		return false
	}
	pos, _ := g.positions.Get(id)
	type cand struct{ d, x, y int }
	var best [fireProbes]cand
	n := 0
	g.eachFire(func(x, y, _ int) {
		d := pathing.Dist(pos.X, pos.Y, x, y)
		if n == len(best) && d >= best[n-1].d {
			return
		}
		if !g.fireEligible(id, x, y) || g.postponedFor(id, job.FightFire, 0, x, y) {
			return
		}
		if o, taken := g.reserve.TileOwner(x, y); taken && o != id {
			return
		}
		if n < len(best) {
			n++
		}
		k := n - 1
		for k > 0 && best[k-1].d > d {
			best[k] = best[k-1]
			k--
		}
		best[k] = cand{d, x, y}
	})
	for _, c := range best[:n] {
		tx, ty, ok := g.fireStand(id, c.x, c.y, false)
		if !ok {
			g.postponeTile(id, job.FightFire, c.x, c.y)
			continue
		}
		if !g.startJob(id, fireJob(tx, ty, c.x, c.y)) {
			return false
		}
		g.reserve.TryTile(id, c.x, c.y)
		return true
	}
	return false
}

// fireJob is the job beating out the fire at (fx, fy) from (tx, ty).
func fireJob(tx, ty, fx, fy int) job.Record {
	return job.Record{Kind: job.FightFire, TX: tx, TY: ty, DestX: fx, DestY: fy, WorkLeft: fireBeatTicks}
}

// fireStand is where colonist id stands to beat out the fire at (fx, fy):
// where it is, when that is beside the fire and not burning; else the
// reachable, unburning, free walkable cell beside the fire nearest to it
// (inside its allowed area unless forced).
func (g *Game) fireStand(id entity.EntityID, fx, fy int, forced bool) (int, int, bool) {
	pos, _ := g.positions.Get(id)
	if pathing.Adjacent(pos.X, pos.Y, fx, fy) && g.fireAt(pos.X, pos.Y) == 0 {
		return pos.X, pos.Y, true
	}
	bx, by, bd, found := 0, 0, 0, false
	for _, d := range [...][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		x, y := fx+d[0], fy+d[1]
		if !g.grid.Walkable(x, y) || g.fireAt(x, y) > 0 || !forced && !g.inArea(id, x, y) {
			continue
		}
		if o, taken := g.reserve.TileOwner(x, y); taken && o != id {
			continue
		}
		dd := pathing.Dist(pos.X, pos.Y, x, y)
		if found && dd >= bd || !g.reachable(id, x, y) {
			continue
		}
		bx, by, bd, found = x, y, dd, true
	}
	return bx, by, found
}

// extinguishJob is the Extinguish order's job (a drafted colonist, any
// fire), or why it can't be done.
func (g *Game) extinguishJob(id entity.EntityID, x, y int) (job.Record, string) {
	if g.fireAt(x, y) == 0 {
		return job.Record{}, ReasonGone
	}
	tx, ty, ok := g.fireStand(id, x, y, true)
	if !ok {
		return job.Record{}, ReasonUnreachable
	}
	return fireJob(tx, ty, x, y), ""
}

// fireBeside is the next burning cell beside (x, y) colonist id may beat
// out (any, when forced), scanning in a fixed order.
func (g *Game) fireBeside(id entity.EntityID, x, y int, forced bool) (int, int, bool) {
	for _, d := range [...][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {1, -1}, {1, 1}, {-1, 1}, {-1, -1}} {
		nx, ny := x+d[0], y+d[1]
		if g.fireAt(nx, ny) > 0 && (forced || g.fireEligible(id, nx, ny)) {
			return nx, ny, true
		}
	}
	return 0, 0, false
}

// doFightFire beats out the job's fire, one beat every fireBeatTicks;
// when it is out (or no longer the colonist's to fight) the colonist
// turns to a burning cell beside it, or is done.
func (g *Game) doFightFire(id entity.EntityID, j job.Record) {
	pos, _ := g.positions.Get(id)
	lost := g.fireAt(j.DestX, j.DestY) == 0 || !j.Forced && !g.fireEligible(id, j.DestX, j.DestY) ||
		!pathing.Adjacent(pos.X, pos.Y, j.DestX, j.DestY) && (pos.X != j.DestX || pos.Y != j.DestY)
	if lost {
		fx, fy, ok := g.fireBeside(id, pos.X, pos.Y, j.Forced)
		if !ok {
			j.State = job.Complete
			g.jobs.Set(id, j)
			return
		}
		g.reserve.TryTile(id, fx, fy)
		j.DestX, j.DestY, j.WorkLeft = fx, fy, fireBeatTicks
	}
	j.WorkLeft--
	if j.WorkLeft <= 0 {
		g.douseFire(j.DestX, j.DestY, fireBeat) // fire.go
		j.WorkLeft = fireBeatTicks
	}
	g.jobs.Set(id, j)
}
