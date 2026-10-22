package game

// WildlifeRow is one wild species on the map, for the Animals tab's Wild
// list (UI checklist 19.4).
type WildlifeRow struct {
	Species    string // key
	Label      string
	Count      int
	Diet       string
	Predator   bool
	TameChance int // percent per attempt
	Meat       int // per animal
	Hunted     int // marked for hunting
	TameMarked int // marked for taming
}

// Wildlife lists the wild species on the map, most numerous first.
func (g *Game) Wildlife() []WildlifeRow {
	return nil // SIM-024: filled by the animal ecology work
}
