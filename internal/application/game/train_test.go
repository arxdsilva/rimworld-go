package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestTrainingDefaultsAndToggles(t *testing.T) {
	t.Parallel()
	g, _, dog := bondYard(t, 20)
	a, _ := g.animals.Get(dog)
	if !trainOn(a, training.Obedience) || trainOn(a, training.Guard) || trainOn(a, training.Release) {
		t.Fatal("Obedience is on by default, the rest off")
	}
	set := func(id entity.EntityID, s training.Skill, on bool) {
		applyNow(g, command.Command{Kind: command.SetTraining, Target: uint64(id), Index: int(s), Enabled: on})
	}
	set(dog, training.Guard, true)
	set(dog, training.Obedience, false)
	set(dog, training.Release, true) // a labrador is no hunter: ignored
	a, _ = g.animals.Get(dog)
	if trainOn(a, training.Obedience) || !trainOn(a, training.Guard) || trainOn(a, training.Release) {
		t.Fatalf("toggled %v", a.Trained)
	}
	if trainSteps(a, training.Guard, 0) != 0 || trainSteps(a, training.Obedience, 0) != 0 {
		t.Fatal("the toggle bit is not a step")
	}
	set(dog, training.Obedience, true)
	if a, _ = g.animals.Get(dog); !trainOn(a, training.Obedience) || a.Trained[training.Obedience] != 0 {
		t.Fatal("back to default clears the bit")
	}
	wolf, _ := g.SpawnAnimal("timber_wolf", 10, 10)
	set(wolf, training.Release, true)
	if w, _ := g.animals.Get(wolf); trainOn(w, training.Release) {
		t.Fatal("wild animals are not trained")
	}
	g.TameAnimal(wolf, "Burr")
	set(wolf, training.Release, true)
	if w, _ := g.animals.Get(wolf); !trainOn(w, training.Release) {
		t.Fatal("a tame wolf can learn Release")
	}
}

func TestTrainingStepsOrderAndDecay(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 21)
	(*g.sk.skills[ada])[skill.Animals].Level = 20
	a, _ := g.animals.Get(dog)
	a.Bond = uint64(ada) // a bonded trainer of 20 never fails
	a.Trained[training.Guard] |= trainToggled
	now := uint64(1000)
	if s, _ := nextTrainSkill(a, now); s != training.Obedience {
		t.Fatal("Obedience first")
	}
	for i := 0; i < 3; i++ {
		s, _ := nextTrainSkill(a, now)
		g.trainSession(ada, dog, &a, s, now)
	}
	if !trainLearned(a, training.Obedience, now) {
		t.Fatalf("three steps learn Obedience: %v", a.Trained)
	}
	if s, ok := nextTrainSkill(a, now); !ok || s != training.Guard {
		t.Fatal("then Guard")
	}
	for i := 0; i < 3; i++ {
		g.trainSession(ada, dog, &a, training.Guard, now)
	}
	if _, ok := nextTrainSkill(a, now); ok {
		t.Fatal("everything on is learned")
	}
	later := now + training.DecayTicks
	if trainSteps(a, training.Guard, later) != 2 || trainLearned(a, training.Guard, later) {
		t.Fatal("ten days unpractised costs a step")
	}
	if s, ok := nextTrainSkill(a, later); !ok || s != training.Obedience {
		t.Fatal("a decayed skill is trained again")
	}
	if trainSteps(a, training.Obedience, now+99*training.DecayTicks) != 1 {
		t.Fatal("learned Obedience keeps its floor")
	}
	if !trainDue(dog, a, now+trainCooldown+trainJitter) || trainDue(dog, a, now+trainCooldown-1) {
		t.Fatal("the next session waits for the cooldown")
	}
}

// A handler trains a tame animal through Handle work until it learns
// Obedience, and never starts a session before the cooldown (no flap).
func TestHandlerTrainsAnimalWithCooldown(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 22)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	(*g.sk.skills[ada])[skill.Animals].Level = 20
	a, _ := g.animals.Get(dog)
	a.Bond = uint64(ada)
	g.animals.Set(dog, a)
	var starts []uint64
	last := job.None
	for i := 0; i < 4*int(trainCooldown) && len(starts) < 4; i++ {
		g.Step(1)
		if i%50 == 0 {
			g.SetNeeds(ada, 1<<20, 1<<20)
		}
		j, _ := g.jobs.Get(ada)
		if j.Kind == job.Train && last != job.Train {
			starts = append(starts, uint64(g.clock.Now()))
		}
		last = j.Kind
	}
	if len(starts) < 3 {
		t.Fatalf("handler started %d sessions", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if starts[i]-starts[i-1] < trainCooldown {
			t.Fatalf("sessions %d ticks apart: %v", starts[i]-starts[i-1], starts)
		}
	}
	a, _ = g.animals.Get(dog)
	if !trainLearned(a, training.Obedience, uint64(g.clock.Now())) {
		t.Fatalf("Obedience learnt after %d sessions: %v", len(starts), a.Trained)
	}
}

// An obedient animal follows its drafted master; undrafted, it does not.
func TestObedientAnimalFollowsDraftedMaster(t *testing.T) {
	t.Parallel()
	run := func(drafted bool) int {
		g := New(Config{Seed: 23, Width: 44, Height: 20})
		ada, _ := g.SpawnNamedPawn(36, 10, "Ada")
		dog, _ := g.SpawnAnimal("labrador", 6, 10)
		g.TameAnimal(dog, "Rook")
		a, _ := g.animals.Get(dog)
		a.Master, a.TrainDecay = uint64(ada), 1
		a.Trained[training.Obedience] = 3
		g.animals.Set(dog, a)
		g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: drafted})
		for i := 0; i < 40; i++ {
			g.Step(20)
			g.SetNeeds(ada, 1<<20, 1<<20)
		}
		dp, mp := g.posOf(dog), g.posOf(ada)
		return weapon.Distance(dp.X, dp.Y, mp.X, mp.Y)
	}
	if d := run(true); d > followFar+1 {
		t.Fatalf("drafted master: the dog stays close (%d tiles)", d)
	}
	if d := run(false); d <= followFar+1 {
		t.Fatalf("undrafted: the dog keeps to the colony (%d tiles)", d)
	}
}

func TestUntrainedAnimalDoesNotFollow(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 24)
	a, _ := g.animals.Get(dog)
	a.Master = uint64(ada)
	g.animals.Set(dog, a)
	applyNow(g, command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: true})
	a, _ = g.animals.Get(dog)
	if g.followMaster(dog, &a, g.posOf(dog), uint64(g.clock.Now())) {
		t.Fatal("no Obedience, no following")
	}
	a.Trained[training.Obedience], a.TrainDecay = 3, uint64(g.clock.Now())
	if !g.followMaster(dog, &a, g.posOf(dog), uint64(g.clock.Now())) {
		t.Fatal("obedient: follows")
	}
}
