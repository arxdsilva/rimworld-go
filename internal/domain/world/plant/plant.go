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
	// Healroot is a medicinal herb: harvested as herbal medicine.
	Healroot
	// Cotton is a fibre crop: picked as cloth for tailoring and carpets (SIM-021).
	Cotton
	// Wild biome plants (SIM-023): append only, saved.
	Pine
	JungleTree
	Cactus
	Shrub
	Reeds
	// Wild ground plants (PLAY-010): append only, saved.
	Bramble
	Moss
	Agave
	BallCactus
	ElephantEar
	FireLily
	Dandelion
	// Drug crops (ENG-003, append only, saved): brewgrain is brewed into
	// beer, ashleaf into tea and tablets. Neither ever has an outline.
	Brewgrain
	Ashleaf
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
	// Extra is a by-product dropped with every harvest (wheat leaves straw).
	Extra    item.Kind
	ExtraQty int
	CutWork  int  // ticks to cut down
	Wood     int  // wood from cutting a fully grown plant
	Regrows  bool // harvest resets growth instead of removing the plant
	RegrowTo int  // growth percent after a regrowing harvest
	Crop     bool // sown in growing zones, harvested as produce, frost-killed
	Sowable  bool // may be sown in a growing zone though not a crop (trees)
	// Sens is how strongly soil fertility changes growth speed, percent:
	// 100 = fully (corn on gravel crawls), 30 = barely (potatoes thrive
	// anywhere). Zero means 100.
	Sens    int
	SowWork int    // ticks to sow one cell
	Label   string // player-facing produce name
	// FrostKill is the air temperature (tenths of a degree C) below which a
	// crop dies. Tender summer crops die at the first frost; hardy ones
	// shrug off a few degrees. Only crops die of frost: wild trees and bushes
	// go dormant.
	FrostKill int
	// ColdKill and HeatKill (tenths of a degree C, 0 = never) kill a wild
	// biome plant out of its climate; the original tree and bush only go
	// dormant.
	ColdKill, HeatKill int
	// Browse: grazing animals eat it down (bushes, shrubs, reeds). Trunk:
	// a woody plant, chopped for wood and gnawed by tree-eating animals.
	Browse, Trunk bool
	// WalkCost is the step time on this plant's cell, percent (bramble
	// 150 = half again as slow); zero means it doesn't slow walkers.
	WalkCost int
	// Ground: a low ground plant (moss, wildflowers, brambles) that never
	// blocks building; placing a building clears it.
	Ground bool
}

// ZoneSowable reports whether a growing zone may sow this species.
func (d Def) ZoneSowable() bool { return d.Crop || d.Sowable }

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
		CutWork: 20, Regrows: true, RegrowTo: 35, Browse: true},
	Tree: {Species: Tree, Name: "tree", GrowTicks: 60000, CutWork: 90, Wood: 8, Sowable: true, SowWork: 40, Trunk: true},
	// Biome plants (SIM-023, our numbers).
	Pine: {Species: Pine, Name: "pine", GrowTicks: 70000, CutWork: 90, Wood: 10, Sowable: true, SowWork: 40,
		HeatKill: 400, Trunk: true},
	JungleTree: {Species: JungleTree, Name: "jungle_tree", GrowTicks: 60000, CutWork: 100, Wood: 12, Sowable: true,
		SowWork: 40, ColdKill: 20, Trunk: true},
	Cactus: {Species: Cactus, Name: "cactus", GrowTicks: 90000, CutWork: 40, Wood: 2, ColdKill: -50, Trunk: true},
	// Shrub and reeds are hardy enough for every biome they grow wild in
	// (tundra, cold bog), blizzards included.
	Shrub: {Species: Shrub, Name: "shrub", GrowTicks: 30000, CutWork: 25, Wood: 3, ColdKill: -400, HeatKill: 650, Browse: true},
	Reeds: {Species: Reeds, Name: "reeds", Label: "straw", GrowTicks: 15000, HarvestWork: 25, Yield: 4, Yields: item.Straw,
		CutWork: 15, Regrows: true, RegrowTo: 30, ColdKill: -300, Browse: true},
	// Wild ground plants (PLAY-010, our numbers): no wood, no yield.
	Bramble: {Species: Bramble, Name: "bramble", GrowTicks: 40000, CutWork: 30, ColdKill: -300, HeatKill: 450,
		Browse: true, WalkCost: 150, Ground: true},
	Moss: {Species: Moss, Name: "moss", GrowTicks: 30000, CutWork: 10, ColdKill: -500, HeatKill: 350,
		Browse: true, Ground: true},
	Agave: {Species: Agave, Name: "agave", GrowTicks: 50000, CutWork: 25, ColdKill: -50, HeatKill: 650, Ground: true},
	BallCactus: {Species: BallCactus, Name: "ball_cactus", GrowTicks: 60000, CutWork: 20, ColdKill: -80, HeatKill: 700,
		Ground: true},
	ElephantEar: {Species: ElephantEar, Name: "elephant_ear", GrowTicks: 30000, CutWork: 15, ColdKill: 50, HeatKill: 500,
		Browse: true, WalkCost: 110, Ground: true},
	FireLily: {Species: FireLily, Name: "fire_lily", GrowTicks: 30000, CutWork: 10, HeatKill: 450, Browse: true,
		Ground: true},
	Dandelion: {Species: Dandelion, Name: "dandelion", GrowTicks: 15000, CutWork: 5, ColdKill: -200, HeatKill: 400,
		Browse: true, Ground: true},
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
		Yields: item.Potatoes, CutWork: 10, Crop: true, Sens: 30, SowWork: 30, FrostKill: -20},
	Corn: {Species: Corn, Name: "corn", Label: "corn", GrowTicks: growDays(8), HarvestWork: 40, Yield: 7,
		Yields: item.Corn, CutWork: 14, Crop: true, Sens: 100, SowWork: 25, FrostKill: -10},
	Wheat: {Species: Wheat, Name: "wheat", Label: "wheat", GrowTicks: growDays(10), HarvestWork: 35, Yield: 8,
		Yields: item.Wheat, CutWork: 10, Crop: true, Sens: 80, SowWork: 20, FrostKill: -80,
		Extra: item.Straw, ExtraQty: 4},
	Beans: {Species: Beans, Name: "beans", Label: "beans", GrowTicks: growDays(5), HarvestWork: 30, Yield: 4,
		Yields: item.Beans, CutWork: 10, Crop: true, Sens: 60, SowWork: 25, FrostKill: -10},
	Lettuce: {Species: Lettuce, Name: "lettuce", Label: "lettuce", GrowTicks: growDays(3.75), HarvestWork: 18, Yield: 4,
		Yields: item.Lettuce, CutWork: 8, Crop: true, Sens: 90, SowWork: 20, FrostKill: -30},
	Radish: {Species: Radish, Name: "radish", Label: "radishes", GrowTicks: growDays(2), HarvestWork: 15, Yield: 3,
		Yields: item.Radishes, CutWork: 8, Crop: true, Sens: 40, SowWork: 15, FrostKill: -50},
	Pumpkin: {Species: Pumpkin, Name: "pumpkin", Label: "pumpkins", GrowTicks: growDays(8.5), HarvestWork: 30, Yield: 12,
		Yields: item.Pumpkins, CutWork: 12, Crop: true, Sens: 70, SowWork: 30, FrostKill: -10},
	// Healroot: slow and fussy about soil, two doses of medicine a plant.
	Healroot: {Species: Healroot, Name: "healroot", Label: "herbal medicine", GrowTicks: growDays(6.5), HarvestWork: 40,
		Yield: 2, Yields: item.HerbalMedicine, CutWork: 8, Crop: true, Sens: 80, SowWork: 35, FrostKill: -20},
	// Cotton: a warm-season fibre crop; five bolls of cloth a plant.
	Cotton: {Species: Cotton, Name: "cotton", Label: "cloth", GrowTicks: growDays(9), HarvestWork: 40, Yield: 5,
		Yields: item.Cloth, CutWork: 10, Crop: true, Sens: 70, SowWork: 25, FrostKill: -10},
	// ENG-003: brewgrain grows like wheat, ashleaf like healroot.
	Brewgrain: {Species: Brewgrain, Name: "brewgrain", Label: "brewgrain", GrowTicks: growDays(10), HarvestWork: 35, Yield: 8,
		Yields: item.Brewgrain, CutWork: 10, Crop: true, Sens: 80, SowWork: 20, FrostKill: -80},
	Ashleaf: {Species: Ashleaf, Name: "ashleaf", Label: "ashleaf", GrowTicks: growDays(6.5), HarvestWork: 40, Yield: 2,
		Yields: item.AshleafLeaf, CutWork: 8, Crop: true, Sens: 80, SowWork: 35, FrostKill: -20},
}

// Growth temperature, tenths of a degree C: nothing grows at or below
// growMinTemp, growth ramps up to full speed at growFullTemp, holds there
// to growHotTemp and fades out by growMaxTemp in extreme heat.
const (
	growMinTemp  = 0
	growFullTemp = 100
	growHotTemp  = 350
	growMaxTemp  = 450
)

// TempFactor is growth speed in permille at air temperature tenthsC.
func TempFactor(tenthsC int) int {
	switch {
	case tenthsC <= growMinTemp || tenthsC >= growMaxTemp:
		return 0
	case tenthsC < growFullTemp:
		return (tenthsC - growMinTemp) * 1000 / (growFullTemp - growMinTemp)
	case tenthsC <= growHotTemp:
		return 1000
	default:
		return (growMaxTemp - tenthsC) * 1000 / (growMaxTemp - growHotTemp)
	}
}

// Frostbitten reports whether a crop of species s dies at air temperature
// tenthsC.
func Frostbitten(s Species, tenthsC int) bool {
	d := DefOf(s)
	return d.Crop && tenthsC < d.FrostKill
}

// Killed reports whether a wild biome plant of species s dies at air
// temperature tenthsC (crops use Frostbitten).
func Killed(s Species, tenthsC int) bool {
	d := DefOf(s)
	return d.ColdKill != 0 && tenthsC < d.ColdKill || d.HeatKill != 0 && tenthsC > d.HeatKill
}

// Crops lists the farm crops in menu order (fastest first). Brewgrain and
// ashleaf (ENG-003) join the menu with their crop art (ART_DIRECTION
// "Social & host systems asset list"); until then they can't be sown.
func Crops() []Species {
	return []Species{Radish, Lettuce, Beans, Healroot, Potato, Corn, Pumpkin, Cotton, Wheat}
}

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

// GrowSpan is the growth (ticks) a plant growing at rate percent (100:
// one growth tick per tick) gains over the ticks t0+1 .. t1, counting
// only the growing hours: exactly the sum of Grow's per-tick steps over
// those ticks at that rate, so spans telescope — GrowSpan(r, a, b) +
// GrowSpan(r, b, c) == GrowSpan(r, a, c). Growth is then integrated per
// window (one rate for the whole span) instead of tick by tick.
func GrowSpan(rate int, t0, t1 uint64) int {
	if rate <= 0 || t1 <= t0 {
		return 0
	}
	f := uint64(rate)
	total := 0
	for day := t0 / clock.TicksPerDay; day*clock.TicksPerDay <= t1; day++ {
		// the day's growing ticks are (s, e]: GrowFromHour:00 .. GrowToHour:00 - 1
		s := day*clock.TicksPerDay + GrowFromHour*clock.TicksPerHour - 1
		e := day*clock.TicksPerDay + GrowToHour*clock.TicksPerHour - 1
		if lo, hi := max(s, t0), min(e, t1); hi > lo {
			total += int(hi*f/100 - lo*f/100)
		}
	}
	return total
}

// AddGrowth is p with n ticks of growth added (capped at ripe).
func (p Plant) AddGrowth(n int) Plant {
	if n <= 0 {
		return p
	}
	d := DefOf(p.Species)
	if p.Growth >= d.GrowTicks {
		return p
	}
	p.Growth = min(p.Growth+n, d.GrowTicks)
	return p
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
