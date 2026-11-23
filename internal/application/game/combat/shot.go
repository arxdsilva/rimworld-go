// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Shots are the projectiles in flight (DEF-1). A gun's shot is decided at
// the muzzle the vanilla way: the aim holds or fails (a wild shot lands
// somewhere around the target, nearer for a better aim), and an aimed shot
// can still be caught by the target's cover. It then flies its line cell
// by cell: rock, walls and doors stop it, and a creature in its way may
// take it (friendly fire), except the shooter's allies close to the
// muzzle. On arrival an aimed shot strikes its target if it still stands
// there; any other shot strikes something in the cell half the time.
// Every roll is on the main stream; Step runs once a tick and the slice is
// reused, so a shot costs no allocation.
//
// An explosive (a grenade or a launcher's shell, DEF-2) never rolls to
// hit: it is aimed at its target's tile, scattered by its forced-miss
// radius at range, arcs over whatever lies between, and bursts where it
// lands (Blasts).
type Shots struct {
	h      Host
	st     *Strikes
	blasts *Blasts
	live   []projectile.Shot
	seen   []Seen // Near's reused buffer
	rules  Rules  // SHT-007: friendly fire (nil: 100 %, no colonists)
	// traps: a bullet landing on a buried charge lights it (DEF-4);
	// drafts: a colonist's thrown grenade comes off its belt.
	traps  *Traps
	drafts *Drafts
}

// Tracking is how far (tiles) an aimed shot's target may have moved from
// the shot's destination and still be struck on arrival. original tuning:
// the wiki publishes no rule; a bullet covers a tile a tick, so a target
// gets at most a step away.
const Tracking = 1

// fire launches attacker's shot with d at target (attacker at (ax, ay),
// target at (tx, ty)) with hit chance r, and records the Fired trace.
// building: a turret fired it (CBT-3; attacker is its building id), its
// muzzle rolls on roll (the defence stream; a pawn's on the main one).
func (s *Shots) fire(attacker, target entity.EntityID, ax, ay, tx, ty int, wk weapon.Kind, d weapon.Def, r attack.Report, building bool, roll func(int) int) {
	h := s.h
	sh := projectile.Shot{Owner: attacker, Target: target, Raider: s.raidersSide(attacker, building), Weapon: wk,
		Damage: d.Damage, Pen: weapon.Penetration(d, d.Damage), Type: d.DamageType, Fired: h.Now(),
		Hit: weapon.HitOf(wk, d), FromBuilding: building} // the firing weapon's hit (CBT-2), carried and saved
	dx, dy := tx, ty
	aim := r.Aim()
	switch {
	case d.Explosive(): // thrown or lobbed at the tile, scattered at range
		if s.drafts != nil {
			s.drafts.spend(attacker, wk)
		}
		if rad := explosion.ForcedMiss(d.ForcedMiss, r.Dist); rad > 0 {
			dx, dy = tx+roll(2*rad+1)-rad, ty+roll(2*rad+1)-rad
		}
		sh.Aimed = dx == tx && dy == ty
	case roll(1000) >= aim: // the aim failed: a wild shot
		dx, dy = s.missAt(ax, ay, tx, ty, attack.MissRadius(aim), roll)
	case r.Cover > 0 && roll(1000) < r.Cover: // the cover took it
		var cid entity.EntityID
		cid, dx, dy, _ = s.st.sight.coverBy(ax, ay, tx, ty)
		sh.Cover = cid
	default:
		sh.Aimed = true
	}
	sh.Launch(ax, ay, dx, dy, d.Speed)
	s.live = append(s.live, sh)
	h.Trace(projectile.Trace{FromX: ax, FromY: ay, ToX: dx, ToY: dy, Tick: sh.Fired, Hit: sh.Aimed,
		FromBuilding: building, Attacker: attacker, Target: target, Weapon: d.Name, Stage: projectile.Fired})
	if !building {
		s.st.learn(attacker, d, sh.Aimed || d.Explosive()) // CBT-2: an aim that held counts as a hit
		h.Struck(attacker, target, sh.Fired)
	}
}

// Turret fires a round of turret id's gun wk from (fx, fy) at target on
// (tx, ty) (CBT-3): a projectile like a pawn's shot. Its aim is the gun's
// band at the straight-line distance (a turret aims as a perfect
// shooter: weapon.HitPermille), the target's size, the weather and
// smoke; then the target's cover may take it. It flies, may stray or
// strike a creature in its way (by its distance from the muzzle: none
// within attack.InterceptNear), and its hit goes through armor. The muzzle rolls on the defence stream.
func (s *Shots) Turret(id entity.EntityID, wk weapon.Kind, fx, fy int, target entity.EntityID, tx, ty int) {
	d := weapon.DefOf(wk)
	r := s.turretReport(d, fx, fy, target, tx, ty)
	if d.Speed == 0 {
		d.Speed = 1000 // a tile a tick: every turret's round flies
	}
	s.fire(id, target, fx, fy, tx, ty, wk, d, r, true, s.h.DefenseRoll)
}

// turretReport is a turret's hit chance with gun d from (fx, fy) on target
// at (tx, ty) (Turret).
func (s *Shots) turretReport(d weapon.Def, fx, fy int, target entity.EntityID, tx, ty int) attack.Report {
	r := attack.Report{Dist: weapon.Range(fx, fy, tx, ty), InRange: true, InSight: true, Shooter: 1000, Size: 1000,
		Light: attack.NoLightPenalty, Weather: s.st.sight.Weather(fx, fy, tx, ty) * s.blasts.SmokeFactor(fx, fy, tx, ty) / 1000,
		Cover: attack.Permille(s.st.sight.Cover(fx, fy, tx, ty))}
	r.Band = attack.Permille(weapon.HitPermille(d, r.Dist))
	if !d.Legacy() {
		r.Size = attack.SizeFactor(s.st.bodySize(target))
	}
	if s.st.downedAt(target) {
		r.Downed = attack.DownedFactor(r.Dist) // SHT-008
	}
	r.Chance = r.Total()
	return r
}

// log records sh striking (or missing) v for the logs: a pawn's strike,
// or the turret that fired it named in v's log.
func (s *Shots) log(sh *projectile.Shot, v entity.EntityID, hit bool, dmg int) {
	if !sh.FromBuilding {
		s.h.LogStrike(sh.Owner, v, sh.Weapon, hit, dmg)
		return
	}
	name := TurretLogName
	if b, ok := s.st.turrets.At(sh.Owner); ok {
		name = turretName(b.Kind)
	}
	s.h.Shot(v, hit, dmg, name)
}

// missAt is where a wild shot at (tx, ty) from (ax, ay) lands: a cell
// within rad of the target, never the target's own (then one cell past
// it).
func (s *Shots) missAt(ax, ay, tx, ty, rad int, roll func(int) int) (int, int) {
	mx := tx + roll(2*rad+1) - rad
	my := ty + roll(2*rad+1) - rad
	if mx == tx && my == ty {
		mx, my = tx+sign(tx-ax), ty+sign(ty-ay)
	}
	return mx, my
}

// Step flies every shot one tick, in the order they were fired, dropping
// those that struck something or landed.
func (s *Shots) Step() {
	n, w := len(s.live), 0
	for i := 0; i < n; i++ {
		sh := s.live[i]
		if s.fly(&sh) {
			s.live[w] = sh
			w++
		}
	}
	w += copy(s.live[w:], s.live[n:]) // fired while stepping: keep, in order
	clear(s.live[w:])
	s.live = s.live[:w]
	s.blasts.prune(s.h.Now())
}

// fly moves sh one tick through the cells it enters; false once it is
// spent.
func (s *Shots) fly(sh *projectile.Shot) bool {
	for k := sh.Fly(); k > 0; k-- {
		x, y, last := sh.Step()
		if last {
			s.land(sh, x, y)
			return false
		}
		if sh.Blast() { // an explosive arcs over what lies between
			if !s.h.InBounds(x, y) {
				s.trace(sh, x, y, false, false, 0, 0)
				return false
			}
			continue
		}
		if s.through(sh, x, y) {
			return false
		}
	}
	if sh.Arrived() { // a zero-length shot
		s.land(sh, sh.X, sh.Y)
		return false
	}
	return true
}

// through is sh entering a cell on its way (x, y): it reports whether the
// shot stopped there, on rock, a wall or a door (which it damages), or in
// a creature it struck. A creature in the line, ally or not, is struck at
// its interception chance (attack.DownedIntercept of it lying downed,
// SHT-006) scaled by its distance from the muzzle (weapon.Range): never
// within attack.InterceptNear tiles, and fully from attack.InterceptFull
// (SHT-005); then by the friendly-fire share (ffPct, SHT-007). Each step
// truncates; no roll is drawn at a chance of 0.
func (s *Shots) through(sh *projectile.Shot, x, y int) bool {
	h := s.h
	if !h.InBounds(x, y) { // it left the map
		s.trace(sh, x, y, false, false, 0, 0)
		return true
	}
	if s.st.sight.blocks(x, y) {
		s.building(sh, x, y)
		return true
	}
	v, downed, ok := s.interceptor(x, y, sh.Owner)
	if !ok {
		return false
	}
	c := attack.InterceptChance(s.st.bodySize(v))
	if downed {
		c = c * attack.DownedIntercept / 1000
	}
	c = c * attack.InterceptScale(weapon.Range(sh.FromX, sh.FromY, x, y)) / 1000
	if c > 0 {
		c = c * attack.Permille(s.ffPct(sh, v)) / 100
	}
	if c == 0 || h.Roll(1000) >= c {
		return false
	}
	s.flesh(sh, v, x, y)
	return true
}

// land is sh arriving on (x, y): the cover it was caught by, its target if
// it was aimed and the target still stands there, else (attack.StrayHit
// of the time) a creature or building in the cell; otherwise a miss. Only
// a standing creature takes a stray, at the friendly-fire share (ffPct;
// at 0 nothing strays, not even into a building there); one lying downed
// leaves the cell's building to it (SHT-006).
func (s *Shots) land(sh *projectile.Shot, x, y int) {
	h := s.h
	if !h.InBounds(x, y) {
		s.miss(sh, x, y)
		return
	}
	if sh.Blast() { // it bursts where it lands
		s.trace(sh, x, y, true, false, 0, 0)
		b := weapon.DefOf(sh.Weapon).Blast
		if b.Kind == explosion.EMP {
			s.st.gear.EMP(x, y, b.Radius) // CBT-3: an EMP breaks the shield belts it reaches
		}
		s.blasts.Explode(sh.Owner, sh.Weapon, x, y, b)
		return
	}
	if s.traps != nil {
		s.traps.Struck(x, y) // wiki: IED trap, bullets and arrows set it off
	}
	if sh.Cover != 0 {
		if b, ok := h.Building(sh.Cover); ok && b.X == x && b.Y == y {
			s.building(sh, x, y)
			s.log(sh, sh.Target, false, 0)
			return
		}
	}
	if sh.Aimed { // a target that took a step while the shot flew still takes it (one lying downed too: CBT-2)
		if tx, ty, ok := s.st.target(sh.Target, true); ok && weapon.Distance(tx, ty, x, y) <= Tracking {
			s.flesh(sh, sh.Target, tx, ty)
			return
		}
	}
	if !s.stray(sh, x, y) {
		s.miss(sh, x, y)
	}
}

// stray is sh, off its target, striking what is in its landing cell (x, y)
// attack.StrayHit of the time: a standing creature (at the friendly-fire
// share, ffPct; at 0 nothing is rolled and nothing strays) or, when none
// stands there (one lying downed included), the cell's building. It
// reports whether it struck.
func (s *Shots) stray(sh *projectile.Shot, x, y int) bool {
	h := s.h
	if v, downed, ok := s.interceptor(x, y, sh.Owner); ok && !downed {
		if c := attack.StrayHit * attack.Permille(s.ffPct(sh, v)) / 100; c > 0 && h.Roll(1000) < c {
			s.flesh(sh, v, x, y)
			return true
		}
		return false
	}
	if id, _ := h.Object(x, y); id != 0 && h.Roll(1000) < attack.StrayHit {
		s.building(sh, x, y)
		return true
	}
	return false
}

// flesh is sh striking creature v on (x, y) with the hit it was fired
// with: armor, then the wound.
func (s *Shots) flesh(sh *projectile.Shot, v entity.EntityID, x, y int) {
	h := s.h
	l := h.Absorb(v, shotHit(sh), h.Roll)
	dmg := max(l.Amount, 0)
	s.trace(sh, x, y, true, false, dmg, v)
	s.log(sh, v, true, dmg)
	by := sh.Owner
	if sh.FromBuilding {
		by = 0 // no reaction ever points at a building
	}
	s.st.wound(by, v, sh.FromX, sh.FromY, x, y, l)
}

// shotHit is sh's hit: the one it was fired with, or (a shot saved before
// CBT-2) the weapon's hit at the shot's own damage, penetration and type.
func shotHit(sh *projectile.Shot) damage.Hit {
	if sh.Hit.Amount > 0 || sh.Damage <= 0 {
		return sh.Hit
	}
	d := weapon.DefOf(sh.Weapon)
	d.Damage, d.DamageType = sh.Damage, sh.Type
	hit := weapon.HitOf(sh.Weapon, d)
	hit.Pen = sh.Pen
	return hit
}

// building is sh striking what blocks (x, y): a building takes the
// damage, rock only the mark.
func (s *Shots) building(sh *projectile.Shot, x, y int) {
	id, _ := s.h.Object(x, y)
	dmg := 0
	if id != 0 {
		dmg = sh.Damage
	}
	s.trace(sh, x, y, true, true, dmg, 0)
	if id != 0 {
		s.h.DamageBuilding(id, dmg)
	}
}

// miss is sh landing on (x, y) without striking anything.
func (s *Shots) miss(sh *projectile.Shot, x, y int) {
	s.trace(sh, x, y, false, false, 0, 0)
	if sh.Target != 0 {
		s.log(sh, sh.Target, false, 0)
	}
}

// trace records sh's Landed trace on (x, y).
func (s *Shots) trace(sh *projectile.Shot, x, y int, hit, building bool, dmg int, victim entity.EntityID) {
	s.h.Trace(projectile.Trace{FromX: sh.FromX, FromY: sh.FromY, ToX: x, ToY: y, Tick: s.h.Now(),
		Hit: hit, Building: building, Damage: dmg, Attacker: sh.Owner, Target: victim,
		Weapon: weapon.DefOf(sh.Weapon).Name, Stage: projectile.Landed, FromBuilding: sh.FromBuilding})
}

// interceptor is the creature a shot can strike on (x, y), never owner
// nor a turret: a standing one (a living animal on its feet or a standing
// humanlike) before one lying downed (SHT-006: downed reports which), the
// lowest id of either when several share the cell (Near's order is not
// fixed: lowest).
func (s *Shots) interceptor(x, y int, owner entity.EntityID) (v entity.EntityID, downed, ok bool) {
	s.seen = s.h.Near(x, y, 0, Everyone, s.seen[:0])
	var lo lowest
	for _, c := range s.seen {
		if c.ID != owner {
			lo.add(c.ID, s.reachOf(c.ID))
		}
	}
	return lo.pick()
}

// reach is how a shot in flight can strike a creature: not at all (a
// turret, the dead, nothing attackable), standing, or lying downed.
type reach uint8

const (
	reachNone reach = iota
	reachStanding
	reachDowned
)

// reachOf is how a shot in flight can strike id (interceptor).
func (s *Shots) reachOf(id entity.EntityID) reach {
	if _, turret := s.st.turrets.At(id); turret {
		return reachNone
	}
	if s.st.downedAt(id) {
		return reachDowned
	}
	if _, _, ok := s.st.Target(id); ok {
		return reachStanding
	}
	return reachNone
}

// lowest is a cell's choice of interceptor: the lowest standing and the
// lowest downed id seen (0: none), whatever order they are added in.
type lowest struct{ up, down entity.EntityID }

// add counts id, reached as r.
func (l *lowest) add(id entity.EntityID, r reach) {
	switch r {
	case reachStanding:
		l.up = lowerID(l.up, id)
	case reachDowned:
		l.down = lowerID(l.down, id)
	}
}

// pick is the standing one if any, else the downed one (downed true).
func (l lowest) pick() (v entity.EntityID, downed, ok bool) {
	if l.up != 0 {
		return l.up, false, true
	}
	return l.down, l.down != 0, l.down != 0
}

// lowerID is the lower of cur (0: none yet) and id.
func lowerID(cur, id entity.EntityID) entity.EntityID {
	if cur == 0 || id < cur {
		return id
	}
	return cur
}

// ffPct is the percent of a strike chance sh keeps on creature v (SHT-007):
// the difficulty's FriendlyFirePct, clamped to 0..100, for an aimed shot a
// colonist fired crossing a colonist; 100 for anything else (a wild shot,
// a turret's round, a raider's) or without rules.
func (s *Shots) ffPct(sh *projectile.Shot, v entity.EntityID) int {
	r := s.rules
	if r == nil || !sh.Aimed || sh.FromBuilding || !r.Colonist(sh.Owner) || !r.Colonist(v) {
		return 100
	}
	return min(max(r.FriendlyFirePct(), 0), 100)
}

// raidersSide reports the shooter on the raiders' side: a raider on its
// feet, or a settlement's turret (MAP-13: its defenders are the shooter's
// allies, colonists in the line are not).
func (s *Shots) raidersSide(attacker entity.EntityID, building bool) bool {
	if building {
		b, ok := s.h.Building(attacker)
		return ok && b.Hostile
	}
	return s.h.StandingRaider(attacker)
}

// Views is every shot in flight for presentation, oldest first (a fresh
// slice: the world view keeps it).
func (s *Shots) Views() []projectile.View {
	if len(s.live) == 0 {
		return nil
	}
	out := make([]projectile.View, len(s.live))
	for i := range s.live {
		out[i] = s.live[i].View()
	}
	return out
}

// Len is how many shots are in flight.
func (s *Shots) Len() int { return len(s.live) }

// Export is the shots in flight for the save (nil when none).
func (s *Shots) Export() []projectile.Shot {
	if len(s.live) == 0 {
		return nil
	}
	return append([]projectile.Shot(nil), s.live...)
}

// Import restores saved shots.
func (s *Shots) Import(saved []projectile.Shot) { s.live = append(s.live[:0], saved...) }

// Lob launches a mortar shell (DEF-3) from (ax, ay), aimed at
// (aimX, aimY) and landing on (lx, ly) (its forced miss): it arcs there and
// bursts as its weapon's blast. The Fired trace points at the aim (the
// launch sound plays at the muzzle, the whistle at the aim).
func (s *Shots) Lob(owner entity.EntityID, wk weapon.Kind, ax, ay, aimX, aimY, lx, ly int) {
	h := s.h
	d := weapon.DefOf(wk)
	sh := projectile.Shot{Owner: owner, Weapon: wk, Damage: d.Damage, Pen: weapon.Penetration(d, d.Damage), Type: d.DamageType,
		Fired: h.Now(), Aimed: lx == aimX && ly == aimY, Hit: weapon.HitOf(wk, d)}
	sh.Launch(ax, ay, lx, ly, d.Speed)
	s.live = append(s.live, sh)
	h.Trace(projectile.Trace{FromX: ax, FromY: ay, ToX: aimX, ToY: aimY, Tick: sh.Fired, Hit: sh.Aimed,
		Attacker: owner, Weapon: d.Name, Stage: projectile.Fired})
}
