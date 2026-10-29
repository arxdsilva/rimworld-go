package game

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Reactions to being hurt and where to run (PLAY-010 2.3 A, B): the
// helpers of animalHurt (animal_needs.go) and fleeCheck (animals.go).

// hurtBlind is a hit with no one behind it (a trap, a burst): a wild
// flee-class animal runs for a while, a sleeper wakes.
func (g *Game) hurtBlind(a *animal.State, now uint64) {
	if animal.DefOf(a.Species).Skittish && !a.Tame && !a.Enraged(now) {
		a.Mode, a.FleeUntil, a.Path = animal.Flee, now+animal.HurtFleeTicks, nil
	}
	if a.Mode == animal.Flee {
		a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	}
	if a.Mode == animal.Rest && !a.Enraged(now) {
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	}
}

// canFightBack reports whether animal id, hurt by by, can and will fight
// back: a fighter (territorial, predator, legendary; a hive insect) that
// is not tame and hit by a colonist, not badly hurt (a legendary fights
// on regardless), and can reach its attacker on its own layer.
func (g *Game) canFightBack(id entity.EntityID, a animal.State, by entity.EntityID) bool {
	def := animal.DefOf(a.Species)
	switch {
	case !animal.Fighter(a.Species) && !def.Hive:
		return false // a flee class
	case a.Tame && g.isPawn(by) && !g.isRaider(by):
		return false // its own colonist: never fights them
	case badlyHurt(a) && !def.Legendary:
		return false
	case !g.validFoe(id, by):
		return false
	}
	pos, _ := g.positions.Get(id)
	fp, _ := g.positions.Get(by)
	return g.reachesFoe(a.Tame, pos, fp)
}

// fightBack turns a hurt fighter on by, or keeps the live foe it is
// already fighting (no hopping between attackers).
func fightBack(a *animal.State, by entity.EntityID, targetLive bool) {
	if a.Mode != animal.Attack {
		a.Path, a.PathI = nil, 0 // a sleeper or grazer drops what it was doing
	}
	if a.Mode != animal.Attack || a.Target == 0 || !targetLive {
		a.Target, a.HuntSince = by, 0
	}
	a.Mode, a.FleeUntil = animal.Attack, 0
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
}

// validFoe: someone an animal can fight: a living animal other than
// itself, or a standing person.
func (g *Game) validFoe(self, t entity.EntityID) bool {
	if t == 0 || t == self {
		return false
	}
	if o := g.animals.Ref(t); o != nil {
		return !o.Dead()
	}
	return g.standingPawn(t)
}

// fleeHit runs an animal that can't fight back from by: every hit
// refreshes the flee window and picks a goal at once, even mid-run, so
// it moves on every hit (fleeFrom).
func (g *Game) fleeHit(id entity.EntityID, a *animal.State, by entity.EntityID, now uint64) {
	if a.Mode != animal.Flee {
		g.logEvent(id, logEntry{Kind: logFled, Other: by, Name: g.creatureName(by)})
		a.Path, a.PathI = nil, 0 // a sleeper wakes, a grazer drops its meal
	}
	a.Target, a.HuntSince = 0, 0
	a.Mode, a.FleeUntil = animal.Flee, now+animal.HurtFleeTicks
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	fp, ok := g.positions.Get(by)
	if !ok {
		return
	}
	pos, _ := g.positions.Get(id)
	g.fleeFrom(id, a, pos, fp, fleeHitDist, true)
}

// Flee goals (PLAY-010 2.3 B).
const (
	// fleeHitDist: a hurt animal runs this far straight away.
	fleeHitDist = 8
	// fleeSearchR, fleeSearchMax: when straight away is shut, the cells
	// within this radius it can walk to (at most this many) are searched
	// for the one farthest from the threat.
	fleeSearchR   = 8
	fleeSearchMax = 300
	// fleeStay: a fleeing animal with nowhere better to go looks again
	// this many ticks later.
	fleeStay = 30
)

// fleeFrom sends a fleeing animal at pos away from threat (fleeSpot).
// A run already under way to a goal at least as far from the threat
// stays on it (no goal flapping); mustMove (a hit) never leaves it
// standing where it is if it can move at all.
func (g *Game) fleeFrom(id entity.EntityID, a *animal.State, pos, threat Position, dist int, mustMove bool) {
	spot, ok := g.fleeSpot(a, pos, threat, dist, mustMove)
	running := a.PathI < len(a.Path)
	if running && (!ok || cellDistSq(Position{X: a.GoalX, Y: a.GoalY}, threat) >= cellDistSq(spot, threat)) {
		return
	}
	if !ok || spot == pos {
		a.Path, a.PathI = nil, 0
		a.RetryAt = uint64(g.clock.Now()) + fleeStay
		return
	}
	g.animalGoTo(id, a, spot.X, spot.Y)
}

// cellDistSq is the squared distance between two cells.
func cellDistSq(p, q Position) int {
	dx, dy := p.X-q.X, p.Y-q.Y
	return dx*dx + dy*dy
}

// fleeSpot is where an animal at pos runs from a threat at threat, on its
// own layer (tame: fences shut, gates open; wild: both shut):
//  1. dist tiles straight away (animal.FleeGoal), if a free tile near it
//     can be reached;
//  2. else the reachable cell within fleeSearchR farthest from the
//     threat (in a pen, its far side);
//  3. else a reachable neighbour not beside the threat;
//  4. else any reachable neighbour.
//
// Without mustMove a spot no farther than pos leaves it where it stands
// (pos); with it, pos itself is never the answer. ok is false when it
// can't move at all.
func (g *Game) fleeSpot(a *animal.State, pos, threat Position, dist int, mustMove bool) (Position, bool) {
	// livestock flee within their pen, never out of the gate (rope.go:
	// gates shut to them unless led)
	q := g.animalPathQuery(a.Tame && !animal.RosterOf(a.Species).Penned, false)
	fx, fy := animal.FleeGoal(pos.X, pos.Y, threat.X, threat.Y, dist)
	if x, y, ok := g.combat.Tiles.WalkableNear(fx, fy, 3); ok && (x != pos.X || y != pos.Y) && q.Walkable(x, y) && g.layerReach(q, pos, x, y) {
		return Position{X: x, Y: y}, true
	}
	if p, ok := fleeSearch(q, pos, threat); ok {
		if !mustMove && cellDistSq(p, threat) <= cellDistSq(pos, threat) {
			return pos, true
		}
		return p, true
	}
	if !mustMove {
		return pos, true
	}
	best, found := Position{}, false
	for pass := 0; pass < 2 && !found; pass++ {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				n := Position{X: pos.X + dx, Y: pos.Y + dy}
				if dx == 0 && dy == 0 || pass == 0 && weapon.Distance(n.X, n.Y, threat.X, threat.Y) <= 1 {
					continue
				}
				if n.X < 0 || n.Y < 0 || n.X >= q.Width() || n.Y >= q.Height() || !q.Walkable(n.X, n.Y) || !g.layerReach(q, pos, n.X, n.Y) {
					continue
				}
				if !found || cellDistSq(n, threat) > cellDistSq(best, threat) {
					best, found = n, true
				}
			}
		}
	}
	return best, found
}

// fleeSearch is a breadth-first walk (4-neighbours, so every cell it
// reaches is walkable to) over q from pos, within fleeSearchR and at most
// fleeSearchMax cells, for the cell other than pos farthest from threat
// (the first found on a tie). It allocates nothing.
func fleeSearch(q pathing.Query, pos, threat Position) (Position, bool) {
	const side = 2*fleeSearchR + 1
	var seen [side * side]bool
	var queue [side * side]int16
	w, h := q.Width(), q.Height()
	at := func(x, y int) int { return (y-pos.Y+fleeSearchR)*side + x - pos.X + fleeSearchR }
	seen[at(pos.X, pos.Y)] = true
	queue[0] = int16(at(pos.X, pos.Y))
	head, tail := 0, 1
	best, bestD, found := pos, -1, false
	for head < tail && head < fleeSearchMax {
		i := int(queue[head])
		head++
		c := Position{X: pos.X + i%side - fleeSearchR, Y: pos.Y + i/side - fleeSearchR}
		if c != pos {
			if d := cellDistSq(c, threat); d > bestD {
				best, bestD, found = c, d, true
			}
		}
		for _, n := range [4]Position{{X: c.X + 1, Y: c.Y}, {X: c.X - 1, Y: c.Y}, {X: c.X, Y: c.Y + 1}, {X: c.X, Y: c.Y - 1}} {
			if n.X < 0 || n.Y < 0 || n.X >= w || n.Y >= h || weapon.Distance(n.X, n.Y, pos.X, pos.Y) > fleeSearchR {
				continue
			}
			if j := at(n.X, n.Y); !seen[j] && q.Walkable(n.X, n.Y) {
				seen[j] = true
				queue[tail] = int16(j)
				tail++
			}
		}
	}
	return best, found
}

// layerReach reports whether a walk over q from pos may reach (x, y):
// the region labels when q has them (a different label for its layer
// means a fence, a wall or water between), else yes.
func (g *Game) layerReach(q pathing.Query, pos Position, x, y int) bool {
	if _, pen := q.(penQuery); pen || g.noRegions() {
		// without regions (or inside a pen, which regions don't know)
		// Reach is optimistic: search for the truth, so a colony with
		// regions off flees exactly as one with them
		_, ok := pathing.Find(plainQuery{q}, pos.X, pos.Y, x, y)
		return ok
	}
	if r, ok := q.(pathing.Reacher); ok {
		return r.Reach(pos.X, pos.Y, x, y)
	}
	return true
}

// reachesFoe reports whether an animal (tame or wild) at pos can get at
// a foe at fp: beside it already, or a walk reaches its cell or a cell
// next to it.
func (g *Game) reachesFoe(tame bool, pos, fp Position) bool {
	if weapon.Distance(pos.X, pos.Y, fp.X, fp.Y) <= 1 {
		return true
	}
	q := g.animalPathQuery(tame, false)
	if q.Walkable(fp.X, fp.Y) {
		return g.layerReach(q, pos, fp.X, fp.Y)
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := fp.X+dx, fp.Y+dy
			if (dx != 0 || dy != 0) && x >= 0 && y >= 0 && x < q.Width() && y < q.Height() &&
				q.Walkable(x, y) && g.layerReach(q, pos, x, y) {
				return true
			}
		}
	}
	return false
}

// penQuery is an animal's walk query cut down to its pen's cells.
type penQuery struct {
	pathing.Query
	in func(x, y int) bool
}

// Walkable is the pen-restricted walkability.
func (p penQuery) Walkable(x, y int) bool { return p.in(x, y) && p.Query.Walkable(x, y) }
