package selection

import "rimworld-go/internal/application/game"

// fogged reports tile (x, y) is unexplored in the world view
// (fog_draw.go worldFogged).
func fogged(w game.WorldView, x, y int) bool {
	if len(w.Fog) == 0 || w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return false
	}
	i := y*w.TerrainW + x
	return i < len(w.Fog) && w.Fog[i] != 0
}

// zoneCellAt is the zone holding tile (x, y) (zone_select.go).
func zoneCellAt(w game.WorldView, x, y int) (game.ZoneView, bool) {
	for i := range w.Zones {
		z := &w.Zones[i]
		if x < z.MinX || x > z.MaxX || y < z.MinY || y > z.MaxY {
			continue
		}
		for _, c := range z.Cells {
			if c[0] == x && c[1] == y {
				return *z, true
			}
		}
	}
	return game.ZoneView{}, false
}
