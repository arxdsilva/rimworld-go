package item

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/quality"
)

func TestStuffTable(t *testing.T) {
	t.Parallel()
	steel, ok := StuffOf(Steel)
	if !ok || steel.Value != 1900 || steel.MaxHP != 1000 || !steel.Immune || steel.ArmorSharp != 900 {
		t.Fatalf("steel %+v", steel)
	}
	wood, _ := StuffOf(Wood)
	if wood.Immune || wood.MaxHP != 650 || wood.SharpDamage != 400 {
		t.Fatalf("wood %+v", wood)
	}
	if g, ok := StuffOf(Stone); !ok || g != stuffs[GraniteBlocks] {
		t.Fatal("legacy stone reads as granite")
	}
	if _, ok := StuffOf(Meal); ok {
		t.Fatal("a meal is no material")
	}
	if _, ok := StuffOf(Kind(250)); ok {
		t.Fatal("unknown kind")
	}
	for _, k := range Stuffs(Woody | Metallic | Stony | Fabric | Leathery) {
		p, ok := StuffOf(k)
		if !ok || p.Value <= 0 || p.MaxHP <= 0 || p.BeautyFactor <= 0 {
			t.Errorf("%v: %+v", k, p)
		}
		if IsMetal(k) || IsStoneBlocks(k) {
			if !p.Immune {
				t.Errorf("%v deteriorates", k)
			}
		}
	}
}

func TestStuffCategories(t *testing.T) {
	t.Parallel()
	if Steel.StuffCategory() != Metallic || Leather.StuffCategory() != Leathery || Wool.StuffCategory() != Fabric ||
		MarbleBlocks.StuffCategory() != Stony || Stone.StuffCategory() != 0 || Meal.StuffCategory() != 0 || Kind(250).StuffCategory() != 0 {
		t.Fatal("categories")
	}
	if !(Woody | Metallic).Has(Metallic) || (Woody).Has(Metallic) || (Woody).Has(0) {
		t.Fatal("Has")
	}
	if got := Stuffs(Fabric | Leathery); !slices.Equal(got, []Kind{Leather, Cloth, Wool}) {
		t.Fatalf("textiles %v", got)
	}
	names := map[StuffCategory]string{0: "", Woody: "wood", Woody | Metallic: "wood or metal",
		Woody | Metallic | Stony: "wood, metal or stone blocks", Fabric | Leathery: "fabric or leather"}
	for c, want := range names {
		if c.String() != want {
			t.Errorf("%d: %q, want %q", c, c.String(), want)
		}
	}
}

func TestMadeFrom(t *testing.T) {
	t.Parallel()
	if MadeFrom(Club) != Woody|Metallic|Stony || MadeFrom(Parka) != Fabric|Leathery || MadeFrom(Rifle) != 0 || MadeFrom(Kind(250)) != 0 {
		t.Fatal("families")
	}
	if DefaultStuff(Knife) != GraniteBlocks || DefaultStuff(Tuque) != Leather || DefaultStuff(Meal) != 0 || DefaultStuff(Kind(250)) != 0 {
		t.Fatal("defaults")
	}
	if (Stack{Kind: Club}).StuffOfStack() != Wood || (Stack{Kind: Club, Stuff: Steel}).StuffOfStack() != Steel ||
		(Stack{Kind: Meal, Stuff: Steel}).StuffOfStack() != 0 {
		t.Fatal("stack stuff")
	}
	value := func(p StuffProps) int { return p.Value }
	if f := StuffFactor(Club, Steel, value); f != 1900*1000/1200 {
		t.Fatalf("steel club value factor %d", f)
	}
	if StuffFactor(Rifle, Steel, value) != 1000 || StuffFactor(Club, Meal, value) != 1000 {
		t.Fatal("no row: unchanged")
	}
	if StuffFactor(Knife, Wood, func(p StuffProps) int { return p.ColdInsulation }) != 1000 {
		t.Fatal("a zero default factor is unchanged")
	}
}

func TestStackValueByStuffQualityAndHP(t *testing.T) {
	t.Parallel()
	base := Stack{Kind: Club, Qty: 1}
	if StackValue(base) != Club.Value() {
		t.Fatalf("plain club %d", StackValue(base))
	}
	steel := base
	steel.Stuff = Steel
	if StackValue(steel) <= StackValue(base) {
		t.Fatal("a steel club is worth more than a wooden one")
	}
	leg := base
	leg.Quality = quality.Stored(quality.Legendary)
	if StackValue(leg) != Club.Value()*5 {
		t.Fatalf("legendary %d", StackValue(leg))
	}
	worn := base
	worn.Wear = worn.MaxHP() / 2 // 50 % hit points: 10 % of the value
	if v := StackValue(worn); v != Club.Value()/10 {
		t.Fatalf("half-worn %d", v)
	}
	if StackValue(Stack{Kind: Meal, Qty: 3}) != Meal.Value() {
		t.Fatal("an unmade kind keeps its value")
	}
}
