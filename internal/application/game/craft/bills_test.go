package craft

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func billCmd(k command.Kind, x, y int, r recipe.ID, mode recipe.Mode, count, index int) command.Command {
	return command.Command{Kind: k, X: x, Y: y, Recipe: uint8(r), Mode: uint8(mode), Count: count, Index: index}
}

func TestApplyBills(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.build(1, construction.Workbench, 4, 4)
	c.Bills.Apply(billCmd(command.AddBill, 4, 4, recipe.Club, recipe.DoCount, 0, 0))
	c.Bills.Apply(billCmd(command.AddBill, 4, 4, recipe.Spear, recipe.Forever, 3, 0))
	c.Bills.Apply(billCmd(command.AddBill, 4, 4, recipe.SimpleMeal, recipe.Forever, 1, 0)) // a stove's recipe
	f.locked[recipe.Knife] = true
	c.Bills.Apply(billCmd(command.AddBill, 4, 4, recipe.Knife, recipe.Forever, 1, 0)) // not researched
	c.Bills.Apply(billCmd(command.AddBill, 9, 9, recipe.Club, recipe.Forever, 1, 0))  // no bench there
	bills := c.Bills.Of(1)
	if len(bills) != 2 || bills[0].Recipe != recipe.Club || bills[0].Target != 1 || bills[1].Target != 3 {
		t.Fatalf("bills %+v", bills)
	}
	set := billCmd(command.SetBill, 4, 4, 0, recipe.UntilHave, 5, 1)
	set.Enabled = true
	c.Bills.Apply(set)
	c.Bills.Apply(billCmd(command.SetBill, 4, 4, 0, recipe.Forever, 0, 7)) // out of range
	if bl := c.Bills.Of(1)[1]; bl.Mode != recipe.UntilHave || bl.Target != 5 || !bl.Suspended {
		t.Fatalf("set bill %+v", bl)
	}
	c.Bills.Apply(billCmd(command.RemoveBill, 4, 4, 0, 0, 0, 0))
	c.Bills.Apply(billCmd(command.RemoveBill, 4, 4, 0, 0, 0, 5))
	if bills := c.Bills.Of(1); len(bills) != 1 || bills[0].Recipe != recipe.Spear {
		t.Fatalf("after remove %+v", bills)
	}
	c.Bills.Forget(1)
	if len(c.Bills.Of(1)) != 0 {
		t.Fatal("forgotten bench keeps bills")
	}
}

func TestDefaultBills(t *testing.T) {
	t.Parallel()
	c := New(newFake())
	c.Bills.Defaults(1, construction.Pyre)
	c.Bills.Defaults(2, construction.Campfire)
	c.Bills.Defaults(3, construction.ButcherTable)
	c.Bills.Defaults(4, construction.DryingRack)
	c.Bills.Defaults(5, construction.Workbench)
	if b := c.Bills.Of(1); len(b) != 1 || b[0].Recipe != recipe.Cremate || b[0].Mode != recipe.Forever {
		t.Fatalf("pyre %+v", b)
	}
	if b := c.Bills.Of(2); len(b) != 1 || b[0].Mode != recipe.UntilHave || b[0].Target != 10 {
		t.Fatalf("campfire %+v", b)
	}
	if len(c.Bills.Of(3)) == 0 || len(c.Bills.Of(4)) == 0 || len(c.Bills.Of(5)) != 0 {
		t.Fatalf("butcher %v rack %v workbench %v", c.Bills.Of(3), c.Bills.Of(4), c.Bills.Of(5))
	}
	if !c.Bills.AnyFor(work.Cook) || c.Bills.AnyFor(work.Doctor) {
		t.Fatal("AnyFor")
	}
}

func TestUpgradeLegacyBills(t *testing.T) {
	t.Parallel()
	c := New(newFake())
	c.Bills.Set(1, []recipe.Bill{
		{Recipe: recipe.ButcherDustbuck, Mode: recipe.DoCount, Target: 4},
		{Recipe: recipe.ButcherRidgewolf, Mode: recipe.Forever, Target: 1},
		{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1},
	})
	c.Bills.Set(2, []recipe.Bill{{Recipe: recipe.Club, Target: 1}})
	c.Bills.UpgradeLegacy()
	if b := c.Bills.Of(1); len(b) != 2 || b[0].Recipe != recipe.ButcherAnimal || b[0].Target != 4 || b[1].Recipe != recipe.SimpleMeal {
		t.Fatalf("upgraded %+v", b)
	}
	if b := c.Bills.Of(2); len(b) != 1 || b[0].Recipe != recipe.Club {
		t.Fatalf("untouched %+v", b)
	}
}

func TestBillsSaveRoundTrip(t *testing.T) {
	t.Parallel()
	c := New(newFake())
	c.Bills.Defaults(1, construction.Pyre)
	c.Drying.Set(2, Batch{Kind: item.Meat, Qty: 3, Left: 9})
	bills, racks := c.Bills.Export(), c.Drying.Export()
	d := New(newFake())
	d.Bills.Import(nil) // an older save: kept
	d.Drying.Import(nil)
	d.Bills.Import(bills)
	d.Drying.Import(racks)
	if len(d.Bills.Of(1)) != 1 || d.Drying.Batch(2).Qty != 3 {
		t.Fatalf("round trip: %v %+v", d.Bills.Of(1), d.Drying.Batch(2))
	}
}

func TestBillLineStatus(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.build(1, construction.Workbench, 2, 2)
	f.build(2, construction.Campfire, 6, 2)
	f.build(3, construction.Pyre, 10, 2)
	f.build(4, construction.ElectricStove, 14, 2)
	f.build(5, construction.DryingRack, 18, 2)
	status := func(bid entity.EntityID, bl recipe.Bill) string {
		c.Bills.Set(bid, []recipe.Bill{bl})
		_, st := c.Bills.Line(bid, 0)
		return st
	}
	cases := []struct {
		name string
		bid  entity.EntityID
		bl   recipe.Bill
		want string
	}{
		{"suspended", 1, recipe.Bill{Recipe: recipe.Club, Mode: recipe.Forever, Target: 1, Suspended: true}, BillSuspended},
		{"done", 1, recipe.Bill{Recipe: recipe.Club, Mode: recipe.DoCount, Target: 1, Done: 1}, BillDone},
		{"no input", 1, recipe.Bill{Recipe: recipe.Club, Mode: recipe.Forever, Target: 1}, BillNoInput},
		{"no fire", 2, recipe.Bill{Recipe: recipe.CampfireMeal, Mode: recipe.Forever, Target: 1}, BillNoFuel},
		{"no wood", 3, recipe.Bill{Recipe: recipe.Cremate, Mode: recipe.Forever, Target: 1}, BillNoFuel},
		{"drying", 5, recipe.Bill{Recipe: recipe.DryFood, Mode: recipe.Forever, Target: 1}, BillDrying},
	}
	c.Drying.Set(5, Batch{Kind: item.Meat, Qty: 2, Left: 5})
	for _, tc := range cases {
		if got := status(tc.bid, tc.bl); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	f.unpower[4] = true
	if got := status(4, recipe.Bill{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1}); got != BillNoPower {
		t.Errorf("unpowered: %q", got)
	}
	f.put(50, item.Stack{Kind: item.Wood, Qty: 4})
	if got := status(1, recipe.Bill{Recipe: recipe.Club, Mode: recipe.Forever, Target: 1}); got != BillActive {
		t.Errorf("active: %q", got)
	}
	f.put(51, item.Stack{Kind: item.Club, Qty: 2})
	if got := status(1, recipe.Bill{Recipe: recipe.Club, Mode: recipe.UntilHave, Target: 2}); got != BillHaveEnough {
		t.Errorf("have enough: %q", got)
	}
}
