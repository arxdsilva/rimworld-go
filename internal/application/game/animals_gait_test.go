package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// PLAY-022 C4 (animal gaits) and C5 (graze only where there is food).

// eastPath is n cells east of (x, y).
func eastPath(x, y, n int) []pathing.Point {
	var p []pathing.Point
	for i := 1; i <= n; i++ {
		p = append(p, pathing.Point{X: x + i, Y: y})
	}
	return p
}

// stepsOf is the two tick counts a step of st ticks takes: straight, and
// diagonal (st * 1.414, rounded).
func stepsOf(st int) map[int]bool { return map[int]bool{st: true, (st*1414 + 500) / 1000: true} }

// startedStep reports the animal's view when it began a step in the tick
// just run (before is the tick number before that Step).
func startedStep(t *testing.T, g *Game, id entity.EntityID, before uint64) (AnimalView, bool) {
	t.Helper()
	av := animalView(t, g, id)
	return av, av.StepTicks > 0 && av.StepTick == before
}

func TestWandererAmblesAtTwiceTheWalkingStep(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id := walkAnimal(t, g, "deer", 1, 1, eastPath(1, 1, 4))
	if av := animalView(t, g, id); av.Gait != "" {
		t.Fatalf("a deer that has not moved has no gait, got %q", av.Gait)
	}
	st := animal.DefOf(animal.Deer).StepTicks
	g.Step(1)
	if av := animalView(t, g, id); av.Gait != GaitAmble || av.StepTicks != 2*st {
		t.Fatalf("the first wandering step: gait %q over %d ticks, want %q over %d", av.Gait, av.StepTicks, GaitAmble, 2*st)
	}
	at, _ := stepTicks(g, id, 400)
	assertIntervals(t, at, 3, 2*st) // the other three steps
	// the walk is over once its last step ends: the view reads still
	if av := animalView(t, g, id); av.Gait != "" || av.HasNext {
		t.Fatalf("arrived: gait %q hasNext %v, want none", av.Gait, av.HasNext)
	}
}

// A mode toggle mid-step never changes the current step's ticks or label
// (no-flap): the new gait starts with the next step.
func TestGaitFixedForTheStepOnceItStarts(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id := walkAnimal(t, g, "deer", 1, 1, eastPath(1, 1, 6))
	d := animal.DefOf(animal.Deer)
	start := uint64(g.Tick())
	g.Step(1)
	first := animalView(t, g, id)
	if first.StepTick != start || first.Gait != GaitAmble || first.StepTicks != 2*d.StepTicks {
		t.Fatalf("first step %+v, want an ambling %d-tick step at %d", first, 2*d.StepTicks, start)
	}
	setAnimal(g, id, func(a *animal.State) { a.Mode = animal.Flee }) // it takes fright mid-step
	for i := 0; i < 2*d.StepTicks-3; i++ {
		g.Step(1)
		av := animalView(t, g, id)
		if av.Gait != GaitAmble || av.StepTicks != first.StepTicks || av.StepTick != first.StepTick || av.X != first.X {
			t.Fatalf("tick %d of the step: %+v changed from %+v", i, av, first)
		}
	}
	// the next step is a flee step
	for i := 0; i < 8; i++ {
		before := uint64(g.Tick())
		g.Step(1)
		if av, ok := startedStep(t, g, id, before); ok {
			if av.Gait != GaitRun || !stepsOf(d.FleeStep)[av.StepTicks] {
				t.Fatalf("second step: gait %q over %d ticks, want %q over a flee step %v", av.Gait, av.StepTicks, GaitRun, stepsOf(d.FleeStep))
			}
			return
		}
	}
	t.Fatal("the fleeing deer never took its next step")
}

func TestFleeingRunsAtTheFleeStep(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 30, Height: 12})
	deer, _ := g.SpawnAnimal("deer", 10, 6)
	g.SpawnAnimal("ridgewolf", 13, 6)
	d := animal.DefOf(animal.Deer)
	for i := 0; i < 300; i++ {
		before := uint64(g.Tick())
		g.Step(1)
		av, ok := startedStep(t, g, deer, before)
		if !ok || av.Mode != "fleeing" {
			continue
		}
		if av.Gait != GaitRun || !stepsOf(d.FleeStep)[av.StepTicks] {
			t.Fatalf("a fleeing deer: gait %q over %d ticks, want %q over %v", av.Gait, av.StepTicks, GaitRun, stepsOf(d.FleeStep))
		}
		return
	}
	t.Fatal("the deer never fled from the wolf")
}

// The balance guard: a hunting or charging wolf reports run but its
// step is what it always was.
func TestChasingRunsButKeepsTodaysStep(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 40, Height: 12})
	victim, _ := g.SpawnNamedPawn(10, 6, "Bait")
	wolf, _ := g.SpawnAnimal("ridgewolf", 22, 6)
	setAnimal(g, wolf, func(a *animal.State) { a.Mode, a.Target, a.HuntSince = animal.Attack, victim, uint64(g.Tick()) })
	d := animal.DefOf(animal.TimberWolf)
	seen := 0
	for i := 0; i < 200 && seen < 3; i++ {
		before := uint64(g.Tick())
		g.Step(1)
		av, ok := startedStep(t, g, wolf, before)
		if !ok {
			continue
		}
		seen++
		if av.Gait != GaitRun || !stepsOf(d.StepTicks)[av.StepTicks] {
			t.Fatalf("a chasing wolf: gait %q over %d ticks, want %q over today's step %v (not the flee step %d)",
				av.Gait, av.StepTicks, GaitRun, stepsOf(d.StepTicks), d.FleeStep)
		}
	}
	if seen == 0 {
		t.Fatal("the wolf never chased")
	}
}

func TestLeavingWalksTheMapOff(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id, _ := g.SpawnAnimal("deer", 6, 5)
	g.startLeaving(id, 19, 5)
	d := animal.DefOf(animal.Deer)
	for i := 0; i < 100; i++ {
		before := uint64(g.Tick())
		g.Step(1)
		if av, ok := startedStep(t, g, id, before); ok {
			if av.Gait != GaitWalk || !stepsOf(d.StepTicks)[av.StepTicks] {
				t.Fatalf("a leaving deer: gait %q over %d ticks, want %q over %v", av.Gait, av.StepTicks, GaitWalk, stepsOf(d.StepTicks))
			}
			return
		}
	}
	t.Fatal("the leaving deer never stepped")
}

// A hungry deer walks to its meal and ambles when it wanders; each step's
// label follows the mode it started in.
func TestGoingToFoodWalksWanderingAmbles(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	sowGrass(g, 16, 8, 20, 12)
	deer, _ := g.SpawnAnimal("deer", 8, 10)
	setAnimal(g, deer, func(a *animal.State) { a.Food = 3000 })
	d := animal.DefOf(animal.Deer)
	want := map[string]struct {
		gait string
		step int
	}{"feeding": {GaitWalk, d.StepTicks}, "wandering": {GaitAmble, 2 * d.StepTicks}}
	feeding := 0
	for i := 0; i < 3000; i++ {
		before := uint64(g.Tick())
		g.Step(1)
		av, ok := startedStep(t, g, deer, before)
		if !ok {
			continue
		}
		w, known := want[av.Mode]
		if !known {
			continue
		}
		if av.Gait != w.gait || !stepsOf(w.step)[av.StepTicks] {
			t.Fatalf("a %s deer: gait %q over %d ticks, want %q over %v", av.Mode, av.Gait, av.StepTicks, w.gait, stepsOf(w.step))
		}
		if av.Mode == "feeding" {
			feeding++
		}
	}
	if feeding == 0 {
		t.Fatal("the deer never walked to the grass")
	}
}

// Led on a rope, and following a drafted master, are errands: walked.
func TestLedAndFollowingAnimalsWalk(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	cow := tameAt(t, g, "cow", "Bess", 8, 10)
	ada, _ := g.SpawnNamedPawn(16, 10, "Ada")
	if !g.reserve.TryEntity(ada, cow) {
		t.Fatal("reserve")
	}
	g.jobs.Set(ada, job.Record{Kind: job.Rope, Target: cow, State: job.Do, Count: 1})
	a := animalOf(g, cow)
	g.thinkAnimal(cow, &a, uint64(g.clock.Now()))
	g.animals.Set(cow, a)
	if !a.Errand || a.GaitAt(uint64(g.clock.Now())) != animal.GaitWalk {
		t.Fatalf("a led cow is on an errand: %+v", a)
	}
	g.phaseMovement()
	av := animalView(t, g, cow)
	if want := animal.DefOf(animal.Cow).StepTicks; av.Gait != GaitWalk || av.StepTicks != want {
		t.Fatalf("a led cow: gait %q over %d ticks, want %q over %d", av.Gait, av.StepTicks, GaitWalk, want)
	}

	h, master, dog := bondYard(t, 24)
	setAnimal(h, dog, func(a *animal.State) {
		a.Master, a.Trained[training.Obedience], a.TrainDecay = uint64(master), 3, uint64(h.clock.Now())
	})
	h.positions.Set(dog, Position{X: 20, Y: 6})
	applyNow(h, command.Command{Kind: command.DraftPawn, Pawn: uint64(master), Enabled: true})
	b := animalOf(h, dog)
	h.thinkAnimal(dog, &b, uint64(h.clock.Now()))
	h.animals.Set(dog, b)
	if !b.Errand {
		t.Fatalf("an obedient dog going to its drafted master is on an errand: %+v", b)
	}
	h.phaseMovement()
	if av := animalView(t, h, dog); av.Gait != GaitWalk {
		t.Fatalf("a following dog: gait %q, want %q", av.Gait, GaitWalk)
	}
	// the errand ends with the following: back to wandering, it ambles
	applyNow(h, command.Command{Kind: command.DraftPawn, Pawn: uint64(master), Enabled: false})
	b = animalOf(h, dog)
	h.thinkAnimal(dog, &b, uint64(h.clock.Now()))
	if b.Errand {
		t.Fatalf("an undrafted master is no errand: %+v", b)
	}
}

func TestViewGaitNamesMatchTheDomain(t *testing.T) {
	t.Parallel()
	for g, want := range map[animal.Gait]string{animal.GaitAmble: GaitAmble, animal.GaitWalk: GaitWalk, animal.GaitRun: GaitRun} {
		if g.String() != want {
			t.Errorf("animal gait %d is %q, the view says %q", g, g.String(), want)
		}
	}
}

// ---- C5: graze only where there is food ----

func grazeView(t *testing.T, g *Game, id entity.EntityID) string {
	t.Helper()
	return animalView(t, g, id).Action
}

func TestGrazeOnlyOnFood(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	sowGrass(g, 12, 8, 14, 10)
	deer, _ := g.SpawnAnimal("deer", 5, 5)
	wolf, _ := g.SpawnAnimal("ridgewolf", 6, 5)
	for _, id := range []entity.EntityID{deer, wolf} {
		setAnimal(g, id, func(a *animal.State) { a.NextThink = 1 << 40 })
	}
	at := func(id entity.EntityID, x, y int) { g.positions.Set(id, Position{X: x, Y: y}) }

	if got := grazeView(t, g, deer); got != "" {
		t.Errorf("a deer standing on bare ground does not graze, got %q", got)
	}
	at(deer, 13, 9)
	if got := grazeView(t, g, deer); got != ActionGraze {
		t.Errorf("a deer standing on grass grazes, got %q", got)
	}
	at(deer, 11, 9) // one step off the patch
	if got := grazeView(t, g, deer); got != "" {
		t.Errorf("a deer next to the grass but on bare ground does not graze, got %q", got)
	}
	// a predator never grazes, grass or not
	at(wolf, 13, 9)
	if got := grazeView(t, g, wolf); got != "" {
		t.Errorf("a wolf on grass does not graze, got %q", got)
	}
	// a moving animal is not idle
	at(deer, 13, 9)
	setAnimal(g, deer, func(a *animal.State) { a.Path, a.PathI = eastPath(13, 9, 1), 0 })
	if got := grazeView(t, g, deer); got != "" {
		t.Errorf("a walking deer does not graze, got %q", got)
	}
	// the grass is eaten away under it: the head comes up
	setAnimal(g, deer, func(a *animal.State) { a.Path, a.PathI = nil, 0 })
	g.clearGrass(13, 9)
	if got := grazeView(t, g, deer); got != "" {
		t.Errorf("grazed bare, the deer stops grazing, got %q", got)
	}
}

func TestGrazeOnABrowsedPlantUnderfootOrBeside(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	deer, _ := g.SpawnAnimal("deer", 5, 5)
	setAnimal(g, deer, func(a *animal.State) { a.NextThink = 1 << 40 })
	moss, err := g.spawnPlant(plant.Moss, 5, 5, plant.DefOf(plant.Moss).GrowTicks) // a walkable ground plant it browses
	if err != nil {
		t.Fatal(err)
	}
	if got := grazeView(t, g, deer); got != ActionGraze {
		t.Errorf("a deer on moss grazes, got %q", got)
	}
	g.removePlant(moss)
	if got := grazeView(t, g, deer); got != "" {
		t.Errorf("the moss is gone: no grazing, got %q", got)
	}
	// a bush is browsed from the next tile: eating at its feed target counts
	bush, err := g.spawnPlant(plant.BerryBush, 6, 5, plant.DefOf(plant.BerryBush).GrowTicks)
	if err != nil {
		t.Fatal(err)
	}
	if got := grazeView(t, g, deer); got != "" {
		t.Errorf("a bush beside a wandering deer is no meal yet, got %q", got)
	}
	setAnimal(g, deer, func(a *animal.State) {
		a.Mode, a.Feed = animal.Feed, animal.FeedTarget{Kind: animal.FeedPlant, ID: bush, X: 6, Y: 5}
	})
	if got := grazeView(t, g, deer); got != ActionGraze {
		t.Errorf("a deer eating the bush beside it grazes, got %q", got)
	}
	setAnimal(g, deer, func(a *animal.State) { a.Feed.X = 9 }) // out of reach
	if got := grazeView(t, g, deer); got != "" {
		t.Errorf("its target is not within reach: no grazing, got %q", got)
	}
}

// A meat eater that is no hunter (a cat) has no grass to graze.
func TestCarnivoreOnGrassDoesNotGraze(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	sowGrass(g, 4, 4, 6, 6)
	cat, _ := g.SpawnAnimal("cat", 5, 5)
	setAnimal(g, cat, func(a *animal.State) { a.NextThink = 1 << 40 })
	if def := animal.DefOf(animal.Cat); def.Predator || animal.EcologyOf(animal.Cat).Diet.Plants() {
		t.Fatalf("the test needs a non-predator that eats no plants: %+v", def)
	}
	if got := grazeView(t, g, cat); got != "" {
		t.Errorf("a cat on grass does not graze, got %q", got)
	}
}
