package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func queueTestGame(t *testing.T) (*Game, uint64) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	if _, err := g.SpawnStack(item.Rifle, 1, 12, 6); err != nil {
		t.Fatal(err)
	}
	return g, uint64(id)
}

func TestQueuedOrdersRunInSequence(t *testing.T) {
	g, id := queueTestGame(t)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 6, Y: 1})
	g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: id, Order: command.OrderEquip, X: 12, Y: 6, Queue: true})
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 20, Y: 10, Queue: true})
	g.Step(1)
	p := pawn(t, g, uint64ToID(id))
	if len(p.QueuedOrders) != 2 || p.QueuedOrders[0].Label != "Equip rifle" || p.QueuedOrders[1].Label != "Go to (20, 10)" {
		t.Fatalf("queue = %+v", p.QueuedOrders)
	}
	reachedFirst := false
	for i := 0; i < 1200; i++ {
		g.Step(1)
		g.SetNeeds(uint64ToID(id), needs.Full, needs.Full)
		p = pawn(t, g, uint64ToID(id))
		if p.X == 6 && p.Y == 1 {
			reachedFirst = true
		}
		if p.X == 20 && p.Y == 10 {
			break
		}
	}
	if !reachedFirst || p.X != 20 || p.Y != 10 {
		t.Fatalf("at (%d,%d), passed first stop %v: should end at the last queued stop", p.X, p.Y, reachedFirst)
	}
	if g.Equipped(uint64ToID(id)) != "rifle" {
		t.Fatalf("should have picked up the rifle on the way, has %q", g.Equipped(uint64ToID(id)))
	}
	if len(p.QueuedOrders) != 0 {
		t.Fatalf("queue should be empty: %+v", p.QueuedOrders)
	}
}

func TestQueueLimitAndPlainOrderReplaces(t *testing.T) {
	g, id := queueTestGame(t)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 22, Y: 10})
	for i := 0; i < 5; i++ {
		g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 2 + i, Y: 8, Queue: true})
	}
	g.Step(1)
	if n := len(pawn(t, g, uint64ToID(id)).QueuedOrders); n != MaxQueuedOrders {
		t.Fatalf("queued = %d, want the limit %d", n, MaxQueuedOrders)
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 3, Y: 3})
	g.Step(1)
	if n := len(pawn(t, g, uint64ToID(id)).QueuedOrders); n != 0 {
		t.Fatalf("a plain order should clear the queue, %d left", n)
	}
}

func TestImpossibleQueuedOrderIsSkipped(t *testing.T) {
	g, id := queueTestGame(t)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 4, Y: 1})
	g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: id, Order: command.OrderEquip, X: 12, Y: 6, Queue: true})
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: 8, Y: 3, Queue: true})
	g.Step(1)
	// someone else takes the rifle before Ada gets to it
	for _, iid := range g.itemIDs {
		g.deleteItem(iid)
	}
	reached := false
	for i := 0; i < 400 && !reached; i++ {
		g.Step(1)
		g.SetNeeds(uint64ToID(id), needs.Full, needs.Full)
		p := pawn(t, g, uint64ToID(id))
		reached = p.X == 8 && p.Y == 3
	}
	if !reached {
		t.Fatal("should skip the gone rifle and go on to (8,3)")
	}
	msgs := g.World().Messages
	if len(msgs) == 0 || msgs[len(msgs)-1].Text != `Ada skipped "Equip rifle": `+ReasonGone {
		t.Fatalf("messages = %+v", msgs)
	}
}

func uint64ToID(v uint64) entity.EntityID { return entity.EntityID(v) }
