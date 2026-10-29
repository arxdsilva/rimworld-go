package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// holdAttacker pins animal id at (x, y), attacking target, and keeps it
// from thinking or walking, so a test controls the threat's distance.
func holdAttacker(g *Game, id, target entity.EntityID, x, y int) {
	a, ok := g.animals.Get(id)
	if !ok {
		return
	}
	a.Target, a.Mode, a.Path, a.PathI, a.RetryAt, a.NextThink = target, animal.Attack, nil, 0, 1<<40, 1<<40
	g.animals.Set(id, a)
	g.positions.Set(id, Position{X: x, Y: y})
	g.invalidateCreatures()
}

// releaseAttacker makes animal id lose interest and stand still.
func releaseAttacker(g *Game, id entity.EntityID, x, y int) {
	a, _ := g.animals.Get(id)
	a.Target, a.Mode, a.Path, a.PathI, a.NextThink = 0, animal.Wander, nil, 0, 1<<40
	g.animals.Set(id, a)
	g.positions.Set(id, Position{X: x, Y: y})
	g.invalidateCreatures()
}

// fleeing reports a colonist running from (or still shaken by) a threat,
// as the read model shows it.
func fleeing(g *Game, id entity.EntityID) PawnView {
	pv := PawnView{ID: id}
	g.fillFleeView(&pv, id)
	return pv
}

func fullNeeds(g *Game, ids ...entity.EntityID) {
	for _, id := range ids {
		g.SetNeeds(id, needs.Full, needs.Full)
	}
}

func TestUnarmedColonistFleesChargeTenTilesAway(t *testing.T) {
	g := New(Config{Seed: 3, Width: 50, Height: 20})
	ada, _ := g.SpawnNamedPawn(25, 10, "Ada")
	fullNeeds(g, ada)
	wolf, _ := g.SpawnAnimal("ridgewolf", 35, 10)
	holdAttacker(g, wolf, ada, 35, 10) // 10 tiles east, charging
	g.Step(2)
	j, _ := g.jobs.Get(ada)
	if j.Kind != job.Move || j.Forced || j.TX >= 25 {
		t.Fatalf("an unarmed colonist runs west, away from a charge 10 tiles off: job %+v", j)
	}
	pv := fleeing(g, ada)
	if !pv.Fleeing || pv.Job != "Fleeing from "+g.creatureName(wolf) {
		t.Fatalf("the read model shows the flight: fleeing %v job %q", pv.Fleeing, pv.Job)
	}
}

func TestUnarmedColonistFleesManhunterElevenTilesAway(t *testing.T) {
	g := New(Config{Seed: 3, Width: 50, Height: 20})
	ada, _ := g.SpawnNamedPawn(25, 10, "Ada")
	fullNeeds(g, ada)
	wolf, _ := g.SpawnAnimal("ridgewolf", 14, 10)
	a, _ := g.animals.Get(wolf)
	g.enrage(wolf, &a, ada, uint64(g.clock.Now()))
	g.animals.Set(wolf, a)
	holdAttacker(g, wolf, ada, 14, 10) // 11 tiles west
	g.Step(2)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Move || j.TX <= 25 {
		t.Fatalf("an unarmed colonist runs east from a manhunter 11 tiles off: job %+v", j)
	}
	// out of reach: 12 tiles is not a threat yet
	g2 := New(Config{Seed: 3, Width: 50, Height: 20})
	bo, _ := g2.SpawnNamedPawn(25, 10, "Bo")
	fullNeeds(g2, bo)
	w2, _ := g2.SpawnAnimal("ridgewolf", 13, 10)
	for i := 0; i < 5; i++ {
		holdAttacker(g2, w2, bo, 13, 10)
		g2.Step(1)
		fullNeeds(g2, bo)
	}
	if _, ok := g2.calm[bo]; ok || fleeing(g2, bo).Fleeing {
		t.Fatal("a charge 12 tiles off doesn't scare a colonist yet")
	}
}

func TestArmedColonistFightsDraftedHolds(t *testing.T) {
	g := New(Config{Seed: 3, Width: 50, Height: 20})
	ada, _ := g.SpawnNamedPawn(25, 10, "Ada")
	g.Arm(ada, item.Club)
	fullNeeds(g, ada)
	wolf, _ := g.SpawnAnimal("ridgewolf", 35, 10)
	for i := 0; i < 5; i++ {
		holdAttacker(g, wolf, ada, 35, 10)
		g.Step(1)
		fullNeeds(g, ada)
	}
	if _, ok := g.calm[ada]; ok {
		t.Fatal("an armed colonist never flees")
	}
	if j, _ := g.jobs.Get(ada); j.Kind == job.Attack {
		t.Fatalf("an armed colonist keeps its 4-tile reach: job %+v", j)
	}
	pos, _ := g.positions.Get(ada)
	holdAttacker(g, wolf, ada, pos.X+3, pos.Y)
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Attack || j.Target != wolf {
		t.Fatalf("an armed colonist fights a close attacker: job %+v", j)
	}

	g2 := New(Config{Seed: 3, Width: 50, Height: 20})
	bo, _ := g2.SpawnNamedPawn(25, 10, "Bo")
	fullNeeds(g2, bo)
	g2.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(bo), Enabled: true})
	w2, _ := g2.SpawnAnimal("ridgewolf", 35, 10)
	for i := 0; i < 10; i++ {
		holdAttacker(g2, w2, bo, 35, 10)
		g2.Step(1)
		fullNeeds(g2, bo)
		if j, _ := g2.jobs.Get(bo); j.Kind != job.None && j.State != job.Idle {
			t.Fatalf("a drafted colonist only follows orders: job %+v", j)
		}
	}
	if _, ok := g2.calm[bo]; ok || fleeing(g2, bo).Fleeing {
		t.Fatal("a drafted colonist doesn't flee")
	}
}

func TestQueuedOrderWaitsForCalmThenResumes(t *testing.T) {
	g := New(Config{Seed: 3, Width: 60, Height: 20})
	ada, _ := g.SpawnNamedPawn(30, 10, "Ada")
	fullNeeds(g, ada)
	g.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: 30, Y: 2})
	g.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: 5, Y: 2, Queue: true})
	g.Step(1)
	if j, _ := g.jobs.Get(ada); !j.Forced || j.TX != 30 || j.TY != 2 {
		t.Fatalf("following the first order: %+v", j)
	}
	wolf, _ := g.SpawnAnimal("ridgewolf", 40, 10)
	holdAttacker(g, wolf, ada, 40, 10)
	g.Step(1)
	j, _ := g.jobs.Get(ada)
	if j.Kind != job.Move || j.Forced {
		t.Fatalf("the colonist drops its order to flee: job %+v", j)
	}
	q := g.orderQueue[ada]
	if len(q) != 2 || q[0].cmd.X != 30 || q[0].cmd.Y != 2 || q[1].cmd.X != 5 {
		t.Fatalf("the interrupted order goes back to the front of the queue: %+v", q)
	}
	start := uint64(g.clock.Now())
	releaseAttacker(g, wolf, 58, 18)
	for uint64(g.clock.Now()) < start+combat.FleeCalm-1 {
		g.Step(1)
		fullNeeds(g, ada)
		if j, _ := g.jobs.Get(ada); j.Forced {
			t.Fatalf("tick %d: still shaken, the order waits: job %+v", uint64(g.clock.Now())-start, j)
		}
	}
	resumed := false
	for i := 0; i < 50 && !resumed; i++ {
		g.Step(1)
		fullNeeds(g, ada)
		j, _ := g.jobs.Get(ada)
		resumed = j.Forced && j.TX == 30 && j.TY == 2
	}
	if !resumed {
		t.Fatal("once calm, the colonist picks its order back up")
	}
	if fleeing(g, ada).Fleeing {
		t.Fatal("calm again, the colonist no longer shows as fleeing")
	}
	if q := g.orderQueue[ada]; len(q) != 1 || q[0].cmd.X != 5 {
		t.Fatalf("the later order still waits behind it: %+v", q)
	}
}

func TestSleepingColonistWakesAndRuns(t *testing.T) {
	g := New(Config{Seed: 3, Width: 50, Height: 20})
	ada, _ := g.SpawnNamedPawn(25, 10, "Ada")
	g.rest.InPlace(ada)
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Sleep {
		t.Fatalf("asleep first: %+v", j)
	}
	wolf, _ := g.SpawnAnimal("ridgewolf", 15, 10)
	holdAttacker(g, wolf, ada, 15, 10)
	g.Step(2)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Move || j.TX <= 25 {
		t.Fatalf("a sleeper wakes and runs from a charge: job %+v", j)
	}
}

func TestFleeGoalNeverAcrossAWall(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 17})
	// a wall from top to bottom at x=6: everything west of it is cut off
	for y := 0; y < 17; y++ {
		if !g.BuildNow("wall", "wood", "south", 6, y) {
			t.Fatalf("wall at 6,%d", y)
		}
	}
	ada, _ := g.SpawnNamedPawn(10, 8, "Ada")
	fullNeeds(g, ada)
	wolf, _ := g.SpawnAnimal("ridgewolf", 16, 8)
	holdAttacker(g, wolf, ada, 16, 8)
	g.Step(2)
	j, _ := g.jobs.Get(ada)
	if j.Kind != job.Move || j.TX <= 6 {
		t.Fatalf("the colonist runs to a goal it can reach, not across the wall: job %+v", j)
	}
	if !g.reachable(ada, j.TX, j.TY) {
		t.Fatalf("goal %d,%d unreachable", j.TX, j.TY)
	}
}

func TestFleeGoalPrefersHome(t *testing.T) {
	g := New(Config{Seed: 3, Width: 50, Height: 30})
	ada, _ := g.SpawnNamedPawn(25, 15, "Ada")
	fullNeeds(g, ada)
	g.paintHome(20, 0, 30, 9, true) // home lies north (90° from straight away)
	wolf, _ := g.SpawnAnimal("ridgewolf", 33, 15)
	holdAttacker(g, wolf, ada, 33, 15)
	g.Step(2)
	j, _ := g.jobs.Get(ada)
	if j.Kind != job.Move || !g.inHome(j.TY*g.grid.Width()+j.TX) {
		t.Fatalf("of the open ways out, the colonist runs home: job %+v", j)
	}
}

func TestCorneredColonistStaysPut(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 16})
	walledIn(t, g, 10, 7, 10, 7) // a one-tile cell
	ada, _ := g.SpawnNamedPawn(10, 7, "Ada")
	fullNeeds(g, ada)
	wolf, _ := g.SpawnAnimal("ridgewolf", 18, 7)
	for i := 0; i < 40; i++ {
		holdAttacker(g, wolf, ada, 18, 7)
		g.Step(1)
		fullNeeds(g, ada)
		if j, _ := g.jobs.Get(ada); j.Kind != job.None && j.State != job.Idle {
			t.Fatalf("cornered, the colonist stays put: job %+v", j)
		}
	}
	if !fleeing(g, ada).Fleeing {
		t.Fatal("a cornered colonist still shows as fleeing")
	}
}

// A threat hovering between 13 and 15 tiles neither starts a flight nor
// ends one: 12 to start, 16 to let go (hysteresis).
func TestFleeReachNoFlap(t *testing.T) {
	g := New(Config{Seed: 3, Width: 90, Height: 20})
	ada, _ := g.SpawnNamedPawn(50, 10, "Ada")
	fullNeeds(g, ada)
	wolf, _ := g.SpawnAnimal("ridgewolf", 63, 10)
	for i := 0; i < 300; i++ {
		pos, _ := g.positions.Get(ada)
		holdAttacker(g, wolf, ada, pos.X+13+(i/10)%3, pos.Y)
		g.Step(1)
		fullNeeds(g, ada)
		if _, ok := g.calm[ada]; ok || fleeing(g, ada).Fleeing {
			t.Fatalf("tick %d: a threat 13-15 tiles off never starts a flight", i)
		}
	}

	g2 := New(Config{Seed: 3, Width: 90, Height: 20})
	bo, _ := g2.SpawnNamedPawn(70, 10, "Bo")
	fullNeeds(g2, bo)
	w2, _ := g2.SpawnAnimal("ridgewolf", 80, 10)
	holdAttacker(g2, w2, bo, 80, 10) // 10 tiles: flee
	g2.Step(1)
	if !fleeing(g2, bo).Fleeing {
		t.Fatal("a threat 10 tiles off starts a flight")
	}
	for i := 0; i < 2*combat.FleeCalm; i++ {
		pos, _ := g2.positions.Get(bo)
		holdAttacker(g2, w2, bo, min(pos.X+13+(i/10)%3, 89), pos.Y)
		g2.Step(1)
		fullNeeds(g2, bo)
		if until, ok := g2.calm[bo]; !ok || uint64(g2.clock.Now()) >= until || !fleeing(g2, bo).Fleeing {
			pos, _ := g2.positions.Get(bo)
			t.Fatalf("tick %d: a threat 13-15 tiles off keeps a colonist fleeing (at %+v)", i, pos)
		}
	}
	if n := countLogs(g2, bo, "Fled from"); n != 1 {
		t.Fatalf("one flight is one log line, got %d", n)
	}
}

func countLogs(g *Game, id entity.EntityID, prefix string) int {
	n := 0
	for _, s := range logTexts(g, id) {
		if strings.HasPrefix(s, prefix) {
			n++
		}
	}
	return n
}

func TestFleeViewWithoutThreatInSight(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 12})
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	if fleeing(g, ada).Fleeing {
		t.Fatal("a calm colonist isn't fleeing")
	}
	g.calm[ada] = uint64(g.clock.Now()) + combat.FleeCalm
	g.jobs.Set(ada, job.Record{Kind: job.Move, State: job.Goto, TX: 20, TY: 5})
	if pv := fleeing(g, ada); !pv.Fleeing || pv.Job != "Fleeing" {
		t.Fatalf("still running with the attacker out of sight: %+v", pv)
	}
	g.jobs.Set(ada, job.Record{})
	if fleeing(g, ada).Fleeing {
		t.Fatal("shaken but standing with no threat about is not fleeing")
	}
}
