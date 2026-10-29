package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Strikes: creatures wield a weapon (fists when unarmed, an animal its
// natural weapon), attacks resolve instantly with a deterministic roll,
// and every attack leaves a short trace for presentation. Drafted pawns
// attack on order and fire at hostiles in range; hunters close in on their
// prey.
type Strikes struct {
	h       Host
	sight   *Sight
	turrets *Turrets
	hunts   Hunts
}

// AimTicks is the warm-up before a first attack on a new target.
func AimTicks(d weapon.Def) int { return d.Cooldown / 3 }

// Target is the tile of a target that can still be attacked: a living
// animal, a standing colonist or a sentry turret (raiders turn on
// turrets).
func (s *Strikes) Target(id entity.EntityID) (x, y int, ok bool) {
	h := s.h
	if a, ok := h.Animal(id); ok {
		if a.Dead() {
			return 0, 0, false
		}
		return h.Pos(id)
	}
	if h.StandingPawn(id) {
		return h.Pos(id)
	}
	if b, ok := s.turrets.At(id); ok {
		return b.X, b.Y, true
	}
	return 0, 0, false
}

// Strike resolves one attack, rolled on the main stream, and records its
// trace.
func (s *Strikes) Strike(attacker, target entity.EntityID, wk weapon.Kind) {
	s.StrikeRoll(attacker, target, wk, s.h.Roll)
}

// StrikeRoll is Strike with the hit rolled from roll (a brawl rolls from
// the social stream).
func (s *Strikes) StrikeRoll(attacker, target entity.EntityID, wk weapon.Kind, roll func(int) int) {
	h := s.h
	d := weapon.DefOf(wk)
	if a, ok := h.Animal(attacker); ok { // a natural weapon hits as hard as its owner is big
		ad := animal.DefOf(a.Species)
		d.Damage, d.Accuracy = ad.Damage, ad.Accuracy
	}
	ax, ay, _ := h.Pos(attacker)
	if b, ok := s.turrets.At(target); ok { // PLAY-024: a raider shooting a sentry turret
		s.building(attacker, target, b.X, b.Y, ax, ay, d, roll)
		return
	}
	tx, ty, ok := h.Pos(target)
	if !ok {
		return
	}
	chance := weapon.HitChance(d, weapon.Distance(ax, ay, tx, ty))
	if _, beast := h.Animal(attacker); !beast {
		chance = h.HitChance(attacker, d.Ranged, chance) // Shooting/Melee, accuracy stats
		h.Learn(attacker, d.Ranged)
	}
	if d.Ranged {
		chance = chance * (100 - s.sight.Cover(ax, ay, tx, ty)) / 100
	}
	hit := roll(100) < chance
	tr := projectile.Trace{
		FromX: ax, FromY: ay, ToX: tx, ToY: ty,
		Tick: h.Now(), Hit: hit, Melee: !d.Ranged,
		Attacker: attacker, Target: target, Weapon: d.Name,
	}
	if hit {
		tr.Damage = d.Damage
	}
	h.Trace(tr)
	h.Struck(attacker, target, tr.Tick)
	h.LogStrike(attacker, target, wk, hit, tr.Damage)
	if hit {
		h.Hit(target, tr.Tick, ax, ay, d.Damage)
		h.Stain(target, ax, ay, tx, ty, d.Damage) // before the hurt, while the target exists
		h.Hurt(target, d.Damage, attacker)
	}
}

// building is StrikeRoll on building target at (tx, ty) (a sentry turret):
// the attacker's hit chance (no cover: the turret is the cover), a trace
// marked Building, and DamageBuilding on a hit (a destroyed turret blows
// up: Turrets.Blast).
func (s *Strikes) building(attacker, target entity.EntityID, tx, ty, ax, ay int, d weapon.Def, roll func(int) int) {
	h := s.h
	chance := weapon.HitChance(d, weapon.Distance(ax, ay, tx, ty))
	if _, beast := h.Animal(attacker); !beast {
		chance = h.HitChance(attacker, d.Ranged, chance)
		h.Learn(attacker, d.Ranged)
	}
	hit := roll(100) < chance
	tr := projectile.Trace{
		FromX: ax, FromY: ay, ToX: tx, ToY: ty,
		Tick: h.Now(), Hit: hit, Melee: !d.Ranged, Building: true,
		Attacker: attacker, Weapon: d.Name,
	}
	if hit {
		tr.Damage = d.Damage
	}
	h.Trace(tr)
	h.Struck(attacker, target, tr.Tick)
	if hit {
		h.DamageBuilding(target, d.Damage)
	}
}

// Do runs a Hunt or Attack job: close in until the target is in range
// (and in sight for guns), then attack on cooldown.
func (s *Strikes) Do(id entity.EntityID, j job.Record, now uint64) {
	h := s.h
	tx, ty, ok := s.Target(j.Target)
	if ok && !h.CanViolence(id) {
		ok = false // incapable of violence: never attacks or hunts
	}
	if ok && j.Kind == job.Hunt {
		a, _ := h.Animal(j.Target)
		ok = a.Designated
	}
	if !ok {
		s.hunts.ClearHuntOut(id)
		j.State, j.Path = job.Complete, nil
		h.SetJob(id, j)
		return
	}
	if j.Kind == job.Hunt && s.hunts.HuntGivenUp(id, j, tx, ty, now) { // the prey stayed out of the area
		s.hunts.AbandonHunt(id, j)
		return
	}
	px, py, _ := h.Pos(id)
	wk := h.Weapon(id)
	d := weapon.DefOf(wk)
	if j.WorkLeft > 0 {
		j.WorkLeft--
	}
	if s.sight.CanHit(px, py, d, tx, ty) {
		j.State, j.Path, j.PathI = job.Do, nil, 0
		if j.WorkLeft == 0 {
			s.Strike(id, j.Target, wk)
			j.WorkLeft = h.AimTicks(id, d.Cooldown)
		}
		h.SetJob(id, j)
		return
	}
	stale := j.State != job.Goto || j.PathI >= len(j.Path) ||
		weapon.Distance(j.TX, j.TY, tx, ty) > 1 || (now+uint64(id))%60 == 0
	if stale {
		path, ok := h.PathTo(id, tx, ty)
		if !ok {
			j.State, j.Fail, j.Path = job.Fail, job.Unreachable, nil
			h.SetJob(id, j)
			return
		}
		j.Path, j.PathI, j.TX, j.TY = path, 0, tx, ty
		j.State = job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
	}
	h.SetJob(id, j)
}

// AutoEngage makes an idle drafted pawn attack the nearest hostile it can
// hit from where it stands.
func (s *Strikes) AutoEngage(id entity.EntityID) {
	h := s.h
	if !h.CanViolence(id) {
		return
	}
	px, py, _ := h.Pos(id)
	d := weapon.DefOf(h.Weapon(id))
	best, bestD := entity.EntityID(0), 1<<30
	for _, aid := range h.AnimalIDs() {
		a, ok := h.Animal(aid)
		if !ok || !h.Hostile(a) {
			continue
		}
		ax, ay, _ := h.Pos(aid)
		dist := weapon.Distance(px, py, ax, ay)
		if dist < bestD && s.sight.CanHit(px, py, d, ax, ay) {
			best, bestD = aid, dist
		}
	}
	for _, rid := range h.RaiderIDs() {
		if !h.StandingRaider(rid) {
			continue
		}
		rx, ry, _ := h.Pos(rid)
		dist := weapon.Distance(px, py, rx, ry)
		if dist < bestD && s.sight.CanHit(px, py, d, rx, ry) {
			best, bestD = rid, dist
		}
	}
	if best == 0 {
		return
	}
	bx, by, _ := h.Pos(best)
	h.StartJob(id, job.Record{Kind: job.Attack, Target: best, TX: bx, TY: by, WorkLeft: AimTicks(d)})
}
