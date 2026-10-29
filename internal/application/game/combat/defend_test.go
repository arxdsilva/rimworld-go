package combat

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
)

func TestDefendArmedFightsBack(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	g := f.combat().Guard
	f.pawn(1, 10, 10, weapon.Rifle)
	if g.Defend(1, job.Record{}) {
		t.Fatal("no threat: nothing to do")
	}
	f.beast(2, 12, 10, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
	if !g.Defend(1, job.Record{}) || len(f.started) != 1 || f.started[0].Kind != job.Attack || f.started[0].Target != 2 || f.interr != 1 {
		t.Fatalf("armed: attack the wolf, started %+v", f.started)
	}
	if !g.Defend(1, f.jobs[1]) || len(f.started) != 1 {
		t.Fatal("already fighting the threat: carry on")
	}
	if f.checks == 0 {
		t.Fatal("every scan is cross-checked")
	}
}

// TestDefendSwitchDoesNotFlap: an armed colonist fighting one attacker only
// turns on another at least SwitchMargin tiles closer.
func TestDefendSwitchDoesNotFlap(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	g := f.combat().Guard
	f.pawn(1, 10, 10, weapon.Rifle)
	f.raider(2, 18, 10, weapon.Rifle)
	f.jobs[2] = job.Record{Kind: job.Attack, Target: 1, State: job.Do}
	f.raider(3, 10, 17, weapon.Rifle)
	f.jobs[3] = job.Record{Kind: job.Attack, Target: 1, State: job.Do}
	cur := job.Record{Kind: job.Attack, Target: 2, State: job.Do}
	for i := 0; i < 50; i++ {
		f.pos[3] = [2]int{10, 17 - i%2} // 7 then 6 off against 8: never SwitchMargin closer
		if !g.Defend(1, cur) || len(f.started) != 0 {
			t.Fatalf("tick %d: switched targets: %+v", i, f.started)
		}
	}
	f.pos[3] = [2]int{10, 15}
	g.Defend(1, cur)
	if len(f.started) != 1 || f.started[0].Target != 3 {
		t.Fatalf("a threat SwitchMargin closer wins: %+v", f.started)
	}
}

func TestDefendUnarmedFleesAndCalms(t *testing.T) {
	t.Parallel()
	f := newFake(60, 30)
	g := f.combat().Guard
	f.pawn(1, 30, 15, weapon.Fists)
	f.beast(2, 30+FleeReach-1, 15, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
	if !g.Defend(1, job.Record{}) {
		t.Fatal("unarmed: run")
	}
	if len(f.started) != 1 || f.started[0].Kind != job.Move || f.started[0].TX >= 30 || f.dropped != 1 {
		t.Fatalf("flee west: %+v", f.started)
	}
	if f.calm[1] != f.now+FleeCalm || len(f.logs) != 1 || f.logs[0] != "fled 1 2" {
		t.Fatalf("calm %d logs %v", f.calm[1], f.logs)
	}
	if !g.InCombat(1) {
		t.Fatal("a shaken colonist is in combat")
	}
	// running: keep going between re-plans; the log is written once
	run := f.started[0]
	run.State = job.Goto
	f.now++
	if !g.Defend(1, run) || len(f.started) != 1 || len(f.logs) != 1 {
		t.Fatal("keep running")
	}
	// the threat out past FleeRelease: still shaken, holding back
	f.pos[2] = [2]int{30 + FleeRelease + 1, 15}
	if !g.Defend(1, run) {
		t.Fatal("still calming down: hold back from work")
	}
	f.now += FleeCalm
	if g.Defend(1, run) || g.InCombat(1) {
		t.Fatal("calm again once the window is over")
	}
	if _, ok := f.calm[1]; ok {
		t.Fatal("the expired window is forgotten")
	}
	// cornered: stays put
	f.cornered = true
	f.pos[2] = [2]int{31, 15}
	before := len(f.started)
	if !g.Defend(1, job.Record{}) || len(f.started) != before {
		t.Fatal("cornered: no flight job")
	}
}

func TestNearestThreatTiesAndRaiders(t *testing.T) {
	t.Parallel()
	f := newFake(60, 30)
	g := f.combat().Guard
	f.pawn(1, 30, 15, weapon.Fists)
	f.beast(5, 33, 15, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
	f.beast(4, 27, 15, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
	f.beast(3, 31, 15, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 9})
	if th, x, _, d := g.NearestThreat(1, 30, 15, AnimalReach, RaiderReach); th != 4 || x != 27 || d != 3 {
		t.Fatalf("tie to the lower id: %d at %d (%d)", th, x, d)
	}
	f.raider(7, 30, 17, weapon.Rifle)
	f.jobs[7] = job.Record{Kind: job.Attack, Target: 1, State: job.Do}
	if th, _, _, d := g.NearestThreat(1, 30, 15, AnimalReach, RaiderReach); th != 7 || d != 2 {
		t.Fatalf("a closer raider: %d (%d)", th, d)
	}
	if th, _, _, _ := g.NearestThreat(1, 30, 15, 0, 1); th != 0 {
		t.Fatal("out of reach: none")
	}
	f.raiders[7] = false
	if _, _, _, ok := g.ThreatTo(1, 7, 30, 15, AnimalReach, RaiderReach); ok {
		t.Fatal("a downed raider is no threat")
	}
}

func TestInCombat(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	g := f.combat().Guard
	f.pawn(1, 5, 5, weapon.Fists)
	if g.InCombat(1) {
		t.Fatal("idle")
	}
	f.drafted[1] = true
	if !g.InCombat(1) {
		t.Fatal("drafted")
	}
	f.drafted[1] = false
	f.jobs[1] = job.Record{Kind: job.Attack, State: job.Do}
	if !g.InCombat(1) {
		t.Fatal("attacking")
	}
	f.jobs[1] = job.Record{Kind: job.Attack, State: job.Fail}
	if g.InCombat(1) {
		t.Fatal("a failed attack is over")
	}
}

func TestBreachStartAndBlows(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	br := f.combat().Breach
	f.raider(1, 2, 5, weapon.Rifle)
	f.build(9, construction.Wall, 5, 5)
	f.route = []pathing.Point{{X: 3, Y: 5}, {X: 4, Y: 5}, {X: 5, Y: 5}, {X: 6, Y: 5}}
	if !br.Start(1, 8, 5) {
		t.Fatal("break in through the wall")
	}
	j := f.jobs[1]
	if j.Kind != job.Breach || j.Target != 9 || j.DestX != 5 || j.TX != 4 || j.WorkLeft != BreachCooldown {
		t.Fatalf("breach job %+v", j)
	}
	f.noRoute = true
	if br.Start(1, 8, 5) {
		t.Fatal("no route: no breach")
	}
	f.noRoute = false
	f.route = []pathing.Point{{X: 3, Y: 5}}
	f.blocked[[2]int{3, 5}] = true // a trap: no building, no rock
	if br.Start(1, 8, 5) {
		t.Fatal("a trap in the way: plan again later")
	}
	f.route = []pathing.Point{{X: 3, Y: 6}}
	if br.Start(1, 8, 5) {
		t.Fatal("an open route needs no breach")
	}

	j.State = job.Goto
	br.Do(1, j)
	f.pos[1] = [2]int{2, 5}
	j.State = job.Do
	br.Do(1, j)
	if f.jobs[1].State != job.Fail {
		t.Fatal("not beside the wall: unreachable")
	}
	f.pos[1] = [2]int{4, 5}
	j.WorkLeft = 1
	br.Do(1, j)
	if f.jobs[1].WorkLeft != 0 || f.damage[9] != 0 {
		t.Fatal("cooling down")
	}
	j.WorkLeft = 0
	br.Do(1, j)
	if f.damage[9] != buttDamage || f.jobs[1].WorkLeft != BreachCooldown || f.struck[1] != 0 {
		t.Fatalf("a rifle butt: %v", f.damage)
	}
	f.weapons[1] = weapon.Knife
	br.Do(1, j)
	if f.damage[9] != buttDamage+weapon.DefOf(weapon.Knife).Damage {
		t.Fatalf("a knife: %v", f.damage)
	}
	delete(f.bldgs, 9)
	br.Do(1, j)
	if f.jobs[1].State != job.Complete {
		t.Fatal("the wall is down")
	}
}

func TestSapperDigs(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	br := f.combat().Breach
	f.raider(1, 4, 5, weapon.Fists)
	f.rock[[2]int{5, 5}] = true
	f.route = []pathing.Point{{X: 5, Y: 5}}
	if !br.Start(1, 8, 5) || f.jobs[1].Target != 0 {
		t.Fatal("dig through rock")
	}
	j := job.Record{Kind: job.Breach, State: job.Goto, DestX: 5, DestY: 5}
	br.Do(1, j)
	j.State = job.Do
	j.WorkLeft = 1
	br.Do(1, j)
	if f.dug != 0 {
		t.Fatal("cooling down")
	}
	j.WorkLeft = 0
	br.Do(1, j)
	if f.dug != digMin {
		t.Fatalf("dug %d, want %d", f.dug, digMin)
	}
	f.pos[1] = [2]int{1, 1}
	br.Do(1, j)
	if f.jobs[1].State != job.Fail {
		t.Fatal("too far to dig")
	}
	delete(f.rock, [2]int{5, 5})
	br.Do(1, j)
	if f.jobs[1].State != job.Complete {
		t.Fatal("the rock is gone")
	}
}
