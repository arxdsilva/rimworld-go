package combat

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// hurtTo sets turret id's hit points to pct of its most, returning it and
// how many it had before.
func hurtTo(f *fakeHost, id entity.EntityID, pct int) (construction.Building, int) {
	b := f.bldgs[id]
	before := b.HP()
	full := construction.MaxHP(b.Kind, b.Stuff)
	b.Damage = full - full*pct/100
	f.bldgs[id] = b
	return b, before
}

// TestTurretFuseBurnsThenBlows (wiki: Mini-turret): the fall below 20 % lights
// a fuse of 240 ticks, then the turret is destroyed, and that sets it off.
func TestTurretFuseBurnsThenBlows(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	if TurretFusePct != 20 || TurretFuseTicks != 240 || TurretBlastPct != 50 {
		t.Fatalf("fuse %d %d %d: the wiki says 20 %%, 240 ticks, 50 %%", TurretFusePct, TurretFuseTicks, TurretBlastPct)
	}
	b, before := hurtTo(f, 50, 25)
	tu.Hurt(50, b, before)
	if tu.Sparking(50) {
		t.Fatal("sparking at 25 %")
	}
	b, before = hurtTo(f, 50, 15)
	f.droll = []int{TurretBlastPct - 1}
	tu.Hurt(50, b, before)
	if !tu.Sparking(50) || len(f.msgs) != 1 {
		t.Fatalf("no fuse: %v", f.msgs)
	}
	lit := f.now
	f.now = lit + TurretFuseTicks - 1
	tu.burnFuses(f.now)
	if f.damage[50] != 0 {
		t.Fatal("blew up early")
	}
	f.now = lit + TurretFuseTicks
	tu.burnFuses(f.now)
	if f.damage[50] != b.HP() {
		t.Fatalf("damage %d, want its last %d", f.damage[50], b.HP())
	}
	f.RemoveBuilding(50)
	tu.Blast(50, b)
	if f.bursts != 1 || f.blasts[0].Radius != turretBlastRadius || f.blasts[0].Damage != TurretBlastDamage || tu.Sparking(50) {
		t.Fatalf("blast %+v, sparking %v", f.blasts, tu.Sparking(50))
	}
}

// TestTurretFuseRollsOnce: the fall is the one roll; more wounds below the mark
// roll nothing. A repair above it makes another fall possible.
func TestTurretFuseRollsOnce(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	b, before := hurtTo(f, 50, 15)
	f.droll = []int{TurretBlastPct, 0, 0} // this fall comes up dud
	tu.Hurt(50, b, before)
	if tu.Sparking(50) || len(f.droll) != 2 {
		t.Fatalf("sparking %v, rolls left %v", tu.Sparking(50), f.droll)
	}
	b, before = hurtTo(f, 50, 5)
	tu.Hurt(50, b, before)
	if tu.Sparking(50) || len(f.droll) != 2 {
		t.Fatalf("a wound below the mark rolled: sparking %v, rolls left %v", tu.Sparking(50), f.droll)
	}
	hurtTo(f, 50, 100) // repaired
	b, before = hurtTo(f, 50, 15)
	tu.Hurt(50, b, before)
	if !tu.Sparking(50) || len(f.droll) != 1 {
		t.Fatalf("a new fall: sparking %v, rolls left %v", tu.Sparking(50), f.droll)
	}
}

// TestTurretKilledBeforeItsFuse: destroyed while it sparks, it blows up at once
// and only once; with no fuse lit it just breaks.
func TestTurretKilledBeforeItsFuse(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	b, before := hurtTo(f, 50, 15)
	tu.Hurt(50, b, before)
	b.Damage = construction.MaxHP(b.Kind, b.Stuff)
	f.RemoveBuilding(50)
	tu.Blast(50, b)
	tu.Blast(50, b)
	tu.burnFuses(f.now + TurretFuseTicks)
	if f.bursts != 1 || f.damage[50] != 0 {
		t.Fatalf("bursts %d, damage %d", f.bursts, f.damage[50])
	}
	f, tu = turretRig(t)
	b, before = hurtTo(f, 50, 0)
	f.droll = []int{TurretBlastPct}
	tu.Hurt(50, b, before)
	f.RemoveBuilding(50)
	tu.Blast(50, b)
	if f.bursts != 0 || len(f.msgs) != 0 {
		t.Fatalf("a dud blew up: bursts %d, msgs %v", f.bursts, f.msgs)
	}
}

// TestHeavyTurretBlastRadius (wiki: Autocannon turret, Uranium slug turret):
// 5.9 tiles, where a mini-turret's is 3.9.
func TestHeavyTurretBlastRadius(t *testing.T) {
	t.Parallel()
	for _, k := range []construction.Kind{construction.Autocannon, construction.UraniumSlugTurret} {
		f := newFake(40, 30)
		f.build(50, k, 10, 5)
		tu := f.combat().Turrets
		if !k.BurstsWhenDestroyed() {
			t.Fatalf("the %v never reaches Blast", k)
		}
		b, before := hurtTo(f, 50, 0)
		tu.Hurt(50, b, before)
		f.RemoveBuilding(50)
		tu.Blast(50, b)
		if f.bursts != 1 || f.blasts[0].Radius != 5900 {
			t.Fatalf("%v: %+v", k, f.blasts)
		}
	}
}

// TestFoamBurstsWithoutAFuse: foam goes on bursting as it breaks, with no roll.
func TestFoamBurstsWithoutAFuse(t *testing.T) {
	t.Parallel()
	f := newFake(40, 30)
	f.build(50, construction.FoamPopper, 10, 5)
	tu := f.combat().Turrets
	b, before := hurtTo(f, 50, 0)
	f.droll = []int{0}
	tu.Hurt(50, b, before)
	if tu.Sparking(50) || len(f.droll) != 1 {
		t.Fatalf("sparking %v, rolls left %v", tu.Sparking(50), f.droll)
	}
}

// TestCratedTurretForgetsItsFuse: a turret that left another way drops its fuse
// and hurts nothing.
func TestCratedTurretForgetsItsFuse(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	b, before := hurtTo(f, 50, 15)
	tu.Hurt(50, b, before)
	f.RemoveBuilding(50)
	tu.burnFuses(f.now + TurretFuseTicks)
	if tu.Sparking(50) || f.damage[50] != 0 || f.bursts != 0 {
		t.Fatalf("sparking %v, damage %d, bursts %d", tu.Sparking(50), f.damage[50], f.bursts)
	}
}

// TestTurretFuseSaved: a burning fuse survives a save and keeps its hour.
func TestTurretFuseSaved(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	b, before := hurtTo(f, 50, 15)
	tu.Hurt(50, b, before)
	s := tu.Export()
	if s == nil || len(s.Sparks) != 1 {
		t.Fatalf("export %+v", s)
	}
	f2, back := turretRig(t)
	f2.bldgs[50] = f.bldgs[50]
	f2.now = f.now
	back.Import(s)
	if !back.Sparking(50) {
		t.Fatal("the fuse went out in the save")
	}
	back.burnFuses(f2.now + TurretFuseTicks - 1)
	if f2.damage[50] != 0 {
		t.Fatal("the fuse got shorter in the save")
	}
	back.burnFuses(f2.now + TurretFuseTicks)
	if f2.damage[50] != b.HP() {
		t.Fatalf("damage %d", f2.damage[50])
	}
}
