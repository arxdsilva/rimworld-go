package game

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Sentry turrets (PLAY-024). A powered turret looks for a target on a
// staggered cadence through the creature index, locks on, aims, and fires
// its weapon.SentryGun on streamDefense; it is shed last in a brownout
// (power.ClassDefense) and bursts when destroyed (turretBlast, from
// damageBuilding). Tuning: .cursor/PLAY024_PLAN.md.
//
// The lock has hysteresis so a turret never flaps between holding and
// idling (nor its power draw between 120 and 40 W): it locks on within
// turretLock tiles and lets go only past turretRelease tiles, or once the
// target has been out of its sight for turretLost ticks.

const (
	// turretScanEvery: an idle turret looks for a target once in this
	// many ticks, on the tick (now+id)%turretScanEvery == 0.
	turretScanEvery = 20
	// turretLock is the reach a target is picked up in (the gun's range);
	// turretRelease the distance past which a held target is let go.
	turretLock    = 10
	turretRelease = 12
	// turretLost: a held target out of sight this long is let go.
	turretLost = 20
	// turretAimTicks is the aim before the first shot at a new target.
	turretAimTicks = 30
	// The blast of a destroyed turret: creature and building damage, its
	// reach (Chebyshev) and the percent chance it sets the cell alight.
	turretBlastCreature = 12
	turretBlastBuilding = 25
	turretBlastRadius   = 2
	turretIgnitePct     = 50
)

// turretState is the turrets' live aims per building (saved through
// turretSave) and the derived turret list.
type turretState struct {
	aims map[entity.EntityID]turretAim
	// ids are the turrets, ascending, per power list generation (derived,
	// never saved: Load resets the state).
	ids   []entity.EntityID
	idsOK bool
	idsAt uint64
}

// turretAim is one turret's aim: the creature it holds (0: idle), its
// baked aim frame (0..15, like BuildingView.AimDir), the tick of its last
// shot (0: never), the first tick it may fire again (the aim, then the
// gun's cooldown) and the last tick it saw its target.
type turretAim struct {
	target  entity.EntityID
	dir     int8
	shotAt  uint64
	readyAt uint64
	seenAt  uint64
}

// turretSave is the saved turret state, nil while no turret ever held an
// aim (absent in older saves, so they hash as before).
type turretSave struct {
	Version int
	Aims    []turretAimSave // ascending by turret
}

// turretAimSave is one turretAim on disk.
type turretAimSave struct {
	ID, Target              entity.EntityID
	Dir                     int8
	ShotAt, ReadyAt, SeenAt uint64
}

// turretSaveVersion versions the Turrets block.
const turretSaveVersion = 1

// turretIDs lists the sentry turrets, ascending, from the power list
// cache (power.go): no scan of the buildings unless the structure changed.
// Aims of turrets that are gone are dropped whenever the list moves.
func (g *Game) turretIDs() []entity.EntityID {
	c := g.powerList()
	ts := &g.turrets
	if ts.idsOK && ts.idsAt == c.gen {
		return ts.ids
	}
	ts.ids = ts.ids[:0]
	for i, s := range c.specs {
		if s.Class == power.ClassDefense && s.Role == power.Consumer {
			ts.ids = append(ts.ids, c.ids[i])
		}
	}
	ts.idsOK, ts.idsAt = true, c.gen
	for id := range ts.aims {
		if _, ok := slices.BinarySearch(ts.ids, id); !ok {
			delete(ts.aims, id)
		}
	}
	return ts.ids
}

// phaseTurrets runs every turret's search, aim and shot (phaseThreats,
// after the raiders think).
func (g *Game) phaseTurrets(now uint64) {
	if g.pw == nil { // no power device on the map: no turret can be on
		if len(g.turrets.aims) > 0 {
			g.turretIDs() // drops the aims of turrets that are gone
		}
		return
	}
	for _, id := range g.turretIDs() {
		b := g.bldgs.Ref(id)
		if b == nil {
			continue
		}
		g.stepTurret(id, Position{X: b.X, Y: b.Y}, now)
	}
}

// stepTurret is one turret's tick at pos.
func (g *Game) stepTurret(id entity.EntityID, pos Position, now uint64) {
	a, had := g.turrets.aims[id]
	if !g.powered(id) {
		if had && a.target != 0 { // no power: it lets go and stays dark
			a.target, a.readyAt = 0, 0
			g.setAim(id, a)
		}
		return
	}
	var tp Position
	if a.target != 0 {
		var keep bool
		if tp, keep = g.turretHolds(pos, &a, now); !keep {
			a.target, a.readyAt = 0, 0
			g.setAim(id, a)
		}
	}
	if a.target == 0 {
		if (now+uint64(id))%turretScanEvery != 0 {
			return
		}
		t, p, ok := g.nearestWhere(pos.X, pos.Y, turretLock, ckRaider|ckAnimal, func(t entity.EntityID, p Position) bool {
			return g.turretTargetable(t, now) && g.lineOfSight(pos.X, pos.Y, p.X, p.Y)
		})
		if !ok {
			return
		}
		a.target, a.readyAt, a.seenAt, tp = t, now+turretAimTicks, now, p
	}
	a.dir = aimFrame(pos, tp, a.dir)
	d := weapon.DefOf(weapon.SentryGun)
	if a.seenAt == now && now >= a.readyAt && weapon.InRange(d, weapon.Distance(pos.X, pos.Y, tp.X, tp.Y)) {
		g.turretShot(id, pos, a.target, tp)
		a.shotAt, a.readyAt = now, now+uint64(d.Cooldown)
	}
	g.setAim(id, a)
}

// setAim stores turret id's aim.
func (g *Game) setAim(id entity.EntityID, a turretAim) {
	if g.turrets.aims == nil {
		g.turrets.aims = map[entity.EntityID]turretAim{}
	}
	g.turrets.aims[id] = a
}

// turretHolds reports whether the turret at pos keeps its target, with the
// target's position; it notes the tick it saw it. A target that can no
// longer be shot at (down, dead, gone, calmed), is past turretRelease, or
// has been out of sight for turretLost ticks is let go.
func (g *Game) turretHolds(pos Position, a *turretAim, now uint64) (Position, bool) {
	tp, ok := g.positions.Get(a.target)
	if !ok || !g.turretTargetable(a.target, now) || weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) > turretRelease {
		return tp, false
	}
	if g.lineOfSight(pos.X, pos.Y, tp.X, tp.Y) {
		a.seenAt = now
	}
	return tp, now-a.seenAt < turretLost
}

// turretTargetable: a standing raider (never a prisoner, visitor or
// trader: standingRaider) or a hostile animal — one attacking a colonist,
// or a manhunter. Colonists and tame animals never are.
func (g *Game) turretTargetable(id entity.EntityID, now uint64) bool {
	if a, ok := g.animals.Get(id); ok {
		return !a.Dead() && !a.Tame && (g.hostile(a) || a.Enraged(now))
	}
	return g.standingRaider(id)
}

// turretShot fires one round from the turret at from on target: the hit
// chance is strikeRoll's for a ranged shot (weapon.HitChance less the
// target's cover), rolled on streamDefense. A hit is hurtAs with no
// attacker, so no animal or social reaction ever points at a building;
// the turret is named in the target's log instead.
func (g *Game) turretShot(id entity.EntityID, from Position, target entity.EntityID, tp Position) {
	d := weapon.DefOf(weapon.SentryGun)
	dist := weapon.Distance(from.X, from.Y, tp.X, tp.Y)
	chance := weapon.HitChance(d, dist) * (100 - g.coverFor(from, tp)) / 100
	hit := g.stream(streamDefense).IntN(100) < chance
	now := uint64(g.clock.Now())
	tr := projectile.Trace{
		FromX: from.X, FromY: from.Y, ToX: tp.X, ToY: tp.Y, Tick: now, Hit: hit,
		FromBuilding: true, Attacker: id, Target: target, Weapon: d.Name,
	}
	if hit {
		tr.Damage = d.Damage
	}
	g.traces = append(g.traces, tr)
	if !hit {
		g.logEvent(target, logEntry{Kind: logWasMissed, Name: turretLogName})
		return
	}
	g.logEvent(target, logEntry{Kind: logAttacked, Amount: d.Damage, Name: turretLogName})
	g.lastHit[target] = hitRec{Tick: now, FromX: from.X, FromY: from.Y, Damage: d.Damage}
	g.stainHit(target, from, tp, d.Damage) // stains.go: before hurt, while the target exists
	g.hurtAs(target, d.Damage, 0, body.Cut, health.CauseInjury)
}

// turretLogName is how a creature's log names the turret that shot it.
const turretLogName = "a sentry turret"

// turretBlast: turret b (already removed) blows up. Creatures within
// turretBlastRadius take turretBlastCreature, object-layer buildings
// turretBlastBuilding (a turret among them may blow up in turn); the
// renderer gets a burst and the cell may catch fire (streamDefense).
func (g *Game) turretBlast(id entity.EntityID, b construction.Building) {
	delete(g.turrets.aims, id)
	now := uint64(g.clock.Now())
	g.wl.Bursts = append(g.wl.Bursts, burstStamp{X: b.X, Y: b.Y, Tick: now, Turret: true})
	g.message(b.X, b.Y, "The sentry turret blew up!")
	if g.stream(streamDefense).IntN(100) < turretIgnitePct {
		g.igniteBurst(b.X, b.Y) // fire.go
	}
	var hit []entity.EntityID
	g.within(b.X, b.Y, turretBlastRadius, ckAll, func(cid entity.EntityID, _ Position) { hit = append(hit, cid) })
	for _, cid := range hit {
		if _, ok := g.positions.Get(cid); ok { // a chained blast may have taken it
			g.hurtAs(cid, turretBlastCreature, 0, body.Burn, health.CauseInjury)
		}
	}
	var near []entity.EntityID
	for y := b.Y - turretBlastRadius; y <= b.Y+turretBlastRadius; y++ {
		for x := b.X - turretBlastRadius; x <= b.X+turretBlastRadius; x++ {
			if !g.grid.InBounds(x, y) {
				continue
			}
			if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 && !slices.Contains(near, bid) {
				near = append(near, bid)
			}
		}
	}
	for _, bid := range near {
		g.damageBuilding(bid, turretBlastBuilding)
	}
}

// raiderTurret is the powered turret a raider at pos turns on instead of
// the nearest colonist cd tiles off: the nearest one within sightRange,
// and only when it is at least switchMargin closer than the colonist (so
// a raider between the two never swaps back and forth). ok false: none.
func (g *Game) raiderTurret(pos Position, cd int) (entity.EntityID, Position, bool) {
	var best entity.EntityID
	var bp Position
	if g.pw == nil {
		return best, bp, false // no power device: no powered turret
	}
	bestD := min(sightRange, cd-switchMargin) + 1
	for _, id := range g.turretIDs() {
		b := g.bldgs.Ref(id)
		if b == nil || !g.powered(id) {
			continue
		}
		if d := weapon.Distance(pos.X, pos.Y, b.X, b.Y); d < bestD {
			best, bp, bestD = id, Position{X: b.X, Y: b.Y}, d
		}
	}
	return best, bp, best != 0
}

// turretAt is the sentry turret with building id (ok false: none).
func (g *Game) turretAt(id entity.EntityID) (construction.Building, bool) {
	b := g.bldgs.Ref(id)
	if b == nil || b.Kind != construction.SentryTurret {
		return construction.Building{}, false
	}
	return *b, true
}

// aimFrame is the baked gun frame (0..15) pointing from p at q: frame 0
// faces south and the frames count clockwise on screen (4 west, 8 north,
// 12 east), each 22.5° wide. Integer only, so it is the same everywhere;
// q on p keeps prev.
func aimFrame(p, q Position, prev int8) int8 {
	u, v := q.Y-p.Y, p.X-q.X // u towards south, v towards west
	if u == 0 && v == 0 {
		return prev
	}
	au, av := abs(u), abs(v)
	// s: the 22.5° sector within the quadrant, by the angle off the u axis
	// against tan 11.25°, 33.75°, 56.25° and 78.75° (in thousandths)
	s := 0
	for _, t := range [...]int{199, 668, 1497, 5027} {
		if av*1000 > t*au {
			s++
		}
	}
	var f int
	switch {
	case v >= 0 && u >= 0:
		f = s
	case v >= 0:
		f = 8 - s
	case u < 0:
		f = 8 + s
	default:
		f = 16 - s
	}
	return int8(f % 16)
}

// turretFiring reports turret id holding a target now: it draws its
// firing power instead of its idle power (power.go powerWant).
func (g *Game) turretFiring(id entity.EntityID) bool {
	return g.turrets.aims[id].target != 0
}

// fillTurretView fills a turret's aim frame and the ticks since its last
// shot (-1: none yet) for the renderer.
func (g *Game) fillTurretView(bv *BuildingView, id entity.EntityID, b construction.Building, now uint64) {
	if b.Kind != construction.SentryTurret {
		return
	}
	a := g.turrets.aims[id]
	bv.AimDir, bv.ShotAge = a.dir, -1
	if a.shotAt != 0 && now >= a.shotAt {
		bv.ShotAge = int(now - a.shotAt)
	}
}

// exportTurrets is the Turrets save field (nil: no turret ever aimed).
func (g *Game) exportTurrets() *turretSave {
	if len(g.turrets.aims) == 0 {
		return nil
	}
	s := &turretSave{Version: turretSaveVersion, Aims: make([]turretAimSave, 0, len(g.turrets.aims))}
	for id, a := range g.turrets.aims {
		s.Aims = append(s.Aims, turretAimSave{ID: id, Target: a.target, Dir: a.dir, ShotAt: a.shotAt, ReadyAt: a.readyAt, SeenAt: a.seenAt})
	}
	slices.SortFunc(s.Aims, func(x, y turretAimSave) int { return cmp.Compare(x.ID, y.ID) })
	return s
}

// importTurrets restores exportTurrets' state (nil: none).
func (g *Game) importTurrets(s *turretSave) {
	g.turrets = turretState{}
	if s == nil || len(s.Aims) == 0 {
		return
	}
	g.turrets.aims = make(map[entity.EntityID]turretAim, len(s.Aims))
	for _, a := range s.Aims {
		g.turrets.aims[a.ID] = turretAim{target: a.Target, dir: a.Dir, shotAt: a.ShotAt, readyAt: a.ReadyAt, seenAt: a.SeenAt}
	}
}
