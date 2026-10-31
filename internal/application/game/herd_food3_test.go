package game

import (
	"testing"

	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// FOOD-3: animal husbandry at RimWorld fidelity (game/herd).

// slaughterOne runs a handler until animal aid is dead (or fails).
func slaughterOne(t *testing.T, g *Game, handler, aid entity.EntityID) {
	t.Helper()
	sawJob := false
	for i := 0; i < 200 && animalAlive(g, aid); i++ {
		g.Step(10)
		g.SetNeeds(handler, needs.Full, needs.Full)
		if j, _ := g.jobs.Get(handler); j.Kind == job.Slaughter {
			sawJob = true
		}
	}
	if animalAlive(g, aid) || !sawJob {
		t.Fatalf("slaughtered %v (job seen %v): %+v", !animalAlive(g, aid), sawJob, animalOf(g, aid))
	}
}

// TestHandlerSlaughtersWithNoWeapon: a tame cow marked for slaughter is
// killed by a handler with no weapon (Handling work), carefully: its body
// is a whole one (wiki: Animal husbandry).
func TestHandlerSlaughtersWithNoWeapon(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 30, Height: 14})
	bareGround(g)
	ada, _ := g.SpawnNamedPawn(4, 7, "Ada")
	onlyWork(g, ada, work.Handle)
	if _, armed := g.equip[ada]; armed {
		t.Fatal("the handler is unarmed")
	}
	cow := spawnSexed(t, g, "cow", 14, 7, true, true, "Bess")
	g.Apply(command.Command{Kind: command.DesignateHunt, X: 14, Y: 7, X2: 14, Y2: 7}) // the Slaughter button on a tame animal
	g.Step(1)
	if !animalOf(g, cow).Designated {
		t.Fatal("marked for slaughter")
	}
	slaughterOne(t, g, ada, cow)
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.CorpseAnimal {
			if m, l := animal.CorpseYield(animal.Cow, int(st.Yield)); m != 336 || l != 96 {
				t.Fatalf("a slaughtered adult cow is a whole body: %d meat, %d leather (yield %d)", m, l, st.Yield)
			}
			return
		}
	}
	t.Fatal("no cow corpse")
}

// TestCowButchersByAge: a juvenile cow butchers to half the wiki's 336
// meat (and a cook at Cooking 10 at full efficiency), as "cow meat".
func TestCowButchersByAge(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 30, Height: 14})
	bareGround(g)
	cook, _ := g.SpawnNamedPawn(4, 7, "Cook")
	onlyWork(g, cook, work.Handle, work.Cook)
	setSkill(g, cook, skill.Cooking, 10)
	placeBuilt(t, g, construction.ButcherTable, construction.South, 8, 3)
	calf := spawnSexed(t, g, "cow", 14, 7, true, true, "Calf")
	now := uint64(g.clock.Now())
	setAnimal(g, calf, func(a *animal.State) {
		a.Born = animal.BornAt(now, uint64(animal.LifeOf(animal.Cow).AdultDays-2)*clock.TicksPerDay)
		a.Designated = true
	})
	slaughterOne(t, g, cook, calf)
	for i := 0; i < 200 && itemsOnGround(g, "meat") == 0; i++ {
		g.Step(30)
		g.SetNeeds(cook, needs.Full, needs.Full)
	}
	if got := itemsOnGround(g, "meat"); got != 168 {
		t.Fatalf("a juvenile cow's meat = %d, want 168 (half of 336)", got)
	}
	for _, v := range g.World().Items {
		if v.Kind == "meat" && v.Meat != "cow" {
			t.Fatalf("the meat is cow meat: %+v", v)
		}
	}
}

// TestAutoSlaughterKeepsTheHerdAtItsCap: six tame dogs (pets keep near
// home; unpenned livestock may roam off), a cap of four females: the
// daily check marks the two oldest, the handler slaughters them, and day
// after day nothing more is marked or unmarked (no flap).
func TestAutoSlaughterKeepsTheHerdAtItsCap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 40, Height: 16})
	bareGround(g)
	ada, _ := g.SpawnNamedPawn(4, 8, "Ada")
	onlyWork(g, ada, work.Handle)
	var cows []entity.EntityID
	now := uint64(g.clock.Now())
	for i := range 6 {
		id := spawnSexed(t, g, "labrador", 12+3*i, 8, true, true, "Dog")
		age := uint64(100+10*i) * clock.TicksPerDay // the last ones oldest
		setAnimal(g, id, func(a *animal.State) { a.Born = animal.BornAt(now, age) })
		cows = append(cows, id)
	}
	g.Apply(command.Command{Kind: command.SetAutoSlaughter, Name: "labrador", X: -1, Y: 4, X2: -1})
	g.Step(1)
	if v := g.World().AutoSlaughter; len(v) != 1 || v[0].Species != "labrador" || v[0].MaxFemales != 4 || v[0].MaxMales != -1 {
		t.Fatalf("view %+v", v)
	}
	marks := []int{}
	for day := 0; day < 4; day++ {
		for i := 0; i < clock.TicksPerDay/250; i++ {
			g.Step(250)
			g.SetNeeds(ada, needs.Full, needs.Full)
			for _, id := range cows {
				if animalAlive(g, id) {
					keepFed(g, id)
				}
			}
		}
		n := 0
		for _, id := range cows {
			if animalAlive(g, id) && animalOf(g, id).Designated {
				n++
			}
		}
		marks = append(marks, n)
	}
	alive := 0
	for _, id := range cows {
		if animalAlive(g, id) {
			alive++
		}
	}
	if alive != 4 || animalAlive(g, cows[5]) || animalAlive(g, cows[4]) {
		t.Fatalf("the two oldest go: %d alive", alive)
	}
	for _, n := range marks {
		if n != 0 {
			t.Fatalf("a herd at its cap is left alone, marks by day %v", marks)
		}
	}
	if marked := g.herdOf().Auto.Check(); marked != 0 {
		t.Fatalf("a second check marks %d (no flap)", marked)
	}
}

// TestTamingCostsFood: the handler fetches the food the animal eats and
// feeds it; with no such food there is no taming job.
func TestTamingCostsFood(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 6, Width: 30, Height: 14})
	bareGround(g)
	ada, _ := g.SpawnNamedPawn(4, 7, "Ada")
	onlyWork(g, ada, work.Handle)
	setSkill(g, ada, skill.Animals, 10)
	goat, _ := g.SpawnAnimal("goat", 16, 7)
	spawn(t, g, item.Meal, 5, 4, 9) // meals are no taming food
	g.Apply(command.Command{Kind: command.DesignateTame, X: 16, Y: 7, X2: 16, Y2: 7})
	if g.herdOf().Taming.Assign(ada) {
		t.Fatal("no taming without raw food or kibble")
	}
	spawn(t, g, item.Potatoes, 20, 4, 10)
	want := 20 - (animal.TameFood(animal.Goat)+item.Potatoes.Nutrition()-1)/item.Potatoes.Nutrition()
	done := false
	for i := 0; i < 300 && !done; i++ {
		g.Step(20)
		g.SetNeeds(ada, needs.Full, needs.Full)
		done = animalOf(g, goat).Tame || animalOf(g, goat).TameRetry != 0
	}
	if !done || itemsOnGround(g, "potatoes") != want {
		t.Fatalf("one attempt (done %v) ate %d potatoes, want %d left", done, itemsOnGround(g, "potatoes"), want)
	}
	if animal.MinHandling(animal.Wildness(animal.Ostrich)) <= 0 {
		t.Fatal("a wild species needs a skilled handler")
	}
}

// TestHunterHaulsItsKill: after the kill the hunter carries the body to
// the stockpile (wiki: Hunt).
func TestHunterHaulsItsKill(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 65, Width: 40, Height: 16})
	bareGround(g)
	p, _ := g.SpawnNamedPawn(2, 8, "Hunter")
	onlyWork(g, p, work.Hunt)
	g.equip[p] = item.Rifle
	applyNow(g, command.Command{Kind: command.DesignateStockpile, X: 2, Y: 2, X2: 4, Y2: 4})
	sz := zoneViewAt(t, g, 3, 3)
	applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(sz.ID), Name: "Corpses/Animal", Enabled: true})
	buck, _ := g.SpawnAnimal("dustbuck", 20, 8)
	bruiseDown(t, g, buck)
	setAnimal(g, buck, func(a *animal.State) { a.Designated = true })
	hauled := false
	for i := 0; i < 400 && !hauled; i++ {
		g.Step(10)
		g.SetNeeds(p, needs.Full, needs.Full)
		if j, _ := g.jobs.Get(p); j.Kind == job.Haul {
			if st, ok := g.stacks.Get(j.Target); ok && st.Kind == item.CorpseAnimal {
				hauled = true
			}
		}
	}
	if animalAlive(g, buck) || !hauled {
		t.Fatalf("killed %v, hunter hauls the body %v", !animalAlive(g, buck), hauled)
	}
	for i := 0; i < 100; i++ {
		g.Step(20)
		g.SetNeeds(p, needs.Full, needs.Full)
	}
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.CorpseAnimal {
			if st.X > 4 || st.Y > 4 {
				t.Fatalf("the body lies in the stockpile: %+v", st)
			}
		}
	}
}

// TestButcherHumanGivesHumanMeatAndMoods: a human body at the butcher
// table gives human meat; the butcher remembers butchering it, the others
// that someone did.
func TestButcherHumanGivesHumanMeatAndMoods(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 30, Height: 14})
	bareGround(g)
	cook, _ := g.SpawnNamedPawn(4, 7, "Cook")
	other, _ := g.SpawnNamedPawn(20, 12, "Other")
	onlyWork(g, cook, work.Cook)
	onlyWork(g, other)
	setSkill(g, cook, skill.Cooking, 10)
	placeBuilt(t, g, construction.ButcherTable, construction.South, 8, 3)
	if w := g.World(); len(w.Buildings[0].Bills) != 1 {
		t.Fatalf("butchering humans is no default bill: %+v", w.Buildings[0].Bills)
	}
	g.Enqueue(commandAddBill(8, 3, recipe.ButcherHuman))
	spawn(t, g, item.CorpseHuman, 1, 5, 9)
	for i := 0; i < 200 && itemsOnGround(g, "human meat") == 0; i++ {
		g.Step(30)
		g.SetNeeds(cook, needs.Full, needs.Full)
		g.SetNeeds(other, needs.Full, needs.Full)
	}
	if got := itemsOnGround(g, "human meat"); got != animal.HumanMeatYield {
		t.Fatalf("human meat %d", got)
	}
	if !remembers(g, cook, mood.ButcheredHuman) || !remembers(g, other, mood.ObservedButchery) || remembers(g, other, mood.ButcheredHuman) {
		t.Fatalf("moods: butcher %+v, other %+v", g.memories[cook], g.memories[other])
	}
}

// TestDownedAnimalsGetFed: a handler brings food to a hungry downed tame
// animal and feeds it where it lies.
func TestDownedAnimalsGetFed(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 8, Width: 30, Height: 14})
	bareGround(g)
	ada, _ := g.SpawnNamedPawn(4, 7, "Ada")
	onlyWork(g, ada, work.Handle)
	dog := spawnSexed(t, g, "labrador", 16, 7, true, true, "Rex")
	bruiseDown(t, g, dog)
	setAnimal(g, dog, func(a *animal.State) { a.Food = animal.FeedStart / 2 })
	spawn(t, g, item.Kibble, 40, 4, 9)
	fed := false
	for i := 0; i < 300 && !fed; i++ {
		g.Step(10)
		g.SetNeeds(ada, needs.Full, needs.Full)
		fed = animalOf(g, dog).Food >= animal.FeedStart && itemsOnGround(g, "kibble") < 40
	}
	if !fed || !animalOf(g, dog).Downed {
		t.Fatalf("the downed dog is fed where it lies: %+v, kibble %d", animalOf(g, dog), itemsOnGround(g, "kibble"))
	}
	if a := animalOf(g, dog); herd.Due(&a) {
		t.Fatal("a fed animal is not due again (FeedStart..FeedStop band)")
	}
}

// TestAutoSlaughterCapsSurviveASave: the caps are saved with the wildlife
// state, and a corpse keeps its butcher yield.
func TestAutoSlaughterCapsSurviveASave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 20, Height: 10})
	applyNow(g, command.Command{Kind: command.SetAutoSlaughter, Name: "goat", X: 1, Y: 3, X2: 0})
	sid := g.dropCorpseAs(animal.Cow, 5, 5, false, 500)
	l := saveLoad(t, g)
	if v := l.World().AutoSlaughter; len(v) != 1 || v[0] != (AutoSlaughterView{Species: "goat", MaxMales: 1, MaxFemales: 3, MaxYoung: 0}) {
		t.Fatalf("caps after a load: %+v", v)
	}
	if st, ok := l.stacks.Get(sid); !ok || st.Yield != 500 {
		t.Fatalf("corpse yield after a load: %+v", st)
	}
}
