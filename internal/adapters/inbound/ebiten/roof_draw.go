package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Roof presentation (ART §16.1, UI-010): roofed tiles are lit as indoors
// (light.go), with soft eave shadows inside walls; the Roof overlay (R or
// the HUD button, forced on while a roof tool is held) shows built,
// natural and planned roofs.

// roofBuilt and the rest mirror roof.Cell (internal/domain/world/roof).
const (
	roofBuilt uint8 = 1 << iota
	roofNatural
	roofPlanBuild
	roofPlanRemove
	roofSupported
	roofNoAuto
)

// roofCellAt is the roof flags at (x, y) of w; 0 out of bounds.
func roofCellAt(w game.WorldView, x, y int) uint8 {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return 0
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoofCells) {
		return 0
	}
	return w.RoofCells[i]
}

// roofed reports a built or natural roof.
func roofed(c uint8) bool { return c&(roofBuilt|roofNatural) != 0 }

// roofLabelAt is the hover panel's roof line for (x, y).
func roofLabelAt(w game.WorldView, x, y int) string {
	return "" // ART-010 W5
}

// drawEaves draws the soft shadow inside walls around roofed interiors.
func (r *Renderer) drawEaves(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	// ART-010 W5
}

// roofOverlayOn reports the Roof overlay is showing.
func (r *Renderer) roofOverlayOn() bool { return r.roofOverlay || r.roofOverlayForced }

// drawRoofOverlay draws the Roof overlay when it is on.
func (r *Renderer) drawRoofOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	// ART-010 W5
}
