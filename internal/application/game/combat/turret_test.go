package combat

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

func TestAimFrame(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		dx, dy int
		want   int8
	}{
		{0, 5, 0}, {-5, 5, 2}, {-5, 0, 4}, {-5, -5, 6}, {0, -5, 8}, {5, -5, 10}, {5, 0, 12}, {5, 5, 14},
		{-1, 5, 1}, {-2, 5, 1}, {1, 5, 15}, {0, 0, 7},
	} {
		if got := aimFrame(10, 10, 10+c.dx, 10+c.dy, 7); got != c.want {
			t.Errorf("aim at (%d, %d): frame %d, want %d", c.dx, c.dy, got, c.want)
		}
	}
}

// TestTurretShotAgeSaturates: a turret's shot age counts up from its last
// shot and stops at ShotAgeMax (still past the renderer's 90-tick engaged
// window), so a quiet turret's view stops changing.
func TestTurretShotAgeSaturates(t *testing.T) {
	t.Parallel()
	const id entity.EntityID = 7
	tu := &Turrets{}
	if _, got := tu.View(id, 500); got != -1 {
		t.Fatalf("never fired: shot age %d, want -1", got)
	}
	tu.setAim(id, Aim{ShotAt: 1000, Dir: 5})
	for _, c := range []struct {
		now  uint64
		want int
	}{{1000, 0}, {1030, 30}, {1000 + ShotAgeMax, ShotAgeMax}, {1000 + ShotAgeMax + 1, ShotAgeMax}, {900000, ShotAgeMax}} {
		if dir, got := tu.View(id, c.now); got != c.want || dir != 5 {
			t.Errorf("tick %d: shot age %d dir %d, want %d dir 5", c.now, got, dir, c.want)
		}
	}
	if ShotAgeMax < 90 {
		t.Fatal("the cap cuts into the renderer's engaged window (turret_draw.go turretEngagedTicks)")
	}
}

func TestTurretSaveRoundTrip(t *testing.T) {
	t.Parallel()
	tu := &Turrets{}
	if tu.Export() != nil {
		t.Fatal("no aim saves nothing")
	}
	tu.setAim(9, Aim{Target: 4, Dir: 3, ShotAt: 10, ReadyAt: 20, SeenAt: 15})
	tu.setAim(2, Aim{Target: 5})
	s := tu.Export()
	if s.Version != TurretSaveVersion || len(s.Aims) != 2 || s.Aims[0].ID != 2 || s.Aims[1].ID != 9 {
		t.Fatalf("export %+v", s)
	}
	back := &Turrets{}
	back.Import(s)
	if back.Aim(9) != tu.Aim(9) || back.Aim(2) != tu.Aim(2) || !back.Firing(9) || back.Firing(1) {
		t.Fatalf("import %+v", back.aims)
	}
	back.Import(nil)
	if back.Firing(9) {
		t.Fatal("import nil clears")
	}
}

func TestNearestTiesToLowerID(t *testing.T) {
	t.Parallel()
	seen := []Seen{{ID: 9, X: 3, Y: 0}, {ID: 4, X: 0, Y: 3}, {ID: 2, X: 5, Y: 5}, {ID: 1, X: 1, Y: 1}}
	s, ok := nearest(seen, 0, 0, func(s Seen) bool { return s.ID != 1 })
	if !ok || s.ID != 4 {
		t.Fatalf("nearest %+v %v, want id 4 (distance 3, lower id than 9)", s, ok)
	}
	if _, ok := nearest(seen, 0, 0, func(Seen) bool { return false }); ok {
		t.Fatal("nothing kept: none")
	}
}

// turretRig is a powered sentry turret (id 50) at (10, 5) on a fake map.
func turretRig(t *testing.T) (*fakeHost, *Turrets) {
	t.Helper()
	f := newFake(40, 30)
	f.build(50, construction.SentryTurret, 10, 5)
	f.wired, f.powered[50], f.defense, f.gen = true, true, []entity.EntityID{50}, 1
	return f, f.combat().Turrets
}

// scanTick is the first tick at or after now on which turret 50 scans.
func scanTick(now uint64) uint64 {
	for (now+50)%TurretScanEvery != 0 {
		now++
	}
	return now
}

func TestTurretLocksAimsAndFires(t *testing.T) {
	t.Parallel()
	f := newFake(40, 30)
	f.build(50, construction.SentryTurret, 10, 5)
	f.wired, f.powered[50], f.defense, f.gen = true, true, []entity.EntityID{50}, 1
	c := f.combat()
	tu := c.Turrets
	f.raider(1, 10, 12, weapon.Rifle)
	f.now = scanTick(f.now) + 1
	tu.Phase(f.now)
	if tu.Firing(50) {
		t.Fatal("off its scan tick an idle turret looks for nobody")
	}
	f.now = scanTick(f.now)
	tu.Phase(f.now)
	a := tu.Aim(50)
	if a.Target != 1 || a.ReadyAt != f.now+turretAimTicks || a.Dir != 0 || a.ShotAt != 0 {
		t.Fatalf("locked on: %+v", a)
	}
	for f.now < a.ReadyAt {
		f.now++
		tu.Phase(f.now)
	}
	if a = tu.Aim(50); a.ShotAt != f.now || len(f.traces) != 1 || !f.traces[0].FromBuilding || c.Shots.Len() != 1 {
		t.Fatalf("fired once aimed: %+v", a)
	}
	fly(c) // CBT-3: the round flies
	if len(f.hurts) != 1 || f.logs[0] != "shot 1 true "+TurretLogName || a.ReadyAt != f.now+8 {
		t.Fatalf("hit: %v %v %+v", f.hurts, f.logs, a)
	}
	f.droll = []int{999}
	f.rolls = []int{999, 999, 999, 999, 999, 999, 999, 999, 999}
	f.now = a.ReadyAt
	tu.Phase(f.now) // the burst's second round
	fly(c)
	if f.logs[1] != "shot 1 false "+TurretLogName || len(f.hurts) != 1 {
		t.Fatalf("a miss: %v", f.logs)
	}
	if dir, age := tu.View(50, f.now+5); dir != 0 || age != 5 {
		t.Fatalf("view %d %d", dir, age)
	}
}

func TestTurretLetsGo(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	f.raider(1, 10, 12, weapon.Rifle)
	f.now = scanTick(f.now)
	tu.Phase(f.now)
	// out to TurretRelease: still held; past it: let go
	f.pos[1] = [2]int{10, 5 + TurretRelease}
	f.now++
	tu.Phase(f.now)
	if !tu.Firing(50) {
		t.Fatal("held up to TurretRelease")
	}
	f.pos[1] = [2]int{10, 6 + TurretRelease}
	f.now++
	tu.Phase(f.now)
	if tu.Firing(50) {
		t.Fatal("let go past TurretRelease")
	}
	// out of sight for TurretLost ticks: let go
	f.pos[1] = [2]int{10, 12}
	f.now = scanTick(f.now)
	tu.Phase(f.now)
	f.build(60, construction.Wall, 10, 8)
	for i := 1; i < TurretLost; i++ {
		f.now++
		tu.Phase(f.now)
	}
	if !tu.Firing(50) {
		t.Fatal("held while out of sight for less than TurretLost")
	}
	f.now++
	tu.Phase(f.now)
	if tu.Firing(50) {
		t.Fatal("let go after TurretLost out of sight")
	}
	// no power: dark
	f.RemoveBuilding(60)
	f.now = scanTick(f.now)
	tu.Phase(f.now)
	f.powered[50] = false
	f.now++
	tu.Phase(f.now)
	if tu.Firing(50) {
		t.Fatal("no power: no aim")
	}
}

func TestTurretTargets(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	f.pawn(1, 10, 7, weapon.Rifle)
	f.beast(2, 11, 7, animal.State{Species: animal.Deer})
	f.beast(3, 12, 7, animal.State{Species: animal.TimberWolf, Tame: true, Mode: animal.Attack, Target: 1})
	f.now = scanTick(f.now)
	tu.Phase(f.now)
	if tu.Firing(50) {
		t.Fatal("colonists, calm and tame animals are never targets")
	}
	f.beast(4, 13, 9, animal.State{Species: animal.Deer, Manhunter: f.now + 1000})
	f.beast(5, 14, 9, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
	f.now += TurretScanEvery
	tu.Phase(f.now)
	if a := tu.Aim(50); a.Target != 4 {
		t.Fatalf("the nearest hostile, a manhunter: %+v", a)
	}
	if !tu.Targetable(5, f.now) || tu.Targetable(2, f.now) {
		t.Fatal("targetable")
	}
}

func TestTurretBlastAndChain(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	f.pawn(1, 11, 6, weapon.Rifle)
	f.pawn(2, 20, 20, weapon.Rifle)
	f.build(61, construction.Wall, 12, 5)
	f.build(62, construction.Wall, 30, 5)
	tu.setAim(50, Aim{Target: 1})
	b := f.bldgs[50]
	f.RemoveBuilding(50)
	f.droll = []int{TurretBlastPct} // half the time it does not blow up
	tu.Blast(50, b)
	if tu.Firing(50) || f.bursts != 0 || len(f.msgs) != 0 {
		t.Fatalf("no blast: bursts %d msgs %v", f.bursts, f.msgs)
	}
	f.droll = []int{TurretBlastPct - 1}
	tu.Blast(50, b)
	if f.bursts != 1 || len(f.msgs) != 1 || f.blasts[0].Radius != 3900 || f.blasts[0].Damage != TurretBlastDamage {
		t.Fatalf("blast: bursts %d msgs %v %+v", f.bursts, f.msgs, f.blasts)
	}
	if len(f.hurts) != 1 || f.damage[61] == 0 || f.damage[62] != 0 {
		t.Fatalf("blast reach: hurts %v damage %v", f.hurts, f.damage)
	}
}

func TestForRaider(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	if id, x, y, ok := tu.ForRaider(10, 13, 8+SwitchMargin, 20); !ok || id != 50 || x != 10 || y != 5 {
		t.Fatal("a turret SwitchMargin closer draws fire")
	}
	if _, _, _, ok := tu.ForRaider(10, 13, 8+SwitchMargin-1, 20); ok {
		t.Fatal("not closer by the margin")
	}
	if _, _, _, ok := tu.ForRaider(10, 13, 40, 7); ok {
		t.Fatal("out of sight")
	}
	f.powered[50] = false
	if _, _, _, ok := tu.ForRaider(10, 13, 40, 20); ok {
		t.Fatal("an unpowered turret draws no fire")
	}
	f.wired = false
	if _, _, _, ok := tu.ForRaider(10, 13, 40, 20); ok {
		t.Fatal("no power device: no turret")
	}
	if _, ok := tu.At(50); !ok {
		t.Fatal("At finds the turret")
	}
}

func TestTurretListDropsGoneAims(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	tu.setAim(50, Aim{Target: 1})
	tu.setAim(77, Aim{Target: 1})
	tu.Phase(f.now)
	if tu.Firing(77) {
		t.Fatal("the aim of a turret that is gone is dropped")
	}
	f.wired = false
	f.defense, f.gen = nil, 2
	tu.Phase(f.now)
	if tu.Firing(50) {
		t.Fatal("unwired: the list moves and old aims go")
	}
}
