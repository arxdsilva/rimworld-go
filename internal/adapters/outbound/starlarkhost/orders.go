// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// orderAttrs are the ctx attributes that queue a ScriptOrder or a
// ScriptPlace.
var orderAttrs = []string{"order", "place"}

// orderNames is the closed table of tile orders a script may name; the
// game checks each like a player's prioritize.
var orderNames = map[string]command.Order{
	"build": command.OrderBuild, "supply": command.OrderSupply, "harvest": command.OrderHarvest,
	"sleep": command.OrderSleep, "eat": command.OrderEat, "deconstruct": command.OrderDeconstruct,
	"haul": command.OrderHaul, "cut": command.OrderCut, "mine": command.OrderMine,
	"sow": command.OrderSow, "hunt": command.OrderHunt, "equip": command.OrderEquip,
	"smooth": command.OrderSmooth, "extinguish": command.OrderExtinguish, "flick": command.OrderFlick,
}

// orderAttr is ctx.<name> for order and place; ok is false for a name this
// group does not serve, or one this call may not use (its hook, its
// plugin's capabilities and the event kind are checked here too, whatever
// allowed did).
func (c *call) orderAttr(name string) (starlark.Value, bool) {
	r, ok := ruleOf(name)
	if !ok || !isOneOf(name, orderAttrs) || !c.mayUse(r) {
		return nil, false
	}
	if name == "order" {
		return starlark.NewBuiltin("order", c.order), true
	}
	return starlark.NewBuiltin("place", c.place), true
}

// mayUse reports that r's hook, event kind and capability fit this call.
func (c *call) mayUse(r attrRule) bool {
	if len(r.hooks) > 0 && !isOneOf(c.hook, r.hooks) {
		return false
	}
	if r.byEvent != nil {
		need, ok := r.byEvent[c.ev.Kind]
		return ok && c.caps.Has(need)
	}
	return c.caps.Has(r.cap)
}

// order is ctx.order(pawn_id, order, x, y): the pawn must be one of this
// call's due pawns and the order a name of orderNames, else an error. A
// pawn takes an order only on its order visit, (tick / PawnEvery) %
// OrderEvery == 0: any other visit it is a no-op that returns False, so a
// script gets at most one order per pawn per OrderEvery*PawnEvery ticks
// with no state to keep (a second order for a pawn already ordered in
// this call also returns False). On the visit it queues a ScriptOrder carrying the
// visit's tick and returns True.
func (c *call) order(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var id uint64
	var name string
	var x, y int
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 4, &id, &name, &x, &y); err != nil {
		return nil, err
	}
	o, ok := orderNames[name]
	if !ok {
		return nil, fmt.Errorf("order: unknown order %q", name)
	}
	if !c.due(id) {
		return nil, fmt.Errorf("order: pawn %d is not due in this call", id)
	}
	if (c.ev.Tick/scripting.PawnEvery)%scripting.OrderEvery != 0 {
		return starlark.False, nil
	}
	for _, q := range c.cmds {
		if q.Kind == command.ScriptOrder && q.Pawn == id {
			return starlark.False, nil // one order per pawn per visit
		}
	}
	err := c.queue(command.Command{Kind: command.ScriptOrder, Pawn: id, Order: o, X: x, Y: y, Tick: c.ev.Tick})
	return starlark.Bool(err == nil), err
}

// due reports that pawn id is among this call's due colonists.
func (c *call) due(id uint64) bool {
	for _, p := range c.ev.Pawns {
		if p.ID == id {
			return true
		}
	}
	return false
}

// place is ctx.place(key, count, x, y): count items of key dropped at x, y;
// the game checks the bounds.
func (c *call) place(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var k string
	var n, x, y int
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 4, &k, &n, &x, &y); err != nil {
		return nil, err
	}
	if n < 1 || n > scripting.MaxSpawn {
		return nil, fmt.Errorf("place: count must be 1 to %d, got %d", scripting.MaxSpawn, n)
	}
	if _, ok := c.w.ItemCount(k); !ok {
		return nil, fmt.Errorf("place: unknown item %q", k)
	}
	return starlark.None, c.queue(command.Command{Kind: command.ScriptPlace, Name: k, Count: n, X: x, Y: y})
}
