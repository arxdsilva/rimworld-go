package game

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/temperature"
)

// seasonDay is the first day (from dayFrom) whose noon falls in season.
func seasonDay(t *testing.T, season string, dayFrom int) int {
	t.Helper()
	for d := dayFrom; d < dayFrom+clock.DaysPerYear; d++ {
		if temperature.Season(clock.Tick(d*clock.TicksPerDay+12*clock.TicksPerHour)) == season {
			return d
		}
	}
	t.Fatalf("no %s day", season)
	return 0
}

func speciesOf(g *Game) []animal.Species {
	var out []animal.Species
	for _, id := range g.animalIDs {
		a, _ := g.animals.Get(id)
		out = append(out, a.Species)
	}
	return out
}

func onlyHabitat(s animal.Species, h animal.Habitat) bool { return animal.RosterOf(s).Habitat == h }

// SIM-024 changed this test: start wildlife now comes from the biome's
// species (animal.Weight), so no single biome has both cold-only and
// hot-only animals to compare. The season still weighs within a biome:
// boreal cold-climate animals are drawn more in winter, arid hot-climate
// ones more in summer.
func TestStartWildlifeBySeason(t *testing.T) {
	cold, hot := map[string]int{}, map[string]int{}
	for _, season := range []string{"Winter", "Summer"} {
		day := seasonDay(t, season, 1)
		for _, b := range []biome.ID{biome.BorealForest, biome.AridShrubland} {
			for seed := uint64(1); seed <= 40; seed++ {
				g := New(Config{Seed: seed, Width: 60, Height: 60, Biome: b})
				jumpTo(g, day, 12)
				g.spawnWildlife()
				preds, grazers := 0, 0
				for _, s := range speciesOf(g) {
					d := animal.DefOf(s)
					if d.Hive || d.Legendary {
						t.Fatalf("%s at colony start", s)
					}
					if animal.Weight(s, b) == 0 {
						t.Fatalf("%s is not wild in %v", s, b)
					}
					if d.Predator {
						preds++
					} else {
						grazers++
					}
					switch {
					case onlyHabitat(s, animal.HabCold):
						cold[season]++
					case onlyHabitat(s, animal.HabHot):
						hot[season]++
					}
				}
				if preds == 0 || grazers == 0 {
					t.Fatalf("seed %d %s %v: %d predators, %d grazers", seed, season, b, preds, grazers)
				}
			}
		}
	}
	if cold["Winter"] <= cold["Summer"] || hot["Summer"] <= hot["Winter"] {
		t.Fatalf("season weighting: cold %v, hot %v", cold, hot)
	}
}

// SIM-024 changed this test: the season factor is seasonWeight; wildWeight
// multiplies it by the biome's commonality (domestic and pet species are
// never wild now instead of rare).
func TestWildlifeWeights(t *testing.T) {
	t.Parallel()
	if w := seasonWeight(animal.Caribou, "Winter"); w != 3000 {
		t.Errorf("cold species in winter %d", w)
	}
	if w := seasonWeight(animal.Caribou, "Summer"); w != 300 {
		t.Errorf("cold species in summer %d", w)
	}
	if w := seasonWeight(animal.Gazelle, "Summer"); w != 3000 {
		t.Errorf("hot species in summer %d", w)
	}
	if w := seasonWeight(animal.Deer, "Winter"); w != 1000 {
		t.Errorf("temperate %d", w)
	}
	if w := seasonWeight(animal.Rat, "Summer"); w != 600 {
		t.Errorf("any-habitat %d", w)
	}
	if w := seasonWeight(animal.Ibex, "Winter"); w != 2000 { // cold|temperate: the mean
		t.Errorf("mixed habitat %d", w)
	}
	if w := wildWeight(animal.Deer, biome.TemperateForest, "Winter"); w != animal.Weight(animal.Deer, biome.TemperateForest)*1000 {
		t.Errorf("deer in a temperate winter %d", w)
	}
	for _, s := range []animal.Species{animal.HiveBeetle, animal.Glimmerstag, animal.Cow, animal.Husky} {
		if wildWeight(s, biome.TemperateForest, "Spring") != 0 {
			t.Errorf("%s never spawns wild", s)
		}
	}
	if wildWeight(animal.Caribou, biome.TemperateForest, "Winter") != 0 {
		t.Error("caribou are not temperate forest wildlife")
	}
}

// TestMigrationDeterministic: the dawn top-up (a prey herd whenever fewer
// than six grazers were left) became the hourly capacity check (SIM-025,
// population.go): an empty wild map gets one group of one species at an
// edge, the same for the same seed; a map at capacity gets nobody, at any
// hour.
func TestMigrationDeterministic(t *testing.T) {
	run := func() ([]animal.Species, []Position) {
		g := New(Config{Seed: 11, Width: 50, Height: 50})
		g.wild = true
		for h := 1; len(g.animalIDs) == 0 && h < 24*20; h++ {
			g.migrate(uint64(3*clock.TicksPerDay + h*clock.TicksPerHour))
		}
		var ps []Position
		for _, id := range g.animalIDs {
			p, _ := g.positions.Get(id)
			ps = append(ps, p)
		}
		return speciesOf(g), ps
	}
	s1, p1 := run()
	s2, p2 := run()
	if len(s1) == 0 {
		t.Fatal("an empty map gets a group within 20 days")
	}
	if !reflect.DeepEqual(s1, s2) || !reflect.DeepEqual(p1, p2) {
		t.Fatalf("migration differs: %v %v vs %v %v", s1, p1, s2, p2)
	}
	for i, s := range s1 {
		if s != s1[0] {
			t.Fatalf("one group of one species arrives: %v", s1)
		}
		if p := p1[i]; min(p.X, p.Y, 49-p.X, 49-p.Y) > 2 {
			t.Fatalf("arrived at %v, not at an edge", p)
		}
	}
	// a stocked map (at capacity) gets nobody, dawn or not
	g := New(Config{Seed: 11, Width: 50, Height: 50})
	g.wild = true
	for i := 0; len(g.animalIDs) < g.wildlifeCap(); i++ {
		_, _ = g.SpawnAnimal("deer", 5+i%40, 5+i/40)
	}
	n := len(g.animalIDs)
	for h := 0; h < 48; h++ {
		g.migrate(uint64(3*clock.TicksPerDay + h*clock.TicksPerHour))
	}
	if len(g.animalIDs) != n {
		t.Fatalf("stocked map grew from %d to %d", n, len(g.animalIDs))
	}
}

func TestPackPredatorsHunt(t *testing.T) {
	g := New(Config{Seed: 4, Width: 40, Height: 20})
	w1, _ := g.SpawnAnimal("timber_wolf", 10, 10)
	w2, _ := g.SpawnAnimal("timber_wolf", 11, 12)
	w3, _ := g.SpawnAnimal("timber_wolf", 9, 8)
	far, _ := g.SpawnAnimal("timber_wolf", 38, 1)
	deer, _ := g.SpawnAnimal("deer", 20, 10)
	a, _ := g.animals.Get(w1)
	a.Hunger = animal.DefOf(animal.TimberWolf).HungerTicks
	a.NextThink = 0
	g.animals.Set(w1, a)
	g.thinkAnimal(w1, &a, uint64(g.clock.Now()))
	g.animals.Set(w1, a)
	if a.Target != deer {
		t.Fatalf("hungry wolf hunts the deer: target %d", a.Target)
	}
	for _, id := range []entity.EntityID{w2, w3} {
		if o, _ := g.animals.Get(id); o.Target != deer {
			t.Fatalf("pack mate %d joins the hunt: %+v", id, o.Target)
		}
	}
	if o, _ := g.animals.Get(far); o.Target != 0 {
		t.Fatal("a wolf across the map is not in the pack")
	}
	// predators never hunt a bigger animal or another predator
	g2 := New(Config{Seed: 4, Width: 40, Height: 20})
	wolf, _ := g2.SpawnAnimal("timber_wolf", 10, 10)
	_, _ = g2.SpawnAnimal("elephant", 12, 10)
	_, _ = g2.SpawnAnimal("cougar", 11, 11)
	pos, _ := g2.positions.Get(wolf)
	if got := g2.findPrey(wolf, pos, animal.DefOf(animal.TimberWolf)); got != 0 {
		t.Fatalf("wolf picked %d", got)
	}
}

func TestSmallPreyOnly(t *testing.T) {
	g := New(Config{Seed: 4, Width: 40, Height: 20})
	fox, _ := g.SpawnAnimal("red_fox", 10, 10)
	_, _ = g.SpawnAnimal("deer", 11, 10)
	hare, _ := g.SpawnAnimal("hare", 16, 10)
	pos, _ := g.positions.Get(fox)
	def := animal.DefOf(animal.RedFox)
	if got := g.findPrey(fox, pos, def); got != hare {
		t.Fatalf("a fox takes the hare, not the nearer deer: got %d", got)
	}
	g.removeAnimal(hare)
	if got := g.findPrey(fox, pos, def); got != 0 {
		t.Fatalf("no small prey left: got %d", got)
	}
}

func TestExplosiveBurstOnDeath(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 20})
	boom, _ := g.SpawnAnimal("emberbuck", 10, 10)
	near, _ := g.SpawnNamedPawn(11, 10, "Near")
	diag, _ := g.SpawnAnimal("deer", 9, 9)
	far, _ := g.SpawnNamedPawn(13, 10, "Far")
	g.hurt(boom, 1000, 0)
	if animalAlive(g, boom) {
		t.Fatal("emberbuck dead")
	}
	if got := g.VitalOf(near).Injury; got != 15 {
		t.Fatalf("adjacent colonist takes 15, got %d", got)
	}
	if d, _ := g.animals.Get(diag); d.Injury != 10 {
		t.Fatalf("diagonal deer takes 10, got %d", d.Injury)
	}
	if g.VitalOf(far).Injury != 0 {
		t.Fatal("two tiles away is out of the burst")
	}
	w := g.World()
	if len(w.Bursts) != 1 || w.Bursts[0].X != 10 || w.Bursts[0].Y != 10 || w.Bursts[0].Species != "emberbuck" {
		t.Fatalf("bursts %+v", w.Bursts)
	}
	if !hasMessage(g, "The emberbuck burst into flames!") {
		t.Fatalf("messages %+v", g.messages)
	}
	// the stamp fades after an hour
	g.clock.Advance(clock.TicksPerHour + 1)
	g.phaseAnimals()
	if len(g.World().Bursts) != 0 {
		t.Fatal("burst stamp kept past an hour")
	}
	// a burst sets off a neighbouring cinder rat in turn
	g2 := New(Config{Seed: 2, Width: 20, Height: 20})
	b1, _ := g2.SpawnAnimal("cinder_rat", 5, 5)
	b2, _ := g2.SpawnAnimal("cinder_rat", 6, 5)
	g2.hurt(b1, 100, 0)
	if animalAlive(g2, b2) || len(g2.wl.Bursts) != 2 {
		t.Fatalf("chain: rat alive %v, bursts %d", animalAlive(g2, b2), len(g2.wl.Bursts))
	}
}

func hasMessage(g *Game, sub string) bool {
	for _, m := range g.messages {
		if strings.Contains(m.Text, sub) {
			return true
		}
	}
	return false
}

func TestButcherYieldBySize(t *testing.T) {
	for _, c := range []struct {
		species       string
		meat, leather int
	}{{"chicken", 2, 0}, {"cow", 30, 10}} {
		g := New(Config{Seed: 1, Width: 16, Height: 10})
		if _, err := g.SpawnNamedPawn(2, 5, "Cook"); err != nil {
			t.Fatal(err)
		}
		placeBuilt(t, g, construction.ButcherTable, construction.South, 6, 3)
		s, _ := animal.ParseSpecies(c.species)
		g.dropCorpse(s, 3, 7)
		label := strings.ReplaceAll(c.species, "_", " ") + " corpse"
		if itemsOnGround(g, label) != 1 {
			t.Fatalf("%s corpse on the ground: %+v", c.species, g.World().Items)
		}
		for i := 0; i < 100 && itemsOnGround(g, "meat") == 0; i++ {
			g.Step(30)
		}
		if got := itemsOnGround(g, "meat"); got != c.meat {
			t.Fatalf("%s meat = %d, want %d", c.species, got, c.meat)
		}
		if got := itemsOnGround(g, "leather"); got != c.leather {
			t.Fatalf("%s leather = %d, want %d", c.species, got, c.leather)
		}
		if itemsOnGround(g, label) != 0 {
			t.Fatal("corpse consumed")
		}
	}
}

func TestAnimalCorpsesStackBySpecies(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	g.dropCorpse(animal.Deer, 4, 4)
	g.dropCorpse(animal.Deer, 4, 4)
	g.dropCorpse(animal.TimberWolf, 4, 4)
	byLabel := map[string]int{}
	for _, it := range g.World().Items {
		byLabel[it.Kind] += it.Qty
		if it.Kind == "deer corpse" && (it.Animal != "deer" || it.Qty != 2 || it.X != 4 || it.Y != 4) {
			t.Fatalf("deer stack %+v", it)
		}
		if it.Kind == "timber wolf corpse" && it.Animal != "timber_wolf" {
			t.Fatalf("wolf stack %+v", it)
		}
	}
	if byLabel["deer corpse"] != 2 || byLabel["timber wolf corpse"] != 1 || len(g.World().Items) != 2 {
		t.Fatalf("items %+v", g.World().Items)
	}
}

func TestOldSaveCorpsesConvert(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	if _, err := g.SpawnStack(item.CorpseDustbuck, 2, 3, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.CorpseRidgewolf, 1, 5, 3); err != nil {
		t.Fatal(err)
	}
	placeBuilt(t, g, construction.ButcherTable, construction.South, 8, 3)
	bid := g.bldgIDs[0]
	g.bills[bid] = []recipe.Bill{
		{Recipe: recipe.ButcherDustbuck, Mode: recipe.DoCount, Target: 4},
		{Recipe: recipe.ButcherRidgewolf, Mode: recipe.Forever, Target: 1},
		{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1},
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ItemView{}
	for _, it := range h.World().Items {
		got[it.Kind] = it
	}
	if d := got["deer corpse"]; d.Qty != 2 || d.Animal != "deer" || d.X != 3 {
		t.Fatalf("dustbuck corpses became deer corpses: %+v", h.World().Items)
	}
	if w := got["timber wolf corpse"]; w.Qty != 1 || w.Animal != "timber_wolf" {
		t.Fatalf("ridgewolf corpse became a timber wolf corpse: %+v", h.World().Items)
	}
	for _, id := range h.itemIDs {
		if st, _ := h.stacks.Get(id); st.Kind == item.CorpseDustbuck || st.Kind == item.CorpseRidgewolf {
			t.Fatalf("legacy corpse left: %+v", st)
		}
	}
	bills := h.bills[bid]
	if len(bills) != 2 || bills[0].Recipe != recipe.ButcherAnimal || bills[0].Mode != recipe.DoCount ||
		bills[0].Target != 4 || bills[1].Recipe != recipe.SimpleMeal {
		t.Fatalf("legacy butcher bills fold into one butcher-animal bill: %+v", bills)
	}
}

func TestAttackVerbs(t *testing.T) {
	t.Parallel()
	for wk, want := range map[weapon.Kind][2]string{
		weapon.Bite: {"Bit deer (-3)", "Bitten by deer (-3)"}, weapon.Claw: {"Clawed deer (-3)", "Clawed by deer (-3)"},
		weapon.Kick: {"Kicked deer (-3)", "Kicked by deer (-3)"}, weapon.Gore: {"Gored deer (-3)", "Gored by deer (-3)"},
		weapon.Headbutt: {"Headbutted deer (-3)", "Headbutted by deer (-3)"},
		weapon.Peck:     {"Pecked deer (-3)", "Pecked by deer (-3)"},
	} {
		by := logText(logEntry{Kind: logBit, Name: "deer", Amount: 3, Weapon: uint8(wk)})
		of := logText(logEntry{Kind: logBitten, Name: "deer", Amount: 3, Weapon: uint8(wk)})
		if by != want[0] || of != want[1] {
			t.Errorf("%v: %q / %q", wk, by, of)
		}
	}
}

func TestAnimalActionIsItsAttack(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 10})
	p, _ := g.SpawnNamedPawn(5, 5, "Ada")
	for _, c := range []struct{ species, action string }{{"cow", "headbutt"}, {"chicken", "peck"}, {"cougar", "claw"},
		{"bison", "gore"}, {"horse", "kick"}, {"timber_wolf", "bite"}} {
		id, err := g.SpawnAnimal(c.species, 6, 5)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := g.animals.Get(id)
		a.Mode, a.Target = animal.Attack, p
		g.animals.Set(id, a)
		var av AnimalView
		for _, v := range g.World().Animals {
			if v.ID == id {
				av = v
			}
		}
		if av.Action != c.action || av.Attack != c.action {
			t.Errorf("%s attacking: action %q attack %q", c.species, av.Action, av.Attack)
		}
		g.removeAnimal(id)
	}
}

func TestAnimalStrikeScalesBySize(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 10})
	p, _ := g.SpawnNamedPawn(5, 5, "Ada")
	chick, _ := g.SpawnAnimal("chicken", 6, 5)
	g.strikeUntilHit(t, chick, p, animal.DefOf(animal.Chicken).Attack)
	if got := g.VitalOf(p).Injury; got != 2 {
		t.Fatalf("a chicken pecks for 2, got %d", got)
	}
	bear, _ := g.SpawnAnimal("grizzly_bear", 4, 5)
	g.strikeUntilHit(t, bear, p, animal.DefOf(animal.GrizzlyBear).Attack)
	if got := g.VitalOf(p).Injury; got != 2+11 {
		t.Fatalf("a grizzly claws for 11, got %d", got-2)
	}
}

func TestSpawnAnimalAcceptsEveryKey(t *testing.T) {
	g := New(Config{Seed: 1, Width: 70, Height: 4})
	for i, s := range animal.All() {
		if _, err := g.SpawnAnimal(s.String(), i, 1); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	for _, alias := range []string{"dustbuck", "ridgewolf"} {
		if _, err := g.SpawnAnimal(alias, 1, 2); err != nil {
			t.Fatalf("%s: %v", alias, err)
		}
		g.removeAnimal(g.animalIDs[len(g.animalIDs)-1])
	}
	if _, err := g.SpawnAnimal("muffalo", 1, 2); err == nil {
		t.Fatal("unknown species")
	}
}
