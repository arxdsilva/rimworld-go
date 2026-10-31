package animal

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestOldAgeChanceRamps(t *testing.T) {
	l := LifeOf(Goat).LifeDays
	for _, c := range []struct{ days, want int }{
		{0, 0}, {l / 2, 0}, {l * OldAgeFrom / 100, 0}, {l * 90 / 100, OldAgeMax / 2}, {l, OldAgeMax},
	} {
		if got := OldAgeChance(Goat, c.days); got != c.want {
			t.Errorf("day %d: %d permille, want %d", c.days, got, c.want)
		}
	}
	if OldAgeChance(Goat, 100*l) != 1000 {
		t.Error("capped at certain death")
	}
}

func TestEggRules(t *testing.T) {
	if !Layer(Chicken) || !Layer(Cobra) || Layer(Goat) {
		t.Fatal("layers")
	}
	fertile := 0
	for m := PlainMatedEggs; m > 0; m-- {
		if FertileEgg(Chicken, m) {
			fertile++
		}
	}
	if fertile != 3 || FertileEgg(Chicken, PlainMatedEggs) || !FertileEgg(Chicken, 1) {
		t.Fatalf("a plain layer's mating: every third egg fertile (%d)", fertile)
	}
	if MatedEggsOf(Turkey) != MatedEggs || !FertileEgg(Turkey, 1) || FertileEgg(Turkey, 0) {
		t.Fatal("a fertile layer lays MatedEggs fertile eggs")
	}
	// wiki: Eggs: a turkey lays every 1.333 days, a tortoise 2 eggs every
	// 6.66 days, a cobra 1.5 eggs every 10.
	for _, c := range []struct {
		s    Species
		want int
	}{{Turkey, 30}, {Chicken, 40}, {Goose, 20}, {Tortoise, 12}, {Cobra, 6}} {
		eggs := 0
		for d := 0; d < 40; d++ {
			eggs += EggsToday(c.s, 7, d)
		}
		if eggs < c.want-1 || eggs > c.want+1 {
			t.Fatalf("%s: %d eggs in 40 days, want about %d", c.s, eggs, c.want)
		}
	}
	if !LaysToday(Chicken, 7, 3) {
		t.Fatal("a chicken lays every day")
	}
	if HatchTicks(Chicken) != 35*clock.TicksPerDay/10 || HatchTicks(Ostrich) != 8*clock.TicksPerDay || HatchTicks(Cow) != 0 {
		t.Fatal("wiki: a chicken's egg hatches in 3.5 days, an ostrich's in 8")
	}
}

func TestDailyTicksAreAwake(t *testing.T) {
	for _, s := range []Species{Goat, Chicken, Raccoon, Cougar} {
		for id := uint64(1); id < 200; id++ {
			for _, tk := range []uint64{MateTick(s, id), LayTick(s, id)} {
				if tk >= clock.TicksPerDay || SleepHours(s, int(tk/clock.TicksPerHour)) {
					t.Fatalf("%v %d: tick %d (hour %d) not an awake hour", s, id, tk, tk/clock.TicksPerHour)
				}
			}
		}
	}
}

func TestProductsAndYoung(t *testing.T) {
	if !Produces(Cow, true, true) || Produces(Cow, false, true) || Produces(Cow, true, false) {
		t.Fatal("milk: grown females")
	}
	if !Produces(Sheep, false, true) || Produces(Deer, true, true) {
		t.Fatal("wool: grown animals of either sex; deer give nothing")
	}
	if ProductPerDay(Cow, Full) != ProductFull || ProductPerDay(Cow, ProductFedFood-1) != ProductFull/2 || ProductPerDay(Cow, 0) != 0 {
		t.Fatal("product rate by food")
	}
	if YoungWord(Goat) != "kid" || YoungWord(Capybara) != "young" || YoungWord(Emu) != "chick" {
		t.Fatal("young words")
	}
	if Breeds(HiveBeetle) || Breeds(Glimmerstag) || !Breeds(Deer) {
		t.Fatal("hive insects and legendaries don't breed")
	}
}

// TestProductsPerSpecies: milk and wool intervals and amounts are the
// wiki's per species (FOOD-3, wiki: Milk, Wool).
func TestProductsPerSpecies(t *testing.T) {
	for _, c := range []struct {
		s         Species
		days, qty int
	}{
		{Cow, 1, 14}, {Goat, 3, 12}, {Caribou, 2, 10}, {Dromedary, 2, 18}, {Yak, 1, 11}, {Elk, 1, 11},
		{Sheep, 10, 45}, {Alpaca, 10, 45}, {Woolhorn, 15, 120}, {Bison, 15, 120}, {GroundSloth, 20, 200},
	} {
		if l := LifeOf(c.s); l.ProductDays != c.days || l.ProductYield != c.qty {
			t.Errorf("%s: %d every %d days, want %d every %d", c.s, l.ProductYield, l.ProductDays, c.qty, c.days)
		}
	}
	if ProductPerDay(Goat, Full) != ProductFull/3 || ProductPerDay(Caribou, Full) != ProductFull/2 {
		t.Fatal("a goat fills in 3 days, a caribou in 2")
	}
}
