package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
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
	blasts  *Blasts
	drafts  *Drafts // DEF-4: fire at will and hold position
	gear    *Gear   // CBT-3: the weapon's make, shield belts, records
	// seen is AutoEngage's reused Near buffer (engage.go).
	seen []Seen
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
func (s *Strikes) Target(id entity.EntityID) (x, y int, ok bool) { return s.target(id, false) }

// target is Target; downed also accepts a humanlike lying downed (CBT-2:
// only an ordered attack goes for one, as in vanilla).
func (s *Strikes) target(id entity.EntityID, downed bool) (x, y int, ok bool) {
	h := s.h
	if a, ok := h.Animal(id); ok {
		if a.Dead() {
			return 0, 0, false
		}
		return h.Pos(id)
	}
	if h.StandingPawn(id) || downed && h.Lying(id) {
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
	s.strike(attacker, target, wk, s.h.Roll)
}

// def is attacker's profile with wk: an animal's natural weapon hits as
// hard and as surely as its owner is big, with its attack's damage type.
func (s *Strikes) def(attacker entity.EntityID, wk weapon.Kind) weapon.Def {
	_, d := s.natural(attacker, wk)
	return d
}

// natural is def with the weapon kind the blow is dealt with: an animal's
// own attack (its hit's form), else wk as its wielder's weapon was made
// (CBT-3: Gear.Def, quality and stuff).
func (s *Strikes) natural(attacker entity.EntityID, wk weapon.Kind) (weapon.Kind, weapon.Def) {
	d := weapon.DefOf(wk)
	if a, ok := s.h.Animal(attacker); ok {
		ad := animal.DefOf(a.Species)
		d.Damage, d.Accuracy, d.DamageType = ad.Damage, ad.Accuracy, weapon.DefOf(ad.Attack).DamageType
		return ad.Attack, d
	}
	return wk, s.gear.Def(attacker, wk)
}

// StrikeRoll is Strike with the hit rolled from roll (a brawl rolls from
// the social stream). A gun's shot is fired instead (Shots, rolled on the
// main stream); a blow on a sentry turret stays instant.
func (s *Strikes) StrikeRoll(attacker, target entity.EntityID, wk weapon.Kind, roll func(int) int) {
	s.strike(attacker, target, wk, roll)
}

// strike is StrikeRoll, returning the profile the attack was made with (a
// melee tool's, whose cooldown follows; a gun's when it fired). A
// humanlike beside its target fights with a tool (melee.go): its melee
// weapon's, or its gun's stock rather than a point-blank shot (CBT-2).
func (s *Strikes) strike(attacker, target entity.EntityID, wk weapon.Kind, roll func(int) int) weapon.Def {
	h := s.h
	hk, d := s.natural(attacker, wk)
	ax, ay, _ := h.Pos(attacker)
	if b, ok := s.turrets.At(target); ok { // PLAY-024: a raider attacking a sentry turret
		s.building(attacker, target, b.X, b.Y, ax, ay, d, roll)
		return d
	}
	tx, ty, ok := h.Pos(target)
	if !ok {
		return d
	}
	if s.melee(attacker, wk, d, ax, ay, tx, ty) {
		return s.blow(attacker, target, wk, hk, d, ax, ay, tx, ty, roll)
	}
	if s.gear.Shielded(attacker) {
		return d // CBT-3: a shield belt's wearer cannot shoot out
	}
	r := s.report(attacker, ax, ay, target, tx, ty, d, false)
	if d.Speed > 0 {
		s.shots.fire(attacker, target, ax, ay, tx, ty, wk, d, r, false, h.Roll)
		return d
	}
	hit := s.rolls(d, r, roll) // an instant gun (none is carried today)
	s.learn(attacker, d, hit)
	s.land(attacker, target, ax, ay, tx, ty, wk, d, weapon.HitOf(hk, d), hit, false, roll)
	return d
}

// land records an attack with d on target and, on a hit, runs hit
// through its armor and wounds it; dodged: a blow the defender dodged
// (CBT-3: the trace tells it from a miss).
func (s *Strikes) land(attacker, target entity.EntityID, ax, ay, tx, ty int, wk weapon.Kind, d weapon.Def, hit damage.Hit, ok, dodged bool, roll func(int) int) {
	h := s.h
	var l armor.Landed
	if ok {
		l = h.Absorb(target, hit, roll) // CBT-1: part by coverage, then its garments
	}
	tr := projectile.Trace{
		FromX: ax, FromY: ay, ToX: tx, ToY: ty,
		Tick: h.Now(), Hit: ok, Melee: !d.Ranged, Damage: max(l.Amount, 0),
		Attacker: attacker, Target: target, Weapon: d.Name, Dodged: dodged,
	}
	h.Trace(tr)
	h.Struck(attacker, target, tr.Tick)
	h.LogStrike(attacker, target, wk, ok, tr.Damage)
	if ok {
		s.wound(attacker, target, ax, ay, tx, ty, l)
	}
}

// learn gives a humanlike attacker the experience of one attack with d
// (attack.CombatXP: by its cycle, a hit teaching more than a miss).
func (s *Strikes) learn(attacker entity.EntityID, d weapon.Def, hit bool) {
	if _, beast := s.h.Animal(attacker); beast {
		return
	}
	s.h.Learn(attacker, d.Ranged, attack.CombatXP(hit, CycleTicks(d), attack.Burst{Shots: d.Burst}.Count()))
}

// rolls decides a hit at chance r: a legacy gun on a percent roll, a
// banded gun or a melee blow on a permille one.
func (s *Strikes) rolls(d weapon.Def, r attack.Report, roll func(int) int) bool {
	if d.Legacy() && d.Ranged {
		return roll(100) < r.Total()/10
	}
	return roll(attack.Permille(1000)) < r.Total()
}

// wound applies what got through armor to target, struck from (fx, fy)
// at (tx, ty): the hit stamp, blood and the wound its form leaves
// (Wounds.HurtHit; a sharp hit armor halved bruises), counted in the
// attacker's records (CBT-3: damage dealt, a kill when it was the killing
// blow). Nothing when armor stopped it.
func (s *Strikes) wound(attacker, target entity.EntityID, fx, fy, tx, ty int, l armor.Landed) {
	h := s.h
	if l.Stopped() {
		return
	}
	h.Hit(target, h.Now(), fx, fy, l.Amount)
	h.Stain(target, fx, fy, tx, ty, l.Amount) // before the hurt, while the target exists
	was := s.alive(target)
	h.HurtHit(target, l, attacker)
	s.gear.Count(attacker, target, l.Amount, was && !s.alive(target))
}

// alive: id is a living animal or a living humanlike (standing or lying
// downed).
func (s *Strikes) alive(id entity.EntityID) bool {
	if a, ok := s.h.Animal(id); ok {
		return !a.Dead()
	}
	return s.h.StandingPawn(id) || s.h.Lying(id)
}

// building is StrikeRoll on building target at (tx, ty) (a sentry turret):
// the attacker's hit chance (no cover: the turret is the cover), a trace
// marked Building, and DamageBuilding on a hit (a destroyed turret blows
// up: Turrets.Blast).
func (s *Strikes) building(attacker, target entity.EntityID, tx, ty, ax, ay int, d weapon.Def, roll func(int) int) {
	h := s.h
	r := s.report(attacker, ax, ay, target, tx, ty, d, true)
	hit := s.rolls(d, r, roll)
	s.learn(attacker, d, hit)
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
	wk := s.h.Weapon(shooter)
	d := s.def(shooter, wk)
	if !turret && s.melee(shooter, wk, d, ax, ay, tx, ty) {
		d.Ranged, d.Bands = false, attack.Bands{} // a gun's stock beside its target
	}
	return s.report(shooter, ax, ay, target, tx, ty, d, turret), true
}

// report is the hit chance of attacker at (ax, ay) on target at (tx, ty)
// with d. A melee blow hits by the wiki's melee hit curve (attack.
// HitChanceOf: skill, sight, manipulation; the dodge is rolled apart); a
// legacy gun at its flat accuracy; a banded gun multiplies its band at the
// straight-line distance, the shooter's accuracy per tile to the power of
// that distance, the target's size, light, the weather under the open sky
// and smoke, and what the target's cover lets through (none for a
// building: it is its own cover).
func (s *Strikes) report(attacker entity.EntityID, ax, ay int, target entity.EntityID, tx, ty int, d weapon.Def, building bool) attack.Report {
	h := s.h
	dist := weapon.Range(ax, ay, tx, ty)
	r := attack.Report{Dist: dist, InRange: weapon.ReachesAt(d, ax, ay, tx, ty), Melee: !d.Ranged,
		Shooter: 1000, Size: 1000, Light: attack.NoLightPenalty, Weather: attack.ClearFactor}
	r.InSight = r.InRange && (!d.Ranged || weapon.Distance(ax, ay, tx, ty) <= 1 || s.sight.Line(ax, ay, tx, ty))
	if d.Ranged && !building {
		r.Cover = s.sight.Cover(ax, ay, tx, ty)
	}
	if d.Ranged {
		r.Weather = s.sight.Weather(ax, ay, tx, ty) * s.blasts.SmokeFactor(ax, ay, tx, ty) / 1000 // DEF-2: smoke in the way
	}
	switch {
	case !d.Ranged:
		r.Band = attack.HitChanceOf(h.Melee(attacker)) // CBT-2: wiki: Melee hit chance
	case d.Legacy():
		r.Band = weapon.HitChance(d, dist) * 10
	default:
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
	return BodySize(a)
}

// BodySize is animal a's body size, permille of a human's: its size
// class's (sizeBody) trimmed by its roster's SizeTrim (1000 for an
// unknown class).
func BodySize(a animal.State) int {
	r := animal.RosterOf(a.Species)
	if int(r.Size) >= len(sizeBody) {
		return 1000
	}
	return sizeBody[r.Size] * (100 + r.SizeTrim) / 100
}

// Do runs a Hunt or Attack job: close in until the target is in range
// (and in sight for guns), then attack on cooldown. Only an ordered
// (Forced) attack goes on at a humanlike lying downed (CBT-2).
func (s *Strikes) Do(id entity.EntityID, j job.Record, now uint64) {
	h := s.h
	tx, ty, ok := s.target(j.Target, j.Forced && j.Kind == job.Attack)
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
	d := s.gear.Reach(id, wk) // CBT-3: as made; a shield belt's wearer only clubs
	if j.WorkLeft > 0 {
		j.WorkLeft--
	}
	if s.sight.CanHit(px, py, d, tx, ty) {
		j.State, j.Path, j.PathI = job.Do, nil, 0
		if j.WorkLeft == 0 {
			used := s.strike(id, j.Target, wk, h.Roll)
			if !used.Ranged {
				j.Count = 0 // a blow ends any burst (a gun's stock)
			}
			j.WorkLeft = s.next(id, used, &j)
			if used.Ranged && j.Count == 0 && s.gear.Spend(id, wk) {
				j.State = job.Complete // CBT-3: a rocket launcher's burst used it up
			}
		}
		h.SetJob(id, j)
		return
	}
	if !j.Forced && s.drafts != nil && s.drafts.Holding(id) && h.Drafted(id) {
		j.State, j.Path = job.Complete, nil // holding position: never chases (DEF-4)
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
