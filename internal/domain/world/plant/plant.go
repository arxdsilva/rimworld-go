// Package plant is map plants: wild trees and berry bushes, and sown crops.
// Growth is an integer tick counter; a plant is ripe at GrowTicks.
package plant

import (
	"strings"

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
	// Duskmoss (MAPGEN-3, append only, saved): blue-grey moss of dark
	// caves; grows without light and withers in it.
	Duskmoss
	// FOOD-0 contract, sown since FOOD-2 (append only, saved): the vanilla
	// crops rice, strawberries, haygrass and hops (wiki numbers). Hops is
	// sown after Brewing (wiki: Hop plant).
	Rice
	Strawberry
	Haygrass
	Hops
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
	// FrostKill is the warmest air temperature (tenths of a degree C)
	// below which a crop takes frost damage, dying within hours
	// (damage.go); each plant's own limit is ColdLimit, up to 8 °C colder
	// (wiki: Plants, -18..-10 °C). Only crops die of frost: wild trees and
	// bushes go dormant.
	FrostKill int
	// ColdKill and HeatKill (tenths of a degree C, 0 = never) damage a
	// wild biome plant out of its climate until it dies (damage.go); the
	// original tree and bush only go dormant.
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
	// Light (MAPGEN-3, percent of full daylight on the cell): MinLight is
	// the light it needs to grow (0: no rule); DiesInLight the light that
	// makes it wither (0: never); Glow the faint light it gives off.
	MinLight, DiesInLight, Glow int
	// Cave: grows wild on cave floor (stone counts as CaveFertility).
	Cave bool
	// FOOD-2: MinSkill is the Plants level a grower needs to sow it;
	// NoHydro keeps it out of a hydroponics basin (corn, haygrass, trees);
	// ColdHardy: a crop whose cold-death limit is its FrostKill exactly
	// (healroot), not a random one below the warmest.
	MinSkill  int
	NoHydro   bool
	ColdHardy bool
}

// CaveFertility is the fertility (percent) stone cave floor gives a Cave
// species (MAPGEN-3); other species get nothing from stone.
const CaveFertility = 40

// ZoneSowable reports whether a growing zone may sow this species.
func (d Def) ZoneSowable() bool { return d.Crop || d.Sowable }

// Plants grow from GrowFromHour to GrowToHour and rest the other hours,
// whatever the light (wiki: Plants, "all plants rest from hour 19 through
// hour 5"): 13 growing hours a day, so a crop's real days are its grow
// days / 0.5417.
const (
	GrowFromHour = 6
	GrowToHour   = 19
	growHours    = GrowToHour - GrowFromHour
)

// GrowingHour reports whether plants grow at this hour of the day.
func GrowingHour(h int) bool { return h >= GrowFromHour && h < GrowToHour }

// wikiDays is a plant's growth units from its wiki grow days: a day of
// growth at 100 % is TicksPerDay units, and growth only accrues in the
// growing hours, so it ripens in days / 0.5417 calendar days.
func wikiDays(days float64) int { return int(days * float64(clock.TicksPerDay)) }

// growDays converts calendar days at the old 14 h daylight rule to growth
// units (the wild plants' original tuning, kept).
func growDays(days float64) int {
	return int(days * float64(clock.TicksPerDay) * 14 / 24)
}

// Field work (wiki: each crop's page). Most crops take 170 ticks to sow
// and 200 to harvest or cut; healroot 800 and 400; trees 4,000 to sow.
const (
	cropSow     = 170 // wiki: Rice plant, Potato plant, Corn plant (sow work)
	cropHarvest = 200 // wiki: Rice plant, Potato plant, Corn plant (harvest work)
	treeSow     = 4000
	treeSkill   = 6 // wiki: Oak tree, Pine tree (minimum Plants skill to sow)
	// coldCrop is the warmest cold-death threshold of a crop, tenths of a
	// degree (wiki: Plants, a random -18..-10 °C per plant).
	coldCrop = -100
)

// crop is a sown crop's definition from its wiki numbers: grow days, base
// yield and fertility sensitivity (percent).
func crop(s Species, name, label string, days float64, yield int, yields item.Kind, sens int) Def {
	return Def{Species: s, Name: name, Label: label, GrowTicks: wikiDays(days), HarvestWork: cropHarvest, Yield: yield,
		Yields: yields, CutWork: cropHarvest, Crop: true, Sens: sens, SowWork: cropSow, FrostKill: coldCrop}
}

// tree is a sowable tree from its wiki numbers: grow days, wood, harvest
// (cut) work; fertility sensitivity 50 % and Plants 6 to sow (wiki: Oak
// tree, Pine tree, Teak tree).
func tree(s Species, name string, days float64, wood, cut int) Def {
	return Def{Species: s, Name: name, GrowTicks: wikiDays(days), CutWork: cut, Wood: wood, Sowable: true,
		SowWork: treeSow, MinSkill: treeSkill, Sens: 50, NoHydro: true, Trunk: true}
}

// with returns d changed by f (a table row's extras).
func with(d Def, f func(*Def)) Def {
	f(&d)
	return d
}

var defs = [numSpecies]Def{
	// wiki: Berry bush (6 grow days, 10 berries, 250 harvest work, back to
	// 30 % when picked, sensitivity 50 %).
	BerryBush: {Species: BerryBush, Name: "berry_bush", GrowTicks: wikiDays(6), HarvestWork: 250, Yield: 10, Yields: item.Berries,
		CutWork: 250, Regrows: true, RegrowTo: 30, Browse: true, Sens: 50},
	// Trees (wiki: Plants tree table): the oak-like tree 30 days and 46
	// wood, pine 20 and 27, the jungle tree as teak 32.5 and 60.
	Tree:       tree(Tree, "tree", 30, 46, 1400),
	Pine:       with(tree(Pine, "pine", 20, 27, 800), func(d *Def) { d.HeatKill = 400 }),
	JungleTree: with(tree(JungleTree, "jungle_tree", 32.5, 60, 1800), func(d *Def) { d.ColdKill = 20 }),
	// wiki: Saguaro cactus (5 grow days, 15 wood, 300 work, no fertility
	// sensitivity).
	Cactus: {Species: Cactus, Name: "cactus", GrowTicks: wikiDays(5), CutWork: 300, Wood: 15, ColdKill: -50, Trunk: true, Sens: 1},
	// Shrub and reeds are hardy enough for every biome they grow wild in
	// (tundra, cold bog), blizzards included. Our numbers.
	Shrub: {Species: Shrub, Name: "shrub", GrowTicks: 30000, CutWork: 25, Wood: 3, ColdKill: -400, HeatKill: 650, Browse: true},
	Reeds: {Species: Reeds, Name: "reeds", Label: "straw", GrowTicks: 15000, HarvestWork: 25, Yield: 4, Yields: item.Straw,
		CutWork: 15, Regrows: true, RegrowTo: 30, ColdKill: -300, Browse: true},
	// Wild ground plants (PLAY-010, our numbers): no wood, no yield.
	Bramble: {Species: Bramble, Name: "bramble", GrowTicks: 40000, CutWork: 30, ColdKill: -300, HeatKill: 450,
		Browse: true, WalkCost: 150, Ground: true},
	Moss: {Species: Moss, Name: "moss", GrowTicks: 30000, CutWork: 10, ColdKill: -500, HeatKill: 350,
		Browse: true, Ground: true, MinLight: 35},
	// Duskmoss (MAPGEN-3, our numbers): very slow, catches feet, a faint
	// glow; needs darkness.
	Duskmoss: {Species: Duskmoss, Name: "duskmoss", GrowTicks: growDays(40), CutWork: 12, ColdKill: -400, HeatKill: 450,
		Browse: true, Ground: true, WalkCost: 125, DiesInLight: 45, Glow: 10, Cave: true},
	// wiki: Agave (6 grow days, 10 agave fruit, 300 harvest work, back to
	// 30 % when picked, sensitivity 50 %); never sown.
	Agave: {Species: Agave, Name: "agave", GrowTicks: wikiDays(6), HarvestWork: 300, Yield: 10, Yields: item.AgaveFruit,
		Regrows: true, RegrowTo: 30, CutWork: 25, ColdKill: -50, HeatKill: 650, Ground: true, Sens: 50},
	BallCactus: {Species: BallCactus, Name: "ball_cactus", GrowTicks: 60000, CutWork: 20, ColdKill: -80, HeatKill: 700,
		Ground: true},
	ElephantEar: {Species: ElephantEar, Name: "elephant_ear", GrowTicks: 30000, CutWork: 15, ColdKill: 50, HeatKill: 500,
		Browse: true, WalkCost: 110, Ground: true},
	FireLily: {Species: FireLily, Name: "fire_lily", GrowTicks: 30000, CutWork: 10, HeatKill: 450, Browse: true,
		Ground: true},
	Dandelion: {Species: Dandelion, Name: "dandelion", GrowTicks: 15000, CutWork: 5, ColdKill: -200, HeatKill: 400,
		Browse: true, Ground: true},
	// Crops at the wiki's grow days, base yield and fertility
	// sensitivity (wiki: Plants crop table and each plant's page).
	Potato: crop(Potato, "potato", "potatoes", 5.8, 11, item.Potatoes, 40),
	Corn:   with(crop(Corn, "corn", "corn", 11.3, 22, item.Corn, 100), func(d *Def) { d.NoHydro = true }),
	Rice:   crop(Rice, "rice", "rice", 3, 6, item.Rice, 100),
	Strawberry: with(crop(Strawberry, "strawberry", "strawberries", 4.6, 8, item.Strawberries, 100),
		func(d *Def) { d.MinSkill = 5 }),
	Haygrass: with(crop(Haygrass, "haygrass", "hay", 7, 18, item.Hay, 60), func(d *Def) { d.NoHydro = true }),
	Cotton:   crop(Cotton, "cotton", "cloth", 8, 10, item.Cloth, 100),
	Hops:     with(crop(Hops, "hops", "hops", 5, 8, item.Hops, 70), func(d *Def) { d.MinSkill = 3 }),
	// wiki: Healroot (7 days, 1 medicine, 800 sow and 400 harvest work,
	// Plants 8; survives -29 °C).
	Healroot: with(crop(Healroot, "healroot", "herbal medicine", 7, 1, item.HerbalMedicine, 100), func(d *Def) {
		d.SowWork, d.HarvestWork, d.CutWork, d.MinSkill, d.FrostKill, d.ColdHardy = 800, 400, 400, 8, -290, true
	}),
	// The original crops on vanilla-style numbers (our tuning, about two
	// items a grow day like the wiki's staples; the old soil sensitivity):
	// radish fast, lettuce, beans, pumpkin and wheat slower. Brewgrain grows
	// like wheat; ashleaf stands in for smokeleaf (wiki: Smokeleaf plant,
	// 7.5 days, 9 leaves, Plants 4).
	Radish:  crop(Radish, "radish", "radishes", 3.2, 6, item.Radishes, 40),
	Lettuce: crop(Lettuce, "lettuce", "lettuce", 4, 7, item.Lettuce, 90),
	Beans:   crop(Beans, "beans", "beans", 5.5, 10, item.Beans, 60),
	Pumpkin: crop(Pumpkin, "pumpkin", "pumpkins", 9, 17, item.Pumpkins, 70),
	Wheat: with(crop(Wheat, "wheat", "wheat", 10, 19, item.Wheat, 80),
		func(d *Def) { d.Extra, d.ExtraQty = item.Straw, 4 }),
	Brewgrain: crop(Brewgrain, "brewgrain", "brewgrain", 10.5, 19, item.Brewgrain, 80),
	Ashleaf:   with(crop(Ashleaf, "ashleaf", "ashleaf", 7.5, 9, item.AshleafLeaf, 100), func(d *Def) { d.MinSkill = 4 }),
}

// Growth temperature, tenths of a degree C (wiki: Plants): full speed
// from growFullTemp to growHotTemp, nothing at or below growMinTemp or at
// or above growMaxTemp; below 6 °C the factor is T/6, above 42 °C
// (58 - T)/16.
const (
	growMinTemp  = 0
	growFullTemp = 60
	growHotTemp  = 420
	growMaxTemp  = 580
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

// Frostbitten reports whether a crop of species s is past the warmest
// frost limit of its kind at air temperature tenthsC (each plant's own
// limit is ColdLimit, colder still).
func Frostbitten(s Species, tenthsC int) bool {
	d := DefOf(s)
	return d.Crop && tenthsC < d.FrostKill
}

// Killed reports whether a wild biome plant of species s is out of its
// climate at air temperature tenthsC: it takes cold or heat damage
// (Expose; crops use Frostbitten).
func Killed(s Species, tenthsC int) bool {
	d := DefOf(s)
	return d.ColdKill != 0 && tenthsC < d.ColdKill || d.HeatKill != 0 && tenthsC > d.HeatKill
}

// Crops lists the farm crops in menu order (fastest first); ashleaf,
// brewgrain and hops are research-gated like healroot.
func Crops() []Species {
	return []Species{Rice, Radish, Lettuce, Strawberry, Hops, Beans, Potato, Haygrass, Healroot, Ashleaf, Cotton, Pumpkin,
		Wheat, Brewgrain, Corn}
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

// DisplayName is species s as players read it ("desert rosette", "ground
// moss"): a few wild plants have a descriptive name, the rest their
// definition name with spaces.
func (s Species) DisplayName() string {
	switch s {
	case BerryBush:
		return "berry bush"
	case Bramble:
		return "bramble thicket"
	case Moss:
		return "ground moss"
	case Agave:
		return "desert rosette"
	case Dandelion:
		return "dandelions"
	}
	return strings.ReplaceAll(s.String(), "_", " ")
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
	// Withering: losing growth in harmful light (MAPGEN-3); it stops only
	// below WitherStopLight (hysteresis).
	Withering bool
	// Damage (PLAY-016) is cold or heat damage, 0..100: the plant dies at
	// 100. DamageHeat tells heat from cold (presentation). Damaging: the
	// air is hurting it now; it stops only DamageStopMargin back on the
	// safe side (hysteresis, damage.go).
	Damage     uint8
	DamageHeat bool
	Damaging   bool
	// FOOD-2 (farm.go): Sown is the tick it was sown or first seen growing
	// (0: not yet seen), for its lifespan; Life the share of its lifespan
	// gone and Light the growing light it last had, percent; Blight the
	// blight's severity, percent (0: healthy).
	Sown                uint64
	Life, Light, Blight uint8
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
