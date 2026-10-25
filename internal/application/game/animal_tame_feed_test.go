package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/animal"
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
	// nearer, but not for animals: a meal, medicine, milk, an egg and
	// forbidden meat
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
