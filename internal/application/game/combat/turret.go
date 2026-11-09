package combat

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Sentry turrets (PLAY-024). A powered turret looks for a target on a
// staggered cadence through the creature index, locks on, aims, and fires
// its weapon.SentryGun on the defence stream; it is shed last in a
// brownout (power.ClassDefense) and bursts when destroyed (Blast, from the
// game's building damage). Tuning: .agents/handoffs/PLAY.md#play024_plan.
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
// neither looks nor shoots).
//
// CBT-3: the sentry turret is the wiki's mini-turret (its gun, range and
// barrel), every turret's round is a projectile (Shots.Turret: strays,
// friendly fire and armor like a pawn's shot), and a turret badly hurt
// lights its fuse half the time and blows up (turret_fuse.go).
//
// MAP-13: a settlement's turret (Building.Hostile) needs no power, shoots
// colonists and tame animals, and nobody at the colony runs it until it is
// claimed (turret_hostile.go).

const (
	// TurretScanEvery: an idle turret looks for a target once in this
	// many ticks, on the tick (now+id)%TurretScanEvery == 0.
	TurretScanEvery = 20
	// TurretLock is the reach a target is picked up in (the sentry gun's
	// range, wiki: Mini-turret 28.9); TurretRelease the distance past
	// which a held target is let go (original margin).
	TurretLock    = 28
	TurretRelease = 30
	// TurretLost: a held target out of sight this long is let go.
	TurretLost = 20
	// turretAimTicks is the aim before the first shot at a new target.
	turretAimTicks = 30
	// A turret's blast (turret_fuse.go): the percent chance its fuse lights,
	// and its bomb (damage, radius in thousandths). wiki: Mini-turret,
	// Autocannon turret, Uranium slug turret.
	TurretBlastPct    = 50
	TurretBlastDamage = 50
	turretBlastRadius = 3900
	heavyBlastRadius  = 5900
	turretBlastPen    = 100 // the other bombs' 10 % (original assumption, as the mortar's HE shell)
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
// AX-5 adds the foam turret (the rocket battery is manned like a mortar:
// battery.go).
func isTurret(k construction.Kind) bool {
	switch k {
	case construction.SentryTurret, construction.Autocannon, construction.UraniumSlugTurret, construction.FoamTurret:
		return true
	}
	return false
}

// gunOf is the gun turret kind k fires.
func gunOf(k construction.Kind) weapon.Kind {
	switch k {
	case construction.Autocannon:
		return weapon.Autocannon
	case construction.UraniumSlugTurret:
		return weapon.SlugTurret
	case construction.FoamTurret:
		return weapon.FoamSpray
	case construction.RocketBattery: // its blast when smashed is named after its rockets
		return weapon.SwarmRocket
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

// reachOf is the lock and release distances of turret kind k: its gun's
// range (the sentry's is TurretLock) and the release margin past it.
func reachOf(k construction.Kind) (lock, release int) {
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
	case construction.FoamTurret:
		return "a foam turret"
	}
	return TurretLogName
}

// Arm is a turret's barrel and orders (DEF-3): Fired rounds since its
// barrel was last replaced and Hold, the player's "hold fire". AX-5: a
// chemfuel turret's Fired counts the charges or rockets used since its
// last reload (a new one is full), and Cool is the tick its wait after
// firing ends.
type Arm struct {
	Fired int
	Hold  bool
	Cool  uint64
}

// ArmSave is one Arm on disk.
type ArmSave struct {
	ID    entity.EntityID
	Fired int
	Hold  bool
	Cool  uint64 // AX-5; absent in older saves
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
	// MAP-21 (absent in older saves): turret fuses burning (turret_fuse.go).
	Sparks []Fuse
	// DEF-4 (absent in older saves): drafted switches and grenade belts,
	// grenade orders in their warmup, the combat log.
	Drafts []DraftSave
	Throws []Throw
	Log    []LogEntry
	// CBT-3 (absent in older saves): weapon makes, shield charges,
	// smokepop uses and combat records.
	Gear []KitSave
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
	// (their shots fly as projectiles, CBT-3), blasts a destroyed
	// turret's explosion.
	arms   map[entity.EntityID]Arm
	sparks []Fuse // burning fuses, in the order lit (turret_fuse.go)
	st     *Strikes
	blasts *Blasts
	// after are the defences stepped after the turrets every tick, saved
	// those whose state rides in the Turrets save block (DEF-3).
	after [6]phaser
	saved saved
	// AX-5: the fires a foam turret sees (foam.go) and its reused buffer.
	foam    Foamer
	fireBuf [][2]int
	// roster is colonyTarget's reused Near buffer (MAP-13).
	roster []Seen
}

// saved are the components whose state the Turrets save block carries.
type saved struct {
	traps   *Traps
	doors   *Doors
	mortars *Mortars
	rearms  *Rearms
	drafts  *Drafts
	log     *Log
	gear    *Gear // CBT-3
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
	slices.Sort(t.ids)
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
	if t.pw.Wired() || t.pw.Armed() { // MAP-13: a settlement's turret runs without a device on the map
		for _, id := range t.list() {
			b, ok := t.h.Building(id)
			if !ok {
				continue
			}
			t.step(id, b, now)
		}
	} else if len(t.aims) > 0 { // no power device on the map: no turret can be on
		t.list() // drops the aims of turrets that are gone
	}
	t.burnFuses(now) // turret_fuse.go
	for _, p := range t.after {
		if p != nil {
			p.phase(now)
		}
	}
}

// step is one turret's tick.
func (t *Turrets) step(id entity.EntityID, b construction.Building, now uint64) {
	k, x, y := b.Kind, b.X, b.Y
	a, had := t.aims[id]
	if now < a.StunnedUntil { // EMP: dark until it recovers
		return
	}
	if !b.Hostile && !t.pw.Powered(id) || t.arms[id].Hold || t.Spent(id, k) || now < t.arms[id].Cool {
		if had && a.Target != 0 { // no power, held or spent: it lets go and stays quiet
			a.Target, a.ReadyAt, a.Burst = 0, 0, 0
			t.setAim(id, a)
		}
		return
	}
	if k == construction.FoamTurret { // AX-5: it aims at fires, not creatures (foam.go)
		t.foamStep(id, x, y, now)
		return
	}
	wk := gunOf(k)
	d := weapon.DefOf(wk)
	lock, release := reachOf(k)
	var tx, ty int
	if a.Target != 0 {
		var keep bool
		if tx, ty, keep = t.holds(b, &a, now, release, d.MinRange); !keep {
			a.Target, a.ReadyAt, a.Burst = 0, 0, 0
			t.setAim(id, a)
		}
	}
	if a.Target == 0 {
		if (now+uint64(id))%TurretScanEvery != 0 {
			return
		}
		t.near = t.h.Near(x, y, lock, searchOf(b), t.near[:0])
		s, ok := nearest(t.near, x, y, func(s Seen) bool {
			return weapon.Distance(x, y, s.X, s.Y) >= d.MinRange && t.aimsAt(b, s, now) && t.sight.Line(x, y, s.X, s.Y)
		})
		if !ok {
			return
		}
		a.Target, a.ReadyAt, a.SeenAt, tx, ty = s.ID, now+turretAimTicks, now, s.X, s.Y
	}
	a.Dir = aimFrame(x, y, tx, ty, a.Dir)
	if a.SeenAt == now && now >= a.ReadyAt && weapon.ReachesAt(d, x, y, tx, ty) { // CBT-2: straight-line range
		t.shot(id, wk, x, y, a.Target, tx, ty)
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

// holds reports whether turret b keeps its target, with the target's
// tile; it notes the tick it saw it. A target that can no longer
// be shot at (down, dead, gone, calmed), is past release (or inside the
// gun's minimum range), or has been out of sight for TurretLost ticks is
// let go.
func (t *Turrets) holds(b construction.Building, a *Aim, now uint64, release, minRange int) (int, int, bool) {
	x, y := b.X, b.Y
	tx, ty, ok := t.h.Pos(a.Target)
	if d := weapon.Distance(x, y, tx, ty); !ok || !t.aimsAt(b, Seen{ID: a.Target, X: tx, Y: ty}, now) || d > release || d < minRange {
		return tx, ty, false
	}
	if t.sight.Line(x, y, tx, ty) {
		a.SeenAt = now
	}
	return tx, ty, now-a.SeenAt < TurretLost
}

// Targetable: a standing raider (never a prisoner, visitor or trader) or a
// hostile animal — one attacking a colonist, or a manhunter (a tame one
// gone manhunter too, EAI-006). Colonists and calm tame animals never
// are.
func (t *Turrets) Targetable(id entity.EntityID, now uint64) bool {
	if a, ok := t.h.Animal(id); ok {
		return !a.Dead() && (!a.Tame || a.Enraged(now)) && (t.h.Hostile(a) || a.Enraged(now)) // a tame manhunter too (EAI-006)
	}
	return t.h.StandingRaider(id)
}

// shot fires one round of gun wk from turret id at (fx, fy) on target at
// (tx, ty): a projectile (Shots.Turret, CBT-3), rolled on the defence
// stream.
func (t *Turrets) shot(id entity.EntityID, wk weapon.Kind, fx, fy int, target entity.EntityID, tx, ty int) {
	t.st.shots.Turret(id, wk, fx, fy, target, tx, ty)
}

// ForRaider is the powered turret a raider at (x, y) turns on instead of
// the nearest colonist cd tiles off: the nearest one within sight tiles,
// and only when it is at least SwitchMargin closer than the colonist (so a
// raider between the two never swaps back and forth). ok false: none. A
// settlement's turret is on the raiders' side and never offered (MAP-13).
func (t *Turrets) ForRaider(x, y, cd, sight int) (id entity.EntityID, bx, by int, ok bool) {
	if !t.pw.Wired() && !t.pw.Armed() {
		return 0, 0, 0, false // no power device and none claimed off a grid: no live turret
	}
	bestD := min(sight, cd-SwitchMargin) + 1
	for _, tid := range t.list() {
		b, ok := t.h.Building(tid)
		if !ok || b.Hostile || !t.pw.Powered(tid) {
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
	au, av := mathx.Abs(u), mathx.Abs(v)
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
		s.Arms = append(s.Arms, ArmSave{ID: id, Fired: r.Fired, Hold: r.Hold, Cool: r.Cool})
	}
	slices.SortFunc(s.Arms, func(x, y ArmSave) int { return cmp.Compare(x.ID, y.ID) })
	if v := t.saved; v.traps != nil {
		s.Fuses, s.Doors, s.Mortars, s.Rearms = v.traps.export(), v.doors.export(), v.mortars.export(), v.rearms.export()
	}
	if v := t.saved; v.drafts != nil {
		s.Drafts, s.Throws = v.drafts.export()
		s.Log = v.log.export()
	}
	if t.saved.gear != nil {
		s.Gear = t.saved.gear.export()
	}
	s.Sparks = slices.Clone(t.sparks)
	if len(s.Sparks)+len(s.Aims)+len(s.Arms)+len(s.Fuses)+len(s.Doors)+len(s.Mortars)+len(s.Rearms)+len(s.Drafts)+len(s.Throws)+len(s.Log)+len(s.Gear) == 0 {
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
	t.sparks = slices.Clone(s.Sparks)
	if v := t.saved; v.traps != nil {
		v.traps.importFuses(s.Fuses)
		v.doors.importDoors(s.Doors)
		v.mortars.importMortars(s.Mortars)
		v.rearms.importRearms(s.Rearms)
	}
	if v := t.saved; v.drafts != nil {
		v.drafts.importDrafts(s.Drafts, s.Throws)
		v.log.importLog(s.Log)
	}
	if t.saved.gear != nil {
		t.saved.gear.importGear(s.Gear)
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
		t.arms[r.ID] = Arm{Fired: r.Fired, Hold: r.Hold, Cool: r.Cool}
	}
}
