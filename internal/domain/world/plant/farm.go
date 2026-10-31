package plant

import "rimworld-go/internal/domain/sim/clock"

// Farming rules at the wiki's numbers (FOOD-2): partial harvests, crop
// lifespans, growing light, hydroponics, the sun lamp's light and blight.

// Harvest (wiki: Plants): a crop may be harvested from HarvestMinPct
// growth, at half its yield, rising linearly to the full yield at 100 %.
const HarvestMinPct = 65

// HarvestableNow reports a food plant grown enough to harvest now (a
// player's harvest mark; growers harvest zone crops when Ripe).
func (p Plant) HarvestableNow() bool {
	return DefOf(p.Species).Yield > 0 && p.Percent() >= HarvestMinPct
}

// HarvestYield is what harvesting (or cutting) p now gives of its base
// yield qty: nothing under HarvestMinPct growth or while blighted, half
// at HarvestMinPct, all of it when ripe; then less for its frost or heat
// damage (DamagedYield).
func (p Plant) HarvestYield(qty int) int {
	pc := p.Percent()
	if qty <= 0 || pc < HarvestMinPct || p.Blight > 0 {
		return 0
	}
	return p.DamagedYield(qty * (50 + 50*(pc-HarvestMinPct)/(100-HarvestMinPct)) / 100)
}

// lifespanFraction is a crop's lifespan in grow days (wiki: Plants,
// "lifespan = grow days x lifespan fraction, usually 5").
const lifespanFraction = 5

// Lifespan is the ticks a crop of species s lives from sowing (0: no
// limit; only crops age to death here).
func Lifespan(s Species) uint64 {
	d := DefOf(s)
	if !d.Crop {
		return 0
	}
	return uint64(d.GrowTicks) * lifespanFraction
}

// LifePct is the share of its lifespan p has lived at tick now, percent
// (0 for a plant with none, or not yet seen).
func (p Plant) LifePct(now uint64) int {
	span := Lifespan(p.Species)
	if span == 0 || p.Sown == 0 || now <= p.Sown {
		return 0
	}
	return int(min((now-p.Sown)*100/span, 100))
}

// Rotted reports p has outlived its lifespan at tick now.
func (p Plant) Rotted(now uint64) bool {
	span := Lifespan(p.Species)
	return span > 0 && p.Sown > 0 && now >= p.Sown+span
}

// ColdLimit is the air temperature (tenths of a degree C) below which p
// takes cold damage, ok false when cold never hurts it. A crop's limit is
// its own, a fixed pseudo-random one between FrostKill and 8 °C colder
// (wiki: Plants, -18..-10 °C), or its FrostKill when ColdHardy.
func (p Plant) ColdLimit() (int, bool) {
	d := DefOf(p.Species)
	if !d.Crop {
		return d.ColdKill, d.ColdKill != 0
	}
	if d.ColdHardy {
		return d.FrostKill, true
	}
	return d.FrostKill - int(Roll(uint64(p.Species), uint64(p.X), uint64(p.Y))%81), true
}

// Growing light (wiki: Plants, Sun lamp), permille of full daylight: a
// plant grows from MinGrowLight (51 %), at full speed at full light.
// Daylight under the open sky counts in full in the growing hours; a
// plain lamp or fire gives at most LampGrowLight (50 %), not enough.
const (
	MinGrowLight  = 510
	FullGrowLight = 1000
	LampGrowLight = 500
)

// LightFactor is the growth speed (permille) at growing light light
// (permille): (light - min) / (full - min), 0 below the minimum.
func LightFactor(light int) int {
	if light < MinGrowLight {
		return 0
	}
	return min((light-MinGrowLight)*1000/(FullGrowLight-MinGrowLight), 1000)
}

// The sun lamp (wiki: Sun lamp): on from 06:00 to 19:12 while powered,
// full light within 5.5 tiles, half to 10 and 30 % to 11.7.
const (
	SunLampFrom   = 6 * clock.TicksPerHour      // 06:00
	SunLampTo     = 19*clock.TicksPerHour + 500 // 19:12
	SunLampReach  = 11                          // tiles, for the light grid and the scan
	sunFull2      = 5*5 + 5                     // 5.5² rounded down: squared tiles at full light
	sunHalf2      = 10 * 10                     // squared tiles at half light
	sunDim2       = 11*11 + 16                  // 11.7²: squared tiles at 30 %
	sunLightFull  = 1000
	sunLightHalf  = 500
	sunLightDim   = 300
	SunLampWatts  = 2900 // wiki: Sun lamp
	BasinWatts    = 70   // wiki: Hydroponics basin
	HydroFertile  = 280  // wiki: Hydroponics basin, soil fertility 280 %
	basinStarve   = 50   // tenths of a percent an hour: dead in 20 h unpowered (our tuning; wiki "wither very quickly")
	BlightStart   = 20   // wiki: Blight, severity it starts at
	BlightSpread  = 28   // wiki: Blight, severity from which it spreads
	BlightRadius  = 4    // wiki: Blight, tiles it spreads over
	BlightPerDay  = 4    // our tuning: points of severity a day (wiki: "progresses slowly")
	BlightMaxDays = 15   // wiki: Blight, only crops of fewer grow days
)

// SunLampOn reports a powered sun lamp shining at tick now.
func SunLampOn(now uint64) bool {
	t := now % clock.TicksPerDay
	return t >= SunLampFrom && t < SunLampTo
}

// SunLampLight is a shining sun lamp's light (permille) dx, dy tiles
// away.
func SunLampLight(dx, dy int) int {
	switch d2 := dx*dx + dy*dy; {
	case d2 <= sunFull2:
		return sunLightFull
	case d2 <= sunHalf2:
		return sunLightHalf
	case d2 <= sunDim2:
		return sunLightDim
	}
	return 0
}

// Hydroponic reports a species a hydroponics basin may grow: a crop not
// kept out (wiki: no corn, haygrass, trees or decorative plants).
func Hydroponic(s Species) bool {
	d := DefOf(s)
	return d.Crop && !d.NoHydro
}

// Starve is p in an unpowered hydroponics basin over ticks t0+1 .. t1: it
// withers by basinStarve an hour (Damage, cold kind), dying at 100.
func (p Plant) Starve(t0, t1 uint64) Plant {
	if t1 > t0 {
		p.Damage = uint8(min(int(p.Damage)+damageSpan(basinStarve, t0, t1), DamageDead))
		p.Damaging, p.DamageHeat = true, false
	}
	return p
}

// Blightable reports a zone-sown crop blight can strike: domesticated,
// fewer than BlightMaxDays grow days (wiki: Blight; trees, wild and
// decorative plants are immune).
func (p Plant) Blightable() bool {
	d := DefOf(p.Species)
	return p.Zone && d.Crop && d.GrowTicks < wikiDays(BlightMaxDays)
}

// Blighten is p's blight over ticks t0+1 .. t1: it worsens BlightPerDay a
// day, at most 100 (a healthy plant stays healthy).
func (p Plant) Blighten(t0, t1 uint64) Plant {
	if p.Blight == 0 || t1 <= t0 {
		return p
	}
	r, per := uint64(BlightPerDay), uint64(clock.TicksPerDay)
	p.Blight = uint8(min(int(p.Blight)+int(t1*r/per-t0*r/per), 100))
	return p
}

// Roll is a deterministic pseudo-random number of its arguments
// (splitmix64): the farm's chances never draw from the simulation's RNG.
func Roll(a, b, c uint64) uint64 {
	z := a*0x9E3779B97F4A7C15 ^ b*0xBF58476D1CE4E5B9 ^ c*0x94D049BB133111EB
	z ^= z >> 30
	z *= 0xBF58476D1CE4E5B9
	z ^= z >> 27
	z *= 0x94D049BB133111EB
	return z ^ z>>31
}

// legacyGrow is each species' growth to ripe before FOOD-2 (14 sun hours a
// day, original tuning), for saves of version 6 and older; 0: unchanged.
var legacyGrow = [numSpecies]int{BerryBush: 12000, Tree: 60000, Pine: 70000, JungleTree: 60000, Cactus: 90000,
	Agave: 50000, Potato: 262500, Corn: 280000, Wheat: 350000, Beans: 175000, Lettuce: 131250, Radish: 70000,
	Pumpkin: 297500, Healroot: 227500, Cotton: 315000, Brewgrain: 385000, Ashleaf: 245000, Rice: 105000,
	Strawberry: 161000, Haygrass: 245000, Hops: 175000}

// FromLegacyGrowth is p from an older save with its growth rescaled to the
// wiki grow days: the same share of the way to ripe.
func (p Plant) FromLegacyGrowth() Plant {
	if old := legacyGrow[p.Species]; old > 0 {
		gt := DefOf(p.Species).GrowTicks
		p.Growth = min(p.Growth*gt/old, gt)
	}
	return p
}
