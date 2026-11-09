package game

import (
	"testing"

	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// EAI-003 and EAI-004: a hurt herd animal's mates answer, and wild
// predators against the colony's people and livestock.

// eaiHerd spawns one wild herd (Herd 42) of species at each of spots,
// with Ada standing at (5, 10) on bare ground: it returns Ada and the herd.
func eaiHerd(t *testing.T, species string, spots ...[2]int) (*Game, entity.EntityID, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 5, Width: 48, Height: 22})
	bareGround(g)
	ada, _ := g.SpawnNamedPawn(5, 10, "Ada")
	fullNeeds(g, ada)
	var herd []entity.EntityID
	for _, p := range spots {
		id, err := g.SpawnAnimal(species, p[0], p[1])
		if err != nil {
			t.Fatal(err)
		}
		setAnimal(g, id, func(a *animal.State) {
			a.Herd, a.Mode, a.Target, a.Path, a.PathI, a.Manhunter = 42, animal.Wander, 0, nil, 0, 0
		})
		herd = append(herd, id)
	}
	g.invalidateCreatures()
	return g, ada, herd
}

// TestTerritorialHerdJoins: Ada hurts a bison; its mate close by joins
// the fight against her (and shows "Defending its herd"), a mate beyond
// the herd radius does not.
func TestTerritorialHerdJoins(t *testing.T) {
	t.Parallel()
	g, ada, herd := eaiHerd(t, "bison", [2]int{12, 10}, [2]int{14, 11}, [2]int{30, 10})
	g.hurt(herd[0], 1, ada)
	if m := animalOf(g, herd[1]); m.Mode != animal.Attack || m.Target != ada {
		t.Fatalf("a bison close by answers its herd: mode %v target %d", m.Mode, m.Target)
	}
	if !eaiView(g, herd[1]).Defending {
		t.Fatal("the inspector shows the mate defending its herd")
	}
	if m := animalOf(g, herd[2]); m.Mode == animal.Attack || m.Target != 0 {
		t.Fatalf("a bison 18 tiles off never hears the call: mode %v target %d", m.Mode, m.Target)
	}
}

// TestGrazerHerdFlees: Ada hurts a caribou; its mates run from her.
func TestGrazerHerdFlees(t *testing.T) {
	t.Parallel()
	g, ada, herd := eaiHerd(t, "caribou", [2]int{12, 10}, [2]int{14, 11}, [2]int{13, 13})
	g.hurt(herd[0], 1, ada)
	for _, id := range herd[1:] {
		m := animalOf(g, id)
		if m.Mode != animal.Flee || m.Attacker != ada || m.Target != 0 {
			t.Fatalf("caribou %d runs from Ada: mode %v attacker %d target %d", id, m.Mode, m.Attacker, m.Target)
		}
		if eaiView(g, id).Defending {
			t.Fatal("a fleeing grazer is not defending its herd")
		}
	}
}

// TestHerdDefenceNoFlap: blows rain on a bison every tick; a mate let
// go (a leash) is not called back before callEvery has passed, and a
// mate already fighting keeps its own foe.
func TestHerdDefenceNoFlap(t *testing.T) {
	t.Parallel()
	g, ada, herd := eaiHerd(t, "bison", [2]int{12, 10}, [2]int{14, 11})
	bo, _ := g.SpawnNamedPawn(6, 12, "Bo")
	g.invalidateCreatures()
	r := rageOf(g)
	now := uint64(g.clock.Now())
	r.HerdCall(herd[0], animalOf(g, herd[0]), ada, now)
	if m := animalOf(g, herd[1]); m.Target != ada {
		t.Fatalf("set-up: the mate answers: target %d", m.Target)
	}
	setAnimal(g, herd[1], func(a *animal.State) { rage.EndFight(a) }) // the leash lets it go
	changes, last := 0, false
	for i := uint64(1); i < 300; i++ {
		by := ada
		if i%2 == 0 {
			by = bo // blows from two colonists in turn
		}
		r.HerdCall(herd[0], animalOf(g, herd[0]), by, now+i)
		on := animalOf(g, herd[1]).Mode == animal.Attack
		if on != last {
			changes++
		}
		last = on
	}
	if changes != 0 {
		t.Fatalf("the herd call flapped %d times inside its window", changes)
	}
	r.HerdCall(herd[0], animalOf(g, herd[0]), bo, now+300)
	if m := animalOf(g, herd[1]); m.Target != bo {
		t.Fatalf("a new call after callEvery brings it back: target %d", m.Target)
	}
	r.HerdCall(herd[0], animalOf(g, herd[0]), ada, now+600)
	if m := animalOf(g, herd[1]); m.Target != bo {
		t.Fatalf("a mate already fighting keeps its foe: target %d", m.Target)
	}
}

// eaiLivestock is a yard with Ada at (20, 10), her tame cow Bessie at
// (22, 10) and a wild timber wolf attacking it from (23, 10).
func eaiLivestock(t *testing.T) (g *Game, ada, cow, wolf entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 31, Width: 48, Height: 24})
	bareGround(g)
	ada, _ = g.SpawnNamedPawn(20, 10, "Ada")
	fullNeeds(g, ada)
	cow, _ = g.SpawnAnimal("cow", 22, 10)
	g.TameAnimal(cow, "Bessie")
	wolf, _ = g.SpawnAnimal("timber_wolf", 23, 10)
	holdAttacker(g, wolf, cow, 23, 10)
	return g, ada, cow, wolf
}

// TestColonistDefendsLivestock: an armed colonist takes on a wild
// animal mauling the colony's cow, and the player is told.
func TestColonistDefendsLivestock(t *testing.T) {
	t.Parallel()
	g, ada, cow, wolf := eaiLivestock(t)
	if !g.hostile(animalOf(g, wolf)) {
		t.Fatal("a wild animal attacking livestock is hostile")
	}
	g.Arm(ada, item.Club)
	fought := false
	for i := 0; i < 30 && !fought; i++ { // the colonist looks for foes every few ticks
		holdAttacker(g, wolf, cow, 23, 10)
		g.Step(1)
		fullNeeds(g, ada)
		j, _ := g.jobs.Get(ada)
		fought = j.Kind == job.Attack && j.Target == wolf
	}
	if !fought {
		j, _ := g.jobs.Get(ada)
		t.Fatalf("an armed colonist defends the cow: job %+v", j)
	}
	a := animalOf(g, wolf)
	rageOf(g).WarnChase(wolf, &a, cow, false)
	ms := dangerAbout(g, wolf)
	if len(ms) == 0 || ms[len(ms)-1].Text != "A timber wolf is attacking Bessie!" {
		t.Fatalf("the player is told: %+v", ms)
	}
}

// eaiGuardDog gives Ada a guard-trained labrador at (6, 10).
func eaiGuardDog(g *Game, ada entity.EntityID) entity.EntityID {
	dog, _ := g.SpawnAnimal("labrador", 6, 10)
	g.TameAnimal(dog, "Rook")
	setAnimal(g, dog, func(a *animal.State) {
		a.Master, a.TrainDecay = uint64(ada), 0
		a.Trained[training.Guard] = training.Needed(training.Guard) | herd.TrainToggled
	})
	g.invalidateCreatures()
	return dog
}

// TestGuardAnimalDefendsLivestock: a guard dog goes for a wild animal
// mauling livestock by its master.
func TestGuardAnimalDefendsLivestock(t *testing.T) {
	t.Parallel()
	g, ada, _, wolf := eaiLivestock(t)
	dog := eaiGuardDog(g, ada)
	a := animalOf(g, dog)
	if !g.masterDuty(dog, &a, dutyTick(dog, uint64(g.clock.Now()))) || a.Target != wolf || a.Mode != animal.Attack {
		t.Fatalf("the guard takes on the wolf at the cow: target %d mode %v", a.Target, a.Mode)
	}
}

// TestLivestockDefenceNoFlap: the wolf at the cow shifts back and forth;
// the guard keeps it inside the keep band, and lets it go beyond.
func TestLivestockDefenceNoFlap(t *testing.T) {
	t.Parallel()
	g, ada, cow, wolf := eaiLivestock(t)
	dog := eaiGuardDog(g, ada)
	a := animalOf(g, dog)
	now := dutyTick(dog, uint64(g.clock.Now()))
	if !g.masterDuty(dog, &a, now) || a.Target != wolf {
		t.Fatal("set-up: the guard engages")
	}
	changes, last := 0, true
	for i := 1; i <= 300; i++ {
		holdAttacker(g, wolf, cow, 23+i%3, 10) // 3..5 tiles from Ada
		on := g.masterDuty(dog, &a, now+uint64(i)) && a.Target == wolf
		if on != last {
			changes++
		}
		last = on
	}
	if changes != 0 {
		t.Fatalf("livestock defence flapped %d times inside the keep band", changes)
	}
	holdAttacker(g, wolf, cow, 20+guardKeepRadius+1, 10)
	if g.masterDuty(dog, &a, now+301) && a.Target == wolf {
		t.Fatal("the guard kept a foe beyond the keep radius")
	}
}

// TestPredatorTakesDownedColonist: a hungry predator with no animal to
// take goes for a downed colonist where the difficulty lets predators
// hunt people, and leaves her be where it does not.
func TestPredatorTakesDownedColonist(t *testing.T) {
	t.Parallel()
	setup := func(off bool) (*Game, entity.EntityID, entity.EntityID) {
		g := New(Config{Seed: 4, Width: 40, Height: 20})
		bareGround(g)
		if off {
			g.ng.setup = &Setup{} // predators never hunt people
		}
		ada, _ := g.SpawnNamedPawn(16, 10, "Ada")
		g.hurt(ada, downDamage(g, ada), 0)
		wolf, _ := g.SpawnAnimal("cougar", 10, 10)
		g.invalidateCreatures()
		return g, ada, wolf
	}
	g, ada, wolf := setup(false)
	if g.standingPawn(ada) {
		t.Fatal("set-up: Ada is downed")
	}
	a := animalOf(g, wolf)
	a.Hunger, a.NextThink = animal.DefOf(a.Species).HungerTicks, 0
	g.thinkAnimal(wolf, &a, uint64(g.clock.Now()))
	if a.Target != ada || a.Mode != animal.Attack {
		t.Fatalf("a hungry cougar takes the downed colonist: target %d mode %v", a.Target, a.Mode)
	}
	g2, _, w2 := setup(true)
	pos, _ := g2.positions.Get(w2)
	if got := rageOf(g2).FindPrey(w2, rage.Pos(pos), animal.DefOf(animalOf(g2, w2).Species)); got != 0 {
		t.Fatalf("where predators may not hunt people, it leaves her be: %d", got)
	}
}
