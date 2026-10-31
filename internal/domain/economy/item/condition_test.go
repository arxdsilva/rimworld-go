package item

import (
	"testing"

	"rimworld-go/internal/domain/economy/quality"
)

func TestMaxHP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    Stack
		want int
	}{
		{Stack{Kind: Parka}, 140 * 1300 / 1000}, // leather by default
		{Stack{Kind: Parka, Stuff: Cloth}, 140},
		{Stack{Kind: Tuque, Stuff: Cloth}, 80},
		{Stack{Kind: Duster, Stuff: Cloth}, 200},
		{Stack{Kind: WoolParka}, 140},
		{Stack{Kind: Club}, 65}, // wood x0.65
		{Stack{Kind: Club, Stuff: Steel}, 100},
		{Stack{Kind: Rifle}, 100},
		{Stack{Kind: Wood, Qty: 50}, 0},
		{Stack{Kind: Meal}, 50}, // FOOD-4: food has hit points (wiki: Simple meal)
	}
	for _, c := range cases {
		if got := c.s.MaxHP(); got != c.want {
			t.Errorf("%v of %v: %d, want %d", c.s.Kind, c.s.Stuff, got, c.want)
		}
	}
	if MaxHP(Meal) != 50 || HasQuality(Meal) || !HasQuality(Club) || !HasQuality(Parka) || HasQuality(Wood) {
		t.Fatal("which kinds have a condition")
	}
	s := Stack{Kind: Rifle, Wear: 30}
	if s.HP() != 70 || s.WornOut() {
		t.Fatal("hp")
	}
	s.Wear = 130
	if s.HP() != 0 || !s.WornOut() {
		t.Fatal("worn out")
	}
	if (Stack{Kind: Wood, Wear: 5}).WornOut() {
		t.Fatal("a kind with no condition never wears out")
	}
}

func TestTaintedAndQualityLevel(t *testing.T) {
	t.Parallel()
	if !(Stack{Kind: Parka, Of: 9}).Tainted() || (Stack{Kind: Parka}).Tainted() || (Stack{Kind: CorpseHuman, Of: 9}).Tainted() {
		t.Fatal("tainted is apparel a dead pawn wore")
	}
	if _, ok := (Stack{Kind: Club}).QualityLevel(); ok {
		t.Fatal("no quality stored")
	}
	if l, ok := (Stack{Kind: Club, Quality: quality.Stored(quality.Good)}).QualityLevel(); !ok || l != quality.Good {
		t.Fatal("stored quality")
	}
}

func TestStageOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		hp, max int
		want    Stage
	}{
		{100, 100, Fine}, {51, 100, Fine}, {50, 100, Ratty}, {20, 100, Ratty}, {19, 100, Tattered}, {0, 100, Tattered}, {0, 0, Fine},
	}
	for _, c := range cases {
		if got := StageOf(c.hp, c.max); got != c.want {
			t.Errorf("StageOf(%d, %d) = %d, want %d", c.hp, c.max, got, c.want)
		}
	}
}

func TestDeteriorationRate(t *testing.T) {
	t.Parallel()
	club := Stack{Kind: Club}
	if DeteriorationRate(club, false, false) != DeteriorationPerDay {
		t.Fatal("dry, outdoors")
	}
	if DeteriorationRate(club, true, false) != DeteriorationPerDay*RainFactor || DeteriorationRate(club, true, true) != DeteriorationPerDay*RainFactor*WetFactor {
		t.Fatal("rain x5, water x3")
	}
	if DeteriorationRate(Stack{Kind: Club, Stuff: Steel}, true, true) != 0 || DeteriorationRate(Stack{Kind: Knife}, false, false) != 0 {
		t.Fatal("metal and stone are immune")
	}
	if DeteriorationRate(Stack{Kind: Wood, Qty: 10}, true, false) != 0 {
		t.Fatal("raw wood keeps")
	}
	leg := Stack{Kind: Club, Quality: quality.Stored(quality.Legendary)}
	awful := Stack{Kind: Club, Quality: quality.Stored(quality.Awful)}
	if DeteriorationRate(leg, false, false) != DeteriorationPerDay/10 || DeteriorationRate(awful, false, false) != DeteriorationPerDay*2 {
		t.Fatal("quality scales it")
	}
}

func TestHPValuePct(t *testing.T) {
	t.Parallel()
	cases := map[int]int{100: 1000, 95: 1000, 90: 1000, 75: 750, 60: 500, 55: 300, 50: 100, 25: 50, 0: 0, -5: 0}
	for hp, want := range cases {
		if got := HPValuePct(hp, 100); got != want {
			t.Errorf("HPValuePct(%d) = %d, want %d", hp, got, want)
		}
	}
	if HPValuePct(0, 0) != 1000 {
		t.Fatal("no condition keeps its value")
	}
}

func TestInsulationByStuffAndQuality(t *testing.T) {
	t.Parallel()
	c0, h0 := Parka.Insulation()
	if c, h := Insulation(Parka, 0, 0); c != c0 || h != h0 {
		t.Fatal("no stuff, no quality: the kind's")
	}
	if c, _ := Insulation(Parka, Cloth, 0); c != c0*1125/1000 {
		t.Fatalf("cloth parka cold %d", c)
	}
	if c, h := Insulation(Parka, Leather, quality.Stored(quality.Legendary)); c != c0*18/10 || h != h0 {
		t.Fatalf("legendary leather parka %d %d: a hot garment gets no hotter", c, h)
	}
	if _, h := Insulation(Duster, Leather, quality.Stored(quality.Good)); h <= 80 {
		t.Fatal("a good duster shades better")
	}
}
