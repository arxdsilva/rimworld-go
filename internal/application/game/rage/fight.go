package rage

import (
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// FightCheck runs a wild animal's fight: a predator's hunt or revenge, a
// woken territorial animal's. A badly hurt fighter breaks off (T10) and a
// hunt that fails gives up (T5). It reports true while the animal fights.
func (r *Rage) FightCheck(id entity.EntityID, a *animal.State, pos Pos, def animal.Def, now uint64) bool {
	// prey, or whoever hurt it (its own kind too: the game's animalHurt)
	if a.Target != 0 && !r.validPrey(id, a.Target) && !(a.Target == a.Attacker && r.ValidFoe(id, a.Target)) {
		EndFight(a)
	}
	calm := !BadlyHurt(*a) && now >= a.FleeUntil
	if a.Target == 0 && def.Predator && !a.Tame && calm && a.Attacker != 0 && r.validPrey(id, a.Attacker) &&
		r.ReachesFoe(a.Tame, pos, r.pos(a.Attacker)) { // never one it can't get at (no flee/attack flapping)
		a.Target = a.Attacker // revenge; on a colonist the player is warned (chase.go)
		r.WarnChase(id, a, a.Target, false)
	}
	if a.Target == 0 {
		a.HuntSince = 0 // its prey died under another's teeth, or was never set
		if def.Predator {
			a.Attacker = 0
		}
		return false
	}
	if BadlyHurt(*a) && !a.Enraged(now) && !def.Legendary {
		r.breakOff(id, a, pos, now)
		return true
	}
	tp := r.pos(a.Target)
	d := weapon.Distance(pos.X, pos.Y, tp.X, tp.Y)
	switch {
	case a.HuntSince != 0 && (now-a.HuntSince > animal.HuntGiveUp || d > HuntRadius(def)+animal.HuntLeash ||
		a.Tame && !r.h.TameHuntOn(id, a.Target, tp)): // a tame hunter's prey is wild and in its area (PLAY-017)
		EndFight(a) // the prey got away: try again later
		a.NextFeed, a.NextDecide = now+animal.HuntRetry, now
		return false
	case a.HuntSince == 0 && !def.Predator && d > territorialLeash:
		EndFight(a) // chased it off
		return false
	}
	r.Chase(id, a, now)
	return true
}

// ChargeCheck is a wild predator's charge (PLAY-010 2.3 C). An awake,
// unhurt, calm predator with no target looks every animal.DecideEvery
// ticks (staggered by id) for a standing colonist or raider within
// animal.ChargeRadius and, when it finds one, rolls animal.ChargePct;
// any roll waits animal.ChargeCooldown for the next. A charge is an
// Attack on that person (FightCheck chases it) that ends beyond
// animal.ChargeLeash, after animal.ChargeTicks, or once the target is
// down or the animal turned to something else; the cooldown starts then.
// Off where predators leave colonists alone (Wanderer, Gentle).
func (r *Rage) ChargeCheck(id entity.EntityID, a *animal.State, pos Pos, now uint64) {
	if a.ChargeSince != 0 {
		tp, ok := r.h.Pos(a.Target)
		if a.Tame || a.Mode != animal.Attack || !ok || !r.h.StandingPawn(a.Target) ||
			now-a.ChargeSince > animal.ChargeTicks || weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) > animal.ChargeLeash {
			if a.Mode == animal.Attack && a.Target != 0 {
				if a.Attacker == a.Target {
					a.Attacker = 0 // it let them go: no revenge straight after
				}
				EndFight(a)
			}
			a.ChargeSince, a.NextCharge = 0, now+animal.ChargeCooldown
		}
		return
	}
	if a.Tame || !animal.Charger(a.Species) || now < a.NextCharge || (now+uint64(id))%animal.DecideEvery != 0 ||
		a.Target != 0 || a.Mode == animal.Flee || r.h.Asleep(*a) || BadlyHurt(*a) || a.Enraged(now) || now < a.FleeUntil ||
		!r.h.PredatorsHuntColonists() { // the difficulty (NG-1)
		return
	}
	who, _, ok := r.h.Nearest(pos.X, pos.Y, animal.ChargeRadius, Pawns|Raiders, Sight{R: r, Keep: standing})
	if !ok {
		return
	}
	a.NextCharge = now + animal.ChargeCooldown
	if r.h.Roll(100) >= animal.ChargePct {
		return
	}
	a.Mode, a.Target, a.HuntSince, a.ChargeSince = animal.Attack, who, 0, now
	a.Path, a.PathI, a.Feed, a.FeedLeft = nil, 0, animal.FeedTarget{}, 0
	r.h.Log(who, evlog.ChargedBy, id, r.h.KillerName(id))
	r.h.Log(id, evlog.Charged, who, r.h.CreatureName(who))
	r.WarnChase(id, a, who, true) // chase.go: "A cougar charges Ada!" (raiders are skipped)
}

// validPrey: a living animal of another species, or a standing colonist
// (a downed one too where predators hunt people: EAI-004).
func (r *Rage) validPrey(self, t entity.EntityID) bool {
	if t == self {
		return false
	}
	if o := r.h.Ref(t); o != nil {
		me := r.h.Ref(self)
		if me == nil {
			me = &animal.State{}
		}
		return !o.Dead() && (o.Species != me.Species || o.Tame != me.Tame)
	}
	return r.h.StandingPawn(t) || r.h.DownedPawn(t) && r.h.PredatorsHuntColonists()
}

// FindPrey: the nearest downed animal the predator can take within its
// hunt radius (Huntable), else (a predator, where the difficulty lets them
// hunt people: EAI-004) the nearest downed colonist there, else the
// nearest animal it can take at all, else a standing colonist within
// AggroRadius. A pack predator's mates nearby join the hunt.
func (r *Rage) FindPrey(id entity.EntityID, pos Pos, def animal.Def) entity.EntityID {
	best, _, ok := r.h.Nearest(pos.X, pos.Y, HuntRadius(def), Animals, Sight{R: r, Self: id, Keep: downedPrey}) // easy prey first
	if !ok && def.Predator && r.h.PredatorsHuntColonists() {
		if best, _, ok = r.h.Nearest(pos.X, pos.Y, HuntRadius(def), Pawns, Sight{R: r, Keep: downedPerson}); ok {
			return best
		}
	}
	if !ok {
		best, _, ok = r.h.Nearest(pos.X, pos.Y, HuntRadius(def), Animals, Sight{R: r, Self: id, Keep: prey})
	}
	if ok {
		if def.Pack {
			r.callPack(id, pos, best)
		}
		return best
	}
	if !r.h.PredatorsHuntColonists() { // the difficulty (NG-1)
		return 0
	}
	best, _, _ = r.h.Nearest(pos.X, pos.Y, def.AggroRadius, Pawns, Sight{R: r, Keep: standing})
	return best
}

// standing keeps a standing pawn; downedPerson a downed one.
func standing(s Sight, o entity.EntityID) bool { return s.R.h.StandingPawn(o) }

func downedPerson(s Sight, o entity.EntityID) bool { return s.R.h.DownedPawn(o) }

// prey keeps an animal s.Self can hunt; downedPrey a downed one.
func prey(s Sight, o entity.EntityID) bool { return s.R.Huntable(s.Self, o) }

func downedPrey(s Sight, o entity.EntityID) bool {
	a := s.R.h.Ref(o)
	return a != nil && a.Downed && s.R.Huntable(s.Self, o)
}

// HuntRadius is how far an animal looks for prey: its species' hunt
// radius, or smallPreyRadius for a meat eater that is no predator.
func HuntRadius(def animal.Def) int {
	if def.HuntRadius > 0 {
		return def.HuntRadius
	}
	return smallPreyRadius
}

// Huntable: prey a predator can take: a living non-predator no bigger
// than itself (XS or S only for a small-prey hunter, or a meat eater that
// is no predator), never a hive insect or a legendary. A downed animal is
// easy prey: a predator takes it whatever its size, a downed predator too.
func (r *Rage) Huntable(self, t entity.EntityID) bool {
	if !r.validPrey(self, t) {
		return false
	}
	o, ok := r.h.Animal(t)
	if !ok {
		return false
	}
	me, _ := r.h.Animal(self)
	od, md := animal.DefOf(o.Species), animal.DefOf(me.Species)
	if od.Hive || od.Legendary || od.Predator && !o.Downed {
		return false
	}
	size := animal.RosterOf(o.Species).Size
	if md.SmallPrey || !md.Predator {
		return size <= animal.SizeS
	}
	return o.Downed || size <= animal.RosterOf(me.Species).Size // a downed animal of any size is easy prey
}

// callPack sets idle wild pack mates of id's species within packRadius on
// the same prey.
func (r *Rage) callPack(id entity.EntityID, pos Pos, quarry entity.EntityID) {
	me, _ := r.h.Animal(id)
	now := r.h.Now()
	for _, aid := range r.h.Within(pos.X, pos.Y, packRadius, Animals) {
		if aid == id {
			continue
		}
		o, ok := r.h.Animal(aid)
		if !ok || o.Species != me.Species || o.Tame != me.Tame || o.Target != 0 || o.Manhunter != 0 || BadlyHurt(o) || !r.h.Grown(o, now) {
			continue
		}
		o.Target, o.HuntSince = quarry, now
		o.Feed, o.FeedLeft = animal.FeedTarget{}, 0
		r.h.SetAnimal(aid, o)
	}
}

// breakOff: a badly hurt fighter runs from its foe (T10).
func (r *Rage) breakOff(id entity.EntityID, a *animal.State, pos Pos, now uint64) {
	foe := a.Target
	if a.Attacker != 0 {
		foe = a.Attacker
	}
	EndFight(a)
	a.Attacker, a.FleeUntil, a.Mode = foe, now+BreakOffTicks, animal.Flee
	if fp, ok := r.h.Pos(foe); ok {
		fx, fy := animal.FleeGoal(pos.X, pos.Y, fp.X, fp.Y, 8)
		*a = r.h.GoTo(id, *a, fx, fy)
	}
	r.h.Log(id, evlog.Fled, foe, r.h.CreatureName(foe))
}

// EndFight drops an animal's target and hunt.
func EndFight(a *animal.State) {
	a.Target, a.HuntSince = 0, 0
	if a.Mode == animal.Attack {
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	}
}
