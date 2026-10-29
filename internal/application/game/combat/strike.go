package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Strikes: creatures wield a weapon (fists when unarmed, an animal its
// natural weapon). A blow resolves at once with a deterministic roll; a
// gun with a projectile speed fires a shot that flies (shot.go, DEF-1).
// Every attack leaves traces for presentation. Worn armor deflects or
// reduces a hit (armor.Hit). Drafted pawns attack on order and fire at
// hostiles in range; hunters close in on their prey.
type Strikes struct {
	h       Host
	sight   *Sight
	turrets *Turrets
	hunts   Hunts
	shots   *Shots
	// layers is Armor's reused buffer.
	layers []armor.Layer
}

// AimTicks is the warm-up before a first attack on a new target: the
// weapon's warmup (none for the wiki's melee weapons).
func AimTicks(d weapon.Def) int { return d.Warmup }

// CycleTicks is the wait after an attack with d for an attacker with
// neutral aiming: the cooldown and, for a gun, the next aim (bursts aside).
func CycleTicks(d weapon.Def) int {
	if d.Ranged {
		return d.Cooldown + d.Warmup
	}
	return d.Cooldown
}

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

// def is attacker's profile with wk: an animal's natural weapon hits as
// hard and as surely as its owner is big, with its attack's damage type.
func (s *Strikes) def(attacker entity.EntityID, wk weapon.Kind) weapon.Def {
	d := weapon.DefOf(wk)
	if a, ok := s.h.Animal(attacker); ok {
		ad := animal.DefOf(a.Species)
		d.Damage, d.Accuracy, d.DamageType = ad.Damage, ad.Accuracy, weapon.DefOf(ad.Attack).DamageType
	}
	return d
}

// StrikeRoll is Strike with the hit rolled from roll (a brawl rolls from
// the social stream). A gun's shot is fired instead (Shots, rolled on the
// main stream); a blow on a sentry turret stays instant.
func (s *Strikes) StrikeRoll(attacker, target entity.EntityID, wk weapon.Kind, roll func(int) int) {
	h := s.h
	d := s.def(attacker, wk)
	ax, ay, _ := h.Pos(attacker)
	if b, ok := s.turrets.At(target); ok { // PLAY-024: a raider attacking a sentry turret
		s.building(attacker, target, b.X, b.Y, ax, ay, d, roll)
		return
	}
	tx, ty, ok := h.Pos(target)
	if !ok {
		return
	}
	r := s.report(attacker, ax, ay, target, tx, ty, d, false)
	if _, beast := h.Animal(attacker); !beast {
		h.Learn(attacker, d.Ranged)
	}
	if d.Ranged && d.Speed > 0 {
		s.shots.fire(attacker, target, ax, ay, tx, ty, wk, d, r)
		return
	}
	hit := s.rolls(d, r, roll)
	res := armor.Result{}
	if hit {
		res = s.armorRoll(target, d.DamageType, d.Damage, weapon.Penetration(d, d.Damage), roll)
	}
	tr := projectile.Trace{
		FromX: ax, FromY: ay, ToX: tx, ToY: ty,
		Tick: h.Now(), Hit: hit, Melee: !d.Ranged, Damage: res.Damage,
		Attacker: attacker, Target: target, Weapon: d.Name,
	}
	h.Trace(tr)
	h.Struck(attacker, target, tr.Tick)
	h.LogStrike(attacker, target, wk, hit, tr.Damage)
	if hit {
		s.wound(attacker, target, ax, ay, tx, ty, d.DamageType, res)
	}
}

// rolls decides a hit at chance r: a legacy weapon on a percent roll, a
// banded gun on a permille one.
func (s *Strikes) rolls(d weapon.Def, r attack.Report, roll func(int) int) bool {
	if d.Legacy() {
		return roll(100) < r.Total()/10
	}
	return roll(attack.Permille(1000)) < r.Total()
}

// armorRoll runs a hit of amount (type t, penetration pen) through what
// target wears: one roll for the body region it lands on, one per layer
// covering it. A target wearing nothing draws no roll.
func (s *Strikes) armorRoll(target entity.EntityID, t damage.Type, amount, pen int, roll func(int) int) armor.Result {
	s.layers = s.h.Armor(target, s.layers[:0])
	if len(s.layers) == 0 {
		return armor.Result{Outcome: armor.Passed, Damage: amount, Type: t}
	}
	return armor.Hit(s.layers, armor.RegionOf(roll(armor.RollMax)), t, amount, armor.Permille(pen), roll)
}

// wound applies what got through armor to target, struck from (fx, fy)
// at (tx, ty): the hit stamp, blood and the wound (a sharp hit that armor
// turned blunt bruises). Nothing when armor stopped it.
func (s *Strikes) wound(attacker, target entity.EntityID, fx, fy, tx, ty int, t damage.Type, res armor.Result) {
	h := s.h
	if res.Damage <= 0 {
		return
	}
	h.Hit(target, h.Now(), fx, fy, res.Damage)
	h.Stain(target, fx, fy, tx, ty, res.Damage) // before the hurt, while the target exists
	if res.Type != t {
		h.HurtAs(target, res.Damage, attacker, body.Bruise, health.CauseInjury)
		return
	}
	h.Hurt(target, res.Damage, attacker)
}

// building is StrikeRoll on building target at (tx, ty) (a sentry turret):
// the attacker's hit chance (no cover: the turret is the cover), a trace
// marked Building, and DamageBuilding on a hit (a destroyed turret blows
// up: Turrets.Blast).
func (s *Strikes) building(attacker, target entity.EntityID, tx, ty, ax, ay int, d weapon.Def, roll func(int) int) {
	h := s.h
	r := s.report(attacker, ax, ay, target, tx, ty, d, true)
	if _, beast := h.Animal(attacker); !beast {
		h.Learn(attacker, d.Ranged)
	}
	hit := s.rolls(d, r, roll)
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

// Report is shooter's hit chance on target from where both stand, with
// its factors (the hit-chance tooltip); ok false when either has no tile.
func (s *Strikes) Report(shooter, target entity.EntityID) (attack.Report, bool) {
	ax, ay, ok := s.h.Pos(shooter)
	if !ok {
		return attack.Report{}, false
	}
	tx, ty, ok := s.Target(target)
	if !ok {
		return attack.Report{}, false
	}
	_, turret := s.turrets.At(target)
	return s.report(shooter, ax, ay, target, tx, ty, s.def(shooter, s.h.Weapon(shooter)), turret), true
}

// report is the hit chance of attacker at (ax, ay) on target at (tx, ty)
// with d. A melee weapon (or a legacy gun) hits at its flat accuracy,
// adjusted for a humanlike's skill; a banded gun multiplies its band at
// the distance, the shooter's accuracy per tile to the power of the
// distance, the target's size, light and weather, and what the target's
// cover lets through (none for a building: it is its own cover).
func (s *Strikes) report(attacker entity.EntityID, ax, ay int, target entity.EntityID, tx, ty int, d weapon.Def, building bool) attack.Report {
	h := s.h
	dist := weapon.Distance(ax, ay, tx, ty)
	r := attack.Report{Dist: dist, InRange: weapon.InRange(d, dist), Melee: !d.Ranged,
		Shooter: 1000, Size: 1000, Light: attack.NoLightPenalty, Weather: attack.ClearFactor}
	r.InSight = r.InRange && (!d.Ranged || dist <= 1 || s.sight.Line(ax, ay, tx, ty))
	if d.Ranged && !building {
		r.Cover = s.sight.Cover(ax, ay, tx, ty) * 10
	}
	_, beast := h.Animal(attacker)
	if d.Legacy() {
		c := weapon.HitChance(d, dist)
		if !beast {
			c = h.HitChance(attacker, d.Ranged, c) // Melee (or Shooting), accuracy stats
		}
		r.Band = c * 10
	} else {
		pt, off := h.ShotAim(attacker)
		r.Band, r.Shooter, r.Offset = d.Bands.Accuracy(dist), attack.ShooterFactor(pt, dist), off
		if !building {
			r.Size = attack.SizeFactor(s.bodySize(target))
		}
	}
	r.Chance = r.Total()
	return r
}

// sizeBody is a size class's body size (permille of a human's). original
// tuning: the classes are ours, the scale is the wiki's (a human is 1).
var sizeBody = [...]int{animal.SizeXS: 250, animal.SizeS: 500, animal.SizeM: 1000, animal.SizeL: 2000, animal.SizeXL: 3500}

// bodySize is id's body size, permille (humanlikes 1000).
func (s *Strikes) bodySize(id entity.EntityID) int {
	a, ok := s.h.Animal(id)
	if !ok {
		return 1000
	}
	r := animal.RosterOf(a.Species)
	if int(r.Size) >= len(sizeBody) {
		return 1000
	}
	return sizeBody[r.Size] * (100 + r.SizeTrim) / 100
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
			j.WorkLeft = s.next(id, d, &j)
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

// next is the wait after an attack with d: BurstTicks to the burst's next
// shot (j.Count holds the shots left in the burst), else the cooldown and,
// for a gun, the aim before the next burst. A melee cooldown and a gun's
// aim scale with the attacker's aiming (Skills.AimTicks).
func (s *Strikes) next(id entity.EntityID, d weapon.Def, j *job.Record) int {
	if j.Count == 0 {
		j.Count = attack.Burst{Shots: d.Burst}.Count()
	}
	if j.Count--; j.Count > 0 {
		return max(d.BurstTicks, 1)
	}
	if !d.Ranged {
		return s.h.AimTicks(id, d.Cooldown)
	}
	return d.Cooldown + s.h.AimTicks(id, d.Warmup)
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
