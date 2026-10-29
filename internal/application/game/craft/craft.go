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
	Bills    *Bills    // bills.go: every bench's bills, the bill commands and the read-model lines
	Shop     *Shop     // shop.go: finding and doing bill work
	Drying   *Drying   // drying.go: drying racks
	Cabinets *Cabinets // cabinet.go: the tool-cabinet bonus
}

// New builds the production cluster on host h.
func New(h Host) *Craft {
	c := &Craft{
		Bills:    &Bills{h: h, list: make(map[entity.EntityID][]recipe.Bill)},
		Drying:   &Drying{h: h, racks: make(map[entity.EntityID]Batch)},
		Cabinets: &Cabinets{h: h},
	}
	c.Bills.drying = c.Drying
	c.Shop = &Shop{h: h, bills: c.Bills, drying: c.Drying, cabinets: c.Cabinets}
	return c
}

// ElectricSpeedPct is how much faster an electric bench works than its
// base kind (PLAY-021, our number).
const ElectricSpeedPct = 40

// ElectricBench reports a bench that runs on power (PLAY-021): the
// electric stove, tailoring bench and research bench. It works only while
// powered, ElectricSpeedPct faster.
func ElectricBench(k construction.Kind) bool { return power.SpecOf(k).Class == power.ClassBench }

// ElectricPct is kind k's extra work speed in percent (0 but for an
// electric bench).
func ElectricPct(k construction.Kind) int {
	if ElectricBench(k) {
		return ElectricSpeedPct
	}
	return 0
}
