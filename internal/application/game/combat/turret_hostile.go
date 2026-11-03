package combat

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// A settlement's turret (MAP-13, Building.Hostile): it stands on a caravan
// attack's map, needs no power and is on no grid, fires on colonists, tame
// animals and manhunters (wiki: Mini-turret), and stays hostile after the
// settlement falls (wiki: Faction base). Nobody at the colony runs it, so a
// spent one goes quiet until claimed; a drafted colonist shoots it (an order,
// or fire at will when no creature is nearer). Claimed once no defender
// stands (orders/claim.go), it keeps its own supply (Building.OffGrid) and is
// ours: it fires on what ours fire on and a hauler reloads it. The lock keeps
// the usual two ranges: it picks up within the gun's reach and lets go past
// the release margin.

// searchOf is the creatures turret b looks through.
func searchOf(b construction.Building) Who {
	if b.Hostile {
		return Pawns | Animals
	}
	return Raiders | Animals
}

// aimsAt reports s a target of turret b, by b's side.
func (t *Turrets) aimsAt(b construction.Building, s Seen, now uint64) bool {
	if b.Hostile {
		return t.colonyTarget(s, now)
	}
	return t.Targetable(s.ID, now)
}

// colonyTarget is Targetable's sibling for a settlement's turret: a tame or
// manhunting animal, or a colonist on its feet. The colonists' roster
// decides, since a raider, visitor or prisoner stands up just the same; a
// raider counts a berserk colonist as its own, this turret does not.
func (t *Turrets) colonyTarget(s Seen, now uint64) bool {
	if a, ok := t.h.Animal(s.ID); ok {
		return !a.Dead() && (a.Tame || a.Enraged(now))
	}
	if !t.h.StandingPawn(s.ID) {
		return false
	}
	t.roster = t.h.Near(s.X, s.Y, 0, Pawns, t.roster[:0])
	return slices.ContainsFunc(t.roster, func(r Seen) bool { return r.ID == s.ID })
}

// HostileAt is the settlement's turret on (x, y) (0: none).
func (t *Turrets) HostileAt(x, y int) entity.EntityID {
	id, k := t.h.Object(x, y)
	if id == 0 || !isTurret(k) {
		return 0
	}
	if b, ok := t.h.Building(id); !ok || !b.Hostile {
		return 0
	}
	return id
}

// hostileInReach is the nearest settlement's turret that can still fire
// and that a colonist at (x, y) with d can hit, ties to the lower id. ok
// false: none (at once on a map with none).
func (t *Turrets) hostileInReach(x, y int, d weapon.Def, sight *Sight) (id entity.EntityID, bx, by int, ok bool) {
	if !t.pw.Armed() {
		return 0, 0, 0, false
	}
	bestD := 0
	for _, tid := range t.list() {
		b, found := t.h.Building(tid)
		if !found || !b.Hostile || t.Spent(tid, b.Kind) || !sight.CanHit(x, y, d, b.X, b.Y) {
			continue
		}
		if dist := weapon.Distance(x, y, b.X, b.Y); !ok || dist < bestD {
			id, bx, by, bestD, ok = tid, b.X, b.Y, dist, true
		}
	}
	return id, bx, by, ok
}

// hostileLive reports a settlement's turret on the map that can still fire.
func (t *Turrets) hostileLive() bool {
	if !t.pw.Armed() {
		return false
	}
	for _, tid := range t.list() {
		if b, ok := t.h.Building(tid); ok && b.Hostile && !t.Spent(tid, b.Kind) {
			return true
		}
	}
	return false
}

// OrderedAt is what a drafted colonist's Attack order on (x, y) goes for:
// the animal there, else the raider (Ordered), else a settlement's turret or
// door (0: nothing to attack). A door is only ever an order: AutoEngage looks
// through the turret list (hostileInReach).
func (s *Strikes) OrderedAt(beast, raider entity.EntityID, x, y int) entity.EntityID {
	switch {
	case beast != 0:
		return beast
	case raider != 0 && s.Ordered(raider):
		return raider
	}
	if id := s.turrets.HostileAt(x, y); id != 0 {
		return id
	}
	return enemyDoorAt(s.h, x, y)
}

// buildingTarget is target id as a building a strike deals with: a turret, or
// a settlement's door (MAP-17).
func (s *Strikes) buildingTarget(id entity.EntityID) (construction.Building, bool) {
	if b, ok := s.turrets.At(id); ok {
		return b, true
	}
	if b, ok := s.h.Building(id); ok && EnemyDoor(b) {
		return b, true
	}
	return construction.Building{}, false
}

// yieldsTurret reports a colonist at (x, y) dropping its own attack on a
// settlement's turret: a foe creature in reach stands SwitchMargin closer
// than the turret. AutoEngage turns on the turret only when it is
// SwitchMargin closer than the creature, so the two never trade places.
func (s *Strikes) yieldsTurret(target entity.EntityID, x, y int, d weapon.Def, now uint64) bool {
	b, ok := s.turrets.At(target)
	if !ok || !b.Hostile {
		return false
	}
	limit := weapon.Distance(x, y, b.X, b.Y) - SwitchMargin
	if limit < 0 {
		return false
	}
	s.seen = s.h.Near(x, y, limit, Animals|Raiders, s.seen[:0])
	for _, c := range s.seen {
		if s.foeID(c.ID, now) && s.sight.CanHit(x, y, d, c.X, c.Y) {
			return true
		}
	}
	return false
}
