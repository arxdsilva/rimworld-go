package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// scheduleAll paints every hour of everyone's schedule with a.
func scheduleAll(g *Game, a schedule.Activity) {
	for h := 0; h < 24; h++ {
		g.setSchedule(0, h, a)
	}
}

// sleepAnytime lets tired colonists go to bed at any hour (by default the
// game starts at 08:00, a Work hour, where they keep working instead).
func sleepAnytime(g *Game) { scheduleAll(g, schedule.Anything) }

// walledIn rings the cells x0..x1, y0..y1 with walls so nothing can reach
// them.
func walledIn(t *testing.T, g *Game, x0, y0, x1, y1 int) {
	t.Helper()
	for y := y0 - 1; y <= y1+1; y++ {
		for x := x0 - 1; x <= x1+1; x++ {
			if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
				continue
			}
			if !g.BuildNow("wall", "wood", "south", x, y) {
				t.Fatalf("wall at %d,%d", x, y)
			}
		}
	}
}

// sleepTicks is how long resting from rest a to b takes.
func sleepTicks(a, b int) int { return (b - a) / needs.SleepGain * needs.SleepPeriod }

func TestCollapseSleepsWhereItStands(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	placeBuilt(t, g, construction.Bed, construction.South, 12, 5) // a bed it never walks to
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 14, Y: 2})
	g.Step(TicksPerCell * 3) // on its way
	g.SetNeeds(id, needs.Full, 0)
	g.Step(1)
	pos, _ := g.positions.Get(id)
	j, _ := g.jobs.Get(id)
	v, _ := g.vital.Get(id)
	if v.Status != health.Downed || v.Cause != health.CauseNoBed {
		t.Fatalf("status %v (%v), want collapsed", v.Status, v.Cause)
	}
	if j.Kind != job.Sleep || j.TX != pos.X || j.TY != pos.Y || g.sleptAt[id] != 0 {
		t.Fatalf("job %v at %d,%d (pawn at %d,%d, bed %d): should sleep where it dropped", j.Kind, j.TX, j.TY, pos.X, pos.Y, g.sleptAt[id])
	}
	if !hasLog(g, id, "Collapsed from exhaustion") {
		t.Fatalf("log %v", logTexts(g, id))
	}
}

// A collapsed colonist sleeps to a reasonable rest: hunger doesn't get it up
// early (which used to wake it every tick, as it can't walk while downed).
func TestCollapseStaysDownUntil80(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SpawnStack(item.Food, 20, 6, 2)
	g.SetNeeds(id, needs.CriticalHunger, 0)
	for i := 0; i < sleepTicks(0, needs.WakeRest)-needs.SleepPeriod; i++ {
		g.Step(1)
		v, _ := g.vital.Get(id)
		j, _ := g.jobs.Get(id)
		if v.Status != health.Downed || j.Kind != job.Sleep {
			n, _ := g.need.Get(id)
			t.Fatalf("tick %d: %v / %v at rest %d, want collapsed asleep until %d", i, v.Status, j.Kind, n.Rest, needs.WakeRest)
		}
	}
	g.Step(2 * needs.SleepPeriod)
	if v, _ := g.vital.Get(id); v.Status != health.Alive {
		t.Fatalf("status %v at 80%% rest, want back up", v.Status)
	}
}

func TestTiredInWorkBlockKeepsWorking(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	scheduleAll(g, schedule.Work)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	placeBuilt(t, g, construction.Bed, construction.South, 6, 5)
	g.SetNeeds(id, needs.Full, needs.CriticalRest-500)
	for i := 0; i < 600; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.Sleep {
			t.Fatalf("tick %d: went to bed in Work hours before collapsing", i)
		}
	}
}

func TestTiredInRecreationSleepsTo80(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	scheduleAll(g, schedule.Recreation)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SetNeeds(id, needs.Full, needs.CriticalRest-500)
	asleep := false
	for i := 0; i < sleepTicks(needs.CriticalRest-600, needs.WakeRest)+500; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		n, _ := g.need.Get(id)
		if j.Kind == job.Sleep && j.State == job.Do {
			asleep = true
			continue
		}
		if asleep {
			if n.Rest < needs.WakeRest {
				t.Fatalf("woke at rest %d, want %d", n.Rest, needs.WakeRest)
			}
			return
		}
	}
	t.Fatalf("asleep=%v: never finished a sleep", asleep)
}

// Bug 4: a Sleep block doesn't keep a fully rested colonist in bed; the rest
// of the block is free time, until it tires again.
func TestSleepBlockWakesAtFullThenFreeTime(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	scheduleAll(g, schedule.Sleep)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SetNeeds(id, needs.Full, needs.Full/2)
	woke := false
	for i := 0; i < sleepTicks(needs.Full/2, needs.Full)+200 && !woke; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		n, _ := g.need.Get(id)
		woke = j.Kind != job.Sleep && n.Rest >= needs.Full-needs.SleepGain && g.restedFree[id]
	}
	if !woke {
		t.Fatal("never woke rested in the Sleep block")
	}
	idled := false
	for i := 0; i < 2000; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Sleep {
			t.Fatalf("tick %d: back to bed while rested", i)
		}
		idled = idled || j.Kind.IsIdle()
	}
	if !idled {
		t.Fatal("no free time after waking")
	}
	g.SetNeeds(id, needs.Full, needs.CriticalRest)
	g.Step(5)
	if j, _ := g.jobs.Get(id); j.Kind != job.Sleep {
		t.Fatalf("job %v: tired again in the Sleep block, should go back to bed", j.Kind)
	}
}

// The user's choice: when Work hours start a sleeper over the tired line
// gets up, even short of WakeRest.
func TestSleepBlockEndWakesForWork(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	scheduleAll(g, schedule.Sleep)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SetNeeds(id, needs.Full, needs.Full/2)
	g.Step(needs.SleepWorkMin + 10)
	if j, _ := g.jobs.Get(id); j.Kind != job.Sleep || j.State != job.Do {
		t.Fatalf("job %v %v: should be asleep", j.Kind, j.State)
	}
	scheduleAll(g, schedule.Work)
	g.Step(5)
	if j, _ := g.jobs.Get(id); j.Kind == job.Sleep {
		t.Fatal("still asleep in Work hours at 50% rest")
	}
}

// The reported flap: a free-schedule colonist bounced between bed and work
// every time it crept over the tired line. Now it sleeps once in a while,
// properly.
func TestNoMicroNaps(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	sleepAnytime(g)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SpawnStack(item.Food, 75, 10, 7)
	g.SetNeeds(id, needs.Full, needs.CriticalRest+200)
	sleeps, start, startRest := 0, 0, 0
	wasAsleep := false
	for i := 0; i < 3*clock.TicksPerDay; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		n, _ := g.need.Get(id)
		asleep := j.Kind == job.Sleep && j.State == job.Do
		switch {
		case asleep && !wasAsleep:
			sleeps++
			start, startRest = i, n.Rest
		case !asleep && wasAsleep && !n.Hungry():
			if n.Rest < needs.WakeRest-needs.SleepGain {
				t.Fatalf("sleep %d (from tick %d at rest %d) ended at rest %d after %d ticks", sleeps, start, startRest, n.Rest, i-start)
			}
		}
		wasAsleep = asleep
	}
	// awake 80% -> 20% is 9.6 h, asleep 20% -> 80% 6 h: about 4.6 sleeps
	if sleeps < 3 || sleeps > 6 {
		t.Fatalf("%d sleeps in 3 days, want 3-6", sleeps)
	}
}

// kindChanges counts how often pawn id's job kind changes over n ticks.
func kindChanges(g *Game, id entity.EntityID, n int) int {
	changes := 0
	last, _ := g.jobs.Get(id)
	for i := 0; i < n; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind != last.Kind {
			changes++
		}
		last = j
	}
	return changes
}

// warmHut is a winter night with a heated hut and a colonist out in the cold.
func warmHut(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	jumpTo(g, 45, 0)
	sleepAnytime(g)
	hut(t, g, 6, 1, 12, 7)
	if !g.BuildNow("campfire", "wood", "south", 9, 3) {
		t.Fatal("build campfire")
	}
	g.FuelCampfire(9, 3)
	g.Step(clock.TicksPerHour * 4) // the hut passes 8 C after about 3 h
	id, _ := g.SpawnNamedPawn(2, 10, "Ada")
	g.exposure[id] = exposureState{hypo: exposure.Mild + 50}
	return g, id
}

// W1: warming up and going to bed used to interrupt each other every tick.
func TestColdAndTiredDoesNotFlap(t *testing.T) {
	g, id := warmHut(t)
	g.SetNeeds(id, needs.Full, needs.CriticalRest-500)
	if c := kindChanges(g, id, 500); c > 3 {
		t.Fatalf("job kind changed %d times in 500 ticks", c)
	}
}

// W2: the same loop between warming up and lying down for a doctor.
func TestColdPatientDoesNotFlap(t *testing.T) {
	g, id := warmHut(t)
	g.SpawnNamedPawn(3, 10, "Doc")
	w := g.wounds[id]
	w.Infection = infection.Sick
	g.wounds[id] = w
	if !g.patientRest(id) {
		t.Fatal("setup: should be a patient")
	}
	if c := kindChanges(g, id, 300); c > 3 {
		t.Fatalf("job kind changed %d times in 300 ticks", c)
	}
}

// W3: with nowhere warm to go, the warmth check must not cancel the job the
// colonist is doing (it used to, every warmRetry ticks).
func TestNoWarmSpotDoesNotCutTheJob(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 8})
	jumpTo(g, 45, 0)
	sleepAnytime(g)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 21, Y: 2})
	g.Step(2)
	g.exposure[id] = exposureState{hypo: exposure.Mild + 50}
	before, _ := g.jobs.Get(id)
	g.Step(1)
	after, _ := g.jobs.Get(id)
	if before.Kind != job.Move || after.Kind != job.Move || after.State == job.Fail || after.TX != 21 {
		t.Fatalf("job %v -> %v (%v): the walk should go on", before.Kind, after.Kind, after.State)
	}
	if !g.noComfortUntil(id) {
		t.Fatal("no warm spot: should wait warmRetry before looking again")
	}
}

// F1: food out of reach used to be retried every tick.
func TestUnreachableFoodBacksOff(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SpawnStack(item.Food, 10, 8, 4)
	walledIn(t, g, 8, 4, 8, 4)
	g.SetNeeds(id, needs.CriticalHunger, needs.Full)
	failed := 0 // ticks spent on a failed attempt to eat
	for i := 0; i < 1000; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.Eat && j.State == job.Fail {
			failed++
		}
	}
	if max := 1000/foodRetry + 1; failed > max {
		t.Fatalf("%d ticks of failed eating in 1000, want at most %d (one per foodRetry)", failed, max)
	}
}

// B1: a bed out of reach means the ground, not retrying every tick.
func TestUnreachableBedSleepsOnGround(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	sleepAnytime(g)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 8, 4)
	walledIn(t, g, 8, 4, 8, 5)
	g.SetNeeds(id, needs.Full, needs.CriticalRest-500)
	g.Step(needs.SleepWorkMin)
	j, _ := g.jobs.Get(id)
	pos, _ := g.positions.Get(id)
	if j.Kind != job.Sleep || j.State != job.Do || j.TX != pos.X || j.TY != pos.Y {
		t.Fatalf("job %v %v at %d,%d (pawn %d,%d): should sleep on the ground", j.Kind, j.State, j.TX, j.TY, pos.X, pos.Y)
	}
}

// I1: a work giver that fails (the stack is walled in) used to overwrite the
// running free-time activity, restarting it every idle recheck.
func TestIdleActivitySurvivesFailedHaul(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SpawnStack(item.Wood, 10, 12, 6)
	walledIn(t, g, 12, 6, 12, 6)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 6, X2: 4, Y2: 8})
	g.Step(5)
	restarts := 0
	last, _ := g.jobs.Get(id)
	for i := 0; i < 1500; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind.IsIdle() && j.Active() && (last.Kind != j.Kind || !last.Active()) {
			restarts++
		}
		last = j
	}
	// activities linger 240-720 ticks: at most about 7 in 1500 ticks
	if restarts > 8 {
		t.Fatalf("free time restarted %d times in 1500 ticks", restarts)
	}
}

// H1: a campfire's room thermostat has a band, so the fire doesn't flicker
// on and off every climate interval.
func TestCampfireBankingHasABand(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	hut(t, g, 6, 1, 12, 7)
	if !g.BuildNow("campfire", "wood", "south", 9, 3) {
		t.Fatal("build campfire")
	}
	g.FuelCampfire(9, 3)
	var bid entity.EntityID
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.Campfire {
			bid = id
		}
	}
	flips, banks := 0, 0
	b, _ := g.bldgs.Get(bid)
	was := g.burning(bid, b)
	for i := 0; i < 2*clock.TicksPerDay/climateInterval; i++ {
		g.fuel[bid] = CampfireCapacity * TicksPerWood // keep it fed
		g.Step(climateInterval)
		now := g.burning(bid, b)
		if now != was {
			flips++
		}
		if !now {
			banks++
		}
		was = now
	}
	if banks == 0 {
		t.Fatal("setup: the fire never banked in a spring hut")
	}
	if max := 2 * 24 / 2; flips > max {
		t.Fatalf("campfire flipped %d times in 2 days, want at most %d", flips, max)
	}
}

func TestAntiFlapStateSurvivesSave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.restedFree[id] = true
	g.noFood[id] = 1234
	g.calm[id] = 5678
	g.banked[99] = true
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !h.restedFree[id] || h.noFood[id] != 1234 || h.calm[id] != 5678 || !h.banked[99] {
		t.Fatalf("lost: restedFree %v noFood %v calm %v banked %v", h.restedFree, h.noFood, h.calm, h.banked)
	}
}
