package game

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// bondYard is a small map with colonist Ada at (4, 6) and a tame dog
// (labrador) beside her.
func bondYard(t *testing.T, seed uint64) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 24, Height: 14})
	ada, err := g.SpawnNamedPawn(4, 6, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	dog, err := g.SpawnAnimal("labrador", 5, 6)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(dog, "Rook")
	return g, ada, dog
}

// withinBinomial fails unless got is within 4 standard deviations of
// n*permille/1000 (a binomial count).
func withinBinomial(t *testing.T, what string, got, n int, permille int32) {
	t.Helper()
	p := float64(permille) / 1000
	mean := float64(n) * p
	sd := 1.0
	if v := float64(n) * p * (1 - p); v > 1 {
		sd = math.Sqrt(v)
	}
	if d := float64(got) - mean; d > 4*sd || d < -4*sd {
		t.Fatalf("%s: %d of %d, want about %.0f (%d permille)", what, got, n, mean, permille)
	}
}

func unbond(g *Game, id entity.EntityID) {
	a, _ := g.animals.Get(id)
	a.Bond, a.Master = 0, 0
	g.animals.Set(id, a)
}

func TestBondOnTameRate(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 3)
	const n = 20000
	bonds := 0
	for i := 0; i < n; i++ {
		g.onTamed(ada, dog)
		if a, _ := g.animals.Get(dog); a.Bond != 0 {
			if a.Bond != uint64(ada) || a.Master != uint64(ada) {
				t.Fatalf("a tame bond is with the tamer, who becomes master: %+v", a)
			}
			bonds++
			unbond(g, dog)
		}
	}
	withinBinomial(t, "tame bonds", bonds, n, training.BondTame)
}

func TestBondOnTrainingSuccessRate(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 4)
	(*g.sk.skills[ada])[skill.Animals].Level = 20 // 60% steps
	const n = 30000
	steps, bonds := 0, 0
	now := uint64(g.clock.Now())
	for i := 0; i < n; i++ {
		a, _ := g.animals.Get(dog)
		a.Trained, a.TrainDecay = [training.NumSkills]uint8{}, 0
		g.trainSession(ada, dog, &a, training.Obedience, now)
		if a.Trained[training.Obedience] > 0 {
			steps++
		}
		if a.Bond != 0 {
			if a.Trained[training.Obedience] == 0 {
				t.Fatal("only a successful step bonds")
			}
			bonds++
			a.Bond, a.Master = 0, 0
		}
		g.animals.Set(dog, a)
	}
	withinBinomial(t, "steps", steps, n, training.StepChance(20, false))
	withinBinomial(t, "training bonds", bonds, steps, training.BondTrain)
}

func TestBondedTrainerIsFiveTimesBetter(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 5)
	(*g.sk.skills[ada])[skill.Animals].Level = 0
	now := uint64(g.clock.Now())
	rate := func(bonded bool) int {
		got := 0
		for i := 0; i < 10000; i++ {
			a, _ := g.animals.Get(dog)
			a.Trained, a.TrainDecay, a.Bond = [training.NumSkills]uint8{}, 0, 0
			if bonded {
				a.Bond = uint64(ada)
			}
			g.trainSession(ada, dog, &a, training.Obedience, now)
			if a.Trained[training.Obedience] > 0 {
				got++
			}
		}
		return got
	}
	plain, bonded := rate(false), rate(true)
	withinBinomial(t, "plain trainer", plain, 10000, training.StepChance(0, false))
	withinBinomial(t, "bonded trainer", bonded, 10000, training.StepChance(0, true))
	if bonded < 4*plain {
		t.Fatalf("bonded trainer %d vs %d: want about x5", bonded, plain)
	}
}

func TestBondOnTendRate(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 6)
	const n = 25000
	bonds := 0
	for i := 0; i < n; i++ {
		a, _ := g.animals.Get(dog)
		a.Injury = 5
		g.animals.Set(dog, a)
		g.doTendAnimal(ada, job.Record{Kind: job.TendAnimal, Target: dog, WorkLeft: 1, State: job.Do})
		a, _ = g.animals.Get(dog)
		if a.Injury >= 5 {
			t.Fatal("a tend mends the animal")
		}
		if a.Bond != 0 {
			bonds++
			unbond(g, dog)
		}
	}
	withinBinomial(t, "tend bonds", bonds, n, training.BondTend)
}

func TestBondProximityRate(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 7)
	a, _ := g.animals.Get(dog)
	a.Master = uint64(ada)
	g.animals.Set(dog, a)
	pos := g.posOf(dog)
	const n = 100000
	bonds := 0
	now := uint64(g.clock.Now())
	a.NextBondCheck = 1
	for i := 0; i < n; i++ {
		now += training.BondCheckTicks
		g.bondDecide(dog, &a, pos, now)
		if a.Bond != 0 {
			bonds++
			a.Bond = 0
		}
	}
	withinBinomial(t, "proximity bonds", bonds, n, training.BondProximity)
}

func TestBondProximityNeedsSightAndRange(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 8)
	a, _ := g.animals.Get(dog)
	if c := g.bondCompanion(dog, a, g.posOf(dog)); c != ada {
		t.Fatalf("an unowned animal looks to the colonist beside it: %d", c)
	}
	g.positions.Set(ada, Position{X: 12, Y: 6})
	g.invalidateCreatures()
	if c := g.bondCompanion(dog, a, g.posOf(dog)); c != 0 {
		t.Fatal("seven tiles is too far")
	}
	g.positions.Set(ada, Position{X: 7, Y: 6})
	g.invalidateCreatures()
	if c := g.bondCompanion(dog, a, g.posOf(dog)); c != ada {
		t.Fatal("two tiles in sight counts")
	}
	placeBuilt(t, g, construction.Wall, construction.South, 6, 6) // a wall between them
	g.positions.Set(dog, Position{X: 5, Y: 6})
	g.positions.Set(ada, Position{X: 7, Y: 6})
	g.invalidateCreatures()
	if c := g.bondCompanion(dog, a, g.posOf(dog)); c != 0 {
		t.Fatal("a wall blocks the bond")
	}
	other, _ := g.SpawnNamedPawn(5, 7, "Bo")
	a.Master = uint64(ada)
	if c := g.bondCompanion(dog, a, g.posOf(dog)); c != 0 && c != ada {
		t.Fatalf("with a master only the master counts, not %d (Bo %d)", c, other)
	}
}

// The proximity check runs only in the decision cycle, once per
// BondCheckTicks: never a per-tick scan.
func TestBondCheckCadence(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 9)
	g.SetNeeds(ada, 1<<20, 1<<20)
	var at []uint64
	last := uint64(0)
	for i := 0; i < 3*training.BondCheckTicks+200; i++ {
		g.Step(1)
		a, ok := g.animals.Get(dog)
		if !ok {
			t.Fatal("dog gone")
		}
		if a.NextBondCheck != last {
			at = append(at, uint64(g.clock.Now()))
			last = a.NextBondCheck
		}
		if a.Bond != 0 {
			break
		}
	}
	if len(at) < 3 {
		t.Fatalf("checks at %v: want one first stagger and one per BondCheckTicks", at)
	}
	t.Logf("checks at %v (dog %d)", at, dog)
	for i, tk := range at {
		// the first look (setting the stagger) may come with the animal's
		// first decision; every check after it is on the decision cycle
		if i > 0 && (tk+uint64(dog))%animal.DecideEvery != (at[1]+uint64(dog))%animal.DecideEvery {
			t.Fatalf("check %d at tick %d is off the decision cycle", i, tk)
		}
		if i > 1 && tk-at[i-1] < training.BondCheckTicks {
			t.Fatalf("checks %d ticks apart", tk-at[i-1])
		}
	}
}

func TestBondThoughts(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 10)
	bo, _ := g.SpawnNamedPawn(8, 6, "Bo")
	has := func(id entity.EntityID, k mood.Kind) int {
		for _, th := range g.bondThoughts(id) {
			if th.Kind == k {
				return th.Value
			}
		}
		return 0
	}
	a, _ := g.animals.Get(dog)
	a.Bond, a.Master = uint64(ada), uint64(ada)
	g.animals.Set(dog, a)
	if has(ada, mood.BondedMaster) != 5 || has(ada, mood.BondedNotMaster) != 0 || len(g.bondThoughts(bo)) != 0 {
		t.Fatalf("bonded master +5: %v / %v", g.bondThoughts(ada), g.bondThoughts(bo))
	}
	applyNow(g, command.Command{Kind: command.SetAnimalMaster, Target: uint64(dog), Pawn: uint64(bo)})
	if has(ada, mood.BondedNotMaster) != -3 || has(ada, mood.BondedMaster) != 0 {
		t.Fatalf("someone else is master: -3, %v", g.bondThoughts(ada))
	}
	if len(g.bondThoughts(bo)) != 0 {
		t.Fatal("the master without the bond thinks nothing of it")
	}
	withTraits(g, ada, false, trait.ColdHearted)
	if has(ada, mood.BondedNotMaster) != 0 {
		t.Fatal("cold-hearted people do not mind")
	}
}

func bondMemCount(g *Game, id entity.EntityID, k mood.Kind) int {
	n := 0
	for _, m := range g.memories[id] {
		if m.Kind == k {
			n++
		}
	}
	return n
}

// §20.14: only the bonded owner grieves the animal.
func TestBondGriefOnlyForBondedOwner(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 11)
	bo, _ := g.SpawnNamedPawn(8, 6, "Bo")
	a, _ := g.animals.Get(dog)
	a.Bond, a.Master = uint64(ada), uint64(bo)
	g.animals.Set(dog, a)
	g.KillAnimal(dog)
	if bondMemCount(g, ada, mood.BondedDied) != 1 || bondMemCount(g, bo, mood.BondedDied) != 0 {
		t.Fatalf("grief: ada %v, bo %v", g.memories[ada], g.memories[bo])
	}
	if d := mood.DefOf(mood.BondedDied); d.Value != -8 || d.Duration != 20*60000 {
		t.Fatalf("bonded died def %+v", d)
	}
	// a cold-hearted owner does not grieve
	g2, cold, cat := bondYard(t, 12)
	withTraits(g2, cold, false, trait.ColdHearted)
	c, _ := g2.animals.Get(cat)
	c.Bond = uint64(cold)
	g2.animals.Set(cat, c)
	g2.KillAnimal(cat)
	if bondMemCount(g2, cold, mood.BondedDied) != 0 {
		t.Fatal("cold-hearted: no grief")
	}
}

func TestBondReleasedGivenAwayAndLost(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 13)
	bond := func(id entity.EntityID) {
		a, _ := g.animals.Get(id)
		a.Bond, a.Master = uint64(ada), uint64(ada)
		g.animals.Set(id, a)
	}
	bond(dog)
	applyNow(g, command.Command{Kind: command.ReleaseAnimal, Pawn: uint64(dog)})
	if a, _ := g.animals.Get(dog); a.Tame || a.Bond != 0 || a.Master != 0 || bondMemCount(g, ada, mood.BondedReleased) != 1 {
		t.Fatalf("release: %+v %v", a, g.memories[ada])
	}
	cat, _ := g.SpawnAnimal("cat", 6, 6)
	g.TameAnimal(cat, "Ash")
	bond(cat)
	applyNow(g, command.Command{Kind: command.GiftAnimal, Target: uint64(cat), Pawn: 0})
	if _, ok := g.animals.Get(cat); !ok {
		t.Fatal("a gift needs a visitor")
	}
	applyNow(g, command.Command{Kind: command.GiftAnimal, Target: uint64(cat), Pawn: uint64(ada)})
	if _, ok := g.animals.Get(cat); !ok {
		t.Fatal("a colonist is no visitor")
	}
	applyNow(g, command.Command{Kind: command.GiftAnimal, Target: uint64(cat), Pawn: 9999})
	if _, ok := g.animals.Get(cat); ok || bondMemCount(g, ada, mood.BondedGivenAway) != 1 {
		t.Fatalf("given away: gone, and missed: %v", g.memories[ada])
	}
	hare, _ := g.SpawnAnimal("labrador", 7, 6)
	g.TameAnimal(hare, "Moss")
	bond(hare)
	g.leaveMap(hare)
	if _, ok := g.animals.Get(hare); ok || bondMemCount(g, ada, mood.BondedLost) != 1 {
		t.Fatalf("lost: %v", g.memories[ada])
	}
}

func TestMasterDeathConfusesOrEnrages(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 14)
	const n = 3000
	mad := 0
	for i := 0; i < n; i++ {
		a, _ := g.animals.Get(dog)
		a.Bond, a.Master, a.Mode, a.Manhunter, a.ConfusedUntil = uint64(ada), uint64(ada), animal.Wander, 0, 0
		g.animals.Set(dog, a)
		g.bondsOnDeath(ada)
		a, _ = g.animals.Get(dog)
		if a.Bond != 0 || a.Master != 0 {
			t.Fatal("the bond and mastery end with the colonist")
		}
		switch {
		case a.Manhunter != 0:
			mad++
		case a.Mode == animal.Confused && a.ConfusedUntil == uint64(g.clock.Now())+bondConfusedTicks:
		default:
			t.Fatalf("bereaved animal %+v", a)
		}
	}
	withinBinomial(t, "manhunters", mad, n, bondManhunterPermille)
	// an animal that only had them as master just loses its master
	a, _ := g.animals.Get(dog)
	a.Bond, a.Master, a.Mode, a.Manhunter = 0, uint64(ada), animal.Wander, 0
	g.animals.Set(dog, a)
	g.bondsOnDeath(ada)
	if a, _ := g.animals.Get(dog); a.Master != 0 || a.Mode != animal.Wander {
		t.Fatalf("master only: %+v", a)
	}
}

func TestConfusedAnimalWandersThenRecovers(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 15)
	g.SetNeeds(ada, 1<<20, 1<<20)
	a, _ := g.animals.Get(dog)
	now := uint64(g.clock.Now())
	a.Mode, a.ConfusedUntil = animal.Confused, now+400
	g.animals.Set(dog, a)
	g.Step(200)
	if a, _ := g.animals.Get(dog); a.Mode != animal.Confused {
		t.Fatalf("still confused: %v", a.Mode)
	}
	var view AnimalView
	for _, v := range g.World().Animals {
		if v.ID == dog {
			view = v
		}
	}
	if view.State != "confused" {
		t.Fatalf("the view says so: %q", view.State)
	}
	g.Step(300)
	if a, _ := g.animals.Get(dog); a.Mode == animal.Confused {
		t.Fatal("confusion passes")
	}
}

func TestSetAnimalMasterValidates(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 16)
	wild, _ := g.SpawnAnimal("deer", 10, 10)
	applyNow(g, command.Command{Kind: command.SetAnimalMaster, Target: uint64(dog), Pawn: uint64(wild)})
	if a, _ := g.animals.Get(dog); a.Master != 0 {
		t.Fatal("an animal is no master")
	}
	applyNow(g, command.Command{Kind: command.SetAnimalMaster, Target: uint64(wild), Pawn: uint64(ada)})
	if a, _ := g.animals.Get(wild); a.Master != 0 {
		t.Fatal("wild animals have no master")
	}
	applyNow(g, command.Command{Kind: command.SetAnimalMaster, Target: uint64(dog), Pawn: uint64(ada)})
	var view AnimalView
	for _, v := range g.World().Animals {
		if v.ID == dog {
			view = v
		}
	}
	if view.Master != "Ada" || view.MasterID != ada || len(view.Trained) != int(training.NumSkills) || len(view.Training) != int(training.NumSkills) {
		t.Fatalf("view %+v", view)
	}
	applyNow(g, command.Command{Kind: command.SetAnimalMaster, Target: uint64(dog), Pawn: 0})
	if a, _ := g.animals.Get(dog); a.Master != 0 {
		t.Fatal("0 clears")
	}
}

func TestBirthRecordsMother(t *testing.T) {
	t.Parallel()
	g, _, dog := bondYard(t, 17)
	a, _ := g.animals.Get(dog)
	a.Female = true
	g.animals.Set(dog, a)
	g.giveBirth(dog, &a, uint64(g.clock.Now()))
	g.animals.Set(dog, a)
	young := 0
	for _, v := range g.World().Animals {
		if c, _ := g.animals.Get(v.ID); c.Mother == uint64(dog) {
			young++
			if v.Mother != "Rook" {
				t.Fatalf("young's view names its mother: %q", v.Mother)
			}
		}
		if v.ID == dog && len(v.Offspring) == 0 {
			t.Fatal("the mother's view lists her young")
		}
	}
	if young == 0 {
		t.Fatal("a litter")
	}
}

func TestBondFieldsSurviveSave(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 18)
	a, _ := g.animals.Get(dog)
	a.Bond, a.Master, a.TrainDecay = uint64(ada), uint64(ada), 7
	a.Trained[training.Obedience] = 2
	a.Trained[training.Guard] = trainToggled
	g.animals.Set(dog, a)
	l := saveLoad(t, g)
	b, _ := l.animals.Get(dog)
	if b.Bond != a.Bond || b.Master != a.Master || b.Trained != a.Trained || b.TrainDecay != 7 {
		t.Fatalf("saved %+v", b)
	}
}
