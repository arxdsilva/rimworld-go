package combat

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
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
//
// DEF-3 adds the heavy turrets (the autocannon and the uranium slug
// turret: wiki guns with a minimum range, the same lock and release
// margin), barrel wear (a barrel lasts a set number of rounds; a spent one
// stops the turret until a rearm, rearm.go) and "hold fire" (the turret
// neither looks nor shoots). The sentry turret keeps its PLAY-024 gun.

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

// Barrel life in rounds per turret kind. wiki: Mini-turret (60 rounds),
// Autocannon turret (90), Uranium slug turret (30 slugs)
const (
	SentryBarrel     = 60
	AutocannonBarrel = 90
	SlugBarrel       = 30
)

// isTurret reports a kind the turret code runs: the sentry turret and the
// two heavy turrets.
func isTurret(k construction.Kind) bool {
	return k == construction.SentryTurret || k == construction.Autocannon || k == construction.UraniumSlugTurret
}

// gunOf is the gun turret kind k fires.
func gunOf(k construction.Kind) weapon.Kind {
	switch k {
	case construction.Autocannon:
		return weapon.Autocannon
	case construction.UraniumSlugTurret:
		return weapon.SlugTurret
	}
	return weapon.SentryGun
}

// BarrelOf is the rounds a fresh barrel of turret kind k fires (0: no
// barrel).
func BarrelOf(k construction.Kind) int {
	switch k {
	case construction.SentryTurret:
		return SentryBarrel
	case construction.Autocannon:
		return AutocannonBarrel
	case construction.UraniumSlugTurret:
		return SlugBarrel
	}
	return 0
}

// reachOf is the lock and release distances of turret kind k: the
// sentry's PLAY-024 pair, a heavy turret its gun's range and the same
// release margin past it.
func reachOf(k construction.Kind) (lock, release int) {
	if k == construction.SentryTurret {
		return TurretLock, TurretRelease
	}
	r := weapon.DefOf(gunOf(k)).Range
	return r, r + TurretRelease - TurretLock
}

// turretName is how a log or message names turret kind k.
func turretName(k construction.Kind) string {
	switch k {
	case construction.Autocannon:
		return "an autocannon"
	case construction.UraniumSlugTurret:
		return "a slug turret"
	}
	return TurretLogName
}

// Arm is a turret's barrel and orders (DEF-3): Fired rounds since its
// barrel was last replaced and Hold, the player's "hold fire".
type Arm struct {
	Fired int
	Hold  bool
}

// ArmSave is one Arm on disk.
type ArmSave struct {
	ID    entity.EntityID
	Fired int
	Hold  bool
}

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
	// StunnedUntil: an EMP knocked it out until this tick (DEF-2).
	StunnedUntil uint64
	// Burst is the shots of the current burst already fired (DEF-3).
	Burst int8
}

// TurretSave is the saved turret state, nil while no turret ever held an
// aim (absent in older saves, so they hash as before). Gob field names are
// those of the game's former turretSave.
type TurretSave struct {
	Version int
	Aims    []AimSave // ascending by turret
	// DEF-3 (absent in older saves): barrels and hold fire, the mortars,
	// held or open doors, IED fuses burning and rearm hauls under way.
	Arms    []ArmSave
	Mortars []MortarSave
	Doors   []DoorSave
	Fuses   []Fuse
	Rearms  []Rearm
}

// AimSave is one Aim on disk.
type AimSave struct {
	ID, Target              entity.EntityID
	Dir                     int8
	ShotAt, ReadyAt, SeenAt uint64
	StunnedUntil            uint64 // DEF-2; absent in older saves
	Burst                   int8   // DEF-3; absent in older saves
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
	// arms are the barrels and hold-fire orders (DEF-3), st the strikes
	// (armor on a heavy turret's hit).
	arms map[entity.EntityID]Arm
	st   *Strikes
	// after are the defences stepped after the turrets every tick, saved
	// those whose state rides in the Turrets save block (DEF-3).
	after [4]phaser
	saved saved
}

// saved are the components whose state the Turrets save block carries.
type saved struct {
	traps   *Traps
	doors   *Doors
	mortars *Mortars
	rearms  *Rearms
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
	t.ids = slices.DeleteFunc(t.ids, func(id entity.EntityID) bool { // turrets only (an autodoor may draw too)
		b, ok := t.h.Building(id)
		return !ok || !isTurret(b.Kind)
	})
	t.idsOK, t.idsAt = true, gen
	for id := range t.aims {
		if _, ok := slices.BinarySearch(t.ids, id); !ok {
			delete(t.aims, id)
		}
	}
	for id := range t.arms {
		if _, ok := slices.BinarySearch(t.ids, id); !ok {
			delete(t.arms, id)
		}
	}
	return t.ids
}

// Phase runs every turret's search, aim and shot (the game's threat
// phase, after the raiders think), then the other defences' tick (IED
// fuses, doors, mortars, rearm hauls; DEF-3).
func (t *Turrets) Phase(now uint64) {
	if t.pw.Wired() {
		for _, id := range t.list() {
			b, ok := t.h.Building(id)
			if !ok {
				continue
			}
			t.step(id, b.Kind, b.X, b.Y, now)
		}
	} else if len(t.aims) > 0 { // no power device on the map: no turret can be on
		t.list() // drops the aims of turrets that are gone
	}
	for _, p := range t.after {
		if p != nil {
			p.phase(now)
		}
	}
}

// step is one turret's tick at (x, y).
func (t *Turrets) step(id entity.EntityID, k construction.Kind, x, y int, now uint64) {
	a, had := t.aims[id]
	if now < a.StunnedUntil { // EMP: dark until it recovers
		return
	}
	if !t.pw.Powered(id) || t.arms[id].Hold || t.Spent(id, k) {
		if had && a.Target != 0 { // no power, held or spent: it lets go and stays quiet
			a.Target, a.ReadyAt, a.Burst = 0, 0, 0
			t.setAim(id, a)
		}
		return
	}
	wk := gunOf(k)
	d := weapon.DefOf(wk)
	lock, release := reachOf(k)
	var tx, ty int
	if a.Target != 0 {
		var keep bool
		if tx, ty, keep = t.holds(x, y, &a, now, release, d.MinRange); !keep {
			a.Target, a.ReadyAt, a.Burst = 0, 0, 0
			t.setAim(id, a)
		}
	}
	if a.Target == 0 {
		if (now+uint64(id))%TurretScanEvery != 0 {
			return
		}
		t.near = t.h.Near(x, y, lock, Raiders|Animals, t.near[:0])
		s, ok := nearest(t.near, x, y, func(s Seen) bool {
			return weapon.Distance(x, y, s.X, s.Y) >= d.MinRange && t.Targetable(s.ID, now) && t.sight.Line(x, y, s.X, s.Y)
		})
		if !ok {
			return
		}
		a.Target, a.ReadyAt, a.SeenAt, tx, ty = s.ID, now+turretAimTicks, now, s.X, s.Y
	}
	a.Dir = aimFrame(x, y, tx, ty, a.Dir)
	if a.SeenAt == now && now >= a.ReadyAt && weapon.InRange(d, weapon.Distance(x, y, tx, ty)) {
		t.shot(id, k, wk, x, y, a.Target, tx, ty)
		t.wear(id)
		a.ShotAt, a.ReadyAt = now, now+uint64(d.Cooldown)
		if a.Burst++; int(a.Burst) < d.Burst { // the rest of the burst
			a.ReadyAt = now + uint64(d.BurstTicks)
		} else {
			a.Burst = 0
		}
	}
	t.setAim(id, a)
}

// wear counts a round off turret id's barrel.
func (t *Turrets) wear(id entity.EntityID) {
	if t.arms == nil {
		t.arms = map[entity.EntityID]Arm{}
	}
	r := t.arms[id]
	r.Fired++
	t.arms[id] = r
}

// Spent: turret id (of kind k) has fired its barrel out and stays quiet
// until a rearm.
func (t *Turrets) Spent(id entity.EntityID, k construction.Kind) bool {
	return t.arms[id].Fired >= BarrelOf(k)
}

// Barrel is the life left in turret id's barrel, permille (1000 fresh).
func (t *Turrets) Barrel(id entity.EntityID, k construction.Kind) int {
	n := BarrelOf(k)
	if n == 0 {
		return 0
	}
	return max(0, n-t.arms[id].Fired) * 1000 / n
}

// Rearm fits turret id with a fresh barrel.
func (t *Turrets) Rearm(id entity.EntityID) {
	if r, ok := t.arms[id]; ok {
		r.Fired = 0
		t.arms[id] = r
	}
}

// SetHold makes turret id hold fire (on) or fire at will. A held turret
// lets go of its target at once and neither looks nor shoots; released, it
// looks again on its next scan tick, so toggling never makes it flap.
func (t *Turrets) SetHold(id entity.EntityID, on bool) {
	if t.arms == nil {
		t.arms = map[entity.EntityID]Arm{}
	}
	r := t.arms[id]
	r.Hold = on
	t.arms[id] = r
	if a, ok := t.aims[id]; ok && on {
		a.Target, a.ReadyAt, a.Burst = 0, 0, 0
		t.setAim(id, a)
	}
}

// Held reports turret id holding fire.
func (t *Turrets) Held(id entity.EntityID) bool { return t.arms[id].Hold }

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
// be shot at (down, dead, gone, calmed), is past release (or inside the
// gun's minimum range), or has been out of sight for TurretLost ticks is
// let go.
func (t *Turrets) holds(x, y int, a *Aim, now uint64, release, minRange int) (int, int, bool) {
	tx, ty, ok := t.h.Pos(a.Target)
	if d := weapon.Distance(x, y, tx, ty); !ok || !t.Targetable(a.Target, now) || d > release || d < minRange {
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

// shot fires one round of gun wk from the turret (of kind k) at (fx, fy)
// on target at (tx, ty): the hit chance is a ranged strike's (the sentry
// gun's flat chance, a heavy gun's accuracy band, less the target's
// cover), rolled on the defence stream. A heavy gun's hit goes through
// armor. A hit is HurtAs with no attacker, so no animal or social reaction
// ever points at a building; the turret is named in the target's log
// instead.
func (t *Turrets) shot(id entity.EntityID, k construction.Kind, wk weapon.Kind, fx, fy int, target entity.EntityID, tx, ty int) {
	d := weapon.DefOf(wk)
	dist := weapon.Distance(fx, fy, tx, ty)
	h := t.h
	cov := t.sight.Cover(fx, fy, tx, ty)
	var hit bool
	if d.Legacy() {
		hit = h.DefenseRoll(100) < weapon.HitChance(d, dist)*(100-cov/10)/100
	} else {
		hit = h.DefenseRoll(1000) < weapon.HitPermille(d, dist)*(1000-cov)/1000
	}
	dmg, wound := d.Damage, body.Cut
	if hit && !d.Legacy() && t.st != nil {
		res := t.st.armorRoll(target, d.DamageType, d.Damage, weapon.Penetration(d, d.Damage), h.DefenseRoll)
		if dmg = res.Damage; res.Type == damage.Blunt {
			wound = body.Bruise
		}
	}
	now := h.Now()
	tr := projectile.Trace{
		FromX: fx, FromY: fy, ToX: tx, ToY: ty, Tick: now, Hit: hit,
		FromBuilding: true, Attacker: id, Target: target, Weapon: d.Name,
	}
	if hit {
		tr.Damage = dmg
	}
	h.Trace(tr)
	name := turretName(k)
	if !hit {
		h.Shot(target, false, 0, name)
		return
	}
	h.Shot(target, true, dmg, name)
	if dmg <= 0 {
		return // the armor stopped it
	}
	h.Hit(target, now, fx, fy, dmg)
	h.Stain(target, fx, fy, tx, ty, dmg) // before the hurt, while the target exists
	h.HurtAs(target, dmg, 0, wound, health.CauseInjury)
}

// Blast: turret b (already removed) blows up. Creatures within
// turretBlastRadius take TurretBlastCreature, object-layer buildings
// TurretBlastBuilding (a turret among them may blow up in turn); the
// renderer gets a burst and the cell may catch fire (defence stream).
func (t *Turrets) Blast(id entity.EntityID, b construction.Building) {
	delete(t.aims, id)
	delete(t.arms, id)
	h := t.h
	h.Burst(b.X, b.Y, h.Now(), explosion.Def{})
	h.Message(b.X, b.Y, fmt.Sprintf("The %s blew up!", construction.DefOf(b.Kind).Name))
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

// At is the turret (sentry or heavy) with building id (ok false: none).
func (t *Turrets) At(id entity.EntityID) (construction.Building, bool) {
	b, ok := t.h.Building(id)
	if !ok || !isTurret(b.Kind) {
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

// Stun knocks turret id out until tick until (an EMP, DEF-2): it lets go
// of its target and neither looks nor fires before then. A longer stun
// already running is kept. Turrets never adapt to EMP. wiki: Mini-turret
func (t *Turrets) Stun(id entity.EntityID, until uint64) {
	a := t.aims[id]
	a.Target, a.ReadyAt = 0, 0
	a.StunnedUntil = max(a.StunnedUntil, until)
	t.setAim(id, a)
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

// Export is the Turrets save field (nil: no turret ever aimed and no
// DEF-3 defence holds any state).
func (t *Turrets) Export() *TurretSave {
	s := &TurretSave{Version: TurretSaveVersion}
	for id, a := range t.aims {
		s.Aims = append(s.Aims, AimSave{ID: id, Target: a.Target, Dir: a.Dir, ShotAt: a.ShotAt, ReadyAt: a.ReadyAt, SeenAt: a.SeenAt,
			StunnedUntil: a.StunnedUntil, Burst: a.Burst})
	}
	slices.SortFunc(s.Aims, func(x, y AimSave) int { return cmp.Compare(x.ID, y.ID) })
	for id, r := range t.arms {
		s.Arms = append(s.Arms, ArmSave{ID: id, Fired: r.Fired, Hold: r.Hold})
	}
	slices.SortFunc(s.Arms, func(x, y ArmSave) int { return cmp.Compare(x.ID, y.ID) })
	if v := t.saved; v.traps != nil {
		s.Fuses, s.Doors, s.Mortars, s.Rearms = v.traps.export(), v.doors.export(), v.mortars.export(), v.rearms.export()
	}
	if len(s.Aims)+len(s.Arms)+len(s.Fuses)+len(s.Doors)+len(s.Mortars)+len(s.Rearms) == 0 {
		return nil
	}
	return s
}

// Import restores Export's state (nil: none) and drops the derived list.
func (t *Turrets) Import(s *TurretSave) {
	t.aims, t.arms, t.ids, t.idsOK, t.idsAt = nil, nil, nil, false, 0
	if s == nil {
		s = &TurretSave{}
	}
	if v := t.saved; v.traps != nil {
		v.traps.importFuses(s.Fuses)
		v.doors.importDoors(s.Doors)
		v.mortars.importMortars(s.Mortars)
		v.rearms.importRearms(s.Rearms)
	}
	if len(s.Aims) > 0 {
		t.aims = make(map[entity.EntityID]Aim, len(s.Aims))
	}
	for _, a := range s.Aims {
		t.aims[a.ID] = Aim{Target: a.Target, Dir: a.Dir, ShotAt: a.ShotAt, ReadyAt: a.ReadyAt, SeenAt: a.SeenAt, StunnedUntil: a.StunnedUntil,
			Burst: a.Burst}
	}
	if len(s.Arms) > 0 {
		t.arms = make(map[entity.EntityID]Arm, len(s.Arms))
	}
	for _, r := range s.Arms {
		t.arms[r.ID] = Arm{Fired: r.Fired, Hold: r.Hold}
	}
}
