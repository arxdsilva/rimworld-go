package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Moving built objects (UI-011): MoveBuilding places a reinstall frame
// (Frame.Reinstall = the building) at the destination; builders work it
// like any blueprint (no materials), and on completion the same building
// entity moves there, keeping its bills, fuel, contents, owners and HP.

// CanMoveTo reports whether building bid may be reinstalled at x, y facing
// facing, and why not ("can't move this", "blocked", "out of bounds",
// "already moving" ...). Its own cells count as free.
func (g *Game) CanMoveTo(bid entity.EntityID, facing string, x, y int) (bool, string) {
	return false, "not implemented" // UI-011 W3
}

// moveBuilding plans the move of bid (a second plan replaces the first).
func (g *Game) moveBuilding(bid entity.EntityID, f construction.Facing, x, y int) {} // UI-011 W3

// completeMove finishes reinstall frame id: the building moves there.
func (g *Game) completeMove(id entity.EntityID, fr construction.Frame) {} // UI-011 W3

// movingTo is where building bid's planned move goes, nil when none.
func (g *Game) movingTo(bid entity.EntityID) *[2]int { return nil } // UI-011 W3
