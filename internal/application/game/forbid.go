package game

import (
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Forbidding (UI-016): a forbidden stack is left alone by colonists'
// autonomous work until allowed. The work givers' filters live with each
// giver (forbidden()).

// forbidden reports whether stack id is forbidden.
func (g *Game) forbidden(id entity.EntityID) bool {
	st, ok := g.stacks.Get(id)
	return ok && st.Forbidden
}

// setForbidden applies a SetForbidden command.
func (g *Game) setForbidden(id entity.EntityID, on bool) {
	st, ok := g.stacks.Get(id)
	if !ok || st.Carrier != 0 {
		return
	}
	st.Forbidden = on
	g.mergeOrSet(id, st)
}

// markForbidden forbids or allows every ground stack in the rectangle.
func (g *Game) markForbidden(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range append([]entity.EntityID(nil), g.itemIDs...) {
		st, ok := g.stacks.Get(id)
		if ok && st.Carrier == 0 && st.X >= minX && st.X <= maxX && st.Y >= minY && st.Y <= maxY {
			g.setForbidden(id, on)
		}
	}
}

// dropForbidden spawns a forbidden stack (raid loot) at (x, y), merging
// into a forbidden stack of the same kind there.
func (g *Game) dropForbidden(k item.Kind, qty, x, y int) {
	sid, err := g.SpawnStack(k, qty, x, y)
	if err != nil {
		return
	}
	if st, ok := g.stacks.Get(sid); ok {
		st.Forbidden = true
		g.mergeOrSet(sid, st)
	}
}

// forbidRaidBody forbids the body of raider id that just died, so nobody
// hauls it off before the player allows it.
func (g *Game) forbidRaidBody(id entity.EntityID) {
	if !g.isRaider(id) {
		return
	}
	if cid, st, ok := g.bodyOf(id); ok && !st.Forbidden && st.Carrier == 0 {
		st.Forbidden = true
		g.stacks.Set(cid, st)
	}
}
