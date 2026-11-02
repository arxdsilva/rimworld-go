package combat

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeFoam is the map's fires for the AX-5 tests.
type fakeFoam struct {
	fires  map[[2]int]int
	spills map[[2]int]int
	doused int
}

func (f *fakeFoam) FiresNear(x, y, r int) bool {
	for c := range f.fires {
		if weapon.Distance(x, y, c[0], c[1]) <= r {
			return true
		}
	}
	return false
}
func (f *fakeFoam) Burning(x, y int) int { return f.fires[[2]int{x, y}] }
func (f *fakeFoam) Douse(x, y int) {
	if _, ok := f.fires[[2]int{x, y}]; ok {
		delete(f.fires, [2]int{x, y})
		f.doused++
	}
}
func (f *fakeFoam) Spill(x, y, n int) { f.spills[[2]int{x, y}] += n }

// foamRig is a 40x30 map with fires and a combat that sees them.
func foamRig() (*fakeHost, *fakeFoam, *Combat) {
	f := newFake(40, 30)
	fm := &fakeFoam{fires: map[[2]int]int{}, spills: map[[2]int]int{}}
	return f, fm, New(f, Roles{Power: f, Flight: f, Hunts: f, Breach: f, Field: f, Supply: f, Draft: f, Foam: fm})
}

func TestFoamBurstDousesAndStains(t *testing.T) {
	t.Parallel()
	f, fm, c := foamRig()
	fm.fires[[2]int{10, 10}], fm.fires[[2]int{12, 10}], fm.fires[[2]int{30, 10}] = 30, 40, 50
	f.raider(1, 11, 10, weapon.Club)
	f.build(7, construction.Wall, 15, 10) // shelters the cell behind it
	c.Blasts.Explode(0, weapon.MortarFoam, 10, 10, foamDef(5000))
	if fm.doused != 2 || fm.fires[[2]int{30, 10}] == 0 {
		t.Fatalf("doused %d left %v", fm.doused, fm.fires)
	}
	if fm.spills[[2]int{10, 10}] != FoamSpill || fm.spills[[2]int{14, 10}] == 0 || fm.spills[[2]int{16, 10}] != 0 {
		t.Fatalf("spills %v", fm.spills)
	}
	if len(f.hurts) != 0 || f.damage[7] != 0 || f.bursts != 1 || f.blasts[0].Kind != explosion.Foam {
		t.Fatalf("foam harmed: hurts %v damage %v bursts %d", f.hurts, f.damage, f.bursts)
	}
}

func TestFoamChargeAndWarheadCharge(t *testing.T) {
	t.Parallel()
	f, fm, c := foamRig()
	fm.fires[[2]int{9, 5}] = 20
	f.build(8, construction.IEDFoam, 5, 5)
	f.build(9, construction.IEDWarhead, 30, 20)
	f.raider(1, 5, 5, weapon.Club)
	if c.Traps.At(5, 5) != 8 || c.Traps.At(30, 20) != 9 || !isIED(construction.IEDWarhead) || isIED(construction.FoamPopper) {
		t.Fatal("the AX-5 charges are traps")
	}
	c.Traps.Spring(1, 5, 5)
	for range IEDFuse {
		f.now++
		c.Traps.phase(f.now)
	}
	if fm.doused != 1 || f.blasts[0].Radius != FoamChargeRadius || f.frames[0] != construction.IEDFoam || len(f.hurts) != 0 {
		t.Fatalf("foam charge: doused %d blasts %+v frames %v", fm.doused, f.blasts, f.frames)
	}
	c.Traps.Struck(30, 20) // a bullet sets the warhead charge off too
	for range IEDFuse {
		f.now++
		c.Traps.phase(f.now)
	}
	if b := f.blasts[len(f.blasts)-1]; b.Kind != explosion.Bomb || b.Radius != 14900 || b.Damage != 550 || b.Rim != 200 {
		t.Fatalf("warhead blast %+v", b)
	}
	if want := 110; weapon.DefOf(weapon.Warhead).Blast.At(14900) != want {
		t.Fatalf("warhead rim damage %d", weapon.DefOf(weapon.Warhead).Blast.At(14900))
	}
	if f.frames[len(f.frames)-1] != construction.IEDWarhead {
		t.Fatal("the warhead charge is re-laid as a blueprint")
	}
}

// TestPopperLightsOnceThenRelays: a fire within 3.9 tiles lights an armed
// popper on its scan tick; while its fuse burns it is never lit again
// (no flapping), it bursts PopperFuse later, puts the fire out, and a
// blueprint takes its place. A fire further off leaves it be.
func TestPopperLightsOnceThenRelays(t *testing.T) {
	t.Parallel()
	f, fm, c := foamRig()
	f.build(40, construction.FoamPopper, 10, 10)
	fm.fires[[2]int{15, 10}] = 20 // 5 tiles off: too far
	step := func(n int) {
		for range n {
			f.now++
			c.Turrets.Phase(f.now)
		}
	}
	step(PopperScanEvery * 2)
	if c.Traps.Burning(40) {
		t.Fatal("lit by a far fire")
	}
	if v := c.Orders.View(40, f.bldgs[40], f.now); !v.Armed {
		t.Fatal("an unlit popper is armed")
	}
	fm.fires[[2]int{13, 10}] = 20
	step(PopperScanEvery)
	if !c.Traps.Burning(40) || len(c.Traps.fuses) != 1 {
		t.Fatalf("fuses %+v", c.Traps.fuses)
	}
	if v := c.Orders.View(40, f.bldgs[40], f.now); v.Armed {
		t.Fatal("a sparking popper is not armed")
	}
	step(PopperFuse - PopperScanEvery)
	if len(c.Traps.fuses) != 1 {
		t.Fatalf("lit twice: %+v", c.Traps.fuses)
	}
	step(PopperScanEvery)
	if fm.doused != 2 || len(fm.fires) != 0 || f.frames[0] != construction.FoamPopper || f.blasts[0].Radius != PopperRadius {
		t.Fatalf("burst: doused %d fires %v frames %v", fm.doused, fm.fires, f.frames)
	}
}

// foamTurretRig is a powered foam turret 50 at (10, 5).
func foamTurretRig() (*fakeHost, *fakeFoam, *Combat) {
	f, fm, c := foamRig()
	f.build(50, construction.FoamTurret, 10, 5)
	f.wired, f.powered[50], f.defense, f.gen = true, true, []entity.EntityID{50}, 1
	return f, fm, c
}

// TestFoamTurretSpraysThenCools: a fire in sight starts a spray of 41
// blobs (one charge), the blobs put it out, and the turret then waits
// FoamCooldown before it looks again: a fire lit meanwhile is left
// alone until the wait is over (no flapping).
func TestFoamTurretSpraysThenCools(t *testing.T) {
	t.Parallel()
	f, fm, c := foamTurretRig()
	tu := c.Turrets
	fm.fires[[2]int{10, 12}], fm.fires[[2]int{11, 12}] = 40, 40
	fm.fires[[2]int{10, 28}] = 40 // past 19.9 tiles
	if v := c.Orders.View(50, f.bldgs[50], f.now); v.Charges != FoamTurretCharges || v.ChargeCap != FoamTurretCharges || v.Barrel != 0 {
		t.Fatalf("a new foam turret is full: %+v", v)
	}
	shots := 0
	for range 200 {
		f.now++
		tu.Phase(f.now)
		c.Shots.Step()
	}
	for _, tr := range f.traces {
		if tr.Weapon == "foam turret" && tr.Stage == projectile.Fired {
			shots++
		}
	}
	if fm.fires[[2]int{10, 12}] != 0 || fm.fires[[2]int{11, 12}] != 0 || fm.fires[[2]int{10, 28}] == 0 {
		t.Fatalf("fires left %v", fm.fires)
	}
	if tu.Charges(50, construction.FoamTurret) != FoamTurretCharges-1 || shots == 0 || shots > 41 {
		t.Fatalf("charges %d shots %d", tu.Charges(50, construction.FoamTurret), shots)
	}
	cool := tu.arms[50].Cool
	if cool <= f.now {
		t.Fatal("no wait after the spray")
	}
	fm.fires[[2]int{12, 12}] = 40
	for f.now < cool-1 {
		f.now++
		tu.Phase(f.now)
		c.Shots.Step()
	}
	if fm.fires[[2]int{12, 12}] == 0 || tu.Charges(50, construction.FoamTurret) != FoamTurretCharges-1 {
		t.Fatal("sprayed while cooling down")
	}
	for range 3 * TurretScanEvery {
		f.now++
		tu.Phase(f.now)
		c.Shots.Step()
	}
	if fm.fires[[2]int{12, 12}] != 0 {
		t.Fatal("no spray after the wait")
	}
}

// TestFoamTurretRefuel: at a quarter of its charges a foam turret asks a
// colonist for chemfuel (what it is missing, 7.2 a charge rounded up, /9),
// and a spent one stays quiet.
func TestFoamTurretRefuel(t *testing.T) {
	t.Parallel()
	f, fm, c := foamTurretRig()
	tu, r := c.Turrets, c.Rearms
	tu.arms = map[entity.EntityID]Arm{50: {Fired: FoamTurretCharges}}
	fm.fires[[2]int{10, 8}] = 40
	if !tu.Spent(50, construction.FoamTurret) || !tu.wantsRearm(50, construction.FoamTurret) {
		t.Fatal("an empty foam turret is spent and asks")
	}
	for range 2 * TurretScanEvery {
		f.now++
		tu.Phase(f.now)
	}
	if fm.fires[[2]int{10, 8}] == 0 || tu.aims[50].Burst != 0 {
		t.Fatal("a spent foam turret sprayed")
	}
	f.stock[600] = item.Stack{Kind: item.Chemfuel, Qty: 30, X: 2, Y: 2}
	f.stock[601] = item.Stack{Kind: item.Steel, Qty: 30, X: 3, Y: 2}
	f.pawn(1, 3, 3, weapon.Fists)
	f.now = RearmScanEvery * 10
	r.phase(f.now)
	if j := f.jobs[1]; j.Kind != job.RearmTurret || j.Target != 600 || j.Count != 8 {
		t.Fatalf("haul %+v", j)
	}
	arrive(f, 1)
	r.Do(1, f.jobs[1])
	arrive(f, 1)
	for i := 0; i < RearmWork; i++ {
		r.Do(1, f.jobs[1])
	}
	if tu.Charges(50, construction.FoamTurret) != FoamTurretCharges || tu.wantsRearm(50, construction.FoamTurret) {
		t.Fatalf("refuelled: %d", tu.Charges(50, construction.FoamTurret))
	}
	tu.arms[50] = Arm{Fired: 3}
	if tu.wantsRearm(50, construction.FoamTurret) || tu.chemfuel(50, construction.FoamTurret) != 3 {
		t.Fatalf("7 charges left asks nothing; 3 charges cost %d", tu.chemfuel(50, construction.FoamTurret))
	}
}

// TestRocketBatteryVolley: a loaded battery fires a volley of 12 rockets
// at a raider, then is spent until a full reload of chemfuel, and stays
// quiet for its recharge even once reloaded.
func TestRocketBatteryVolley(t *testing.T) {
	t.Parallel()
	f, _, c := foamRig()
	f.build(50, construction.RocketBattery, 10, 5)
	f.wired, f.powered[50], f.defense, f.gen = true, true, []entity.EntityID{50}, 1
	tu := c.Turrets
	f.raider(1, 10, 25, weapon.Club)
	for range 400 {
		f.now++
		tu.Phase(f.now)
	}
	rockets := 0
	for _, tr := range f.traces {
		if tr.Weapon == "rocket battery" && tr.Stage == projectile.Fired {
			rockets++
		}
	}
	if rockets != RocketVolley || !tu.Spent(50, construction.RocketBattery) || !tu.wantsRearm(50, construction.RocketBattery) {
		t.Fatalf("rockets %d spent %v", rockets, tu.Spent(50, construction.RocketBattery))
	}
	if tu.chemfuel(50, construction.RocketBattery) != 20 {
		t.Fatal("a reload is 180 chemfuel, /9")
	}
	tu.Rearm(50)
	if tu.Spent(50, construction.RocketBattery) || tu.arms[50].Cool < f.now+weapon.RocketRecharge-400 {
		t.Fatalf("reloaded %+v", tu.arms[50])
	}
	for range 400 {
		f.now++
		tu.Phase(f.now)
	}
	n := 0
	for _, tr := range f.traces {
		if tr.Weapon == "rocket battery" && tr.Stage == projectile.Fired {
			n++
		}
	}
	if n != RocketVolley {
		t.Fatal("fired again inside its recharge")
	}
}

func TestSmashedFoamDefencesBurst(t *testing.T) {
	t.Parallel()
	f, fm, c := foamRig()
	fm.fires[[2]int{12, 12}] = 30
	c.Turrets.Blast(40, construction.Building{Kind: construction.FoamPopper, X: 10, Y: 10})
	if fm.doused != 1 || f.blasts[0].Kind != explosion.Foam || f.blasts[0].Radius != PopperRadius {
		t.Fatalf("popper %+v", f.blasts)
	}
	c.Turrets.Blast(41, construction.Building{Kind: construction.FoamTurret, X: 10, Y: 10})
	if f.blasts[1].Radius != FoamTurretBurstRadius {
		t.Fatal("foam turret")
	}
	c.Turrets.Blast(42, construction.Building{Kind: construction.RocketBattery, X: 10, Y: 10})
	if b := f.blasts[2]; b.Kind != explosion.Bomb || b.Radius != RocketBlastRadius {
		t.Fatalf("rocket battery %+v", b)
	}
	for _, k := range []construction.Kind{construction.SentryTurret, construction.FoamPopper, construction.FoamTurret, construction.RocketBattery} {
		if !k.BurstsWhenDestroyed() {
			t.Fatalf("%v bursts when smashed", k)
		}
	}
}

// TestMortarFoamShells: foam shells load into a mortar after the others
// and a cooking-off warhead stack bursts as the warhead.
func TestMortarFoamShells(t *testing.T) {
	t.Parallel()
	f, _, c := foamRig()
	f.build(60, construction.Mortar, 10, 10)
	if n := c.Mortars.load(60, item.FoamShells, 4); n != 4 || c.Mortars.Shells(60) != 4 || c.Mortars.State(60).Foam != 4 {
		t.Fatalf("loaded %d", n)
	}
	if wk, ok := weapon.ForShell(item.FoamShells); !ok || wk != weapon.MortarFoam || !item.FoamShells.IsShell() {
		t.Fatal("foam shells are mortar shells")
	}
	if wk, ok := cookOff(item.AntimatterWarhead); !ok || wk != weapon.Warhead {
		t.Fatal("a warhead cooks off")
	}
	// the antimatter warhead is mortar ammo too (wiki: Antigrain warhead),
	// fired after the ordinary shells and before foam, with the charge's blast
	if wk, ok := weapon.ForShell(item.AntimatterWarhead); !ok || wk != weapon.Warhead || !item.AntimatterWarhead.IsShell() {
		t.Fatal("the warhead is a mortar shell")
	}
	if n := c.Mortars.load(60, item.AntimatterWarhead, 1); n != 1 || c.Mortars.Shells(60) != 5 {
		t.Fatalf("loaded %d warheads", n)
	}
	if d := weapon.DefOf(weapon.Warhead); d.Range != weapon.MortarRange || d.Blast.Radius != 14900 || d.Blast.Damage != 550 {
		t.Fatalf("warhead shell %+v", d)
	}
	st := c.Mortars.State(60)
	f.pawn(1, 10, 11, weapon.Fists)
	st.Crew = 1
	c.Mortars.fire(f.bldgs[60], &st, 10, 60, f.now)
	if st.Warhead != 0 || st.Foam != 4 {
		t.Fatalf("fired: warheads %d foam %d, want the warhead first", st.Warhead, st.Foam)
	}
}
