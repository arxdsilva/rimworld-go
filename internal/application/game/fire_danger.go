package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Fire danger (PLAY-015 §5.2). Paths avoid burning cells (fireAvoidCost,
// wired into the costed path query). A colonist caught standing in fire
// (not drafted, not fighting it) runs out; an idle one in or beside fire
// steps out. Animals flee fire within fireFleeR. Hysteresis: trouble
// starts within 1 cell (4 for animals) of a fire, but a step-out goal is
// at least fireSafeDist away and lingers there fireShyTicks, and no idle
// or animal walk targets a cell near a fire again while it burns.

const (
	// fireCostBurning and fireCostBeside are the extra path costs, in
	// percent of a plain step, of entering a burning cell (x20 in all)
	// and a cell beside one.
	fireCostBurning = 1900
	fireCostBeside  = 200
	// fireSafeDist: a step-out goal is at least this far (Chebyshev) from
	// every fire; idle walks never target a cell nearer than it.
	fireSafeDist = 3
	// fireShyTicks is how long a colonist that stepped out of fire
	// lingers at its safe spot (real work may still call it away).
	fireShyTicks = 600
	// fireStepR bounds the search for a safe spot.
	fireStepR = 8
	// fireFleeR: animals flee fires this close; their own walks keep
	// fireFleeR+1 off (animal.FleeRelease more while fleeing).
	fireFleeR = 4
)

// anyFire reports whether anything burns (the cheap early-out of every
// fire danger check).
func (g *Game) anyFire() bool { return len(g.fire.fires) > 0 }

// eachFire calls fn for every burning cell, in the active set's order.
func (g *Game) eachFire(fn func(x, y, size int)) {
	for _, f := range g.fire.fires {
		if f.Size > 0 {
			fn(f.X, f.Y, int(f.Size))
		}
	}
}

// fireNear reports a fire within Chebyshev distance r of (x, y).
func (g *Game) fireNear(x, y, r int) bool {
	_, ok := g.nearestFire(x, y, r)
	return ok
}

// nearestFire is the nearest fire (squared distance; ties: row by row,
// left to right) within Chebyshev distance r of (x, y): the chunk counts
// first (fire.go firesAround), then the cells.
func (g *Game) nearestFire(x, y, r int) (Position, bool) {
	if !g.anyFire() || !g.firesAround(x, y, r) {
		return Position{}, false
	}
	best, bestD, found := Position{}, 0, false
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if d := dx*dx + dy*dy; (!found || d < bestD) && g.fireAt(x+dx, y+dy) > 0 {
				best, bestD, found = Position{X: x + dx, Y: y + dy}, d, true
			}
		}
	}
	return best, found
}

// fireDangerCost is fireAvoidCost: the extra path cost (percent of a
// plain step) of stepping onto (x, y) because of fire on or beside it.
func (g *Game) fireDangerCost(x, y int) int {
	if !g.anyFire() {
		return 0
	}
	if g.fireAt(x, y) > 0 {
		return fireCostBurning
	}
	if g.fireNear(x, y, 1) {
		return fireCostBeside
	}
	return 0
}

// fireDanger moves colonist id out of fire: out of it when caught
// standing in it, whatever it was doing (a player order goes back on its
// queue, and the job's spot is put off a while); out of reach of it when
// idling in or beside it, or idly heading there. Drafted colonists and
// firefighters stay. It reports true when the colonist was sent off.
func (g *Game) fireDanger(id entity.EntityID, j job.Record) bool {
	if !g.anyFire() || g.drafted(id) || j.Kind == job.FightFire {
		return false
	}
	pos, _ := g.positions.Get(id)
	moving := j.State == job.Goto && j.PathI < len(j.Path)
	if g.fireAt(pos.X, pos.Y) > 0 && !moving {
		if j.Kind.IsIdle() && !g.fireNear(j.TX, j.TY, 1) {
			return false // already on its way out
		}
		g.dropForFlight(id, j)
		if j.Kind != job.None && !j.Kind.IsIdle() {
			g.postponeTile(id, j.Kind, j.TX, j.TY) // postpone.go: not straight back in
		}
		g.stepOutOfFire(id, pos)
		return true
	}
	if !j.Kind.IsIdle() || !j.Active() {
		return false
	}
	if g.fireNear(j.TX, j.TY, 1) || !moving && g.fireNear(pos.X, pos.Y, 1) {
		g.interrupt(id, job.Interrupted)
		g.stepOutOfFire(id, pos)
		return true
	}
	return false
}

// stepOutOfFire walks colonist id at pos to the nearest safe spot (a
// lingering stroll: real work may still call it away).
func (g *Game) stepOutOfFire(id entity.EntityID, pos Position) bool {
	x, y, ok := g.fireSafeSpot(id, pos)
	if !ok {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Wander, TX: x, TY: y, WorkLeft: fireShyTicks})
}

// fireSafeSpot is the nearest free walkable cell, ring by ring out to
// fireStepR, at least fireSafeDist from every fire, that colonist id can
// reach.
func (g *Game) fireSafeSpot(id entity.EntityID, pos Position) (int, int, bool) {
	for r := 1; r <= fireStepR; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx != -r && dx != r && dy != -r && dy != r {
					continue // ring r only
				}
				x, y := pos.X+dx, pos.Y+dy
				if !g.grid.Walkable(x, y) || g.fireNear(x, y, fireSafeDist-1) {
					continue
				}
				if _, taken := g.reserve.TileOwner(x, y); taken {
					continue
				}
				if g.reachable(id, x, y) {
					return x, y, true
				}
			}
		}
	}
	return 0, 0, false
}

// idleFireBlocked reports an idle walk's goal too near a fire.
func (g *Game) idleFireBlocked(x, y int) bool {
	return g.fireNear(x, y, fireSafeDist-1)
}

// animalFireThreat is the fire an animal at pos flees (the nearest within
// fireFleeR; FleeRelease more while it is already fleeing).
func (g *Game) animalFireThreat(a *animal.State, pos Position) (Position, bool) {
	if !g.anyFire() {
		return Position{}, false
	}
	r := fireFleeR
	if a.Mode == animal.Flee {
		r += animal.FleeRelease
	}
	return g.nearestFire(pos.X, pos.Y, r)
}

// animalFireBlocked reports an animal walk's goal within reach of a fire
// it would flee (a fleeing animal's own run is never blocked).
func (g *Game) animalFireBlocked(a *animal.State, x, y int) bool {
	return a.Mode != animal.Flee && g.fireNear(x, y, fireFleeR+1)
}
