package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
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
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	sid, _ := g.SpawnStack(item.Berries, 10, 6, 4)
	g.setForbidden(sid, true)
	if g.hasEdible() {
		t.Fatal("forbidden food counts as edible")
	}
	if best, _, _ := g.bestFood(ada); best != 0 {
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

func TestRaidDropsForbidden(t *testing.T) {
	g, _ := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	weapon := g.equip[rid]
	g.die(rid, health.CauseInjury)
	cid, body, ok := g.bodyOf(rid)
	if !ok || !body.Forbidden {
		t.Fatalf("a raider's body starts forbidden: %+v %v", body, ok)
	}
	st, ok := stackOf(g, weapon)
	if !ok || !st.Forbidden {
		t.Fatalf("its %s starts forbidden: %+v %v", weapon, st, ok)
	}
	if iv, _ := itemView(g, cid); !iv.Forbidden {
		t.Fatalf("corpse view %+v", iv)
	}
	// a dead colonist is not forbidden
	var col entity.EntityID
	for _, id := range g.pawnIDs {
		if !g.isRaider(id) {
			col = id
			break
		}
	}
	g.die(col, health.CauseInjury)
	if _, cb, ok := g.bodyOf(col); !ok || cb.Forbidden {
		t.Fatalf("a colonist's body is not forbidden: %+v %v", cb, ok)
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
