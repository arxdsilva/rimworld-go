package architect

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Reinstall (AX-1): a crated building's Reinstall button runs Move mode on
// the crate. The ghost is the packed building in its material; the rule
// is the Architect's own placement rule (Sim.CanPlaceAt) and a left click
// sends command.Reinstall, which lays the blueprint a builder carries the
// crate to.

// StartCrate enters Move mode for crated building id (an item that is not
// a crate is ignored).
func (m *Mover) StartCrate(id entity.EntityID) {
	it, ok := m.crate(id)
	if !ok {
		return
	}
	s := &m.st
	s.On, s.Bid, s.Crate, s.Facing, s.Fresh = true, 0, it.ID, 0, true
	m.ToolID = ""
	m.Drag = false
}

// crate is the crated building id on the map.
func (m *Mover) crate(id entity.EntityID) (game.ItemView, bool) {
	if m.sim() == nil {
		return game.ItemView{}, false
	}
	for _, it := range m.sh.World().Items {
		if it.ID == id && it.AX.Building != "" {
			return it, true
		}
	}
	return game.ItemView{}, false
}

// CrateGhost is the building a crate places, standing where the crate lies
// (its ID is the crate's).
func CrateGhost(it game.ItemView) game.BuildingView {
	b := game.BuildingView{ID: it.ID, Kind: it.AX.Building, Stuff: it.Stuff, X: it.X, Y: it.Y, W: 1, H: 1}
	if bv, ok := Buildable(it.AX.Building); ok {
		b.W, b.H = bv.W, bv.H
	}
	return b
}

// ReinstallCmd is the command a click at (x, y) sends for crate.
func ReinstallCmd(crate entity.EntityID, facing uint8, x, y int) command.Command {
	return command.Command{Kind: command.Reinstall, Target: uint64(crate), X: x, Y: y, Facing: facing}
}
