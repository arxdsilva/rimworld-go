package room

// SurgeryCleanliness is the cleanliness (RimWorld cleanliness x100) a room
// of contents c and tally t holding filthSum dirt plus stain offers
// surgery: -2 per filth permille a cell on average, -100 for a room left
// on bare ground, plus its floors' own cleanliness (AX-3: sterile tile
// +0.6), never below -500. The filth and bare-ground terms are original
// tuning of how our filth maps onto the wiki's cleanliness.
func SurgeryCleanliness(c Contents, t Tally, filthSum int) int32 {
	cells := max(c.Cells, 1)
	clean := -2*filthSum/cells - 100*(cells-min(c.Floored, cells))/cells + t.CleanHundredths/cells
	return int32(max(clean, -500))
}
