package game

import "rimworld-go/internal/domain/sim/entity"

// Allowed areas (ENG-003, M9): g.areas is the colony's area set; nil until
// the first area is painted. Saved as Areas.

// inArea reports whether pawn id may go to (x, y) under its allowed area.
// The stub allows everywhere.
func (g *Game) inArea(id entity.EntityID, x, y int) bool {
	return true // TODO(M9)
}

// areaViews lists the colony's allowed areas (M9).
func (g *Game) areaViews() []AreaView { return nil }

// cmdPaintArea paints (on) or clears area i over the rectangle (M9).
func (g *Game) cmdPaintArea(i, x, y, x2, y2 int, on bool) {}

// cmdRenameArea renames area i (M9).
func (g *Game) cmdRenameArea(i int, name string) {}

// cmdAssignArea restricts pawn to area i (-1: unrestricted; M9).
func (g *Game) cmdAssignArea(pawn entity.EntityID, i int) {}
