package craft

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// stove is a fake with a stocked fueled stove (id 1) running bill r once.
func stove(r recipe.ID) (*fakeHost, *Craft) {
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Stove, 4, 4)
	c.Bills.Set(1, []recipe.Bill{{Recipe: r, Mode: recipe.DoCount, Target: 1}})
	return f, c
}

// meals is the stacks of kind k the fake holds.
func meals(f *fakeHost, k item.Kind) []item.Stack {
	var out []item.Stack
	for _, id := range f.ItemIDs() {
		if st, ok := f.Stack(id); ok && st.Kind == k {
			out = append(out, st)
		}
	}
	return out
}

// TestSimpleMealTakesNutrition (wiki: Simple meal): 0.5 nutrition of raw
// food, so two chicken eggs (0.25 each) make a meal, not ten; the meal
// remembers them.
func TestSimpleMealTakesNutrition(t *testing.T) {
	t.Parallel()
	f, c := stove(recipe.SimpleMeal)
	f.put(50, item.Stack{Kind: item.Egg, Qty: 9, X: 1, Y: 1})
	if j := runBill(t, f, c, cookOnly, 400); j.State != job.Complete || f.Stock(item.Egg) != 7 || f.meals != 1 {
		t.Fatalf("job %v eggs left %d meals %d", j.State, f.Stock(item.Egg), f.meals)
	}
	if m := meals(f, item.Meal); len(m) != 1 || m[0].Ingredients[0] != item.Egg || m[0].Ingredients[1] != 0 {
		t.Fatalf("meal %+v", m)
	}
}

// TestFineMealRemembersItsIngredients: corn and two kinds of meat
// gathered from several stacks are all on the meal, the meat with its
// species.
func TestFineMealRemembersItsIngredients(t *testing.T) {
	t.Parallel()
	f, c := stove(recipe.CookFineMeal)
	f.put(50, item.Stack{Kind: item.Corn, Qty: 5, X: 1, Y: 1})
	f.put(51, item.Stack{Kind: item.Meat, Qty: 3, Animal: 4, X: 2, Y: 1})
	f.put(52, item.Stack{Kind: item.Meat, Qty: 3, Animal: 7, X: 3, Y: 1})
	if j := runBill(t, f, c, cookOnly, 600); j.State != job.Complete {
		t.Fatalf("job %+v", j)
	}
	m := meals(f, item.FineMeal)
	if len(m) != 1 || !m[0].HasIngredient(item.Corn) || !m[0].HasIngredient(item.Meat) {
		t.Fatalf("fine meal %+v", m)
	}
	n := 0
	for i, k := range m[0].Ingredients {
		if k == item.Meat && (m[0].IngredientSpecies[i] == 4 || m[0].IngredientSpecies[i] == 7) {
			n++
		}
	}
	if n != 2 || f.Stock(item.Meat) != 1 {
		t.Fatalf("meat species on the meal %d, meat left %d", n, f.Stock(item.Meat))
	}
}

// TestMealVariantsTakeOneSide (wiki: Meals): the vegetarian fine meal
// takes 0.75 of plant food and no meat; the carnivore lavish 1.25 of meat.
func TestMealVariantsTakeOneSide(t *testing.T) {
	t.Parallel()
	f, c := stove(recipe.CookFineMealVeg)
	f.put(50, item.Stack{Kind: item.Meat, Qty: 40, X: 1, Y: 1})
	if c.Shop.Assign(pawn, cookOnly) {
		t.Fatal("a vegetarian meal took meat")
	}
	f.put(51, item.Stack{Kind: item.Rice, Qty: 20, X: 2, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.CookFineMealVeg, Mode: recipe.DoCount, Target: 1}})
	if j := runBill(t, f, c, cookOnly, 600); j.State != job.Complete || f.Stock(item.Rice) != 5 || f.Stock(item.Meat) != 40 {
		t.Fatalf("veg fine: %v rice %d meat %d", j.State, f.Stock(item.Rice), f.Stock(item.Meat))
	}
	if m := meals(f, item.FineMeal); len(m) != 1 || m[0].HasIngredient(item.Meat) {
		t.Fatalf("veg fine meal %+v", m)
	}
	f2, c2 := stove(recipe.CookLavishMealMeat)
	f2.put(50, item.Stack{Kind: item.Meat, Qty: 30, X: 1, Y: 1})
	f2.put(51, item.Stack{Kind: item.Rice, Qty: 30, X: 2, Y: 1})
	if j := runBill(t, f2, c2, cookOnly, 900); j.State != job.Complete || f2.Stock(item.Meat) != 5 || f2.Stock(item.Rice) != 30 {
		t.Fatalf("carnivore lavish: %v meat %d rice %d", j.State, f2.Stock(item.Meat), f2.Stock(item.Rice))
	}
}

// TestBatchBillCooksFour (wiki: Simple meal x4): four times the food,
// four meals, the wiki's quadruple work.
func TestBatchBillCooksFour(t *testing.T) {
	t.Parallel()
	f, c := stove(recipe.CookSimpleMealBulk)
	f.put(50, item.Stack{Kind: item.Potatoes, Qty: 45, X: 1, Y: 1})
	if d := recipe.DefOf(recipe.CookSimpleMealBulk); d.Work != 4*recipe.DefOf(recipe.SimpleMeal).Work || d.OutQty != recipe.BatchSize {
		t.Fatalf("bulk def %+v", d)
	}
	if j := runBill(t, f, c, cookOnly, 1000); j.State != job.Complete || f.meals != 4 || f.Stock(item.Potatoes) != 5 {
		t.Fatalf("job %v meals %d potatoes %d", j.State, f.meals, f.Stock(item.Potatoes))
	}
	if m := meals(f, item.Meal); len(m) != 1 || m[0].Qty != 4 || !m[0].HasIngredient(item.Potatoes) {
		t.Fatalf("batch %+v", m)
	}
}

// TestPemmicanKeepsIngredients: food that is not a cooked meal keeps its
// ingredients too (no poison roll).
func TestPemmicanKeepsIngredients(t *testing.T) {
	t.Parallel()
	f, c := stove(recipe.CookPemmican)
	f.put(50, item.Stack{Kind: item.Berries, Qty: 5, X: 1, Y: 1})
	f.put(51, item.Stack{Kind: item.Milk, Qty: 5, X: 2, Y: 1})
	if j := runBill(t, f, c, cookOnly, 900); j.State != job.Complete || f.meals != 0 {
		t.Fatalf("job %v meals %d", j.State, f.meals)
	}
	if p := meals(f, item.Pemmican); len(p) != 1 || p[0].Qty != 16 || !p[0].HasIngredient(item.Berries) || !p[0].HasIngredient(item.Milk) {
		t.Fatalf("pemmican %+v", p)
	}
}

// TestStoveBurnsWood (wiki: Fueled stove): a stove with no wood takes no
// bill, and one burns a log per WorkPerWood work, failing when it burns
// out mid-meal; the electric stove burns nothing.
func TestStoveBurnsWood(t *testing.T) {
	t.Parallel()
	f, c := stove(recipe.SimpleMeal)
	f.put(50, item.Stack{Kind: item.Rice, Qty: 30, X: 1, Y: 1})
	f.fuel[1] = 0
	if c.Shop.Assign(pawn, cookOnly) {
		t.Fatal("a cold stove took a bill")
	}
	if _, st := c.Bills.Line(1, 0); st != BillNoFuel {
		t.Fatalf("status %q", st)
	}
	f.fuel[1] = 10
	j := runBill(t, f, c, cookOnly, 400)
	if j.State != job.Fail || f.burnt == 0 || f.burnt >= recipe.DefOf(recipe.SimpleMeal).Work {
		t.Fatalf("burnt out: job %v burnt %d", j.State, f.burnt)
	}
	f.fuel[1] = 1 << 20
	f.burnt = 0
	f.jobs = map[entity.EntityID]job.Record{}
	f.claims = map[entity.EntityID]entity.EntityID{}
	if j := runBill(t, f, c, cookOnly, 400); j.State != job.Complete || f.burnt < recipe.DefOf(recipe.SimpleMeal).Work {
		t.Fatalf("cooked: job %v burnt %d", j.State, f.burnt)
	}
	if !Fueled(construction.Stove) || BurnsWhileWorked(construction.ElectricStove) || BurnsWhileWorked(construction.Campfire) {
		t.Fatal("which benches burn while worked")
	}
}

// TestBenchTemperature (wiki: Temperature): a bench works at 70 % below
// 10 C or above 35 C at its work spot.
func TestBenchTemperature(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ temp, want int }{{20, 1000}, {10, 1000}, {35, 1000}, {9, 700}, {36, 700}, {-20, 700}, {50, 700}} {
		f := newFake()
		cr := New(f)
		f.build(1, construction.Workbench, 4, 4)
		f.temp = c.temp
		if got := cr.Stations.Speed(1); got != c.want {
			t.Errorf("%d C: speed %d, want %d", c.temp, got, c.want)
		}
	}
}

// TestBenchTemperatureNoFlap: air wobbling around 10 C moves a bench's
// speed at most once per speedEvery window (the term's minimum stay).
func TestBenchTemperatureNoFlap(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.build(1, construction.Workbench, 4, 4)
	changes, last := 0, c.Stations.Speed(1)
	for i := range speedEvery * 4 {
		f.now = uint64(i)
		f.temp = 9 + i%3 // 9, 10, 11, 9, ...
		if v := c.Stations.Speed(1); v != last {
			changes++
			last = v
		}
	}
	if changes > 4 {
		t.Fatalf("speed changed %d times in 4 windows", changes)
	}
}

// TestFoodDeterioratesOutdoors (wiki: Deterioration): a meal under the
// open sky loses its wiki 10 HP a day and goes bad at none; under a roof
// (or on a shelf: the host's Exposed) it keeps; a survival meal loses a
// quarter point a day.
func TestFoodDeterioratesOutdoors(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.put(1, item.Stack{Kind: item.Meal, Qty: 3, X: 1, Y: 1})
	f.put(2, item.Stack{Kind: item.Meal, Qty: 3, X: 2, Y: 2})
	f.put(3, item.Stack{Kind: item.SurvivalMeal, Qty: 3, X: 3, Y: 3})
	f.roofed = map[[2]int]bool{{2, 2}: true}
	weatherDays(c, 1)
	if out, in, sv := f.stacks[1], f.stacks[2], f.stacks[3]; out.HP() != 40 || in.HP() != 50 || sv.Wear != 0 {
		t.Fatalf("after a day: outdoors %d, roofed %d, survival wear %d", out.HP(), in.HP(), sv.Wear)
	}
	weatherDays(c, 4)
	if _, ok := f.stacks[1]; ok || !strings.Contains(f.msgs[len(f.msgs)-1], "3 meal left outside went bad") {
		t.Fatalf("an outdoor meal after 5 days: %+v %v", f.stacks[1], f.msgs)
	}
	if f.stacks[3].Wear != 1 || f.stacks[2].Wear != 0 {
		t.Fatalf("survival wear %d, roofed wear %d", f.stacks[3].Wear, f.stacks[2].Wear)
	}
}

// TestIngredientNames: the view names tagged meat by its species.
func TestIngredientNames(t *testing.T) {
	t.Parallel()
	st := item.Stack{Kind: item.Meal}
	if IngredientNames(st) != nil {
		t.Fatal("no ingredients, no names")
	}
	st.AddIngredient(item.Rice, 0)
	st.AddIngredient(item.Meat, 1)
	st.AddIngredient(item.Meat, 0)
	if n := IngredientNames(st); len(n) != 3 || n[0] != "rice" || n[1] == "meat" || n[2] != "meat" {
		t.Fatalf("names %v", n)
	}
}
