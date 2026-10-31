package craft

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// TestButcherYieldsTheBody: a juvenile cow's body gives half the wiki's
// 336 meat, as cow meat (FOOD-3).
func TestButcherYieldsTheBody(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(2, construction.ButcherTable, 8, 4)
	f.put(50, item.Stack{Kind: item.CorpseAnimal, Qty: 1, Animal: uint8(animal.Cow), Yield: 500, X: 1, Y: 1})
	c.Bills.Set(2, []recipe.Bill{{Recipe: recipe.ButcherAnimal, Mode: recipe.DoCount, Target: 1}})
	j := runBill(t, f, c, work.Mask(0).Set(work.Cook, true), 400)
	if j.State != job.Complete || f.Stock(item.Meat) != 168 || f.Stock(item.Leather) != 48 {
		t.Fatalf("job %v meat %d leather %d", j.State, f.Stock(item.Meat), f.Stock(item.Leather))
	}
	for _, d := range f.drops {
		if d.Kind == item.Meat && animal.MeatLabel(d) != "cow meat" {
			t.Fatalf("meat %+v is not cow meat", d)
		}
	}
}

// TestButcherHuman: a human body gives human meat and the butcher's mood
// call; the bill is never a default, and a rotten body is left alone.
func TestButcherHuman(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(2, construction.ButcherTable, 8, 4)
	c.Bills.Defaults(2, construction.ButcherTable)
	for _, b := range c.Bills.Of(2) {
		if b.Recipe == recipe.ButcherHuman {
			t.Fatal("butchering humans is never a default bill")
		}
	}
	rotten := item.Stack{Kind: item.CorpseHuman, Qty: 1, X: 1, Y: 1}
	rotten.Rot = rotten.Shelf() + 1
	f.put(50, rotten)
	c.Bills.Set(2, []recipe.Bill{{Recipe: recipe.ButcherHuman, Mode: recipe.DoCount, Target: 1}})
	if c.Shop.Assign(pawn, cookOnly) {
		t.Fatal("a rotten body is never butchered (wiki: Corpse)")
	}
	f.put(50, item.Stack{Kind: item.CorpseHuman, Qty: 1, X: 1, Y: 1})
	f.now += IngredientRetry + 1 // the failed search's cooldown
	j := runBill(t, f, c, work.Mask(0).Set(work.Cook, true), 400)
	if j.State != job.Complete || f.Stock(item.HumanMeat) != animal.HumanMeatYield || len(f.butchered) != 1 || f.butchered[0] != pawn {
		t.Fatalf("job %v human meat %d butchered %v", j.State, f.Stock(item.HumanMeat), f.butchered)
	}
}
