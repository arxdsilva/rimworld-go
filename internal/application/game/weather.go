package game

import (
	"math"

	"rimworld-go/internal/application/game/snow"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// Weather (SIM-016): the sky over open ground, its wind and what it
// leaves behind, plus the light gameplay it causes.

// weatherBlockTicks is one weather block: 6 hours.
const weatherBlockTicks = clock.TicksPerHour * 6

const (
	dousedIntensity = 0.3 // rain or snow at least this hard puts out a fire
)

// weatherSeed is a stable seed for the weather, derived from the saved
// terrain (the Game keeps no seed of its own), so a loaded game continues
// the same sky.
func (g *Game) weatherSeed() uint64 {
	c := &g.seedCache // cached per structVer, which every terrain change bumps
	if c.ok && c.ver == g.structVer {
		if selfCheck() && c.seed != g.scanWeatherSeed() {
			panic("weatherSeed: terrain changed without a structVer bump")
		}
		return c.seed
	}
	c.seed, c.ver, c.ok = g.scanWeatherSeed(), g.structVer, true
	return c.seed
}

func (g *Game) scanWeatherSeed() uint64 {
	h := uint64(0xcbf29ce484222325) ^ uint64(g.terr.W)<<32 ^ uint64(g.terr.H)
	for _, c := range g.terr.Cells {
		h = (h ^ uint64(c)) * 0x100000001b3
	}
	return h
}

// outdoorsAt reports open ground under the sky: no roof over the tile
// (SIM-018; before roofs it meant no room around it).
func (g *Game) outdoorsAt(x, y int) bool { return g.underSky(x, y) }

// phaseWeather moves the weather on: a new block every 6 hours, easing
// and ground build-up every climate interval.
func (g *Game) phaseWeather() {
	now := g.clock.Now()
	sn := g.snowOf()
	sn.SeedDue()      // game/snow: a global depth with no layer yet seeds it
	defer sn.Chunks() // game/snow: per-tile snow, a few chunks a tick, after the step
	if uint64(now)%weatherBlockTicks == 0 && !g.weatherFix {
		mean := float64(g.climate.DailyMean(now)) / 10
		wet := g.weatherWet(biome.DefOf(g.biome).Wetness) // rain and snow odds, percent of temperate (weather_ng1.go: still air)
		g.weather = weather.NextWet(g.weatherSeed(), uint64(now)/weatherBlockTicks, g.weather, mean, g.season(uint64(now)), wet)
	}
	g.stepLightning() // lightning.go (PLAY-015): storms strike
	g.stepFires()     // fire.go (PLAY-015): every fire.StepTicks
	if uint64(now)%climateInterval != 0 {
		return
	}
	tempC := float64(g.climate.Outdoor(now)) / 10
	g.weather = g.weather.Step(climateInterval, tempC, g.season(uint64(now)))
	g.weatherThoughts()
	g.stainDecay() // stains.go: fade, and wash under rain or snow
}

// soaking reports weather that wets whoever is out in it.
func soaking(k weather.Kind) bool { return k == weather.Rain || k == weather.Storm }

// weatherThoughts gives colonists out in the rain a wet mood and those
// out in light snow a snow day.
func (g *Game) weatherThoughts() {
	k := g.weather.Kind
	snowDay := k == weather.Snow && g.weather.Intensity < 0.6
	if !soaking(k) && !snowDay {
		return
	}
	for _, id := range g.pawnIDs {
		if !g.standingPawn(id) {
			continue
		}
		pos, ok := g.positions.Get(id)
		if !ok || !g.outdoorsAt(pos.X, pos.Y) {
			continue
		}
		switch {
		case soaking(k):
			g.remember(id, mood.Soaked)
		case !g.hasMemory(id, mood.SnowDay):
			g.remember(id, mood.SnowDay) // lasts a day: once per day
		}
	}
}

func (g *Game) hasMemory(id entity.EntityID, k mood.Kind) bool {
	now := uint64(g.clock.Now())
	for _, m := range g.memories[id] {
		if m.Kind == k && m.Expires > now {
			return true
		}
	}
	return false
}

// weatherSlow is base step ticks plus what rain, snow or a blizzard adds
// on an outdoor tile, after the ground's own cost (SIM-023: marsh, sand and
// ice are slow unless floored over; plain ground 100 %, a floor 87 %, AX-3).
func (g *Game) weatherSlow(x, y int, base int) int {
	base = g.fenceSlow(x, y, base) // fences.go: climbing over a fence
	if c := g.terr.At(x, y).WalkCost(); c != 100 && g.treadAt(y*g.grid.Width()+x)&treadFloor == 0 {
		base = (base*c + 99) / 100
	}
	base = (base*construction.FloorStepPct(g.bldgs.Get(g.layerBldgAt(x, y, construction.LayerFloor))) + 50) / 100 // AX-3: floors are quicker than soil
	// PLAY-010: a bramble thicket or elephant ears slow the step too
	if c := g.plantWalkCost(x, y); c > 100 && g.treadAt(y*g.grid.Width()+x)&treadFloor == 0 {
		base = (base*c + 99) / 100
	}
	pct := 0.0
	switch g.weather.Kind {
	case weather.Rain:
		pct = 0.15
	case weather.Storm:
		pct = 0.25
	case weather.Blizzard:
		pct = 0.40
	}
	if g.snowOf().At(x, y) > snow.Deep { // game/snow: this cell's own snow (PLAY-015)
		pct = math.Max(pct, 0.20)
	}
	if pct == 0 || !g.outdoorsAt(x, y) {
		return base
	}
	return int(math.Ceil(float64(base)*(1+pct) - 1e-9))
}

// weatherTempDelta is the weather's effect on outdoor air at (x, y).
func (g *Game) weatherTempDelta(x, y int) temperature.Temp {
	var d temperature.Temp
	switch g.weather.Kind {
	case weather.Snow:
		d = -20
	case weather.Blizzard:
		d = -50
	default:
		return 0
	}
	if !g.outdoorsAt(x, y) {
		return 0
	}
	return d
}

// doused reports an outdoor campfire put out by rain or snow for now.
func (g *Game) doused(bid entity.EntityID) bool {
	if !g.weather.Kind.Falling() || g.weather.Intensity < dousedIntensity {
		return false
	}
	b, ok := g.bldgs.Get(bid)
	return ok && g.outdoorsAt(b.X, b.Y)
}

// SetWeather forces the weather (dev scenarios).
func (g *Game) SetWeather(k weather.Kind, intensity float64) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.weather.Kind, g.weather.Target = k, k
	g.weather.Intensity, g.weather.Goal = intensity, intensity
}

// HoldWeather keeps the current weather kind through block changes (dev
// scenarios such as day-cycle clips; not saved).
func (g *Game) HoldWeather() {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.weatherFix = true
}

// Weather is the current outdoor weather.
func (g *Game) Weather() weather.State { return g.weather }

// fillWorldWeather copies the weather into the world view.
func (g *Game) fillWorldWeather(w *WorldView) {
	s := g.weather
	w.Weather, w.WeatherIntensity = s.Kind.String(), s.Intensity
	w.WindX, w.WindY = s.WindSpeed*math.Cos(s.WindDir), s.WindSpeed*math.Sin(s.WindDir)
	w.SnowDepth, w.Wetness = s.SnowDepth, s.Wetness
}

// fillPawnWeather sets PawnView.Soaked.
func (g *Game) fillPawnWeather(pv *PawnView) {
	pv.Soaked = soaking(g.weather.Kind) && !pv.Embodied && g.outdoorsAt(pv.X, pv.Y)
}

// frostMargin is how close to its frost limit (tenths of a degree C) a
// crop starts to droop.
const frostMargin = 20

// fillPlantWeather sets PlantView.Wilted: an outdoor crop in air close to
// its frost limit, or one that cannot grow at this temperature.
func (g *Game) fillPlantWeather(pv *PlantView, p plant.Plant) {
	d := plant.DefOf(p.Species)
	if !d.Crop || !g.outdoorsAt(p.X, p.Y) {
		return
	}
	t := int(g.temperatureAt(p.X, p.Y))
	pv.Wilted = t < d.FrostKill+frostMargin || (!p.Ripe() && plant.TempFactor(t) == 0)
}

// snowState is the per-tile snow depth and the snow clearing area
// (game/snow).
type snowState = snow.State

// snowOf is the snow layer over the current map's state.
func (g *Game) snowOf() snow.Layer {
	return snow.New(snowHost{g}, &g.snow, snow.Steps{Climate: climateInterval, WeatherBlock: weatherBlockTicks})
}

func (g *Game) paintSnowClear(x, y, x2, y2 int, on bool)     { g.snowOf().PaintClear(x, y, x2, y2, on) }
func (g *Game) doClearSnow(id entity.EntityID, j job.Record) { g.snowOf().DoClear(id, j) }
func (g *Game) exportSnow() ([]byte, []uint64)               { return g.snowOf().Export() }
func (g *Game) importSnow(depth []byte, clear []uint64)      { g.snowOf().Import(depth, clear) }

// fillSnowView copies the snow layer and the clearing area into the view.
func (g *Game) fillSnowView(w *WorldView) {
	v := g.snowOf().View()
	if v.Snow != nil {
		w.Snow, w.SnowVer = v.Snow, v.SnowVer
	}
	if v.Clear != nil {
		w.SnowClear, w.SnowClearVer = v.Clear, v.ClearVer
	}
}

// snowHost implements snow.Host (snow/deps.go) over the game. It is only a
// pointer, so handing it to the layer does not allocate.
type snowHost struct{ g *Game }

var _ snow.Host = snowHost{}

func (h snowHost) Now() clock.Tick                       { return h.g.clock.Now() }
func (h snowHost) Weather() weather.State                { return h.g.weather }
func (h snowHost) SetMapDepth(d float64)                 { h.g.weather.SnowDepth = d }
func (h snowHost) WeatherHeld() bool                     { return h.g.weatherFix }
func (h snowHost) Outdoor(t clock.Tick) temperature.Temp { return h.g.climate.Outdoor(t) }
func (h snowHost) Size() (int, int)                      { return h.g.grid.Width(), h.g.grid.Height() }
func (h snowHost) EnsureRooms()                          { h.g.ensureRooms() }
func (h snowHost) Cover() ([]int, []roof.Cell, []terrain.Type) {
	return h.g.roomGrid, h.g.roofs, h.g.terr.Cells
}
func (h snowHost) UnderSky(x, y int) bool        { return h.g.underSky(x, y) }
func (h snowHost) Terrain(x, y int) terrain.Type { return h.g.terr.At(x, y) }
func (h snowHost) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h snowHost) Allowed(id entity.EntityID, x, y int) bool { return h.g.inArea(id, x, y) }
func (h snowHost) PostponedFor(id entity.EntityID, k job.Kind, x, y int) bool {
	return h.g.postponedFor(id, k, 0, x, y)
}
func (h snowHost) PostponeTile(id entity.EntityID, k job.Kind, x, y int) {
	h.g.postponeTile(id, k, x, y)
}
func (h snowHost) ReachableSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.reachableSpot(id, x, y)
}
func (h snowHost) Work(id entity.EntityID) int { return h.g.workProgress(id, skNone, false) }
func (h snowHost) TileTaken(x, y int) bool {
	_, taken := h.g.reserve.TileOwner(x, y)
	return taken
}
func (h snowHost) ClaimTile(id entity.EntityID, x, y int)           { h.g.reserve.TryTile(id, x, y) }
func (h snowHost) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }
func (h snowHost) SetJob(id entity.EntityID, j job.Record)          { h.g.jobs.Set(id, j) }
