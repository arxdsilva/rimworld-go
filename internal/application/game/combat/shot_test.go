// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"fmt"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// settle flies every shot until none is left.
func settle(c *Combat) {
	for i := 0; c.Shots.Len() > 0; i++ {
		if i > 1000 {
			panic("a shot never lands")
		}
		c.Shots.Step()
	}
}

// landed is the last Landed trace.
func landed(f *fakeHost) projectile.Trace {
	for i := len(f.traces) - 1; i >= 0; i-- {
		if f.traces[i].Stage == projectile.Landed {
			return f.traces[i]
		}
	}
	return projectile.Trace{}
}

type fixedFriendlyFire struct {
	host    *fakeHost
	percent int
}

func (d fixedFriendlyFire) FriendlyFirePct() int             { return d.percent }
func (d fixedFriendlyFire) Colonist(id entity.EntityID) bool { return d.host.pawns[id] }

func combatWithFriendlyFire(f *fakeHost, percent int) *Combat {
	return New(f, Roles{MortarAim: f, Power: f, Flight: f, Hunts: f, Breach: f, Field: f, Supply: f, Draft: f,
		Rules: fixedFriendlyFire{f, percent}})
}

func TestBurstTiming(t *testing.T) {
	t.Parallel()
	f := newFake(10, 10)
	s := f.combat().Strikes
	d := weapon.Def{Ranged: true, Burst: 3, BurstTicks: 10, Cooldown: 102, Warmup: 60}
	var j job.Record
	var waits []int
	for i := 0; i < 4; i++ {
		waits = append(waits, s.next(1, d, &j))
	}
	if !reflect.DeepEqual(waits, []int{10, 10, 162, 10}) || j.Count != 2 {
		t.Fatalf("burst waits %v count %d", waits, j.Count)
	}
	d.BurstTicks = 0
	j = job.Record{}
	if s.next(1, d, &j) != 1 {
		t.Fatal("burst shots are at least a tick apart")
	}
	j = job.Record{}
	if got := s.next(1, weapon.DefOf(weapon.Knife), &j); got != 90 || j.Count != 0 {
		t.Fatalf("a melee blow waits its cooldown, no aim: %d", got)
	}
	// the warm-up of a first attack is the weapon's
	if AimTicks(f, 1, weapon.DefOf(weapon.Rifle)) != 102 || AimTicks(f, 1, weapon.DefOf(weapon.Spear)) != 0 {
		t.Fatal("warmup")
	}
}

func TestAccuracyAtBandDistances(t *testing.T) {
	t.Parallel()
	f := newFake(60, 10)
	s := f.combat().Strikes
	f.pawn(1, 0, 5, weapon.Rifle)
	for dist, want := range map[int]int{3: 650, 12: 800, 25: 900, 36: 900 - 100*11/15} {
		f.raider(2, dist, 5, weapon.Fists)
		r, ok := s.Report(1, 2)
		if !ok || r.Chance != want || r.Band != want || r.Dist != dist || !r.InRange || !r.InSight || r.Melee {
			t.Errorf("perfect shooter at %d: %+v want %d", dist, r, want)
		}
	}
	f.aims[1] = [2]int{attack.ShooterPerTile(10), 10}
	f.pos[2] = [2]int{12, 5}
	r, _ := s.Report(1, 2)
	if want := 800 * attack.ShooterFactor(attack.ShooterPerTile(10), 12) / 1000; r.Chance != want || r.Shooter < 692 || r.Shooter > 694 {
		t.Fatalf("skill 10 at 12: %+v want %d", r, want)
	}
	// SHT-002: Deliberate aim's +5 points join the score before the
	// per-tile curve; the shooter factor carries them, no flat term after range
	f.aims[1] = [2]int{attack.ShooterPerTile(attack.ShootingScore(10, 5)), 10}
	if r5, _ := s.Report(1, 2); r5.Shooter != attack.ShooterFactor(attack.ShooterPerTile(attack.ShootingScore(10, 5)), 12) ||
		r5.Shooter <= r.Shooter || r5.Chance != r5.Total() {
		t.Fatalf("score points: %+v (level 10 alone %+v)", r5, r)
	}
	f.aims[1] = [2]int{attack.PerfectShooter, 20}
	f.beast(3, 12, 6, animal.State{Species: animal.Squirrel})
	if r, _ := s.Report(1, 3); r.Size != 500 || r.Chance != 800*r.Size/1000 {
		t.Fatalf("a small target: %+v", r)
	}
	f.pos[2] = [2]int{37, 5}
	if r, _ := s.Report(1, 2); r.InRange || r.InSight {
		t.Fatal("out of range")
	}
	f.pos[2] = [2]int{12, 5}
	f.rock[[2]int{6, 5}] = true
	if r, _ := s.Report(1, 2); !r.InRange || r.InSight {
		t.Fatal("rock blocks the sight")
	}
	if _, ok := s.Report(1, 99); ok {
		t.Fatal("no target")
	}
	f.pawn(4, 13, 5, weapon.Knife)
	if r, _ := s.Report(4, 2); !r.Melee || r.Chance != attack.HitChanceOf(f.Melee(4)) || r.Dist != 1 {
		t.Fatalf("melee report %+v", r)
	}
}

func TestShotFliesThenHits(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 20, 5, weapon.Fists)
	c.Strikes.Strike(1, 2, weapon.Rifle)
	fired := f.traces[0]
	if fired.Stage != projectile.Fired || !fired.Hit || fired.ToX != 20 || fired.Damage != 0 || f.struck[1] != 2 || f.learn != 1 {
		t.Fatalf("fired %+v", fired)
	}
	v := c.Shots.Views()
	if len(v) != 1 || v[0].X != 2000 || v[0].Weapon != "rifle" || v[0].Owner != 1 || v[0].Target != 2 {
		t.Fatalf("view %+v", v)
	}
	c.Shots.Step()
	if v := c.Shots.Views(); v[0].X != 2000+1166 || len(f.hurts) != 0 {
		t.Fatalf("in flight %+v", v)
	}
	settle(c)
	d := weapon.DefOf(weapon.Rifle)
	l := landed(f)
	if len(f.hurts) != 1 || f.hits[2] != d.Damage || f.stains != 1 || !l.Hit || l.Building || l.Damage != d.Damage ||
		l.ToX != 20 || l.FromX != 2 || l.Target != 2 || l.Attacker != 1 {
		t.Fatalf("landed %+v hurts %v", l, f.hurts)
	}
	if c.Shots.Views() != nil {
		t.Fatal("nothing in flight")
	}
	// a target that took a step while the shot flew still takes it; one
	// that got two away is missed
	c.Strikes.Strike(1, 2, weapon.Rifle)
	f.pos[2] = [2]int{20, 5 + Tracking}
	settle(c)
	if l := landed(f); len(f.hurts) != 2 || l.ToY != 5+Tracking || l.Target != 2 {
		t.Fatalf("tracked: %+v", l)
	}
	f.pos[2] = [2]int{20, 5}
	f.rolls = []int{0, 999} // aimed; nothing to strike where it lands
	c.Strikes.Strike(1, 2, weapon.Rifle)
	f.pos[2] = [2]int{20, 5 + Tracking + 1}
	settle(c)
	if l := landed(f); len(f.hurts) != 2 || l.Hit {
		t.Fatalf("stepped away: %+v", l)
	}
}

func TestStrayHitAndMiss(t *testing.T) {
	t.Parallel()
	f := newFake(40, 12)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 20, 5, weapon.Fists)
	f.beast(3, 20, 6, animal.State{Species: animal.Deer})
	// aim 846 at 18 tiles: a miss radius of 1. The aim fails, the shot
	// lands one cell below the target, on the deer, and strikes it.
	f.rolls = []int{999, 1, 2, attack.StrayHit - 1}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	if f.traces[0].Hit || f.traces[0].ToY != 6 {
		t.Fatalf("a wild shot is headed off target: %+v", f.traces[0])
	}
	settle(c)
	if l := landed(f); len(f.hurts) != 1 || f.hits[3] != weapon.DefOf(weapon.Rifle).Damage || l.Target != 3 || !l.Hit {
		t.Fatalf("stray hit: %+v %v", l, f.hurts)
	}
	f.rolls = []int{999, 1, 2, attack.StrayHit}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if l := landed(f); len(f.hurts) != 1 || l.Hit || l.ToY != 6 || f.logs[len(f.logs)-1] != "strike 1 2 false" {
		t.Fatalf("stray miss: %+v %v", l, f.logs)
	}
	// a stray shot landing on a building damages it half the time
	f.build(9, construction.Sandbags, 19, 4)
	f.rolls = []int{999, 0, 0, 0}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if l := landed(f); f.damage[9] != weapon.DefOf(weapon.Rifle).Damage || !l.Building {
		t.Fatalf("stray on sandbags: %+v %v", l, f.damage)
	}
	// a wild shot never lands on the target's own cell
	if x, y := c.Shots.missAt(2, 5, 20, 5, 0, f.Roll); x != 21 || y != 5 {
		t.Fatalf("missAt %d,%d", x, y)
	}
}

// interceptAt is a human-sized creature's interception chance (permille) a
// distance tiles down the line of fire: 400 scaled by the distance factor.
func interceptAt(distance int) attack.Permille {
	return attack.InterceptChance(1000) * attack.DistanceInterceptFactor(distance, 0) / 1000
}

// TestFriendlyFire: a creature in the line, ally or not, is struck at its
// interception chance scaled by its distance from the muzzle (SHT-005):
// none and no roll within InterceptNear, linear to the full chance at
// InterceptFull.
func TestFriendlyFire(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 20, 5, weapon.Fists)
	f.pawn(3, 10, 5, weapon.Fists) // an ally 8 tiles down the line
	at8 := attack.InterceptChance(1000) * attack.InterceptScale(8) / 1000
	if at8 != 171 {
		t.Fatalf("chance at 8 tiles %d, want 171", at8)
	}
	f.rolls = []int{0, at8 - 1}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if l := landed(f); len(f.hurts) != 1 || f.hits[3] == 0 || f.hits[2] != 0 || l.Target != 3 || l.ToX != 10 {
		t.Fatalf("the ally takes it: %+v %v", l, f.hurts)
	}
	f.rolls = []int{0, at8} // the ally is missed, the raider hit
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if f.hits[2] == 0 {
		t.Fatal("past the ally")
	}
	// a clear line: the draws of a shot with nobody in the way
	f.pos[3] = [2]int{10, 8}
	f.rolls, f.rolled = nil, 0
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	clear := f.rolled
	// within InterceptNear of the muzzle nobody is struck, ally or hostile,
	// and no roll is drawn
	f.pos[3] = [2]int{2 + attack.InterceptNear, 5} // the ally 5 tiles out
	f.rolls, f.rolled = nil, 0
	before := f.hits[2]
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if f.hits[3] != weapon.DefOf(weapon.Rifle).Damage || f.hits[2] == before || f.rolled != clear {
		t.Fatalf("a close ally: hits %v, %d draws vs %d on a clear line", f.hits, f.rolled, clear)
	}
	f.pos[3] = [2]int{10, 8}
	f.raider(4, 4, 5, weapon.Fists) // a raider 2 tiles from the muzzle
	f.rolls, f.rolled = nil, 0
	before = f.hits[2]
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if f.hits[4] != 0 || f.hits[2] == before || f.rolled != clear {
		t.Fatalf("a close raider: hits %v, %d draws vs %d on a clear line", f.hits, f.rolled, clear)
	}
	// from InterceptFull on, the full chance
	f.pos[4] = [2]int{4, 8}
	f.raider(5, 2+attack.InterceptFull, 5, weapon.Fists)
	f.rolls = []int{0, attack.InterceptChance(1000) - 1}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if f.hits[5] == 0 {
		t.Fatal("a raider 12 tiles out is struck at the full chance")
	}
	before, five := f.hits[2], f.hits[5]
	f.rolls = []int{0, attack.InterceptChance(1000)}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if f.hits[5] != five || f.hits[2] == before {
		t.Fatalf("past the raider at the full chance: %v", f.hits)
	}
}

func TestDownedInterceptorReducedChance(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 20, 5, weapon.Fists)
	f.pawn(3, 10, 5, weapon.Fists)
	f.lying[3] = true
	chance := attack.DownedInterceptChance(interceptAt(8))
	if chance != 17 {
		t.Fatalf("downed interception chance %d, want a tenth of 171 permille", chance)
	}
	f.rolls = []int{0, chance - 1}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if f.hits[3] == 0 || f.hits[2] != 0 {
		t.Fatal("a downed colonist can still intercept a shot at its reduced chance")
	}
	f.rolls = []int{0, chance}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if f.hits[2] == 0 {
		t.Fatal("a roll at the reduced chance passes the downed interceptor")
	}
}

func TestFriendlyFireDifficultyScalesFriendlyInterception(t *testing.T) {
	t.Parallel()
	d := weapon.DefOf(weapon.Rifle)
	shot := projectile.Shot{Owner: 1, Weapon: weapon.Rifle, Damage: d.Damage, Pen: weapon.Penetration(d, d.Damage),
		Type: d.DamageType, Hit: weapon.HitOf(weapon.Rifle, d), FromX: 1, FromY: 5, Aimed: true}

	t.Run("side to side", func(t *testing.T) {
		f := newFake(30, 10)
		f.pawn(1, 1, 5, weapon.Rifle)
		f.pawn(2, 13, 5, weapon.Fists)
		c := combatWithFriendlyFire(f, 50)
		f.rolls = []int{199, 0, 0, 0}
		if !c.Shots.through(&shot, 13, 5) || f.hits[2] == 0 {
			t.Fatal("50% friendly fire keeps the 200-permille interception at roll 199")
		}
	})

	t.Run("raiders shooting raiders are not scaled", func(t *testing.T) {
		f := newFake(30, 10)
		f.raider(1, 1, 5, weapon.Rifle)
		f.raider(2, 13, 5, weapon.Fists)
		c := combatWithFriendlyFire(f, 0)
		raiderShot := shot
		raiderShot.Raider = true
		f.rolls = []int{399, 0, 0, 0}
		if !c.Shots.through(&raiderShot, 13, 5) || f.hits[2] == 0 {
			t.Fatal("friendly-fire difficulty must not touch raider-to-raider interception")
		}
	})

	t.Run("a raider interceptor of a colonist's shot is not scaled", func(t *testing.T) {
		f := newFake(30, 10)
		f.pawn(1, 1, 5, weapon.Rifle)
		f.raider(2, 13, 5, weapon.Fists)
		c := combatWithFriendlyFire(f, 0)
		f.rolls = []int{399, 0, 0, 0}
		if !c.Shots.through(&shot, 13, 5) || f.hits[2] == 0 {
			t.Fatal("a hostile in the way is caught at the full chance")
		}
	})

	t.Run("zero suppresses colonist interception without a roll", func(t *testing.T) {
		f := newFake(30, 10)
		f.pawn(1, 1, 5, weapon.Rifle)
		f.pawn(2, 13, 5, weapon.Fists)
		c := combatWithFriendlyFire(f, 0)
		f.rolls = []int{0}
		if c.Shots.through(&shot, 13, 5) || f.hits[2] != 0 || len(f.rolls) != 1 {
			t.Fatal("zero friendly fire prevents the interception roll")
		}
	})
}

func TestDownedTargetAccuracy(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.pawn(2, 5, 5, weapon.Fists)
	f.lying[2] = true
	d := weapon.DefOf(weapon.Rifle)
	close := c.Strikes.report(1, 2, 5, 2, 5, 5, d, false)
	if close.Downed == 0 || close.Dist != 3 || close.Aim() != 1000 {
		t.Fatalf("three-tile execution bonus: %+v aim %d", close, close.Aim())
	}

	four := c.Strikes.report(1, 2, 5, 2, 6, 5, d, false)
	f.lying[2] = false
	fourStanding := c.Strikes.report(1, 2, 5, 2, 6, 5, d, false)
	if four.Downed == 0 || four.Dist != 4 || four.Aim() != fourStanding.Aim() {
		t.Fatalf("exactly four tiles is neutral: downed %+v standing %+v", four, fourStanding)
	}

	f.lying[2] = true
	five := c.Strikes.report(1, 2, 5, 2, 7, 5, d, false)
	f.lying[2] = false
	fiveStanding := c.Strikes.report(1, 2, 5, 2, 7, 5, d, false)
	if five.Downed == 0 || five.Dist != 5 || five.Aim() != fiveStanding.Aim()/2 {
		t.Fatalf("beyond four tiles halves chance: downed %+v standing %+v", five, fiveStanding)
	}

	f.lying[2] = true
	melee := c.Strikes.report(1, 2, 5, 2, 5, 5, weapon.DefOf(weapon.Knife), false)
	if melee.Downed != 0 || melee.Total() != melee.Band {
		t.Fatalf("the ranged execution bonus must not change melee: %+v", melee)
	}
}

func TestWallsAndMapEdgeStopShots(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 20, 5, weapon.Fists)
	f.build(7, construction.Wall, 10, 5)
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if l := landed(f); f.damage[7] != weapon.DefOf(weapon.Rifle).Damage || !l.Building || !l.Hit || l.ToX != 10 || len(f.hurts) != 0 {
		t.Fatalf("the wall stops it: %+v", l)
	}
	delete(f.objs, [2]int{10, 5})
	f.rock[[2]int{10, 5}] = true
	c.Strikes.Strike(1, 2, weapon.Rifle)
	settle(c)
	if l := landed(f); !l.Building || l.Damage != 0 || len(f.hurts) != 0 {
		t.Fatalf("rock stops it: %+v", l)
	}
	// a wild shot off the map edge is gone
	f.pawn(3, 5, 5, weapon.Rifle)
	f.raider(4, 0, 5, weapon.Fists)
	f.rolls = []int{999, 0, 1, 999} // wild, one cell past; it flies through the target untouched
	c.Strikes.Strike(3, 4, weapon.Rifle)
	settle(c)
	if l := landed(f); l.Hit || l.ToX != -1 {
		t.Fatalf("off the map: %+v", l)
	}
}

func TestArmorDeflectHalvePass(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.pawn(1, 5, 5, weapon.Knife)
	f.raider(2, 6, 5, weapon.Fists)
	duster, _ := armor.Apparel(item.Duster)
	f.worn[2] = []armor.Layer{duster} // A = 243 - 180 = 63
	f.rolls = []int{0, 0, 200, 10}    // the blade, a hit, x1.0, deflected (Wounds.Absorb: one roll per garment)
	s.Strike(1, 2, weapon.Knife)
	if tr := f.traces[0]; !tr.Hit || tr.Damage != 0 || len(f.hurts) != 0 || f.stains != 0 {
		t.Fatalf("deflected: %+v %v", tr, f.hurts)
	}
	f.rolls = []int{0, 0, 200, 40} // halved: a sharp blow turns blunt
	s.Strike(1, 2, weapon.Knife)
	if want := fmt.Sprintf("2-6-1-%v", damage.FormBlunt); len(f.hurts) != 1 || f.hurts[0] != want || f.traces[1].Damage != 6 {
		t.Fatalf("halved: %v want %s", f.hurts, want)
	}
	f.rolls = []int{0, 0, 200, 63} // passed: the firing weapon's form
	s.Strike(1, 2, weapon.Knife)
	if want := fmt.Sprintf("2-12-1-%v", weapon.HitOf(weapon.Knife, weapon.DefOf(weapon.Knife)).Form); len(f.hurts) != 2 || f.hurts[1] != want {
		t.Fatalf("passed: %v want %s", f.hurts, want)
	}
	// a bullet's armor roll at landing: the rifle's 27 percent
	// penetration beats the duster's sharp 24.3 percent
	c := f.combat()
	f.pawn(3, 1, 1, weapon.Rifle)
	f.rolls = []int{0}
	c.Strikes.Strike(3, 2, weapon.Rifle)
	settle(c)
	if l := landed(f); l.Damage != weapon.DefOf(weapon.Rifle).Damage || len(f.rolls) != 0 {
		t.Fatalf("penetrated: %+v rolls left %v", l, f.rolls)
	}
}

func TestShotsSaveAndDeterminism(t *testing.T) {
	t.Parallel()
	run := func(save bool) []projectile.Trace {
		f := newFake(40, 12)
		c := f.combat()
		f.pawn(1, 2, 5, weapon.Rifle)
		f.raider(2, 30, 6, weapon.Fists)
		f.pawn(3, 12, 5, weapon.Fists)
		f.rolls = []int{0, 700, 999, 3, 1, 200, 0, 999}
		c.Strikes.Strike(1, 2, weapon.Rifle)
		c.Strikes.Strike(1, 2, weapon.Rifle)
		c.Shots.Step()
		if save {
			saved := c.Shots.Export()
			c = f.combat()
			c.Shots.Import(saved)
		}
		settle(c)
		return f.traces
	}
	a, b := run(false), run(true)
	if !reflect.DeepEqual(a, b) || len(a) != 4 {
		t.Fatalf("a save in flight changes the outcome:\n%+v\n%+v", a, b)
	}
	f := newFake(5, 5)
	if f.combat().Shots.Export() != nil {
		t.Fatal("nothing saved when nothing flies")
	}
}
