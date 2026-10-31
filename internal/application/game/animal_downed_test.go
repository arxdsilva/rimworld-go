package game

import (
	"testing"

	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Downed animals: too hurt or bled to stand, an animal lies where it fell.

// injuryPct is pct percent of animal id's hit points, rounded down.
func injuryPct(g *Game, id entity.EntityID, pct int) int {
	return animal.DefOf(animalOf(g, id).Species).MaxHP * pct / 100
}

// bruiseDown beats animal id down with one bruise (it does not bleed) to
// 85% of its hit points.
func bruiseDown(t *testing.T, g *Game, id entity.EntityID) {
	t.Helper()
	g.hurtAs(id, injuryPct(g, id, 85)-animalOf(g, id).Injury, 0, body.Bruise, health.CauseInjury)
	if a := animalOf(g, id); !a.Downed || a.DownedTick == 0 {
		t.Fatalf("85%% injury downs it: %+v", a)
	}
}

// Injury held around the thresholds does not flap the downed state: an
// animal goes down at 80%, stays down between 51% and 79%, stands at 50%,
// and stays up between 51% and 79% again; blood loss downs it at the
// severe stage and it stands once the blood is back below moderate.
func TestAnimalDownsAndRecovers(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 61, Width: 24, Height: 14})
	buck, err := g.SpawnAnimal("deer", 12, 7)
	if err != nil {
		t.Fatal(err)
	}
	hold := func(pct int) {
		setAnimal(g, buck, func(a *animal.State) { a.Injury, a.Food, a.Starving = injuryPct(g, buck, pct), animal.Full, 0 })
	}
	toggles := func(pcts ...int) int {
		n, was := 0, animalOf(g, buck).Downed
		for h := 0; h < 48; h++ {
			hold(pcts[h%len(pcts)])
			g.Step(clock.TicksPerHour)
			if d := animalOf(g, buck).Downed; d != was {
				n, was = n+1, d
			}
		}
		return n
	}
	if n := toggles(animal.DownInjuryPct, 79, 51, 79); n != 1 || !animalOf(g, buck).Downed {
		t.Fatalf("down once at 80%%, then held down: %d toggles, %+v", n, animalOf(g, buck))
	}
	if n := toggles(79, 51, 60, 70); n != 0 {
		t.Fatalf("between the lines it stays down: %d toggles", n)
	}
	if n := toggles(animal.UpInjuryPct, 79, 51, 79); n != 1 || animalOf(g, buck).Downed {
		t.Fatalf("up once at 50%%, then held up: %d toggles, %+v", n, animalOf(g, buck))
	}
	if n := toggles(79, 51, 60, 70); n != 0 {
		t.Fatalf("between the lines it stays up: %d toggles", n)
	}

	// blood loss: severe downs, regenerating below moderate stands it
	hold(0)
	setAnimal(g, buck, func(a *animal.State) { a.Body = &body.Body{BloodLoss: animal.DownBloodLoss + 40} })
	g.markAnimalPatient(buck)
	toNextHour(g)
	if a := animalOf(g, buck); !a.Downed {
		t.Fatalf("severe blood loss downs it on the hourly step: %+v", a)
	}
	n, was := 0, true
	for h := 0; h < 48; h++ {
		toNextHour(g)
		if d := animalOf(g, buck).Downed; d != was {
			n, was = n+1, d
		}
	}
	if a := animalOf(g, buck); n != 1 || a.Downed {
		t.Fatalf("the blood comes back and it stands, once: %d toggles, %+v", n, a)
	}
}

// A downed deer neither flees from the colonist hitting it nor moves; a
// downed manhunter stops hunting and reads as no threat to auto-engage.
func TestDownedAnimalDoesNotFlee(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 62, Width: 30, Height: 16})
	bareGround(g)
	ada := draftedPawn(t, g, 10, 8)
	deer, _ := g.SpawnAnimal("deer", 12, 8)
	bruiseDown(t, g, deer)
	at := pos(g, deer)
	for i := 0; i < 600; i++ {
		if i%120 == 0 {
			g.hurt(deer, 1, ada)
		}
		g.Step(1)
		a, ok := g.animals.Get(deer)
		if !ok {
			t.Fatalf("tick %d: the deer died", i)
		}
		if a.Mode == animal.Flee || pos(g, deer) != at || !a.Downed {
			t.Fatalf("tick %d: a downed deer lies still: %+v at %v", i, a, pos(g, deer))
		}
	}
	if av := animalViewOf(t, g, deer); !av.Downed || !av.Sleeping || av.State != "downed" || av.Action != "" {
		t.Fatalf("the view says downed and draws the rest pose: %+v", av)
	}

	h := New(Config{Seed: 63, Width: 30, Height: 16})
	bareGround(h)
	bo := draftedPawn(t, h, 10, 8)
	wolf, _ := h.SpawnAnimal("timber_wolf", 14, 8)
	a := animalOf(h, wolf)
	h.enrage(wolf, &a, bo, uint64(h.clock.Now()))
	h.animals.Set(wolf, a)
	bruiseDown(t, h, wolf)
	at = pos(h, wolf)
	h.Step(300)
	if a := animalOf(h, wolf); a.Mode == animal.Attack || a.Target != 0 || h.hostile(a) || pos(h, wolf) != at {
		t.Fatalf("a downed manhunter stops hunting: %+v at %v", a, pos(h, wolf))
	}
}

// A tame dog downed by a deep cut is tended where it lies: the vet comes
// to it, the bleeding stops, and the dog never moves.
func TestDownedAnimalIsTendedInPlace(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 64)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	g.SpawnStack(item.HerbalMedicine, 3, 8, 3)
	g.setPriority(ada, work.Doctor, 1)
	cutAnimal(g, dog, injuryPct(g, dog, 82))
	if a := animalOf(g, dog); !a.Downed {
		t.Fatalf("the cut downs the dog: %+v", a)
	}
	at := pos(g, dog)
	moved := false
	tended := runCare(g, 300, func() bool {
		moved = moved || pos(g, dog) != at
		return g.tended(dog)
	}, ada)
	if !tended || moved {
		t.Fatalf("tended %v, moved %v: %+v", tended, moved, animalOf(g, dog))
	}
	if a := animalOf(g, dog); !a.Downed || a.Body == nil || a.Body.BleedPerDay() != 0 {
		t.Fatalf("still down, bleeding stopped: %+v", a)
	}
}

// A hunter finishes a downed marked animal before a nearer standing one.
func TestHunterFinishesDownedAnimal(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 65, Width: 40, Height: 16})
	bareGround(g)
	p, _ := g.SpawnNamedPawn(2, 8, "Hunter")
	g.equip[p] = item.Rifle
	near, _ := g.SpawnAnimal("dustbuck", 8, 8)
	down, _ := g.SpawnAnimal("dustbuck", 20, 8)
	bruiseDown(t, g, down)
	for _, id := range []entity.EntityID{near, down} {
		setAnimal(g, id, func(a *animal.State) { a.Designated = true })
	}
	if !g.combat.Hunting.Assign(p) {
		t.Fatal("the hunter takes a hunt")
	}
	if j, _ := g.jobs.Get(p); j.Target != down {
		t.Fatalf("the downed buck comes first: target %d, want %d", j.Target, down)
	}
	for i := 0; i < 60 && animalAlive(g, down); i++ {
		g.Step(50)
	}
	if animalAlive(g, down) {
		t.Fatalf("the hunter finished the downed buck: %+v", animalOf(g, down))
	}
}

// A predator takes a downed animal over a nearer standing one, even one
// it would not hunt standing (another predator).
func TestPredatorPrefersDownedPrey(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 66, Width: 40, Height: 16})
	bareGround(g)
	wolf, _ := g.SpawnAnimal("timber_wolf", 10, 8)
	hare, _ := g.SpawnAnimal("hare", 12, 8)
	cougar, _ := g.SpawnAnimal("cougar", 16, 8)
	if g.huntable(wolf, cougar) {
		t.Fatal("a standing cougar is no prey")
	}
	bruiseDown(t, g, cougar)
	if got := g.findPrey(wolf, pos(g, wolf), animal.DefOf(animal.TimberWolf)); got != cougar {
		t.Fatalf("the downed cougar is easy prey: got %d, hare %d, cougar %d", got, hare, cougar)
	}
}

// Taming a downed wild animal is twice as likely (at most certain).
func TestDownedWildAnimalTamesEasier(t *testing.T) {
	t.Parallel()
	up, down, cow := animal.State{Species: animal.Deer}, animal.State{Species: animal.Deer, Downed: true}, animal.State{Species: animal.Cow, Downed: true}
	if herd.Chance(10, up) != 170 || herd.Chance(10, down) != 340 || herd.Chance(20, cow) != 1000 {
		t.Fatalf("tame chance: up %d, down %d, capped %d", herd.Chance(10, up), herd.Chance(10, down), herd.Chance(20, cow))
	}
	// a downed animal holds still for the handler, who tames it with the
	// food it brings (FOOD-3)
	g := New(Config{Seed: 67, Width: 30, Height: 12})
	bareGround(g)
	h, _ := g.SpawnNamedPawn(4, 6, "Handler")
	setSkill(g, h, skill.Animals, 10)
	spawn(t, g, item.Potatoes, 75, 3, 6)
	deer, _ := g.SpawnAnimal("deer", 10, 6)
	bruiseDown(t, g, deer)
	setAnimal(g, deer, func(a *animal.State) { a.TameMark = true })
	for i := 0; i < 200 && !animalOf(g, deer).Tame; i++ {
		g.Step(30)
		if !animalAlive(g, deer) {
			t.Fatal("the deer died")
		}
		if a := animalOf(g, deer); a.TameRetry != 0 && !a.Tame {
			setAnimal(g, deer, func(a *animal.State) { a.TameRetry, a.Manhunter, a.TameMark = 0, 0, true })
		}
	}
	if a := animalOf(g, deer); !a.Tame || !a.Downed {
		t.Fatalf("the downed deer is tamed where it lies: %+v", a)
	}
}

// The downed state and the tick it began survive a save.
func TestDownedStateSurvivesSave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 68, Width: 24, Height: 14})
	deer, _ := g.SpawnAnimal("deer", 12, 7)
	bruiseDown(t, g, deer)
	want := animalOf(g, deer)
	h := saveLoad(t, g)
	got, ok := h.animals.Get(deer)
	if !ok || !got.Downed || got.DownedTick != want.DownedTick {
		t.Fatalf("after load: %+v, want downed at %d", got, want.DownedTick)
	}
	h.Step(120)
	if a := animalOf(h, deer); !a.Downed || pos(h, deer) != pos(g, deer) {
		t.Fatalf("still lying there after load: %+v", a)
	}
}
