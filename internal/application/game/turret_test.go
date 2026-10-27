package game

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// PLAY-024 sentry turrets: search cadence, lock hysteresis, power, shots,
// blasts and the raiders' answer, on small open maps.

// turretRig is an open w×h map with a conduit along row 4 from x 2 to
// w-1 and a sentry turret below it at each of xs (row 5). Every producer
// makes what out says (a solar panel at (0, 0) is the producer).
func turretRig(t testing.TB, seed uint64, w, h int, out *int, xs ...int) (*Game, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: w, Height: h})
	for y := 0; y < h; y++ { // flat open ground: walls go where a test puts them
		for x := 0; x < w; x++ {
			g.setTerrain(x, y, terrain.Soil)
		}
	}
	tb := t.(interface {
		Helper()
		Fatalf(string, ...any)
	})
	build := func(k construction.Kind, stuff item.Kind, x, y int) entity.EntityID {
		tb.Helper()
		g.placeFrameStuff(k, stuff, construction.South, x, y)
		if fid := g.layerFrameAt(x, y, construction.DefOf(k).Layer); fid != 0 {
			fr, _ := g.frames.Get(fid)
			g.completeFrame(fid, fr)
		}
		bid := g.layerBldgAt(x, y, construction.DefOf(k).Layer)
		if bid == 0 {
			tb.Fatalf("no %s at %d,%d", k, x, y)
		}
		return bid
	}
	build(construction.SolarPanel, item.Steel, 0, 0)
	for x := 0; x < w; x++ {
		build(construction.PowerConduit, item.Steel, x, 4)
	}
	var ids []entity.EntityID
	for _, x := range xs {
		ids = append(ids, build(construction.SentryTurret, item.Steel, x, 5))
	}
	g.pwc.genHook = func(entity.EntityID) int { return *out }
	return g, ids
}

// turretRaider sends one rifle raider and puts it on (x, y).
func turretRaider(t testing.TB, g *Game, x, y int) entity.EntityID {
	t.Helper()
	g.raidWith([]item.Kind{item.Rifle}, planAssault)
	rid := g.raiderIDs[len(g.raiderIDs)-1]
	g.positions.Set(rid, Position{X: x, Y: y})
	g.invalidateCreatures()
	return rid
}

// holdFire keeps every turret from shooting (its next shot far off), so a
// test of the lock alone keeps its target alive.
func holdFire(g *Game) {
	for id, a := range g.turrets.aims {
		if a.target != 0 {
			a.readyAt = math.MaxUint64 / 2
			g.turrets.aims[id] = a
		}
	}
}

// turretTraces counts the live traces a turret fired.
func turretTraces(g *Game) int {
	n := 0
	for _, tr := range g.traceViews() {
		if tr.FromBuilding {
			n++
		}
	}
	return n
}

func TestTurretFiresOnlyWhenPowered(t *testing.T) {
	t.Parallel()
	out := 0
	g, ts := turretRig(t, 1, 40, 30, &out, 20)
	tid := ts[0]
	rid := turretRaider(t, g, 20, 12)
	for i := 0; i < 200; i++ {
		g.Step(1)
		g.positions.Set(rid, Position{X: 20, Y: 12})
		if turretTraces(g) != 0 || g.turretFiring(tid) {
			t.Fatalf("tick %d: an unpowered turret aimed or fired", g.Tick())
		}
	}
	if v := psView(t, g, tid); v.ShotAge != -1 || v.Powered {
		t.Fatalf("unpowered view %+v", v)
	}
	out = 1000
	fired := false
	for i := 0; i < 200 && !fired; i++ {
		g.Step(1)
		fired = turretTraces(g) > 0
	}
	if !fired {
		t.Fatal("a powered turret never fired at a raider 7 tiles off")
	}
	tr := g.traceViews()[len(g.traceViews())-1]
	if !tr.FromBuilding || tr.Attacker != tid || tr.FromX != 20 || tr.FromY != 5 || tr.Weapon != "sentry gun" {
		t.Fatalf("trace %+v", tr)
	}
	if v := psView(t, g, tid); v.ShotAge < 0 || v.AimDir != 0 || !v.Powered || v.PowerUse != 120 {
		t.Fatalf("firing view %+v", v) // the raider is due south: frame 0, drawing its firing power
	}
	// the power goes: it lets go at the next balance and fires no more
	out = 0
	g.Step(powerInterval)
	shot := g.turrets.aims[tid].shotAt
	for i := 0; i < 200; i++ {
		g.Step(1)
		if g.turretFiring(tid) || g.turrets.aims[tid].shotAt != shot {
			t.Fatalf("tick %d: the turret kept firing without power", g.Tick())
		}
	}
}

func TestTurretNeedsLineOfSight(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 2, 40, 30, &out, 20)
	tid := ts[0]
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 20, 9)
	rid := turretRaider(t, g, 20, 12)
	for i := 0; i < 120; i++ {
		g.Step(1)
		g.positions.Set(rid, Position{X: 20, Y: 12})
		if g.turretFiring(tid) {
			t.Fatal("the turret locked on through a wall")
		}
	}
	g.removeBuilding(wall)
	for i := 0; i < turretScanEvery && !g.turretFiring(tid); i++ {
		g.Step(1)
		g.positions.Set(rid, Position{X: 20, Y: 12})
	}
	if g.turrets.aims[tid].target != rid {
		t.Fatalf("with the wall gone the turret should lock on within a scan: aim %+v", g.turrets.aims[tid])
	}
}

func TestTurretIgnoresVisitorsPrisoners(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 3, 40, 30, &out, 20)
	tid := ts[0]
	visitor := turretRaider(t, g, 18, 9)
	r := g.raiders[visitor]
	r.Leaving = true // a visitor or trader: on the raider list, not hostile
	g.raiders[visitor] = r
	pris := turretRaider(t, g, 22, 9)
	if g.prisoners == nil {
		g.prisoners = map[entity.EntityID]*prisoner.State{}
	}
	g.prisoners[pris] = &prisoner.State{}
	ada, err := g.SpawnNamedPawn(20, 8, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	pet, err := g.spawnAnimal(animal.Chicken, 19, 10)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := g.animals.Get(pet)
	a.Tame = true
	g.animals.Set(pet, a)
	wild, err := g.spawnAnimal(animal.Chicken, 21, 11)
	if err != nil {
		t.Fatal(err)
	}
	keep := map[entity.EntityID]Position{visitor: {X: 18, Y: 9}, pris: {X: 22, Y: 9}, ada: {X: 20, Y: 8}, pet: {X: 19, Y: 10}, wild: {X: 21, Y: 11}}
	place := func() {
		for id, p := range keep {
			g.positions.Set(id, p)
		}
		g.invalidateCreatures()
	}
	for i := 0; i < 120; i++ {
		g.Step(1)
		place()
		if tg := g.turrets.aims[tid].target; tg != 0 {
			t.Fatalf("tick %d: the turret locked on %d (visitor %d, prisoner %d, colonist %d, pet %d, wild %d)",
				g.Tick(), tg, visitor, pris, ada, pet, wild)
		}
	}
	rid := turretRaider(t, g, 20, 14)
	for i := 0; i < turretScanEvery+1 && g.turrets.aims[tid].target == 0; i++ {
		g.Step(1)
		place()
	}
	if tg := g.turrets.aims[tid].target; tg != rid {
		t.Fatalf("the turret should pick the raider %d past the others, got %d", rid, tg)
	}
}

func TestTurretAcquireCadenceStaggered(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 4, 40, 30, &out, 18, 19, 20, 21)
	g.Step(powerInterval)
	rid := turretRaider(t, g, 19, 12)
	got := map[entity.EntityID]uint64{}
	for i := 0; i < 2*turretScanEvery && len(got) < len(ts); i++ {
		now := g.Tick()
		g.Step(1)
		g.positions.Set(rid, Position{X: 19, Y: 12})
		holdFire(g)
		for _, id := range ts {
			if _, seen := got[id]; !seen && g.turrets.aims[id].target != 0 {
				got[id] = uint64(now)
			}
		}
	}
	slots := map[uint64]bool{}
	for _, id := range ts {
		at, ok := got[id]
		if !ok {
			t.Fatalf("turret %d never locked on", id)
		}
		if (at+uint64(id))%turretScanEvery != 0 {
			t.Fatalf("turret %d locked on at tick %d, off its slot", id, at)
		}
		slots[at] = true
	}
	if len(slots) != len(ts) {
		t.Fatalf("turrets with consecutive ids searched on the same tick: %v", got)
	}
}

func TestTurretTargetNoFlap(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 5, 40, 40, &out, 20)
	tid := ts[0]
	g.Step(powerInterval)
	rid := turretRaider(t, g, 20, 14) // 9 tiles
	at := func(dist int, ticks int) (lets int) {
		t.Helper()
		for i := 0; i < ticks; i++ {
			had := g.turrets.aims[tid].target != 0
			g.positions.Set(rid, Position{X: 20, Y: 5 + dist})
			g.invalidateCreatures()
			g.Step(1)
			holdFire(g)
			if had && g.turrets.aims[tid].target == 0 {
				lets++
			}
		}
		return lets
	}
	at(9, turretScanEvery+1)
	if g.turrets.aims[tid].target != rid {
		t.Fatal("no lock on a raider 9 tiles off")
	}
	// wandering across the lock radius, inside the release radius: held
	for k := 0; k < 40; k++ {
		if at(9+2*(k%2), 5) != 0 {
			t.Fatalf("let go at %d tiles inside the release radius", 9+2*(k%2))
		}
	}
	if !g.turretFiring(tid) {
		t.Fatal("the lock should hold across the band")
	}
	// past the release radius: let go, and not taken back from the band
	if at(turretRelease+1, 1) != 1 {
		t.Fatal("the turret should let go past the release radius")
	}
	at(turretLock+1, 3*turretScanEvery)
	if g.turretFiring(tid) {
		t.Fatal("a target in the band should not be picked up again")
	}
	at(turretLock, turretScanEvery)
	if g.turrets.aims[tid].target != rid {
		t.Fatal("back within the lock radius it should be picked up within a scan")
	}
	// out of sight for less than turretLost: held; for turretLost: let go
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 20, 10)
	if at(turretLock, turretLost-1) != 0 {
		t.Fatal("let go before turretLost ticks out of sight")
	}
	g.removeBuilding(wall)
	at(turretLock, 1)                                                        // seen again: the out-of-sight count starts over
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 20, 11) // the refund lies on (20, 10)
	if at(turretLock, turretLost) != 1 {
		t.Fatal("a target out of sight for turretLost ticks should be let go")
	}
}

func TestRaiderTurretSwitchNoFlap(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 6, 50, 40, &out, 25)
	tid := ts[0]
	g.Step(powerInterval)
	// the rule: within sight and at least switchMargin closer than the colonist
	for _, c := range []struct {
		raider    Position
		colonist  int
		wantTower bool
	}{
		{Position{X: 25, Y: 13}, 8 + switchMargin, true},      // 8 off the turret
		{Position{X: 25, Y: 13}, 8 + switchMargin - 1, false}, // not closer by the margin
		{Position{X: 25, Y: 5 + sightRange + 1}, 40, false},   // out of sight
		{Position{X: 25, Y: 5 + sightRange}, 40, true},
	} {
		id, _, ok := g.raiderTurret(c.raider, c.colonist)
		if ok != c.wantTower || ok && id != tid {
			t.Fatalf("raider at %+v, colonist %d off: turret %v (%d)", c.raider, c.colonist, ok, id)
		}
	}
	// a raider just inside or just outside the switch rule: it picks one
	// target and keeps to it while the distances change as it closes in
	// (the attack job holds; turret fire held so it lives)
	for _, c := range []struct {
		y       int
		turret0 bool // the first target is the turret
	}{{17, false}, {14, true}} {
		g, ts := turretRig(t, 6, 50, 40, &out, 25)
		tid := ts[0]
		g.Step(powerInterval)
		ada, _ := g.SpawnNamedPawn(25, 30, "Ada")
		g.draft[ada] = true // drafted: holds still instead of running
		rid := turretRaider(t, g, 25, c.y)
		var seq []entity.EntityID
		for i := 0; i < 600; i++ {
			g.Step(1)
			fed(g, ada)
			holdFire(g)
			if !g.standingRaider(rid) {
				break
			}
			if j, _ := g.jobs.Get(rid); j.Kind == job.Attack && (len(seq) == 0 || seq[len(seq)-1] != j.Target) {
				seq = append(seq, j.Target)
			}
		}
		if len(seq) == 0 || (seq[0] == tid) != c.turret0 {
			t.Fatalf("raider from row %d: attack targets %v (turret %d, Ada %d)", c.y, seq, tid, ada)
		}
		for i := 2; i < len(seq); i++ {
			if seq[i] == seq[i-2] {
				t.Fatalf("raider from row %d flapped between targets: %v", c.y, seq)
			}
		}
	}
}

func TestRaiderTargetsTurret(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 7, 50, 50, &out, 25)
	tid := ts[0]
	g.Step(powerInterval)
	ada, _ := g.SpawnNamedPawn(25, 45, "Ada")
	g.draft[ada] = true
	rid := turretRaider(t, g, 31, 11) // 6 off the turret, 34 off Ada
	attacked := false
	for i := 0; i < 600; i++ {
		g.Step(1)
		fed(g, ada)
		holdFire(g)
		if j, _ := g.jobs.Get(rid); j.Kind == job.Attack && j.Target == tid {
			attacked = true
		}
		if b, ok := g.bldgs.Get(tid); !ok || b.Damage > 0 {
			break
		}
	}
	if !attacked {
		j, _ := g.jobs.Get(rid)
		t.Fatalf("the raider should turn on the nearby powered turret (job %+v)", j)
	}
	if b, ok := g.bldgs.Get(tid); ok && b.Damage == 0 {
		t.Fatal("the raider never hurt the turret")
	}
	hitBldg := false
	for _, tr := range g.traceViews() {
		hitBldg = hitBldg || tr.Building && tr.Attacker == rid
	}
	if !hitBldg {
		t.Fatal("no Building trace from the raider")
	}
}

func TestTurretDestroyedExplodes(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 8, 40, 30, &out, 20, 22)
	tid, other := ts[0], ts[1]
	g.Step(powerInterval)
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 18, 7)
	far := buildNow(t, g, construction.Wall, item.Wood, construction.South, 26, 8)
	near := turretRaider(t, g, 20, 6)
	edge := turretRaider(t, g, 18, 6)
	away := turretRaider(t, g, 20, 9)
	ob, _ := g.bldgs.Get(other)
	ob.Damage = construction.MaxHP(ob.Kind, ob.Stuff) - turretBlastBuilding // the blast finishes it: a chain
	g.bldgs.Set(other, ob)

	g.damageBuilding(tid, 10_000)
	if _, ok := g.bldgs.Get(tid); ok {
		t.Fatal("the turret should be gone")
	}
	if _, ok := g.bldgs.Get(other); ok {
		t.Fatal("the neighbouring turret should go up with it")
	}
	if g.layerFrameAt(20, 5, construction.LayerObject) == 0 {
		t.Fatal("a destroyed turret leaves a rebuild blueprint")
	}
	for _, id := range []entity.EntityID{near, edge} {
		if v, _ := g.vital.Get(id); v.Injury < turretBlastCreature {
			t.Fatalf("raider %d within the blast: injury %d", id, v.Injury)
		}
	}
	if v, _ := g.vital.Get(away); v.Injury != 0 {
		t.Fatalf("a raider 4 tiles off was hurt: %d", v.Injury)
	}
	if b, _ := g.bldgs.Get(wall); b.Damage != turretBlastBuilding {
		t.Fatalf("wall within the blast: damage %d", b.Damage)
	}
	if b, _ := g.bldgs.Get(far); b.Damage != 0 {
		t.Fatalf("wall out of the blast: damage %d", b.Damage)
	}
	bursts := 0
	for _, b := range g.World().Bursts {
		if b.Turret && b.Species == "" {
			bursts++
		}
	}
	if bursts != 2 {
		t.Fatalf("turret bursts %d, want 2: %+v", bursts, g.World().Bursts)
	}
	if _, ok := g.turrets.aims[tid]; ok {
		t.Fatal("a destroyed turret keeps no aim")
	}
}

func TestTurretShedLast(t *testing.T) {
	t.Parallel()
	out := 45
	g, ts := turretRig(t, 9, 40, 30, &out) // lamps first, the turret newest
	l1 := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 10, 5)
	l2 := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 12, 5)
	tid := buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 20, 5)
	_ = ts
	g.Step(powerInterval + 1)
	if !g.powered(tid) || g.powered(l1) || g.powered(l2) {
		t.Fatalf("45 W for two lamps and an idle turret: turret %v, lamps %v %v", g.powered(tid), g.powered(l1), g.powered(l2))
	}
	if v := psView(t, g, tid); v.PowerUse != 40 {
		t.Fatalf("idle turret draw %d", v.PowerUse)
	}
}

func TestTurretSaveRoundTrip(t *testing.T) {
	t.Parallel()
	out := 0
	bare, ts := turretRig(t, 10, 40, 30, &out, 20)
	if bare.exportTurrets() != nil {
		t.Fatal("a turret that never aimed saves nothing")
	}
	_ = ts
	g := New(Config{Seed: 10, Width: 40, Height: 30})
	gen := psFuelled(t, g, 2, 2)
	_ = gen
	psConduits(t, g, 4, 2, 30)
	tid := buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 20, 5)
	turretRaider(t, g, 20, 12)
	turretRaider(t, g, 24, 13)
	for i := 0; i < 400 && g.turrets.aims[tid].shotAt == 0; i++ {
		g.Step(1)
	}
	if g.turrets.aims[tid].shotAt == 0 {
		t.Fatal("the turret never fired")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "turrets"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if s := g.exportTurrets(); s == nil || s.Version != turretSaveVersion || !reflect.DeepEqual(s, h.exportTurrets()) {
		t.Fatalf("turret state %+v vs loaded %+v", s, h.exportTurrets())
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("hash differs after load")
	}
	for i := 0; i < 3; i++ {
		g.Step(150)
		h.Step(150)
		sameState(t, "turrets", g, h)
		if g.StateHash() != h.StateHash() {
			t.Fatalf("hash differs %d ticks after load", 150*(i+1))
		}
	}
}

// turretRun is a raid of three on a colonist behind two turrets, n ticks:
// the end hash and the shots fired.
func turretRun(t *testing.T, seed uint64, n int) (uint64, int) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 40, Height: 30})
	psFuelled(t, g, 2, 2)
	psConduits(t, g, 4, 2, 30)
	buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 15, 5)
	buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 25, 5)
	ada, _ := g.SpawnNamedPawn(20, 2, "Ada")
	g.equip[ada] = item.Rifle
	g.Step(powerInterval)
	g.Raid(3)
	shots := 0
	for i := 0; i < n; i++ {
		g.Step(1)
		fed(g, ada)
		for _, a := range g.turrets.aims {
			if a.shotAt == uint64(g.Tick())-1 {
				shots++
			}
		}
	}
	return g.StateHash(), shots
}

func TestTurretSameSeed(t *testing.T) {
	t.Parallel()
	h1, s1 := turretRun(t, 12, 1500)
	h2, s2 := turretRun(t, 12, 1500)
	if h1 != h2 || s1 != s2 {
		t.Fatalf("same seed, different runs: hash %x/%x, shots %d/%d", h1, h2, s1, s2)
	}
	if s1 == 0 {
		t.Fatal("the turrets never fired in the raid")
	}
}

func TestAimFrame(t *testing.T) {
	t.Parallel()
	p := Position{X: 10, Y: 10}
	for _, c := range []struct {
		dx, dy int
		want   int8
	}{
		{0, 5, 0}, {-5, 5, 2}, {-5, 0, 4}, {-5, -5, 6}, {0, -5, 8}, {5, -5, 10}, {5, 0, 12}, {5, 5, 14},
		{-1, 5, 1}, {-2, 5, 1}, {1, 5, 15}, {0, 0, 7},
	} {
		if got := aimFrame(p, Position{X: 10 + c.dx, Y: 10 + c.dy}, 7); got != c.want {
			t.Errorf("aim at (%d, %d): frame %d, want %d", c.dx, c.dy, got, c.want)
		}
	}
}

func BenchmarkTurretsRaid(b *testing.B) {
	SetIndexSelfCheck(false) // time the indexed lookups, not the self-check scans
	defer SetIndexSelfCheck(true)
	out := 100_000
	xs := make([]int, 20)
	for i := range xs {
		xs[i] = 10 + 5*i
	}
	fresh := func() *Game {
		g, _ := turretRig(b, 13, 120, 120, &out, xs...)
		g.Step(powerInterval)
		for i := 0; i < 30; i++ {
			rid := turretRaider(b, g, 10+3*i, 8+(i%4)*2) // within the guns
			_ = rid
		}
		return g
	}
	g := fresh()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i > 0 && i%600 == 0 {
			b.StopTimer()
			g = fresh()
			b.StartTimer()
		}
		g.Step(1)
	}
}
