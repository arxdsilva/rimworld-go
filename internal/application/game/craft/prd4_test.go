package craft

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

var cookOnly = work.Mask(0).Set(work.Cook, true)

// barrel is a fake with a fermenting barrel (id 1) and its default fill
// bill, and wort lying about.
func barrel(t *testing.T, wort int) (*fakeHost, *Craft) {
	t.Helper()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.FermentingBarrel, 4, 4)
	c.Bills.Defaults(1, construction.FermentingBarrel)
	if bills := c.Bills.Of(1); len(bills) != 1 || bills[0].Recipe != recipe.FillBarrel || bills[0].Mode != recipe.Forever {
		t.Fatalf("a barrel's default bill: %+v", bills)
	}
	if wort > 0 {
		f.put(50, item.Stack{Kind: item.Wort, Qty: wort, X: 1, Y: 1})
	}
	return f, c
}

// TestBarrelFermentsWortIntoBeer (wiki: Fermenting barrel): a cook pours
// up to 25 wort in; at 20 C it ferments into as much beer in 6 days,
// set down on the barrel's spot; the bill status says fermenting.
func TestBarrelFermentsWortIntoBeer(t *testing.T) {
	t.Parallel()
	f, c := barrel(t, 40)
	j := runBill(t, f, c, cookOnly, 200)
	if j.State != job.Complete || c.Drying.Batch(1).Qty != recipe.BarrelWort || f.Stock(item.Wort) != 15 {
		t.Fatalf("job %v barrel %+v wort left %d", j.State, c.Drying.Batch(1), f.Stock(item.Wort))
	}
	if _, st := c.Bills.Line(1, 0); st != BillFermenting {
		t.Fatalf("status %q", st)
	}
	if c.Shop.Assign(pawn, cookOnly) {
		t.Fatal("a full barrel took more wort")
	}
	c.Drying.Advance(FermentTicks / 2)
	if p := c.Drying.Batch(1).Percent(); p < 49 || p > 51 {
		t.Fatalf("half way: %d%%", p)
	}
	c.Drying.Advance(FermentTicks / 2)
	if c.Drying.Batch(1).Qty != 0 || f.Stock(item.Beer) != recipe.BarrelWort {
		t.Fatalf("after 6 days: barrel %+v beer %d", c.Drying.Batch(1), f.Stock(item.Beer))
	}
	if !strings.Contains(f.msgs[len(f.msgs)-1], "25 beer ready") {
		t.Fatalf("messages %v", f.msgs)
	}
}

// TestBarrelTemperature: cool air pauses it, frost or heat spoils it
// after a day, and wort poured later stretches the time left.
func TestBarrelTemperature(t *testing.T) {
	t.Parallel()
	f, c := barrel(t, 0)
	c.Drying.pour(1, item.Stack{Kind: item.Wort, Qty: 10}, 4, 5)
	f.temp = 3 // paused
	c.Drying.Advance(FermentTicks)
	if b := c.Drying.Batch(1); b.Left != FermentTicks || b.Rot != 0 {
		t.Fatalf("paused barrel %+v", b)
	}
	f.temp = 20
	c.Drying.Advance(FermentTicks / 20) // 5 %
	c.Drying.pour(1, item.Stack{Kind: item.Wort, Qty: 10}, 4, 5)
	if b := c.Drying.Batch(1); b.Qty != 20 || b.Left <= FermentTicks*95/100 || b.Left >= FermentTicks {
		t.Fatalf("topped up: %+v (the time left stretches)", b)
	}
	c.Drying.pour(1, item.Stack{Kind: item.Wort, Qty: 9}, 4, 5) // 5 over the top stay on the spot
	if c.Drying.Batch(1).Qty != recipe.BarrelWort || f.Stock(item.Wort) != 4 {
		t.Fatalf("overfill: %+v, %d left on the spot", c.Drying.Batch(1), f.Stock(item.Wort))
	}
	f.temp = 40
	c.Drying.Advance(RuinTicks - 1)
	if c.Drying.Batch(1).Qty == 0 {
		t.Fatal("ruined too soon")
	}
	c.Drying.Advance(1)
	if c.Drying.Batch(1).Qty != 0 || f.Stock(item.Beer) != 0 || !strings.Contains(f.msgs[len(f.msgs)-1], "spoiled") {
		t.Fatalf("hot barrel: %+v beer %d msgs %v", c.Drying.Batch(1), f.Stock(item.Beer), f.msgs)
	}
	// a barrel taken down leaves its wort on the ground
	c.Drying.pour(1, item.Stack{Kind: item.Wort, Qty: 3}, 4, 5)
	f.remove(1)
	c.Drying.Empty(1, 4, 5)
	if c.Drying.Batch(1).Qty != 0 || f.Stock(item.Wort) != 7 {
		t.Fatalf("a removed barrel: %+v, wort %d", c.Drying.Batch(1), f.Stock(item.Wort))
	}
}

// TestBarrelFillNoFlap: the barrel takes wort while empty or barely
// started; once fermenting past TopUpPct it stays closed until emptied,
// however the stock of wort or beer moves, so cooks never flap between
// topping up and waiting.
func TestBarrelFillNoFlap(t *testing.T) {
	t.Parallel()
	f, c := barrel(t, 0)
	if !c.Drying.barrelOpen(1) {
		t.Fatal("an empty barrel is open")
	}
	c.Drying.pour(1, item.Stack{Kind: item.Wort, Qty: 5}, 4, 5)
	if !c.Drying.barrelOpen(1) {
		t.Fatal("a barely started barrel takes more")
	}
	opened := 0
	was := true
	for range 100 {
		c.Drying.Advance(FermentTicks / 200)
		f.put(f.next+1, item.Stack{Kind: item.Wort, Qty: 1}) // wort keeps arriving
		f.next++
		if now := c.Drying.barrelOpen(1); now != was {
			opened++
			was = now
		}
	}
	if opened != 1 || was {
		t.Fatalf("the gate changed %d times (open now %v), want one close", opened, was)
	}
	c.Drying.Advance(FermentTicks)
	if !c.Drying.barrelOpen(1) || f.Stock(item.Beer) != 5 {
		t.Fatalf("emptied barrel reopens: open %v beer %d", c.Drying.barrelOpen(1), f.Stock(item.Beer))
	}
}

// TestFineMealTakesVegAndMeat (wiki: Fine meal): five plant food and
// five meat or animal product units (0.25 + 0.25 nutrition, FOOD-1) make
// a fine meal, which carries the cook's poison roll like the simple meal.
func TestFineMealTakesVegAndMeat(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Stove, 4, 4)
	f.put(50, item.Stack{Kind: item.Corn, Qty: 7, X: 1, Y: 1})
	f.put(51, item.Stack{Kind: item.Egg, Qty: 7, X: 2, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.CookFineMeal, Mode: recipe.DoCount, Target: 1}})
	j := runBill(t, f, c, cookOnly, 400)
	if j.State != job.Complete || f.Stock(item.FineMeal) != 1 || f.meals != 1 || f.Stock(item.Corn) != 2 || f.Stock(item.Egg) != 2 {
		t.Fatalf("job %v fine %d meals %d corn %d egg %d", j.State, f.Stock(item.FineMeal), f.meals, f.Stock(item.Corn), f.Stock(item.Egg))
	}
}

// TestSmeltApparelGivesSteelBack: metal armor smelts to a quarter of its
// steel; cloth clothes are not taken.
func TestSmeltApparelGivesSteelBack(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.ElectricSmelter, 4, 4)
	f.put(50, item.Stack{Kind: item.Parka, Qty: 1, X: 1, Y: 1})
	f.put(51, item.Stack{Kind: item.PlateArmor, Qty: 1, Stuff: item.Steel, X: 2, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.SmeltApparel, Mode: recipe.DoCount, Target: 1}})
	j := runBill(t, f, c, work.Mask(0).Set(work.Craft, true), 400)
	want := recipe.DefOf(recipe.MakePlateArmor).Input.Qty * recipe.SmeltPct / 100
	if j.State != job.Complete || f.Stock(item.Steel) != want || f.Stock(item.Parka) != 1 || f.Stock(item.PlateArmor) != 0 {
		t.Fatalf("job %v steel %d (want %d) parka %d plate %d", j.State, f.Stock(item.Steel), want, f.Stock(item.Parka), f.Stock(item.PlateArmor))
	}
}

// TestButcherDefaultsLeaveKibble: a new butcher table butchers by
// default but makes kibble only on a bill the player adds.
func TestButcherDefaultsLeaveKibble(t *testing.T) {
	t.Parallel()
	c := New(newFake())
	c.Bills.Defaults(1, construction.ButcherTable)
	if bills := c.Bills.Of(1); len(bills) != 1 || bills[0].Recipe != recipe.ButcherAnimal {
		t.Fatalf("butcher table defaults %+v", bills)
	}
	if Action(construction.FermentingBarrel) != ActionCook {
		t.Fatal("filling a barrel is a cook's job")
	}
}

// TestShredMechGivesSalvage: a wreck shredded at the machining table
// gives its mechanoid's steel and plasteel; a butcher skips it.
func TestShredMechGivesSalvage(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.MachiningTable, 4, 4)
	f.build(2, construction.ButcherTable, 8, 4)
	f.put(50, item.Stack{Kind: item.CorpseAnimal, Qty: 1, Animal: uint8(animal.Bulwark), Dried: true, X: 1, Y: 1})
	c.Bills.Defaults(2, construction.ButcherTable)
	if c.Shop.Assign(pawn, cookOnly) {
		t.Fatal("a butcher took a mechanoid wreck")
	}
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.ShredMech, Mode: recipe.DoCount, Target: 1}})
	j := runBill(t, f, c, work.Mask(0).Set(work.Craft, true), 100)
	if j.State != job.Complete || f.Stock(item.Steel) != 3 || f.Stock(item.Plasteel) != 1 || f.Stock(item.CorpseAnimal) != 0 {
		t.Fatalf("job %v steel %d plasteel %d wrecks %d", j.State, f.Stock(item.Steel), f.Stock(item.Plasteel), f.Stock(item.CorpseAnimal))
	}
}
