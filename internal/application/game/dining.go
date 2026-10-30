package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Dining: a hungry colonist picks up the portions it needs and carries
// them to a nearby table, preferring a seat pulled up to it (rest/seats.go
// AtTable), instead of eating where the food lies.

// nextToTable reports a table beside (x, y), diagonals included.
func (g *Game) nextToTable(x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			bid := g.layerBldgAt(x+dx, y+dy, construction.LayerObject)
			if b, ok := g.bldgs.Get(bid); ok && b.Kind == construction.Table {
				return true
			}
		}
	}
	return false
}

// portionsFor is how many items of kind k a colonist eats to be full.
func portionsFor(hunger int, k item.Kind) int {
	nut := k.Nutrition()
	if nut <= 0 {
		return 1
	}
	n := (needs.Full - hunger + nut - 1) / nut
	return max(1, min(12, n))
}

// carryToTable starts the walk from the food to a table: it picks up the
// portions it needs and heads for a seat. It returns false (eat here) when
// there is no table in reach or the colonist already stands at one.
func (g *Game) carryToTable(id entity.EntityID, j job.Record) bool {
	pos, _ := g.positions.Get(id)
	if g.nextToTable(pos.X, pos.Y) {
		return false
	}
	st, ok := g.stacks.Get(j.Target)
	if !ok || st.Carrier != 0 {
		return false
	}
	sx, sy, ok := g.rest.Seats.AtTable(id, st.X, st.Y) // rest/seats.go: a seat by a table
	if !ok {
		return false
	}
	path, ok := g.pathTo(id, sx, sy)
	if !ok {
		return false
	}
	n, _ := g.need.Get(id)
	take := min(st.Qty, portionsFor(n.Hunger, st.Kind), g.reserve.FreeFor(id, j.Target, st.Qty)) // RES-1: never another eater's portions (it holds at least one)
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
	g.reserve.TryTile(id, sx, sy)
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
