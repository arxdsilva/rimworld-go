// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"

	"rimworld-go/internal/application/game/orders"
	"rimworld-go/internal/application/game/viewmodel"
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
type QueuedOrderView = viewmodel.QueuedOrderView

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
		if g.execOrder(id, &c) == "" {
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
		if reason := g.execOrder(id, &q.cmd); reason != "" {
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

// replaceOrder runs an explicit order without losing the existing queue
// on refusal. Interrupt mode resumes the previous player command after
// any continuation of the new order.
func (g *Game) replaceOrder(id entity.EntityID, c command.Command) {
	old := g.orderQueue[id]
	prev, following := g.postponed.orders[id]
	following = following && g.followingOrder(id)
	resume := c.Mode == command.PrioritizeInterrupt
	c.Mode = 0
	delete(g.orderQueue, id)
	if g.execOrder(id, &c) != "" {
		if following && !g.followingOrder(id) {
			delete(g.postponed.orders, id)
			old = append([]queuedOrder{prev}, old...)
		}
		if len(old) > 0 {
			g.orderQueue[id] = old
		}
		return
	}
	g.trackOrder(id, queuedOrder{cmd: c})
	if resume {
		if following {
			g.orderQueue[id] = append(g.orderQueue[id], prev)
		}
		g.orderQueue[id] = append(g.orderQueue[id], old...)
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
func (g *Game) execOrder(id entity.EntityID, c *command.Command) string {
	if !g.alive(id) || g.breaking(id) {
		return ReasonIncapable
	}
	switch c.Kind {
	case command.MovePawn:
		if !g.grid.Walkable(c.X, c.Y) {
			return ReasonBlocked
		}
		if !g.reachable(id, c.X, c.Y) {
			return ReasonUnreachable
		}
		g.cmdMove(id, c.X, c.Y)
	case command.EmergencyTend:
		if reason := g.care.Doctor.CanEmergencyTend(id, entity.EntityID(c.Target)); reason != "" {
			return reason
		}
		g.care.Doctor.EmergencyTend(id, entity.EntityID(c.Target))
	case command.DropGear:
		return senseOf(g).Gear.Drop(id, int(c.Mode), c.Index) // game/sense/drop.go (GEAR-1)
	case command.Prioritize:
		if c.Order.NativeOrder() {
			return nativeOrder(g, id, c)
		}
		if c.Order.TargetsPawn() {
			if reason := g.care.Force(id, c.Order, entity.EntityID(c.Target)); reason != "" {
				return reason
			}
			break
		}
		if _, reason := g.forcedJob(id, c.Order, c.X, c.Y, construction.Layer(c.Layer)); reason != "" {
			return reason
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
	if label := orders.DirectLabel(c); label != "" {
		return label
	}
	for _, o := range g.OrdersAt(id, c.X, c.Y) {
		if o.Cmd.Kind == c.Kind && o.Cmd.Order == c.Order && o.Cmd.Layer == c.Layer && o.Cmd.Target == c.Target {
			return o.Label
		}
	}
	return fmt.Sprintf("Order at (%d, %d)", c.X, c.Y)
}

// nativeOrder routes explicit orders to their existing gameplay owners.
func nativeOrder(g *Game, id entity.EntityID, c *command.Command) string {
	if !g.colonist(id) || g.prison.Is(id) || !g.standingPawn(id) {
		return ReasonIncapable
	}
	m, _ := g.labor.Get(id)
	n := orders.Native{Host: nativeOrderHost{haulHost{g}}, Claims: g.reserve, Craft: g.craft, Gear: senseOf(g).Gear}
	tail, why := n.Start(id, m&^g.incapableWork(id), c)
	if tail != "" {
		next := *c
		next.Name, next.Queue, next.Mode = tail, false, 0
		g.orderQueue[id] = append([]queuedOrder{{cmd: next, label: orders.DirectLabel(next)}}, g.orderQueue[id]...)
	}
	return why
}

type nativeOrderHost struct{ haulHost }

func (h nativeOrderHost) Interrupt(id entity.EntityID) { h.g.interrupt(id, job.Cancelled) }

// Allow preserves the exact physical identity already claimed by this order.
func (h nativeOrderHost) Allow(id entity.EntityID) {
	if st, ok := h.g.stacks.Get(id); ok && st.Carrier == 0 && st.Forbidden {
		st.Forbidden = false
		h.g.putStack(id, st)
	}
}
func (h nativeOrderHost) StartForced(id entity.EntityID, rec job.Record) bool {
	// A deferred long search must not destroy the current job/carry/claims.
	rec.Forced = true
	bypass := h.g.areaBypass(id, rec)
	path, ok := h.g.pathToJob(id, rec.TX, rec.TY)
	if bypass {
		h.g.endAreaBypass()
	}
	if !ok {
		return false
	}
	h.Interrupt(id)
	if startJobWithPath(h.g, id, rec, path, true) {
		return true
	}
	h.g.reserve.ReleaseOwner(id)
	return false
}
