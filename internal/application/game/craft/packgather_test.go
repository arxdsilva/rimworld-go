package craft

import (
	"cmp"
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// woodBench is a club bill (4 wood) with wood in three small stacks.
func woodBench(t *testing.T, rules ...gameplay.Toggle) (*fakeHost, *Craft) {
	t.Helper()
	f, c := clubBench(t, recipe.Bill{})
	f.on(rules...)
	f.put(50, item.Stack{Kind: item.Wood, Qty: 2, X: 5, Y: 6})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 3, X: 9, Y: 9})
	f.put(52, item.Stack{Kind: item.Wood, Qty: 1, X: 6, Y: 6})
	return f, c
}

// walkBill plays pawn's job on the fake until it ends or turns plain, arriving at
// once wherever it walks, and returns how many times it stood at the bench.
func walkBill(t *testing.T, f *fakeHost, c *Craft) (trips int) {
	t.Helper()
	for range 100 {
		j := f.jobs[pawn]
		if j.State == job.Complete || j.State == job.Fail || j.Kind == job.Bill || j.Kind == job.CleanRoom {
			return trips
		}
		if j.Target == j.Dest {
			trips++
		}
		j.State, j.Path, j.PathI = job.Do, nil, 0
		f.jobs[pawn] = j
		c.Shop.Do(pawn, j)
	}
	t.Fatalf("job never settled: %+v", f.jobs[pawn])
	return trips
}

func TestPackGatherOneTrip(t *testing.T) {
	t.Parallel()
	f, c := woodBench(t, gameplay.HaulAllIngredients)
	if !c.Shop.Assign(pawn, crafting) {
		t.Fatal("no bill")
	}
	if j := f.jobs[pawn]; j.Kind != job.PackIngredients {
		t.Fatalf("job %+v", j)
	}
	if trips := walkBill(t, f, c); trips != 1 {
		t.Fatalf("%d trips to the bench, want the one", trips)
	}
	j := f.jobs[pawn]
	if j.Kind != job.Bill || j.Carry == 0 || j.SpareX != 0 || j.Count != 0 {
		t.Fatalf("the plain bill: %+v", j)
	}
	if st := f.stacks[j.Carry]; st.Qty != 4 || st.Carrier != pawn {
		t.Fatalf("one load, four wood, in hand: %+v", st)
	}
	if len(f.pack(pawn).Items) != 0 {
		t.Fatalf("left in the pack: %+v", f.pack(pawn))
	}
	if f.inPlay(item.Wood) != 6 {
		t.Fatalf("%d wood", f.inPlay(item.Wood))
	}
	// then it works it off, the wood gone and a club made
	j.WorkLeft = 1
	f.jobs[pawn] = j
	c.Shop.Do(pawn, j)
	if j := f.jobs[pawn]; j.State != job.Complete || f.inPlay(item.Wood) != 2 || f.Stock(item.Club) != 1 {
		t.Fatalf("job %+v, wood %d, clubs %d", j, f.inPlay(item.Wood), f.Stock(item.Club))
	}
}

func TestPackGatherInterruptedKeepsPack(t *testing.T) {
	t.Parallel()
	f, c := woodBench(t, gameplay.HaulAllIngredients)
	c.Shop.Assign(pawn, crafting)
	j := f.jobs[pawn]
	j.State = job.Do
	c.Shop.Do(pawn, j) // the first stack stowed; the job then ends under it
	f.jobs[pawn] = job.Record{}
	f.releaseOwner(pawn)
	p := f.pack(pawn)
	if len(p.Items) != 1 || !p.Items[0].Unload {
		t.Fatalf("CSE-040: a leftover stays marked: %+v", p)
	}
	if f.inPlay(item.Wood) != 6 {
		t.Fatalf("%d wood", f.inPlay(item.Wood))
	}
	for s, o := range f.claims {
		if o == pawn {
			t.Fatalf("claim on %d kept", s)
		}
	}
}

func TestPackGatherFailsWhenTargetGone(t *testing.T) {
	t.Parallel()
	for _, gone := range []string{"at a stack", "unpacking"} {
		f, c := woodBench(t, gameplay.HaulAllIngredients)
		c.Shop.Assign(pawn, crafting)
		j := f.jobs[pawn]
		if gone == "at a stack" {
			f.DeleteItem(j.Target)
		} else {
			j.State = job.Do
			c.Shop.Do(pawn, j)
			f.pack(pawn).Items = nil // rotted away
		}
		walkBill(t, f, c)
		if j := f.jobs[pawn]; j.State != job.Fail || j.Fail != job.TargetGone || j.Carry != 0 {
			t.Fatalf("%s: %+v", gone, j)
		}
	}
}

func TestPackGatherStackTakenFails(t *testing.T) {
	t.Parallel()
	f, c := woodBench(t, gameplay.HaulAllIngredients)
	c.Shop.Assign(pawn, crafting)
	j := f.jobs[pawn]
	st := f.stacks[j.Target]
	st.Carrier = 99
	f.stacks[j.Target] = st
	j.State = job.Do
	c.Shop.Do(pawn, j)
	if j := f.jobs[pawn]; j.State != job.Fail || f.inPlay(item.Wood) != 6 || f.stowCalls != 0 {
		t.Fatalf("job %+v, wood %d, stows %d", j, f.inPlay(item.Wood), f.stowCalls)
	}
}

func TestPackGatherSplitsByCapacity(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.on(gameplay.HaulAllIngredients)
	f.room = item.Wood.MassGrams() * 2 // two logs a trip
	f.put(50, item.Stack{Kind: item.Wood, Qty: 3, X: 5, Y: 6})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 3, X: 9, Y: 9})
	d := recipe.DefOf(recipe.Club)
	bl := c.Bills.Of(1)[0]
	rec, ok := c.Shop.plan(pawn, bl, d, [2]int{4, 5})
	if !ok || rec.Kind != job.PackIngredients {
		// one stack cannot hold the bill's four: the search takes both
		t.Fatalf("plan %+v", rec)
	}
	rec.Dest, rec.DestX, rec.DestY, rec.WorkLeft = 1, 4, 5, d.Work
	if !c.Shop.start(pawn, rec) {
		t.Fatal("start")
	}
	seen := 0
	for range 50 {
		j := f.jobs[pawn]
		if j.Kind != job.PackIngredients {
			break
		}
		if g := f.pack(pawn).Grams(); g > f.room {
			t.Fatalf("%d grams in a pack of %d", g, f.room)
		}
		if j.Target == j.Dest {
			seen++
		}
		j.State, j.Path, j.PathI = job.Do, nil, 0
		f.jobs[pawn] = j
		c.Shop.Do(pawn, j)
		if n := f.inPlay(item.Wood); n != 6 {
			t.Fatalf("%d wood in play", n)
		}
	}
	j := f.jobs[pawn]
	if seen != 2 || j.Kind != job.Bill {
		t.Fatalf("%d trips, job %+v", seen, j)
	}
	total := f.stacks[j.Carry].Qty
	for _, rid := range j.Route {
		total += f.stacks[rid].Qty
	}
	if total != 4 {
		t.Fatalf("%d wood at the bench (route %v)", total, j.Route)
	}
}

// A pawn whose hands fail between trips stops, rather than standing at the bench
// for good, and the next search takes it by hand: no flap.
func TestPackGatherNoRoomStops(t *testing.T) {
	t.Parallel()
	f, c := woodBench(t, gameplay.HaulAllIngredients)
	c.Shop.Assign(pawn, crafting)
	f.room = 1
	walkBill(t, f, c)
	if j := f.jobs[pawn]; j.State != job.Fail || j.Fail != job.Blocked {
		t.Fatalf("job %+v", j)
	}
	if f.inPlay(item.Wood) != 6 {
		t.Fatalf("%d wood", f.inPlay(item.Wood))
	}
	f.jobs[pawn] = job.Record{}
	f.releaseOwner(pawn)
	if !c.Shop.Assign(pawn, crafting) || f.jobs[pawn].Kind != job.HaulIngredients {
		t.Fatalf("the hand path: %+v", f.jobs[pawn])
	}
}

func TestPackOnlyForColonistsAndWhatFits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		set  func(*fakeHost)
	}{
		{"rule off", func(f *fakeHost) { f.rules[gameplay.HaulAllIngredients] = false }},
		{"no colonist", func(f *fakeHost) { delete(f.names, pawn) }},
		{"no room for one", func(f *fakeHost) { f.room = 1 }},
	}
	for _, tc := range cases {
		f, c := woodBench(t, gameplay.HaulAllIngredients)
		tc.set(f)
		c.Shop.Assign(pawn, crafting)
		if j := f.jobs[pawn]; j.Kind != job.HaulIngredients {
			t.Fatalf("%s: job %+v", tc.name, j)
		}
	}
	f := newFake()
	s := &Shop{h: f}
	if s.packable(pawn, []pick{{kind: item.CorpseHuman}}) {
		t.Fatal("no pack takes a corpse")
	}
}

// A reordered or removed bill is followed while the pack gathering walks.
func TestPackGatherFollowsItsBill(t *testing.T) {
	t.Parallel()
	bills := []recipe.Bill{{Recipe: recipe.Club}, {Recipe: recipe.Cremate}}
	if i := billOf(job.Record{BillIndex: 0, SpareX: int(recipe.Club)}, bills); i != 0 {
		t.Fatalf("in place: %d", i)
	}
	if i := billOf(job.Record{BillIndex: 1, SpareX: int(recipe.Club)}, bills); i != 0 {
		t.Fatalf("moved: %d", i)
	}
	if i := billOf(job.Record{BillIndex: 5, SpareX: int(recipe.Club)}, bills); i != 0 {
		t.Fatalf("past the end: %d", i)
	}
	if i := billOf(job.Record{BillIndex: 0, SpareX: int(recipe.Club)}, bills[1:]); i != 1 {
		t.Fatalf("removed: %d", i)
	}
	f, c := woodBench(t, gameplay.HaulAllIngredients)
	c.Shop.Assign(pawn, crafting)
	c.Bills.Set(1, nil)
	walkBill(t, f, c)
	if j := f.jobs[pawn]; j.State != job.Fail || j.Fail != job.TargetGone || f.inPlay(item.Wood) != 6 {
		t.Fatalf("cancelled: %+v, %d wood", j, f.inPlay(item.Wood))
	}
}

func TestPendingCountsAPackGathering(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.jobs[pawn] = job.Record{Kind: job.PackIngredients, State: job.Goto, Dest: 1}
	bl := recipe.Bill{Recipe: recipe.Club, Mode: recipe.UntilHave}
	if c.Shop.pending(bl, recipe.DefOf(recipe.Club)) != recipe.DefOf(recipe.Club).OutQty {
		t.Fatal("two crafters would both start the last club")
	}
}

func TestOrderPicksNearestNeighbour(t *testing.T) {
	t.Parallel()
	picks := []pick{{id: 1, x: 10, y: 0}, {id: 2, x: 1, y: 0}, {id: 3, x: 20, y: 0}, {id: 4, x: 11, y: 0}, {id: 5, x: 2, y: 0}}
	orderPicks(picks, 0, 0)
	var got []entity.EntityID
	for _, p := range picks {
		got = append(got, p.id)
	}
	if !slices.Equal(got, []entity.EntityID{2, 5, 1, 4, 3}) {
		t.Fatalf("order %v", got)
	}
	// ties by id, whatever order they came in
	ties := []pick{{id: 9, x: 3}, {id: 7, x: -3}, {id: 8, y: 3}}
	orderPicks(ties, 0, 0)
	if ties[0].id != 7 {
		t.Fatalf("tie: %+v", ties)
	}
}

// CSE-032: every load keeps its id on the bench tile, in the order laid, so
// nothing merges them; one leaving fails the bill and the others stay.
func TestLaidLoadsKeepOrder(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.on(gameplay.HaulAllIngredients)
	f.put(50, item.Stack{Kind: item.Steel, Qty: 5, X: 5, Y: 7})
	f.put(51, item.Stack{Kind: item.Wood, Qty: 4, X: 5, Y: 6})
	f.put(52, item.Stack{Kind: item.Cloth, Qty: 1, X: 6, Y: 6})
	d := recipe.DefOf(recipe.Club)
	d.Inputs = []recipe.Input{{Kind: item.Wood, Qty: 4}, {Kind: item.Steel, Qty: 2}, {Kind: item.Cloth, Qty: 1}}
	rec, ok := c.Shop.plan(pawn, c.Bills.Of(1)[0], d, [2]int{4, 5})
	if !ok || rec.Kind != job.PackIngredients {
		t.Fatalf("plan %+v", rec)
	}
	rec.Dest, rec.DestX, rec.DestY, rec.WorkLeft = 1, 4, 5, d.Work
	c.Shop.start(pawn, rec)
	walkBill(t, f, c)
	j := f.jobs[pawn]
	if j.Kind != job.Bill || len(j.Route) != 2 {
		t.Fatalf("job %+v", j)
	}
	var kinds []item.Kind
	for _, rid := range j.Route {
		st := f.stacks[rid]
		if st.X != 4 || st.Y != 4 || st.Carrier != 0 || f.claims[rid] != pawn {
			t.Fatalf("load %d: %+v, claim %d", rid, st, f.claims[rid])
		}
		kinds = append(kinds, st.Kind)
	}
	if want := []item.Kind{item.Wood, item.Steel}; !slices.Equal(kinds, want) { // nearest next, ties by id; the cloth in hand
		t.Fatalf("laid %v, want %v", kinds, want)
	}
	if !c.Shop.laidOut(j) {
		t.Fatal("laid out")
	}
	f.DeleteItem(j.Route[1])
	if c.Shop.laidOut(j) {
		t.Fatal("a load left the bench: the bill must fail")
	}
}

// foodDef is a cook's recipe taking 10 of any raw food, by way of the search.
func foodDef() recipe.Def {
	return recipe.Def{Inputs: []recipe.Input{{AnyRaw: true, Qty: 10}}, Skill: work.Cook}
}

func planFor(t *testing.T, c *Craft, d recipe.Def) []pick {
	t.Helper()
	picks, ok := c.Shop.p.plan(pawn, c.Bills.Of(1)[0], d, [2]int{4, 5}, &c.Shop.buf)
	if !ok {
		t.Fatal("no plan")
	}
	return picks
}

func TestPlanPrefersSpoiling(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.on(gameplay.SpoilingIngredients)
	shelf := item.Stack{Kind: item.Meat}.Shelf()
	f.put(50, item.Stack{Kind: item.Meat, Qty: 20, X: 5, Y: 5})                   // near, fresh
	f.put(51, item.Stack{Kind: item.Meat, Qty: 20, X: 15, Y: 15, Rot: shelf - 1}) // far, spoiling
	if p := planFor(t, c, foodDef()); p[0].id != 51 {
		t.Fatalf("picks %+v", p)
	}
}

// Past the horizon a stack spoiling sooner is no reason to walk further.
func TestPlanSpoilingHorizon(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.on(gameplay.SpoilingIngredients)
	st := item.Stack{Kind: item.Pemmican, Qty: 20, X: 5, Y: 5}
	if st.Shelf() < 2*spoilHorizon*clock.TicksPerHour {
		t.Fatalf("fixture: pemmican keeps only %d ticks", st.Shelf())
	}
	f.put(50, st)
	st.X, st.Y, st.Rot = 15, 15, 1000
	f.put(51, st)
	d := foodDef()
	d.Inputs = []recipe.Input{{Kind: item.Pemmican, Qty: 10}}
	if p := planFor(t, c, d); p[0].id != 50 {
		t.Fatalf("picks %+v", p)
	}
}

func TestPlanIgnoresRotForSurgery(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	f.on(gameplay.SpoilingIngredients)
	shelf := item.Stack{Kind: item.Meat}.Shelf()
	f.put(50, item.Stack{Kind: item.Meat, Qty: 20, X: 5, Y: 5})
	f.put(51, item.Stack{Kind: item.Meat, Qty: 20, X: 15, Y: 15, Rot: shelf - 1})
	d := foodDef()
	d.Skill = work.Doctor
	if p := planFor(t, c, d); p[0].id != 50 {
		t.Fatalf("picks %+v", p)
	}
}

func TestPlanOffKeepsNearest(t *testing.T) {
	t.Parallel()
	f, c := clubBench(t, recipe.Bill{})
	shelf := item.Stack{Kind: item.Meat}.Shelf()
	f.put(50, item.Stack{Kind: item.Meat, Qty: 20, X: 5, Y: 5})
	f.put(51, item.Stack{Kind: item.Meat, Qty: 20, X: 15, Y: 15, Rot: shelf - 1})
	if p := planFor(t, c, foodDef()); p[0].id != 50 {
		t.Fatalf("picks %+v", p)
	}
}

// A single-input bill runs on the nearest stack unless something really spoils.
func TestSingleStackBillMindsSpoiling(t *testing.T) {
	t.Parallel()
	shelf := item.Stack{Kind: item.Meat}.Shelf()
	d := recipe.Def{Input: recipe.Input{AnyRaw: true, Qty: 10}, Skill: work.Cook}
	for _, tc := range []struct {
		name string
		rule bool
		rot  int
		want entity.EntityID
	}{
		{"spoiling, rule on", true, shelf - 1, 51},
		{"spoiling, rule off", false, shelf - 1, 50},
		{"both fresh, rule on", true, 0, 50},
	} {
		f, c := clubBench(t, recipe.Bill{})
		if tc.rule {
			f.on(gameplay.SpoilingIngredients)
		}
		f.put(50, item.Stack{Kind: item.Meat, Qty: 20, X: 5, Y: 5})
		f.put(51, item.Stack{Kind: item.Meat, Qty: 20, X: 15, Y: 15, Rot: tc.rot})
		rec, ok := c.Shop.plan(pawn, c.Bills.Of(1)[0], d, [2]int{4, 5})
		if !ok || rec.Kind != job.Bill || rec.Target != tc.want {
			t.Fatalf("%s: %+v", tc.name, rec)
		}
	}
}

// CSE-034 closes with CSE-033 for as long as this holds: potency only orders a
// choice among medicines, and no recipe offers one.
func TestPlanPotencyFirst(t *testing.T) {
	t.Parallel()
	for _, d := range recipe.All() {
		for _, in := range d.AppendInputs(nil) {
			n := 0
			for _, k := range []item.Kind{item.HerbalMedicine, item.IndustrialMedicine, item.NanomedKit} {
				if in.Accepts(k) {
					n++
				}
			}
			if n > 1 {
				t.Fatalf("%s takes %d medicines: order them by potency in planner.plan (CSE-034)", d.Name, n)
			}
		}
	}
}

// legacyOrder is the search's order as it was before the habits: distance from
// the work spot, then id; each kind's run together, kinds by first appearance.
func legacyOrder(stacks map[entity.EntityID]item.Stack, spot [2]int, need int) []entity.EntityID {
	var ids []entity.EntityID
	for id := range stacks {
		ids = append(ids, id)
	}
	dist := func(id entity.EntityID) int {
		st := stacks[id]
		return max(abs(st.X-spot[0]), abs(st.Y-spot[1]))
	}
	slices.SortFunc(ids, func(a, b entity.EntityID) int {
		if c := cmp.Compare(dist(a), dist(b)); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	var out []entity.EntityID
	for _, id := range ids {
		if need <= 0 {
			break
		}
		take := min(need, stacks[id].Qty)
		out = append(out, id, entity.EntityID(take))
		need -= take
	}
	return out
}

func abs(n int) int { return max(n, -n) }

// With every rule off the bill goes today's way: the same route, planned as it
// was planned, walked by hand, with no host habit asked at all. Compared against
// the rules on, which take the same stacks to the same bill by a different path.
func TestRulesOffKeepTodaysPath(t *testing.T) {
	t.Parallel()
	rot := uint64(0x9E3779B97F4A7C15)
	for seed := range 40 {
		layout := map[entity.EntityID]item.Stack{}
		for i := range 5 {
			rot = rot*0xBF58476D1CE4E5B9 + uint64(seed+i)
			layout[entity.EntityID(50+i)] = item.Stack{Kind: item.Wood, Qty: 1 + int(rot>>60)%2,
				X: int(rot>>8) % 30, Y: int(rot>>16) % 30}
		}
		off, offC := clubBench(t, recipe.Bill{})
		on, onC := clubBench(t, recipe.Bill{})
		on.on(gameplay.HaulAllIngredients, gameplay.RespectCapacity, gameplay.SpoilingIngredients, gameplay.StackToStockpile)
		total := 0
		for id, st := range layout {
			off.put(id, st)
			on.put(id, st)
			total += st.Qty
		}
		if total < 4 {
			continue
		}
		if !offC.Shop.Assign(pawn, crafting) || !onC.Shop.Assign(pawn, crafting) {
			t.Fatal("no bill")
		}
		offJ, onJ := off.jobs[pawn], on.jobs[pawn]
		if offJ.Kind != job.HaulIngredients || onJ.Kind != job.PackIngredients {
			continue // one stack held enough (the plain bill) or a layout with a single run
		}
		if want := legacyOrder(layout, [2]int{offJ.DestX, offJ.DestY}, 4); !slices.Equal(offJ.Route, want) {
			t.Fatalf("seed %d: route %v, today's %v", seed, offJ.Route, want)
		}
		if offJ.SpareX != 0 {
			t.Fatalf("seed %d: the hand path marked: %+v", seed, offJ)
		}
		walkBill(t, off, offC)
		walkBill(t, on, onC)
		if off.stowCalls+off.stackCalls != 0 {
			t.Fatalf("seed %d: a habit asked with the rules off: %d %d", seed, off.stowCalls, off.stackCalls)
		}
		a, b := off.jobs[pawn], on.jobs[pawn]
		if a.Kind != job.Bill || b.Kind != job.Bill || off.inPlay(item.Wood) != total || on.inPlay(item.Wood) != total {
			t.Fatalf("seed %d: %+v / %+v, wood %d %d", seed, a, b, off.inPlay(item.Wood), on.inPlay(item.Wood))
		}
		sum := func(f *fakeHost, j job.Record) int {
			n := f.stacks[j.Carry].Qty
			for _, rid := range j.Route {
				n += f.stacks[rid].Qty
			}
			return n
		}
		if sum(off, a) != 4 || sum(on, b) != 4 {
			t.Fatalf("seed %d: at the bench %d by hand, %d by pack", seed, sum(off, a), sum(on, b))
		}
	}
}

// The bill's call (CSE-018): once the last load is in, on the first work tick,
// with the load still in hand; the host keeps it through the clean.
func TestBillCleansBeforeWork(t *testing.T) {
	t.Parallel()
	f, c := woodBench(t, gameplay.HaulAllIngredients)
	f.tidyStarts = true
	c.Shop.Assign(pawn, crafting)
	walkBill(t, f, c)
	j := f.jobs[pawn]
	left := j.WorkLeft
	c.Shop.Do(pawn, j)
	if f.jobs[pawn].Kind != job.CleanRoom || f.tidyCalls != 1 {
		t.Fatalf("job %+v, calls %d", f.jobs[pawn], f.tidyCalls)
	}
	if f.resume.Kind != job.Bill || f.resume.Carry != j.Carry || f.resume.WorkLeft != left {
		t.Fatalf("resume %+v", f.resume)
	}
	if f.inPlay(item.Wood) != 6 {
		t.Fatalf("%d wood", f.inPlay(item.Wood))
	}
}

// TestBillCleanDoesNotFlap: a whole bill, one clean, however long the work.
func TestBillCleanDoesNotFlap(t *testing.T) {
	t.Parallel()
	f, c := woodBench(t)
	f.tidyStarts = true
	c.Shop.Assign(pawn, crafting)
	walkBill(t, f, c)
	cleans := 0
	for range 2000 {
		j := f.jobs[pawn]
		if j.Kind == job.CleanRoom {
			cleans++
			j = f.resume
		}
		if j.State == job.Complete {
			break
		}
		j.State, j.Path, j.PathI = job.Do, nil, 0
		f.jobs[pawn] = j
		f.now++
		c.Shop.Do(pawn, j)
	}
	if cleans != 1 || f.jobs[pawn].State != job.Complete {
		t.Fatalf("%d cleans, job %+v", cleans, f.jobs[pawn])
	}
	if f.tidyCalls != 2 { // the one that cleaned, the one declined on the way back
		t.Fatalf("asked %d times", f.tidyCalls)
	}
	if f.inPlay(item.Wood) != 2 {
		t.Fatalf("%d wood", f.inPlay(item.Wood))
	}
}

func TestStackFirstAskedOnce(t *testing.T) {
	t.Parallel()
	f, c := woodBench(t, gameplay.StackToStockpile)
	f.stackStarts = true
	if !c.Shop.Assign(pawn, crafting) {
		t.Fatal("no work")
	}
	if j := f.jobs[pawn]; j.Kind != job.Haul || f.stackCalls != 1 {
		t.Fatalf("job %+v, calls %d", j, f.stackCalls)
	}
	if owner, held := f.claims[1]; held {
		t.Fatalf("bench claimed by %d for a bill not started", owner)
	}
	// the host declines (its cooldown): the bill starts
	f.jobs[pawn] = job.Record{}
	f.stackStarts = false
	if !c.Shop.Assign(pawn, crafting) || f.jobs[pawn].Kind != job.HaulIngredients {
		t.Fatalf("job %+v", f.jobs[pawn])
	}
}

// benchFake is a bench with n loose stacks of one kind, none large enough alone.
func benchFake(b *testing.B, n int, rules ...gameplay.Toggle) (*fakeHost, *Craft) {
	b.Helper()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.names[pawn] = "Ada"
	f.on(rules...)
	bl := recipe.Bill{Recipe: recipe.Club, Mode: recipe.Forever, Target: 1}
	f.build(1, 0, 4, 4)
	c.Bills.Set(1, []recipe.Bill{bl})
	for i := range n {
		f.put(entity.EntityID(100+i), item.Stack{Kind: item.Wood, Qty: 1, X: i % 200, Y: i / 200})
	}
	return f, c
}

// BenchmarkPlanManyStacks is a bill's search over 2000 stacks (the row's scan
// bound: the search reads the item list as it always did; the pack path adds a
// nearest-neighbour order over the handful of picks).
func BenchmarkPlanManyStacks(b *testing.B) {
	for _, tc := range []struct {
		name  string
		rules []gameplay.Toggle
	}{
		{"rules_off", nil},
		{"rules_on", []gameplay.Toggle{gameplay.HaulAllIngredients, gameplay.SpoilingIngredients}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			_, c := benchFake(b, 2000, tc.rules...)
			d := recipe.DefOf(recipe.Club)
			bl := c.Bills.Of(1)[0]
			b.ReportAllocs()
			for b.Loop() {
				if _, ok := c.Shop.plan(pawn, bl, d, [2]int{4, 5}); !ok {
					b.Fatal("no plan")
				}
			}
		})
	}
}

// BenchmarkPackGatherManyPicks is one whole gathering, 20 picks, in the pack.
func BenchmarkPackGatherManyPicks(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		f, c := benchFake(b, 2000, gameplay.HaulAllIngredients)
		d := recipe.DefOf(recipe.Club)
		d.Input.Qty = 20
		rec, ok := c.Shop.plan(pawn, c.Bills.Of(1)[0], d, [2]int{4, 5})
		if !ok {
			b.Fatal("no plan")
		}
		rec.Dest, rec.DestX, rec.DestY, rec.WorkLeft = 1, 4, 5, d.Work
		c.Shop.start(pawn, rec)
		b.StartTimer()
		for {
			j := f.jobs[pawn]
			if j.Kind != job.PackIngredients {
				break
			}
			j.State, j.Path, j.PathI = job.Do, nil, 0
			f.jobs[pawn] = j
			c.Shop.Do(pawn, j)
		}
	}
}
