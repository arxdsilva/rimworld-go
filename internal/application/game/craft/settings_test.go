package craft

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

var crafting = work.Mask(0).Set(work.Craft, true)

// clubBench is a workbench at (4, 4) with one "make a club" bill.
func clubBench(t *testing.T, bl recipe.Bill) (*fakeHost, *Craft) {
	t.Helper()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.names[pawn] = "Ada"
	f.build(1, construction.Workbench, 4, 4)
	bl.Recipe, bl.Mode, bl.Target = recipe.Club, recipe.Forever, 1
	c.Bills.Set(1, []recipe.Bill{bl})
	return f, c
}

func cmdAt(k command.Kind, i int) command.Command {
	return command.Command{Kind: k, X: 4, Y: 4, Index: i}
}

func TestGatherWoodFromSeveralStacks(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.put(50, item.Stack{Kind: item.Wood, Qty: 2, X: 5, Y: 6})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 1, X: 9, Y: 9})
	f.put(52, item.Stack{Kind: item.Wood, Qty: 3, X: 6, Y: 6})
	if !c.Shop.Assign(pawn, crafting) {
		t.Fatal("no single stack holds 4 wood, but three together do")
	}
	j := f.jobs[pawn]
	if j.Kind != job.HaulIngredients || j.Count != 4 || j.Route[0] != 50 || j.Route[1] != 2 || j.Route[2] != 52 || j.Route[3] != 2 {
		t.Fatalf("gathering plan %+v", j)
	}
	if f.claims[1] != pawn || f.claims[52] != pawn { // the first stack: StartJob reserves the target
		t.Fatalf("bench and stacks reserved: %v", f.claims)
	}
	for n := 0; n < 50 && f.jobs[pawn].State != job.Complete && f.jobs[pawn].State != job.Fail; n++ {
		c.Shop.Do(pawn, f.jobs[pawn])
	}
	if j := f.jobs[pawn]; j.State != job.Complete || f.Stock(item.Club) != 1 || f.Stock(item.Wood) != 2 {
		t.Fatalf("job %v fail %v clubs %d wood %d", j.State, j.Fail, f.Stock(item.Club), f.Stock(item.Wood))
	}
	if st := f.stacks[51]; st.Qty != 1 || st.Carrier != 0 {
		t.Fatalf("the farthest stack was not needed: %+v", st)
	}
}

func TestGatherRespectsRadiusAndForbiddenKinds(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{Radius: 3})
	f.put(50, item.Stack{Kind: item.Wood, Qty: 10, X: 20, Y: 20})
	if c.Shop.Assign(pawn, crafting) {
		t.Fatal("took wood outside the radius")
	}
	f, c = clubBench(t, recipe.Bill{Radius: 3})
	f.put(50, item.Stack{Kind: item.Wood, Qty: 10, X: 20, Y: 20}) // the single-stack search finds this first
	f.put(51, item.Stack{Kind: item.Wood, Qty: 10, X: 5, Y: 6})
	if !c.Shop.Assign(pawn, crafting) || f.jobs[pawn].Kind != job.Bill || f.jobs[pawn].Target != 51 {
		t.Fatalf("the stack inside the radius: %+v", f.jobs[pawn])
	}

	// raw food for a meal: berries forbidden, so the meat
	f = newFake()
	c = New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Stove, 4, 4)
	f.put(50, item.Stack{Kind: item.Berries, Qty: 10, X: 5, Y: 6})
	f.put(51, item.Stack{Kind: item.Meat, Qty: 10, X: 9, Y: 9})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1}})
	forbid := cmdAt(command.SetBillIngredient, 0)
	forbid.Stuff = uint8(item.Berries)
	c.Bills.Apply(forbid)
	forbid.Stuff = uint8(item.Wood) // not an ingredient of a meal: ignored
	c.Bills.Apply(forbid)
	if bl := c.Bills.Of(1)[0]; len(bl.Forbid) != 1 || bl.Allows(item.Berries) {
		t.Fatalf("forbid list %v", bl.Forbid)
	}
	cook := work.Mask(0).Set(work.Cook, true)
	if !c.Shop.Assign(pawn, cook) || f.jobs[pawn].Target != 51 {
		t.Fatalf("cooked forbidden berries: %+v", f.jobs[pawn])
	}
	f.jobs[pawn] = job.Record{}
	f.claims = map[entity.EntityID]entity.EntityID{}
	forbid.Stuff, forbid.Enabled = uint8(item.Berries), true
	c.Bills.Apply(forbid)
	if bl := c.Bills.Of(1)[0]; bl.Forbid != nil {
		t.Fatalf("berries allowed again: %v", bl.Forbid)
	}
	if !c.Shop.Assign(pawn, cook) || f.jobs[pawn].Target != 50 {
		t.Fatalf("berries are allowed again: %+v", f.jobs[pawn])
	}
}

func TestBillSkillRangeAndWorker(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		bl   recipe.Bill
		lvl  uint8
		want bool
	}{
		{recipe.Bill{}, 0, true},
		{recipe.Bill{SkillMin: 12}, 10, false},
		{recipe.Bill{SkillMin: 10}, 10, true},
		{recipe.Bill{SkillMax: 5}, 10, false},
		{recipe.Bill{SkillMin: 3, SkillMax: 12}, 10, true},
		{recipe.Bill{Worker: 8}, 10, false},
		{recipe.Bill{Worker: pawn}, 10, true},
	} {
		f, c := clubBench(t, tc.bl)
		f.skills[pawn] = tc.lvl
		f.put(50, item.Stack{Kind: item.Wood, Qty: 10})
		if got := c.Shop.Assign(pawn, crafting); got != tc.want {
			t.Errorf("bill %+v level %d: assigned %v", tc.bl, tc.lvl, got)
		}
	}
}

// finishClub works a club bill with store mode st to the end and returns
// the store requests.
func finishClub(t *testing.T, st recipe.StoreMode, zone uint64) []storeCall {
	t.Helper()
	f, c := clubBench(t, recipe.Bill{Store: st, Zone: zone})
	f.put(50, item.Stack{Kind: item.Wood, Qty: 10})
	if j := runBill(t, f, c, crafting, 100); j.State != job.Complete {
		t.Fatalf("store %v: job %v", st, j.State)
	}
	return f.stored
}

func TestBillStoreModes(t *testing.T) {
	t.Parallel()
	if s := finishClub(t, recipe.BestStockpile, 0); len(s) != 1 || s[0] != (storeCall{pawn, item.Club, 4, 5, 0}) {
		t.Fatalf("best stockpile: %+v", s)
	}
	if s := finishClub(t, recipe.DropOnFloor, 0); len(s) != 0 {
		t.Fatalf("drop on floor: %+v", s)
	}
	if s := finishClub(t, recipe.StockpileZone, 9); len(s) != 1 || s[0].zone != 9 {
		t.Fatalf("specific stockpile: %+v", s)
	}
	if s := finishClub(t, recipe.StockpileZone, 0); len(s) != 0 {
		t.Fatalf("no stockpile chosen: %+v", s)
	}
	// a cremation leaves nothing to store
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Pyre, 4, 4)
	f.put(50, item.Stack{Kind: item.CorpseHuman, Qty: 1})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 10})
	c.Bills.Defaults(1, construction.Pyre)
	if j := runBill(t, f, c, crafting, 1000); j.State != job.Complete || len(f.stored) != 0 {
		t.Fatalf("cremation: %v %+v", j.State, f.stored)
	}
}

func TestConfigureBillClampsAndResetsTheRetry(t *testing.T) {
	t.Parallel()
	_, c := clubBench(t, recipe.Bill{SearchAt: 900})
	cfg := cmdAt(command.ConfigureBill, 0)
	cfg.Store, cfg.Zone, cfg.Radius, cfg.SkillMin, cfg.SkillMax, cfg.Worker, cfg.UnpauseAt = 7, 3, -4, 30, 12, 8, -1
	c.Bills.Apply(cfg)
	bl := c.Bills.Of(1)[0]
	if bl.Store != recipe.BestStockpile || bl.Zone != 3 || bl.Radius != 0 || bl.SkillMin != 12 || bl.SkillMax != 12 ||
		bl.Worker != 8 || bl.UnpauseAt != 0 || bl.SearchAt != 0 {
		t.Fatalf("configured %+v", bl)
	}
	cfg.Store, cfg.SkillMin, cfg.SkillMax, cfg.Radius, cfg.UnpauseAt = uint8(recipe.DropOnFloor), 4, 0, 6, 2
	c.Bills.Apply(cfg)
	if bl = c.Bills.Of(1)[0]; bl.Store != recipe.DropOnFloor || bl.SkillMin != 4 || bl.MaxSkill() != 20 || bl.Radius != 6 || bl.UnpauseAt != 2 {
		t.Fatalf("configured %+v", bl)
	}
	bad := cmdAt(command.ConfigureBill, 3)
	c.Bills.Apply(bad) // no bill 3: ignored
}

func TestReorderBillMovesBillsAndTheirJobs(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club}, {Recipe: recipe.Spear}, {Recipe: recipe.Knife}})
	f.pawns = []entity.EntityID{pawn, 8, 9}
	f.jobs[pawn] = job.Record{Kind: job.Bill, Dest: 1, BillIndex: 2}
	f.jobs[8] = job.Record{Kind: job.HaulIngredients, Dest: 1, BillIndex: 0}
	f.jobs[9] = job.Record{Kind: job.Bill, Dest: 2, BillIndex: 0} // another bench
	up := cmdAt(command.ReorderBill, 2)
	up.Count = -2
	c.Bills.Apply(up)
	order := func() []recipe.ID {
		var out []recipe.ID
		for _, bl := range c.Bills.Of(1) {
			out = append(out, bl.Recipe)
		}
		return out
	}
	if o := order(); o[0] != recipe.Knife || o[1] != recipe.Club || o[2] != recipe.Spear {
		t.Fatalf("order %v", o)
	}
	if f.jobs[pawn].BillIndex != 0 || f.jobs[8].BillIndex != 1 || f.jobs[9].BillIndex != 0 {
		t.Fatalf("jobs %d %d %d", f.jobs[pawn].BillIndex, f.jobs[8].BillIndex, f.jobs[9].BillIndex)
	}
	down := cmdAt(command.ReorderBill, 0)
	down.Count = 9 // clamped to the last place
	c.Bills.Apply(down)
	if o := order(); o[0] != recipe.Club || o[1] != recipe.Spear || o[2] != recipe.Knife || f.jobs[pawn].BillIndex != 2 {
		t.Fatalf("order %v job %d", o, f.jobs[pawn].BillIndex)
	}
	still := cmdAt(command.ReorderBill, 1) // Count 0: nothing moves
	c.Bills.Apply(still)
	if o := order(); o[1] != recipe.Spear {
		t.Fatalf("order %v", o)
	}
}

func TestUntilHaveResumesAtUnpauseAt(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Workbench, 4, 4)
	f.put(50, item.Stack{Kind: item.Wood, Qty: 100})
	f.put(60, item.Stack{Kind: item.Club, Qty: 10, X: 9, Y: 9})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club, Mode: recipe.UntilHave, Target: 10, UnpauseAt: 4}})
	if c.Shop.Assign(pawn, crafting) {
		t.Fatal("at the target")
	}
	f.stacks[60] = item.Stack{Kind: item.Club, Qty: 5, X: 9, Y: 9} // the default would resume at 8
	if c.Shop.Assign(pawn, crafting) {
		t.Fatal("resumed above the unpause level")
	}
	f.stacks[60] = item.Stack{Kind: item.Club, Qty: 4, X: 9, Y: 9}
	if !c.Shop.Assign(pawn, crafting) {
		t.Fatal("did not resume at the unpause level")
	}
}

// TestNoIngredientsRetriesAfterACooldown is the no-flap test: a bill with
// nothing to gather searches once, then rests IngredientRetry ticks
// however often it is asked, and a reconfigured bill searches at once.
func TestNoIngredientsRetriesAfterACooldown(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.now = 1000
	for i := 0; i < IngredientRetry; i++ {
		if c.Shop.Assign(pawn, crafting) {
			t.Fatal("nothing to make a club from")
		}
		f.now++
		if i == 10 {
			f.put(50, item.Stack{Kind: item.Wood, Qty: 10}) // wood arrives during the cooldown
		}
	}
	if f.scans != 1 {
		t.Fatalf("searched %d times during one cooldown", f.scans)
	}
	if f.jobs[pawn].Kind != job.None {
		t.Fatal("started during the cooldown")
	}
	if !c.Shop.Assign(pawn, crafting) {
		t.Fatal("the cooldown is over and the wood is there")
	}
	f.jobs[pawn] = job.Record{}
	f.claims = map[entity.EntityID]entity.EntityID{}
	f.DeleteItem(50)
	c.Shop.Assign(pawn, crafting)
	scans := f.scans
	c.Bills.Apply(cmdAt(command.ConfigureBill, 0))
	f.put(51, item.Stack{Kind: item.Wood, Qty: 10})
	if !c.Shop.Assign(pawn, crafting) || f.scans != scans {
		t.Fatalf("a reconfigured bill searches again at once (scans %d → %d)", scans, f.scans)
	}
}

func TestGatherSeveralIngredientsOntoTheBench(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.put(50, item.Stack{Kind: item.Steel, Qty: 5, X: 5, Y: 7})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 3, X: 5, Y: 6})
	f.put(52, item.Stack{Kind: item.Wood, Qty: 3, X: 6, Y: 6})
	d := recipe.DefOf(recipe.Club)
	d.Inputs = []recipe.Input{{Kind: item.Wood, Qty: 4}, {Kind: item.Steel, Qty: 2}}
	bl := c.Bills.Of(1)[0]
	rec, ok := c.Shop.plan(pawn, bl, d, [2]int{4, 5})
	if !ok || rec.Kind != job.HaulIngredients || rec.Count != 6 {
		t.Fatalf("plan %+v", rec)
	}
	rec.Dest, rec.DestX, rec.DestY, rec.WorkLeft = 1, 4, 5, d.Work
	if !c.Shop.start(pawn, rec) {
		t.Fatal("start")
	}
	var laid bool
	for n := 0; n < 50 && f.jobs[pawn].State != job.Complete && f.jobs[pawn].State != job.Fail; n++ {
		c.Shop.Do(pawn, f.jobs[pawn])
		if j := f.jobs[pawn]; j.Kind == job.Bill && len(j.Route) == 1 {
			laid = true
			if st := f.stacks[j.Route[0]]; st.Kind != item.Wood || st.Qty != 4 || st.X != 4 || st.Y != 4 || f.claims[j.Route[0]] != pawn {
				t.Fatalf("wood laid on the bench: %+v", st)
			}
		}
	}
	if !laid || f.jobs[pawn].State != job.Complete {
		t.Fatalf("laid %v job %+v", laid, f.jobs[pawn])
	}
	if f.Stock(item.Wood) != 2 || f.Stock(item.Steel) != 3 || f.Stock(item.Club) != 1 {
		t.Fatalf("wood %d steel %d clubs %d", f.Stock(item.Wood), f.Stock(item.Steel), f.Stock(item.Club))
	}
	// one ingredient short: no plan
	f.put(53, item.Stack{Kind: item.Wood, Qty: 9})
	f.DeleteItem(50)
	if _, ok := c.Shop.plan(pawn, bl, d, [2]int{4, 5}); ok {
		t.Fatal("planned without steel")
	}
}

func TestGatherFailures(t *testing.T) {
	t.Parallel()
	setup := func() (*fakeHost, *Craft) {
		f, c := clubBench(t, recipe.Bill{})
		f.put(50, item.Stack{Kind: item.Wood, Qty: 2, X: 5, Y: 6})
		f.put(51, item.Stack{Kind: item.Wood, Qty: 2, X: 6, Y: 6})
		if !c.Shop.Assign(pawn, crafting) {
			t.Fatal("no gathering")
		}
		return f, c
	}
	f, c := setup()
	f.DeleteItem(50) // gone before it is picked up
	c.Shop.Do(pawn, f.jobs[pawn])
	if j := f.jobs[pawn]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("job %+v", j)
	}
	f, c = setup()
	c.Shop.Do(pawn, f.jobs[pawn]) // picks the first stack up
	f.DeleteItem(51)
	c.Shop.Do(pawn, f.jobs[pawn])
	if j := f.jobs[pawn]; j.State != job.Fail {
		t.Fatalf("second stack gone: %+v", j)
	}
	f, c = setup()
	f.unreach = true
	c.Shop.Do(pawn, f.jobs[pawn])
	if j := f.jobs[pawn]; j.State != job.Fail || j.Fail != job.Unreachable || !f.Postponed(pawn, 1) {
		t.Fatalf("unreachable: %+v", j)
	}
	// an ingredient taken off the bench before the work ends
	f, c = setup()
	j := f.jobs[pawn]
	j.Kind, j.Carry, j.Route, j.Count, j.WorkLeft = job.Bill, 50, []entity.EntityID{77}, 0, 1
	c.Shop.Do(pawn, j)
	if j := f.jobs[pawn]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("missing bench ingredient: %+v", j)
	}
}

func TestBillViews(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn, 8}
	f.names[pawn], f.names[8] = "Ada", "Bo"
	f.build(1, construction.Stove, 4, 4)
	f.put(50, item.Stack{Kind: item.Berries, Qty: 10})
	c.Bills.Set(1, []recipe.Bill{
		{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1, Worker: 8, Store: recipe.StockpileZone, Zone: 4, Radius: 6},
		{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1, Worker: 99},
		{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1, SkillMin: 15},
		{Recipe: recipe.SimpleMeal, Mode: recipe.Forever, Target: 1, Worker: 8, SkillMax: 4},
	})
	forbid := cmdAt(command.SetBillIngredient, 0)
	forbid.Stuff = uint8(item.Meat)
	c.Bills.Apply(forbid)
	f.jobs[8] = job.Record{Kind: job.Bill, State: job.Do, Dest: 1, BillIndex: 0, Carry: 50, WorkLeft: 30}
	f.jobs[pawn] = job.Record{Kind: job.HaulIngredients, State: job.Goto, Dest: 1, BillIndex: 2}
	vs := c.Bills.AppendViews(nil, 1)
	if len(vs) != 4 {
		t.Fatalf("%d views", len(vs))
	}
	v := vs[0]
	if v.Worker != 8 || v.WorkerName != "Bo" || v.Store != uint8(recipe.StockpileZone) || v.StoreZone != 4 ||
		v.StoreLabel != "specific stockpile" || v.Radius != 6 || v.SkillMax != 20 || v.Inputs != "2 raw food" || v.NoWorker != "" {
		t.Fatalf("view %+v", v)
	}
	if v.CurrentWorker != "Bo" || v.Gathering || v.Progress != 75 {
		t.Fatalf("current %q gathering %v progress %d", v.CurrentWorker, v.Gathering, v.Progress)
	}
	meat, berries := false, false
	for i := 0; i < v.Ingredients.Len(); i++ {
		in := v.Ingredients.At(i)
		switch item.Kind(in.Kind) {
		case item.Meat:
			meat = !in.Allowed
		case item.Berries:
			berries = in.Allowed && in.Label == "berries"
		}
	}
	if l := NewBillIngredients(BillIngredientView{Kind: uint8(item.Wood), Allowed: true}, BillIngredientView{Kind: uint8(item.Steel)}); l.Len() != 2 ||
		l.At(0) != (BillIngredientView{Kind: uint8(item.Wood), Label: "wood", Allowed: true}) || l.At(1).Allowed {
		t.Fatalf("built list %+v", l)
	}
	if !meat || !berries {
		t.Fatalf("ingredients %+v", v.Ingredients)
	}
	if vs[1].NoWorker != NoWorkerGone || vs[2].NoWorker != NoWorkerNone || vs[3].NoWorker != NoWorkerSkill {
		t.Fatalf("reasons %q %q %q", vs[1].NoWorker, vs[2].NoWorker, vs[3].NoWorker)
	}
	if vs[2].CurrentWorker != "Ada" || !vs[2].Gathering {
		t.Fatalf("gathering view %+v", vs[2])
	}
	f.skills[8] = 16
	if vs = c.Bills.AppendViews(nil, 1); vs[2].NoWorker != "" {
		t.Fatalf("Bo has the skill now: %q", vs[2].NoWorker)
	}
	if rv := Recipes(); len(rv) == 0 || rv[0].Input != "2 raw food" {
		t.Fatalf("recipes %+v", rv)
	}
}

func TestIngredientSearchEdges(t *testing.T) {
	t.Parallel()
	// the read model sees a club's wood across stacks, and none when the
	// stacks are forbidden, reserved by another or out of the radius
	f, c := clubBench(t, recipe.Bill{})
	f.put(50, item.Stack{Kind: item.Wood, Qty: 2, X: 5, Y: 6})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 2, X: 6, Y: 6, Forbidden: true})
	f.put(52, item.Stack{Kind: item.Wood, Qty: 1, X: 6, Y: 7})
	if _, st := c.Bills.Line(1, 0); st != BillNoInput {
		t.Fatalf("3 usable wood of 4: %q", st)
	}
	f.stacks[51] = item.Stack{Kind: item.Wood, Qty: 2, X: 6, Y: 6}
	if _, st := c.Bills.Line(1, 0); st != BillActive {
		t.Fatalf("5 wood in three stacks: %q", st)
	}
	f.claims[51] = 99
	if _, st := c.Bills.Line(1, 0); st != BillNoInput {
		t.Fatalf("a reserved stack counted: %q", st)
	}
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club, Mode: recipe.Forever, Target: 1, Radius: 1}})
	delete(f.claims, 51)
	if _, st := c.Bills.Line(1, 0); st != BillNoInput {
		t.Fatalf("stacks outside the radius counted: %q", st)
	}

	// a drying rack takes one stack up to its capacity, never two
	f = newFake()
	c = New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.DryingRack, 4, 4)
	f.put(50, item.Stack{Kind: item.Meat, Qty: 30, X: 20, Y: 20})
	f.put(51, item.Stack{Kind: item.Berries, Qty: 6, X: 5, Y: 6})
	f.put(52, item.Stack{Kind: item.Meat, Qty: 8, X: 5, Y: 6, Rot: 1 << 30}) // spoilt
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.DryFood, Mode: recipe.Forever, Target: 1, Radius: 4}})
	if _, st := c.Bills.Line(1, 0); st != BillActive {
		t.Fatalf("berries in reach: %q", st)
	}
	if !c.Shop.Assign(pawn, work.Mask(0).Set(work.Cook, true)) || f.jobs[pawn].Kind != job.Bill || f.jobs[pawn].Target != 51 {
		t.Fatalf("dry the berries in reach: %+v", f.jobs[pawn])
	}

	// two inputs taking the same kind fold onto one stack; one short fails
	d := recipe.DefOf(recipe.Club)
	d.Inputs = []recipe.Input{{Kind: item.Wood, Qty: 2}, {Kind: item.Wood, Qty: 1}}
	f, c = clubBench(t, recipe.Bill{})
	f.put(50, item.Stack{Kind: item.Wood, Qty: 5})
	picks, ok := c.Shop.p.plan(pawn, c.Bills.Of(1)[0], d, [2]int{4, 5}, &c.Shop.buf)
	if !ok || len(picks) != 1 || picks[0].take != 3 {
		t.Fatalf("folded picks %+v", picks)
	}
	if !c.Shop.p.available(c.Bills.Of(1)[0], d, [2]int{4, 5}) {
		t.Fatal("multi-input availability")
	}
	d.Inputs = append(d.Inputs, recipe.Input{AnyDryable: true, Qty: 1, Max: 9})
	if _, ok := c.Shop.p.plan(pawn, c.Bills.Of(1)[0], d, [2]int{4, 5}, &c.Shop.buf); ok {
		t.Fatal("planned a missing dryable input")
	}

	// a gathering whose start finds its first stack unreachable puts the bench off
	f, c = clubBench(t, recipe.Bill{})
	f.put(50, item.Stack{Kind: item.Wood, Qty: 2})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 2})
	f.startFail = job.Unreachable
	if c.Shop.Assign(pawn, crafting) || !f.Postponed(pawn, 1) {
		t.Fatal("unreachable gathering not put off")
	}

	var l BillIngredients
	for k := 0; k < maxIngredients+3; k++ {
		l.add(item.Kind(k), k%2 == 0)
	}
	if l.Len() != maxIngredients || !l.At(0).Allowed || l.At(1).Allowed {
		t.Fatalf("list %d", l.Len())
	}
	if movedIndex(3, 1, 4) != 2 || movedIndex(5, 1, 4) != 5 || movedIndex(1, 3, 1) != 2 {
		t.Fatal("moved index")
	}
	if c.Bills.spot(999) != ([2]int{}) {
		t.Fatal("a gone bench has no spot")
	}
}
