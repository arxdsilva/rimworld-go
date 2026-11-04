package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// CSE-018, 029…035, 040 as the whole game runs them: a bill gathered in the pack,
// and nothing duplicated, lost or left claimed on any way out of it.

// packBill is a whole game around one bill with two or more ingredients, each in
// two stacks with a few to spare.
type packBill struct {
	g           *Game
	ada, bench  entity.EntityID
	d           recipe.Def
	start, used map[item.Kind]int
}

// quickMixedRecipe is the quickest recipe of two or more plain ingredients.
func quickMixedRecipe(t *testing.T) recipe.Def {
	t.Helper()
	var best recipe.Def
	for _, d := range recipe.All() {
		if len(d.Inputs) < 2 || d.Legacy {
			continue
		}
		plain := true
		for _, in := range d.Inputs {
			plain = plain && in.Kind != 0 && in.Qty >= 2 && in.Qty < item.CarryLimit
		}
		if plain && (best.Inputs == nil || d.Work < best.Work) {
			best = d
		}
	}
	if best.Inputs == nil {
		t.Fatal("no mixed recipe")
	}
	return best
}

func newPackBill(t *testing.T, setup ...func(*Game)) *packBill {
	t.Helper()
	d := quickMixedRecipe(t)
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	for _, f := range setup {
		f(g)
	}
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	setSkill(g, ada, d.Learn, max(d.MinSkill, 10))
	bench := buildNow(t, g, d.Bench, construction.Stuffs(d.Bench)[0], construction.South, 15, 10)
	if power.SpecOf(d.Bench).Role == power.Consumer {
		onPower(t, g, 15, 10, 18, 13)
	}
	if _, fueled := fuelSpec(d.Bench); fueled {
		g.setFuel(bench, g.fuelCap(bench))
	}
	p := &packBill{g: g, ada: ada, bench: bench, d: d, start: map[item.Kind]int{}, used: map[item.Kind]int{}}
	for i, in := range d.Inputs {
		stack(t, g, in.Kind, in.Qty/2, 4+i, 6)
		stack(t, g, in.Kind, in.Qty-in.Qty/2+3, 8+i, 3)
		p.start[in.Kind] += in.Qty + 3
		p.used[in.Kind] += in.Qty
	}
	g.Enqueue(command.Command{Kind: command.AddBill, X: 15, Y: 10, Recipe: uint8(d.ID), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	return p
}

func (p *packBill) done() bool {
	bills := p.g.craft.Bills.Of(p.bench)
	return len(bills) > 0 && bills[0].Done > 0
}

// balanced fails the test unless every ingredient adds up across map, hands and
// packs: what the bill started with, less what a finished bill used.
func (p *packBill) balanced(t *testing.T, tick int) {
	t.Helper()
	for k, want := range p.start {
		if p.done() {
			want -= p.used[k]
		}
		if got := inPlay(p.g, k); got != want {
			t.Fatalf("tick %d: %d of %s in play, want %d", tick, got, k, want)
		}
	}
}

// step runs n ticks with needs full, checking the balance each one.
func (p *packBill) step(t *testing.T, n int, stop func() bool) {
	t.Helper()
	for i := 0; i < n && (stop == nil || !stop()); i++ {
		if p.g.clock.Now()%100 == 0 {
			p.g.SetNeeds(p.ada, needs.Full, needs.Full)
		}
		p.g.Step(1)
		p.balanced(t, i)
	}
}

// billStages are the points a bill can be cut at.
var billStages = []struct {
	name string
	at   func(p *packBill) bool
}{
	{"stowed", func(p *packBill) bool {
		j, _ := p.g.jobs.Get(p.ada)
		return j.Kind == job.PackIngredients && senseOf(p.g).Gear.Has(p.ada)
	}},
	{"laid", func(p *packBill) bool {
		j, _ := p.g.jobs.Get(p.ada)
		return j.Kind == job.Bill && j.Carry != 0 && len(j.Route) > 0 && j.Dest == p.bench
	}},
	{"working", func(p *packBill) bool {
		j, _ := p.g.jobs.Get(p.ada)
		b, _ := p.g.bldgs.Get(p.bench)
		return j.Kind == job.Bill && j.Carry != 0 && j.Dest == p.bench && j.WorkLeft < craft.BillWork(b.Kind, p.d)
	}},
}

// runTo steps to a stage of the bill.
func (p *packBill) runTo(t *testing.T, name string, at func(*packBill) bool) {
	t.Helper()
	p.step(t, 12000, func() bool { return at(p) })
	if !at(p) {
		j, _ := p.g.jobs.Get(p.ada)
		t.Fatalf("never reached %s: job %v, %d worked", name, j.Kind, p.g.craft.Bills.Of(p.bench)[0].Done)
	}
}

func claimsOf(g *Game, id entity.EntityID) []reservation.Claim {
	var out []reservation.Claim
	for _, c := range g.reserve.Export() {
		if c.Owner == id {
			out = append(out, c)
		}
	}
	return out
}

// inHands is what stands in id's hands.
func inHands(g *Game, id entity.EntityID) []entity.EntityID {
	var out []entity.EntityID
	for _, sid := range g.itemIDs {
		if st, ok := g.stacks.Get(sid); ok && st.Carrier == id {
			out = append(out, sid)
		}
	}
	return out
}

// noBillClaims fails the test if anyone holds the bench or an ingredient.
func (p *packBill) noBillClaims(t *testing.T) {
	t.Helper()
	for _, c := range p.g.reserve.Export() {
		if c.Tile {
			continue
		}
		st, isStack := p.g.stacks.Get(c.Target)
		if c.Target == p.bench || isStack && p.start[st.Kind] > 0 {
			t.Fatalf("still claimed: %+v (job %v)", c, jobKind(p.g, c.Owner))
		}
	}
}

func TestPackBillBalancesWholeBill(t *testing.T) {
	t.Parallel()
	p := newPackBill(t)
	s := senseOf(p.g)
	seen := map[job.Kind]bool{}
	for i := 0; i < 30000 && !p.done(); i++ {
		p.step(t, 1, nil)
		seen[jobKind(p.g, p.ada)] = true
		if g, c := s.Packs[p.ada].Grams(), s.Gear.Capacity(p.ada); g > c {
			t.Fatalf("tick %d: %d g in a pack of %d", i, g, c)
		}
	}
	if !p.done() || !seen[job.PackIngredients] {
		t.Fatalf("done %v, kinds %v", p.done(), seen)
	}
	if _, from := s.FromPack[p.ada]; s.Gear.Has(p.ada) || from {
		t.Fatalf("after the bill: pack %+v, from pack %+v", s.Packs[p.ada], s.FromPack)
	}
	if p.g.stockOf(p.d.Output) < p.d.OutQty {
		t.Fatalf("no %s made", p.d.Output)
	}
	p.step(t, 50, nil)
	p.noBillClaims(t)
}

func TestPackBillCancelledMidwayBalances(t *testing.T) {
	t.Parallel()
	for _, st := range billStages {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			p := newPackBill(t)
			p.runTo(t, st.name, st.at)
			p.g.Enqueue(command.Command{Kind: command.RemoveBill, X: 15, Y: 10, Index: 0})
			p.step(t, 600, nil)
			if k := jobKind(p.g, p.ada); k == job.PackIngredients || k == job.Bill {
				t.Fatalf("the job outlived its bill: %v", k)
			}
			p.noBillClaims(t)
			if h := inHands(p.g, p.ada); len(h) != 0 {
				if j, _ := p.g.jobs.Get(p.ada); j.Carry == 0 {
					t.Fatalf("held by nobody's job: %v", h)
				}
			}
		})
	}
}

func TestPackBillInterruptReleasesAll(t *testing.T) {
	t.Parallel()
	for _, st := range billStages {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			p := newPackBill(t)
			p.runTo(t, st.name, st.at)
			p.g.interrupt(p.ada, job.Interrupted)
			if c := claimsOf(p.g, p.ada); len(c) != 0 {
				t.Fatalf("claims left: %+v", c)
			}
			if h := inHands(p.g, p.ada); len(h) != 0 {
				t.Fatalf("still in hand: %v", h)
			}
			p.balanced(t, 0)
			p.step(t, 30000, p.done) // and the bill still gets done
			if !p.done() {
				t.Fatalf("no recovery, job %v", jobKind(p.g, p.ada))
			}
		})
	}
}

func TestPackBillDownedReleasesAll(t *testing.T) {
	t.Parallel()
	for _, st := range billStages {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			p := newPackBill(t)
			p.runTo(t, st.name, st.at)
			p.g.down(p.ada, health.CauseInjury)
			p.step(t, 5, nil)
			if c := claimsOf(p.g, p.ada); len(c) != 0 {
				t.Fatalf("claims left: %+v", c)
			}
			if h := inHands(p.g, p.ada); len(h) != 0 {
				t.Fatalf("still in hand: %v", h)
			}
		})
	}
}

func TestPackBillBenchDestroyedReleasesAll(t *testing.T) {
	t.Parallel()
	for _, st := range billStages {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			p := newPackBill(t)
			p.runTo(t, st.name, st.at)
			p.g.removeBuildingAs(p.bench, false)
			p.step(t, 600, nil)
			if k := jobKind(p.g, p.ada); k == job.PackIngredients || k == job.Bill {
				t.Fatalf("the job outlived its bench: %v", k)
			}
			p.noBillClaims(t)
		})
	}
}

// TestPackBillSavesMidway: a save at each stage hashes the same, and both games
// go on the same.
func TestPackBillSavesMidway(t *testing.T) {
	t.Parallel()
	for _, st := range billStages {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			p := newPackBill(t)
			p.runTo(t, st.name, st.at)
			var buf bytes.Buffer
			if err := p.g.Save(&buf, "mid bill"); err != nil {
				t.Fatal(err)
			}
			h, _, err := Load(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if a, b := p.g.StateHash(), h.StateHash(); a != b {
				t.Fatalf("hash %x, loaded %x", a, b)
			}
			for i := range 400 {
				p.g.Step(1)
				h.Step(1)
				if a, b := p.g.StateHash(), h.StateHash(); a != b {
					t.Fatalf("tick %d after the load: hash %x, loaded %x", i, a, b)
				}
			}
		})
	}
}

// offHabits turns the bill rows' rules off.
func offHabits(g *Game) {
	s := senseOf(g)
	for _, r := range []gameplay.Toggle{gameplay.HaulAllIngredients, gameplay.RespectCapacity, gameplay.SpoilingIngredients, gameplay.StackToStockpile, gameplay.CleanBeforeBill} {
		s.Rules = s.Rules.With(r, false)
	}
}

// TestPackBillRulesOffGoesByHand compares both paths on one bill: off, the hands
// do it as ever and no state of the rules is touched; on, one pack trip does it
// sooner, for the same product.
func TestPackBillRulesOffGoesByHand(t *testing.T) {
	t.Parallel()
	run := func(off bool) (ticks int, kinds map[job.Kind]bool, p *packBill) {
		if off {
			p = newPackBill(t, offHabits)
		} else {
			p = newPackBill(t)
		}
		kinds = map[job.Kind]bool{}
		for ticks = 0; ticks < 30000 && !p.done(); ticks++ {
			p.step(t, 1, nil)
			kinds[jobKind(p.g, p.ada)] = true
		}
		if !p.done() {
			t.Fatalf("off %v: never done", off)
		}
		return ticks, kinds, p
	}
	offTicks, off, po := run(true)
	onTicks, on, pn := run(false)
	if off[job.PackIngredients] || !off[job.HaulIngredients] || !on[job.PackIngredients] {
		t.Fatalf("off %v, on %v", off, on)
	}
	s := senseOf(po.g)
	if len(s.Packs) != 0 || len(s.HaulAt) != 0 {
		t.Fatalf("rules off touched packs %+v, cooldowns %+v", s.Packs, s.HaulAt)
	}
	if onTicks >= offTicks {
		t.Fatalf("the pack took %d ticks, the hands %d", onTicks, offTicks)
	}
	if a, b := po.g.stockOf(po.d.Output), pn.g.stockOf(pn.d.Output); a != b {
		t.Fatalf("made %d by hand, %d by pack", a, b)
	}
}

// TestLeftoversUnloadAfterBill (CSE-040): what a cut bill left in the pack is
// marked, and the unload takes it to storage.
func TestLeftoversUnloadAfterBill(t *testing.T) {
	t.Parallel()
	p := newPackBill(t, func(g *Game) {
		s := senseOf(g)
		s.Rules = s.Rules.With(gameplay.StackToStockpile, false)
	})
	p.g.Apply(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 8, X2: 4, Y2: 10})
	offStack := senseOf(p.g)
	p.runTo(t, "stowed", billStages[0].at)
	p.g.Enqueue(command.Command{Kind: command.RemoveBill, X: 15, Y: 10, Index: 0})
	p.step(t, 1, nil)
	items := offStack.Packs[p.ada].Items
	if len(items) == 0 {
		t.Fatal("nothing left in the pack")
	}
	for _, e := range items {
		if !e.Unload {
			t.Fatalf("not marked: %+v", e)
		}
	}
	p.step(t, 3000, func() bool { return !offStack.Gear.Has(p.ada) })
	if offStack.Gear.Has(p.ada) {
		t.Fatalf("still packed: %+v, job %v", offStack.Packs[p.ada], jobKind(p.g, p.ada))
	}
	p.balanced(t, 0)
}

// TestPackBillRespectsCapacity (CSE-030): a nearly full pack takes the bill in
// trips, never over its limit, and what it held already stays unmarked.
func TestPackBillRespectsCapacity(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	for _, c := range [][2]int{{9, 6}, {11, 8}, {2, 8}} {
		stack(t, g, item.Wood, 2, c[0], c[1])
	}
	s := senseOf(g)
	wood, steel := item.Wood.MassGrams(), item.Steel.MassGrams()
	n := (s.Gear.Room(ada) - wood*2) / steel
	packed(t, g, ada, stack(t, g, item.Steel, n, 1, 2), n)
	if r := s.Gear.Room(ada); r < wood || r >= 4*wood {
		t.Fatalf("fixture: room %d for wood of %d", r, wood)
	}
	trips, was := 0, false
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	for i := 0; i < 20000 && !done(); i++ {
		if i%100 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
		if got, c := s.Packs[ada].Grams(), s.Gear.Capacity(ada); got > c {
			t.Fatalf("tick %d: %d g in a pack of %d", i, got, c)
		}
		has := slices.ContainsFunc(s.Packs[ada].Items, func(e inventory.Entry) bool { return e.Stack.Kind == item.Wood })
		if has && !was {
			trips++
		}
		was = has
		if got := inPlay(g, item.Wood); got != 6 && !done() {
			t.Fatalf("tick %d: %d wood", i, got)
		}
	}
	if !done() || trips < 2 {
		t.Fatalf("done %v, %d trips", done(), trips)
	}
	if got := inPlay(g, item.Steel); got != n {
		t.Fatalf("%d steel of %d", got, n)
	}
	for _, e := range s.Packs[ada].Items {
		if e.Stack.Kind == item.Steel && e.Unload {
			t.Fatalf("what the pack held is marked: %+v", e)
		}
	}
}

// stackFirstGame is a crafter at (1, 1) working by hand (the pack rule off), a
// club bill at a bench at (bx, by) and storage at (zx, zy), on open ground.
func stackFirstGame(t *testing.T, bx, by, zx, zy int) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	senseOf(g).Rules = senseOf(g).Rules.With(gameplay.HaulAllIngredients, false)
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.setPriority(ada, work.Craft, 1) // the bill before any plain hauling
	bench := buildNow(t, g, construction.Workbench, item.Wood, construction.South, bx, by)
	g.Apply(command.Command{Kind: command.AddBill, X: bx, Y: by, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: zx, Y: zy, X2: zx + 1, Y2: zy + 1})
	return g, ada, bench
}

// TestBillHaulsWholeStackFirst (CSE-035): storage nearer the pawn than the bench
// outdoors: the stack goes there whole, then the bill is done.
func TestBillHaulsWholeStackFirst(t *testing.T) {
	t.Parallel()
	g, ada, bench := stackFirstGame(t, 16, 8, 2, 1)
	wood := stack(t, g, item.Wood, 20, 8, 9)
	done := func() bool { b := g.craft.Bills.Of(bench); return len(b) > 0 && b[0].Done == 1 }
	s := senseOf(g)
	var haulAt uint64
	seen := runBillTicks(g, ada, 12000, func() bool {
		if haulAt == 0 {
			haulAt = s.HaulAt[ada]
		}
		return done()
	})
	if !done() || haulAt == 0 || !seen[job.Haul] {
		t.Fatalf("done %v, cooldown %d, jobs %v", done(), haulAt, seen)
	}
	if st, ok := g.stacks.Get(wood); ok && st.X == 8 && st.Y == 9 {
		t.Fatal("the stack stayed where it was")
	}
	if got := inPlay(g, item.Wood); got != 20-recipe.DefOf(recipe.Club).Input.Qty {
		t.Fatalf("%d wood", got)
	}
}

func TestBillSkipsHaulWhenBenchNearer(t *testing.T) {
	t.Parallel()
	g, ada, bench := stackFirstGame(t, 5, 2, 17, 9)
	stack(t, g, item.Wood, 20, 8, 9)
	done := func() bool { b := g.craft.Bills.Of(bench); return len(b) > 0 && b[0].Done == 1 }
	s := senseOf(g)
	runBillTicks(g, ada, 12000, func() bool {
		if s.HaulAt[ada] != 0 {
			t.Fatal("hauled with the bench nearer")
		}
		return done()
	})
	if !done() {
		t.Fatal("no club")
	}
}

// TestStackFirstDoesNotFlap: one haul a cooldown at most, whatever the pawn does
// in between, and another once it is over.
func TestStackFirstDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, ada, _ := stackFirstGame(t, 16, 8, 2, 1)
	g.Step(1)
	h := craftPrep{g}
	a := stack(t, g, item.Wood, 20, 8, 9)
	if !h.StackFirst(ada, 19, 11, []entity.EntityID{a}) || jobKind(g, ada) != job.Haul {
		t.Fatalf("no haul: job %v", jobKind(g, ada))
	}
	until := senseOf(g).HaulAt[ada]
	for range 5 {
		g.interrupt(ada, job.Interrupted)
		if h.StackFirst(ada, 19, 11, []entity.EntityID{a}) {
			t.Fatal("hauled again inside the cooldown")
		}
		g.Step(1)
	}
	for uint64(g.clock.Now()) < until {
		g.Step(1)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	g.interrupt(ada, job.Interrupted)
	b := stack(t, g, item.Wood, 20, 12, 9)
	if !h.StackFirst(ada, 19, 11, []entity.EntityID{b}) {
		t.Fatal("no haul after the cooldown")
	}
}

func TestStackFirstStaysInTheRoom(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	m, _ := g.labor.Get(id)
	g.labor.Set(id, m.Set(work.Haul, true))
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 3, Y: 5, X2: 4, Y2: 5})
	g.Step(1)
	wood := stack(t, g, item.Wood, 20, 7, 7)
	if (craftPrep{g}).StackFirst(id, 8, 8, []entity.EntityID{wood}) {
		t.Fatal("hauled a stack already in the bench's room")
	}
}

// billCleanGame is a crafter and cleaner in a dirty hut with a club bill, only
// the tidy at the bill's last load left on (CSE-018).
func billCleanGame(t *testing.T) (g *Game, id, bench entity.EntityID) {
	t.Helper()
	g, id = tidyGame(t)
	g.setPriority(id, work.Clean, 3)
	g.setPriority(id, work.Craft, 1)
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.CleanBeforeWork, false).With(gameplay.StackToStockpile, false).With(gameplay.CleanBeforeBill, true)
	bench = buildNow(t, g, construction.Workbench, item.Wood, construction.South, 5, 4)
	stack(t, g, item.Wood, 10, 7, 7)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 4, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
	dirty(g, [2]int{3, 3}, [2]int{7, 3}, [2]int{8, 8})
	g.Step(1)
	return g, id, bench
}

func TestBillCleansAroundItsLastLoad(t *testing.T) {
	t.Parallel()
	g, id, bench := billCleanGame(t)
	s := senseOf(g)
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	cleans, was := 0, false
	var load entity.EntityID
	for i := 0; i < 20000 && !done(); i++ {
		if i%100 == 0 {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
		g.Step(1)
		r, cleaning := s.Resume[id]
		cleaning = cleaning && r.Kind == job.Bill && jobKind(g, id) == job.CleanRoom
		if cleaning && !was {
			cleans++
			load = r.Carry
		}
		was = cleaning
		if cleaning {
			if st, ok := g.stacks.Get(r.Carry); r.Carry == 0 || !ok || st.Carrier != id {
				t.Fatalf("tick %d: the load is not in hand: %+v", i, r)
			}
			if o, ok := g.reserve.EntityOwner(bench); !ok || o != id {
				t.Fatalf("tick %d: the bench is free during the clean", i)
			}
		}
		if got := inPlay(g, item.Wood); got != 10 && !done() {
			t.Fatalf("tick %d: %d wood", i, got)
		}
	}
	if !done() || cleans != 1 || load == 0 {
		t.Fatalf("done %v, %d cleans, load %d", done(), cleans, load)
	}
}

func TestBillCleanInterruptedPutsTheLoadDown(t *testing.T) {
	t.Parallel()
	g, id, _ := billCleanGame(t)
	s := senseOf(g)
	for i := 0; ; i++ {
		if i == 20000 {
			t.Fatal("no clean")
		}
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
		if r, ok := s.Resume[id]; ok && r.Kind == job.Bill && r.Carry != 0 && jobKind(g, id) == job.CleanRoom {
			break
		}
	}
	g.interrupt(id, job.Interrupted)
	if c := claimsOf(g, id); len(c) != 0 {
		t.Fatalf("claims left: %+v", c)
	}
	if h := inHands(g, id); len(h) != 0 || len(s.Resume) != 0 {
		t.Fatalf("hands %v, resume %+v", h, s.Resume)
	}
	if got := inPlay(g, item.Wood); got != 10 {
		t.Fatalf("%d wood", got)
	}
}
