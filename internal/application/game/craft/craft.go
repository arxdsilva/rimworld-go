package craft

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// Craft is the production cluster's root: the game builds one with New
// and keeps one field. Its components are exported so the game reaches
// each by name (g.craft.Bills.Apply, g.craft.Shop.Do).
type Craft struct {
	Bills  *Bills  // bills.go: every bench's bills, the bill commands and the read-model lines
	Shop   *Shop   // shop.go: finding and doing bill work
	Drying *Drying // drying.go: drying racks
	// PRD-3: linked facilities (facility.go), bench work speed and view
	// lines (station.go), sculptures (art.go).
	Facilities *Facilities
	Stations   *Stations
	Art        *Art
	// PRD-2: quality rolls (quality.go); rot, deterioration and worn
	// apparel wear (condition.go).
	Quality   *Quality
	Condition *Condition
}

// New builds the production cluster on host h.
func New(h Host) *Craft {
	c := &Craft{
		Bills:   &Bills{h: h, p: planner{h}, list: make(map[entity.EntityID][]recipe.Bill)},
		Drying:  &Drying{h: h, racks: make(map[entity.EntityID]Batch)},
		Quality: &Quality{h: h},
	}
	c.Condition = &Condition{h: h}
	c.Bills.drying = c.Drying
	c.Facilities = &Facilities{h: h}
	c.Art = &Art{h: h, quality: c.Quality, pieces: map[entity.EntityID]Artwork{}}
	c.Stations = &Stations{h: h, facilities: c.Facilities, art: c.Art}
	c.Shop = &Shop{h: h, bills: c.Bills, drying: c.Drying, stations: c.Stations, art: c.Art, quality: c.Quality, p: planner{h}}
	return c
}

// ElectricSpeedPct is how much faster an electric twin works than its
// base kind (PLAY-021, our number).
const ElectricSpeedPct = 40

// ElectricBench reports a bench that runs on power (PLAY-021, PRD-3): the
// electric stove, tailoring and research benches, the electric smithy,
// the machining table, the fabrication bench, the smelter and the
// refinery. It works only while powered.
func ElectricBench(k construction.Kind) bool { return power.SpecOf(k).Class == power.ClassBench }

// ElectricPct is kind k's extra work speed in percent: ElectricSpeedPct
// for an electric twin of a plain or fueled bench (recipe.Base), 0 for
// any other kind (a machining table has no slower twin).
func ElectricPct(k construction.Kind) int {
	if ElectricBench(k) && recipe.Base(k) != k {
		return ElectricSpeedPct
	}
	return 0
}

// WorkPerWood is the forge work one log of fuel lasts (wiki: Fueled
// stove, one log per 375 ticks of work; the fueled smithy burns the
// same; logs and work both scaled / 10).
const WorkPerWood = 375

// Fueled reports a bench that works only with fuel in it: the campfire
// (it burns on its own), the wood-fired forge and the fueled stove
// (FOOD-4, wiki: Fueled stove: 50 wood, scaled / 10 as the forge's), which
// burn while worked (BurnsWhileWorked).
func Fueled(k construction.Kind) bool {
	return k == construction.Campfire || BurnsWhileWorked(k)
}

// BurnsWhileWorked reports a bench that burns one log per WorkPerWood
// work done at it: the fueled smithy and the fueled stove.
func BurnsWhileWorked(k construction.Kind) bool {
	return k == construction.FueledSmithy || k == construction.Stove
}
