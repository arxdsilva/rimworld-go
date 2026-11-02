package game

import (
	"rimworld-go/internal/application/game/allow"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Forbidding (UI-016, STK-AL): the rules live in game/allow; each work
// giver filters forbidden stacks itself. allowHost implements allow.Host.
type allowHost struct{ g *Game }

var _ allow.Host = allowHost{}

func (h allowHost) ItemIDs() []entity.EntityID                  { return h.g.itemIDs }
func (h allowHost) Stack(id entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(id) }
func (h allowHost) PutStack(id entity.EntityID, st item.Stack)  { h.g.mergeOrSet(id, st) }
func (h allowHost) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h allowHost) SetBuilding(id entity.EntityID, b construction.Building) { h.g.bldgs.Set(id, b) }
func (h allowHost) InHome(x, y int) bool {
	return h.g.grid.InBounds(x, y) && h.g.inHome(y*h.g.grid.Width()+x)
}
func (h allowHost) Seen(x, y int) bool { return !h.g.fogged(x, y) }

// DoorChanged: DEF-3, a forbidden door is shut to colonists (bridge.go
// cellWalkable).
func (h allowHost) DoorChanged(b construction.Building) {
	h.g.grid.SetWalkable(b.X, b.Y, h.g.cellWalkable(b.X, b.Y))
	h.g.structVer++
}

// forbids is the allow rules over g.
func forbids(g *Game) allow.Forbids { return allow.New(allowHost{g}) }

// setForbidden applies a SetForbidden command (a stack or a building).
func (g *Game) setForbidden(id entity.EntityID, on bool) { forbids(g).Set(id, on) }

// markForbidden forbids or allows every ground stack in the rectangle.
func (g *Game) markForbidden(x1, y1, x2, y2 int, on bool) { forbids(g).Rect(x1, y1, x2, y2, on) }

// killTaken reports a kill its killer takes (no auto-forbid): a predator
// or a hunter on its hunt for prey.
func killTaken(g *Game, by, prey entity.EntityID) bool {
	j, _ := g.jobs.Get(by)
	return g.animals.Ref(by) != nil || j.Kind == job.Hunt && j.Target == prey
}
