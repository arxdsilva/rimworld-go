package rage

import (
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
)

// Rage is the animals' fight-or-flight brain over the host's animals.
// It keeps no state of its own: every animal's lives in its animal.State.
type Rage struct {
	h Host
}

// New is the rage over host h.
func New(h Host) *Rage { return &Rage{h: h} }

const (
	// legendaryRadius: legendaries this close answer one's rage.
	legendaryRadius = 30
	// guardRadius: a tame guard looks this far for a threat.
	guardRadius = 10
	// packRadius: pack mates this close join a hunt.
	packRadius = 10
	// territorialLeash: a woken territorial animal chases its attacker this
	// far before letting it go.
	territorialLeash = 12
	// smallPreyRadius: how far a meat eater that is no predator looks for
	// small prey.
	smallPreyRadius = 12
	// BreakOffTicks: a badly hurt fighter runs this long (T10).
	BreakOffTicks = 900
)

// pos is id's cell (the zero cell when it has none).
func (r *Rage) pos(id entity.EntityID) Pos {
	p, _ := r.h.Pos(id)
	return p
}

// BadlyHurt: below animal.BadlyHurt percent of its hit points.
func BadlyHurt(a animal.State) bool {
	return a.HP()*100 < animal.DefOf(a.Species).MaxHP*animal.BadlyHurt
}

// Enrage turns a wild animal manhunter for a day, going for who; a
// legendary's rage calls every legendary nearby.
func (r *Rage) Enrage(id entity.EntityID, a *animal.State, who entity.EntityID, now uint64) {
	if a.Tame || a.Enraged(now) {
		return
	}
	a.Manhunter = now + animal.ManhunterTicks
	a.Mode, a.Target, a.TameMark, a.FleeUntil = animal.Attack, who, false, 0
	a.Path, a.PathI = nil, 0
	if a.Downed { // enraged, but it cannot hunt until it stands
		a.Mode, a.Target = animal.Wander, 0
	}
	r.h.Log(id, evlog.Manhunter, who, "")
	p, _ := r.h.Pos(id)
	r.h.Alarm(now, p.X, p.Y, id, "", msgfmt.Of("A %s has gone manhunter!", a.Species.Label()))
	if animal.DefOf(a.Species).Legendary {
		r.rallyLegendaries(id, who, now)
	}
}

// rallyLegendaries: a legendary's rage calls every wild legendary within
// legendaryRadius to the same fight.
func (r *Rage) rallyLegendaries(id entity.EntityID, who entity.EntityID, now uint64) {
	pos := r.pos(id)
	for _, aid := range r.h.Within(pos.X, pos.Y, legendaryRadius, Animals) {
		if aid == id {
			continue
		}
		o, ok := r.h.Animal(aid)
		if !ok || o.Tame || o.Enraged(now) || !animal.DefOf(o.Species).Legendary {
			continue
		}
		o.Manhunter = now + animal.ManhunterTicks
		o.Mode, o.Target, o.TameMark, o.FleeUntil, o.Path, o.PathI = animal.Attack, who, false, 0, nil, 0
		r.h.SetAnimal(aid, o)
		r.h.Log(aid, evlog.Manhunter, who, "")
	}
}

// Provoked: a wild animal hurt by a colonist may go manhunter.
func (r *Rage) Provoked(id entity.EntityID, a *animal.State, by entity.EntityID) {
	if a.Tame || a.Dead() || by == 0 || !r.h.IsPawn(by) || r.h.IsRaider(by) {
		return
	}
	now := r.h.Now()
	if !a.Enraged(now) && r.h.Roll(100) < r.h.RevengePct(animal.DefOf(a.Species).Revenge) { // the difficulty (NG-1)
		r.Enrage(id, a, by, now)
	}
}

// ThinkManhunter hunts the nearest standing colonist anywhere on the map.
// It reports false once the rage has passed.
func (r *Rage) ThinkManhunter(id entity.EntityID, a *animal.State, now uint64) bool {
	if !a.Enraged(now) {
		if a.Manhunter != 0 {
			a.Manhunter, a.Target, a.Mode = 0, 0, animal.Wander
		}
		return false
	}
	if a.Target == 0 || !r.h.StandingPawn(a.Target) || r.h.IsRaider(a.Target) {
		a.Target = r.h.NearestColonist(r.pos(id))
	}
	if a.Target == 0 {
		return false // nobody left standing: calm down for now
	}
	r.Chase(id, a, now)
	return true
}

// Chase closes on a.Target and bites on cooldown.
func (r *Rage) Chase(id entity.EntityID, a *animal.State, now uint64) {
	def := animal.DefOf(a.Species)
	pos := r.pos(id)
	a.Mode = animal.Attack
	tp := r.pos(a.Target)
	if weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) <= 1 {
		a.Path, a.PathI = nil, 0
		if a.Cooldown == 0 {
			a.Cooldown = def.Cooldown
			if a.HuntSince != 0 {
				a.HuntSince = now // a strike: the hunt is still on (T5)
			}
			r.h.SetAnimal(id, *a)
			r.h.Strike(id, a.Target, def.Attack)
			if cur, ok := r.h.Animal(id); ok {
				*a = cur // a kill updates this animal (hunger, target)
			}
		}
		return
	}
	if now < a.RetryAt && a.PathI >= len(a.Path) {
		return // backing off after a failed path search
	}
	if a.PathI >= len(a.Path) || weapon.Distance(a.GoalX, a.GoalY, tp.X, tp.Y) > 1 || (now+uint64(id))%30 == 0 {
		*a = r.h.GoTo(id, *a, tp.X, tp.Y)
	}
}

// ManhunterPack sends n enraged animals of one season-fitting grazer or
// territorial species in from a map edge (half as many of a large one).
func (r *Rage) ManhunterPack(n int) {
	s, ok := r.h.PickSpecies(ManhunterSpecies)
	if !ok {
		return
	}
	ex, ey, ok := r.h.EdgeTile()
	if !ok {
		return
	}
	if animal.RosterOf(s).Size >= animal.SizeL {
		n = (n + 1) / 2
	}
	now := r.h.Now()
	spawned := 0
	for i := 0; i < n; i++ {
		for try := 0; try < 10; try++ {
			x, y := ex+r.h.Roll(5)-2, ey+r.h.Roll(5)-2
			if !r.h.InBounds(x, y) {
				continue
			}
			if id, err := r.h.Spawn(s, x, y); err == nil {
				a, _ := r.h.Animal(id)
				a.Manhunter, a.Mode = now+animal.ManhunterTicks, animal.Attack
				r.h.SetAnimal(id, a)
				spawned++
				break
			}
		}
	}
	if spawned == 0 {
		return
	}
	name := animal.Plural(s)
	if spawned == 1 {
		name = s.Label()
	}
	r.h.Threat("Manhunter pack", ex, ey, "A manhunter pack of %d %s is charging the colony!", spawned, name)
}

// ManhunterSpecies: who forms manhunter packs (grazers and territorial
// animals).
func ManhunterSpecies(s animal.Species) bool {
	b := animal.RosterOf(s).Behaviour
	return b == animal.Grazer || b == animal.Territorial
}
