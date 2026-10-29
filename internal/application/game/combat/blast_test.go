package combat

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
)

var fragBlast = weapon.DefOf(weapon.FragGrenade).Blast

// hurtOf is the damage hurts recorded for id (HurtAs lines) and the wound
// kind of the last.
func hurtOf(f *fakeHost, id entity.EntityID) (dmg int, kind body.WoundKind, n int) {
	for _, h := range f.hurts {
		var t, d, by, k int
		if c, _ := fmt.Sscanf(h, "%d-%d-%d-%d", &t, &d, &by, &k); c == 4 && entity.EntityID(t) == id {
			dmg, kind, n = dmg+d, body.WoundKind(k), n+1
		}
	}
	return dmg, kind, n
}

// TestBlastDamageByDistance: everyone within the radius is wounded, the
// centre hardest; nobody past it.
func TestBlastDamageByDistance(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	c := f.combat()
	f.pawn(1, 10, 10, weapon.Fists) // on the centre
	f.pawn(2, 11, 11, weapon.Fists) // 1.41 tiles
	f.pawn(3, 12, 10, weapon.Fists) // 2 tiles: outside 1.9
	f.beast(4, 10, 11, animal.State{})
	c.Blasts.Explode(9, weapon.FragGrenade, 10, 10, fragBlast)
	d1, k1, _ := hurtOf(f, 1)
	d2, _, _ := hurtOf(f, 2)
	if d1 != 50 || k1 != body.Cut || d2 != fragBlast.At(1414) || d2 >= d1 || d2 < 25 {
		t.Fatalf("centre %d (kind %d), diagonal %d", d1, k1, d2)
	}
	if _, _, n := hurtOf(f, 3); n != 0 {
		t.Fatal("two tiles out is past a 1.9 blast")
	}
	if _, _, n := hurtOf(f, 4); n != 1 {
		t.Fatal("animals are caught too")
	}
	if f.bursts != 1 || f.blasts[0] != fragBlast || f.fires != 0 || f.stains != 3 {
		t.Fatalf("burst %d %+v fires %d stains %d", f.bursts, f.blasts, f.fires, f.stains)
	}
}

// TestBlastBlockedByWall: a wall between the centre and a creature
// shelters it, and takes the blast itself (bomb damage doubled).
func TestBlastBlockedByWall(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	c := f.combat()
	big := explosion.Def{Kind: explosion.Bomb, Radius: 3900, Damage: 50, Pen: 100}
	f.build(30, construction.Wall, 11, 10)
	f.pawn(1, 12, 10, weapon.Fists) // behind the wall
	f.pawn(2, 10, 12, weapon.Fists) // in the open, 2 tiles
	f.rock[[2]int{9, 10}] = true
	f.pawn(3, 8, 10, weapon.Fists) // behind rock
	c.Blasts.Explode(9, weapon.FragGrenade, 10, 10, big)
	if _, _, n := hurtOf(f, 1); n != 0 {
		t.Fatal("the wall sheltered nobody")
	}
	if _, _, n := hurtOf(f, 3); n != 0 {
		t.Fatal("the rock sheltered nobody")
	}
	if _, _, n := hurtOf(f, 2); n != 1 {
		t.Fatal("the open side is hit")
	}
	if f.damage[30] != big.At(1000)*2 {
		t.Fatalf("wall took %d, want %d", f.damage[30], big.At(1000)*2)
	}
}

// TestBlastArmor: worn armor takes its share of a blast.
func TestBlastArmor(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	c := f.combat()
	f.pawn(1, 10, 10, weapon.Fists)
	marine, _ := armor.Apparel(item.MarineArmor)
	f.worn[1] = []armor.Layer{marine}
	f.rolls = []int{500, 0} // the torso, then a deflecting roll
	c.Blasts.Explode(9, weapon.FragGrenade, 10, 10, fragBlast)
	if _, _, n := hurtOf(f, 1); n != 0 || f.stains != 0 {
		t.Fatal("marine armor deflected the shrapnel")
	}
}

// TestMolotovIgnitesAndBurns: a flame blast sets the centre alight and
// burns those it reaches, only the orthogonal neighbours at radius 1.1.
func TestMolotovIgnitesAndBurns(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	c := f.combat()
	f.pawn(1, 11, 10, weapon.Fists)
	f.pawn(2, 11, 11, weapon.Fists)
	f.build(30, construction.Wall, 9, 10)
	c.Blasts.Explode(9, weapon.Molotov, 10, 10, weapon.DefOf(weapon.Molotov).Blast)
	if f.fires != 1 {
		t.Fatal("no fire")
	}
	if d, k, _ := hurtOf(f, 1); d == 0 || k != body.Burn {
		t.Fatalf("burn %d kind %d", d, k)
	}
	if _, _, n := hurtOf(f, 2); n != 0 {
		t.Fatal("the diagonal is past 1.1")
	}
	if f.damage[30] == 0 {
		t.Fatal("flame scorches the wall")
	}
}

// TestEMPStunsTurret: an EMP harms no flesh and no building, but knocks a
// sentry turret out: it neither scans nor fires until it recovers, and
// the stun survives a save.
func TestEMPStunsTurret(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	c := New(f, Roles{Power: f, Flight: f, Hunts: f, Breach: f})
	tu = c.Turrets
	f.pawn(1, 11, 5, weapon.Fists)
	emp := weapon.DefOf(weapon.EMPGrenade).Blast
	c.Blasts.Explode(9, weapon.EMPGrenade, 10, 6, emp)
	if len(f.hurts) != 0 || len(f.damage) != 0 {
		t.Fatalf("EMP hurt %v damaged %v", f.hurts, f.damage)
	}
	until := tu.Aim(50).StunnedUntil
	if want := f.now + uint64(emp.StunTicks(1000)); until != want {
		t.Fatalf("stunned until %d, want %d", until, want)
	}
	f.raider(2, 10, 9, weapon.Rifle)
	for ; f.now < until; f.now++ {
		tu.Phase(f.now)
	}
	if len(f.traces) != 0 || tu.Firing(50) {
		t.Fatal("a stunned turret fired")
	}
	saved := tu.Export()
	tu.Import(saved)
	if tu.Aim(50).StunnedUntil != until {
		t.Fatal("the stun is lost on load")
	}
	tu.Stun(50, until-5) // a shorter stun keeps the longer one
	if tu.Aim(50).StunnedUntil != until {
		t.Fatal("a shorter stun cut the running one")
	}
	for i := 0; i < 100 && len(f.traces) == 0; i++ {
		f.now++
		tu.Phase(f.now)
	}
	if len(f.traces) == 0 {
		t.Fatal("the turret never recovered")
	}
}

// TestSmokeSpoilsAim: a smoke cloud between shooter and target cuts the
// hit chance to attack.SmokeFactor, thins away after SmokeTicks, and is
// saved.
func TestSmokeSpoilsAim(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 14, 5, weapon.Fists)
	clear, _ := c.Strikes.Report(1, 2)
	c.Blasts.Explode(0, weapon.Fists, 8, 5, explosion.Def{Kind: explosion.Smoke, Radius: 2400})
	if len(f.hurts) != 0 || len(f.damage) != 0 {
		t.Fatal("smoke harms nothing")
	}
	smoky, _ := c.Strikes.Report(1, 2)
	if smoky.Chance != clear.Chance*attack.SmokeFactor/1000 || smoky.Weather != attack.SmokeFactor {
		t.Fatalf("through smoke %d, clear %d", smoky.Chance, clear.Chance)
	}
	f.raider(3, 2, 15, weapon.Fists)
	if r, _ := c.Strikes.Report(1, 3); r.Weather != attack.ClearFactor {
		t.Fatal("a shot clear of the cloud")
	}
	saved := c.Blasts.Export()
	c2 := f.combat()
	c2.Blasts.Import(saved)
	if r, _ := c2.Strikes.Report(1, 2); r.Chance != smoky.Chance {
		t.Fatal("the cloud is lost on load")
	}
	f.now += explosion.SmokeTicks
	c.Shots.Step()
	if r, _ := c.Strikes.Report(1, 2); r.Chance != clear.Chance || c.Blasts.Export() != nil {
		t.Fatal("the smoke never cleared")
	}
}

// TestGrenadeThrownBursts: a raider's frag grenade flies over a cover
// wall, bursts on its target's tile and wounds the colonists around it.
func TestGrenadeThrownBursts(t *testing.T) {
	t.Parallel()
	f := newFake(30, 20)
	c := f.combat()
	f.raider(1, 2, 5, weapon.FragGrenade)
	f.pawn(2, 8, 5, weapon.Fists)
	f.pawn(3, 9, 5, weapon.Fists)
	f.build(30, construction.Sandbags, 5, 5) // in the way: a grenade arcs over it
	c.Strikes.Strike(1, 2, weapon.FragGrenade)
	if c.Shots.Len() != 1 {
		t.Fatal("thrown")
	}
	steps := 0
	for ; c.Shots.Len() > 0; steps++ {
		c.Shots.Step()
	}
	if steps < 6*60/12-1 {
		t.Fatalf("a grenade flies at 12 cells a second: %d ticks for 6", steps)
	}
	if l := landed(f); l.ToX != 8 || l.ToY != 5 || !l.Hit || l.Weapon != "frag grenade" {
		t.Fatalf("landed %+v", l)
	}
	if _, _, n := hurtOf(f, 2); n != 1 {
		t.Fatal("the target is caught")
	}
	if _, _, n := hurtOf(f, 3); n != 1 {
		t.Fatal("the one beside it is caught")
	}
	if f.damage[30] != 0 || f.bursts != 1 || !strings.Contains(strings.Join(f.logs, ","), "strike 1 3 true") {
		t.Fatalf("sandbags %d bursts %d logs %v", f.damage[30], f.bursts, f.logs)
	}
}

// TestGrenadeScattersAtRange: past 8 tiles an explosive lands within its
// forced-miss radius of the target, never further.
func TestGrenadeScattersAtRange(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	c := f.combat()
	f.raider(1, 2, 10, weapon.FragGrenade)
	f.pawn(2, 14, 10, weapon.Fists)
	f.rolls = []int{0, 2} // dx -1, dy +1
	c.Strikes.Strike(1, 2, weapon.FragGrenade)
	sh := c.Shots.live[0]
	if sh.ToX != 13 || sh.ToY != 11 || sh.Aimed {
		t.Fatalf("scattered to (%d, %d) aimed %v", sh.ToX, sh.ToY, sh.Aimed)
	}
	settle(c)
	if l := landed(f); l.Stage != projectile.Landed || l.ToX != 13 {
		t.Fatalf("landed %+v", l)
	}
}

// TestBurstWeaponFiresSeveralShots: an assault rifle's attack is a burst
// of three shots ten ticks apart, then its cooldown and warmup.
func TestBurstWeaponFiresSeveralShots(t *testing.T) {
	t.Parallel()
	f := newFake(30, 20)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.AssaultRifle)
	f.raider(2, 12, 5, weapon.Fists)
	d := weapon.DefOf(weapon.AssaultRifle)
	f.jobs[1] = job.Record{Kind: job.Attack, Target: 2, WorkLeft: AimTicks(d)}
	var fired []uint64
	for i := 0; i < d.Warmup+d.Cooldown+40; i++ {
		f.now++
		c.Strikes.Do(1, f.jobs[1], f.now)
		c.Shots.Step()
		for len(fired) < len(f.traces) && f.traces[len(fired)].Stage == projectile.Fired {
			fired = append(fired, f.traces[len(fired)].Tick)
		}
		f.traces = f.traces[:len(fired)]
	}
	if len(fired) != 3 || fired[1]-fired[0] != 10 || fired[2]-fired[1] != 10 {
		t.Fatalf("burst ticks %v", fired)
	}
}
