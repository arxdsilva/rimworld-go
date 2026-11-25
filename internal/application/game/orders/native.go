// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package orders

import (
	"fmt"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// NativeHost supplies map facts and the usual forced-job interruption,
// reservation and start path. The gameplay owners plan their own jobs.
type NativeHost interface {
	haul.ExplicitHost
	Position(entity.EntityID) (int, int)
	StartForced(entity.EntityID, job.Record) bool
	Allow(entity.EntityID)
}

type Native struct {
	Host   NativeHost
	Claims haul.ExplicitClaims
	Craft  *craft.Craft
	Gear   sense.Gear
}

// Start runs one native order, returning any remaining haul legs for
// the player queue. A refused plan does not interrupt the old job.
func (n Native) Start(id entity.EntityID, m work.Mask, c *command.Command) (tail, why string) {
	h := n.Host
	switch c.Order {
	case command.OrderDoBill:
		if !h.Visible(c.X, c.Y) {
			return "", "unexplored"
		}
		bid, index, ok := n.Craft.Bills.ResolveOrder(*c)
		if !ok {
			return "", "no longer there"
		}
		if !n.Craft.Shop.AssignAt(id, m, bid, index, func(r job.Record) bool { return h.StartForced(id, r) }) {
			return "", "nothing to do at that bench"
		}
	case command.OrderDeliverItem:
		rec, why := n.Gear.Give(id, entity.EntityID(c.Target), c.Index, c.Count)
		if why != "" {
			return "", why
		}
		x, y := h.Position(rec.Dest)
		if !h.Reachable(id, x, y) || !h.StartForced(id, rec) {
			return "", "cannot reach"
		}
	case command.OrderHaulMulti:
		if !m.Enabled(work.Haul) {
			return "", "hauling disabled"
		}
		legs, ok := haul.ParseLegs(c.Name)
		if !ok {
			return "", "invalid haul legs"
		}
		rec, used, why := haul.ExplicitPlan(h, n.Claims, id, legs)
		if why != "" {
			return "", why
		}
		if !h.StartForced(id, rec) {
			return "", "cannot reach"
		}
		h.Allow(rec.Target)
		for i := 0; i < len(rec.Pickups); i += 2 {
			h.Allow(rec.Pickups[i])
		}
		if used == 0 {
			// A large request spans trips. Its physical source remains the
			// same stack; only the quantity of the next leg is reduced.
			first := legs[0]
			first.Count = rec.Count
			c.Name = haul.EncodeLegs([]haul.Leg{first})
			legs[0].Count -= rec.Count
			return haul.EncodeLegs(legs), ""
		}
		if used < len(legs) {
			tail = haul.EncodeLegs(legs[used:])
		}
		c.Name = haul.EncodeLegs(legs[:used]) // blocked retry must not duplicate the tail
	default:
		return "", "no longer there"
	}
	return tail, ""
}

// DirectLabel names orders that do not depend on a tile's right-click menu.
func DirectLabel(c command.Command) string {
	switch c.Kind {
	case command.MovePawn:
		return fmt.Sprintf("Go to (%d, %d)", c.X, c.Y)
	case command.DropGear:
		return "Drop gear"
	case command.Prioritize:
		switch c.Order {
		case command.OrderDoBill:
			return "Work bill"
		case command.OrderDeliverItem:
			return "Hand over item"
		case command.OrderHaulMulti:
			return "Haul items"
		}
	}
	return ""
}
