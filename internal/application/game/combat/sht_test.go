// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// SHT-006/007/008/102 unit tests: downed interceptors, the friendly-fire
// share, the downed-target factor and the first aim's scaling.

// fakeRules is the difficulty's friendly-fire share and the colonist set.
type fakeRules struct {
	pct int
	col map[entity.EntityID]bool
}

func (r fakeRules) FriendlyFirePct() int             { return r.pct }
func (r fakeRules) Colonist(id entity.EntityID) bool { return r.col[id] }

// ruled is f's combat with friendly-fire rules r (nil: none, as f.combat).
func (f *fakeHost) ruled(r Rules) *Combat {
	return New(f, Roles{MortarAim: f, Power: f, Flight: f, Hunts: f, Breach: f, Field: f, Supply: f, Draft: f, Rules: r})
}

// rifleShot is owner's aimed (or wild) rifle shot from (fx, fy) at target,
// as Shots.fire builds it.
func rifleShot(owner, target entity.EntityID, fx, fy, tx, ty int, aimed bool) projectile.Shot {
	d := weapon.DefOf(weapon.Rifle)
	sh := projectile.Shot{Owner: owner, Target: target, Weapon: weapon.Rifle, Damage: d.Damage,
		Pen: weapon.Penetration(d, d.Damage), Type: d.DamageType, Hit: weapon.HitOf(weapon.Rifle, d), Aimed: aimed}
	sh.Launch(fx, fy, tx, ty, d.Speed)
	return sh
}

// passes runs sh through (x, y) on roll r and reports whether it stopped
// there and how many rolls it drew.
func passes(f *fakeHost, c *Combat, sh projectile.Shot, x, y, r int) (stopped bool, drawn int) {
	f.rolls, f.rolled = []int{r}, 0
	stopped = c.Shots.through(&sh, x, y)
	return stopped, f.rolled
}

// TestDownedInterceptor (SHT-006): a creature lying downed in a shot's
// path keeps DownedIntercept of its interception chance; a standing one
// in the same cell is preferred; none within InterceptNear draws a roll;
// and a downed creature is never a stray's victim on landing.
func TestDownedInterceptor(t *testing.T) {
	t.Parallel()
	if BodySize(animal.State{Species: animal.Deer}) != 1000 || attack.InterceptChance(1000) != 400 {
		t.Fatal("the test assumes a deer of size 1000 and a 400 permille interception")
	}
	const want = 40 // 400 x 100 / 1000, 12 tiles out (full scale)
	for _, tc := range []struct {
		name string
		put  func(f *fakeHost)
	}{
		{"downed humanlike", func(f *fakeHost) { f.pos[3], f.lying[3] = [2]int{14, 5}, true }},
		{"downed animal", func(f *fakeHost) { f.beast(3, 14, 5, animal.State{Species: animal.Deer, Downed: true}) }},
	} {
		f := newFake(40, 10)
		c := f.combat()
		f.pawn(1, 2, 5, weapon.Rifle)
		f.raider(2, 30, 5, weapon.Fists)
		tc.put(f)
		sh := rifleShot(1, 2, 2, 5, 30, 5, true)
		if stopped, n := passes(f, c, sh, 14, 5, want-1); !stopped || n != 1 || f.hits[3] == 0 {
			t.Fatalf("%s: struck under %d: stopped %v, %d rolls, hits %v", tc.name, want, stopped, n, f.hits)
		}
		before := f.hits[3]
		if stopped, n := passes(f, c, sh, 14, 5, want); stopped || n != 1 || f.hits[3] != before {
			t.Fatalf("%s: passes at %d: stopped %v, %d rolls", tc.name, want, stopped, n)
		}
		// within InterceptNear of the muzzle no roll is drawn
		f.pos[3] = [2]int{2 + attack.InterceptNear, 5}
		if stopped, n := passes(f, c, sh, 2+attack.InterceptNear, 5, 0); stopped || n != 0 {
			t.Fatalf("%s: close: stopped %v, %d rolls", tc.name, stopped, n)
		}
	}
	// a standing animal keeps its full 400; a standing creature sharing a
	// cell with a downed one is the one picked (even at a higher id)
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 30, 5, weapon.Fists)
	f.beast(7, 14, 5, animal.State{Species: animal.Deer})
	f.pos[3], f.lying[3] = [2]int{14, 5}, true
	sh := rifleShot(1, 2, 2, 5, 30, 5, true)
	if stopped, _ := passes(f, c, sh, 14, 5, 399); !stopped || f.hits[7] == 0 || f.hits[3] != 0 {
		t.Fatalf("the standing deer takes it at 399: hits %v", f.hits)
	}
	if v, downed, ok := c.Shots.interceptor(14, 5, 1); !ok || downed || v != 7 {
		t.Fatalf("interceptor %d downed %v ok %v, want the standing deer 7", v, downed, ok)
	}
	if stopped, _ := passes(f, c, sh, 14, 5, 400); stopped {
		t.Fatal("the deer is missed at 400")
	}
	// two downed: the lowest id; the owner and a turret are never picked
	f.animals[7] = animal.State{Species: animal.Deer, Downed: true}
	if v, downed, ok := c.Shots.interceptor(14, 5, 1); !ok || !downed || v != 3 {
		t.Fatalf("two downed: %d %v %v, want 3", v, downed, ok)
	}
	if v, _, ok := c.Shots.interceptor(2, 5, 1); ok {
		t.Fatalf("the owner is no interceptor: %d", v)
	}
	// a dead animal is none
	f.animals[7] = animal.State{Species: animal.Deer, Injury: 1 << 20}
	f.lying[3] = false
	if v, _, ok := c.Shots.interceptor(14, 5, 1); ok {
		t.Fatalf("the dead are no interceptors: %d", v)
	}
	// land(): a downed creature in the cell is no stray victim; the stray
	// goes on to the building there, one roll, as on an empty cell
	f.beast(8, 20, 8, animal.State{Species: animal.Deer, Downed: true})
	f.build(9, construction.Sandbags, 20, 8)
	f.rolls, f.rolled = []int{0}, 0
	wild := rifleShot(1, 2, 2, 5, 20, 8, false)
	c.Shots.land(&wild, 20, 8)
	if f.hits[8] != 0 || f.damage[9] == 0 || f.rolled != 1 {
		t.Fatalf("downed deer in the landing cell: hits %v, damage %v, %d rolls", f.hits, f.damage, f.rolled)
	}
}

// TestFriendlyFireScale (SHT-007): an aimed shot from a colonist crossing
// a colonist keeps FriendlyFirePct percent of its chance, in flight and
// as a stray; a wild shot, a raider's and a turret's are never scaled;
// at 0 % no roll is drawn and a creature pick strays no further (N3);
// the setting is clamped 0..100 where it is used.
func TestFriendlyFireScale(t *testing.T) {
	t.Parallel()
	col := map[entity.EntityID]bool{1: true, 3: true, 50: true}
	setup := func(pct int) (*fakeHost, *Combat) {
		f := newFake(40, 12)
		c := f.ruled(fakeRules{pct: pct, col: col})
		f.pawn(1, 2, 5, weapon.Rifle)
		f.raider(2, 30, 5, weapon.Fists)
		f.pawn(3, 14, 5, weapon.Fists) // an ally 12 tiles out: 400 permille at 100 %
		return f, c
	}
	aimed := rifleShot(1, 2, 2, 5, 30, 5, true)
	// threshold is the lowest roll that misses the ally (0: no roll drawn).
	for _, tc := range []struct {
		name      string
		pct       int
		sh        projectile.Shot
		threshold int
	}{
		{"FF 0 aimed", 0, aimed, 0},
		{"FF 50 aimed", 50, aimed, 200},
		{"FF 100 aimed", 100, aimed, 400},
		{"FF 250 clamps to 100", 250, aimed, 400},
		{"FF -5 clamps to 0", -5, aimed, 0},
		{"FF 0 wild", 0, rifleShot(1, 2, 2, 5, 30, 5, false), 400},
		{"FF 0 raider", 0, rifleShot(2, 1, 30, 5, 2, 5, true), 400},
	} {
		f, c := setup(tc.pct)
		if tc.name == "FF 0 raider" {
			f.pos[3] = [2]int{18, 5} // 12 tiles from the raider's muzzle
		}
		x, y := f.pos[3][0], f.pos[3][1]
		if tc.threshold == 0 {
			if stopped, n := passes(f, c, tc.sh, x, y, 0); stopped || n != 0 || f.hits[3] != 0 {
				t.Fatalf("%s: stopped %v, %d rolls, hits %v", tc.name, stopped, n, f.hits)
			}
			continue
		}
		if stopped, n := passes(f, c, tc.sh, x, y, tc.threshold-1); !stopped || n != 1 {
			t.Fatalf("%s: struck at %d: stopped %v, %d rolls", tc.name, tc.threshold-1, stopped, n)
		}
		if stopped, _ := passes(f, c, tc.sh, x, y, tc.threshold); stopped {
			t.Fatalf("%s: passes at %d", tc.name, tc.threshold)
		}
	}
	// FF 0 spares colonists only: a raider and a deer in the line of an
	// aimed colonist's shot keep their full 400
	for _, put := range []func(f *fakeHost){
		func(f *fakeHost) { f.raider(6, 14, 5, weapon.Fists) },
		func(f *fakeHost) { f.beast(6, 14, 5, animal.State{Species: animal.Deer}) },
	} {
		f, c := setup(0)
		f.pos[3] = [2]int{14, 9}
		put(f)
		if stopped, _ := passes(f, c, aimed, 14, 5, 399); !stopped || f.hits[6] == 0 {
			t.Fatalf("FF 0: a non-colonist in the line is unscaled: hits %v", f.hits)
		}
		if stopped, _ := passes(f, c, aimed, 14, 5, 400); stopped {
			t.Fatal("FF 0: a non-colonist is missed at 400")
		}
	}
	// a turret's round is never scaled, even with its id on the roster
	f, c := setup(0)
	turret := rifleShot(50, 2, 2, 5, 30, 5, true)
	turret.FromBuilding = true
	if stopped, _ := passes(f, c, turret, 14, 5, 399); !stopped {
		t.Fatal("a turret's round crossing a colonist is unscaled")
	}
	// land(): an aimed shot whose target stepped away strays into the
	// colonist in the landing cell: 500 x FF %; at 0 nothing is rolled, the
	// sandbags there included (N3)
	for _, tc := range []struct{ pct, threshold int }{{0, 0}, {50, 250}, {100, attack.StrayHit}} {
		f, c := setup(tc.pct)
		f.pos[3] = [2]int{30, 8} // the target is at (30, 5): 3 > Tracking
		f.build(9, construction.Sandbags, 30, 8)
		sh := rifleShot(1, 2, 2, 5, 30, 8, true)
		if tc.threshold == 0 {
			f.rolls, f.rolled = []int{0}, 0
			c.Shots.land(&sh, 30, 8)
			if f.rolled != 0 || f.hits[3] != 0 || f.damage[9] != 0 || landed(f).Hit {
				t.Fatalf("FF 0 stray: %d rolls, hits %v, damage %v", f.rolled, f.hits, f.damage)
			}
			continue
		}
		f.rolls, f.rolled = []int{tc.threshold - 1}, 0
		c.Shots.land(&sh, 30, 8)
		if f.rolled != 1 || f.hits[3] == 0 {
			t.Fatalf("FF %d stray at %d: %d rolls, hits %v", tc.pct, tc.threshold-1, f.rolled, f.hits)
		}
		before := f.hits[3]
		f.rolls, f.rolled = []int{tc.threshold}, 0
		c.Shots.land(&sh, 30, 8)
		if f.rolled != 1 || f.hits[3] != before || f.damage[9] != 0 {
			t.Fatalf("FF %d stray at %d: %d rolls, hits %v", tc.pct, tc.threshold, f.rolled, f.hits)
		}
	}
}

// TestFriendlyFireFullEqualsNoRules: at 100 % a burst of shots draws the
// same rolls and leaves the same traces as a game without rules.
func TestFriendlyFireFullEqualsNoRules(t *testing.T) {
	t.Parallel()
	run := func(r Rules) (int, []projectile.Trace, map[entity.EntityID]int) {
		f := newFake(40, 12)
		c := f.ruled(r)
		f.pawn(1, 2, 5, weapon.Rifle)
		f.raider(2, 30, 5, weapon.Fists)
		f.pawn(3, 14, 5, weapon.Fists)
		f.pawn(4, 30, 6, weapon.Fists)
		for _, rolls := range [][]int{{0, 399}, {0, 400}, {999, 1, 0, 499}, {999, 0, 0, 0}, {0, 0}} {
			f.rolls = rolls
			c.Strikes.Strike(1, 2, weapon.Rifle)
			settle(c)
		}
		return f.rolled, f.traces, f.hits
	}
	n0, tr0, h0 := run(nil)
	n1, tr1, h1 := run(fakeRules{pct: 100, col: map[entity.EntityID]bool{1: true, 3: true, 4: true}})
	if n0 != n1 || !reflect.DeepEqual(tr0, tr1) || !reflect.DeepEqual(h0, h1) {
		t.Fatalf("FF 100 differs from no rules: %d vs %d rolls, hits %v vs %v", n0, n1, h0, h1)
	}
}

// TestDownedTargetFactor (SHT-008): a ranged report on a downed target
// carries DownedFactor by distance, for a pawn and a turret alike; a
// standing target and a melee blow carry none.
func TestDownedTargetFactor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		dist int
		want attack.Permille
	}{{3, 8500}, {4, 1000}, {5, 500}} {
		for _, kind := range []string{"humanlike", "animal"} {
			f := newFake(40, 10)
			c := f.combat()
			f.pawn(1, 2, 5, weapon.Rifle)
			if kind == "humanlike" {
				f.pos[3], f.lying[3] = [2]int{2 + tc.dist, 5}, true
			} else {
				f.beast(3, 2+tc.dist, 5, animal.State{Species: animal.Deer, Downed: true})
			}
			r, ok := c.Strikes.Report(1, 3)
			if !ok || r.Downed != tc.want || r.Chance != r.Total() || r.Chance > 1000 || r.Melee {
				t.Fatalf("%s at %d: ok %v %+v", kind, tc.dist, ok, r)
			}
			if tr := c.Shots.turretReport(weapon.DefOf(weapon.SentryGun), 2, 5, 3, 2+tc.dist, 5); tr.Downed != tc.want || tr.Chance != tr.Total() {
				t.Fatalf("turret on %s at %d: %+v", kind, tc.dist, tr)
			}
		}
	}
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 5, 5, weapon.Fists)
	if r, _ := c.Strikes.Report(1, 2); r.Downed != 0 {
		t.Fatalf("a standing target: %+v", r)
	}
	if tr := c.Shots.turretReport(weapon.DefOf(weapon.SentryGun), 2, 5, 2, 5, 5); tr.Downed != 0 {
		t.Fatalf("a turret on a standing target: %+v", tr)
	}
	f.weapons[1] = weapon.Knife
	f.pos[3], f.lying[3] = [2]int{3, 5}, true
	if r, ok := c.Strikes.Report(1, 3); !ok || !r.Melee || r.Downed != 0 {
		t.Fatalf("melee on a downed target is unaffected: %+v", r)
	}
}

// TestReportDownedTarget (SHT-008, B2): the hit card resolves a downed
// humanlike, never a dead one, and the shot fired at it rolls against the
// very chance the card shows.
func TestReportDownedTarget(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.pos[3], f.lying[3] = [2]int{7, 5}, true // 5 tiles: x0.5
	r, ok := c.Strikes.Report(1, 3)
	if !ok || r.Downed != 500 {
		t.Fatalf("downed humanlike: ok %v %+v", ok, r)
	}
	aim := int(r.Aim())
	f.rolls = []int{aim - 1}
	c.Strikes.Strike(1, 3, weapon.Rifle)
	f.rolls = []int{aim}
	c.Strikes.Strike(1, 3, weapon.Rifle)
	var fired []bool
	for _, tr := range f.traces {
		if tr.Stage == projectile.Fired {
			fired = append(fired, tr.Hit)
		}
	}
	if !reflect.DeepEqual(fired, []bool{true, false}) {
		t.Fatalf("the shot's aim is the card's %d: aimed %v", aim, fired)
	}
	f.lying[3] = false // dead: neither standing nor lying
	if _, ok := c.Strikes.Report(1, 3); ok {
		t.Fatal("a dead humanlike gets no card")
	}
	f.beast(4, 7, 6, animal.State{Species: animal.Deer, Injury: 1 << 20})
	if _, ok := c.Strikes.Report(1, 4); ok {
		t.Fatal("a dead animal gets no card")
	}
	if _, _, ok := c.Strikes.Target(3); ok {
		t.Fatal("AI target picks still skip the downed")
	}
}

// TestFirstAimScales (SHT-102): every first aim of a gun is its warmup
// scaled by the shooter's aiming time, at every site that starts one;
// melee is never scaled; no aiming stat (identity) is the warmup.
func TestFirstAimScales(t *testing.T) {
	t.Parallel()
	rifle, knife := weapon.DefOf(weapon.Rifle), weapon.DefOf(weapon.Knife)
	half := rifle.Warmup * 500 / 1000
	f := newFake(80, 20)
	f.aimMul[1] = 500 // quick draw
	if got := AimTicks(f, 1, rifle); got != half || half == rifle.Warmup {
		t.Fatalf("quick draw rifle aim %d, want %d", got, half)
	}
	if got := AimTicks(f, 1, knife); got != knife.Warmup {
		t.Fatalf("melee aim %d, want %d", got, knife.Warmup)
	}
	if got := AimTicks(f, 2, rifle); got != rifle.Warmup {
		t.Fatalf("no aiming stat: %d, want %d", got, rifle.Warmup)
	}
	j := job.Record{Count: 1}
	if got := f.combat().Strikes.next(1, rifle, &j); got != rifle.Cooldown+half {
		t.Fatalf("next aim %d, want %d", got, rifle.Cooldown+half)
	}
	// AutoEngage (engage.go)
	c := f.combat()
	f.pawn(1, 10, 10, weapon.Rifle)
	f.raider(4, 15, 10, weapon.Fists)
	c.Strikes.AutoEngage(1)
	if len(f.started) != 1 || f.started[0].WorkLeft != half {
		t.Fatalf("auto-engage: %+v", f.started)
	}
	// Guard.Defend (defend.go)
	f2 := newFake(40, 20)
	f2.aimMul[1] = 500
	f2.pawn(1, 10, 10, weapon.Rifle)
	f2.beast(2, 12, 10, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
	if !f2.combat().Guard.Defend(1, job.Record{}) || len(f2.started) != 1 || f2.started[0].WorkLeft != half {
		t.Fatalf("defend: %+v", f2.started)
	}
	// Hunting.Assign (hunt.go)
	f3 := newFake(40, 20)
	f3.aimMul[1], f3.marked = 500, true
	f3.pawn(1, 10, 10, weapon.Rifle)
	f3.beast(2, 20, 10, animal.State{Species: animal.Deer, Designated: true})
	if !f3.combat().Hunting.Assign(1) || len(f3.started) != 1 || f3.started[0].WorkLeft != half {
		t.Fatalf("hunt: %+v", f3.started)
	}
}

// TestLowestInterceptor: the cell's choice whatever Near's order: the
// lowest-id standing creature before any downed one, else the lowest-id
// downed one; nothing strikable is no pick.
func TestLowestInterceptor(t *testing.T) {
	t.Parallel()
	type in struct {
		id entity.EntityID
		r  reach
	}
	for _, tc := range []struct {
		name   string
		adds   []in
		v      entity.EntityID
		downed bool
		ok     bool
	}{
		{"empty", nil, 0, false, false},
		{"none strikable", []in{{4, reachNone}}, 0, false, false},
		{"standing low then high", []in{{3, reachStanding}, {5, reachStanding}}, 3, false, true},
		{"standing high then low", []in{{5, reachStanding}, {3, reachStanding}}, 3, false, true},
		{"downed high then low", []in{{7, reachDowned}, {4, reachDowned}}, 4, true, true},
		{"downed low then high", []in{{4, reachDowned}, {7, reachDowned}}, 4, true, true},
		{"standing beats a lower downed", []in{{2, reachDowned}, {9, reachStanding}, {1, reachNone}}, 9, false, true},
	} {
		var lo lowest
		for _, a := range tc.adds {
			lo.add(a.id, a.r)
		}
		if v, downed, ok := lo.pick(); v != tc.v || downed != tc.downed || ok != tc.ok {
			t.Errorf("%s: %d %v %v, want %d %v %v", tc.name, v, downed, ok, tc.v, tc.downed, tc.ok)
		}
	}
}

// TestInterceptorSkipsTurret: a turret reported in a cell is never the
// creature a shot strikes there (it is attackable, but not in flight).
func TestInterceptorSkipsTurret(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.build(4, construction.SentryTurret, 14, 5)
	f.pos[4] = [2]int{14, 5}
	if _, _, ok := c.Strikes.Target(4); !ok {
		t.Fatal("the test assumes a turret is an attack target")
	}
	if v, _, ok := c.Shots.interceptor(14, 5, 1); ok {
		t.Fatalf("a turret intercepted: %d", v)
	}
	f.beast(7, 14, 5, animal.State{Species: animal.Deer})
	if v, downed, ok := c.Shots.interceptor(14, 5, 1); !ok || downed || v != 7 {
		t.Fatalf("the deer beside the turret: %d %v %v", v, downed, ok)
	}
}
