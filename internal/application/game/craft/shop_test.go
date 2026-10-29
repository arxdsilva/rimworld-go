package craft

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

const pawn entity.EntityID = 7

// runBill assigns pawn a bill with work types m and works it to the end
// (at most n ticks), returning the last job.
func runBill(t *testing.T, f *fakeHost, c *Craft, m work.Mask, n int) job.Record {
	t.Helper()
	if !c.Shop.Assign(pawn, m) {
		t.Fatal("no bill assigned")
	}
	for i := 0; i < n; i++ {
		j := f.jobs[pawn]
		if j.State == job.Complete || j.State == job.Fail {
			return j
		}
		c.Shop.Do(pawn, j)
	}
	return f.jobs[pawn]
}

func TestShopMakesAClub(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Workbench, 4, 4)
	f.build(2, construction.ToolCabinet, 5, 6)
	f.put(50, item.Stack{Kind: item.Wood, Qty: 10, X: 1, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club, Mode: recipe.DoCount, Target: 1}})
	if c.Shop.Assign(pawn, work.Mask(0).Set(work.Cook, true)) {
		t.Fatal("a cook took a crafting bill")
	}
	j := runBill(t, f, c, work.Mask(0).Set(work.Craft, true), 100)
	if j.State != job.Complete || f.Stock(item.Club) != 1 || f.Stock(item.Wood) != 6 {
		t.Fatalf("job %v clubs %d wood %d", j.State, f.Stock(item.Club), f.Stock(item.Wood))
	}
	if f.lastPct != 100+cabinetPct {
		t.Fatalf("bench pct %d", f.lastPct)
	}
	if f.claims[1] != pawn || c.Bills.Of(1)[0].Done != 1 {
		t.Fatalf("claim %v done %d", f.claims, c.Bills.Of(1)[0].Done)
	}
	if c.Shop.Assign(pawn, work.Mask(0).Set(work.Craft, true)) {
		t.Fatal("a done bill was assigned again")
	}
}

func TestShopSkipsBench(t *testing.T) {
	t.Parallel()
	craftOnly := work.Mask(0).Set(work.Craft, true)
	setup := func() (*fakeHost, *Craft) {
		f := newFake()
		c := New(f)
		f.pawns = []entity.EntityID{pawn}
		f.build(1, construction.Workbench, 4, 4)
		f.put(50, item.Stack{Kind: item.Wood, Qty: 10})
		c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club, Mode: recipe.Forever, Target: 1}})
		return f, c
	}
	f, c := setup()
	f.claims[1] = 99
	if c.Shop.Assign(pawn, craftOnly) {
		t.Fatal("took a bench reserved by another")
	}
	f, c = setup()
	f.noSkill = true
	if c.Shop.Assign(pawn, craftOnly) {
		t.Fatal("below the minimum skill")
	}
	f, c = setup()
	f.unreach = true
	if c.Shop.Assign(pawn, craftOnly) || !f.Postponed(pawn, 1) {
		t.Fatal("an unreachable bench is put off")
	}
	if c.Shop.Assign(pawn, craftOnly) {
		t.Fatal("a put-off bench was tried again")
	}
}

func TestShopCookingWaitsForFireAndPower(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Campfire, 4, 4)
	f.build(2, construction.ElectricStove, 8, 4)
	f.put(50, item.Stack{Kind: item.Berries, Qty: 10})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.CampfireMeal, Mode: recipe.Forever, Target: 1}})
	c.Bills.Set(2, []recipe.Bill{{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1}})
	f.unpower[2] = true
	cook := work.Mask(0).Set(work.Cook, true)
	if c.Shop.Assign(pawn, cook) {
		t.Fatal("cooked with no fire and no power")
	}
	f.unpower[2] = false
	j := runBill(t, f, c, cook, 1) // fetched, not yet worked
	if j.Dest != 2 {
		t.Fatalf("cooking at %d", j.Dest)
	}
	f.unpower[2] = true // the power goes while it cooks
	c.Shop.Do(pawn, f.jobs[pawn])
	if j := f.jobs[pawn]; j.State != job.Fail || j.Fail != job.Blocked {
		t.Fatalf("unpowered stove: %+v", j)
	}
	f.unpower[2] = false
	f.jobs = map[entity.EntityID]job.Record{}
	f.claims = map[entity.EntityID]entity.EntityID{}
	if j := runBill(t, f, c, cook, 200); j.State != job.Complete || f.meals != 1 || f.lastPct != 100+ElectricSpeedPct {
		t.Fatalf("meal %+v meals %d pct %d", j, f.meals, f.lastPct)
	}
}

// TestShopUntilHavePending: a second cook does not start the last meal
// another cook is already making.
func TestShopUntilHavePending(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn, pawn + 1}
	f.build(1, construction.Stove, 4, 4)
	f.put(50, item.Stack{Kind: item.Berries, Qty: 10})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.SimpleMeal, Mode: recipe.UntilHave, Target: 1}})
	cook := work.Mask(0).Set(work.Cook, true)
	if !c.Shop.Assign(pawn, cook) {
		t.Fatal("first cook")
	}
	delete(f.claims, 1)
	if c.Shop.Assign(pawn+1, cook) {
		t.Fatal("second cook started the same last meal")
	}
}

func TestShopDriesButchersAndCremates(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	cook := work.Mask(0).Set(work.Cook, true)
	f.build(1, construction.DryingRack, 4, 4)
	f.put(50, item.Stack{Kind: item.Meat, Qty: 5})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.DryFood, Mode: recipe.Forever, Target: 1}})
	if j := runBill(t, f, c, cook, 100); j.State != job.Complete || c.Drying.Batch(1).Qty != 5 || f.Stock(item.Meat) != 0 {
		t.Fatalf("dry %+v batch %+v", j, c.Drying.Batch(1))
	}
	f.jobs, f.claims = map[entity.EntityID]job.Record{}, map[entity.EntityID]entity.EntityID{}
	f.put(51, item.Stack{Kind: item.Meat, Qty: 5})
	if c.Shop.Assign(pawn, cook) {
		t.Fatal("a full rack took another batch")
	}

	f.build(2, construction.ButcherTable, 8, 4)
	f.put(52, item.Stack{Kind: item.CorpseAnimal, Qty: 1, Animal: 1})
	c.Bills.Set(2, []recipe.Bill{{Recipe: recipe.ButcherAnimal, Mode: recipe.Forever, Target: 1}})
	if j := runBill(t, f, c, cook, 400); j.State != job.Complete || f.Stock(item.CorpseAnimal) != 0 || f.Stock(item.Meat) <= 5 {
		t.Fatalf("butcher %+v meat %d", j, f.Stock(item.Meat))
	}

	f.jobs, f.claims = map[entity.EntityID]job.Record{}, map[entity.EntityID]entity.EntityID{}
	f.build(3, construction.Pyre, 12, 4)
	f.put(53, item.Stack{Kind: item.CorpseHuman, Qty: 1})
	c.Bills.Set(3, []recipe.Bill{{Recipe: recipe.Cremate, Mode: recipe.Forever, Target: 1}})
	crafter := work.Mask(0).Set(work.Craft, true)
	if c.Shop.Assign(pawn, crafter) {
		t.Fatal("cremated without wood")
	}
	f.put(54, item.Stack{Kind: item.Wood, Qty: 2})
	if j := runBill(t, f, c, crafter, 400); j.State != job.Complete || f.cremated != 1 {
		t.Fatalf("cremate %+v cremated %d", j, f.cremated)
	}
}

func TestShopFails(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.build(1, construction.Workbench, 4, 4)
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club, Mode: recipe.Forever, Target: 1}})
	c.Shop.Do(pawn, job.Record{Kind: job.Bill, Dest: 9})
	if j := f.jobs[pawn]; j.Fail != job.TargetGone {
		t.Fatalf("no bench: %+v", j)
	}
	c.Shop.Do(pawn, job.Record{Kind: job.Bill, Dest: 1, Target: 50})
	if j := f.jobs[pawn]; j.Fail != job.TargetGone {
		t.Fatalf("no ingredient: %+v", j)
	}
	f.put(50, item.Stack{Kind: item.Wood, Qty: 4})
	f.unreach = true
	c.Shop.Do(pawn, job.Record{Kind: job.Bill, Dest: 1, Target: 50})
	if j := f.jobs[pawn]; j.Fail != job.Unreachable || f.stacks[50].Carrier != pawn {
		t.Fatalf("unreachable: %+v", j)
	}
	f.work = -1 // not a working tick: nothing changes
	c.Shop.Do(pawn, job.Record{Kind: job.Bill, Dest: 1, Carry: 50, WorkLeft: 1000})
	if j := f.jobs[pawn]; j.WorkLeft != 1000 {
		t.Fatalf("no working tick: %+v", j)
	}
	f.work = 0
	c.Shop.Do(pawn, job.Record{Kind: job.Bill, Dest: 1, Carry: 50, WorkLeft: 1000})
	if j := f.jobs[pawn]; j.State == job.Complete || j.WorkLeft >= 1000 {
		t.Fatalf("one tick of work: %+v", j)
	}
}
