package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
)

// freeTime lets everyone do as they please all day, so work does not compete.
func freeTime(g *Game) {
	for h := range 24 {
		g.setSchedule(0, h, schedule.Recreation)
	}
}

// loose is the quantity of k lying free on the map.
func loose(g *Game, k item.Kind) int {
	n := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k && st.Carrier == 0 {
			n += st.Qty
		}
	}
	return n
}

func TestUnloadMarkedInTheGame(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	freeTime(g)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 14, Y: 14, X2: 16, Y2: 16})
	packed(t, g, ada, pile, 12)
	g.SetNeeds(ada, needs.Full, needs.Full)
	id := int(senseOf(g).Gear.Pack(ada).Items[0].ID)
	g.Enqueue(command.Command{Kind: command.MarkUnload, Pawn: uint64(ada), Mode: 0, Index: id, Enabled: true})
	for i := 0; i < 3000 && senseOf(g).Gear.Has(ada); i++ {
		g.Step(1)
		if inPlay(g, item.Steel) != 20 {
			t.Fatalf("tick %d: %d steel in play, want 20", i, inPlay(g, item.Steel))
		}
	}
	if senseOf(g).Gear.Has(ada) {
		t.Fatal("the marked steel never left the pack")
	}
	g.Step(600) // it walks the load to the stockpile
	if loose(g, item.Steel) != 20 || inPlay(g, item.Steel) != 20 {
		t.Fatalf("%d loose, %d in play: want all 20 on the ground", loose(g, item.Steel), inPlay(g, item.Steel))
	}
	if _, stale := senseOf(g).FromPack[ada]; stale {
		t.Fatal("still recorded as in hand")
	}
}

func TestSaveMidUnload(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	freeTime(g)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 14, Y: 14, X2: 16, Y2: 16})
	packed(t, g, ada, pile, 12)
	g.SetNeeds(ada, needs.Full, needs.Full)
	senseOf(g).Gear.Mark(ada, 0, int(senseOf(g).Gear.Pack(ada).Items[0].ID), true)
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.UnloadPack && j.Carry != 0 {
			break
		}
	}
	if j, _ := g.jobs.Get(ada); j.Kind != job.UnloadPack || j.Carry == 0 {
		t.Fatalf("fixture: job %+v, want an unload with the steel in hand", j)
	}
	hash := g.StateHash()
	var buf bytes.Buffer
	if err := g.Save(&buf, "unload"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got := h.StateHash(); got != hash {
		t.Fatalf("state hash %#x, want %#x", got, hash)
	}
	g.Step(300)
	h.Step(300)
	if a, b := g.StateHash(), h.StateHash(); a != b {
		t.Fatalf("after 300 ticks: %#x, %#x", a, b)
	}
}

func TestEatsFromPackInTheGame(t *testing.T) {
	t.Parallel()
	g, ada, _, _ := packGame(t)
	freeTime(g)
	meal := stack(t, g, item.Meal, 1, 10, 10)
	packed(t, g, ada, meal, 1)
	g.SetNeeds(ada, needs.HungryAt-100, needs.Full)
	for i := 0; i < 1500 && senseOf(g).Gear.Has(ada); i++ {
		g.Step(1)
	}
	g.Step(needs.EatWorkTicks + 50)
	if n, _ := g.need.Get(ada); senseOf(g).Gear.Has(ada) || n.Hunger < needs.HungryAt+2000 || inPlay(g, item.Meal) != 0 {
		t.Fatalf("hunger %d, pack %v, %d meals: want it eaten, the pack empty", n.Hunger, senseOf(g).Gear.Has(ada), inPlay(g, item.Meal))
	}
}

func TestSpareMealInTheGame(t *testing.T) {
	t.Parallel()
	g, ada, _, _ := packGame(t)
	freeTime(g)
	stack(t, g, item.Meal, 5, 12, 12)
	g.SetNeeds(ada, needs.Full, needs.Full)
	for i := 0; i < 1500 && !senseOf(g).Gear.Has(ada); i++ {
		g.Step(1)
	}
	items := senseOf(g).Gear.Pack(ada).Items
	if len(items) != 1 || items[0].Stack.Kind != item.Meal || items[0].Stack.Qty != 1 {
		t.Fatalf("pack %+v, want one spare meal", items)
	}
	g.Step(5000)
	if p := senseOf(g).Gear.Pack(ada); len(p.Items) != 1 || p.Items[0].Stack.Qty != 1 {
		t.Fatalf("pack %+v after a while, want still the one", p)
	}
}
