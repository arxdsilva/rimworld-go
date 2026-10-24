package game

import "rimworld-go/internal/domain/sim/entity"

// socialOnDeath is called from dieBy before mourn (ENG-003, M7): the dead
// pawn's relations grieve, and they think less of killer (0: none).
func (g *Game) socialOnDeath(dead, killer entity.EntityID) {
	// TODO(M7)
}
