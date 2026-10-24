package game

import "rimworld-go/internal/domain/sim/entity"

// Allowed areas (ENG-003, M9): g.areas is the colony's area set; nil until
// the first area is painted. Saved as Areas.

// inArea reports whether pawn id may go to (x, y) under its allowed area.
// The stub allows everywhere.
func (g *Game) inArea(id entity.EntityID, x, y int) bool {
	return true // TODO(M9)
}
