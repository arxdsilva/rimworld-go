package combat

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/door"
	"rimworld-go/internal/domain/combat/cover"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// TestNaturalCover (DEF-3): rock, grown trees, cacti, bushes and stone
// chunks beside a target give the cover domain's values; a sapling none;
// a building on the tile wins over what grows there.
func TestNaturalCover(t *testing.T) {
	t.Parallel()
	f := newFake(30, 30)
	s := f.combat().Sight
	f.plants[[2]int{9, 10}] = plant.Tree
	if c := s.Cover(2, 10, 10, 10); c != cover.Permille(cover.Tree) {
		t.Fatalf("tree cover %d", c)
	}
	if id, x, y, _ := s.coverBy(2, 10, 10, 10); id != 0 || x != 9 || y != 10 {
		t.Fatalf("natural cover has no building: %d (%d,%d)", id, x, y)
	}
	f.plants[[2]int{10, 9}] = plant.Cactus
	if c := s.Cover(10, 2, 10, 10); c != cover.Permille(cover.Cactus) {
		t.Fatalf("cactus cover %d", c)
	}
	f.rock[[2]int{11, 10}] = true
	if c := s.Cover(20, 10, 10, 10); c != cover.Permille(cover.Rock) {
		t.Fatalf("rock cover %d", c)
	}
	f.chunks = [][2]int{{10, 11}}
	if c := s.Cover(10, 20, 10, 10); c != cover.Permille(cover.Chunk) {
		t.Fatalf("chunk cover %d", c)
	}
	f.chunks = nil // the chunk tiles are trusted for chunkEvery ticks
	s.Cover(10, 20, 10, 10)
	if f.chunkN != 1 {
		t.Fatalf("chunk scans %d, want 1 within chunkEvery", f.chunkN)
	}
	f.now += chunkEvery
	if c := s.Cover(10, 20, 10, 10); c != 0 || f.chunkN != 2 {
		t.Fatalf("after chunkEvery the chunk is gone: cover %d scans %d", c, f.chunkN)
	}
	f.build(5, construction.Barricade, 9, 10)
	if id, _, _, c := s.coverBy(2, 10, 10, 10); id != 5 || c != cover.Permille(cover.Barricade) {
		t.Fatalf("barricade over the tree: %d %d", id, c)
	}
	f.plants[[2]int{20, 20}] = plant.Corn
	if s.CoverAt(20, 20) != cover.None || s.CoverAt(29, 0) != cover.None {
		t.Fatal("a crop and bare ground give none")
	}
	if (&Sight{g: f}).CoverAt(9, 10) != cover.Barricade || (&Sight{g: f}).CoverAt(10, 9) != cover.None {
		t.Fatal("without a field only buildings and rock count")
	}
}

// TestOpenDoorLetsShotsThrough: a shut door blocks sight; held open, it
// lets shots through once it has opened at its stuff's speed.
func TestOpenDoorLetsShotsThrough(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	c := f.combat()
	f.build(3, construction.Door, 5, 5) // steel: 100 %
	if c.Sight.Line(2, 5, 8, 5) {
		t.Fatal("a shut door blocks")
	}
	c.Doors.SetHold(3, true)
	if c.Sight.Line(2, 5, 8, 5) {
		t.Fatal("still opening")
	}
	f.now += door.BaseOpenTicks
	if !c.Sight.Line(2, 5, 8, 5) || !c.Doors.Held(3) || !c.Doors.HeldOpen(5, 5) {
		t.Fatal("an open door lets a shot through")
	}
	c.Doors.SetHold(3, false)
	f.now += door.StayOpen
	c.Doors.phase(f.now)
	if c.Sight.Line(2, 5, 8, 5) || c.Doors.HeldOpen(5, 5) || len(c.Doors.states) != 0 {
		t.Fatal("released, it closes after StayOpen")
	}
	c.Doors.SetHold(3, true)
	set := make([]uint64, 400/64+1)
	set[(5*20+5)>>6] |= 1 << ((5*20 + 5) & 63)
	set[0] |= 1 // another door, not held
	if got := c.Doors.Shut(set, 20); got[(5*20+5)>>6]&(1<<((5*20+5)&63)) != 0 || got[0]&1 == 0 {
		t.Fatal("Shut drops held doors only")
	}
	if (*Doors)(nil).isOpen(3) {
		t.Fatal("no doors component: shut")
	}
}

// TestDoorsOpenForColonists: a colonist stepping in opens the door and
// waits for it (an autodoor four times faster with power); it shuts
// StayOpen after, once, but never on someone standing in it.
func TestDoorsOpenForColonists(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	d := f.combat().Doors
	f.build(4, construction.Autodoor, 5, 5)
	f.powered[4] = true
	b := f.bldgs[4]
	if got := d.openTicks(4, b); got != door.OpenTicks(400) {
		t.Fatalf("powered steel autodoor opens in %d", got)
	}
	if w := d.Wait(5, 5, f.now); w != door.OpenTicks(400) || d.Wait(0, 0, f.now) != 0 || d.State(4).OpenedAt != f.now {
		t.Fatalf("a shut door makes a walker wait %d", w)
	}
	f.powered[4] = false
	if got := d.openTicks(4, b); got != door.BaseOpenTicks {
		t.Fatalf("unpowered autodoor is a plain door: %d", got)
	}
	f.pawn(1, 5, 5, weapon.Fists) // standing in the doorway
	for i := 0; i < door.StayOpen+10; i++ {
		f.now++
		d.phase(f.now)
	}
	if len(d.states) != 1 {
		t.Fatal("the door shut on someone standing in it")
	}
	delete(f.pos, 1)
	closes := 0
	for i := 0; i < 3*door.StayOpen; i++ {
		f.now++
		had := len(d.states)
		d.phase(f.now)
		if had == 1 && len(d.states) == 0 {
			closes++
		}
	}
	if closes != 1 {
		t.Fatalf("closed %d times", closes)
	}
	b.Forbidden = true
	f.bldgs[4] = b
	if !d.Barred(5, 5) || d.Barred(0, 0) {
		t.Fatal("a forbidden door is barred")
	}
	d.Wait(5, 5, f.now)
	f.RemoveBuilding(4)
	d.phase(f.now)
	if len(d.states) != 0 {
		t.Fatal("a door that is gone forgets its state")
	}
}

// TestIEDFuseAndBurst: a raider on a blast charge lights its fuse; IEDFuse
// ticks later it bursts (wiki radius 3.9) and is re-laid; lighting it again
// keeps the first fuse.
func TestIEDFuseAndBurst(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	c := f.combat()
	f.build(8, construction.IEDHighExplosive, 5, 5)
	f.raider(1, 5, 5, weapon.Club)
	if c.Traps.At(5, 5) != 8 || !c.Traps.Has() {
		t.Fatal("a charge is a trap")
	}
	c.Traps.Underfoot([]entity.EntityID{1})
	c.Traps.Spring(1, 5, 5)
	if len(c.Traps.fuses) != 1 || !c.Traps.Burning(8) || c.Traps.Burning(9) {
		t.Fatalf("fuses %+v", c.Traps.fuses)
	}
	for i := 1; i < IEDFuse; i++ {
		f.now++
		c.Traps.phase(f.now)
	}
	if f.bursts != 0 {
		t.Fatal("burst before the fuse ran out")
	}
	f.now++
	c.Traps.phase(f.now)
	if f.bursts != 1 || f.blasts[0].Kind != explosion.Bomb || f.blasts[0].Radius != 3900 || len(f.frames) != 1 ||
		f.frames[0] != construction.IEDHighExplosive || len(f.hurts) == 0 || c.Traps.At(5, 5) != 0 {
		t.Fatalf("burst: %d %+v frames %v hurts %v", f.bursts, f.blasts, f.frames, f.hurts)
	}
	// a charge gone before its fuse ran out does nothing
	f.build(9, construction.IEDSmoke, 8, 8)
	c.Traps.Spring(1, 8, 8)
	f.RemoveBuilding(9)
	f.now += IEDFuse
	c.Traps.phase(f.now)
	if f.bursts != 1 || len(c.Traps.fuses) != 0 {
		t.Fatal("a removed charge bursts no more")
	}
	for k, want := range map[construction.Kind]explosion.Kind{construction.IEDIncendiary: explosion.Flame,
		construction.IEDEMP: explosion.EMP, construction.IEDSmoke: explosion.Smoke} {
		if _, d := iedBlast(k); d.Kind != want {
			t.Errorf("%v bursts as %v", k, d.Kind)
		}
	}
}

// heavyRig is a powered autocannon (id 51) at (10, 5).
func heavyRig(t *testing.T) (*fakeHost, *Turrets) {
	t.Helper()
	f := newFake(60, 60)
	f.build(51, construction.Autocannon, 10, 5)
	f.wired, f.powered[51], f.defense, f.gen = true, true, []entity.EntityID{51}, 1
	return f, f.combat().Turrets
}

func scan51(now uint64) uint64 {
	for (now+51)%TurretScanEvery != 0 {
		now++
	}
	return now
}

// TestHeavyTurretRangeBurstAndBarrel: the autocannon ignores what is
// inside its minimum range, fires three-round bursts 17 ticks apart then
// cools 210 ticks, wears its barrel a round at a time and falls quiet when
// it is spent.
func TestHeavyTurretRangeBurstAndBarrel(t *testing.T) {
	t.Parallel()
	f, tu := heavyRig(t)
	f.raider(1, 10, 10, weapon.Rifle) // 5 tiles: too close
	f.now = scan51(f.now)
	tu.Phase(f.now)
	if tu.Firing(51) {
		t.Fatal("inside the minimum range")
	}
	f.pos[1] = [2]int{10, 25}
	f.now = scan51(f.now + 1)
	tu.Phase(f.now)
	a := tu.Aim(51)
	if a.Target != 1 {
		t.Fatalf("locks on at 20 tiles: %+v", a)
	}
	var shots []uint64
	for i := 0; i < turretAimTicks+300; i++ {
		f.now++
		before := len(f.traces)
		tu.Phase(f.now)
		if len(f.traces) > before {
			shots = append(shots, f.now)
		}
	}
	if len(shots) < 4 || shots[1]-shots[0] != 17 || shots[2]-shots[1] != 17 || shots[3]-shots[2] != 210 {
		t.Fatalf("burst timing %v", shots)
	}
	if f.traces[0].Weapon != "autocannon" || tu.Barrel(51, construction.Autocannon) != (AutocannonBarrel-len(shots))*1000/AutocannonBarrel {
		t.Fatalf("weapon %q barrel %d", f.traces[0].Weapon, tu.Barrel(51, construction.Autocannon))
	}
	tu.arms[51] = Arm{Fired: AutocannonBarrel}
	f.now++
	tu.Phase(f.now)
	if tu.Firing(51) || !tu.Spent(51, construction.Autocannon) || tu.Barrel(51, construction.Autocannon) != 0 {
		t.Fatal("a spent barrel stops the turret")
	}
	tu.Rearm(51)
	if tu.Barrel(51, construction.Autocannon) != 1000 || BarrelOf(construction.Mortar) != 0 || tu.Barrel(51, construction.Mortar) != 0 {
		t.Fatal("rearmed")
	}
	f.pos[1] = [2]int{10, 9} // inside the minimum range while held: let go
	tu.setAim(51, Aim{Target: 1, SeenAt: f.now})
	f.now++
	tu.Phase(f.now)
	if tu.Firing(51) {
		t.Fatal("a held target inside the minimum range is let go")
	}
}

// TestSlugTurretArmor: a slug turret's hit goes through armor; its log
// names it.
func TestSlugTurretArmor(t *testing.T) {
	t.Parallel()
	f := newFake(60, 60)
	f.build(52, construction.UraniumSlugTurret, 10, 5)
	f.wired, f.powered[52], f.defense, f.gen = true, true, []entity.EntityID{52}, 1
	tu := f.combat().Turrets
	f.raider(1, 10, 30, weapon.Rifle)
	tu.shot(52, construction.UraniumSlugTurret, weapon.SlugTurret, 10, 5, 1, 10, 30)
	if len(f.hurts) != 1 || f.logs[0] != "shot 1 true a slug turret" || f.traces[0].Weapon != "slug turret" {
		t.Fatalf("slug hit: %v %v", f.hurts, f.logs)
	}
	f.droll = []int{999}
	tu.shot(52, construction.UraniumSlugTurret, weapon.SlugTurret, 10, 5, 1, 10, 30)
	if f.logs[1] != "shot 1 false a slug turret" {
		t.Fatalf("slug miss: %v", f.logs)
	}
	if turretName(construction.Autocannon) != "an autocannon" {
		t.Fatal("names")
	}
	b := f.bldgs[52]
	f.RemoveBuilding(52)
	tu.Blast(52, b)
	if f.msgs[0] != "The slug turret blew up!" {
		t.Fatalf("blast message %q", f.msgs[0])
	}
}

// TestTurretHoldDoesNotFlap: holding fire drops the target at once and
// keeps the turret quiet; released, it locks on at its next scan and
// keeps the target while it hovers inside the release distance, without
// flapping between holding and idling.
func TestTurretHoldDoesNotFlap(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	f.raider(1, 10, 12, weapon.Rifle)
	f.now = scanTick(f.now)
	tu.Phase(f.now)
	tu.SetHold(50, true)
	if tu.Firing(50) || !tu.Held(50) {
		t.Fatal("hold fire lets go")
	}
	shots := len(f.traces)
	for i := 0; i < 200; i++ {
		f.now++
		tu.SetHold(50, true) // repeated orders change nothing
		tu.Phase(f.now)
	}
	if tu.Firing(50) || len(f.traces) != shots {
		t.Fatal("a held turret neither looks nor shoots")
	}
	tu.SetHold(50, false)
	flips, was := 0, false
	for i := 0; i < 400; i++ {
		f.now++
		y := 5 + TurretLock - 1 + i%3 // in and out past the lock distance, inside release
		f.pos[1] = [2]int{10, y}
		tu.Phase(f.now)
		if now := tu.Firing(50); now != was {
			flips++
			was = now
		}
	}
	if flips != 1 {
		t.Fatalf("the released turret flipped %d times, want one lock", flips)
	}
}

// mortarRig is a mortar (id 70) at (5, 5), crew spot (5, 6), loaded.
func mortarRig(t *testing.T) (*fakeHost, *Combat) {
	t.Helper()
	f := newFake(80, 80)
	f.build(70, construction.Mortar, 5, 5)
	c := f.combat()
	c.Mortars.load(70, item.MortarShells, 3)
	c.Mortars.load(70, item.SmokeShells, 1)
	return f, c
}

func scan70(now uint64) uint64 {
	for (now+70)%TurretScanEvery != 0 {
		now++
	}
	return now
}

// arrive puts colonist id on its job's tile and sets the job doing.
func arrive(f *fakeHost, id entity.EntityID) {
	j := f.jobs[id]
	f.pos[id] = [2]int{j.TX, j.TY}
	j.State = job.Do
	f.jobs[id] = j
}

// TestMortarForcedTarget: a forced target in range calls the nearest idle
// drafted colonist; manned, the mortar warms up, lobs a shell (trace
// "mortar" aimed at the target), then waits cooldown and warmup.
func TestMortarForcedTarget(t *testing.T) {
	t.Parallel()
	f, c := mortarRig(t)
	m := c.Mortars
	f.pawn(1, 7, 9, weapon.Rifle)
	f.drafted[1] = true
	f.SetCalm(1, 0)
	c.Orders.Apply(command.Command{Kind: command.SetMortarTarget, X: 5, Y: 5, X2: 5, Y2: 12}) // 7 tiles: too near
	f.now = scan70(f.now)
	m.phase(f.now)
	if m.State(70).Crew != 0 {
		t.Fatal("a target inside the minimum range calls nobody")
	}
	c.Orders.Apply(command.Command{Kind: command.SetMortarTarget, X: 5, Y: 5, X2: 45, Y2: 40})
	f.now = scan70(f.now + 1)
	m.phase(f.now)
	st := m.State(70)
	if st.Crew != 1 || f.jobs[1].Kind != job.ManMortar || f.jobs[1].TX != 5 || f.jobs[1].TY != 6 {
		t.Fatalf("crew %+v job %+v", st, f.jobs[1])
	}
	arrive(f, 1)
	start := f.now
	for len(f.traces) == 0 && f.now < start+1000 {
		f.now++
		m.phase(f.now)
	}
	tr := f.traces[0]
	if f.now-start != uint64(weapon.DefOf(weapon.MortarHE).Warmup)+1 || // aiming starts the tick after it arrives tr.Weapon != "mortar" || tr.ToX != 45 || tr.ToY != 40 ||
		c.Shots.Len() != 1 || m.Shells(70) != 3 {
		t.Fatalf("first shell after %d: %+v shells %d", f.now-start, tr, m.Shells(70))
	}
	if m.State(70).ReadyAt != f.now+1680+240 || m.Barrel(70) != (MortarBarrel-1)*1000/MortarBarrel {
		t.Fatalf("after the shot %+v", m.State(70))
	}
	v := c.Orders.View(70, f.bldgs[70], f.now)
	if !v.HasTarget || v.TargetX != 45 || v.TargetY != 40 || v.Shells != 3 || v.ShotAge != 0 {
		t.Fatalf("view %+v", v)
	}
	c.Orders.Apply(command.Command{Kind: command.ClearMortarTarget, X: 5, Y: 5})
	if m.State(70).Forced {
		t.Fatal("cleared")
	}
	// the shell flies and bursts
	for i := 0; i < 200 && c.Shots.Len() > 0; i++ {
		f.now++
		c.Shots.Step()
	}
	if f.bursts != 1 || f.blasts[0].Kind != explosion.Bomb {
		t.Fatalf("burst %d %+v", f.bursts, f.blasts)
	}
}

// TestMortarHostileAndSpot: without a forced target a manned mortar shoots
// at the nearest hostile in range; a drafted colonist on its spot is the
// first called.
func TestMortarHostileAndSpot(t *testing.T) {
	t.Parallel()
	f, c := mortarRig(t)
	m := c.Mortars
	f.pawn(1, 7, 9, weapon.Rifle)
	f.pawn(2, 5, 6, weapon.Rifle)
	f.drafted[1], f.drafted[2] = true, true
	f.raider(9, 5, 45, weapon.Rifle)
	f.now = scan70(f.now)
	m.phase(f.now)
	if m.State(70).Crew != 2 || m.State(70).Target != 9 {
		t.Fatalf("the colonist on the spot crews it: %+v", m.State(70))
	}
	arrive(f, 2)
	for i := 0; i < 241; i++ {
		f.now++
		m.phase(f.now)
	}
	if len(f.traces) != 1 || f.traces[0].ToY != 45 {
		t.Fatalf("shot at the raider: %+v", f.traces)
	}
	delete(f.raiders, 9) // down: no target left
	f.now++
	m.phase(f.now)
	if m.State(70).Target != 0 {
		t.Fatal("a raider down is no target")
	}
	if MissRadius(0) != 10800 || MissRadius(8) != 9000 || MissRadius(20) != 6300 || MissRadius(99) != 6300 {
		t.Fatal("miss radius by Shooting")
	}
	f.aims[2] = [2]int{0, 0}
	if m.crewLevel(2) != 0 || m.crewLevel(1) != 20 {
		t.Fatalf("crew levels %d %d", m.crewLevel(2), m.crewLevel(1))
	}
}

// TestMortarCrewDoesNotFlap: an order names an undrafted colonist; it
// stays at least MortarManMin, and is stood down only after
// MortarIdleRelease ticks with nothing to shoot, once, while a hostile
// blinks in and out of range.
func TestMortarCrewDoesNotFlap(t *testing.T) {
	t.Parallel()
	f, c := mortarRig(t)
	m := c.Mortars
	f.pawn(3, 20, 20, weapon.Fists)
	c.Orders.Apply(command.Command{Kind: command.SetMortarTarget, X: 5, Y: 5, X2: 50, Y2: 50, Pawn: 3})
	f.now = scan70(f.now)
	m.phase(f.now)
	if m.State(70).Crew != 3 || f.interr != 1 {
		t.Fatalf("the named colonist crews it: %+v", m.State(70))
	}
	arrive(f, 3)
	m.ClearTarget(70)
	f.raider(9, 5, 45, weapon.Rifle)
	changes, manned := 0, true
	for i := 0; i < 2*MortarIdleRelease+MortarManMin; i++ {
		f.now++
		if i < 300 { // the raider blinks in and out of range for a while
			f.raiders[9] = i%40 < 5
		} else {
			f.raiders[9] = false
		}
		m.phase(f.now)
		if now := m.State(70).Crew != 0; now != manned {
			changes++
			manned = now
			if f.now < m.State(70).ManSince+MortarManMin {
				t.Fatal("stood down before MortarManMin")
			}
		}
	}
	if changes != 1 || f.jobs[3].State != job.Complete {
		t.Fatalf("crew changed %d times, want one stand-down; job %+v", changes, f.jobs[3])
	}
}

// TestRearmHaul: a spent sentry turret gets an idle colonist, who takes
// the steel from a stack, carries it over and fits the barrel; a mortar
// under half a rack gets shells.
func TestRearmHaul(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	c := tu.saved
	r := c.rearms
	tu.wear(50)
	tu.arms[50] = Arm{Fired: SentryBarrel}
	f.stock[600] = item.Stack{Kind: item.Steel, Qty: 30, X: 2, Y: 2}
	f.pawn(1, 3, 3, weapon.Fists)
	f.pawn(2, 4, 4, weapon.Fists)
	f.drafted[2] = true
	f.now = 250 * 10
	r.phase(f.now)
	j := f.jobs[1]
	if len(r.jobs) != 1 || j.Kind != job.RearmTurret || j.Target != 600 || j.Dest != 50 || j.Count != BarrelSteel(construction.SentryTurret) ||
		r.Hauler(50) != 1 || r.Hauler(51) != 0 {
		t.Fatalf("haul %+v job %+v", r.jobs, j)
	}
	r.phase(f.now + RearmScanEvery) // a busy turret asks no second hauler
	if len(r.jobs) != 1 {
		t.Fatal("asked twice")
	}
	arrive(f, 1)
	r.Do(1, f.jobs[1])
	if j = f.jobs[1]; j.Carry == 0 || j.State != job.Goto || f.stock[600].Qty != 30-9 {
		t.Fatalf("picked up: %+v stock %+v", j, f.stock[600])
	}
	arrive(f, 1)
	for i := 0; i < RearmWork; i++ {
		r.phase(f.now + 2 + uint64(i))
		r.Do(1, f.jobs[1])
	}
	if tu.Barrel(50, construction.SentryTurret) != 1000 || len(f.spent) != 1 || f.jobs[1].State != job.Complete || len(r.jobs) != 0 {
		t.Fatalf("fitted: barrel %d spent %v job %+v", tu.Barrel(50, construction.SentryTurret), f.spent, f.jobs[1])
	}
	// a mortar: shells
	f.build(70, construction.Mortar, 20, 20)
	f.stock[601] = item.Stack{Kind: item.SmokeShells, Qty: 4, X: 22, Y: 22}
	f.jobs[1] = job.Record{}
	r.phase(250 * 20)
	if j = f.jobs[1]; j.Kind != job.RearmTurret || j.Dest != 70 || j.Count != MortarRack || j.DestX != 20 || j.DestY != 21 {
		t.Fatalf("shell haul %+v", j)
	}
	arrive(f, 1)
	r.Do(1, f.jobs[1])
	arrive(f, 1)
	for i := 0; i < RearmWork; i++ {
		r.Do(1, f.jobs[1])
	}
	if c.mortars.Shells(70) != 4 || c.mortars.State(70).Shells[3] != 4 {
		t.Fatalf("loaded %+v", c.mortars.State(70))
	}
	// a spent mortar wants a barrel
	st := c.mortars.State(70)
	st.Fired = MortarBarrel
	c.mortars.set(70, st)
	if barrel, _ := c.mortars.wants(70); !barrel {
		t.Fatal("a spent mortar wants a barrel")
	}
	c.mortars.rearm(70)
	if c.mortars.Barrel(70) != 1000 || c.mortars.load(70, item.Steel, 3) != 0 {
		t.Fatal("rearmed mortar")
	}
}

// TestRearmFailures: a haul whose job was replaced, whose stack vanished
// or whose building is gone ends.
func TestRearmFailures(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	r := tu.saved.rearms
	r.jobs = []Rearm{{Building: 50, Pawn: 1, Kind: item.Steel}}
	f.jobs[1] = job.Record{Kind: job.Haul}
	r.phase(1)
	if len(r.jobs) != 0 {
		t.Fatal("a replaced job ends the haul")
	}
	r.jobs = []Rearm{{Building: 50, Pawn: 1, Kind: item.Steel}}
	f.jobs[1] = job.Record{Kind: job.RearmTurret, Dest: 50, Target: 999, State: job.Do, Count: 9}
	r.Do(1, f.jobs[1])
	if len(r.jobs) != 0 || f.jobs[1].Fail != job.TargetGone {
		t.Fatal("the stack is gone")
	}
	f.stock[600] = item.Stack{Kind: item.Steel, Qty: 30}
	r.jobs = []Rearm{{Building: 50, Pawn: 1, Kind: item.Steel}}
	f.jobs[1] = job.Record{Kind: job.RearmTurret, Dest: 50, Target: 600, State: job.Do, Count: 9}
	f.noPath = true
	r.Do(1, f.jobs[1])
	if len(r.jobs) != 0 || f.jobs[1].Fail != job.Unreachable {
		t.Fatal("unreachable")
	}
	r.jobs = []Rearm{{Building: 50, Pawn: 1, Kind: item.Steel}}
	f.jobs[1] = job.Record{Kind: job.RearmTurret, Dest: 50, State: job.Goto}
	f.RemoveBuilding(50)
	r.phase(1)
	if len(r.jobs) != 0 || f.jobs[1].Fail != job.TargetGone {
		t.Fatal("the turret is gone")
	}
	// a haul nobody tracks fails; Orders.Do routes a rearm and leaves a crew be
	f.jobs[2] = job.Record{Kind: job.RearmTurret, Dest: 50, State: job.Do}
	o := &Orders{h: f, rearms: r}
	o.Do(2, f.jobs[2])
	f.jobs[3] = job.Record{Kind: job.ManMortar, State: job.Do}
	o.Do(3, f.jobs[3])
	if f.jobs[2].Fail != job.TargetGone || f.jobs[3].State != job.Do {
		t.Fatalf("untracked %+v crew %+v", f.jobs[2], f.jobs[3])
	}
}

// TestOrdersApply: each command acts on its kind of building only.
func TestOrdersApply(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	o := &Orders{h: f, turrets: tu, mortars: tu.saved.mortars, doors: tu.saved.doors}
	f.build(3, construction.Door, 2, 2)
	o.Apply(command.Command{Kind: command.SetTurretHold, X: 2, Y: 2, Enabled: true}) // a door: no turret
	o.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: 10, Y: 5, Enabled: true})
	o.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: 0, Y: 0, Enabled: true}) // nothing there
	if tu.Held(3) || tu.saved.doors.Held(50) {
		t.Fatal("wrong kinds ignored")
	}
	o.Apply(command.Command{Kind: command.SetTurretHold, X: 10, Y: 5, Enabled: true})
	o.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: 2, Y: 2, Enabled: true})
	if v := o.View(50, f.bldgs[50], f.now); !v.HoldFire || v.Barrel != 1000 || v.ShotAge != -1 {
		t.Fatalf("turret view %+v", v)
	}
	if v := o.View(3, f.bldgs[3], f.now); !v.HoldOpen || v.ShotAge != 0 || v.Barrel != 0 {
		t.Fatalf("door view %+v", v)
	}
	o.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: 2, Y: 2})
	o.Apply(command.Command{Kind: command.SetTurretHold, X: 10, Y: 5})
	if o.View(3, f.bldgs[3], f.now).HoldOpen || o.View(50, f.bldgs[50], f.now).HoldFire {
		t.Fatal("toggled off")
	}
	o.Apply(command.Command{Kind: command.ClearMortarTarget, X: 2, Y: 2})
	if v := o.View(1, construction.Building{Kind: construction.Wall}, f.now); v != (Facts{}) {
		t.Fatalf("a wall has no facts: %+v", v)
	}
}

// TestHuntingAssign (moved from the game): a marked animal, the downed one
// first; an unarmed hunter fetches a rifle; reserved and postponed prey is
// skipped.
func TestHuntingAssign(t *testing.T) {
	t.Parallel()
	f := newFake(30, 30)
	hn := f.combat().Hunting
	f.pawn(1, 0, 0, weapon.Rifle)
	f.beast(5, 3, 3, animal.State{Species: animal.Deer, Designated: true})
	if hn.Assign(1) {
		t.Fatal("nothing marked in the index")
	}
	f.marked = true
	f.beast(6, 20, 20, animal.State{Species: animal.Deer, Designated: true, Downed: true})
	if !hn.Assign(1) || f.jobs[1].Kind != job.Hunt || f.jobs[1].Target != 6 {
		t.Fatalf("the downed one first: %+v", f.jobs[1])
	}
	f.reserved[6] = true
	if !hn.Assign(1) || f.jobs[1].Target != 5 {
		t.Fatalf("reserved skipped: %+v", f.jobs[1])
	}
	f.postpone[5] = true
	if hn.Assign(1) {
		t.Fatal("all postponed or reserved")
	}
	f.postpone[5] = false
	f.weapons[1] = weapon.Fists
	f.free[item.Rifle] = [3]int{77, 4, 4}
	if !hn.Assign(1) || f.jobs[1].Kind != job.Equip || f.jobs[1].Target != 77 {
		t.Fatalf("fetch a rifle: %+v", f.jobs[1])
	}
	f.free[item.Rifle] = [3]int{}
	if hn.Assign(1) {
		t.Fatal("no rifle to fetch")
	}
}

// TestTilesWalkableNear (moved from the game): the clamped tile itself,
// else the nearest free walkable ring.
func TestTilesWalkableNear(t *testing.T) {
	t.Parallel()
	f := newFake(10, 10)
	tl := f.combat().Tiles
	if x, y, ok := tl.WalkableNear(-5, 50, 2); !ok || x != 0 || y != 9 {
		t.Fatalf("clamped: %d %d %v", x, y, ok)
	}
	f.blocked[[2]int{5, 5}] = true
	f.blocked[[2]int{4, 4 + 1000}] = true // reserved
	if x, y, ok := tl.WalkableNear(5, 5, 1); !ok || x != 5 || y != 4 {
		t.Fatalf("ring: %d %d %v", x, y, ok)
	}
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			f.blocked[[2]int{x, y}] = true
		}
	}
	if _, _, ok := tl.WalkableNear(5, 5, 3); ok {
		t.Fatal("nowhere")
	}
}

// TestDefenceSaveRoundTrip: barrels, hold fire, mortars, doors, fuses and
// hauls ride in the Turrets save block and come back.
func TestDefenceSaveRoundTrip(t *testing.T) {
	t.Parallel()
	f, c := mortarRig(t)
	f.build(50, construction.SentryTurret, 30, 30)
	f.build(3, construction.Door, 2, 2)
	f.build(8, construction.IEDEMP, 9, 9)
	c.Turrets.SetHold(50, true)
	c.Turrets.wear(50)
	c.Turrets.setAim(50, Aim{Burst: 1})
	c.Mortars.SetTarget(70, 50, 50, 0)
	c.Doors.SetHold(3, true)
	c.Traps.light(8, construction.IEDEMP, 9, 9)
	c.Rearms.jobs = []Rearm{{Building: 50, Pawn: 1, Kind: item.Steel}}
	s := c.Turrets.Export()
	if len(s.Arms) != 1 || len(s.Mortars) != 1 || len(s.Doors) != 1 || len(s.Fuses) != 1 || len(s.Rearms) != 1 || s.Aims[0].Burst != 1 {
		t.Fatalf("export %+v", s)
	}
	back := newFake(80, 80).combat()
	back.Turrets.Import(s)
	if !back.Turrets.Held(50) || back.Turrets.arms[50].Fired != 1 || back.Mortars.State(70) != c.Mortars.State(70) ||
		!back.Doors.Held(3) || !back.Traps.Burning(8) || back.Rearms.Hauler(50) != 1 || back.Turrets.Aim(50).Burst != 1 {
		t.Fatal("import")
	}
	back.Turrets.Import(nil)
	if back.Turrets.Held(50) || back.Doors.Held(3) || back.Traps.Burning(8) || back.Turrets.Export() != nil {
		t.Fatal("import nil clears everything")
	}
}
