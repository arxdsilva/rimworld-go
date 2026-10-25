package lighting

// Cell is the light on one map cell for the simulation (plants that need
// light or wither in it, MAPGEN-3): the outdoor daylight on an unroofed
// cell, else only artificial light (lamps, fires, glowing plants), in [0, 1].
func Cell(outdoor Level, roofed bool, artificial float64) Level {
	l := artificial
	if !roofed && float64(outdoor) > l {
		l = float64(outdoor)
	}
	return Level(min(1, max(0, l)))
}
