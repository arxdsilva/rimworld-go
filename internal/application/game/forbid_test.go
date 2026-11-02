package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Forbidding (UI-016): colonists leave forbidden stacks alone.

func forbidCmd(g *Game, id entity.EntityID, on bool) {
	g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(id), Enabled: on})
}

func itemView(g *Game, id entity.EntityID) (ItemView, bool) {
	for _, iv := range g.World().Items {
		if iv.ID == id {
			return iv, true
		}
	}
	return ItemView{}, false
}

func TestForbiddenStackNotHauled(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(ada, wt, 0)
	}
	g.setPriority(ada, work.Haul, work.DefaultPriority)
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 1, X2: 14, Y2: 3})
	sid, _ := g.SpawnStack(item.Wood, 20, 4, 5)
	forbidCmd(g, sid, true)
	g.Step(1)
	if iv, ok := itemView(g, sid); !ok || !iv.Forbidden {
		t.Fatalf("the item view shows it forbidden: %+v", iv)
	}
	for i := 0; i < 100; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(TicksPerCell * 4)
		if st, _ := g.stacks.Get(sid); st.Carrier != 0 || st.X != 4 || st.Y != 5 {
			t.Fatalf("a forbidden stack was picked up: %+v", st)
		}
	}
	// a frame needing wood does not take it either
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 8, 5)
	if _, _, ok := g.planDelivery(ada, Position{1, 1}); ok {
		t.Fatal("a delivery planned from a forbidden stack")
	}
	// allowed: it goes into the stockpile (or the wall)
	forbidCmd(g, sid, false)
	moved := false
	for i := 0; i < 200 && !moved; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(TicksPerCell * 4)
		st, ok := g.stacks.Get(sid)
		moved = !ok || st.X != 4 || st.Y != 5
	}
	if !moved {
		t.Fatal("the allowed stack is hauled")
	}
}

func TestForbiddenFoodNotEaten(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	sid, _ := g.SpawnStack(item.Berries, 10, 6, 4)
	g.setForbidden(sid, true)
	if g.hasEdible(false) {
		t.Fatal("forbidden food counts as edible")
	}
	if best, _, _, _ := g.bestFood(ada, ada); best != 0 {
		t.Fatalf("bestFood picked forbidden food %d", best)
	}
	for i := 0; i < 60; i++ {
		g.SetNeeds(ada, needs.CriticalHunger-1, needs.Full)
		g.Step(TicksPerCell * 4)
	}
	if st, ok := g.stacks.Get(sid); !ok || st.Qty != 10 {
		t.Fatalf("forbidden berries were eaten: %+v %v", st, ok)
	}
	g.setForbidden(sid, false)
	eaten := false
	for i := 0; i < 200 && !eaten; i++ {
		g.SetNeeds(ada, needs.CriticalHunger-1, needs.Full)
		g.Step(TicksPerCell * 4)
		st, ok := g.stacks.Get(sid)
		eaten = !ok || st.Qty < 10
	}
	if !eaten {
		t.Fatal("allowed berries get eaten")
	}
}

// STK-AL (game/allow): what a death drops starts forbidden outside the home
// area, whatever the faction; inside Home it stays allowed.
func TestDeathDropsForbiddenOutsideHome(t *testing.T) {
	t.Parallel()
	g, _ := raidGame(t)
	g.paintHome(10, 5, 20, 15, true) // Home round the colonist at (15, 10)
	g.Raid(2)
	out, in := g.raiderIDs[0], g.raiderIDs[1]
	g.positions.Set(in, Position{X: 14, Y: 10}) // one raider dies inside Home
	if p, _ := g.positions.Get(out); g.inHome(p.Y*g.grid.Width() + p.X) {
		t.Fatalf("the raid spawned inside Home at %+v", p)
	}
	wOut, wIn := g.equip[out], g.equip[in]
	g.die(out, health.CauseInjury)
	g.die(in, health.CauseInjury)
	cid, body, ok := g.bodyOf(out)
	if !ok || !body.Forbidden {
		t.Fatalf("a raider's body outside Home starts forbidden: %+v %v", body, ok)
	}
	if iv, _ := itemView(g, cid); !iv.Forbidden {
		t.Fatalf("corpse view %+v", iv)
	}
	if st, ok := stackOf(g, wOut); !ok || !st.Forbidden {
		t.Fatalf("its %s starts forbidden: %+v %v", wOut, st, ok)
	}
	if _, b, ok := g.bodyOf(in); !ok || b.Forbidden {
		t.Fatalf("a raider's body inside Home stays allowed: %+v %v", b, ok)
	}
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == wIn && st.X == 14 && st.Y == 10 && st.Forbidden {
			t.Fatalf("the %s dropped inside Home is forbidden: %+v", wIn, st)
		}
	}
	// a colonist is no exception: inside Home allowed, outside forbidden
	col, _ := g.SpawnNamedPawn(15, 10, "Bo")
	far, _ := g.SpawnNamedPawn(2, 2, "Cy")
	g.die(col, health.CauseInjury)
	g.die(far, health.CauseInjury)
	if _, cb, ok := g.bodyOf(col); !ok || cb.Forbidden {
		t.Fatalf("a colonist's body inside Home is allowed: %+v %v", cb, ok)
	}
	if _, cb, ok := g.bodyOf(far); !ok || !cb.Forbidden {
		t.Fatalf("a colonist's body outside Home is forbidden: %+v %v", cb, ok)
	}
}

// STK-AL: a carcass outside Home starts forbidden unless its hunter or a
// predator takes the kill.
func TestCarcassForbiddenOutsideHome(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	g.paintHome(0, 0, 9, 11, true)
	carcass := func(x, y int) item.Stack {
		for _, id := range g.itemIDs {
			if st, _ := g.stacks.Get(id); st.Kind == item.CorpseAnimal && st.X == x && st.Y == y {
				return st
			}
		}
		t.Fatalf("no carcass at %d,%d", x, y)
		return item.Stack{}
	}
	in, _ := g.SpawnAnimal("deer", 4, 4)
	out, _ := g.SpawnAnimal("deer", 18, 4)
	pin, _ := g.positions.Get(in)
	pout, _ := g.positions.Get(out)
	g.killAnimal(in, 0)
	g.killAnimal(out, 0)
	if carcass(pin.X, pin.Y).Forbidden || !carcass(pout.X, pout.Y).Forbidden {
		t.Fatalf("inside Home allowed, outside forbidden: %+v %+v", carcass(pin.X, pin.Y), carcass(pout.X, pout.Y))
	}
	// a hunter's kill outside Home stays allowed: the hunter carries it
	hunter, _ := g.SpawnNamedPawn(16, 8, "Hu")
	prey, _ := g.SpawnAnimal("deer", 20, 8)
	pp, _ := g.positions.Get(prey)
	g.jobs.Set(hunter, job.Record{Kind: job.Hunt, Target: prey})
	g.killAnimal(prey, hunter)
	if carcass(pp.X, pp.Y).Forbidden {
		t.Fatal("a hunter's kill was forbidden")
	}
	if !killTaken(g, hunter, prey) || killTaken(g, hunter, out) || killTaken(g, 0, prey) {
		t.Fatal("killTaken: the hunter's own prey only")
	}
}

// STK-AL: map loot starts forbidden and found; Haul things allows and marks
// it, and allows a forbidden colony stack without marking it.
func TestHaulThingsAllows(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	g.spawnUnclaimed(item.Steel, 10, 3, 3, true)
	loot := g.itemAt(3, 3)
	own, _ := g.SpawnStack(item.Wood, 5, 6, 3)
	g.setForbidden(own, true)
	g.Enqueue(command.Command{Kind: command.DesignateHaul, X: 0, Y: 0, X2: 8, Y2: 5, Enabled: true})
	g.Step(1)
	if st, _ := g.stacks.Get(loot); st.Forbidden || !st.HaulMarked || !st.Unclaimed {
		t.Fatalf("loot after Haul things: %+v", st)
	}
	if st, _ := g.stacks.Get(own); st.Forbidden || st.HaulMarked {
		t.Fatalf("the colony's stack is allowed, unmarked: %+v", st)
	}
	// the item's Mark for hauling allows too
	g.spawnUnclaimed(item.Silver, 7, 10, 3, true)
	coin := g.itemAt(10, 3)
	g.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(coin), Enabled: true})
	g.Step(1)
	if st, _ := g.stacks.Get(coin); st.Forbidden || !st.HaulMarked {
		t.Fatalf("marked loot: %+v", st)
	}
}

// STK-AL: an order on a forbidden stack allows it.
func TestOrderAllowsTarget(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		o    command.Order
		kind item.Kind
	}{{command.OrderEat, item.Berries}, {command.OrderEquip, item.Knife}, {command.OrderHaul, item.Wood}} {
		g := New(Config{Seed: 1, Width: 16, Height: 8})
		ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
		g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 1, X2: 14, Y2: 3})
		sid, _ := g.SpawnStack(c.kind, 3, 6, 4)
		g.setForbidden(sid, true)
		g.Step(1)
		g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: c.o, X: 6, Y: 4})
		g.Step(1)
		if st, ok := g.stacks.Get(sid); ok && st.Forbidden {
			t.Fatalf("order %v left its %s forbidden: %+v", c.o, c.kind, st)
		}
	}
}

// STK-AL: a binge ignores forbidding; a calm colonist does not.
func TestBingeIgnoresForbidden(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	sid, _ := g.SpawnStack(item.Berries, 10, 6, 4)
	g.setForbidden(sid, true)
	if best, _, _, _ := g.bestFood(ada, ada); best != 0 {
		t.Fatalf("a calm colonist picked forbidden food %d", best)
	}
	g.breaks[ada] = breakState{kind: mentalbreak.FoodBinge, until: 1 << 40}
	if best, _, _, _ := g.bestFood(ada, ada); best != sid {
		t.Fatalf("a binge picked %d, want the forbidden berries %d", best, sid)
	}
}

func TestDesignateForbidArea(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	in1, _ := g.SpawnStack(item.Wood, 5, 2, 2)
	in2, _ := g.SpawnStack(item.Berries, 5, 4, 3)
	out, _ := g.SpawnStack(item.Wood, 5, 8, 6)
	g.Enqueue(command.Command{Kind: command.DesignateForbid, X: 4, Y: 4, X2: 1, Y2: 1, Enabled: true})
	g.Step(1)
	for _, c := range []struct {
		id   entity.EntityID
		want bool
	}{{in1, true}, {in2, true}, {out, false}} {
		if st, _ := g.stacks.Get(c.id); st.Forbidden != c.want {
			t.Fatalf("stack %d at %d,%d forbidden %v, want %v", c.id, st.X, st.Y, st.Forbidden, c.want)
		}
	}
	g.Enqueue(command.Command{Kind: command.DesignateForbid, X: 1, Y: 1, X2: 3, Y2: 3, Enabled: false})
	g.Step(1)
	if st, _ := g.stacks.Get(in1); st.Forbidden {
		t.Fatal("Allow area clears it")
	}
	if st, _ := g.stacks.Get(in2); !st.Forbidden {
		t.Fatal("outside the Allow area it stays forbidden")
	}
}

func TestForbidMergeRules(t *testing.T) {
	t.Parallel()
	a := item.Stack{Kind: item.Wood, Qty: 3, X: 1, Y: 1}
	b := a
	b.Forbidden = true
	if item.CanMerge(a, b) {
		t.Fatal("a forbidden stack does not merge into an allowed one")
	}
	if c := b; !item.CanMerge(b, c) {
		t.Fatal("two forbidden stacks merge")
	}
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	f1, _ := g.SpawnStack(item.Wood, 3, 4, 4)
	g.setForbidden(f1, true)
	// forbidden wood set down on it joins it; allowed wood spills beside
	f2, _ := g.SpawnStack(item.Wood, 2, 4, 4)
	g.setForbidden(f2, true)
	if st, _ := g.stacks.Get(f1); st.Qty != 5 || !st.Forbidden {
		t.Fatalf("forbidden stacks merge: %+v", st)
	}
	g.dropStack(item.Wood, 4, 4, 4)
	if st, _ := g.stacks.Get(f1); st.Qty != 5 {
		t.Fatalf("allowed wood merged into the forbidden stack: %+v", st)
	}
	allowed := 0
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); !st.Forbidden && st.Kind == item.Wood {
			allowed += st.Qty
			if st.X == 4 && st.Y == 4 {
				t.Fatal("the allowed wood shares the forbidden stack's tile")
			}
		}
	}
	if allowed != 4 {
		t.Fatalf("allowed wood = %d, want 4", allowed)
	}
	if g.StockOf("wood") != 4 {
		t.Fatalf("StockOf counts free wood only: %d", g.StockOf("wood"))
	}
	// a carried stack can't be forbidden
	st, _ := g.stacks.Get(f1)
	st.Carrier, st.Forbidden = 99, false
	g.putStack(f1, st)
	g.setForbidden(f1, true)
	if st, _ := g.stacks.Get(f1); st.Forbidden {
		t.Fatal("a carried stack was forbidden")
	}
}
