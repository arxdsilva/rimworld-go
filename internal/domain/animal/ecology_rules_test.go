package animal

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
)

// The pure rules of ecology.go that the game package exercises only
// through whole simulations (package coverage bar, PLAY-022).

func TestDietAndProductWords(t *testing.T) {
	t.Parallel()
	for d, want := range map[Diet]string{Herbivore: "plants", Carnivore: "meat", Omnivore: "plants and meat", Dendrovore: "trees", NoDiet: "", Diet(99): ""} {
		if d.String() != want {
			t.Errorf("diet %d = %q, want %q", d, d.String(), want)
		}
	}
	if !Herbivore.Plants() || Herbivore.Meat() || !Carnivore.Meat() || Carnivore.Plants() || !Omnivore.Plants() || !Omnivore.Meat() ||
		Dendrovore.Plants() || NoDiet.Meat() {
		t.Error("diets graze and eat meat as named")
	}
	for p, want := range map[Product]string{NoProduct: "", Milk: "milk", Wool: "wool", EmberOil: "ember oil", Product(99): ""} {
		if p.String() != want {
			t.Errorf("product %d = %q, want %q", p, p.String(), want)
		}
	}
}

func TestEveryModeHasAWord(t *testing.T) {
	t.Parallel()
	for m, want := range map[Mode]string{Wander: "wandering", Flee: "fleeing", Attack: "attacking", Feed: "feeding",
		Rest: "resting", Hunt: "hunting", Leave: "leaving", Confused: "confused"} {
		if m.String() != want {
			t.Errorf("mode %d = %q, want %q", m, m.String(), want)
		}
	}
}

func TestAgeStagesAndGrowth(t *testing.T) {
	t.Parallel()
	adult := LifeOf(Deer).AdultDays
	now := uint64(40 * clock.TicksPerDay)
	if AgeDays(0, now, Deer) != 2*adult {
		t.Error("born 0: an adult from before ages were kept")
	}
	if got := AgeDays(BornAt(now, uint64(3*clock.TicksPerDay)), now, Deer); got != 3 {
		t.Errorf("3 days old reads %d", got)
	}
	if AgeDays(now+5, now, Deer) != 0 {
		t.Error("born in the future is newborn")
	}
	if BornAt(10, 10) != 1 {
		t.Error("born tick 0 means an old save: never produced")
	}
	if Adult(Deer, adult-1) || !Adult(Deer, adult) {
		t.Error("adult at AdultDays")
	}
	if GrowthScale(Deer, adult) != 1 || GrowthScale(Deer, -3) != 0.55 || GrowthScale(Deer, adult/2) <= 0.55 {
		t.Error("growth runs from 0.55 to 1")
	}
	for s, want := range map[Species]string{Chicken: "chick", Tortoise: "hatchling", Deer: "young"} {
		if Stage(s, false) != want || Stage(s, true) != "adult" {
			t.Errorf("%s stage %q / %q", s, Stage(s, false), Stage(s, true))
		}
	}
	if ArtFemale(1) == ArtFemale(2) && ArtFemale(2) == ArtFemale(3) && ArtFemale(3) == ArtFemale(4) {
		t.Error("the art's sex hash splits ids")
	}
}

func TestNormaliseOlderSaves(t *testing.T) {
	t.Parallel()
	s := State{Species: TimberWolf, Hunger: DefOf(TimberWolf).HungerTicks / 2}
	s.Normalise(7)
	if !s.HasSex || s.Female != ArtFemale(7) || s.Rest != Full || s.Hunger != 0 || s.Food <= 0 || s.Food >= Full {
		t.Errorf("a pre-needs wolf half way to hungry: %+v", s)
	}
	fed := State{Species: Deer, HasSex: true, Food: 5000, Rest: 4000}
	fed.Normalise(1)
	if fed.Food != 5000 || fed.Rest != 4000 {
		t.Errorf("a current save is left alone: %+v", fed)
	}
}

func TestFeedValues(t *testing.T) {
	t.Parallel()
	if FeedValue(Herbivore, item.Straw) != HayFood || FeedValue(Carnivore, item.Straw) != 0 {
		t.Error("straw feeds plant eaters")
	}
	if FeedValue(Carnivore, item.Meat) != item.Meat.Nutrition() || FeedValue(Herbivore, item.Meat) != 0 {
		t.Error("raw meat feeds meat eaters")
	}
	if FeedValue(Herbivore, item.Berries) != item.Berries.Nutrition() || FeedValue(Carnivore, item.Berries) != 0 || !PlantFeed(item.Corn) || PlantFeed(item.Meat) {
		t.Error("raw plant food feeds plant eaters")
	}
	if CarcassFood(Deer) < item.Meat.Nutrition() || GestTicks(Deer) != uint64(LifeOf(Deer).GestCenti)*clock.TicksPerDay/100 {
		t.Error("a carcass is its meat; gestation is days")
	}
}
