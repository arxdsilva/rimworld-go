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
	days := 0
	for d := 0; d < 40; d++ {
		if LaysToday(Turkey, 7, d) {
			days++
		}
	}
	if e := LifeOf(Turkey).EggDays; days != 40/e {
		t.Fatalf("an egg every %d days: %d in 40", e, days)
	}
	if HatchTicks(Chicken) != 6*clock.TicksPerDay || HatchTicks(Tortoise) != 10*clock.TicksPerDay {
		t.Fatal("birds hatch in 6 days, reptiles in 10")
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
	if ProductPerDay(Goat, Full) != ProductFull/2 || ProductPerDay(Goat, ProductFedFood-1) != ProductFull/4 || ProductPerDay(Goat, 0) != 0 {
		t.Fatal("product rate by food")
	}
	if YoungWord(Goat) != "kid" || YoungWord(Capybara) != "young" || YoungWord(Emu) != "chick" {
		t.Fatal("young words")
	}
	if Breeds(HiveBeetle) || Breeds(Glimmerstag) || !Breeds(Deer) {
		t.Fatal("hive insects and legendaries don't breed")
	}
}
