// Package plant is map plants: wild trees and berry bushes, and sown crops.
// Growth is an integer tick counter; a plant is ripe at GrowTicks.
package plant

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
)

// Species is a plant kind.
type Species uint8

const (
	BerryBush Species = iota
	Tree
	// Farm crops (sown in growing zones).
	Potato
	Corn
	Wheat
	Beans
	Lettuce
	Radish
	Pumpkin
	numSpecies
)

// Def is the static definition of a species.
type Def struct {
	Species     Species
	Name        string
	GrowTicks   int // ticks from sown to ripe at 100% fertility
	HarvestWork int // ticks to harvest when ripe
	Yield       int // food units from a ripe harvest
	Yields      item.Kind
	CutWork     int  // ticks to cut down
	Wood        int  // wood from cutting a fully grown plant
	Regrows     bool // harvest resets growth instead of removing the plant
	RegrowTo    int  // growth percent after a regrowing harvest
	Crop        bool // sown in growing zones
	// Sens is how strongly soil fertility changes growth speed, percent:
	// 100 = fully (corn on gravel crawls), 30 = barely (potatoes thrive
	// anywhere). Zero means 100.
	Sens    int
	SowWork int    // ticks to sow one cell
	Label   string // player-facing produce name
}

// Plants grow only in daylight, GrowFromHour to GrowToHour.
const (
	GrowFromHour = 6
	GrowToHour   = 20
	growHours    = GrowToHour - GrowFromHour
)

// GrowingHour reports whether plants grow at this hour of the day.
func GrowingHour(h int) bool { return h >= GrowFromHour && h < GrowToHour }

// growDays converts calendar days to daylight growth ticks on plain soil.
func growDays(days float64) int {
	return int(days * float64(clock.TicksPerDay) * growHours / 24)
}

var defs = [numSpecies]Def{
	BerryBush: {Species: BerryBush, Name: "berry_bush", GrowTicks: 12000, HarvestWork: 30, Yield: 5, Yields: item.Berries,
		CutWork: 20, Regrows: true, RegrowTo: 35},
	Tree: {Species: Tree, Name: "tree", GrowTicks: 60000, CutWork: 90, Wood: 8},
	// Crops: growth follows real days-to-harvest at roughly twelve real
	// days per game day; yields are per plant, balanced so a colonist
	// eats from about three tiles of potatoes.
	//            real days | game days | yield | soil sensitivity
	//   radish        25        2          3        40%
	//   lettuce       45        3.75       4        90%
	//   beans         60        5          4        60%
	//   potato        90        7.5        8        30%
	//   corn          95        8          7       100%
	//   pumpkin      100        8.5       12        70%
	//   wheat        120       10          8        80%
	Potato: {Species: Potato, Name: "potato", Label: "potatoes", GrowTicks: growDays(7.5), HarvestWork: 35, Yield: 8,
		Yields: item.Potatoes, CutWork: 10, Crop: true, Sens: 30, SowWork: 30},
	Corn: {Species: Corn, Name: "corn", Label: "corn", GrowTicks: growDays(8), HarvestWork: 40, Yield: 7,
		Yields: item.Corn, CutWork: 14, Crop: true, Sens: 100, SowWork: 25},
	Wheat: {Species: Wheat, Name: "wheat", Label: "wheat", GrowTicks: growDays(10), HarvestWork: 35, Yield: 8,
		Yields: item.Wheat, CutWork: 10, Crop: true, Sens: 80, SowWork: 20},
	Beans: {Species: Beans, Name: "beans", Label: "beans", GrowTicks: growDays(5), HarvestWork: 30, Yield: 4,
		Yields: item.Beans, CutWork: 10, Crop: true, Sens: 60, SowWork: 25},
	Lettuce: {Species: Lettuce, Name: "lettuce", Label: "lettuce", GrowTicks: growDays(3.75), HarvestWork: 18, Yield: 4,
		Yields: item.Lettuce, CutWork: 8, Crop: true, Sens: 90, SowWork: 20},
	Radish: {Species: Radish, Name: "radish", Label: "radishes", GrowTicks: growDays(2), HarvestWork: 15, Yield: 3,
		Yields: item.Radishes, CutWork: 8, Crop: true, Sens: 40, SowWork: 15},
	Pumpkin: {Species: Pumpkin, Name: "pumpkin", Label: "pumpkins", GrowTicks: growDays(8.5), HarvestWork: 30, Yield: 12,
		Yields: item.Pumpkins, CutWork: 12, Crop: true, Sens: 70, SowWork: 30},
}

// Crops lists the farm crops in menu order (fastest first).
func Crops() []Species { return []Species{Radish, Lettuce, Beans, Potato, Corn, Pumpkin, Wheat} }

// Rate is the growth speed in percent on soil of the given fertility
// (percent), scaled by how sensitive species s is to soil.
func Rate(s Species, fertility int) int {
	if fertility <= 0 {
		return 0
	}
	sens := DefOf(s).Sens
	if sens == 0 {
		sens = 100
	}
	return 100 + (fertility-100)*sens/100
}

// GrowDays is the calendar days species s takes on soil of this
// fertility (growing only in daylight).
func GrowDays(s Species, fertility int) float64 {
	r := Rate(s, fertility)
	if r <= 0 {
		return 0
	}
	return float64(DefOf(s).GrowTicks) * 100 / float64(r) / (float64(clock.TicksPerDay) * growHours / 24)
}

// DefOf returns the definition for s.
func DefOf(s Species) Def {
	if s >= numSpecies {
		return Def{}
	}
	return defs[s]
}

func (s Species) String() string { return DefOf(s).Name }

// Parse returns the species for a name.
func Parse(name string) (Species, bool) {
	for _, d := range defs {
		if d.Name == name {
			return d.Species, true
		}
	}
	return 0, false
}

// Legacy constants kept for older fixtures.
const (
	HarvestWork  = 30
	HarvestYield = 5
)

// Plant is a plant on the map.
type Plant struct {
	Species    Species
	X, Y       int
	Growth     int  // 0..GrowTicks
	Designated bool // marked for harvest
	Cut        bool // marked for cutting
	Zone       bool // sown by a growing zone
}

// Wild is the historical name for Plant.
type Wild = Plant

// Ripe reports the plant can be harvested (or a tree is fully grown).
func (p Plant) Ripe() bool {
	d := DefOf(p.Species)
	return d.GrowTicks > 0 && p.Growth >= d.GrowTicks
}

// Harvestable is true for ripe food plants.
func (p Plant) Harvestable() bool { return p.Ripe() && DefOf(p.Species).Yield > 0 }

// Percent is growth in 0..100.
func (p Plant) Percent() int {
	d := DefOf(p.Species)
	if d.GrowTicks <= 0 {
		return 100
	}
	v := p.Growth * 100 / d.GrowTicks
	if v > 100 {
		return 100
	}
	return v
}

// Stage is the visual/gameplay growth stage 0 (sown) .. 3 (ripe).
func (p Plant) Stage() int {
	switch pc := p.Percent(); {
	case pc >= 100:
		return 3
	case pc >= 55:
		return 2
	case pc >= 20:
		return 1
	default:
		return 0
	}
}

// WoodYield is the wood from cutting p now (scaled by growth).
func (p Plant) WoodYield() int {
	return DefOf(p.Species).Wood * p.Percent() / 100
}

// Grow advances growth by one tick at fertility percent. Growth uses an
// integer accumulator so it stays deterministic.
func (p Plant) Grow(fertility int, tick uint64) Plant {
	d := DefOf(p.Species)
	if fertility <= 0 || p.Growth >= d.GrowTicks {
		return p
	}
	// fertility 100 -> +1 per tick, 140 -> +1.4 per tick on average,
	// using the integer difference of a running total (deterministic).
	if tick == 0 {
		return p
	}
	f := uint64(fertility)
	p.Growth += int(tick*f/100 - (tick-1)*f/100)
	if p.Growth > d.GrowTicks {
		p.Growth = d.GrowTicks
	}
	return p
}
