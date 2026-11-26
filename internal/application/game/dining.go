// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Dining: a hungry colonist picks up the portions it needs and carries
// them to a nearby table, preferring a seat pulled up to it (rest/seats.go
// AtTable), instead of eating where the food lies.

// nextToTable reports a cardinally adjacent eating surface.
func (g *Game) nextToTable(x, y int) bool {
	for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
		bid := g.layerBldgAt(x+d[0], y+d[1], construction.LayerObject)
		if b, ok := g.bldgs.Get(bid); ok && b.Kind.IsTable() {
			return true
		}
	}
	return false
}

// portionsFor is how many items of stack st a pawn at food level hunger
// eats in one sitting (needs.Portions; eggs and carcasses by species).
func portionsFor(hunger int, st item.Stack) int {
	return needs.Portions(hunger, animal.StackNutrition(st), st.Kind.MaxEatAtOnce())
}

// carryToTable starts the walk from the food to a table: it picks up the
// portions it needs and heads for a seat. It returns false (eat here) when
// there is no unreserved reachable chair by a table within search range.
func (g *Game) carryToTable(id entity.EntityID, j job.Record) bool {
	pos, _ := g.positions.Get(id)
	st, ok := g.stacks.Get(j.Target)
	if !ok || st.Carrier != 0 && st.Carrier != id {
		return false
	}
	sx, sy, ok := g.rest.Seats.AtTable(id, pos.X, pos.Y)
	if !ok {
		return false
	}
	path, ok := g.pathTo(id, sx, sy)
	if !ok || !g.reserve.TryTile(id, sx, sy) {
		return false
	}
	if st.Carrier == id {
		j.TX, j.TY, j.DestX, j.DestY = sx, sy, sx, sy
		j.Path, j.PathI = path, 0
		j.WorkLeft = needs.EatWorkTicks
		j.Count = 1
		j.State = job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		g.jobs.Set(id, j)
		return true
	}
	n, _ := g.need.Get(id)
	take := min(st.Qty, portionsFor(n.Hunger, st), g.reserve.FreeFor(id, j.Target, st.Qty))
	if take <= 0 {
		g.reserve.ReleaseTile(sx, sy)
		return false
	}
	carry := j.Target
	if st.Qty > take {
		g.reserve.ReleaseEntity(id, j.Target) // RES-1: picked up, the rest is free for other eaters
		st.Qty -= take
		g.putStack(j.Target, st)
		carry = g.entities.Next()
		g.itemIDs = append(g.itemIDs, carry)
		cs := st
		cs.Qty = take
		cs.Carrier = id
		g.putStack(carry, cs)
	} else {
		st.Carrier = id
		g.putStack(j.Target, st)
	}
	j.Carry, j.Target = carry, carry
	j.TX, j.TY, j.DestX, j.DestY = sx, sy, sx, sy
	j.Path, j.PathI = path, 0
	j.WorkLeft = needs.EatWorkTicks
	j.Count = 1 // carried to a table
	j.State = job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	g.jobs.Set(id, j)
	return true
}
