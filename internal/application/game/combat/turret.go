package combat

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Sentry turrets (PLAY-024). A powered turret looks for a target on a
// staggered cadence through the creature index, locks on, aims, and fires
// its weapon.SentryGun on the defence stream; it is shed last in a
// brownout (power.ClassDefense) and bursts when destroyed (Blast, from the
// game's building damage). Tuning: .cursor/PLAY024_PLAN.md.
//
// The lock has hysteresis so a turret never flaps between holding and
// idling (nor its power draw between 120 and 40 W): it locks on within
// TurretLock tiles and lets go only past TurretRelease tiles, or once the
// target has been out of its sight for TurretLost ticks.

const (
	// TurretScanEvery: an idle turret looks for a target once in this
	// many ticks, on the tick (now+id)%TurretScanEvery == 0.
	TurretScanEvery = 20
	// TurretLock is the reach a target is picked up in (the gun's range);
	// TurretRelease the distance past which a held target is let go.
	TurretLock    = 10
	TurretRelease = 12
	// TurretLost: a held target out of sight this long is let go.
	TurretLost = 20
	// turretAimTicks is the aim before the first shot at a new target.
	turretAimTicks = 30
	// The blast of a destroyed turret: creature and building damage, its
	// reach (Chebyshev) and the percent chance it sets the cell alight.
	TurretBlastCreature = 12
	TurretBlastBuilding = 25
	turretBlastRadius   = 2
	turretIgnitePct     = 50
	// ShotAgeMax caps a turret view's shot age: past it the renderer's
	// recoil, flash and "engaged" read (turret_draw.go, 90 ticks) are long
	// over, and a capped age lets a quiet turret's view stop changing every
	// tick (the read model keeps its identity, PERF-002).
	ShotAgeMax = 120
	// TurretLogName is how a creature's log names the turret that shot it.
	TurretLogName = "a sentry turret"
	// TurretSaveVersion versions the Turrets save block.
	TurretSaveVersion = 1
)

// Aim is one turret's aim: the creature it holds (0: idle), its baked aim
// frame (0..15, like the game's BuildingView.AimDir), the tick of its last
// shot (0: never), the first tick it may fire again (the aim, then the
// gun's cooldown) and the last tick it saw its target.
type Aim struct {
	Target  entity.EntityID
	Dir     int8
	ShotAt  uint64
	ReadyAt uint64
	SeenAt  uint64
}

// TurretSave is the saved turret state, nil while no turret ever held an
// aim (absent in older saves, so they hash as before). Gob field names are
// those of the game's former turretSave.
type TurretSave struct {
	Version int
	Aims    []AimSave // ascending by turret
}

// AimSave is one Aim on disk.
type AimSave struct {
	ID, Target              entity.EntityID
	Dir                     int8
	ShotAt, ReadyAt, SeenAt uint64
}

// Turrets are the sentry turrets' live aims per building and the derived
// turret list.
type Turrets struct {
	h     Host
	pw    Power
	sight *Sight
	aims  map[entity.EntityID]Aim
	// ids are the turrets, ascending, per power list generation (derived,
	// never saved: Import resets it).
	ids   []entity.EntityID
	idsOK bool
	idsAt uint64
	near  []Seen // target candidates, reused
}

func newTurrets(h Host, pw Power, s *Sight) *Turrets {
	return &Turrets{h: h, pw: pw, sight: s}
}

// list is the sentry turrets, ascending, from the host's power list
// cache: no scan of the buildings unless the structure changed. Aims of
// turrets that are gone are dropped whenever the list moves.
func (t *Turrets) list() []entity.EntityID {
	gen := t.pw.PowerGen()
	if t.idsOK && t.idsAt == gen {
		return t.ids
	}
	t.ids = t.pw.AppendDefense(t.ids[:0])
	t.idsOK, t.idsAt = true, gen
	for id := range t.aims {
		if _, ok := slices.BinarySearch(t.ids, id); !ok {
			delete(t.aims, id)
		}
	}
	return t.ids
}

// Phase runs every turret's search, aim and shot (the game's threat
// phase, after the raiders think).
func (t *Turrets) Phase(now uint64) {
	if !t.pw.Wired() { // no power device on the map: no turret can be on
		if len(t.aims) > 0 {
			t.list() // drops the aims of turrets that are gone
		}
		return
	}
	for _, id := range t.list() {
		b, ok := t.h.Building(id)
		if !ok {
			continue
		}
		t.step(id, b.X, b.Y, now)
	}
}

// step is one turret's tick at (x, y).
func (t *Turrets) step(id entity.EntityID, x, y int, now uint64) {
	a, had := t.aims[id]
	if !t.pw.Powered(id) {
		if had && a.Target != 0 { // no power: it lets go and stays dark
			a.Target, a.ReadyAt = 0, 0
			t.setAim(id, a)
		}
		return
	}
	var tx, ty int
	if a.Target != 0 {
		var keep bool
		if tx, ty, keep = t.holds(x, y, &a, now); !keep {
			a.Target, a.ReadyAt = 0, 0
			t.setAim(id, a)
		}
	}
	if a.Target == 0 {
		if (now+uint64(id))%TurretScanEvery != 0 {
			return
		}
		t.near = t.h.Near(x, y, TurretLock, Raiders|Animals, t.near[:0])
		s, ok := nearest(t.near, x, y, func(s Seen) bool {
			return t.Targetable(s.ID, now) && t.sight.Line(x, y, s.X, s.Y)
		})
		if !ok {
			return
		}
		a.Target, a.ReadyAt, a.SeenAt, tx, ty = s.ID, now+turretAimTicks, now, s.X, s.Y
	}
	a.Dir = aimFrame(x, y, tx, ty, a.Dir)
	d := weapon.DefOf(weapon.SentryGun)
	if a.SeenAt == now && now >= a.ReadyAt && weapon.InRange(d, weapon.Distance(x, y, tx, ty)) {
		t.shot(id, x, y, a.Target, tx, ty)
		a.ShotAt, a.ReadyAt = now, now+uint64(d.Cooldown)
	}
	t.setAim(id, a)
}

// nearest is the creature in seen nearest to (x, y) that keep accepts,
// ties to the lower id: the game's creature-index nearest search
// (nearestWhere), walking seen in the index's order and checking keep only
// on a candidate that would beat the best so far.
func nearest(seen []Seen, x, y int, keep func(Seen) bool) (Seen, bool) {
	var best Seen
	bestD := -1
	for _, s := range seen {
		d := weapon.Distance(x, y, s.X, s.Y)
		if bestD >= 0 && (d > bestD || d == bestD && s.ID > best.ID) {
			continue
		}
		if keep(s) {
			best, bestD = s, d
		}
	}
	return best, bestD >= 0
}

// setAim stores turret id's aim.
func (t *Turrets) setAim(id entity.EntityID, a Aim) {
	if t.aims == nil {
		t.aims = map[entity.EntityID]Aim{}
	}
	t.aims[id] = a
}

// holds reports whether the turret at (x, y) keeps its target, with the
// target's tile; it notes the tick it saw it. A target that can no longer
// be shot at (down, dead, gone, calmed), is past TurretRelease, or has
// been out of sight for TurretLost ticks is let go.
func (t *Turrets) holds(x, y int, a *Aim, now uint64) (int, int, bool) {
	tx, ty, ok := t.h.Pos(a.Target)
	if !ok || !t.Targetable(a.Target, now) || weapon.Distance(x, y, tx, ty) > TurretRelease {
		return tx, ty, false
	}
	if t.sight.Line(x, y, tx, ty) {
		a.SeenAt = now
	}
	return tx, ty, now-a.SeenAt < TurretLost
}

// Targetable: a standing raider (never a prisoner, visitor or trader) or a
// hostile animal — one attacking a colonist, or a manhunter. Colonists
// and tame animals never are.
func (t *Turrets) Targetable(id entity.EntityID, now uint64) bool {
	if a, ok := t.h.Animal(id); ok {
		return !a.Dead() && !a.Tame && (t.h.Hostile(a) || a.Enraged(now))
	}
	return t.h.StandingRaider(id)
}

// shot fires one round from the turret at (fx, fy) on target at (tx, ty):
// the hit chance is a ranged strike's (weapon.HitChance less the target's
// cover), rolled on the defence stream. A hit is HurtAs with no attacker,
// so no animal or social reaction ever points at a building; the turret is
// named in the target's log instead.
func (t *Turrets) shot(id entity.EntityID, fx, fy int, target entity.EntityID, tx, ty int) {
	d := weapon.DefOf(weapon.SentryGun)
	dist := weapon.Distance(fx, fy, tx, ty)
	chance := weapon.HitChance(d, dist) * (100 - t.sight.Cover(fx, fy, tx, ty)) / 100
	h := t.h
	hit := h.DefenseRoll(100) < chance
	now := h.Now()
	tr := projectile.Trace{
		FromX: fx, FromY: fy, ToX: tx, ToY: ty, Tick: now, Hit: hit,
		FromBuilding: true, Attacker: id, Target: target, Weapon: d.Name,
	}
	if hit {
		tr.Damage = d.Damage
	}
	h.Trace(tr)
	if !hit {
		h.Shot(target, false, 0, TurretLogName)
		return
	}
	h.Shot(target, true, d.Damage, TurretLogName)
	h.Hit(target, now, fx, fy, d.Damage)
	h.Stain(target, fx, fy, tx, ty, d.Damage) // before the hurt, while the target exists
	h.HurtAs(target, d.Damage, 0, body.Cut, health.CauseInjury)
}

// Blast: turret b (already removed) blows up. Creatures within
// turretBlastRadius take TurretBlastCreature, object-layer buildings
// TurretBlastBuilding (a turret among them may blow up in turn); the
// renderer gets a burst and the cell may catch fire (defence stream).
func (t *Turrets) Blast(id entity.EntityID, b construction.Building) {
	delete(t.aims, id)
	h := t.h
	h.Burst(b.X, b.Y, h.Now())
	h.Message(b.X, b.Y, "The sentry turret blew up!")
	if h.DefenseRoll(100) < turretIgnitePct {
		h.Ignite(b.X, b.Y)
	}
	// a fresh list: a chained blast runs inside this one
	hit := h.Near(b.X, b.Y, turretBlastRadius, Everyone, nil)
	slices.SortFunc(hit, func(p, q Seen) int { return cmp.Compare(p.ID, q.ID) })
	for _, s := range hit {
		if _, _, ok := h.Pos(s.ID); ok { // a chained blast may have taken it
			h.HurtAs(s.ID, TurretBlastCreature, 0, body.Burn, health.CauseInjury)
		}
	}
	var near []entity.EntityID
	for y := b.Y - turretBlastRadius; y <= b.Y+turretBlastRadius; y++ {
		for x := b.X - turretBlastRadius; x <= b.X+turretBlastRadius; x++ {
			if !h.InBounds(x, y) {
				continue
			}
			if bid, _ := h.Object(x, y); bid != 0 && !slices.Contains(near, bid) {
				near = append(near, bid)
			}
		}
	}
	for _, bid := range near {
		h.DamageBuilding(bid, TurretBlastBuilding)
	}
}

// ForRaider is the powered turret a raider at (x, y) turns on instead of
// the nearest colonist cd tiles off: the nearest one within sight tiles,
// and only when it is at least SwitchMargin closer than the colonist (so a
// raider between the two never swaps back and forth). ok false: none.
func (t *Turrets) ForRaider(x, y, cd, sight int) (id entity.EntityID, bx, by int, ok bool) {
	if !t.pw.Wired() {
		return 0, 0, 0, false // no power device: no powered turret
	}
	bestD := min(sight, cd-SwitchMargin) + 1
	for _, tid := range t.list() {
		b, ok := t.h.Building(tid)
		if !ok || !t.pw.Powered(tid) {
			continue
		}
		if d := weapon.Distance(x, y, b.X, b.Y); d < bestD {
			id, bx, by, bestD = tid, b.X, b.Y, d
		}
	}
	return id, bx, by, id != 0
}

// At is the sentry turret with building id (ok false: none).
func (t *Turrets) At(id entity.EntityID) (construction.Building, bool) {
	b, ok := t.h.Building(id)
	if !ok || b.Kind != construction.SentryTurret {
		return construction.Building{}, false
	}
	return b, true
}

// aimFrame is the baked gun frame (0..15) pointing from (px, py) at
// (qx, qy): frame 0 faces south and the frames count clockwise on screen
// (4 west, 8 north, 12 east), each 22.5° wide. Integer only, so it is the
// same everywhere; q on p keeps prev.
func aimFrame(px, py, qx, qy int, prev int8) int8 {
	u, v := qy-py, px-qx // u towards south, v towards west
	if u == 0 && v == 0 {
		return prev
	}
	au, av := abs(u), abs(v)
	// s: the 22.5° sector within the quadrant, by the angle off the u axis
	// against tan 11.25°, 33.75°, 56.25° and 78.75° (in thousandths)
	s := 0
	for _, th := range [...]int{199, 668, 1497, 5027} {
		if av*1000 > th*au {
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

// Firing reports turret id holding a target now: it draws its firing
// power instead of its idle power.
func (t *Turrets) Firing(id entity.EntityID) bool { return t.aims[id].Target != 0 }

// Aim is turret id's aim (zero: none).
func (t *Turrets) Aim(id entity.EntityID) Aim { return t.aims[id] }

// View is a turret's aim frame and the ticks since its last shot (-1: none
// yet; at most ShotAgeMax) for the renderer.
func (t *Turrets) View(id entity.EntityID, now uint64) (dir int8, shotAge int) {
	a := t.aims[id]
	dir, shotAge = a.Dir, -1
	if a.ShotAt != 0 && now >= a.ShotAt {
		shotAge = int(min(now-a.ShotAt, ShotAgeMax))
	}
	return dir, shotAge
}

// Export is the Turrets save field (nil: no turret ever aimed).
func (t *Turrets) Export() *TurretSave {
	if len(t.aims) == 0 {
		return nil
	}
	s := &TurretSave{Version: TurretSaveVersion, Aims: make([]AimSave, 0, len(t.aims))}
	for id, a := range t.aims {
		s.Aims = append(s.Aims, AimSave{ID: id, Target: a.Target, Dir: a.Dir, ShotAt: a.ShotAt, ReadyAt: a.ReadyAt, SeenAt: a.SeenAt})
	}
	slices.SortFunc(s.Aims, func(x, y AimSave) int { return cmp.Compare(x.ID, y.ID) })
	return s
}

// Import restores Export's state (nil: none) and drops the derived list.
func (t *Turrets) Import(s *TurretSave) {
	t.aims, t.ids, t.idsOK, t.idsAt = nil, nil, false, 0
	if s == nil || len(s.Aims) == 0 {
		return
	}
	t.aims = make(map[entity.EntityID]Aim, len(s.Aims))
	for _, a := range s.Aims {
		t.aims[a.ID] = Aim{Target: a.Target, Dir: a.Dir, ShotAt: a.ShotAt, ReadyAt: a.ReadyAt, SeenAt: a.SeenAt}
	}
}
