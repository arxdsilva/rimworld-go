package combat

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Mortars (DEF-3). A mortar fires only while a colonist crews it (job
// ManMortar, standing on its spot): a drafted colonist standing there, the
// colonist a SetMortarTarget order named, or else the nearest idle
// drafted colonist within mortarCallR tiles is called when the mortar has
// something to shoot at. It lobs its loaded shells (Shells, filled by a
// rearm haul, rearm.go) at the player's forced target, or else at the
// nearest hostile, never nearer than its minimum range: a warmup, the
// shot, then the cooldown and the next warmup. Every shell lands within a
// forced-miss radius of the aim (scaled by the crewman's Shooting) and
// bursts through Blasts. A barrel lasts MortarBarrel shells. A crew stays
// at least MortarManMin ticks and is stood down only after
// MortarIdleRelease ticks with nothing to shoot, so it never flaps between
// manning and leaving. Every roll is on the defence stream.
type Mortars struct {
	h       Host
	f       Field
	turrets *Turrets // Targetable
	shots   *Shots
	states  map[entity.EntityID]Mortar // saved
	ids     []entity.EntityID          // the mortars, ascending (derived)
	idsAt   uint64
	idsOK   bool
	near    []Seen
}

// Mortar numbers. wiki: Mortar (20 shells a barrel); the rest original
// tuning.
const (
	MortarBarrel = 20
	// MortarRack is how many shells a mortar keeps loaded beside it.
	MortarRack = 10
	// MortarManMin is the least a crew stays; MortarIdleRelease how long
	// a crew waits with nothing to shoot before it is stood down.
	MortarManMin      = 600
	MortarIdleRelease = 2500
	// mortarCallR is how near an idle drafted colonist must be to be
	// called to an unmanned mortar.
	mortarCallR = 12
	// mortarScanR caps the hostile search (the creature index clamps it
	// to the map).
	mortarScanR = 250
)

// Mortar is one mortar's state. All fields are exported for the save.
type Mortar struct {
	// Crew mans it (0: nobody), since ManSince; Assigned is the colonist
	// a SetMortarTarget order named, waiting to be called.
	Crew, Assigned entity.EntityID
	ManSince       uint64
	// Forced: the player's target TX, TY. Target is the hostile it aims
	// at otherwise (0: none); SeenAt the last tick it had a target.
	Forced bool
	TX, TY int
	Target entity.EntityID
	SeenAt uint64
	// Aiming: the warmup runs until ReadyAt.
	Aiming  bool
	ReadyAt uint64
	// Shells loaded per shell kind (high explosive, incendiary, EMP,
	// smoke); Fired shells since its barrel was fitted.
	Shells [4]int
	Fired  int
	// Hostile (DEF-4): a besieging raid's mortar, crewed by its raiders
	// and aimed at the colonists; colonists never man or rearm it.
	Hostile bool
}

// MortarSave is one Mortar on disk.
type MortarSave struct {
	ID entity.EntityID
	M  Mortar
}

// list is the mortars, ascending.
func (m *Mortars) list(now uint64) []entity.EntityID {
	if m.idsOK && now >= m.idsAt && now < m.idsAt+doorListEvery {
		return m.ids
	}
	if m.f != nil {
		m.ids = m.f.AppendBuilt(isMortar, m.ids[:0])
	}
	m.idsAt, m.idsOK = now, true
	return m.ids
}

func isMortar(k construction.Kind) bool { return k == construction.Mortar }

// spot is where the crew of mortar b stands.
func spot(b construction.Building) (int, int) {
	_, s, ok := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if !ok {
		return b.X, b.Y + 1
	}
	return s[0], s[1]
}

// phase crews, aims and fires every mortar.
func (m *Mortars) phase(now uint64) {
	for _, id := range m.list(now) {
		b, ok := m.h.Building(id)
		if !ok || b.Kind != construction.Mortar {
			continue
		}
		st := m.states[id]
		before := st
		m.step(id, b, &st, now)
		if st != before {
			m.set(id, st)
		}
	}
	for id := range m.states {
		if _, ok := m.h.Building(id); !ok {
			delete(m.states, id)
		}
	}
}

func (m *Mortars) set(id entity.EntityID, st Mortar) {
	if m.states == nil {
		m.states = map[entity.EntityID]Mortar{}
	}
	m.states[id] = st
}

// step is one mortar's tick.
func (m *Mortars) step(id entity.EntityID, b construction.Building, st *Mortar, now uint64) {
	h := m.h
	scan := (now+uint64(id))%TurretScanEvery == 0
	if st.Crew != 0 && !m.crewing(st.Crew, id, st.Hostile) {
		st.Crew, st.Aiming = 0, false
	}
	ax, ay, aim := m.aim(id, b, st, now, scan)
	if aim {
		st.SeenAt = now
	}
	if st.Crew == 0 {
		if !aim || !scan || st.Hostile {
			return // a raiders' mortar waits for its raid to send a crew
		}
		sx, sy := spot(b)
		m.call(id, st, sx, sy, now)
		return
	}
	cx, cy, _ := h.Pos(st.Crew)
	if j, _ := h.Job(st.Crew); j.State != job.Do || cx != j.TX || cy != j.TY {
		return // still walking up
	}
	if !aim {
		st.Aiming = false
		if !st.Forced && !st.Hostile && now >= st.ManSince+MortarManMin && now >= st.SeenAt+MortarIdleRelease {
			m.standDown(st)
		}
		return
	}
	if st.Fired >= MortarBarrel || st.shells() == 0 {
		return
	}
	if !st.Aiming {
		st.Aiming, st.ReadyAt = true, max(st.ReadyAt, now+uint64(weapon.DefOf(weapon.MortarHE).Warmup))
	}
	if now >= st.ReadyAt {
		m.fire(b, st, ax, ay, now)
	}
}

// crewing: colonist c (a raider for a hostile mortar) is on its way to or
// manning mortar id.
func (m *Mortars) crewing(c, id entity.EntityID, hostile bool) bool {
	j, ok := m.h.Job(c)
	standing := m.h.StandingPawn(c)
	if hostile {
		standing = m.h.StandingRaider(c)
	}
	return ok && j.Kind == job.ManMortar && j.Target == id && (j.State == job.Goto || j.State == job.Do) && standing
}

// standDown ends the crew's job: it goes back to its day.
func (m *Mortars) standDown(st *Mortar) {
	if j, ok := m.h.Job(st.Crew); ok && j.Kind == job.ManMortar {
		j.State = job.Complete
		m.h.SetJob(st.Crew, j)
	}
	st.Crew, st.Aiming = 0, false
}

// inBand: (tx, ty) lies between the mortar's minimum and full range from
// (x, y).
func inBand(x, y, tx, ty int) bool {
	return weapon.InRange(weapon.DefOf(weapon.MortarHE), weapon.Distance(x, y, tx, ty))
}

// aim is where mortar id aims now: its forced target in range, else the
// hostile it follows (looked for again on scan ticks).
func (m *Mortars) aim(id entity.EntityID, b construction.Building, st *Mortar, now uint64, scan bool) (int, int, bool) {
	if st.Forced {
		return st.TX, st.TY, inBand(b.X, b.Y, st.TX, st.TY)
	}
	if st.Target != 0 {
		if x, y, ok := m.h.Pos(st.Target); ok && (st.Hostile && m.colonist(st.Target) || !st.Hostile && m.turrets.Targetable(st.Target, now)) &&
			inBand(b.X, b.Y, x, y) {
			return x, y, true
		}
		st.Target = 0
	}
	if !scan {
		return 0, 0, false
	}
	who, hostile := Raiders|Animals, st.Hostile
	if hostile {
		who = Pawns
	}
	m.near = m.h.Near(b.X, b.Y, mortarScanR, who, m.near[:0])
	s, ok := nearest(m.near, b.X, b.Y, func(s Seen) bool {
		return inBand(b.X, b.Y, s.X, s.Y) && (hostile && m.colonist(s.ID) || !hostile && m.turrets.Targetable(s.ID, now))
	})
	if !ok {
		return 0, 0, false
	}
	st.Target = s.ID
	return s.X, s.Y, true
}

// call finds mortar id a crew: a drafted colonist on its spot, the
// colonist an order named, or the nearest idle drafted colonist near it.
func (m *Mortars) call(id entity.EntityID, st *Mortar, sx, sy int, now uint64) {
	h := m.h
	m.near = h.Near(sx, sy, mortarCallR, Pawns, m.near[:0])
	pick := entity.EntityID(0)
	for _, s := range m.near {
		if s.X == sx && s.Y == sy && m.idleDrafted(s.ID) {
			pick = s.ID
		}
	}
	if pick == 0 && st.Assigned != 0 && h.StandingPawn(st.Assigned) && !h.StandingRaider(st.Assigned) {
		pick = st.Assigned
		h.Interrupt(pick, job.Cancelled) // a player's order: it drops what it does
	}
	if pick == 0 {
		if s, ok := nearest(m.near, sx, sy, func(s Seen) bool { return m.idleDrafted(s.ID) }); ok {
			pick = s.ID
		}
	}
	if pick == 0 || !h.StartJob(pick, job.Record{Kind: job.ManMortar, Target: id, TX: sx, TY: sy, Forced: true}) {
		return
	}
	st.Crew, st.Assigned, st.ManSince, st.SeenAt, st.Aiming = pick, 0, now, now, false
}

// idleDrafted: c is a drafted colonist standing about with no order.
func (m *Mortars) idleDrafted(c entity.EntityID) bool {
	h := m.h
	if !h.Drafted(c) || !h.StandingPawn(c) || h.StandingRaider(c) {
		return false
	}
	j, _ := h.Job(c)
	return j.Kind == job.None || j.State == job.Idle || j.State == job.Complete || j.Kind.IsIdle()
}

// shells is how many shells are loaded.
func (st Mortar) shells() int { return st.Shells[0] + st.Shells[1] + st.Shells[2] + st.Shells[3] }

// fire lobs one shell (the first loaded kind, high explosive first) from
// mortar b at (ax, ay): it lands within the forced-miss radius.
func (m *Mortars) fire(b construction.Building, st *Mortar, ax, ay int, now uint64) {
	i := 0
	for st.Shells[i] == 0 {
		i++
	}
	st.Shells[i]--
	st.Fired++
	d := weapon.DefOf(weapon.MortarHE)
	st.ReadyAt = now + uint64(d.Cooldown+d.Warmup)
	lx, ly := m.scatter(ax, ay, MissRadius(m.crewLevel(st.Crew)))
	m.shots.Lob(st.Crew, weapon.MortarHE+weapon.Kind(i), b.X, b.Y, ax, ay, lx, ly)
}

// MissRadius is the mortar's forced-miss radius (thousandths of a tile)
// for a crewman of Shooting level: the mortar's 9 tiles times 1.2 at
// level 0, 1.0 at level 8, 0.7 at level 20 (-0.025 a level). wiki:
// Mortar, Shooting
func MissRadius(level int) int {
	f := 1200 - 25*min(max(level, 0), 20)
	return weapon.MortarForcedMiss * f / 1000
}

// crewLevel is crewman c's Shooting level, read back from its accuracy
// per tile (the highest level whose accuracy it reaches).
func (m *Mortars) crewLevel(c entity.EntityID) int {
	pt, _ := m.h.ShotAim(c)
	lvl := 0
	for l := 0; l <= 20; l++ {
		if attack.ShooterPerTile(l) <= pt {
			lvl = l
		}
	}
	return lvl
}

// scatter is a cell within rad (thousandths of a tile) of (x, y),
// uniform over the disc, rolled on the defence stream (the aim itself
// after a few misses of the disc).
func (m *Mortars) scatter(x, y, rad int) (int, int) {
	r := (rad + 500) / 1000
	if r <= 0 {
		return x, y
	}
	for range 8 {
		dx, dy := m.h.DefenseRoll(2*r+1)-r, m.h.DefenseRoll(2*r+1)-r
		if dx*dx+dy*dy <= r*r {
			return x + dx, y + dy
		}
	}
	return x, y
}

// SetTarget forces mortar id to fire at (x, y); crew (0: none) is the
// colonist to call to it. A raiders' mortar takes no orders.
func (m *Mortars) SetTarget(id entity.EntityID, x, y int, crew entity.EntityID) {
	st := m.states[id]
	if st.Hostile {
		return
	}
	st.Forced, st.TX, st.TY, st.Aiming = true, x, y, false
	if crew != 0 {
		st.Assigned = crew
	}
	m.set(id, st)
}

// ClearTarget drops mortar id's forced target: it picks hostiles itself.
func (m *Mortars) ClearTarget(id entity.EntityID) {
	st, ok := m.states[id]
	if !ok {
		return
	}
	st.Forced, st.TX, st.TY, st.Aiming = false, 0, 0, false
	m.set(id, st)
}

// Siege mortar shells (DEF-4). wiki: Raid, a siege bombards with high
// explosive and incendiary shells; the split of one barrel's 20 is
// original tuning.
const (
	SiegeHEShells         = 15
	SiegeIncendiaryShells = 5
)

// Raise makes mortar id a besieging raid's, loaded for one barrel.
func (m *Mortars) Raise(id entity.EntityID) {
	st := m.states[id]
	st.Hostile = true
	st.Shells = [4]int{SiegeHEShells, SiegeIncendiaryShells, 0, 0}
	m.set(id, st)
	m.idsOK = false // a new mortar: list again
}

// Crew is who mans mortar id (0: nobody); ok false when it is gone.
func (m *Mortars) Crew(id entity.EntityID) (entity.EntityID, bool) {
	b, ok := m.h.Building(id)
	if !ok || b.Kind != construction.Mortar {
		return 0, false
	}
	return m.states[id].Crew, true
}

// Man sends raider c to crew the raiders' mortar id.
func (m *Mortars) Man(c, id entity.EntityID) bool {
	b, ok := m.h.Building(id)
	st := m.states[id]
	if !ok || b.Kind != construction.Mortar || !st.Hostile || st.Crew != 0 {
		return false
	}
	sx, sy := spot(b)
	if !m.h.StartJob(c, job.Record{Kind: job.ManMortar, Target: id, TX: sx, TY: sy}) {
		return false
	}
	st.Crew, st.ManSince, st.SeenAt, st.Aiming = c, m.h.Now(), m.h.Now(), false
	m.set(id, st)
	return true
}

// colonist: a standing colonist (a raiders' mortar's mark).
func (m *Mortars) colonist(id entity.EntityID) bool {
	return m.h.StandingPawn(id) && !m.h.StandingRaider(id)
}

// State is mortar id's state (zero: none).
func (m *Mortars) State(id entity.EntityID) Mortar { return m.states[id] }

// Shells is the total loaded in mortar id.
func (m *Mortars) Shells(id entity.EntityID) int { return m.states[id].shells() }

// Barrel is the life left in mortar id's barrel, permille.
func (m *Mortars) Barrel(id entity.EntityID) int {
	return max(0, MortarBarrel-m.states[id].Fired) * 1000 / MortarBarrel
}

// wants is the load mortar id needs: a barrel (steel) when spent, else
// shells while its rack is under half full; ok false: nothing.
func (m *Mortars) wants(id entity.EntityID) (barrel bool, room int) {
	st := m.states[id]
	if st.Hostile {
		return false, 0 // the raiders' own
	}
	if st.Fired >= MortarBarrel {
		return true, 0
	}
	if n := st.shells(); n < MortarRack/2 {
		return false, MortarRack - n
	}
	return false, 0
}

// load puts n shells of kind k into mortar id (the rack keeps
// MortarRack).
func (m *Mortars) load(id entity.EntityID, k item.Kind, n int) int {
	wk, ok := weapon.ForShell(k)
	if !ok {
		return 0
	}
	st := m.states[id]
	n = min(n, MortarRack-st.shells())
	if n <= 0 {
		return 0
	}
	st.Shells[wk-weapon.MortarHE] += n
	m.set(id, st)
	return n
}

// rearm fits mortar id with a fresh barrel.
func (m *Mortars) rearm(id entity.EntityID) {
	st := m.states[id]
	st.Fired = 0
	m.set(id, st)
}

// export is the mortars with any state, ascending (nil when none).
func (m *Mortars) export() []MortarSave {
	if len(m.states) == 0 {
		return nil
	}
	out := make([]MortarSave, 0, len(m.states))
	for id, st := range m.states {
		out = append(out, MortarSave{ID: id, M: st})
	}
	slices.SortFunc(out, func(a, b MortarSave) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// importMortars restores saved mortars and drops the derived list.
func (m *Mortars) importMortars(s []MortarSave) {
	m.states, m.ids, m.idsOK = nil, nil, false
	for _, v := range s {
		m.set(v.ID, v.M)
	}
}
