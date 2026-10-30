package raid

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/entity"
)

// mechWorld is a tiny map for the mechanoids: creatures on tiles, the
// colonists among them, plants, buildings and what the mechanoids did.
// Any host call it does not answer panics (the embedded nil roles).
type mechWorld struct {
	Host
	Wild
	MechHost
	w, h      int
	next      entity.EntityID
	pos       map[entity.EntityID][2]int
	animals   map[entity.EntityID]animal.State
	colonists []entity.EntityID
	downed    map[entity.EntityID]bool
	blocked   map[[2]int]bool // tiles that break a line of sight
	plants    map[[2]int]bool
	drops     map[item.Kind]int
	parts     int
	rolls     int
	shots     []entity.EntityID
	goals     [][2]int
	threats   []string
	home      [2]int
	turret    entity.EntityID // a sentry turret (0: none) on turretAt
	turretAt  [2]int
}

func newMechWorld() *mechWorld {
	return &mechWorld{w: 120, h: 120, next: 100, pos: map[entity.EntityID][2]int{}, animals: map[entity.EntityID]animal.State{},
		downed: map[entity.EntityID]bool{}, blocked: map[[2]int]bool{}, plants: map[[2]int]bool{}, drops: map[item.Kind]int{},
		home: [2]int{60, 60}}
}

func (m *mechWorld) mechs() (*Mechs, *ShipParts) { return newMechs(m, Roles{Wild: m, Mech: m}) }

func (m *mechWorld) colonist(x, y int) entity.EntityID {
	m.next++
	m.pos[m.next] = [2]int{x, y}
	m.colonists = append(m.colonists, m.next)
	return m.next
}

func (m *mechWorld) mech(s animal.Species, x, y int) entity.EntityID {
	id, err := m.SpawnAnimal(s, x, y)
	if err != nil {
		panic(err)
	}
	return id
}

// Host
func (m *mechWorld) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := m.pos[id]
	return p[0], p[1], ok
}
func (m *mechWorld) Vital(id entity.EntityID) health.Record {
	if m.downed[id] {
		return health.Record{Status: health.Downed}
	}
	return health.Record{}
}
func (m *mechWorld) RaidTarget(x, y int) (entity.EntityID, int, int, bool) {
	best, bd := entity.EntityID(0), 1<<30
	for _, c := range m.colonists {
		p, ok := m.pos[c]
		if !ok || m.downed[c] {
			continue
		}
		if d := weapon.Distance(x, y, p[0], p[1]); d < bd {
			best, bd = c, d
		}
	}
	p := m.pos[best]
	return best, p[0], p[1], best != 0
}
func (m *mechWorld) TurretFor(x, y, cd, sight int) (entity.EntityID, int, int, bool) {
	if m.turret == 0 || weapon.Distance(x, y, m.turretAt[0], m.turretAt[1]) >= cd {
		return 0, 0, 0, false
	}
	return m.turret, m.turretAt[0], m.turretAt[1], true
}
func (m *mechWorld) CanHit(x, y int, d weapon.Def, tx, ty int) bool {
	return weapon.InRange(d, weapon.Distance(x, y, tx, ty)) && m.Line(x, y, tx, ty)
}
func (m *mechWorld) Roll(n int) int                            { m.rolls++; return m.rolls % n }
func (m *mechWorld) Home() (int, int)                          { return m.home[0], m.home[1] }
func (m *mechWorld) Threat(title string, _, _ int, _ string)   { m.threats = append(m.threats, title) }
func (m *mechWorld) Message(int, int, string)                  {}
func (m *mechWorld) InBounds(x, y int) bool                    { return x >= 0 && y >= 0 && x < m.w && y < m.h }
func (m *mechWorld) Rock(int, int) bool                        { return false }
func (m *mechWorld) Size() (int, int)                          { return m.w, m.h }
func (m *mechWorld) WalkableNear(x, y, r int) (int, int, bool) { return x, y, true }

// Wild
func (m *mechWorld) SpawnAnimal(s animal.Species, x, y int) (entity.EntityID, error) {
	m.next++
	m.pos[m.next] = [2]int{x, y}
	m.animals[m.next] = animal.State{Species: s}
	return m.next, nil
}
func (m *mechWorld) Animal(id entity.EntityID) animal.State       { return m.animals[id] }
func (m *mechWorld) SetAnimal(id entity.EntityID, a animal.State) { m.animals[id] = a }
func (m *mechWorld) EdgeTile() (int, int, bool)                   { return 0, 60, true }

// MechHost
func (m *mechWorld) Shoot(id, target entity.EntityID, wk weapon.Kind) {
	m.shots = append(m.shots, target)
}
func (m *mechWorld) GoTo(id entity.EntityID, a animal.State, x, y int) animal.State {
	m.goals = append(m.goals, [2]int{x, y})
	a.GoalX, a.GoalY = x, y
	return a
}
func (m *mechWorld) Line(x, y, tx, ty int) bool {
	for p := range m.blocked {
		if min(x, tx) <= p[0] && p[0] <= max(x, tx) && min(y, ty) <= p[1] && p[1] <= max(y, ty) {
			return false
		}
	}
	return true
}
func (m *mechWorld) MechsNear(x, y, r int, buf []entity.EntityID) []entity.EntityID {
	for id := range m.animals {
		if p := m.pos[id]; animal.IsMech(m.animals[id].Species) && weapon.Distance(x, y, p[0], p[1]) <= r {
			buf = append(buf, id)
		}
	}
	return buf
}
func (m *mechWorld) ColonistNear(x, y, r int) bool {
	for _, c := range m.colonists {
		if p, ok := m.pos[c]; ok && !m.downed[c] && weapon.Distance(x, y, p[0], p[1]) <= r {
			return true
		}
	}
	return false
}
func (m *mechWorld) PlaceShipPart(x, y int) (entity.EntityID, bool) {
	m.next++
	m.parts++
	return m.next, true
}
func (m *mechWorld) KillPlant(x, y int)                 { delete(m.plants, [2]int{x, y}) }
func (m *mechWorld) Drop(kind item.Kind, qty, x, y int) { m.drops[kind] += qty }

// tick is one tick of the game's animal phase for mechanoid id.
func tick(ms *Mechs, w *mechWorld, id entity.EntityID, now uint64) {
	a := w.animals[id]
	if a.Dormant {
		return
	}
	if a.Cooldown > 0 {
		a.Cooldown--
	}
	ms.Think(id, &a, now)
	w.animals[id] = a
}

// TestMechShootsOnItsWeaponsCycle (DEF-4): a sniper in range of a colonist
// stands and fires its own weapon: the first shot after its aim, then one
// every cooldown plus aim, and it is hostile the whole time.
func TestMechShootsOnItsWeaponsCycle(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	c := w.colonist(20, 10)
	id := w.mech(animal.Spindle, 0, 10)
	ms, _ := w.mechs()
	wd := weapon.DefOf(weapon.PulseLance)
	var at []uint64
	for now := uint64(1); now <= 1000; now++ {
		n := len(w.shots)
		tick(ms, w, id, now)
		if len(w.shots) > n {
			at = append(at, now)
		}
	}
	if len(at) < 3 || w.shots[0] != c || len(w.goals) != 0 {
		t.Fatalf("shots at %v on %v, goals %v", at, w.shots, w.goals)
	}
	if gap := at[1] - at[0]; gap != uint64(wd.Cooldown+wd.Warmup) {
		t.Errorf("gap %d, want its cooldown and aim (%d)", gap, wd.Cooldown+wd.Warmup)
	}
	if a := w.animals[id]; a.Mode != animal.Attack || !a.Enraged(1000) || a.Target != c {
		t.Errorf("a mechanoid is hostile, on its target: %+v", a)
	}
}

// TestMechClosesInAndBursts: a blade unit out of reach walks at its target;
// the heavy gunner fires its whole burst, a shot every burst gap.
func TestMechClosesInAndBursts(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	w.colonist(30, 10)
	blade := w.mech(animal.Harrow, 0, 10)
	gun := w.mech(animal.Bulwark, 5, 10) // the minigun reaches 30
	ms, _ := w.mechs()
	tick(ms, w, blade, 1)
	if len(w.goals) != 1 || w.goals[0] != [2]int{30, 10} || len(w.shots) != 0 {
		t.Fatalf("the blade closes in: goals %v shots %v", w.goals, w.shots)
	}
	wd := weapon.DefOf(weapon.Minigun)
	for now := uint64(1); now <= uint64(wd.Warmup+wd.Burst*wd.BurstTicks+10); now++ {
		tick(ms, w, gun, now)
	}
	if len(w.shots) != wd.Burst {
		t.Fatalf("%d shots, want the minigun's burst of %d", len(w.shots), wd.Burst)
	}
}

// TestMechTargetFalls: a downed colonist is dropped at once for the next.
func TestMechTargetFalls(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	a, b := w.colonist(10, 10), w.colonist(14, 10)
	id := w.mech(animal.Spindle, 0, 10)
	ms, _ := w.mechs()
	tick(ms, w, id, 1)
	if w.animals[id].Target != a {
		t.Fatalf("target %d, want the nearest %d", w.animals[id].Target, a)
	}
	w.downed[a] = true
	tick(ms, w, id, 2)
	if w.animals[id].Target != b {
		t.Fatalf("target %d, want %d once %d is down", w.animals[id].Target, b, a)
	}
	w.downed[b] = true
	w.animals[id] = animal.State{Species: animal.Spindle, Target: b, Mode: animal.Attack}
	tick(ms, w, id, 3)
	if st := w.animals[id]; st.Target != 0 || len(w.goals) != 1 || w.goals[0] != w.home {
		t.Fatalf("nobody standing: it heads for the colony, %+v goals %v", st, w.goals)
	}
}

// TestEMPStunsThenAdapts (DEF-4): an EMP stuns the mechanoids it reaches
// for its stun at their distance; a stunned one neither moves nor shoots;
// for EMPAdaptTicks after a stun another EMP does nothing (no stun-lock);
// then it stuns again. Out of reach, behind a wall or a blast of another
// kind: nothing.
func TestEMPStunsThenAdapts(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	w.colonist(25, 10)
	id := w.mech(animal.Spindle, 10, 10)
	far := w.mech(animal.Harrow, 40, 40)
	hid := w.mech(animal.Harrow, 12, 14)
	w.blocked[[2]int{11, 12}] = true
	ms, _ := w.mechs()
	emp := explosion.Def{Kind: explosion.EMP, Radius: 3500, Damage: 50} // wiki: EMP grenade
	ms.EMP(10, 10, explosion.Def{Kind: explosion.Bomb, Radius: 3500, Damage: 50}, 1)
	if ms.Stunned(id, 1) {
		t.Fatal("a bomb stuns nothing")
	}
	ms.EMP(10, 10, emp, 1)
	stun := uint64(emp.StunTicks(0))
	if !ms.Stunned(id, 1) || !ms.Stunned(id, stun) || ms.Stunned(id, 1+stun) || ms.Stunned(far, 1) || ms.Stunned(hid, 1) {
		t.Fatalf("stun window: %v %v %v far %v hid %v", ms.Stunned(id, 1), ms.Stunned(id, stun), ms.Stunned(id, 1+stun),
			ms.Stunned(far, 1), ms.Stunned(hid, 1))
	}
	for now := uint64(1); now <= stun; now++ {
		tick(ms, w, id, now)
	}
	if len(w.shots) != 0 || len(w.goals) != 0 {
		t.Fatalf("stunned, it acted: shots %v goals %v", w.shots, w.goals)
	}
	ms.EMP(10, 10, emp, stun+5) // adapted
	if ms.Stunned(id, stun+5) {
		t.Fatal("an adapted mechanoid shrugs off the EMP")
	}
	again := uint64(1 + animal.EMPAdaptTicks)
	ms.EMP(10, 10, emp, again)
	if !ms.Stunned(id, again) {
		t.Fatal("past its adaptation an EMP stuns it again")
	}
	for now := again; now < again+stun+uint64(weapon.DefOf(weapon.PulseLance).Warmup)+5; now++ {
		tick(ms, w, id, now)
	}
	if len(w.shots) == 0 {
		t.Fatal("rebooted, it fights again")
	}
}

// TestGuardLeashNoFlap: a woken guard takes on colonists within GuardLeash
// of its post and keeps one until beyond GuardRelease: a colonist pacing
// between the two never makes it flip between chasing and going home.
func TestGuardLeashNoFlap(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	c := w.colonist(GuardLeash+3, 0)
	id := w.mech(animal.Harrow, 0, 0)
	ms, _ := w.mechs()
	ms.recs[id] = MechRecord{Guard: true}
	tick(ms, w, id, 1)
	if w.animals[id].Target != 0 {
		t.Fatal("beyond its leash a colonist is left alone")
	}
	w.pos[c] = [2]int{GuardLeash - 2, 0}
	tick(ms, w, id, 2)
	if w.animals[id].Target != c {
		t.Fatal("within its leash it goes for the colonist")
	}
	flips, had := 0, true
	for now := uint64(3); now < 600; now++ {
		x := GuardLeash + 1 + int(now%5) // pacing between the leash and the release
		w.pos[c] = [2]int{x, 0}
		w.pos[id] = [2]int{0, 0}
		tick(ms, w, id, now)
		if has := w.animals[id].Target == c; has != had {
			flips++
			had = has
		}
	}
	if flips != 0 {
		t.Fatalf("the guard flipped %d times between the leash and the release", flips)
	}
	w.pos[c] = [2]int{GuardRelease + 1, 0}
	tick(ms, w, id, 600)
	if w.animals[id].Target != 0 {
		t.Fatal("beyond the release it lets go")
	}
}

// TestMechGroupAndRaid: a group spends its points on what it can afford
// (at least the cheapest, at most mechGroupMax); a raid lands it as its
// arrival says, every one of them hostile.
func TestMechGroupAndRaid(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	ms, _ := w.mechs()
	if g := ms.Group(0); len(g) != 1 || g[0] != animal.Quill {
		t.Fatalf("no points: one skirmisher, got %v", g)
	}
	if g := ms.Group(1_000_000); len(g) != mechGroupMax {
		t.Fatalf("%d in a huge group, want %d", len(g), mechGroupMax)
	}
	g, spent := ms.Group(1000), 0
	for _, s := range g {
		m, _ := animal.MechOf(s)
		spent += m.CombatPower
	}
	if spent > 1000 || 1000-spent >= 110 {
		t.Fatalf("1000 points bought %v for %d", g, spent)
	}
	if n := ms.Raid(600, raiddef.DropPodCentre, 7); n == 0 || len(w.threats) != 1 {
		t.Fatalf("pod raid: %d came, letters %v", n, w.threats)
	}
	for id, a := range w.animals {
		p := w.pos[id]
		if !a.Enraged(1<<40) || a.Mode != animal.Attack || weapon.Distance(p[0], p[1], 60, 60) > 2 {
			t.Errorf("mech %d at %v: %+v", id, p, a)
		}
	}
	before := len(w.animals)
	if n := ms.Raid(300, raiddef.WalkInEdge, 8); n == 0 {
		t.Fatal("walk-in raid: nobody came")
	}
	for id := range w.animals {
		if p := w.pos[id]; id > entity.EntityID(100+before) && p[0] > 2 {
			t.Errorf("a walk-in mech landed at %v, not the edge", p)
		}
	}
	ms.Raid(300, raiddef.DropPodScatter, 9)
	if len(w.threats) != 3 {
		t.Fatalf("letters %v", w.threats)
	}
}

// TestMechSaveRoundTrip: the stuns, guard posts and ship parts survive a
// save; nothing to keep exports nil and an older save imports clean.
func TestMechSaveRoundTrip(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	ms, sp := w.mechs()
	if ms.Export() != nil {
		t.Fatal("nothing to keep: nil")
	}
	id := w.mech(animal.Harrow, 10, 10)
	ms.EMP(10, 10, explosion.Def{Kind: explosion.EMP, Radius: 3500, Damage: 50}, 5)
	w.home = [2]int{0, 0}
	if !sp.Crash(500, 5) {
		t.Fatal("no crash")
	}
	saved := ms.Export()
	w2 := newMechWorld()
	back, bsp := w2.mechs()
	back.Import(saved)
	if !back.Stunned(id, 6) || bsp.Parts() != 1 || len(bsp.parts[0].Guards) == 0 || !back.recs[bsp.parts[0].Guards[0]].Guard {
		t.Fatalf("restored: %+v parts %+v", back.recs, bsp.parts)
	}
	saved.Parts[0].Guards[0] = 0 // the restored copy is its own
	if bsp.parts[0].Guards[0] == 0 {
		t.Fatal("import aliases the save")
	}
	back.Import(nil)
	if back.Stunned(id, 6) || bsp.Parts() != 0 {
		t.Fatal("an older save has no mechanoid state")
	}
	ms.Forget(id)
	if ms.Stunned(id, 6) {
		t.Fatal("a broken mechanoid is forgotten")
	}
}

// TestMechTurnsOnTurret: a turret well closer than any colonist draws a
// mechanoid's fire; once it is gone the mechanoid turns to the colonist.
func TestMechTurnsOnTurret(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	c := w.colonist(30, 10)
	w.turret, w.turretAt = 500, [2]int{8, 10}
	id := w.mech(animal.Spindle, 0, 10)
	ms, _ := w.mechs()
	for now := uint64(1); now < 200; now++ {
		tick(ms, w, id, now)
	}
	if len(w.shots) == 0 || w.shots[0] != 500 {
		t.Fatalf("shots %v, want the turret first", w.shots)
	}
	w.turret = 0
	tick(ms, w, id, 200)
	tick(ms, w, id, 201)
	if w.animals[id].Target != c {
		t.Fatalf("target %d, want the colonist once the turret is gone", w.animals[id].Target)
	}
}
