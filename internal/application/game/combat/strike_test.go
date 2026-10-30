package combat

import (
	"rimworld-go/internal/domain/combat/cover"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/weapon"
)

func TestStrikeHitAndMiss(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.pawn(1, 5, 5, weapon.Knife)
	f.raider(2, 6, 5, weapon.Fists)
	f.rolls = []int{0, 999} // the blade, a miss
	s.Strike(1, 2, weapon.Knife)
	if len(f.hurts) != 0 || len(f.traces) != 1 || f.traces[0].Hit || f.struck[1] != 2 || f.learn != 1 {
		t.Fatalf("miss: hurts %v traces %+v", f.hurts, f.traces)
	}
	f.rolls = []int{0, 0, 200} // the blade, a hit, x1.0 damage
	s.Strike(1, 2, weapon.Knife)
	d := weapon.DefOf(weapon.Knife)
	if len(f.hurts) != 1 || f.hits[2] != d.Damage || f.stains != 1 || !f.traces[1].Hit || f.traces[1].Damage != d.Damage || !f.traces[1].Melee {
		t.Fatalf("hit: hurts %v traces %+v", f.hurts, f.traces)
	}
	s.Strike(1, 99, weapon.Knife) // no such target
	if len(f.traces) != 2 {
		t.Fatal("a vanished target takes no strike")
	}
}

func TestStrikeRollUsesItsRoll(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.pawn(1, 5, 5, weapon.Fists)
	f.pawn(2, 6, 5, weapon.Fists)
	n := 0
	s.StrikeRoll(1, 2, weapon.Fists, func(int) int { n++; return 0 })
	if n != 3 || len(f.hurts) != 1 || len(f.rolls) != 0 {
		t.Fatalf("the given roll decides the tool, the hit and the damage: %d draws", n)
	}
}

func TestAnimalStrikesWithItsNaturalWeapon(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.beast(1, 5, 5, animal.State{Species: animal.TimberWolf})
	f.pawn(2, 6, 5, weapon.Fists)
	s.Strike(1, 2, weapon.Fists)
	if want := attack.MeleeDamage(animal.DefOf(animal.TimberWolf).Damage, func(int) int { return 0 }); f.hits[2] != want || f.learn != 0 {
		t.Fatalf("wolf bite %d, want %d (and no skill learnt)", f.hits[2], want)
	}
}

func TestRangedStrikeAgainstCoverAndTurret(t *testing.T) {
	t.Parallel()
	f := newFake(30, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 10, 5, weapon.Fists)
	f.build(3, construction.Sandbags, 9, 5)
	d := weapon.DefOf(weapon.Rifle)
	cp := cover.Permille(cover.Sandbags)
	f.rolls = []int{0, cp - 1} // the aim holds, the sandbags take it
	c.Strikes.Strike(1, 2, weapon.Rifle)
	for c.Shots.Len() > 0 {
		c.Shots.Step()
	}
	if len(f.hurts) != 0 || f.damage[3] != d.Damage {
		t.Fatalf("the cover takes the shot: hurts %v damage %v", f.hurts, f.damage)
	}
	f.rolls = []int{0, cp} // the aim holds, the cover lets it through
	c.Strikes.Strike(1, 2, weapon.Rifle)
	for c.Shots.Len() > 0 {
		c.Shots.Step()
	}
	if len(f.hurts) != 1 || f.damage[3] != d.Damage {
		t.Fatalf("past the cover: hurts %v damage %v", f.hurts, f.damage)
	}
	if r, ok := c.Strikes.Report(1, 2); !ok || r.Cover != cp || r.Chance != r.Aim()*(1000-cp)/1000 {
		t.Fatalf("report %+v", r)
	}
	f.build(4, construction.SentryTurret, 12, 5)
	f.raider(5, 13, 5, weapon.Fists)
	c.Strikes.Strike(5, 4, weapon.Fists)
	if f.damage[4] != weapon.DefOf(weapon.Fists).Damage || !f.traces[len(f.traces)-1].Building || f.struck[5] != 4 {
		t.Fatalf("a blow on a turret damages the building: %v", f.damage)
	}
	f.rolls = []int{999}
	c.Strikes.Strike(5, 4, weapon.Fists)
	if f.damage[4] != weapon.DefOf(weapon.Fists).Damage {
		t.Fatal("a miss leaves the turret alone")
	}
}

func TestStrikeTarget(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.beast(1, 3, 3, animal.State{Species: animal.Deer})
	f.beast(2, 4, 4, animal.State{Species: animal.Deer, Injury: 1 << 20})
	f.pawn(3, 5, 5, weapon.Fists)
	f.build(4, construction.SentryTurret, 7, 7)
	f.build(5, construction.Wall, 8, 8)
	for _, c := range []struct {
		id   uint64
		x, y int
		ok   bool
	}{{1, 3, 3, true}, {2, 0, 0, false}, {3, 5, 5, true}, {4, 7, 7, true}, {5, 0, 0, false}, {9, 0, 0, false}} {
		x, y, ok := s.Target(entityID(c.id))
		if ok != c.ok || x != c.x || y != c.y {
			t.Errorf("target %d: %d,%d %v", c.id, x, y, ok)
		}
	}
}

func TestDoClosesInThenStrikesOnCooldown(t *testing.T) {
	t.Parallel()
	f := newFake(30, 10)
	s := f.combat().Strikes
	f.pawn(1, 2, 5, weapon.Knife)
	f.raider(2, 10, 5, weapon.Fists)
	j := job.Record{Kind: job.Attack, Target: 2, State: job.Goto}
	s.Do(1, j, 1000)
	if j = f.jobs[1]; j.State != job.Goto || j.TX != 10 || len(j.Path) != 1 {
		t.Fatalf("closing in: %+v", j)
	}
	f.pos[1] = [2]int{9, 5}
	j.WorkLeft = 1
	s.Do(1, j, 1001)
	if j = f.jobs[1]; j.State != job.Do || len(f.hurts) != 1 || j.WorkLeft != weapon.ToolsOf(weapon.Knife)[0].Cooldown {
		t.Fatalf("in reach the blow lands and the cooldown starts: %+v hurts %v", j, f.hurts)
	}
	s.Do(1, j, 1002)
	if j = f.jobs[1]; len(f.hurts) != 1 || j.WorkLeft != weapon.ToolsOf(weapon.Knife)[0].Cooldown-1 {
		t.Fatalf("on cooldown: %+v", j)
	}
	f.pos[1] = [2]int{2, 5}
	f.noPath = true
	s.Do(1, j, 1003)
	if j = f.jobs[1]; j.State != job.Fail || j.Fail != job.Unreachable {
		t.Fatalf("unreachable: %+v", j)
	}
	f.raiders[2] = false // down: no longer a target
	s.Do(1, j, 1004)
	if j = f.jobs[1]; j.State != job.Complete || f.cleared != 1 {
		t.Fatalf("target gone: %+v", j)
	}
	f.pacific[1] = true
	f.raiders[2] = true
	s.Do(1, job.Record{Kind: job.Attack, Target: 2}, 1005)
	if f.jobs[1].State != job.Complete {
		t.Fatal("a pacifist never attacks")
	}
}

func TestDoHunt(t *testing.T) {
	t.Parallel()
	f := newFake(30, 10)
	c := f.combat()
	s := c.Strikes
	f.pawn(1, 2, 5, weapon.Rifle)
	f.beast(2, 8, 5, animal.State{Species: animal.Deer})
	s.Do(1, job.Record{Kind: job.Hunt, Target: 2}, 1000)
	if f.jobs[1].State != job.Complete {
		t.Fatal("an undesignated animal is not hunted")
	}
	f.animals[2] = animal.State{Species: animal.Deer, Designated: true}
	f.givenUp = true
	s.Do(1, job.Record{Kind: job.Hunt, Target: 2}, 1000)
	if f.abandoned != 1 {
		t.Fatal("a hunt given up is abandoned")
	}
	f.givenUp = false
	s.Do(1, job.Record{Kind: job.Hunt, Target: 2}, 1000)
	if j := f.jobs[1]; j.State != job.Do || c.Shots.Len() != 1 || len(f.hurts) != 0 {
		t.Fatalf("the hunter shoots from range: %+v", j)
	}
	d := weapon.DefOf(weapon.Rifle)
	if j := f.jobs[1]; j.WorkLeft != d.Cooldown+d.Warmup {
		t.Fatalf("cooldown then the next aim: %d", j.WorkLeft)
	}
	c.Shots.Step() // 1166 milli-cells a tick: 6 tiles take 6 ticks
	for i := 0; i < 5; i++ {
		if len(f.hurts) != 0 {
			t.Fatal("the bullet is still flying")
		}
		c.Shots.Step()
	}
	if len(f.hurts) != 1 || c.Shots.Len() != 0 {
		t.Fatalf("the bullet lands: %v", f.hurts)
	}
}

func TestAutoEngagePicksNearestHostile(t *testing.T) {
	t.Parallel()
	f := newFake(80, 20)
	s := f.combat().Strikes
	f.pawn(1, 10, 10, weapon.Rifle)
	f.beast(2, 13, 10, animal.State{Species: animal.TimberWolf}) // not attacking
	f.beast(3, 16, 10, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
	f.raider(4, 15, 10, weapon.Fists)
	f.raider(5, 11, 10, weapon.Fists)
	f.raiders[5] = false // downed
	s.AutoEngage(1)
	if len(f.started) != 1 || f.started[0].Target != 4 || f.started[0].WorkLeft != AimTicks(weapon.DefOf(weapon.Rifle)) {
		t.Fatalf("started %+v", f.started)
	}
	f.raider(6, 47, 10, weapon.Fists) // 37 tiles: one past the rifle's range
	delete(f.raiders, 4)
	f.pos[3] = [2]int{79, 19}
	s.AutoEngage(1)
	if len(f.started) != 1 {
		t.Fatal("nothing in range: no attack")
	}
	f.pacific[1] = true
	f.pos[6] = [2]int{12, 10}
	s.AutoEngage(1)
	if len(f.started) != 1 {
		t.Fatal("a pacifist never engages")
	}
}
