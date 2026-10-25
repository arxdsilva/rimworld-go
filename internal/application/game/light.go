package game

import "rimworld-go/internal/domain/world/lighting"

// cellLight is the simulation's light on (x, y) (MAPGEN-3): daylight on
// unroofed cells, else artificial light. Stub: CAVE-PLANTS adds the cached
// artificial light grid (lamps, fires, glowing plants).
func (g *Game) cellLight(x, y int) lighting.Level {
	return lighting.Cell(lighting.Outdoor(g.clock.Now()), g.roofAt(x, y).Roofed(), 0)
}
