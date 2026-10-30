package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// AutoEngage makes an idle drafted pawn attack the nearest hostile it can
// hit from where it stands (CBT-2: any hostile, not only one attacking a
// colonist: raiders, manhunters, mechanoids, insects; found through the
// creature index within the weapon's reach, never a scan of every
// creature). It also runs the pawn's auto-undraft timer (Drafts.idle):
// it is called only while the pawn stands idle.
func (s *Strikes) AutoEngage(id entity.EntityID) {
	h := s.h
	now := h.Now()
	if s.drafts != nil && s.drafts.idle(id, now, func() bool { return s.Threatened(now) }) {
		return // released: nothing on the map for AutoUndraft ticks (draft.go)
	}
	if !h.CanViolence(id) || s.drafts != nil && !s.drafts.FireAtWill(id) {
		return // DEF-4: fire at will off, it shoots only when ordered
	}
	px, py, _ := h.Pos(id)
	d := weapon.DefOf(h.Weapon(id))
	s.seen = h.Near(px, py, d.Range+1, Animals|Raiders, s.seen[:0])
	best, bestSq := entity.EntityID(0), 1<<62
	for _, c := range s.seen {
		sq := (c.X-px)*(c.X-px) + (c.Y-py)*(c.Y-py)
		if sq > bestSq || sq == bestSq && c.ID > best || !s.foeID(c.ID, now) || !s.sight.CanHit(px, py, d, c.X, c.Y) {
			continue
		}
		best, bestSq = c.ID, sq
	}
	if best == 0 {
		return
	}
	bx, by, _ := h.Pos(best)
	rec := job.Record{Kind: job.Attack, Target: best, TX: bx, TY: by, WorkLeft: AimTicks(d)}
	if s.drafts != nil && s.drafts.Holding(id) {
		rec.State = job.Do // in reach from its tile: no step towards it
		h.SetJob(id, rec)
		return
	}
	h.StartJob(id, rec)
}

// AttackVerb is the order menu's verb for an attack with a ranged weapon
// or in melee ("Shoot ", "Melee attack "; the target's name follows).
func AttackVerb(ranged bool) string {
	if ranged {
		return "Shoot "
	}
	return "Melee attack "
}

// Ordered reports whether humanlike id may be attacked on a drafted
// colonist's order: a raider on its feet, or one lying downed (CBT-2:
// only an order goes for a downed one, as in vanilla; Do keeps at it).
func (s *Strikes) Ordered(id entity.EntityID) bool { return s.h.StandingRaider(id) || s.h.Lying(id) }

// foeID reports creature id hostile to the colony: a raider on its feet
// or a hostile animal (foe).
func (s *Strikes) foeID(id entity.EntityID, now uint64) bool {
	if a, ok := s.h.Animal(id); ok {
		return s.foe(a, now)
	}
	return s.h.StandingRaider(id)
}

// foe reports animal a hostile to the colony (CBT-2): alive, wild and
// awake, and attacking the colony, enraged (manhunter), a mechanoid or a
// hive insect.
func (s *Strikes) foe(a animal.State, now uint64) bool {
	if a.Dead() || a.Tame || a.Dormant {
		return false
	}
	return s.h.Hostile(a) || a.Manhunter > now || animal.IsMech(a.Species) || animal.DefOf(a.Species).Hive
}

// Threatened reports a hostile anywhere on the map: a raider on its feet
// or a hostile animal (a scan of the raiders and animals; the auto-undraft
// timer asks once in threatEvery ticks per pawn).
func (s *Strikes) Threatened(now uint64) bool {
	h := s.h
	for _, rid := range h.RaiderIDs() {
		if h.StandingRaider(rid) {
			return true
		}
	}
	for _, aid := range h.AnimalIDs() {
		if a, ok := h.Animal(aid); ok && s.foe(a, now) {
			return true
		}
	}
	return false
}
