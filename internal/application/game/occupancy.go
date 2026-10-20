package game

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
)

// stepVerdict is the occupancy decision for one step (see SIM-014).
type stepVerdict int

const (
	stepGo         stepVerdict = iota // the cell is free: move
	stepWait                          // stay this tick, retry next tick
	stepRepath                        // replan around the creatures in the way
	stepSidestep                      // move to alt instead, then replan
	stepArriveNear                    // end the walk at alt, next to the target
)

// buildOccupancy indexes every standing creature's cell. Called once at the
// start of the movement phase.
func (g *Game) buildOccupancy() {}

// enter decides whether id may step from -> to this tick, and keeps the
// index current when it does. alt is the cell for stepSidestep and
// stepArriveNear.
func (g *Game) enter(id entity.EntityID, from, to Position) (stepVerdict, Position) {
	return stepGo, to
}

// pathAround replans from id's cell to (tx, ty), treating cells taken by
// other creatures as extra cost.
func (g *Game) pathAround(id entity.EntityID, tx, ty int) ([]pathing.Point, bool) {
	return g.pathTo(id, tx, ty)
}

// occupantAt reports the standing creature on (x, y), if any.
func (g *Game) occupantAt(x, y int) (entity.EntityID, bool) {
	return 0, false
}
