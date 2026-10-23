package game

import (
	"bytes"
	"sort"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
)

// Animal life (SIM-026, plan 4.1-4.3).

// advance moves the clock straight to tick (no-op when it is past).
func advance(g *Game, tick uint64) {
	if now := uint64(g.clock.Now()); tick > now {
		g.clock.Advance(int(tick - now))
	}
}

// keepFed tops an animal's food and rest up and keeps it wandering.
func keepFed(g *Game, id entity.EntityID) {
	setAnimal(g, id, func(a *animal.State) {
		a.Food, a.Rest = animal.Full, animal.Full
		if a.Mode == animal.Rest || a.Mode == animal.Feed {
			a.Mode = animal.Wander
		}
	})
}

// spawnSexed places an animal of s of the given sex and tameness.
func spawnSexed(t *testing.T, g *Game, s string, x, y int, female, tame bool, name string) entity.EntityID {
	t.Helper()
	id, err := g.SpawnAnimal(s, x, y)
	if err != nil {
		t.Fatal(err)
	}
	setAnimal(g, id, func(a *animal.State) { a.Female = female })
	if tame {
		g.TameAnimal(id, name)
	}
	return id
}

func animalCount(g *Game, s animal.Species) int {
	n := 0
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && a.Species == s {
			n++
		}
	}
	return n
}

func viewOf(g *Game, id entity.EntityID) AnimalView {
	for _, v := range g.World().Animals {
		if v.ID == id {
			return v
		}
	}
	return AnimalView{}
}

func hasLogKind(g *Game, id entity.EntityID, kind uint8) bool {
	for _, e := range g.logs[id] {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// mateUntilPregnant runs daily mating checks of male m until female f
// conceives (both kept fed); it fails after 60 days.
func mateUntilPregnant(t *testing.T, g *Game, s animal.Species, m, f entity.EntityID) {
	t.Helper()
	for day := uint64(1); day < 60; day++ {
		advance(g, day*clock.TicksPerDay+animal.MateTick(s, uint64(m)))
		keepFed(g, m)
		keepFed(g, f)
		for i := 0; i < animal.MateWalkLimit+animal.MateTicks+20; i += 50 {
			g.Step(50)
			keepFed(g, m)
			keepFed(g, f)
			if a := animalOf(g, f); a.Pregnant != 0 || a.Mated > 0 {
				return
			}
		}
	}
	t.Fatalf("no mating in 60 days: %+v / %+v", animalOf(g, m), animalOf(g, f))
}

func TestMatingPregnancyBirth(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	billy := spawnSexed(t, g, "goat", 13, 10, false, true, "Billy")
	daisy := spawnSexed(t, g, "goat", 16, 10, true, true, "Daisy")
	mateUntilPregnant(t, g, animal.Goat, billy, daisy)
	now := uint64(g.clock.Now())
	d := animalOf(g, daisy)
	if gest := animal.GestTicks(animal.Goat); d.Pregnant <= now || d.Pregnant > now+gest {
		t.Fatalf("due %d, now %d: want within a gestation (%d)", d.Pregnant, now, gest)
	}
	if !hasLogKind(g, billy, logMated) || !hasLogKind(g, daisy, logMated) {
		t.Fatal("both log the mating")
	}
	if b := animalOf(g, billy); b.Mate != 0 || d.Mate != 0 {
		t.Fatal("the mating is over")
	}
	if v := viewOf(g, daisy); !v.Pregnant || v.PregPct < 0 || v.PregPct > 5 {
		t.Fatalf("view: pregnant %v at %d%%", v.Pregnant, v.PregPct)
	}
	advance(g, d.Pregnant-clock.TicksPerDay)
	keepFed(g, daisy)
	g.Step(1)
	if v := viewOf(g, daisy); v.PregPct < 80 || v.PregPct >= 100 {
		t.Fatalf("a day before the birth: %d%%", v.PregPct)
	}
	advance(g, d.Pregnant)
	keepFed(g, daisy)
	// where she stood when she gave birth: she may walk on through her
	// young in the same tick (tame animals pass through each other, PLAY-010)
	var birthAt Position
	for range 2 {
		at := pos(g, daisy)
		g.Step(1)
		if birthAt == (Position{}) && animalOf(g, daisy).Pregnant == 0 {
			birthAt = at
		}
	}
	if animalOf(g, daisy).Pregnant != 0 {
		t.Fatal("she has given birth")
	}
	n := animalCount(g, animal.Goat) - 2
	life := animal.LifeOf(animal.Goat)
	if n < life.LitterMin || n > life.LitterMax {
		t.Fatalf("litter of %d, want %d..%d", n, life.LitterMin, life.LitterMax)
	}
	for _, id := range g.animalIDs {
		if id == billy || id == daisy {
			continue
		}
		v := viewOf(g, id)
		if v.Adult || v.Stage != "young" || v.Scale > 0.56 {
			t.Fatalf("newborn view %+v", v)
		}
		if p := pos(g, id); chebyshev(p, birthAt) > 1 {
			t.Fatalf("born at %v, beside the mother at %v", p, birthAt)
		}
	}
	if !hasMessage(g, "Daisy gave birth to") || !hasMessage(g, "goat kid") {
		t.Fatalf("birth message: %+v", g.messages)
	}
	if !hasLogKind(g, daisy, logGaveBirth) {
		t.Fatal("the mother logs the birth")
	}

	// a pregnancy dies with its mother
	ewe := spawnSexed(t, g, "goat", 5, 5, true, false, "")
	setAnimal(g, ewe, func(a *animal.State) { a.Pregnant = uint64(g.clock.Now()) + 100 })
	g.killAnimal(ewe, 0)
	before := animalCount(g, animal.Goat)
	g.Step(200)
	if animalCount(g, animal.Goat) != before {
		t.Fatal("a dead mother's litter is never born")
	}
}

func TestLitterInheritsTameness(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	now := uint64(g.clock.Now())
	doe := spawnSexed(t, g, "deer", 8, 8, true, false, "")
	cow := spawnSexed(t, g, "cow", 20, 8, true, true, "Bess")
	setAnimal(g, doe, func(a *animal.State) { a.Pregnant, a.Herd = now+10, 7 })
	setAnimal(g, cow, func(a *animal.State) { a.Pregnant, a.Herd = now+10, 9 })
	g.Step(20)
	var wild, tame int
	for _, id := range g.animalIDs {
		a := animalOf(g, id)
		if id == doe || id == cow || a.Born == 0 {
			continue
		}
		switch a.Species {
		case animal.Deer:
			wild++
			if a.Tame || a.Name != "" || a.Herd != 7 {
				t.Fatalf("a wild doe's fawn is wild, in her herd: %+v", a)
			}
		case animal.Cow:
			tame++
			if !a.Tame || a.Name == "" || a.Herd != 9 {
				t.Fatalf("a tame cow's calf is tame and named, in her herd: %+v", a)
			}
		}
	}
	if wild == 0 || tame != 1 {
		t.Fatalf("born: %d fawns, %d calves", wild, tame)
	}
	if !hasMessage(g, "Bess gave birth to a cow calf.") {
		t.Fatalf("messages %+v", g.messages)
	}
	if hasMessage(g, "fawn") {
		t.Fatal("no message for wild births")
	}
}

func TestJuvenileGrowsAndScales(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	now := uint64(g.clock.Now())
	kid := spawnSexed(t, g, "goat", 10, 10, false, true, "Kid")
	setAnimal(g, kid, func(a *animal.State) { a.Born = animal.BornAt(now, 0) })
	v := viewOf(g, kid)
	if v.Adult || v.Stage != "young" || v.Scale != 0.55 || v.Product != "" {
		t.Fatalf("newborn %+v", v)
	}
	adult := animal.LifeOf(animal.Goat).AdultDays
	advance(g, now+uint64(adult/2)*clock.TicksPerDay)
	keepFed(g, kid)
	if v := viewOf(g, kid); v.Adult || v.Scale < 0.7 || v.Scale > 0.8 {
		t.Fatalf("half grown %+v", v)
	}
	// a young one eats half portions: a grass tile fills it twice as far
	sowGrass(g, 0, 0, 29, 19)
	grown := spawnSexed(t, g, "goat", 20, 10, false, true, "Old")
	for _, id := range []entity.EntityID{kid, grown} {
		setAnimal(g, id, func(a *animal.State) { a.Food = 1000 })
	}
	a, b := animalOf(g, kid), animalOf(g, grown)
	now = uint64(g.clock.Now())
	g.eat(kid, &a, animal.FeedTarget{Kind: animal.FeedGrass, X: 10, Y: 10}, now)
	g.eat(grown, &b, animal.FeedTarget{Kind: animal.FeedGrass, X: 20, Y: 10}, now)
	if a.Food-1000 != 2*(b.Food-1000) || b.Food <= 1000 {
		t.Fatalf("young meal %d, grown meal %d", a.Food-1000, b.Food-1000)
	}
	// the young don't mate or hunt
	nanny := spawnSexed(t, g, "goat", 11, 10, true, true, "Nan")
	advance(g, uint64(g.clock.Now())/clock.TicksPerDay*clock.TicksPerDay+clock.TicksPerDay+animal.MateTick(animal.Goat, uint64(kid)))
	keepFed(g, kid)
	keepFed(g, nanny)
	g.Step(2)
	if animalOf(g, kid).Mate != 0 {
		t.Fatal("a young male does not mate")
	}
	pup, _ := g.SpawnAnimal("timber_wolf", 3, 3)
	_, _ = g.SpawnAnimal("hare", 5, 3)
	setAnimal(g, pup, func(a *animal.State) { a.Born, a.Food = animal.BornAt(uint64(g.clock.Now()), 0), 1000 })
	p := animalOf(g, pup)
	g.nothingToEat(pup, &p, pos(g, pup), uint64(g.clock.Now()))
	if p.Mode == animal.Attack || p.Target != 0 {
		t.Fatal("a young predator does not hunt")
	}
	// grown at AdultDays
	advance(g, now+uint64(adult)*clock.TicksPerDay)
	keepFed(g, kid)
	if v := viewOf(g, kid); !v.Adult || v.Scale != 1 || v.Stage != "adult" {
		t.Fatalf("grown %+v", v)
	}
}

func TestYoungFollowsMother(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 20})
	now := uint64(g.clock.Now())
	mom := spawnSexed(t, g, "goat", 30, 10, true, true, "Mom")
	kid := spawnSexed(t, g, "goat", 16, 10, false, true, "Kid") // within MotherRadius
	setAnimal(g, kid, func(a *animal.State) { a.Born = animal.BornAt(now, 0) })
	setAnimal(g, mom, func(a *animal.State) { a.NextThink = 1 << 40 }) // she stays put
	near := false
	for i := 0; i < 40 && !near; i++ {
		g.Step(100)
		keepFed(g, kid)
		keepFed(g, mom)
		setAnimal(g, mom, func(a *animal.State) { a.Path, a.PathI = nil, 0 })
		near = chebyshev(pos(g, kid), pos(g, mom)) <= animal.FollowNear+1
	}
	if !near {
		t.Fatalf("the kid follows its mother: kid %v, mother %v", pos(g, kid), pos(g, mom))
	}
}

func TestOldAgeDeath(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	now := uint64(g.clock.Now())
	life := animal.LifeOf(animal.Goat)
	old := spawnSexed(t, g, "goat", 10, 10, true, true, "Gran")
	mid := spawnSexed(t, g, "goat", 20, 10, true, true, "Mid")
	setAnimal(g, old, func(a *animal.State) { a.Born = animal.BornAt(now, uint64(life.LifeDays)*clock.TicksPerDay) })
	setAnimal(g, mid, func(a *animal.State) { a.Born = animal.BornAt(now, uint64(life.LifeDays/2)*clock.TicksPerDay) })
	if p := animal.OldAgeChance(animal.Goat, life.LifeDays); p != animal.OldAgeMax {
		t.Fatalf("chance at the lifespan %d", p)
	}
	died := 0
	for day := uint64(1); day <= 120 && died == 0; day++ {
		advance(g, day*clock.TicksPerDay+dawnHour*clock.TicksPerHour)
		keepFed(g, mid)
		keepFed(g, old)
		g.Step(1)
		if _, ok := g.animals.Get(old); !ok {
			died = int(day)
		}
	}
	if died == 0 {
		t.Fatal("an animal at its lifespan dies of old age (10% a day)")
	}
	if !hasMessage(g, "Gran the goat died of old age.") {
		t.Fatalf("messages %+v", g.messages)
	}
	if itemsOnGround(g, "goat corpse") != 1 {
		t.Fatal("it leaves a corpse")
	}
	if _, ok := g.animals.Get(mid); !ok {
		t.Fatal("a middle-aged goat never dies of old age")
	}
}

// warmDay jumps g to a summer noon (eggs incubate).
func warmDay(t *testing.T, g *Game) {
	jumpTo(g, seasonDay(t, "Summer", 1), 12)
}

// eggStack is the first stack of kind k (ok false: none).
func eggStack(g *Game, k item.Kind) (entity.EntityID, item.Stack, bool) {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k {
			return id, st, true
		}
	}
	return 0, item.Stack{}, false
}

func TestFertileEggHatches(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	warmDay(t, g)
	// a mated turkey hen lays a fertile egg on one of her laying days
	hen := spawnSexed(t, g, "turkey", 10, 10, true, true, "Hen")
	setAnimal(g, hen, func(a *animal.State) { a.Mated = animal.MatedEggs })
	day := int(uint64(g.clock.Now())/clock.TicksPerDay) + 1
	for !animal.LaysToday(animal.Turkey, uint64(hen), day) {
		day++
	}
	advance(g, uint64(day)*clock.TicksPerDay+animal.LayTick(animal.Turkey, uint64(hen)))
	keepFed(g, hen)
	g.Step(1)
	_, st, ok := eggStack(g, item.FertileEgg)
	if !ok || st.Animal != uint8(animal.Turkey) || !st.Tame || st.Qty != 1 {
		t.Fatalf("laid %+v (%v)", st, ok)
	}
	if a := animalOf(g, hen); a.Mated != animal.MatedEggs-1 || !viewOf(g, hen).EggsFertile {
		t.Fatalf("eggs left %d", a.Mated)
	}
	// a stack about to hatch: a tame chicken's eggs
	sid, _ := g.SpawnStack(item.FertileEgg, 2, 20, 10)
	cs, _ := g.stacks.Get(sid)
	cs.Animal, cs.Tame, cs.Hatch = uint8(animal.Chicken), true, animal.HatchTicks(animal.Chicken)-climateInterval
	g.stacks.Set(sid, cs)
	g.Step(climateInterval + 1)
	if _, ok := g.stacks.Get(sid); ok {
		t.Fatal("the stack hatched")
	}
	chicks := 0
	for _, id := range g.animalIDs {
		if a := animalOf(g, id); a.Species == animal.Chicken {
			chicks++
			if !a.Tame || a.Name == "" || g.grown(a, uint64(g.clock.Now())) || !hasLogKind(g, id, logHatched) {
				t.Fatalf("a tame young chick: %+v", a)
			}
		}
	}
	if chicks != 2 || !hasMessage(g, "2 chicken chicks hatched.") {
		t.Fatalf("%d chicks, messages %+v", chicks, g.messages)
	}
}

func TestFrozenEggDies(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20, Biome: biome.IceSheet})
	jumpTo(g, seasonDay(t, "Winter", 1), 2)
	if tc := g.TemperatureAt(10, 10); tc >= animal.EggFreeze {
		t.Fatalf("not cold enough: %d", tc)
	}
	sid, _ := g.SpawnStack(item.FertileEgg, 3, 10, 10)
	st, _ := g.stacks.Get(sid)
	st.Animal, st.Tame, st.Hatch = uint8(animal.Chicken), true, 1000
	g.stacks.Set(sid, st)
	g.Step(climateInterval + 1)
	if _, _, ok := eggStack(g, item.FertileEgg); ok {
		t.Fatal("a frozen egg dies")
	}
	if _, e, ok := eggStack(g, item.Egg); !ok || e.Qty != 3 || e.Animal != 0 || e.Tame || e.Hatch != 0 {
		t.Fatalf("it becomes a plain egg: %+v", e)
	}
}

func TestChickensLayWithoutRooster(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	hen := spawnSexed(t, g, "chicken", 10, 10, true, true, "Hen")
	wild := spawnSexed(t, g, "chicken", 20, 10, true, false, "")
	hungry := spawnSexed(t, g, "chicken", 25, 10, true, true, "Thin")
	ids := []entity.EntityID{hen, wild, hungry}
	sort.Slice(ids, func(i, j int) bool {
		return animal.LayTick(animal.Chicken, uint64(ids[i])) < animal.LayTick(animal.Chicken, uint64(ids[j]))
	})
	for day := uint64(1); day <= 3; day++ {
		for _, id := range ids { // each hen at her own tick of the day
			advance(g, day*clock.TicksPerDay+animal.LayTick(animal.Chicken, uint64(id)))
			keepFed(g, hen)
			keepFed(g, wild)
			setAnimal(g, hungry, func(a *animal.State) { a.Food = animal.LayFood - 1 })
			g.Step(1)
		}
	}
	if n := itemsOnGround(g, "egg"); n != 3 {
		t.Fatalf("a tame hen lays an egg a day: %d in 3 days", n)
	}
	if _, _, ok := eggStack(g, item.FertileEgg); ok {
		t.Fatal("no rooster: no fertile eggs")
	}
	// mated: every third egg of the mating is fertile
	setAnimal(g, hen, func(a *animal.State) { a.Mated = animal.PlainMatedEggs })
	fertile := 0
	for day := uint64(4); day < 4+animal.PlainMatedEggs; day++ {
		advance(g, day*clock.TicksPerDay+animal.LayTick(animal.Chicken, uint64(hen)))
		keepFed(g, hen)
		g.Step(1)
	}
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.FertileEgg {
			fertile += st.Qty
		}
	}
	if fertile != 3 || animalOf(g, hen).Mated != 0 {
		t.Fatalf("%d fertile eggs of %d, %d left", fertile, animal.PlainMatedEggs, animalOf(g, hen).Mated)
	}
}

func TestNoMatingFlap(t *testing.T) {
	// one check a day, at the male's own tick
	for _, s := range []animal.Species{animal.Goat, animal.Raccoon} {
		for id := uint64(1); id < 50; id++ {
			mt := animal.MateTick(s, id)
			if mt >= clock.TicksPerDay {
				t.Fatalf("%v %d: tick %d", s, id, mt)
			}
			if animal.SleepHours(s, int(mt/clock.TicksPerHour)) {
				t.Fatalf("%v %d checks at %d, asleep", s, id, mt/clock.TicksPerHour)
			}
		}
	}
	g := New(Config{Seed: 7, Width: 30, Height: 20})
	billy := spawnSexed(t, g, "goat", 13, 10, false, true, "Billy")
	nan := spawnSexed(t, g, "goat", 15, 10, true, true, "Nan")
	day := uint64(1)
	mt := animal.MateTick(animal.Goat, uint64(billy))
	for ; day < 30; day++ {
		advance(g, day*clock.TicksPerDay+mt)
		keepFed(g, billy)
		keepFed(g, nan)
		g.Step(1)
		if animalOf(g, billy).Mate != 0 {
			break
		}
		// no luck today: no second try until tomorrow's tick
		for uint64(g.clock.Now())+500 < (day+1)*clock.TicksPerDay+mt {
			g.Step(500)
			keepFed(g, billy)
			keepFed(g, nan)
			if animalOf(g, billy).Mate != 0 || animalOf(g, nan).Pregnant != 0 {
				t.Fatalf("day %d: a second try the same day", day)
			}
		}
	}
	if animalOf(g, billy).Mate != nan || animalOf(g, nan).Mate != billy {
		t.Fatal("a mating started")
	}
	if day == 1 {
		// fixture: seed 7 loses the first rolls, so the days without a
		// second try above are exercised
		t.Fatal("mated at the first check: pick a seed that misses it")
	}
	// called off when she lies down; not taken up again that day
	setAnimal(g, nan, func(a *animal.State) { a.Mode = animal.Rest })
	g.Step(1)
	if animalOf(g, billy).Mate != 0 || animalOf(g, nan).Mate != 0 {
		t.Fatal("the mating is called off on both sides")
	}
	for uint64(g.clock.Now())+500 < (day+1)*clock.TicksPerDay+mt {
		g.Step(500)
		keepFed(g, billy)
		keepFed(g, nan)
		if animalOf(g, billy).Mate != 0 {
			t.Fatal("no new try before tomorrow")
		}
	}
}

func TestLifeSaveRoundTrip(t *testing.T) {
	g := New(Config{Seed: 2, Width: 30, Height: 20})
	warmDay(t, g)
	now := uint64(g.clock.Now())
	daisy := spawnSexed(t, g, "goat", 10, 10, true, true, "Daisy")
	setAnimal(g, daisy, func(a *animal.State) { a.Pregnant, a.Product = now+3000, 400 })
	hen := spawnSexed(t, g, "turkey", 20, 10, true, true, "Hen")
	setAnimal(g, hen, func(a *animal.State) { a.Mated = 2 })
	sid, _ := g.SpawnStack(item.FertileEgg, 2, 25, 15)
	st, _ := g.stacks.Get(sid)
	st.Animal, st.Tame, st.Hatch = uint8(animal.Chicken), true, animal.HatchTicks(animal.Chicken)-2*climateInterval
	g.stacks.Set(sid, st)
	var buf bytes.Buffer
	if err := g.Save(&buf, "life"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := animalOf(g, daisy), animalOf(h, daisy); a.Pregnant != b.Pregnant || a.Product != b.Product || a.Female != b.Female {
		t.Fatalf("pregnancy saved: %+v vs %+v", a, b)
	}
	if animalOf(h, hen).Mated != 2 {
		t.Fatal("mated eggs saved")
	}
	if hs, _ := h.stacks.Get(sid); hs != st {
		t.Fatalf("egg stack saved: %+v vs %+v", hs, st)
	}
	for i := 0; i < 8; i++ {
		g.Step(500)
		h.Step(500)
		if len(g.animalIDs) != len(h.animalIDs) {
			t.Fatalf("step %d: %d vs %d animals", i, len(g.animalIDs), len(h.animalIDs))
		}
	}
	if animalCount(h, animal.Goat) < 2 || animalCount(h, animal.Chicken) != 2 {
		t.Fatalf("after the load the birth and the hatch happen: %d goats, %d chickens",
			animalCount(h, animal.Goat), animalCount(h, animal.Chicken))
	}
}
