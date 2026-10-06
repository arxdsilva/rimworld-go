package game

import (
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Dining: a hungry colonist picks up the portions it needs and carries
// them to a nearby table, preferring a chair pulled up to it, instead of
// eating where the food lies.

// tableReach is how far (tiles, food to seat) a colonist carries a meal to
// sit down with it.
const tableReach = 30

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

// tableSeat is the best free place to eat near (fx, fy): a chair next to a
// table, else any free floor beside one.
func (g *Game) tableSeat(id entity.EntityID, fx, fy int) (int, int, bool) {
	type seat struct{ x, y, score int }
	var seats []seat
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.Table {
			continue
		}
		for _, d := range [4][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}} {
			x, y := b.X+d[0], b.Y+d[1]
			if !g.grid.Walkable(x, y) || g.bedCovering(x, y) {
				continue
			}
			if owner, taken := g.reserve.TileOwner(x, y); taken && owner != id {
				continue
			}
			s := pathing.Dist(fx, fy, x, y)
			if s > tableReach {
				continue
			}
			obj := g.layerBldgAt(x, y, construction.LayerObject)
			if c, ok := g.bldgs.Get(obj); ok && c.Kind == construction.Chair {
				s -= 6 // a chair is worth a few extra steps
			} else if obj != 0 {
				continue // something else stands there
			}
			seats = append(seats, seat{x, y, s})
		}
	}
	sort.Slice(seats, func(i, j int) bool {
		if seats[i].score != seats[j].score {
			return seats[i].score < seats[j].score
		}
		return seats[i].y*g.grid.Width()+seats[i].x < seats[j].y*g.grid.Width()+seats[j].x
	})
	for i, s := range seats {
		if i >= 8 {
			break
		}
		if g.reachable(id, s.x, s.y) {
			return s.x, s.y, true
		}
	}
	return 0, 0, false
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
	sx, sy, ok := g.tableSeat(id, st.X, st.Y)
	if !ok {
		return false
	}
	path, ok := g.pathTo(id, sx, sy)
	if !ok {
		return false
	}
	n, _ := g.need.Get(id)
	take := min(st.Qty, portionsFor(n.Hunger, st.Kind))
	carry := j.Target
	if st.Qty > take {
		st.Qty -= take
		g.stacks.Set(j.Target, st)
		carry = g.entities.Next()
		g.itemIDs = append(g.itemIDs, carry)
		cs := st
		cs.Qty = take
		cs.Carrier = id
		g.stacks.Set(carry, cs)
	} else {
		st.Carrier = id
		g.stacks.Set(j.Target, st)
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
