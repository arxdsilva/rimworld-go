package animal

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
)

// TestButcherYieldByAge: a cow's body yields the wiki's 336 meat and 96
// leather grown, a juvenile's half its size, a calf's a fifth, and a body
// not carefully slaughtered two thirds (wiki: Meat amount, Animals).
func TestButcherYieldByAge(t *testing.T) {
	now := uint64(100 * clock.TicksPerDay)
	adult := LifeOf(Cow).AdultDays
	born := func(days int) uint64 { return now - uint64(days)*clock.TicksPerDay }
	for _, c := range []struct {
		name          string
		days          int
		careful       bool
		meat, leather int
	}{
		{"adult", adult + 1, true, 336, 96},
		{"juvenile", adult - 1, true, 168, 48},
		{"calf", 1, true, 67, 19}, // 0.48 body size: 67.2 meat; leather scales by the meat's share
		{"shot adult", adult + 1, false, 222, 63},
	} {
		y := CorpseYieldOf(State{Species: Cow, Born: born(c.days)}, now, c.careful)
		if m, l := CorpseYield(Cow, y); m != c.meat || l != c.leather {
			t.Errorf("%s cow: %d meat %d leather (yield %d), want %d / %d", c.name, m, l, y, c.meat, c.leather)
		}
	}
	if m, l := CorpseYield(Cow, 0); m != 336 || l != 96 {
		t.Fatal("yield 0 (an older save's corpse) is a whole adult")
	}
	if m, l := Yield(Rat); m != 31 || l != 16 {
		t.Fatalf("wiki: a rat gives 31 meat and 16 leather through the small curve, got %d/%d", m, l)
	}
	if _, l := Yield(Chicken); l != 0 {
		t.Fatal("wiki: chickens give no leather")
	}
	if m, l := Yield(Harrow); m != 0 || l != 0 {
		t.Fatal("a mechanoid is no meat")
	}
}

// TestPartsLeft: a lost leg takes its coverage off the yield.
func TestPartsLeft(t *testing.T) {
	if PartsLeft(nil) != 1000 {
		t.Fatal("no body: whole")
	}
	b := &body.Body{Anatomy: body.Quadruped, Scale: 1000}
	whole := PartsLeft(b)
	var leg body.Part
	for p := range b.PartCount() {
		if d := b.PartDef(body.Part(p)); !d.Vital && d.Coverage > 0 && body.Part(p) != b.Root() {
			leg = body.Part(p)
			break
		}
	}
	b.Missing |= 1 << leg
	if got := PartsLeft(b); got >= whole || got != whole-b.PartDef(leg).Coverage {
		t.Fatalf("parts left %d of %d after losing %s", got, whole, b.PartLabel(leg))
	}
}

func TestButcheryAndHandlingCurves(t *testing.T) {
	if ButcheryEfficiency(0) != 750 || ButcheryEfficiency(10) != 1000 || ButcheryEfficiency(20) != 1250 {
		t.Fatal("wiki: butchery efficiency 75 % + 2.5 % a level")
	}
	if MinHandling(Wildness(Cow)) != 0 || MinHandling(150) != 1 || MinHandling(980) != 10 || MinHandling(Wildness(Ostrich)) != 9 {
		t.Fatalf("minimum handling: cow %d, 15 %% %d, 98 %% %d, ostrich %d", MinHandling(0), MinHandling(150), MinHandling(980), MinHandling(950))
	}
	if TameChance(7, 0) != 500 || TameChance(10, Wildness(Deer)) != 170 || TameChance(0, 1000) != 0 {
		t.Fatalf("tame chance: %d, %d", TameChance(7, 0), TameChance(10, Wildness(Deer)))
	}
	if GatherYield(0) != 600 || GatherYield(8) != 1000 || GatherYield(20) != 1130 {
		t.Fatal("wiki: animal gather yield")
	}
	if Revenge(TimberWolf) != 100 || Revenge(Cow) != 0 || TameRevenge(Emu) != 100 || TameRevenge(Lynx) != 20 {
		t.Fatal("wiki: revenge chances")
	}
}

func TestTameFood(t *testing.T) {
	if f := TameFood(Cow); f <= 0 || f > tameFeedMax {
		t.Fatalf("cow tame food %d", f)
	}
	if TameFood(Chicken) >= TameFood(Cow) {
		t.Fatal("a small animal eats less")
	}
	if !TameFeed(Herbivore, item.Potatoes) || TameFeed(Herbivore, item.Hay) || TameFeed(Omnivore, item.Meal) ||
		TameFeed(Herbivore, item.Meat) || !TameFeed(Carnivore, item.Meat) || !TameFeed(Herbivore, item.Kibble) {
		t.Fatal("tame feed: raw food or kibble the animal eats, never hay or meals")
	}
}

func TestMeatKinds(t *testing.T) {
	st := item.Stack{Kind: item.Meat, Animal: MeatTag(Cow)}
	if MeatLabel(st) != "cow meat" || MeatLabel(item.Stack{Kind: item.Meat}) != "" {
		t.Fatal("meat names its species; untagged meat is plain meat")
	}
	if MeatOf(HiveCrawler) != item.InsectMeat || MeatOf(Deer) != item.Meat {
		t.Fatal("insects give insect meat")
	}
	if s, ok := MeatSpecies(item.Stack{Kind: item.Meat, Animal: MeatTag(Deer)}); !ok || s != Deer {
		t.Fatal("deer is species 0: the tag is the species plus one")
	}
}

func TestOmnivoresEatEggsAndMilk(t *testing.T) {
	if FeedValue(Omnivore, item.Egg) == 0 || FeedValue(Omnivore, item.Milk) == 0 || FeedValue(Herbivore, item.Egg) != 0 ||
		FeedValue(Omnivore, item.FertileEgg) != 0 || FeedValue(Carnivore, item.HumanMeat) == 0 {
		t.Fatal("omnivores eat eggs and milk; meat eaters any meat")
	}
}
