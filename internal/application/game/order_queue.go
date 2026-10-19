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
		g.execOrder(id, c)
		return
	}
	if len(g.orderQueue[id]) >= MaxQueuedOrders {
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, "%s already has %d orders queued", g.name(id), MaxQueuedOrders)
		return
	}
	g.orderQueue[id] = append(g.orderQueue[id], queuedOrder{cmd: c, label: g.orderLabel(id, c)})
}

// nextOrder starts the first queued order that is still possible.
func (g *Game) nextOrder(id entity.EntityID) {
	for len(g.orderQueue[id]) > 0 {
		q := g.orderQueue[id][0]
		g.orderQueue[id] = g.orderQueue[id][1:]
		if reason := g.execOrder(id, q.cmd); reason != "" {
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "%s skipped %q: %s", g.name(id), q.label, reason)
			continue
		}
		break
	}
	if len(g.orderQueue[id]) == 0 {
		delete(g.orderQueue, id)
	}
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
