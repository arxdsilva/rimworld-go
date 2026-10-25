package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
)

// Animal needs and states (SIM-024, plan 2.3).

// bareGround takes every blade of grass off the map.
func bareGround(g *Game) {
	s := g.seedGrass()
	clear(s.cells)
}

// sowGrass puts grass on every tile of the rectangle.
func sowGrass(g *Game, x0, y0, x1, y1 int) {
	s := g.seedGrass()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			s.cells[y*g.grid.Width()+x] = 1
		}
	}
}

func grassCount(g *Game) int {
	n := 0
	for _, c := range g.seedGrass().cells {
		n += int(c)
	}
	return n
}

func setAnimal(g *Game, id entity.EntityID, f func(a *animal.State)) {
	a, _ := g.animals.Get(id)
	f(&a)
	g.animals.Set(id, a)
}

func animalOf(g *Game, id entity.EntityID) animal.State {
	a, _ := g.animals.Get(id)
	return a
}

func TestHungryDeerGrazesAndStops(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	sowGrass(g, 12, 8, 17, 12) // 30 tiles
	deer, _ := g.SpawnAnimal("deer", 10, 10)
	setAnimal(g, deer, func(a *animal.State) { a.Food = 3000 })
	fed, stopped := false, false
	for i := 0; i < 4000 && !stopped; i++ {
		g.Step(1)
		a := animalOf(g, deer)
		if a.Mode == animal.Feed {
			fed = true
		}
		if fed && a.Mode == animal.Wander {
			stopped = true
			if a.Food < animal.FeedStop {
				t.Fatalf("stopped eating at %d, want >= %d", a.Food, animal.FeedStop)
			}
		}
	}
	if !fed || !stopped {
		t.Fatalf("the deer grazes and then stops: fed %v stopped %v, %+v", fed, stopped, animalOf(g, deer))
	}
	if eaten := 30 - grassCount(g); eaten < 4 || eaten > 6 {
		t.Fatalf("grazed %d tiles from 30%% to 95%%+ (1500 a tile), want 5", eaten)
	}
	if len(g.grass.regrow) == 0 {
		t.Fatal("grazed tiles grow back (queued for regrowth)")
	}
	// sated: it does not look for food again soon
	for i := 0; i < 2000; i++ {
		g.Step(1)
		if animalOf(g, deer).Mode == animal.Feed {
			t.Fatal("a sated deer went back to feeding")
		}
	}
}

func TestWildDeerEatsCrop(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	crop, _ := g.spawnPlant(plant.Potato, 14, 10, plant.DefOf(plant.Potato).GrowTicks)
	setPlant := func(f func(p *plant.Plant)) {
		p, _ := g.plants.Get(crop)
		f(&p)
		g.plants.Set(crop, p)
		g.notePlantMark(crop) // jobindex.go: as doSow does
	}
	setPlant(func(p *plant.Plant) { p.Zone = true })
	deer, _ := g.SpawnAnimal("deer", 10, 10)
	setAnimal(g, deer, func(a *animal.State) { a.Food = 3000 })
	for i := 0; i < 1500 && animalOf(g, deer).Food < 4000; i++ {
		g.Step(1)
	}
	p, ok := g.plants.Get(crop)
	if !ok || p.Percent() != 100-animal.CropBite {
		t.Fatalf("a wild deer eats 40%% of a crop: %+v (alive %v)", p, ok)
	}
	if r := g.CropRaids(); len(r) != 1 || r[0].X != 14 || r[0].Species != "deer" {
		t.Fatalf("crop raid for the alert: %+v", r)
	}
	// a crop eaten under 20% dies
	setPlant(func(p *plant.Plant) { p.Growth = plant.DefOf(plant.Potato).GrowTicks * 50 / 100 })
	setAnimal(g, deer, func(a *animal.State) { a.Food, a.NextDecide = 3000, 0 })
	for i := 0; i < 1500 && animalOf(g, deer).Food < 4000; i++ {
		g.Step(1)
	}
	if _, ok := g.plants.Get(crop); ok {
		t.Fatal("a crop under 20% after a meal dies")
	}
	// a tame deer leaves crops alone
	h := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(h)
	c2, _ := h.spawnPlant(plant.Potato, 14, 10, plant.DefOf(plant.Potato).GrowTicks)
	p2, _ := h.plants.Get(c2)
	p2.Zone = true
	h.plants.Set(c2, p2)
	h.notePlantMark(c2) // jobindex.go: as doSow does
	pet, _ := h.SpawnAnimal("deer", 10, 10)
	h.TameAnimal(pet, "Clover")
	setAnimal(h, pet, func(a *animal.State) { a.Food = 3000 })
	h.Step(600)
	if p, _ := h.plants.Get(c2); p.Percent() != 100 || len(h.CropRaids()) != 0 {
		t.Fatalf("a tame deer ate the crop: %+v", p)
	}
}

func TestWolfEatsItsKill(t *testing.T) {
	g := New(Config{Seed: 2, Width: 30, Height: 20})
	bareGround(g)
	wolf, _ := g.SpawnAnimal("timber_wolf", 10, 10)
	deer, _ := g.SpawnAnimal("deer", 11, 10)
	setAnimal(g, wolf, func(a *animal.State) { a.Food = 2000 })
	setAnimal(g, deer, func(a *animal.State) { a.Injury = animal.DefOf(animal.Deer).MaxHP - 1 })
	ate := false
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if a := animalOf(g, wolf); a.Mode == animal.Feed && a.Feed.Kind == animal.FeedCarcass {
			ate = true
		}
		if !animalAlive(g, deer) && itemsOnGround(g, "deer corpse") == 0 {
			break
		}
	}
	if animalAlive(g, deer) {
		t.Fatal("the hungry wolf kills the deer")
	}
	if !ate {
		t.Fatal("the killer feeds on its kill")
	}
	if n := itemsOnGround(g, "deer corpse"); n != 0 {
		t.Fatalf("eating consumes the carcass: %d left", n)
	}
	if a := animalOf(g, wolf); a.Food < animal.Full-10 || a.Mode == animal.Attack {
		t.Fatalf("a carcass fills the wolf: %+v", a)
	}
	if !hasLog(g, wolf, "Ate deer carcass") {
		t.Fatalf("log %q", logTexts(g, wolf))
	}
}

func TestStarvingAnimalDies(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	bareGround(g)
	deer, _ := g.SpawnAnimal("deer", 10, 6)
	g.TameAnimal(deer, "Clover")
	setAnimal(g, deer, func(a *animal.State) { a.Food = 1 })
	g.Step(5)
	if a := animalOf(g, deer); a.Food != 0 || a.Starving == 0 || !hasLog(g, deer, "Starving") {
		t.Fatalf("food ran out: %+v, log %q", a, logTexts(g, deer))
	}
	g.Step(animal.StarveEvery * 3)
	if a := animalOf(g, deer); a.Injury != 3 {
		t.Fatalf("one injury per %d ticks starving, got %d", animal.StarveEvery, a.Injury)
	}
	for i := 0; i < 60 && animalAlive(g, deer); i++ {
		g.Step(animal.StarveEvery)
	}
	if animalAlive(g, deer) {
		t.Fatal("a starving animal dies")
	}
	if !hasMessage(g, "Clover the deer has starved to death.") {
		t.Fatalf("messages %+v", g.messages)
	}
}

func TestAnimalsSleepAtNight(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	jumpTo(g, 1, 20)
	deer, _ := g.SpawnAnimal("deer", 10, 10)
	setAnimal(g, deer, func(a *animal.State) { a.Rest = 5000 })
	g.Step(2 * clock.TicksPerHour) // 22:00
	a := animalOf(g, deer)
	if a.Mode != animal.Rest || !sleeping(a) || !animalView(t, g, deer).Sleeping {
		t.Fatalf("a deer sleeps at night: %+v", a)
	}
	r0 := a.Rest
	g.Step(clock.TicksPerHour)
	if a := animalOf(g, deer); a.Rest-r0 < animal.SleepPerHour-10 {
		t.Fatalf("an hour of sleep restores %d, got %d", animal.SleepPerHour, a.Rest-r0)
	}
	jumpTo(g, 2, 9)
	g.Step(animal.DecideEvery + 1)
	if a := animalOf(g, deer); a.Mode == animal.Rest || animalView(t, g, deer).Sleeping {
		t.Fatalf("awake in the morning: %+v", a)
	}
}

func TestNocturnalSleepsByDay(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	jumpTo(g, 1, 11)
	coon, _ := g.SpawnAnimal("raccoon", 10, 10)
	deer, _ := g.SpawnAnimal("deer", 20, 10)
	for _, id := range []entity.EntityID{coon, deer} {
		setAnimal(g, id, func(a *animal.State) { a.Rest = 5000 })
	}
	g.Step(animal.DecideEvery + 1)
	if animalOf(g, coon).Mode != animal.Rest {
		t.Fatal("a raccoon sleeps by day")
	}
	if animalOf(g, deer).Mode == animal.Rest {
		t.Fatal("a deer is up by day")
	}
	jumpTo(g, 1, 23)
	setAnimal(g, coon, func(a *animal.State) { a.Rest = 6500 })
	g.Step(animal.DecideEvery + 1)
	if animalOf(g, coon).Mode == animal.Rest {
		t.Fatal("a raccoon is up at night")
	}
	if animalOf(g, deer).Mode != animal.Rest {
		t.Fatal("the deer beds down at night")
	}
}

func TestWokenTerritorialFightsBack(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	jumpTo(g, 1, 22)
	ada, _ := g.SpawnNamedPawn(12, 10, "Ada")
	g.SetNeeds(ada, needs.Full, needs.Full)
	bison, _ := g.SpawnAnimal("bison", 10, 10)
	setAnimal(g, bison, func(a *animal.State) { a.Rest = 3000 })
	g.Step(animal.DecideEvery + 1)
	if animalOf(g, bison).Mode != animal.Rest {
		t.Fatal("the bison sleeps")
	}
	g.hurt(bison, 3, ada)
	g.Step(1)
	if a := animalOf(g, bison); a.Mode != animal.Attack || a.Target != ada {
		t.Fatalf("a woken bison goes for whoever woke it: %+v", a)
	}
	// a sleeping deer just runs
	deer, _ := g.SpawnAnimal("deer", 20, 15)
	setAnimal(g, deer, func(a *animal.State) { a.Mode, a.Rest = animal.Rest, 3000 })
	g.hurt(deer, 1, ada)
	if a := animalOf(g, deer); a.Mode != animal.Flee {
		t.Fatalf("a hurt sleeping deer flees: %+v", a)
	}
}

func TestHurtPredatorBreaksOff(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 20})
	bareGround(g)
	wolf, _ := g.SpawnAnimal("timber_wolf", 11, 10)
	deer, _ := g.SpawnAnimal("deer", 13, 10)
	now := uint64(g.clock.Now())
	setAnimal(g, wolf, func(a *animal.State) {
		a.Mode, a.Target, a.HuntSince, a.Food = animal.Attack, deer, now, 2000
	})
	g.Step(5)
	if a := animalOf(g, wolf); a.Mode != animal.Attack {
		t.Fatalf("a healthy wolf hunts on: %+v", a)
	}
	maxHP := animal.DefOf(animal.TimberWolf).MaxHP
	g.hurt(wolf, maxHP-maxHP*animal.BadlyHurt/100+1, deer) // the deer kicks back
	g.Step(1)
	if a := animalOf(g, wolf); a.Mode != animal.Flee || a.Target != 0 {
		t.Fatalf("a badly hurt wolf breaks off: %+v", a)
	}
	for i := 0; i < 3*breakOffTicks; i++ { // hungry, but too hurt to hunt (no flee/hunt flapping)
		g.Step(1)
		if a := animalOf(g, wolf); a.Mode == animal.Attack {
			t.Fatalf("tick %d: back in the fight while badly hurt: %+v", i, a)
		}
	}
	// an enraged one fights to the death
	h := New(Config{Seed: 1, Width: 40, Height: 20})
	bo := draftedPawn(t, h, 10, 10)
	mad, _ := h.SpawnAnimal("timber_wolf", 11, 10)
	a := animalOf(h, mad)
	h.enrage(mad, &a, bo, uint64(h.clock.Now()))
	a.Injury = maxHP - 2
	h.animals.Set(mad, a)
	h.Step(1)
	if a := animalOf(h, mad); a.Mode != animal.Attack {
		t.Fatalf("a manhunter does not break off: %+v", a)
	}
}

func TestPredatorGivesUpChase(t *testing.T) {
	g := New(Config{Seed: 1, Width: 80, Height: 20})
	bareGround(g)
	wolf, _ := g.SpawnAnimal("timber_wolf", 5, 10)
	hare, _ := g.SpawnAnimal("hare", 10, 10)
	now := uint64(g.clock.Now())
	setAnimal(g, wolf, func(a *animal.State) {
		a.Mode, a.Target, a.Food = animal.Attack, hare, 2000
		a.HuntSince = now - animal.HuntGiveUp - 1 // no strike for too long
	})
	g.Step(1)
	a := animalOf(g, wolf)
	if a.Target != 0 || a.HuntSince != 0 || a.NextFeed <= now {
		t.Fatalf("a hunt without a strike for %d ticks ends: %+v", animal.HuntGiveUp, a)
	}
	for i := 0; uint64(i) < animal.HuntRetry-2; i++ {
		g.Step(1)
		if animalOf(g, wolf).Target != 0 {
			t.Fatalf("tick %d: hunting again before the retry", i)
		}
	}
	// prey beyond the leash
	h := New(Config{Seed: 1, Width: 80, Height: 20})
	w2, _ := h.SpawnAnimal("timber_wolf", 5, 10)
	far, _ := h.SpawnAnimal("hare", 5+22+animal.HuntLeash+2, 10)
	setAnimal(h, w2, func(a *animal.State) {
		a.Mode, a.Target, a.HuntSince, a.Food = animal.Attack, far, uint64(h.clock.Now()), 2000
	})
	h.Step(1)
	if a := animalOf(h, w2); a.Target != 0 {
		t.Fatalf("prey beyond the hunt radius plus %d escapes: %+v", animal.HuntLeash, a)
	}
}

func TestPenTroughFeedsInWinter(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	jumpTo(g, seasonDay(t, "Winter", 1), 10)
	bareGround(g)
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(ada, wt, 0)
	}
	g.setPriority(ada, work.Haul, work.DefaultPriority)
	g.SpawnStack(item.Berries, 40, 3, 15)
	// PLAY-015: a fenced pen with a marker (painted pens are gone)
	fenceRing(t, g, 14, 4, 23, 13, [2]int{18, 13})
	buildMarker(t, g, 16, 10)
	placeBuilt(t, g, construction.FeedingTrough, construction.South, 18, 8)
	var bid entity.EntityID
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.FeedingTrough {
			bid = id
		}
	}
	if bid == 0 {
		t.Fatal("trough built")
	}
	deer, _ := g.SpawnAnimal("deer", 16, 6)
	g.TameAnimal(deer, "Clover")
	for i := 0; i < 200 && g.troughFill(bid) == 0; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(25)
	}
	fill := g.troughFill(bid)
	if fill == 0 || fill > construction.TroughCapacity {
		t.Fatalf("a hauler fills the trough: %d", fill)
	}
	if itemsOnGround(g, "berries") != 40-fill {
		t.Fatalf("berries left on the map %d, in the trough %d", itemsOnGround(g, "berries"), fill)
	}
	setAnimal(g, deer, func(a *animal.State) { a.Food, a.NextDecide = 3000, 0 })
	fromTrough := false
	for i := 0; i < 1500 && animalOf(g, deer).Food < 4000; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(1)
		if animalOf(g, deer).Feed.Kind == animal.FeedTrough {
			fromTrough = true
		}
	}
	if !fromTrough || animalOf(g, deer).Food < 4000 || g.troughFill(bid) >= fill {
		t.Fatalf("the penned deer eats from the trough: %+v, trough %d of %d", animalOf(g, deer), g.troughFill(bid), fill)
	}
	if bv := buildingView(g, bid); bv.TroughFill != g.troughFill(bid) || bv.TroughFilter != "plants" {
		t.Fatalf("trough view %+v", bv)
	}
}

func buildingView(g *Game, id entity.EntityID) BuildingView {
	for _, b := range g.World().Buildings {
		if b.ID == id {
			return b
		}
	}
	return BuildingView{}
}

func TestTroughFilterCommand(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	fenceRing(t, g, 14, 4, 23, 13, [2]int{18, 13}) // PLAY-015: a marker pen
	buildMarker(t, g, 16, 10)
	wolf, _ := g.SpawnAnimal("timber_wolf", 16, 6)
	g.TameAnimal(wolf, "Ash")
	placeBuilt(t, g, construction.FeedingTrough, construction.South, 18, 8)
	var bid entity.EntityID
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.FeedingTrough {
			bid = id
		}
	}
	g.Step(troughSweepEvery)
	if f := buildingView(g, bid).TroughFilter; f != "meat" {
		t.Fatalf("a wolf pen's trough defaults to meat, got %q", f)
	}
	g.Enqueue(command.Command{Kind: command.SetTroughFilter, Target: uint64(bid), Mode: TroughAny})
	g.Step(1)
	if f := buildingView(g, bid).TroughFilter; f != "any" {
		t.Fatalf("filter after the command %q", f)
	}
	g.Enqueue(command.Command{Kind: command.SetTroughFilter, Target: uint64(bid), Mode: 9})
	g.Enqueue(command.Command{Kind: command.SetTroughFilter, Target: 99999, Mode: TroughMeat})
	g.Step(1)
	if f := buildingView(g, bid).TroughFilter; f != "any" {
		t.Fatalf("bad filter commands are ignored, got %q", f)
	}
	// the filter decides what goes in
	g.Enqueue(command.Command{Kind: command.SetTroughFilter, Target: uint64(bid), Mode: TroughMeat})
	g.Step(1)
	b, _ := g.bldgs.Get(bid)
	sid, _ := g.SpawnStack(item.Berries, 5, b.X, b.Y)
	g.absorbInto(bid, sid)
	if g.troughFill(bid) != 0 {
		t.Fatal("a meat trough refuses berries")
	}
	mid, _ := g.SpawnStack(item.Meat, 80, b.X+1, b.Y)
	g.absorbInto(bid, mid)
	if g.troughFill(bid) != construction.TroughCapacity || itemsOnGround(g, "meat") != 80-construction.TroughCapacity {
		t.Fatalf("a trough takes meat up to its capacity: %d, %d left", g.troughFill(bid), itemsOnGround(g, "meat"))
	}
}

// feedFlapRun steps a deer in a meadow and checks every switch between
// wandering and feeding against the thresholds.
func TestNoFeedFlap(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 30})
	bareGround(g)
	sowGrass(g, 0, 0, 39, 29)
	deer, _ := g.SpawnAnimal("deer", 20, 15)
	setAnimal(g, deer, func(a *animal.State) { a.Food = animal.FeedStart + 50 })
	prev, starts := animal.Wander, 0
	for i := 0; i < clock.TicksPerDay; i++ {
		g.Step(1)
		a := animalOf(g, deer)
		if a.Mode == animal.Rest || a.Mode == prev {
			continue
		}
		switch {
		case prev != animal.Feed && a.Mode == animal.Feed:
			starts++
			if a.Food >= animal.FeedStart {
				t.Fatalf("tick %d: started feeding at %d", i, a.Food)
			}
		case prev == animal.Feed && a.Food < animal.FeedStop && a.NextFeed <= uint64(g.clock.Now()):
			t.Fatalf("tick %d: stopped feeding at %d with food around", i, a.Food)
		}
		prev = a.Mode
	}
	if starts == 0 || starts > 2 {
		t.Fatalf("feeding bouts in a day: %d, want 1-2", starts)
	}
}

func TestNoRestFlap(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 30})
	sowGrass(g, 0, 0, 39, 29)
	deer, _ := g.SpawnAnimal("deer", 20, 15)
	beds := 0
	resting := false
	for i := 0; i < 2*clock.TicksPerDay; i++ {
		g.Step(1)
		a := animalOf(g, deer)
		h := g.Date().Hour
		switch now := a.Mode == animal.Rest; {
		case now && !resting:
			beds++
			if !animal.SleepHours(animal.Deer, h) && a.Rest >= animal.RestCollapse {
				t.Fatalf("tick %d: bed down at %d:00 with rest %d", i, h, a.Rest)
			}
		case !now && resting && a.Mode == animal.Wander:
			if a.Rest < animal.Full && (animal.SleepHours(animal.Deer, h) || a.Rest < animal.RestWakeDay) {
				t.Fatalf("tick %d: up at %d:00 with rest %d", i, h, a.Rest)
			}
		}
		resting = a.Mode == animal.Rest
	}
	if beds == 0 || beds > 3 {
		t.Fatalf("rest bouts in two days: %d", beds)
	}
}

func TestDecisionPacing(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	deer, _ := g.SpawnAnimal("deer", 10, 10)
	g.Step(1)
	decisions, last := 0, animalOf(g, deer).NextDecide
	for i := 0; i < 600; i++ {
		g.Step(1)
		if nd := animalOf(g, deer).NextDecide; nd != last {
			decisions++
			last = nd
			if (nd+uint64(deer))%animal.DecideEvery != 0 {
				t.Fatalf("decision at %d is off the id's cycle", nd)
			}
		}
	}
	if decisions > 600/animal.DecideEvery {
		t.Fatalf("%d decisions in 600 ticks, want at most %d", decisions, 600/animal.DecideEvery)
	}
	// hurt: it decides at once
	g.hurt(deer, 1, 0)
	if a := animalOf(g, deer); a.NextDecide != uint64(g.clock.Now()) {
		t.Fatalf("hurt triggers a decision: next %d now %d", a.NextDecide, g.clock.Now())
	}
	g.Step(1)
	if a := animalOf(g, deer); a.NextDecide <= uint64(g.clock.Now()) {
		t.Fatalf("the triggered decision ran: %+v", a)
	}
}

func TestTimbergnawEatsTrees(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	sowGrass(g, 0, 0, 29, 19) // grass is no food to a tree-eater
	tree, _ := g.spawnPlant(plant.Tree, 14, 10, plant.DefOf(plant.Tree).GrowTicks)
	gnaw, _ := g.SpawnAnimal("timbergnaw", 10, 10)
	setAnimal(g, gnaw, func(a *animal.State) { a.Food = 2000 })
	for i := 0; i < 2000 && animalOf(g, gnaw).Food < 3000; i++ {
		g.Step(1)
	}
	if _, ok := g.plants.Get(tree); ok {
		t.Fatal("the timbergnaw ate the tree")
	}
	if itemsOnGround(g, "wood") != 0 {
		t.Fatal("a gnawed tree leaves no wood")
	}
	if a := animalOf(g, gnaw); a.Food < 2000+animal.TreeFood-200 {
		t.Fatalf("a tree is a big meal: food %d", a.Food)
	}
	if grassCount(g) != 30*20 {
		t.Fatal("a tree-eater does not graze")
	}
}

func TestIndexMatchesBruteForce(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		g := New(Config{Seed: seed, Width: 70, Height: 50})
		for i := 0; i < 60; i++ {
			x, y := g.rng.IntN(70), g.rng.IntN(50)
			if i%5 == 0 {
				_, _ = g.SpawnPawn(x, y)
				continue
			}
			_, _ = g.SpawnAnimal(animal.All()[g.rng.IntN(40)].String(), x, y)
		}
		g.Step(3) // creatures move off their spawn tiles
		for q := 0; q < 200; q++ {
			x, y, r := g.rng.IntN(70), g.rng.IntN(50), g.rng.IntN(25)
			keep := func(id entity.EntityID, _ Position) bool { return id%3 != 0 }
			got, _, _ := g.nearestWhere(x, y, r, ckAll, keep)
			var want entity.EntityID
			bestD := r + 1
			var seen []entity.EntityID
			for _, list := range [][]entity.EntityID{g.pawnIDs, g.raiderIDs, g.animalIDs} {
				for _, id := range list {
					p, _ := g.positions.Get(id)
					d := chebyshev(Position{x, y}, p)
					if d <= r {
						seen = append(seen, id)
					}
					if d <= r && keep(id, p) && (d < bestD || d == bestD && id < want) {
						want, bestD = id, d
					}
				}
			}
			if got != want {
				t.Fatalf("seed %d: nearest to (%d,%d) r %d: index %d, brute force %d", seed, x, y, r, got, want)
			}
			var in []entity.EntityID
			g.within(x, y, r, ckAll, func(id entity.EntityID, _ Position) { in = append(in, id) })
			if len(in) != len(seen) {
				t.Fatalf("seed %d: within (%d,%d) r %d: %d, brute force %d", seed, x, y, r, len(in), len(seen))
			}
		}
	}
}

func TestOldSaveAnimalsNormalised(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	wolf, _ := g.SpawnAnimal("timber_wolf", 5, 5)
	deer, _ := g.SpawnAnimal("deer", 9, 5)
	old := func(id entity.EntityID, hunger uint64) {
		a := animalOf(g, id)
		g.animals.Set(id, animal.State{Species: a.Species, Hunger: hunger, NextThink: a.NextThink}) // a pre-needs record
	}
	old(wolf, animal.DefOf(animal.TimberWolf).HungerTicks)
	old(deer, 0)
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	d := animalOf(h, deer)
	if !d.HasSex || d.Female != animal.ArtFemale(uint64(deer)) || d.Food != animal.Full || d.Rest != animal.Full || d.Born != 0 {
		t.Fatalf("an old deer loads fed, rested, adult, with its art's sex: %+v", d)
	}
	w := animalOf(h, wolf)
	if w.Hunger != 0 || w.Food >= animal.FeedStart || w.Food < animal.FeedStart-10 || w.Rest != animal.Full {
		t.Fatalf("an old hungry wolf loads just hungry: %+v", w)
	}
	if v := animalView(t, h, deer); !v.Adult || v.Food != 100 || v.Stage != "adult" || v.Scale != 1 {
		t.Fatalf("view %+v", v)
	}
}

func TestWildlifeRows(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 20})
	for i := 0; i < 3; i++ {
		_, _ = g.SpawnAnimal("deer", 5+i, 5)
	}
	_, _ = g.SpawnAnimal("timber_wolf", 20, 10)
	pet, _ := g.SpawnAnimal("deer", 30, 10)
	g.TameAnimal(pet, "Clover")
	g.Apply(command.Command{Kind: command.DesignateHunt, X: 5, Y: 5, X2: 5, Y2: 5})
	g.Apply(command.Command{Kind: command.DesignateTame, X: 20, Y: 10, X2: 20, Y2: 10})
	g.Step(1)
	rows := g.Wildlife()
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	d, w := rows[0], rows[1]
	if d.Species != "deer" || d.Count != 3 || d.Hunted != 1 || d.Predator || d.Diet != "plants" ||
		d.Meat != animal.DefOf(animal.Deer).Meat || d.TameChance != animal.DefOf(animal.Deer).TameChance || d.Label != "deer" {
		t.Fatalf("deer row %+v", d)
	}
	if w.Species != "timber_wolf" || w.Count != 1 || !w.Predator || w.TameMarked != 1 || w.Diet != "meat" {
		t.Fatalf("wolf row %+v", w)
	}
}

func TestDomesticNotWildAtStart(t *testing.T) {
	for seed := uint64(1); seed <= 25; seed++ {
		g := New(Config{Seed: seed, Width: 60, Height: 60})
		g.spawnWildlife()
		for _, s := range speciesOf(g) {
			if animal.Weight(s, g.biome) == 0 {
				t.Fatalf("seed %d: %s is never wild here", seed, s)
			}
			switch s {
			case animal.Cow, animal.Goat, animal.Pig, animal.Sheep, animal.Chicken, animal.Cat, animal.Husky,
				animal.Labrador, animal.YorkshireTerrier, animal.GuineaPig:
				t.Fatalf("seed %d: domestic %s at the start", seed, s)
			}
		}
	}
}

func TestBiomeWildlifeAtStart(t *testing.T) {
	cold := map[animal.Species]bool{}
	for seed := uint64(1); seed <= 25; seed++ {
		g := NewColony(Config{Seed: seed, Width: 60, Height: 60, Biome: biome.Tundra})
		herds := map[uint32][]animal.State{}
		for _, id := range g.animalIDs {
			a := animalOf(g, id)
			if animal.Weight(a.Species, biome.Tundra) == 0 {
				t.Fatalf("seed %d: %s in the tundra", seed, a.Species)
			}
			cold[a.Species] = true
			if a.Herd == 0 || !a.HasSex || a.Food < animal.Full*60/100 || a.Rest < animal.Full*60/100 {
				t.Fatalf("seed %d: start animal %+v", seed, a)
			}
			herds[a.Herd] = append(herds[a.Herd], a)
		}
		now := uint64(g.clock.Now())
		for h, members := range herds {
			adult := false
			for _, a := range members {
				days := animal.AgeDays(a.Born, now, a.Species)
				life := animal.LifeOf(a.Species)
				if days < life.LifeDays*2/10 && days < life.AdultDays || days > life.LifeDays*8/10 {
					t.Fatalf("herd %d: age %d days of %d", h, days, life.LifeDays)
				}
				adult = adult || animal.Adult(a.Species, days)
			}
			if !adult {
				t.Fatalf("herd %d has no adult", h)
			}
		}
	}
	if cold[animal.Deer] || len(cold) < 3 {
		t.Fatalf("tundra species %v", cold)
	}
}

// The indexes are derived state: a game saved mid-day and loaded goes on
// exactly as the one that kept running.
func TestWildlifeSaveContinuesDeterministically(t *testing.T) {
	g := New(Config{Seed: 4, Width: 60, Height: 50})
	g.generate(4)
	g.seedGrass()
	g.wild = true
	g.spawnWildlife()
	for _, id := range g.animalIDs {
		setAnimal(g, id, func(a *animal.State) { a.Food = animal.FeedStart - 100 })
	}
	g.Step(3000)
	var buf bytes.Buffer
	if err := g.Save(&buf, "mid"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		g.Step(100)
		h.Step(100)
		if len(g.animalIDs) != len(h.animalIDs) {
			t.Fatalf("step %d: %d vs %d animals", i, len(g.animalIDs), len(h.animalIDs))
		}
		for _, id := range g.animalIDs {
			a, b := animalOf(g, id), animalOf(h, id)
			if a.Food != b.Food || a.Mode != b.Mode || a.Feed != b.Feed || pos(g, id) != pos(h, id) {
				t.Fatalf("step %d: animal %d diverged after load:\n%+v\n%+v", i, id, a, b)
			}
		}
	}
}
