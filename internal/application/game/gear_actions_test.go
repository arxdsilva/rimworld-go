package game

import (
	"testing"

	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// GEAR-1: the Gear tab's Drop, Eat and Lock through the game (game/sense/drop.go).

// wornSlot is the slot of the n-th garment id wears.
func wornSlot(t *testing.T, g *Game, id entity.EntityID, n int) item.Slot {
	t.Helper()
	slot, ok := senseGear{g}.Worn(id, n)
	if !ok {
		t.Fatalf("%d wears no garment %d", id, n)
	}
	return item.Slot(slot)
}

func TestDropGearInTheGame(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	packed(t, g, ada, pile, 20)
	entry := int(senseOf(g).Gear.Pack(ada).Items[0].ID)
	g.Enqueue(command.Command{Kind: command.DropGear, Pawn: uint64(ada), Mode: 0, Index: entry})
	g.Step(1)
	if senseOf(g).Gear.Has(ada) || loose(g, item.Steel) != 20 {
		t.Fatalf("the steel should lie at its feet: pack %v, %d loose", senseOf(g).Gear.Has(ada), loose(g, item.Steel))
	}
	worn := len(pawn(t, g, ada).Worn)
	slot := wornSlot(t, g, ada, 0)
	kind := g.apparel[ada].Kind(slot)
	g.Enqueue(command.Command{Kind: command.DropGear, Pawn: uint64(ada), Mode: 1, Index: 0})
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.UnloadPack || !j.Forced {
		t.Fatalf("a garment comes off through a forced job: %+v", j)
	}
	for i := 0; i < 200 && g.apparel[ada].Kind(slot) != 0; i++ {
		g.Step(1)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	if g.apparel[ada].Kind(slot) != 0 || len(pawn(t, g, ada).Worn) != worn-1 || loose(g, kind) != 1 {
		t.Fatalf("the %s should be off and on the ground: %d worn, %d loose", kind, len(pawn(t, g, ada).Worn), loose(g, kind))
	}
}

func TestQueuedDropWaitsItsTurn(t *testing.T) {
	t.Parallel()
	g, id := queueTestGame(t)
	ada := uint64ToID(id)
	pile := stack(t, g, item.Steel, 10, 1, 1)
	packed(t, g, ada, pile, 10)
	entry := int(senseOf(g).Gear.Pack(ada).Items[0].ID)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 8, Y: 1})
	g.Enqueue(command.Command{Kind: command.DropGear, Pawn: id, Mode: 0, Index: entry, Queue: true})
	g.Step(1)
	if q := pawn(t, g, ada).QueuedOrders; len(q) != 1 || q[0].Label != "Drop gear" || !senseOf(g).Gear.Has(ada) {
		t.Fatalf("the drop should wait behind the walk: queue %+v, pack %v", q, senseOf(g).Gear.Has(ada))
	}
	for i := 0; i < 600 && senseOf(g).Gear.Has(ada); i++ {
		g.Step(1)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	p := pawn(t, g, ada)
	if senseOf(g).Gear.Has(ada) || p.X != 8 || p.Y != 1 {
		t.Fatalf("the steel should drop on arrival: at (%d, %d), pack %v", p.X, p.Y, senseOf(g).Gear.Has(ada))
	}
	for _, sid := range g.itemIDs {
		if st, _ := g.stacks.Get(sid); st.Kind == item.Steel && (st.X != 8 || st.Y != 1) {
			t.Fatalf("dropped at (%d, %d), want the walk's end", st.X, st.Y)
		}
	}
	// a plain drop while walking happens now and leaves the queue alone
	pile = stack(t, g, item.Steel, 5, 8, 1)
	packed(t, g, ada, pile, 5)
	entry = int(senseOf(g).Gear.Pack(ada).Items[0].ID)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 20, Y: 1})
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 20, Y: 10, Queue: true})
	g.Enqueue(command.Command{Kind: command.DropGear, Pawn: id, Mode: 0, Index: entry})
	g.Step(1)
	if senseOf(g).Gear.Has(ada) || len(pawn(t, g, ada).QueuedOrders) != 1 {
		t.Fatalf("a plain drop is at once and keeps the queue: pack %v, queue %+v", senseOf(g).Gear.Has(ada), pawn(t, g, ada).QueuedOrders)
	}
}

func TestPrisonerGearDropsAtOnce(t *testing.T) {
	t.Parallel()
	g, ada, pid, _ := packGame(t)
	g.makePrisoner(pid, ada)
	if !g.prison.Is(pid) {
		t.Fatal("fixture: not a prisoner")
	}
	if why := senseOf(g).Gear.DropWhy(pid, sense.ModeAny, 0); why != "" {
		t.Fatalf("a prisoner's gear may be dropped: %q", why)
	}
	slot := wornSlot(t, g, pid, 0)
	g.Enqueue(command.Command{Kind: command.DropGear, Pawn: uint64(pid), Mode: 1, Index: 0})
	g.Step(1)
	if g.apparel[pid].Kind(slot) != 0 {
		t.Fatal("the prisoner's garment should come off at once")
	}
}

func TestGearRefusalsInTheReadModel(t *testing.T) {
	t.Parallel()
	g, ada, _, _ := packGame(t)
	meal := stack(t, g, item.Meal, 1, 10, 11)
	packed(t, g, ada, meal, 1)
	g.SetNeeds(ada, needs.Full, needs.Full)
	p := pawn(t, g, ada)
	if p.Gear.NoDrop != "" || len(p.Pack) != 1 || p.Pack[0].Gear.Why != sense.WhyFull || p.Pack[0].Gear.Grams != item.Meal.MassGrams() {
		t.Fatalf("read model: NoDrop %q, pack %+v", p.Gear.NoDrop, p.Pack)
	}
	g.SetNeeds(ada, needs.HungryAt, needs.Full)
	if p = pawn(t, g, ada); p.Pack[0].Gear.Why != "" {
		t.Fatalf("a hungry colonist may eat: %q", p.Pack[0].Gear.Why)
	}
	g.Enqueue(command.Command{Kind: command.ConsumeFromPack, Pawn: uint64(ada), Index: int(p.Pack[0].ID)})
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Eat || !j.Forced || j.Carry == 0 {
		t.Fatalf("Eat should start a forced meal from the pack: %+v", j)
	}
	g.Step(needs.EatWorkTicks + 20)
	if n, _ := g.need.Get(ada); senseOf(g).Gear.Has(ada) || n.Hunger <= needs.HungryAt+1000 {
		t.Fatalf("the meal should be eaten: hunger %d, pack %v", n.Hunger, senseOf(g).Gear.Has(ada))
	}
}

func TestLockGarmentInTheGame(t *testing.T) {
	t.Parallel()
	g, ada, _, _ := packGame(t)
	slot := wornSlot(t, g, ada, 1)
	g.Enqueue(command.Command{Kind: command.SetApparelForced, Pawn: uint64(ada), Index: 1, Enabled: true})
	g.Step(1)
	if !g.apparel[ada].IsForced(slot) {
		t.Fatal("the garment should be locked")
	}
	g.Enqueue(command.Command{Kind: command.SetApparelForced, Pawn: uint64(ada), Index: 1, Enabled: false})
	g.Step(1)
	if g.apparel[ada].IsForced(slot) {
		t.Fatal("the lock should come off")
	}
}
