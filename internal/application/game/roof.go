package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
)

// Roofs (SIM-018): per-tile roofs that keep rooms warm and keep the sky
// (sun, moon, rain and snow) off. Closed rooms of at most
// roof.AutoMaxCells cells are planned automatically; the player plans
// more with DesignateRoof and takes roofs down with DesignateRemoveRoof.

// RoofCollapseView is a recent roof collapse, for presentation (dust).
type RoofCollapseView struct {
	X, Y int
	Tick uint64
}

// roofCollapse is a collapse stamp, kept for an hour.
type roofCollapse struct {
	x, y int
	tick uint64
}

// roofAt is the roof cell at (x, y); out of bounds is no roof.
func (g *Game) roofAt(x, y int) roof.Cell {
	if !g.grid.InBounds(x, y) || len(g.roofs) == 0 {
		return 0
	}
	return g.roofs[y*g.grid.Width()+x]
}

// underSky reports a tile open to the sky: no roof over it. Walls and
// doors count as covered.
func (g *Game) underSky(x, y int) bool {
	return g.roomAt(x, y) < 0 // SIM-018 W1: roof-aware
}

// roomRoofed is the roofed share of room r's cells, in permille.
func (g *Game) roomRoofed(r int) int {
	return 1000 // SIM-018 W1: count roofed cells
}

// afterRoomsRebuilt runs after every room rebuild: support is recomputed
// and closed rooms are planned for roofs.
func (g *Game) afterRoomsRebuilt() {
	g.refreshRoofSupport()
	g.autoRoof()
}

// refreshRoofSupport recomputes the Supported flag of every tile.
func (g *Game) refreshRoofSupport() {} // SIM-018 W1

// autoRoof plans roofs over closed rooms of at most roof.AutoMaxCells.
func (g *Game) autoRoof() {} // SIM-018 W1

// designateRoof plans roofs over (build) or roof removal from (!build)
// the rectangle.
func (g *Game) designateRoof(x1, y1, x2, y2 int, build bool) {} // SIM-018 W1

// clearRoofPlans drops roof plans inside the (normalized) rectangle.
func (g *Game) clearRoofPlans(minX, minY, maxX, maxY int) {} // SIM-018 W1

// assignRoof gives builder id the nearest roof job, if any.
func (g *Game) assignRoof(id entity.EntityID) bool { return false } // SIM-018 W1

// doRoof advances a Roof or Unroof job.
func (g *Game) doRoof(id entity.EntityID, j job.Record) {} // SIM-018 W1

// checkRoofSupport collapses unsupported built roofs (climate interval).
func (g *Game) checkRoofSupport() {} // SIM-018 W1

// roofOnMined leaves a natural roof where rock was mined away.
func (g *Game) roofOnMined(x, y int) {} // SIM-018 W1

// fillRoofView copies roofs, collapses and the sun clock into the view.
func (g *Game) fillRoofView(w *WorldView) {} // SIM-018 W1

// exportRoofs is the roof grid for a save.
func (g *Game) exportRoofs() []uint8 { return nil } // SIM-018 W1

// importRoofs installs a saved roof grid; nil (an older save) leaves
// every room to be auto-roofed.
func (g *Game) importRoofs(cells []uint8) {} // SIM-018 W1

// RoofRooms completes every planned roof at once (dev scenarios, tests).
func (g *Game) RoofRooms() {} // SIM-018 W1
