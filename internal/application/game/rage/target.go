package rage

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// A manhunter's foes (EAI-002): any standing person (a colonist, a guest
// or visitor, a raider, a prisoner) and the colony's tame animals, looked
// for through the creature index in widening rings, never a map scan. It
// prefers one it can reach on its own layer, then (an animal that breaks
// doors) one it could reach through doors, checking at most reachProbes
// candidates each (region checks); with none of those it goes for the
// nearest anyway (and breaks through to it: bash.go).
//
// Cost: a foe it cannot reach even through doors is not chased: bash.go
// marks it at once (no search), the animal waits, and on its next look it
// takes any reachable foe instead (TestUnreachableFoeIsSkipped).
//
// Hysteresis: the foe is locked (Memory.Lock); it switches only when the
// foe is gone, or when, once lockTicks have passed since the lock, another
// is retargetMargin tiles closer (TestManhunterRetargetNoFlap). A rage
// with no foe in sight looks again only every animal.DecideEvery ticks.
const (
	// retargetMargin: how much closer another foe must be to switch.
	retargetMargin = 6
	// lockTicks: a foe is kept at least this long.
	lockTicks = 180
	// reachProbes: reach checks a look may spend.
	reachProbes = 6
)

// foeRings are the radii a manhunter looks within, nearest first; the
// last is the whole map.
func foeRings() [4]int { return [4]int{12, 32, 96, 1 << 14} }

// foeKinds: a manhunter's foes are people and animals.
const foeKinds = Pawns | Raiders | Animals

// isFoe keeps a foe of enraged animal s.Self: a standing person, or a
// living, standing tame animal other than itself.
func isFoe(s Sight, o entity.EntityID) bool {
	if o == s.Self || o == 0 {
		return false
	}
	if a := s.R.h.Ref(o); a != nil {
		return a.Tame && !a.Dead() && !a.Downed
	}
	return s.R.h.StandingPawn(o)
}

// reachableFoe keeps a foe s.Self can reach, while probes last.
func reachableFoe(s Sight, o entity.EntityID) bool {
	r := s.R
	if !isFoe(s, o) || r.probes <= 0 {
		return false
	}
	r.probes--
	me := r.h.Ref(s.Self)
	tame := me != nil && me.Tame
	return r.ReachesFoe(tame, r.pos(s.Self), r.pos(o))
}

// breachableFoe keeps a foe s.Self could reach through doors (the tame
// layer, which counts them open), while probes last.
func breachableFoe(s Sight, o entity.EntityID) bool {
	r := s.R
	if !isFoe(s, o) || r.probes <= 0 {
		return false
	}
	r.probes--
	return r.ReachesFoe(true, r.pos(s.Self), r.pos(o))
}

// pickFoe is the nearest foe id at pos can reach, else (a door breaker)
// the nearest it could reach through doors, else the nearest at all (0:
// none on the map); reach reports one of the first two.
func pickFoe(r *Rage, id entity.EntityID, pos Pos, now uint64) (f entity.EntityID, reach bool) {
	s := Sight{R: r, Self: id, Now: now, Keep: reachableFoe}
	if f = nearestFoe(r, pos, s, true); f != 0 {
		return f, true
	}
	if me := r.h.Ref(id); me != nil && !me.Tame && !animal.DefOf(me.Species).Hive {
		s.Keep = breachableFoe
		if f = nearestFoe(r, pos, s, true); f != 0 {
			return f, true
		}
	}
	s.Keep = isFoe
	return nearestFoe(r, pos, s, false), false
}

// nearestFoe is the nearest foe s keeps, ring by ring; probed stops once
// the reach probes are spent.
func nearestFoe(r *Rage, pos Pos, s Sight, probed bool) entity.EntityID {
	r.probes = reachProbes
	for _, rad := range foeRings() {
		if f, _, ok := r.h.Nearest(pos.X, pos.Y, rad, foeKinds, s); ok {
			return f
		}
		if probed && r.probes <= 0 {
			break
		}
	}
	return 0
}

// ThinkManhunter hunts the manhunter's foe (pickFoe, locked). It reports
// false once the rage has passed, or while no foe is left.
func (r *Rage) ThinkManhunter(id entity.EntityID, a *animal.State, now uint64) bool {
	if !a.Enraged(now) {
		if a.Manhunter != 0 {
			a.Manhunter, a.Target, a.Mode = 0, 0, animal.Wander
			r.m.Forget(id)
		}
		return false
	}
	look := (now+uint64(id))%animal.DecideEvery == 0
	if !isFoe(Sight{R: r, Self: id}, a.Target) {
		a.Target = 0
		if a.Mode == animal.Attack || look { // a foe just fell: look at once; nobody about: on the decision beat
			retarget(r, id, a, now)
		}
	} else if look && (now >= r.m.Lock[id].Until || r.m.unreachable(id, a.Target, now)) {
		closer(r, id, a, now)
	}
	if a.Target == 0 {
		a.Mode = animal.Wander
		return false // nobody left standing: calm down for now
	}
	r.Chase(id, a, now)
	return true
}

// retarget locks the nearest foe (pickFoe) for animal id.
func retarget(r *Rage, id entity.EntityID, a *animal.State, now uint64) {
	a.Target, _ = pickFoe(r, id, r.pos(id), now)
	if a.Target != 0 {
		a.Path, a.PathI = nil, 0
		r.m.lock(id, Lock{Foe: a.Target, Until: now + lockTicks})
		if bs, ok := r.m.Bash[id]; ok && bs.X < 0 {
			delete(r.m.Bash, id) // the unreachable mark was the old foe's
		}
	}
}

// closer switches animal id to another foe retargetMargin tiles closer
// than its own, if there is one, or to any reachable one while its own is
// marked unreachable (bash.go).
func closer(r *Rage, id entity.EntityID, a *animal.State, now uint64) {
	pos := r.pos(id)
	f, reach := pickFoe(r, id, pos, now)
	if f == 0 || f == a.Target {
		return
	}
	fp, tp := r.pos(f), r.pos(a.Target)
	if reach && r.m.unreachable(id, a.Target, now) ||
		weapon.Distance(pos.X, pos.Y, fp.X, fp.Y)+retargetMargin <= weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) {
		a.Target, a.Path, a.PathI = f, nil, 0
		delete(r.m.Bash, id) // a new foe: a new way to it
		r.m.lock(id, Lock{Foe: f, Until: now + lockTicks})
	}
}
