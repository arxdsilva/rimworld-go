package animalview

// WildRow is one wild species on the map, for the Animals tab's Wild
// list (UI checklist 19.4).
type WildRow struct {
	Species     string // key
	Label       string
	Count       int
	Diet        string
	Predator    bool
	TameChance  int // percent per attempt
	Revenge     int // FOOD-3: percent a hurt one turns on its hunter (wiki, the difficulty's)
	Wildness    int // FOOD-3: percent (wiki); MinHandling the Animals level taming needs
	MinHandling int
	Meat        int // per animal
	Hunted      int // marked for hunting
	TameMarked  int // marked for taming
}
