package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Order queue: the player can stack up to MaxQueuedOrders orders behind a
// colonist's current one ("go here, then equip that rifle, then go
// there"). Each queued order is re-checked when its turn comes and is
// skipped, with a message, if it no longer makes sense. A plain order
// replaces the queue; bedtime pauses it until morning.

// MaxQueuedOrders is how many orders can wait behind the current one.
const MaxQueuedOrders = 3

type queuedOrder struct {
	cmd   command.Command
	label string
	// retryAt: a blocked order is not started again before this tick;
	// tries counts its blocked attempts (SIM-027, postpone.go).
	retryAt uint64
	tries   int
}

// QueuedOrderView is one waiting order in the read model.
type QueuedOrderView struct {
	Label string
	X, Y  int
}

// followingOrder reports a colonist busy with a player order.
func (g *Game) followingOrder(id entity.EntityID) bool {
	j, _ := g.jobs.Get(id)
	return j.Forced && j.Active()
}

// queueOrder runs c now if the colonist isn't following an order,
// otherwise lines it up behind the current one.
func (g *Game) queueOrder(id entity.EntityID, c command.Command) {
	if !g.alive(id) {
		return
	}
	c.Queue = false
	if !g.followingOrder(id) && len(g.orderQueue[id]) == 0 {
		if g.execOrder(id, c) == "" {
			g.trackOrder(id, queuedOrder{cmd: c})
		}
		return
	}
	if len(g.orderQueue[id]) >= MaxQueuedOrders {
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, "%s already has %d orders queued", g.name(id), MaxQueuedOrders)
		return
	}
	g.orderQueue[id] = append(g.orderQueue[id], queuedOrder{cmd: c, label: g.orderLabel(id, c)})
}

// orderDue reports a queued order ready to start: the first one, unless
// it was blocked and its retry tick has not come (the colonist works
// meanwhile; later orders wait behind it).
func (g *Game) orderDue(id entity.EntityID) bool {
	q := g.orderQueue[id]
	return len(q) > 0 && uint64(g.clock.Now()) >= q[0].retryAt
}

// nextOrder starts the first queued order that is still possible.
func (g *Game) nextOrder(id entity.EntityID) {
	for g.orderDue(id) {
		q := g.orderQueue[id][0]
		g.orderQueue[id] = g.orderQueue[id][1:]
		if reason := g.execOrder(id, q.cmd); reason != "" {
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "%s skipped %q: %s", g.name(id), q.label, reason)
			continue
		}
		g.trackOrder(id, q) // postpone.go: retried if it gets blocked
		break
	}
	if len(g.orderQueue[id]) == 0 {
		delete(g.orderQueue, id)
	}
}

// followPlainOrder records plain order c as the one pawn id follows, once
// it has started a player job (else forgets any earlier one).
func (g *Game) followPlainOrder(id entity.EntityID, c command.Command) {
	if j, _ := g.jobs.Get(id); j.Forced && (j.Active() || j.State == job.Fail) {
		g.trackOrder(id, queuedOrder{cmd: c})
		return
	}
	delete(g.postponed.orders, id)
}

// execOrder applies a move or prioritize order now; it returns why the
// order couldn't start ("" when it did).
func (g *Game) execOrder(id entity.EntityID, c command.Command) string {
	switch c.Kind {
	case command.MovePawn:
		if !g.grid.Walkable(c.X, c.Y) {
			return ReasonBlocked
		}
		if !g.reachable(id, c.X, c.Y) {
			return ReasonUnreachable
		}
		g.cmdMove(id, c.X, c.Y)
	case command.Prioritize:
		if c.Order != command.OrderMine {
			if _, reason := g.forcedJob(id, c.Order, c.X, c.Y, construction.Layer(c.Layer)); reason != "" {
				return reason
			}
		}
		g.prioritize(id, c.Order, c.X, c.Y, construction.Layer(c.Layer))
	}
	if j, _ := g.jobs.Get(id); j.State == job.Fail {
		return j.Fail.String()
	}
	return ""
}

// orderLabel names an order the way the order menu shows it.
func (g *Game) orderLabel(id entity.EntityID, c command.Command) string {
	if c.Kind == command.MovePawn {
		return fmt.Sprintf("Go to (%d, %d)", c.X, c.Y)
	}
	for _, o := range g.OrdersAt(id, c.X, c.Y) {
		if o.Cmd.Kind == c.Kind && o.Cmd.Order == c.Order && o.Cmd.Layer == c.Layer {
			return o.Label
		}
	}
	return fmt.Sprintf("Order at (%d, %d)", c.X, c.Y)
}
