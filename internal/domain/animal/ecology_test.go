package animal

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
)

// Every livable biome has prey to hunt and something hunting it.
func TestEveryBiomeHasPreyAndPredator(t *testing.T) {
	for _, b := range biome.All() {
		if b.ID == biome.SeaIce || b.ID == biome.IceSheet {
			continue
		}
		prey, hunters := 0, 0
		for _, s := range All() {
			if Weight(s, b.ID) == 0 {
				continue
			}
			if d := DefOf(s); d.Predator {
				hunters++
			} else {
				prey++
			}
		}
		if prey < 3 || hunters < 1 {
			t.Errorf("%s: %d prey, %d predators", b.Key, prey, hunters)
		}
	}
}

func TestDomesticAndEventAnimalsNeverWild(t *testing.T) {
	for _, s := range []Species{Cow, Goat, Pig, Sheep, Cat, Husky, Labrador, YorkshireTerrier, Chicken,
		Timbergnaw, Glimmerstag, HiveBeetle, HiveBrute, HiveCrawler} {
		for _, b := range biome.All() {
			if Wild(s, b.ID) {
				t.Errorf("%s wild in %s", s, b.Key)
			}
		}
	}
}

func TestWeightFavoursSmallAndCalm(t *testing.T) {
	if hare, wolf := Weight(Hare, biome.TemperateForest), Weight(TimberWolf, biome.TemperateForest); hare <= wolf*3 {
		t.Fatalf("hare %d vs wolf %d: small prey should far outnumber pack predators", hare, wolf)
	}
	if core, marg := Weight(Deer, biome.BorealForest), Weight(Deer, biome.ColdBog); marg == 0 || marg >= core {
		t.Fatalf("deer core %d marginal %d", core, marg)
	}
}

func TestEcologyRows(t *testing.T) {
	for _, s := range All() {
		e, r, l := EcologyOf(s), RosterOf(s), LifeOf(s)
		if e.Eggs != NoEggs && r.Archetype != ArchBird && r.Archetype != ArchSnake && r.Archetype != ArchLizard && r.Archetype != ArchTortoise {
			t.Errorf("%s lays eggs but is a %d", s, r.Archetype)
		}
		if e.Eggs != NoEggs && l.HatchDays == 0 {
			t.Errorf("%s: eggs with no hatch time", s)
		}
		if e.Product != NoProduct && (l.ProductDays == 0 || l.ProductYield == 0 || e.Product.Item() == 0) {
			t.Errorf("%s: product %s without numbers", s, e.Product)
		}
		if r.Behaviour == Hive && e.Diet != NoDiet {
			t.Errorf("%s: hive insects have no needs", s)
		}
		if l.AdultDays <= 0 || l.LifeDays <= l.AdultDays || l.LitterMin < 1 || l.LitterMax < l.LitterMin {
			t.Errorf("%s: life %+v", s, l)
		}
	}
}

func TestSleepHoursAndMeals(t *testing.T) {
	if !SleepHours(Deer, 23) || SleepHours(Deer, 12) || !SleepHours(RedFox, 12) || SleepHours(RedFox, 23) {
		t.Fatal("diurnal deer sleep at night, nocturnal foxes by day")
	}
	if Meal(Rat, GrazeFood) <= Meal(Elephant, GrazeFood) {
		t.Fatal("one tile means more to a small body")
	}
	if AgeDays(0, 999999, Deer) < LifeOf(Deer).AdultDays {
		t.Fatal("an animal from an older save is an adult")
	}
}
