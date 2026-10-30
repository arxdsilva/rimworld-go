package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/forage"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// PLAY-017 §8: a tame animal eats the nearest food its diet allows
// (grass, browse, loose or stockpiled raw food, carcasses), and a tame
// predator hunts the nearest wild prey; pens and areas still bound it.

// feedField is an empty map: no grass, no plants.
func feedField(w, h int) *Game {
	g := New(Config{Seed: 1, Width: w, Height: h})
	bareGround(g)
	for _, id := range slices.Clone(g.plantIDs) {
		g.removePlant(id)
	}
	return g
}

// hungryTame spawns a tame, hungry animal of species s that won't wander.
func hungryTame(t *testing.T, g *Game, s string, x, y int, name string) entity.EntityID {
	t.Helper()
	id, err := g.SpawnAnimal(s, x, y)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, name)
	setAnimal(g, id, func(a *animal.State) { a.Food, a.NextDecide, a.NextThink = 3000, 0, 1<<40 })
	return id
}

// stack spawns qty of k at (x, y).
func stack(t *testing.T, g *Game, k item.Kind, qty, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnStack(k, qty, x, y)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// firstFeed steps until the animal has picked a meal (or hunt) and returns it.
func firstFeed(g *Game, id entity.EntityID, ticks int) animal.State {
	for i := 0; i < ticks; i++ {
		g.Step(1)
		if a := animalOf(g, id); a.Mode == animal.Feed && a.Feed.Kind != animal.FeedNone || a.Mode == animal.Attack && a.HuntSince != 0 {
			return a
		}
	}
	return animalOf(g, id)
}

func TestTameDeerEatsTheNearBushNotFarGrass(t *testing.T) {
	g := feedField(40, 20)
	sowGrass(g, 30, 10, 30, 10) // 20 tiles away
	bush, _ := g.spawnPlant(plant.BerryBush, 12, 10, plant.DefOf(plant.BerryBush).GrowTicks)
	deer := hungryTame(t, g, "deer", 10, 10, "Clover")
	a := firstFeed(g, deer, 2*animal.DecideEvery)
	if a.Feed.Kind != animal.FeedPlant || a.Feed.ID != bush {
		t.Fatalf("a tame deer picks the bush 2 tiles away: %+v", a.Feed)
	}
	if v := viewOf(g, deer); v.State != "eating berries" {
		t.Fatalf("state %q", v.State)
	}
	for i := 0; i < 600 && animalOf(g, deer).Food < 3000+animal.BrowseFood/2; i++ {
		g.Step(1)
	}
	if p, ok := g.plants.Get(bush); !ok || p.Percent() != plant.DefOf(plant.BerryBush).RegrowTo {
		t.Fatalf("the bush is cropped back, not killed: %+v (%v)", p, ok)
	}
	// a wild deer keeps its old order: grass before browse
	w := feedField(40, 20)
	sowGrass(w, 30, 10, 30, 10)
	w.spawnPlant(plant.BerryBush, 12, 10, plant.DefOf(plant.BerryBush).GrowTicks)
	wild, _ := w.SpawnAnimal("deer", 10, 10)
	setAnimal(w, wild, func(a *animal.State) { a.Food, a.NextDecide = 3000, 0 })
	if a := firstFeed(w, wild, 2*animal.DecideEvery); a.Feed.Kind != animal.FeedGrass {
		t.Fatalf("a wild deer grazes first: %+v", a.Feed)
	}
}

func TestTameDogEatsStockpiledMeat(t *testing.T) {
	g := feedField(30, 20)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 15, Y: 9, X2: 17, Y2: 11})
	meat := stack(t, g, item.Meat, 30, 16, 10)
	// nearer, but passed over: medicine, milk, an egg and forbidden meat
	// are no animal food; the meal is (FEED-001: every diet but strict
	// carnivores eats meals), but animal feed comes first, so with meat in
	// reach the meal is left alone (it used to be left because meals fed
	// no animal at all)
	for i, k := range []item.Kind{item.Meal, item.HerbalMedicine, item.Milk, item.Egg} {
		stack(t, g, k, 5, 12, 7+i)
	}
	fb := stack(t, g, item.Meat, 5, 11, 12)
	st, _ := g.stacks.Get(fb)
	st.Forbidden = true
	g.stacks.Set(fb, st)
	dog := hungryTame(t, g, "husky", 10, 10, "Rex")
	a := firstFeed(g, dog, 2*animal.DecideEvery)
	if a.Feed.Kind != animal.FeedItem || a.Feed.ID != meat {
		t.Fatalf("a tame dog goes for the stockpiled meat: %+v", a.Feed)
	}
	if o, ok := g.reserve.EntityOwner(meat); !ok || o != dog {
		t.Fatalf("the meat is reserved for the dog while it walks: %d %v", o, ok)
	}
	if v := viewOf(g, dog); v.State != "eating from the stockpile" {
		t.Fatalf("state %q", v.State)
	}
	for i := 0; i < 600 && animalOf(g, dog).Mode == animal.Feed; i++ {
		g.Step(1)
	}
	got := animalOf(g, dog)
	if got.Food < animal.FeedStop {
		t.Fatalf("it eats until full: %+v", got)
	}
	left, _ := g.stacks.Get(meat)
	per := animal.Meal(animal.Husky, animal.FeedValue(animal.Carnivore, item.Meat))
	if eaten := 30 - left.Qty; eaten <= 0 || eaten > (animal.Full-3000)/per+2 {
		t.Fatalf("it takes only what fills it: ate %d (each %d)", eaten, per)
	}
	if _, ok := g.reserve.EntityOwner(meat); ok {
		t.Fatal("the reservation ends with the meal")
	}
	if fs, _ := g.stacks.Get(fb); fs.Qty != 5 {
		t.Fatal("forbidden meat is left alone")
	}
	for _, k := range []string{"meal", "herbal medicine", "milk", "egg"} {
		if itemsOnGround(g, k) != 5 {
			t.Fatalf("%s eaten", k)
		}
	}
}

// A stack another owner holds is skipped, and one that becomes forbidden
// on the way is given up (re-checked every step).
func TestTameFeedRechecksTheStack(t *testing.T) {
	g := feedField(30, 20)
	near := stack(t, g, item.Meat, 10, 13, 10)
	far := stack(t, g, item.Meat, 10, 20, 10)
	g.reserve.TryEntity(9999, near) // a colonist's haul
	dog := hungryTame(t, g, "husky", 10, 10, "Rex")
	if a := firstFeed(g, dog, 2*animal.DecideEvery); a.Feed.ID != far {
		t.Fatalf("a reserved stack is skipped: %+v", a.Feed)
	}
	st, _ := g.stacks.Get(far)
	st.Forbidden = true
	g.stacks.Set(far, st)
	g.Step(2)
	if a := animalOf(g, dog); a.Feed.ID == far {
		t.Fatalf("a stack forbidden on the way is given up: %+v", a.Feed)
	}
	if _, ok := g.reserve.EntityOwner(far); ok {
		t.Fatal("and its reservation released")
	}
}

// wallRing walls in the rectangle's border.
func wallRing(t *testing.T, g *Game, x0, y0, x1, y1 int) {
	t.Helper()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if (x == x0 || x == x1 || y == y0 || y == y1) && !g.BuildNow("wall", "wood", "south", x, y) {
				t.Fatalf("wall at %d,%d not built", x, y)
			}
		}
	}
}

// A tame wolf in a walled yard with a wild hare (which can't outrun it
// there), colony animals and a colonist, all closer than the hare: it
// hunts only the hare.
func TestTameWolfHuntsAWildHare(t *testing.T) {
	g := feedField(40, 24)
	wallRing(t, g, 8, 7, 17, 13)
	rex := hungryTame(t, g, "timber_wolf", 10, 10, "Rex")
	ada, _ := g.SpawnNamedPawn(11, 11, "Ada")
	sheep := hungryTame(t, g, "sheep", 11, 9, "Wool")
	deer := hungryTame(t, g, "deer", 9, 11, "Clover")
	hare, _ := g.SpawnAnimal("hare", 14, 10)
	for _, id := range []entity.EntityID{sheep, deer} {
		setAnimal(g, id, func(a *animal.State) { a.Food = animal.Full })
	}
	setAnimal(g, hare, func(a *animal.State) { a.Injury = animal.DefOf(animal.Hare).MaxHP - 1 })
	setAnimal(g, rex, func(a *animal.State) { a.Food = 2000 })
	never := []entity.EntityID{ada, sheep, deer}
	hunted := false
	for i := 0; i < 3000 && animalAlive(g, hare); i++ {
		setAnimal(g, sheep, func(a *animal.State) { a.Food = animal.Full })
		setAnimal(g, deer, func(a *animal.State) { a.Food = animal.Full })
		g.Step(1)
		a := animalOf(g, rex)
		if slices.Contains(never, a.Target) {
			t.Fatalf("tick %d: the wolf went for %d: %+v", i, a.Target, a)
		}
		if a.Target == hare && a.HuntSince != 0 {
			hunted = true
			if v := viewOf(g, rex); v.State != "hunting a hare" {
				t.Fatalf("state %q", v.State)
			}
		}
	}
	if !hunted || animalAlive(g, hare) {
		t.Fatalf("the wolf hunts the wild hare: hunted %v, hare alive %v", hunted, animalAlive(g, hare))
	}
	if a := animalOf(g, rex); a.Mode != animal.Feed || a.Feed.Kind != animal.FeedCarcass {
		t.Fatalf("it eats its kill: %+v", a)
	}
	carcass, _ := g.stacks.Get(animalOf(g, rex).Feed.ID)
	if carcass.Unclaimed {
		t.Fatal("a tame animal's kill is the colony's")
	}
	if !hasMessage(g, "Rex hunted a hare.") {
		t.Fatalf("messages %+v", g.messages)
	}
	for _, m := range g.messages {
		if m.Danger {
			t.Fatalf("a hunt is no danger: %+v", m)
		}
	}
}

func TestTameWolfLeavesColonyAnimalsAndPeopleAlone(t *testing.T) {
	g := feedField(40, 24)
	ada, _ := g.SpawnNamedPawn(12, 12, "Ada")
	rex := hungryTame(t, g, "timber_wolf", 10, 10, "Rex")
	sheep := hungryTame(t, g, "sheep", 11, 10, "Wool")
	hare := hungryTame(t, g, "hare", 9, 10, "Hop") // tame
	marked, _ := g.SpawnAnimal("hare", 14, 10)
	setAnimal(g, marked, func(a *animal.State) { a.TameMark, a.NextThink = true, 1<<40 })
	setAnimal(g, rex, func(a *animal.State) { a.Food = 2000 })
	for i := 0; i < 3*animal.HuntRetry; i++ {
		for _, id := range []entity.EntityID{sheep, hare, marked} {
			setAnimal(g, id, func(a *animal.State) { a.Food = animal.Full })
		}
		g.Step(1)
		if a := animalOf(g, rex); a.Target != 0 || a.Mode == animal.Attack {
			t.Fatalf("tick %d: no prey here (colonist %d): %+v", i, ada, a)
		}
	}
	// a pet (not a predator) never hunts
	h := feedField(40, 24)
	dog := hungryTame(t, h, "husky", 10, 10, "Bo")
	h.SpawnAnimal("hare", 12, 10)
	for i := 0; i < 3*animal.DecideEvery; i++ {
		h.Step(1)
		if a := animalOf(h, dog); a.Target != 0 || a.HuntSince != 0 {
			t.Fatalf("a pet hunted: %+v", a)
		}
	}
}

func TestTameFeedStaysInItsArea(t *testing.T) {
	g := feedField(40, 20)
	paintArea(g, 1, 0, 0, 7, 19)
	sowGrass(g, 1, 10, 1, 10) // inside, 5 away
	g.spawnPlant(plant.BerryBush, 9, 10, plant.DefOf(plant.BerryBush).GrowTicks)
	stack(t, g, item.Berries, 10, 8, 11) // outside, 2 away
	dog := hungryTame(t, g, "labrador", 6, 10, "Bo")
	assign(g, dog, 1)
	if a := firstFeed(g, dog, 2*animal.DecideEvery); a.Feed.Kind != animal.FeedGrass || a.Feed.X != 1 {
		t.Fatalf("an area animal eats inside its area: %+v", a.Feed)
	}
	// a tame wolf hunts only inside its area
	h := feedField(40, 20)
	paintArea(h, 1, 0, 0, 7, 19)
	rex := hungryTame(t, h, "timber_wolf", 6, 10, "Rex")
	assign(h, rex, 1)
	setAnimal(h, rex, func(a *animal.State) { a.Food = 2000 })
	out, _ := h.SpawnAnimal("hare", 10, 10)
	setAnimal(h, out, func(a *animal.State) { a.NextThink = 1 << 40 })
	for i := 0; i < 3*animal.DecideEvery; i++ {
		h.Step(1)
		if a := animalOf(h, rex); a.Target == out {
			t.Fatalf("hunted outside its area: %+v", a)
		}
	}
}

func TestPennedAnimalEatsInsideItsPen(t *testing.T) {
	g := feedField(40, 24)
	fenceRing(t, g, 14, 4, 23, 13, [2]int{18, 13})
	buildMarker(t, g, 16, 10)
	sowGrass(g, 21, 6, 21, 6)            // inside the pen
	stack(t, g, item.Berries, 10, 12, 8) // outside, nearer
	g.spawnPlant(plant.BerryBush, 13, 9, plant.DefOf(plant.BerryBush).GrowTicks)
	cow := hungryTame(t, g, "cow", 15, 8, "Daisy")
	if a := firstFeed(g, cow, 2*animal.DecideEvery); a.Feed.Kind != animal.FeedGrass || a.Feed.X != 21 {
		t.Fatalf("a penned cow eats in its pen: %+v", a.Feed)
	}
}

func TestTameAnimalsSpareCropsAndMarkedBushes(t *testing.T) {
	g := feedField(40, 20)
	crop, _ := g.spawnPlant(plant.Potato, 11, 10, plant.DefOf(plant.Potato).GrowTicks)
	p, _ := g.plants.Get(crop)
	p.Zone = true
	g.plants.Set(crop, p)
	g.notePlantMark(crop)
	marked, _ := g.spawnPlant(plant.BerryBush, 9, 10, plant.DefOf(plant.BerryBush).GrowTicks)
	m, _ := g.plants.Get(marked)
	m.Designated = true
	g.plants.Set(marked, m)
	g.notePlantMark(marked)
	// a wild bush standing in a growing zone
	g.Apply(command.Command{Kind: command.DesignateGrowZone, X: 10, Y: 12, X2: 10, Y2: 12})
	zoned, _ := g.spawnPlant(plant.BerryBush, 10, 12, plant.DefOf(plant.BerryBush).GrowTicks)
	sowGrass(g, 25, 10, 25, 10)
	deer := hungryTame(t, g, "deer", 10, 10, "Clover")
	if a := firstFeed(g, deer, 2*animal.DecideEvery); a.Feed.Kind != animal.FeedGrass {
		t.Fatalf("a tame deer passes crops and marked bushes: %+v", a.Feed)
	}
	for _, id := range []entity.EntityID{crop, marked, zoned} {
		if p, ok := g.plants.Get(id); !ok || p.Percent() != 100 {
			t.Fatalf("plant %d eaten: %+v", id, p)
		}
	}
}

// No-flap (1): two equal stacks at equal distance. The dog takes one
// and eats from it without ever switching; a second dog takes the other.
func TestTameFeedNoFlapEqualStacks(t *testing.T) {
	g := feedField(30, 20)
	left := stack(t, g, item.Meat, 20, 6, 10)
	right := stack(t, g, item.Meat, 20, 14, 10)
	dog := hungryTame(t, g, "husky", 10, 10, "Rex")
	pup := hungryTame(t, g, "husky", 10, 11, "Bo")
	var dogT, pupT []entity.EntityID
	note := func(seq *[]entity.EntityID, a animal.State) {
		if a.Mode == animal.Feed && a.Feed.ID != 0 && (len(*seq) == 0 || (*seq)[len(*seq)-1] != a.Feed.ID) {
			*seq = append(*seq, a.Feed.ID)
		}
	}
	for i := 0; i < 1500; i++ {
		g.Step(1)
		note(&dogT, animalOf(g, dog))
		note(&pupT, animalOf(g, pup))
	}
	if len(dogT) != 1 || len(pupT) != 1 || dogT[0] == pupT[0] {
		t.Fatalf("each dog keeps one stack: %v %v (left %d right %d)", dogT, pupT, left, right)
	}
	if dogT[0] != left {
		t.Fatalf("ties go to the lower id: %v", dogT)
	}
	for _, id := range []entity.EntityID{dog, pup} {
		if a := animalOf(g, id); a.Food < animal.FeedStop {
			t.Fatalf("fed: %+v", a)
		}
	}
}

// No-flap (2): a wild hare and stockpiled meat at equal distance. The
// wolf commits to one of them and never swaps back and forth.
func TestTameFeedNoFlapPreyVersusMeat(t *testing.T) {
	g := feedField(40, 20)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 23, Y: 10, X2: 23, Y2: 10})
	meat := stack(t, g, item.Meat, 40, 23, 10)
	hare, _ := g.SpawnAnimal("hare", 17, 10)
	rex := hungryTame(t, g, "timber_wolf", 20, 10, "Rex")
	setAnimal(g, rex, func(a *animal.State) { a.Food = 2000 })
	var seq []string
	for i := 0; i < 3000 && animalOf(g, rex).Food < animal.FeedStop; i++ {
		g.Step(1)
		a := animalOf(g, rex)
		cur := ""
		switch {
		case a.Mode == animal.Feed && a.Feed.ID == meat:
			cur = "meat"
		case a.Mode == animal.Attack && a.Target == hare:
			cur = "hunt"
		}
		if cur != "" && (len(seq) == 0 || seq[len(seq)-1] != cur) {
			seq = append(seq, cur)
		}
	}
	if len(seq) == 0 {
		t.Fatal("the wolf ate or hunted")
	}
	if len(seq) > 2 {
		t.Fatalf("flapped between prey and meat: %v", seq)
	}
	if seq[0] != "meat" {
		t.Fatalf("at equal distance and enough food either way, the stockpile wins over a hunt: %v", seq)
	}
}

func TestTamePredatorDietLine(t *testing.T) {
	g := feedField(20, 12)
	rex := hungryTame(t, g, "timber_wolf", 5, 5, "Rex")
	dog := hungryTame(t, g, "husky", 9, 5, "Bo")
	wild, _ := g.SpawnAnimal("timber_wolf", 14, 5)
	if !viewOf(g, rex).Hunts || viewOf(g, dog).Hunts || viewOf(g, wild).Hunts {
		t.Fatalf("only tame predators carry the hunt note: %v %v %v", viewOf(g, rex).Hunts, viewOf(g, dog).Hunts, viewOf(g, wild).Hunts)
	}
}

// --- FEED-001: tame animals reach the colony's food -----------------------

// feedUntil steps until id's food reaches at least want (or ticks pass)
// and returns the feed targets it walked to, in order, without repeats.
func feedUntil(g *Game, id entity.EntityID, want, ticks int) []animal.FeedTarget {
	var seq []animal.FeedTarget
	for i := 0; i < ticks && animalOf(g, id).Food < want; i++ {
		g.Step(1)
		if a := animalOf(g, id); a.Mode == animal.Feed && a.Feed.Kind != animal.FeedNone &&
			(len(seq) == 0 || seq[len(seq)-1] != a.Feed) {
			seq = append(seq, a.Feed)
		}
	}
	return seq
}

func TestTameDeerEatsStockpiledKibble(t *testing.T) {
	g := feedField(30, 20)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 15, Y: 9, X2: 17, Y2: 11})
	kibble := stack(t, g, item.Kibble, 50, 16, 10)
	deer := hungryTame(t, g, "deer", 10, 10, "Clover")
	if a := firstFeed(g, deer, 2*animal.DecideEvery); a.Feed.Kind != animal.FeedItem || a.Feed.ID != kibble {
		t.Fatalf("a hungry tame deer goes for the stockpiled kibble: %+v", a.Feed)
	}
	if v := viewOf(g, deer); v.State != "eating kibble" {
		t.Fatalf("state %q", v.State)
	}
	feedUntil(g, deer, animal.FeedStop, 900)
	if a := animalOf(g, deer); a.Food < animal.FeedStop {
		t.Fatalf("kibble fills it: %+v", a)
	}
	if st, _ := g.stacks.Get(kibble); st.Qty >= 50 {
		t.Fatalf("kibble eaten: %+v", st)
	}
}

// Feed first (user decision 2026-10-30): kibble 10 tiles off beats a meal
// beside the deer; with only the meal in reach it eats the meal.
func TestTameFeedFirstThenMeal(t *testing.T) {
	g := feedField(30, 20)
	meal := stack(t, g, item.Meal, 3, 11, 10)
	kibble := stack(t, g, item.Kibble, 50, 20, 10)
	deer := hungryTame(t, g, "deer", 10, 10, "Clover")
	if a := firstFeed(g, deer, 2*animal.DecideEvery); a.Feed.ID != kibble {
		t.Fatalf("feed before a meal, even a nearer one: %+v (meal %d)", a.Feed, meal)
	}
	h := feedField(30, 20)
	only := stack(t, h, item.FineMeal, 3, 14, 10)
	doe := hungryTame(t, h, "deer", 10, 10, "Fern")
	if a := firstFeed(h, doe, 2*animal.DecideEvery); a.Feed.ID != only {
		t.Fatalf("no feed in reach: the meal: %+v", a.Feed)
	}
	if v := viewOf(h, doe); v.State != "eating a meal (no feed in reach)" {
		t.Fatalf("state %q", v.State)
	}
	feedUntil(h, doe, animal.FeedStop, 600)
	if a := animalOf(h, doe); a.Food < animal.FeedStop {
		t.Fatalf("one meal fills it: %+v", a)
	}
}

// A strict carnivore (the dire wolf) passes kibble and meals for meat,
// and with only kibble and meals in reach eats nothing.
func TestStrictCarnivoreLeavesKibbleAlone(t *testing.T) {
	g := feedField(30, 20)
	stack(t, g, item.Kibble, 50, 11, 10)
	stack(t, g, item.Meal, 3, 10, 11)
	meat := stack(t, g, item.Meat, 20, 18, 10)
	wolf := hungryTame(t, g, "dire_wolf", 10, 10, "Fang")
	if a := firstFeed(g, wolf, 2*animal.DecideEvery); a.Feed.ID != meat {
		t.Fatalf("a strict carnivore goes for the meat: %+v", a.Feed)
	}
	h := feedField(30, 20)
	stack(t, h, item.Kibble, 50, 11, 10)
	stack(t, h, item.Meal, 3, 10, 11)
	fang := hungryTame(t, h, "dire_wolf", 10, 10, "Fang")
	for i := 0; i < 3*animal.DecideEvery; i++ {
		h.Step(1)
		if a := animalOf(h, fang); a.Mode == animal.Feed && a.Feed.Kind != animal.FeedNone {
			t.Fatalf("a strict carnivore took %+v", a.Feed)
		}
	}
	if itemsOnGround(h, "kibble") != 50 || itemsOnGround(h, "meal") != 3 {
		t.Fatal("kibble and meals left alone")
	}
	if !viewOf(h, fang).Hunts || viewOf(h, fang).Diet != "meat only" {
		t.Fatalf("view %+v", viewOf(h, fang))
	}
}

// doorRoom walls in the rectangle with a door on (dx, dy) and returns the
// door's id.
func doorRoom(t *testing.T, g *Game, x0, y0, x1, y1, dx, dy int) entity.EntityID {
	t.Helper()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x != x0 && x != x1 && y != y0 && y != y1 {
				continue
			}
			kind := "wall"
			if x == dx && y == dy {
				kind = "door"
			}
			if !g.BuildNow(kind, "wood", "south", x, y) {
				t.Fatalf("%s at %d,%d not built", kind, x, y)
			}
		}
	}
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.Door {
			return id
		}
	}
	t.Fatal("no door")
	return 0
}

// The reported bug (FEED-001): the nearest stockpile sits behind a door a
// loose tame deer (livestock, unled: doors are shut to it) can't pass,
// and grass grows farther off. It eats the grass instead of walking at
// the door forever; once the door is gone it eats from the stockpile,
// and it never swaps back and forth.
func TestTameDeerSkipsTheStockpileBehindAShutDoor(t *testing.T) {
	g := feedField(50, 20)
	door := doorRoom(t, g, 20, 7, 26, 13, 20, 10)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 22, Y: 9, X2: 24, Y2: 11})
	corn := stack(t, g, item.Corn, 30, 22, 10)
	kibble := stack(t, g, item.Kibble, 50, 22, 11)
	sowGrass(g, 1, 10, 1, 10) // 15 tiles off, outside
	deer := hungryTame(t, g, "deer", 16, 10, "Clover")
	seq := feedUntil(g, deer, 3000+animal.GrazeFood/2, 1500)
	if a := animalOf(g, deer); a.Food < 3000 {
		t.Fatalf("the deer eats within 1500 ticks: %+v, targets %+v", a, seq)
	}
	for _, f := range seq {
		if f.ID == corn || f.ID == kibble {
			t.Fatalf("it went for the stockpile behind the shut door: %+v", seq)
		}
	}
	if len(seq) == 0 || seq[0].Kind != animal.FeedGrass {
		t.Fatalf("it grazes the far grass: %+v", seq)
	}
	// the door taken out: the stockpile is in reach and nearer
	g.removeBuilding(door)
	setAnimal(g, deer, func(a *animal.State) {
		a.Mode, a.Feed, a.FeedLeft, a.Food, a.NextFeed, a.NextDecide = animal.Wander, animal.FeedTarget{}, 0, 3000, 0, 0
	})
	seq = feedUntil(g, deer, animal.FeedStop, 1500)
	if len(seq) != 1 || seq[0].ID != corn && seq[0].ID != kibble {
		t.Fatalf("with the door gone it eats from the stockpile, one target: %+v", seq)
	}
	if a := animalOf(g, deer); a.Food < animal.FeedStop {
		t.Fatalf("fed: %+v", a)
	}
}

// A spot a tame animal gave up on (it could not get there in
// feedWalkLimit) is skipped for forage.ForgetTicks, then tried again.
func TestTameFeedForgetsAGivenUpSpot(t *testing.T) {
	g := feedField(40, 20)
	kibble := stack(t, g, item.Kibble, 50, 14, 10)
	sowGrass(g, 30, 10, 30, 10)
	deer := hungryTame(t, g, "deer", 10, 10, "Clover")
	setAnimal(g, deer, func(a *animal.State) {
		a.Mode, a.Feed, a.FeedLeft = animal.Feed, animal.FeedTarget{Kind: animal.FeedItem, ID: kibble, X: 14, Y: 10}, animal.ItemTicks
		a.NextFeed, a.NextDecide, a.RetryAt = 0, 1<<40, 1<<40 // no walk yet, and the walk limit passed
	})
	g.Step(1)
	now := uint64(g.clock.Now())
	if a := animalOf(g, deer); a.Mode != animal.Wander || !forage.Skips(g.wl.Forage.Marks(deer), 14, 10, now) {
		t.Fatalf("it gives the stack up and remembers it: %+v %+v", a, g.wl.Forage.Marks(deer))
	}
	hungry := func() {
		setAnimal(g, deer, func(a *animal.State) { a.Food, a.NextFeed, a.NextDecide, a.RetryAt = 3000, 0, 0, 0 })
	}
	hungry()
	if a := firstFeed(g, deer, 2*animal.DecideEvery); a.Feed.Kind != animal.FeedGrass {
		t.Fatalf("it skips the remembered stack for the far grass: %+v", a.Feed)
	}
	setAnimal(g, deer, func(a *animal.State) {
		a.Mode, a.Feed, a.FeedLeft, a.Food, a.Path, a.PathI = animal.Wander, animal.FeedTarget{}, 0, animal.Full, nil, 0
	})
	g.Step(forage.ForgetTicks)
	if len(g.wl.Forage.Marks(deer)) != 0 {
		t.Fatalf("the mark expires and is pruned: %+v", g.wl.Forage.Marks(deer))
	}
	hungry()
	if a := firstFeed(g, deer, 2*animal.DecideEvery); a.Feed.ID != kibble {
		t.Fatalf("after ForgetTicks the stack is tried again: %+v", a.Feed)
	}
}

// Starving fallback: below animal.StarveSearch a tame animal looks past
// FeedRadius; at 30 % it doesn't. No flap: on its long walk it keeps one
// target and searches far until it has eaten.
func TestStarvingTameSearchesFar(t *testing.T) {
	far := func(food int) (*Game, entity.EntityID, entity.EntityID) {
		g := feedField(80, 20)
		k := stack(t, g, item.Kibble, 60, 60, 10) // 55 tiles off
		deer := hungryTame(t, g, "deer", 5, 10, "Clover")
		setAnimal(g, deer, func(a *animal.State) { a.Food = food })
		return g, deer, k
	}
	g, deer, kibble := far(animal.Full * 30 / 100)
	for i := 0; i < 3*animal.DecideEvery; i++ {
		g.Step(1)
		if a := animalOf(g, deer); a.Feed.ID == kibble {
			t.Fatalf("at 30 %% it stays within FeedRadius: %+v", a.Feed)
		}
	}
	g, deer, kibble = far(animal.Full * 10 / 100)
	if a := firstFeed(g, deer, 2*animal.DecideEvery); a.Feed.ID != kibble {
		t.Fatalf("at 10 %% it searches the whole map: %+v", a.Feed)
	}
	if v := viewOf(g, deer); v.State != "searching far for food" {
		t.Fatalf("state %q", v.State)
	}
	var flips []bool // the far flag, each time it changes
	var seq []entity.EntityID
	for i := 0; i < 4000 && animalOf(g, deer).Food < animal.FeedStop; i++ {
		g.Step(1)
		if f := g.wl.Forage.IsFar(deer); len(flips) == 0 || flips[len(flips)-1] != f {
			flips = append(flips, f)
		}
		if a := animalOf(g, deer); a.Mode == animal.Feed && a.Feed.Kind != animal.FeedNone && (len(seq) == 0 || seq[len(seq)-1] != a.Feed.ID) {
			seq = append(seq, a.Feed.ID)
		}
	}
	if len(seq) != 1 || seq[0] != kibble {
		t.Fatalf("one target the whole walk: %v", seq)
	}
	if a := animalOf(g, deer); a.Food < animal.FeedStop || !slices.Equal(flips, []bool{true, false}) {
		t.Fatalf("fed, far until it ate and never again: %+v far %v", a, flips)
	}
}

// The far search keeps to the allowed area and the pen.
func TestStarvingSearchStaysInAreaAndPen(t *testing.T) {
	g := feedField(80, 20)
	paintArea(g, 1, 0, 0, 20, 19)
	stack(t, g, item.Kibble, 60, 60, 10)
	dog := hungryTame(t, g, "labrador", 5, 10, "Bo")
	assign(g, dog, 1)
	setAnimal(g, dog, func(a *animal.State) { a.Food = 1000 })
	h := feedField(80, 24)
	fenceRing(t, h, 14, 4, 23, 13, [2]int{18, 13})
	buildMarker(t, h, 16, 10)
	stack(t, h, item.Kibble, 60, 60, 10)
	cow := hungryTame(t, h, "cow", 15, 8, "Daisy")
	setAnimal(h, cow, func(a *animal.State) { a.Food = 1000 })
	for i := 0; i < 3*animal.DecideEvery; i++ {
		g.Step(1)
		h.Step(1)
		if a := animalOf(g, dog); a.Feed.Kind != animal.FeedNone {
			t.Fatalf("the area dog left its area: %+v", a.Feed)
		}
		if a := animalOf(h, cow); a.Feed.Kind != animal.FeedNone {
			t.Fatalf("the penned cow left its pen: %+v", a.Feed)
		}
	}
	if v := viewOf(g, dog); v.State != "searching far for food" {
		t.Fatalf("a starving animal with nothing in reach says so: %q", v.State)
	}
}

// A trough filled with kibble feeds the pen (kibble passes every filter).
func TestPenTroughFeedsKibble(t *testing.T) {
	for _, f := range []uint8{TroughPlants, TroughMeat, TroughAny} {
		if !troughFeeds(f, item.Kibble) {
			t.Fatalf("filter %d refuses kibble", f)
		}
	}
	g := feedField(40, 24)
	fenceRing(t, g, 14, 4, 23, 13, [2]int{18, 13})
	buildMarker(t, g, 16, 10)
	placeBuilt(t, g, construction.FeedingTrough, construction.South, 18, 8)
	var bid entity.EntityID
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.FeedingTrough {
			bid = id
		}
	}
	cow := hungryTame(t, g, "cow", 16, 6, "Daisy")
	tr := g.trough(bid)
	tr.Food = map[item.Kind]int{item.Kibble: 40}
	g.troughs[bid] = tr
	fromTrough := false
	for i := 0; i < 900 && animalOf(g, cow).Food < animal.FeedStart; i++ {
		g.Step(1)
		fromTrough = fromTrough || animalOf(g, cow).Feed.Kind == animal.FeedTrough
	}
	if !fromTrough || animalOf(g, cow).Food < animal.FeedStart || g.troughFill(bid) >= 40 {
		t.Fatalf("the penned cow eats kibble from the trough: %+v, fill %d", animalOf(g, cow), g.troughFill(bid))
	}
}
