package rage

import (
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Reactions to threats and to being hurt, and where to run (PLAY-010
// 2.3 A, B): FleeCheck runs every tick; the game's animalHurt calls
// HurtBlind, CanFightBack, FightBack and FleeHit.

// Flee goals (PLAY-010 2.3 B).
const (
	// fleeHitDist: a hurt animal runs this far straight away.
	fleeHitDist = 8
	// fleeStay: a fleeing animal with nowhere better to go looks again
	// this many ticks later.
	fleeStay = 30
)

// FleeCheck runs a skittish animal away from threats (a tame one from
// wild predators only), and any animal, wild or tame, still in its
// FleeUntil window from whoever hurt it. Threats are looked for every
// animal.ThreatEvery ticks (staggered by id) and every tick while
// fleeing; the goal (fleeFrom) is picked again when the run ends or
// every 40 ticks, and kept while it is at least as far from the threat.
// It reports true while the animal is fleeing.
func (r *Rage) FleeCheck(id entity.EntityID, a *animal.State, pos Pos, def animal.Def, now uint64) bool {
	var tp Pos
	threat := false
	scan := a.Mode == animal.Flee || (now+uint64(id))%animal.ThreatEvery == 0
	rad := def.FleeRadius
	if a.Mode == animal.Flee {
		rad += animal.FleeRelease // run well clear before grazing again
	}
	dist := 8
	switch {
	case a.Tame && def.Predator: // guards don't run from predators
	case a.Tame:
		dist = 6
		if scan {
			tp, threat = r.NearestPredator(id, pos, rad)
		}
	case def.Skittish && rad > 0 && scan: // tame-natured animals only run once hurt
		tp, threat = r.nearestThreat(id, pos, rad)
	}
	// a predator seen across a fence or water can't get at it: no reason
	// to run (running would take it out through a gate, into the threat)
	if threat && !r.ReachesFoe(false, tp, pos) {
		threat = false
	}
	if !threat && scan {
		tp, threat = r.h.FireThreat(*a, pos) // every animal runs from fire
	}
	if now < a.FleeUntil && a.Attacker != 0 {
		if p, ok := r.h.Pos(a.Attacker); ok {
			tp, threat = p, true
		}
	}
	if threat {
		lost := a.PathI >= len(a.Path) && now >= a.RetryAt
		if a.Mode != animal.Flee || lost || (now+uint64(id))%40 == 0 {
			if a.Mode != animal.Flee {
				a.Feed, a.FeedLeft, a.Target = animal.FeedTarget{}, 0, 0 // a sleeper wakes, a grazer drops its meal
				a.Path, a.PathI = nil, 0
			}
			a.Mode = animal.Flee
			r.fleeFrom(id, a, pos, tp, dist, false)
		}
		return true
	}
	if a.Mode == animal.Flee {
		if a.PathI < len(a.Path) {
			return true // finish the run
		}
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
		a.NextDecide = now // the run is over: think again at once
	}
	return false
}

// nearestThreat: the closest standing colonist or predator within rad.
func (r *Rage) nearestThreat(id entity.EntityID, pos Pos, rad int) (Pos, bool) {
	_, p, ok := r.h.Nearest(pos.X, pos.Y, rad, Pawns|Animals, Sight{R: r, Self: id, Keep: threatTo})
	return p, ok
}

// threatTo keeps a predator or a standing pawn other than s.Self.
func threatTo(s Sight, o entity.EntityID) bool {
	if o == s.Self {
		return false
	}
	if a := s.R.h.Ref(o); a != nil {
		return animal.DefOf(a.Species).Predator
	}
	return s.R.h.StandingPawn(o)
}

// NearestPredator: the closest wild predator within rad (tame grazers do
// not fear people or the colony's own wolves).
func (r *Rage) NearestPredator(id entity.EntityID, pos Pos, rad int) (Pos, bool) {
	_, p, ok := r.h.Nearest(pos.X, pos.Y, rad, Animals, Sight{R: r, Self: id, Keep: wildPredator})
	return p, ok
}

// wildPredator keeps a wild predator other than s.Self.
func wildPredator(s Sight, o entity.EntityID) bool {
	a, ok := s.R.h.Animal(o)
	return o != s.Self && ok && !a.Tame && animal.DefOf(a.Species).Predator
}

// HurtBlind is a hit with no one behind it (a trap, a burst): a wild
// flee-class animal runs for a while, a sleeper wakes.
func HurtBlind(a *animal.State, now uint64) {
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

// CanFightBack reports whether animal id, hurt by by, can and will fight
// back: a fighter (territorial, predator, legendary; a hive insect) that
// is not tame and hit by a colonist, not badly hurt (a legendary fights
// on regardless), and can reach its attacker on its own layer.
func (r *Rage) CanFightBack(id entity.EntityID, a animal.State, by entity.EntityID) bool {
	def := animal.DefOf(a.Species)
	switch {
	case !animal.Fighter(a.Species) && !def.Hive:
		return false // a flee class
	case a.Tame && r.h.IsPawn(by) && !r.h.IsRaider(by):
		return false // its own colonist: never fights them
	case BadlyHurt(a) && !def.Legendary:
		return false
	case !r.ValidFoe(id, by):
		return false
	}
	return r.ReachesFoe(a.Tame, r.pos(id), r.pos(by))
}

// FightBack turns a hurt fighter on by, or keeps the live foe it is
// already fighting (no hopping between attackers).
func FightBack(a *animal.State, by entity.EntityID, targetLive bool) {
	if a.Mode != animal.Attack {
		a.Path, a.PathI = nil, 0 // a sleeper or grazer drops what it was doing
	}
	if a.Mode != animal.Attack || a.Target == 0 || !targetLive {
		a.Target, a.HuntSince = by, 0
	}
	a.Mode, a.FleeUntil = animal.Attack, 0
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
}

// ValidFoe: someone an animal can fight: a living animal other than
// itself, or a standing person.
func (r *Rage) ValidFoe(self, t entity.EntityID) bool {
	if t == 0 || t == self {
		return false
	}
	if o := r.h.Ref(t); o != nil {
		return !o.Dead()
	}
	return r.h.StandingPawn(t)
}

// FleeHit runs an animal that can't fight back from by: every hit
// refreshes the flee window and picks a goal at once, even mid-run, so
// it moves on every hit (fleeFrom).
func (r *Rage) FleeHit(id entity.EntityID, a *animal.State, by entity.EntityID, now uint64) {
	if a.Mode != animal.Flee {
		r.h.Log(id, evlog.Fled, by, r.h.CreatureName(by))
		a.Path, a.PathI = nil, 0 // a sleeper wakes, a grazer drops its meal
	}
	a.Target, a.HuntSince = 0, 0
	a.Mode, a.FleeUntil = animal.Flee, now+animal.HurtFleeTicks
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	fp, ok := r.h.Pos(by)
	if !ok {
		return
	}
	r.fleeFrom(id, a, r.pos(id), fp, fleeHitDist, true)
}

// fleeFrom sends a fleeing animal at pos away from threat (FleeSpot).
// A run already under way to a goal at least as far from the threat
// stays on it (no goal flapping); mustMove (a hit) never leaves it
// standing where it is if it can move at all.
func (r *Rage) fleeFrom(id entity.EntityID, a *animal.State, pos, threat Pos, dist int, mustMove bool) {
	spot, ok := r.FleeSpot(a, pos, threat, dist, mustMove)
	running := a.PathI < len(a.Path)
	if running && (!ok || DistSq(Pos{X: a.GoalX, Y: a.GoalY}, threat) >= DistSq(spot, threat)) {
		return
	}
	if !ok || spot == pos {
		a.Path, a.PathI = nil, 0
		a.RetryAt = r.h.Now() + fleeStay
		return
	}
	*a = r.h.GoTo(id, *a, spot.X, spot.Y)
}

// DistSq is the squared distance between two cells.
func DistSq(p, q Pos) int {
	dx, dy := p.X-q.X, p.Y-q.Y
	return dx*dx + dy*dy
}

// FleeSpot is where an animal at pos runs from a threat at threat, on its
// own layer (tame: fences shut, gates open; wild: both shut):
//  1. dist tiles straight away (animal.FleeGoal), if a free tile near it
//     can be reached;
//  2. else the reachable cell nearby (combat.FleeCell) farthest from the
//     threat (in a pen, its far side);
//  3. else a reachable neighbour not beside the threat;
//  4. else any reachable neighbour.
//
// Without mustMove a spot no farther than pos leaves it where it stands
// (pos); with it, pos itself is never the answer. ok is false when it
// can't move at all.
func (r *Rage) FleeSpot(a *animal.State, pos, threat Pos, dist int, mustMove bool) (Pos, bool) {
	// livestock flee within their pen, never out of the gate (gates shut
	// to them unless led)
	q := r.h.PathQuery(a.Tame && !animal.RosterOf(a.Species).Penned)
	fx, fy := animal.FleeGoal(pos.X, pos.Y, threat.X, threat.Y, dist)
	if x, y, ok := r.h.WalkableNear(fx, fy, 3); ok && (x != pos.X || y != pos.Y) && q.Walkable(x, y) && r.h.LayerReach(q, pos, x, y) {
		return Pos{X: x, Y: y}, true
	}
	if x, y, ok := combat.FleeCell(q, pos.X, pos.Y, threat.X, threat.Y); ok {
		p := Pos{X: x, Y: y}
		if !mustMove && DistSq(p, threat) <= DistSq(pos, threat) {
			return pos, true
		}
		return p, true
	}
	if !mustMove {
		return pos, true
	}
	best, found := Pos{}, false
	for pass := 0; pass < 2 && !found; pass++ {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				n := Pos{X: pos.X + dx, Y: pos.Y + dy}
				if dx == 0 && dy == 0 || pass == 0 && weapon.Distance(n.X, n.Y, threat.X, threat.Y) <= 1 {
					continue
				}
				if n.X < 0 || n.Y < 0 || n.X >= q.Width() || n.Y >= q.Height() || !q.Walkable(n.X, n.Y) || !r.h.LayerReach(q, pos, n.X, n.Y) {
					continue
				}
				if !found || DistSq(n, threat) > DistSq(best, threat) {
					best, found = n, true
				}
			}
		}
	}
	return best, found
}

// ReachesFoe reports whether an animal (tame or wild) at pos can get at
// a foe at fp: beside it already, or a walk reaches its cell or a cell
// next to it.
func (r *Rage) ReachesFoe(tame bool, pos, fp Pos) bool {
	if weapon.Distance(pos.X, pos.Y, fp.X, fp.Y) <= 1 {
		return true
	}
	q := r.h.PathQuery(tame)
	if q.Walkable(fp.X, fp.Y) {
		return r.h.LayerReach(q, pos, fp.X, fp.Y)
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := fp.X+dx, fp.Y+dy
			if (dx != 0 || dy != 0) && x >= 0 && y >= 0 && x < q.Width() && y < q.Height() &&
				q.Walkable(x, y) && r.h.LayerReach(q, pos, x, y) {
				return true
			}
		}
	}
	return false
}
