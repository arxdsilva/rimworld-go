package combat

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// hostileRig is a settlement's sentry turret (id 50) at (10, 5) on a map
// with no power device (MAP-13).
func hostileRig(t *testing.T) (*fakeHost, *Combat) {
	t.Helper()
	f := newFake(60, 60)
	f.build(50, construction.SentryTurret, 10, 5)
	makeHostile(f, 50)
	f.armed, f.defense, f.gen = true, []entity.EntityID{50}, 1
	return f, f.combatArmed()
}

// makeHostile makes building id a settlement's.
func makeHostile(f *fakeHost, id entity.EntityID) {
	b := f.bldgs[id]
	b.Hostile, b.Ancient = true, true
	f.bldgs[id] = b
}

// scanOnce runs the turrets on turret 50's next scan tick.
func scanOnce(f *fakeHost, c *Combat) {
	f.now = scanTick(f.now + 1)
	c.Turrets.Phase(f.now)
}

// TestHostileTurretFiresWithoutPower: no device on the map, no power, and
// it locks on, aims and lands a round on a colonist; the hit names no
// attacker.
func TestHostileTurretFiresWithoutPower(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	tu := c.Turrets
	f.pawn(1, 10, 12, weapon.Rifle)
	scanOnce(f, c)
	a := tu.Aim(50)
	if a.Target != 1 {
		t.Fatalf("locked on: %+v", a)
	}
	for f.now < a.ReadyAt {
		f.now++
		tu.Phase(f.now)
	}
	if len(f.traces) != 1 || !f.traces[0].FromBuilding || c.Shots.Len() != 1 {
		t.Fatalf("fired once aimed: %+v", f.traces)
	}
	fly(c)
	if len(f.hurts) != 1 || f.logs[0] != "shot 1 true "+TurretLogName {
		t.Fatalf("a round landed: %v %v", f.hurts, f.logs)
	}
	if len(f.struck) != 0 {
		t.Fatalf("a building strikes in nobody's name: %v", f.struck)
	}
}

// TestHostileTurretPicksTheColonyOnly: raiders, visitors, prisoners and wild
// animals hunting nobody in particular are its side or none of its business;
// colonists, tame animals and manhunters (wiki: Mini-turret) are targets,
// nearest first.
func TestHostileTurretPicksTheColonyOnly(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	tu := c.Turrets
	f.guests = map[entity.EntityID]bool{4: true}
	f.raider(1, 10, 7, weapon.Rifle)
	f.pos[4] = [2]int{10, 8}
	f.beast(2, 11, 7, animal.State{Species: animal.Deer})
	f.beast(3, 12, 7, animal.State{Species: animal.TimberWolf})
	f.beast(5, 13, 7, animal.State{Species: animal.Deer, Tame: true, Injury: 1 << 20}) // dead
	scanOnce(f, c)
	if tu.Firing(50) {
		t.Fatalf("no target for it here: %+v", tu.Aim(50))
	}
	f.pawn(6, 10, 15, weapon.Rifle)
	f.beast(7, 10, 14, animal.State{Species: animal.Deer, Tame: true})
	scanOnce(f, c)
	if tu.Aim(50).Target != 7 {
		t.Fatalf("the nearer of colonist and tame animal: %+v", tu.Aim(50))
	}
	f2, c2 := hostileRig(t)
	f2.beast(3, 12, 7, animal.State{Species: animal.TimberWolf, Manhunter: f2.now + 100000})
	scanOnce(f2, c2)
	if c2.Turrets.Aim(50).Target != 3 {
		t.Fatalf("a manhunter is its enemy too: %+v", c2.Turrets.Aim(50))
	}
	delete(f.pos, 7)
	delete(f.animals, 7)
	f.now++
	tu.Phase(f.now)
	scanOnce(f, c)
	if tu.Aim(50).Target != 6 {
		t.Fatalf("then the colonist: %+v", tu.Aim(50))
	}
}

// TestHostileTurretLetsGoByItsOwnRule: a held target that stops being a
// target by this turret's rule is dropped: downed, or no longer on the
// colonists' roster (a visitor stands up just the same).
func TestHostileTurretLetsGoByItsOwnRule(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*fakeHost){
		"downed":  func(f *fakeHost) { f.pawns[1] = false },
		"visitor": func(f *fakeHost) { f.pawns[1], f.guests = false, map[entity.EntityID]bool{1: true} },
	} {
		f, c := hostileRig(t)
		f.pawn(1, 10, 12, weapon.Rifle)
		scanOnce(f, c)
		if !c.Turrets.Firing(50) {
			t.Fatalf("%s: locks on first", name)
		}
		change(f)
		f.now++
		c.Turrets.Phase(f.now)
		if c.Turrets.Firing(50) {
			t.Fatalf("%s: let go", name)
		}
	}
}

// TestPlayerTurretKeepsItsRules: a settlement's turret on the map wakes
// nothing of ours: an unpowered one of ours stays dark, a powered one goes
// on shooting raiders and leaves colonists alone.
func TestPlayerTurretKeepsItsRules(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	tu := c.Turrets
	f.build(50+TurretScanEvery, construction.SentryTurret, 40, 40) // same scan tick as 50
	mine := entity.EntityID(50 + TurretScanEvery)
	f.defense = []entity.EntityID{50, mine}
	f.gen++
	f.pawn(1, 40, 44, weapon.Rifle)
	f.raider(2, 40, 46, weapon.Rifle)
	scanOnce(f, c)
	if tu.Firing(mine) {
		t.Fatal("no power, no device: ours stays dark")
	}
	f.wired, f.powered[mine] = true, true
	scanOnce(f, c)
	if tu.Aim(mine).Target != 2 {
		t.Fatalf("powered, ours shoots the raider, not the nearer colonist: %+v", tu.Aim(mine))
	}
}

// TestHostileTurretIsNotARaiderTarget: however wired and powered the map
// says it is, raiders and defenders never turn on their own turret, and
// allies never get it as a raider's target.
func TestHostileTurretIsNotARaiderTarget(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	f.wired, f.powered[50] = true, true
	if _, _, _, ok := c.Turrets.ForRaider(10, 13, 40, 20); ok {
		t.Fatal("a settlement's turret is on the raiders' side")
	}
}

// TestHostileTurretShotSides: its round spares a defender near the muzzle
// (the raiders' side) and can strike a colonist in the line; ours spares
// the colonist and strikes the raider.
func TestHostileTurretShotSides(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	f.pawn(1, 10, 12, weapon.Rifle)
	f.raider(2, 10, 7, weapon.Rifle)
	c.Shots.Turret(50, weapon.SentryGun, 10, 5, 1, 10, 12)
	if !c.Shots.Export()[0].Raider {
		t.Fatal("a settlement's round is on the raiders' side")
	}
	fly(c)
	if len(f.hurts) != 1 || f.hits[2] != 0 {
		t.Fatalf("the defender in the line is spared: %v %v", f.hurts, f.logs)
	}
	f.hurts, f.logs = nil, nil
	f.pos[2] = [2]int{10, 20} // the raider steps away; a second colonist in the line
	f.pawn(3, 10, 7, weapon.Rifle)
	c.Shots.Turret(50, weapon.SentryGun, 10, 5, 1, 10, 12)
	fly(c)
	if len(f.logs) == 0 || f.logs[0] != "shot 3 true "+TurretLogName {
		t.Fatalf("a colonist in the line is no friend of it: %v", f.logs)
	}
	f.logs = nil
	f.build(51, construction.SentryTurret, 30, 5)
	c.Shots.Turret(51, weapon.SentryGun, 30, 5, 2, 10, 20)
	if c.Shots.Export()[0].Raider {
		t.Fatal("our round stays on our side")
	}
	fly(c)
}

// TestHostileTurretSpentStaysQuiet: a spent barrel silences it, and nobody
// comes to rearm it even with steel and a free hand on the map; a turret
// of ours beside it gets its haul.
func TestHostileTurretSpentStaysQuiet(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	tu := c.Turrets
	f.build(51, construction.SentryTurret, 40, 40)
	f.wired, f.powered[51], f.defense = true, true, []entity.EntityID{50, 51}
	f.gen++
	tu.arms = map[entity.EntityID]Arm{50: {Fired: SentryBarrel}, 51: {Fired: SentryBarrel}}
	f.stock[600] = item.Stack{Kind: item.Steel, Qty: 30, X: 2, Y: 2}
	f.pawn(1, 12, 5, weapon.Fists)
	scanOnce(f, c)
	if tu.Firing(50) {
		t.Fatal("spent: it goes quiet")
	}
	f.now = 250 * 10
	tu.saved.rearms.phase(f.now)
	if rs := tu.saved.rearms; rs.Hauler(50) != 0 || rs.Hauler(51) != 1 {
		t.Fatalf("hauls: %+v", rs.jobs)
	}
}

// TestHostileTurretTakesNoOrders: hold fire does nothing on it, and still
// works on ours.
func TestHostileTurretTakesNoOrders(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	f.build(51, construction.SentryTurret, 20, 5)
	c.Orders.Apply(command.Command{Kind: command.SetTurretHold, X: 10, Y: 5, Enabled: true})
	c.Orders.Apply(command.Command{Kind: command.SetTurretHold, X: 20, Y: 5, Enabled: true})
	if c.Turrets.Held(50) {
		t.Fatal("a settlement's turret holds fire for nobody")
	}
	if !c.Turrets.Held(51) {
		t.Fatal("ours still holds")
	}
	f.pawn(1, 10, 12, weapon.Rifle)
	scanOnce(f, c)
	if !c.Turrets.Firing(50) {
		t.Fatal("the refused order left it firing")
	}
}

// TestOrderedAttackGoesForIt: the order finds the animal, then the raider,
// then a settlement's turret; never a turret of ours. The strike lands on
// the building.
func TestOrderedAttackGoesForIt(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	st := c.Strikes
	f.build(51, construction.SentryTurret, 20, 5)
	f.raider(2, 10, 5, weapon.Rifle)
	f.beast(3, 10, 5, animal.State{Species: animal.Deer})
	if st.OrderedAt(3, 2, 10, 5) != 3 || st.OrderedAt(0, 2, 10, 5) != 2 || st.OrderedAt(0, 0, 10, 5) != 50 {
		t.Fatal("animal, then raider, then the turret")
	}
	if st.OrderedAt(0, 0, 20, 5) != 0 || st.OrderedAt(0, 0, 30, 30) != 0 {
		t.Fatal("ours or nothing: no target")
	}
	f.pawn(1, 10, 12, weapon.Rifle)
	f.drafted[1] = true
	j := job.Record{Kind: job.Attack, Target: 50, TX: 10, TY: 5, Forced: true}
	st.Do(1, j, f.now)
	if f.damage[50] == 0 || f.jobs[1].Kind != job.Attack || f.jobs[1].State != job.Do {
		t.Fatalf("struck: %v job %+v", f.damage, f.jobs[1])
	}
}

// TestAutoEngageSettlementTurret: fire at will shoots a settlement's turret
// when nothing nearer calls; it never touches ours or a spent one, and it
// wants the turret SwitchMargin closer than a creature to prefer it.
func TestAutoEngageSettlementTurret(t *testing.T) {
	t.Parallel()
	drafted := func(t *testing.T) (*fakeHost, *Combat) {
		t.Helper()
		f, c := hostileRig(t)
		f.pawn(1, 10, 12, weapon.Rifle)
		f.drafted[1] = true
		return f, c
	}
	t.Run("alone", func(t *testing.T) {
		f, c := drafted(t)
		c.Strikes.AutoEngage(1)
		if j := f.jobs[1]; j.Kind != job.Attack || j.Target != 50 || j.TX != 10 || j.TY != 5 {
			t.Fatalf("job %+v", j)
		}
	})
	t.Run("ours is no target", func(t *testing.T) {
		f := newFake(60, 60)
		c := f.combatArmed()
		f.build(50, construction.SentryTurret, 10, 5)
		f.wired, f.powered[50], f.defense, f.gen = true, true, []entity.EntityID{50}, 1
		f.pawn(1, 10, 12, weapon.Rifle)
		f.drafted[1] = true
		c.Strikes.AutoEngage(1)
		if f.jobs[1].Kind == job.Attack {
			t.Fatalf("job %+v", f.jobs[1])
		}
		f.armed = true // another turret stands on the map; this one is still ours
		c.Strikes.AutoEngage(1)
		if f.jobs[1].Kind == job.Attack {
			t.Fatalf("job %+v", f.jobs[1])
		}
	})
	t.Run("spent", func(t *testing.T) {
		f, c := drafted(t)
		c.Turrets.arms = map[entity.EntityID]Arm{50: {Fired: SentryBarrel}}
		c.Strikes.AutoEngage(1)
		if f.jobs[1].Kind == job.Attack {
			t.Fatalf("job %+v", f.jobs[1])
		}
	})
	t.Run("out of reach", func(t *testing.T) {
		f, c := drafted(t)
		f.pos[1] = [2]int{10, 50}
		c.Strikes.AutoEngage(1)
		if f.jobs[1].Kind == job.Attack {
			t.Fatalf("job %+v", f.jobs[1])
		}
	})
	t.Run("creature nearer", func(t *testing.T) {
		f, c := drafted(t)
		f.raider(2, 10, 18, weapon.Club) // 6 off, the turret 7: inside the margin
		c.Strikes.AutoEngage(1)
		if j := f.jobs[1]; j.Target != 2 {
			t.Fatalf("job %+v", j)
		}
	})
	t.Run("turret nearer inside the margin", func(t *testing.T) {
		f, c := drafted(t)
		f.raider(2, 10, 12+7+SwitchMargin-1, weapon.Club)
		c.Strikes.AutoEngage(1)
		if j := f.jobs[1]; j.Target != 2 {
			t.Fatalf("job %+v", j)
		}
	})
	t.Run("turret nearer by the margin", func(t *testing.T) {
		f, c := drafted(t)
		f.raider(2, 10, 12+7+SwitchMargin, weapon.Club)
		c.Strikes.AutoEngage(1)
		if j := f.jobs[1]; j.Target != 50 {
			t.Fatalf("job %+v", j)
		}
	})
}

// TestThreatenedBySettlementTurret: a turret that can still fire keeps a
// drafted colonist drafted; a spent one, or one of ours, does not.
func TestThreatenedBySettlementTurret(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	if !c.Strikes.Threatened(f.now) {
		t.Fatal("it can fire: a threat")
	}
	c.Turrets.arms = map[entity.EntityID]Arm{50: {Fired: SentryBarrel}}
	if c.Strikes.Threatened(f.now) {
		t.Fatal("spent: no threat")
	}
	c.Turrets.arms = nil
	b := f.bldgs[50]
	b.Hostile = false
	f.bldgs[50] = b
	if c.Strikes.Threatened(f.now) {
		t.Fatal("ours is no threat")
	}
}

// TestHostileTurretDoesNotFlap (MAP-13): the lock and the release sit at
// different ranges: a colonist hovering between them is kept once locked
// and not picked up once let go, with no target toggling tick to tick; the
// colonist's own choice between a turret and a creature between its two
// margins keeps whatever it has.
func TestHostileTurretDoesNotFlap(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	tu := c.Turrets
	lock, release := reachOf(construction.SentryTurret)
	if lock >= release {
		t.Fatalf("lock %d, release %d", lock, release)
	}
	tu.arms = map[entity.EntityID]Arm{}
	f.pawn(1, 10, 5+lock, weapon.Rifle)
	scanOnce(f, c)
	if tu.Aim(50).Target != 1 {
		t.Fatalf("locks on at %d: %+v", lock, tu.Aim(50))
	}
	hold := func(n int, want entity.EntityID) {
		t.Helper()
		for i := 0; i < n; i++ {
			f.now++
			tu.Phase(f.now)
			if got := tu.Aim(50).Target; got != want {
				t.Fatalf("tick %d: target %d, want %d", f.now, got, want)
			}
			delete(tu.arms, 50) // the barrel stays fresh over a long hover
		}
	}
	f.pos[1] = [2]int{10, 5 + release}
	hold(200, 1) // between the two: kept
	f.pos[1] = [2]int{10, 6 + release}
	hold(1, 0) // past release: gone
	f.pos[1] = [2]int{10, 5 + lock + 1}
	hold(200, 0) // between the two: not picked up again
	f.pos[1] = [2]int{10, 5 + lock}
	f.now = scanTick(f.now + 1)
	tu.Phase(f.now)
	if tu.Aim(50).Target != 1 {
		t.Fatalf("locks on again at %d", lock)
	}

	// The colonist's side: a raider hovering inside both margins leaves it
	// on the turret; only SwitchMargin closer does it turn, then it stays.
	f, c = hostileRig(t)
	f.pawn(1, 10, 12, weapon.Rifle) // the turret 7 off
	f.drafted[1] = true
	f.raider(2, 10, 12+7-SwitchMargin+1, weapon.Club) // inside the margin
	j := job.Record{Kind: job.Attack, Target: 50, TX: 10, TY: 5, State: job.Do, WorkLeft: 1 << 20}
	f.jobs[1] = j
	for i := 0; i < 120; i++ {
		f.now++
		c.Strikes.Do(1, f.jobs[1], f.now)
		if got := f.jobs[1]; got.Target != 50 || got.State != job.Do {
			t.Fatalf("tick %d: left the turret: %+v", f.now, got)
		}
	}
	f.pos[2] = [2]int{10, 12 + 7 - SwitchMargin}
	done := false
	for i := 0; i < foeEvery && !done; i++ {
		f.now++
		c.Strikes.Do(1, f.jobs[1], f.now)
		done = f.jobs[1].State == job.Complete
	}
	if !done {
		t.Fatalf("a raider SwitchMargin closer ends the attack: %+v", f.jobs[1])
	}
	f.jobs[1] = job.Record{}
	c.Strikes.AutoEngage(1)
	if f.jobs[1].Target != 2 {
		t.Fatalf("then it turns on the raider: %+v", f.jobs[1])
	}
	for i := 0; i < 120; i++ { // an ordered attack is never dropped
		f.now++
		c.Strikes.Do(1, job.Record{Kind: job.Attack, Target: 50, TX: 10, TY: 5, Forced: true, WorkLeft: 1 << 20}, f.now)
		if f.jobs[1].State == job.Complete {
			t.Fatalf("an order is kept: %+v", f.jobs[1])
		}
	}
}

// TestDestroyedHostileTurretClears: Blast drops its aim and its barrel
// like any turret's.
func TestDestroyedHostileTurretClears(t *testing.T) {
	t.Parallel()
	f, c := hostileRig(t)
	f.pawn(1, 10, 12, weapon.Rifle)
	scanOnce(f, c)
	f.droll = []int{TurretBlastPct} // no blast this time
	b := f.bldgs[50]
	c.Turrets.Blast(50, b)
	if c.Turrets.Firing(50) {
		t.Fatal("gone")
	}
}
