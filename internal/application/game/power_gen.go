package game

import (
	"math"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// PLAY-021 generator outputs (FUELGEN owns this file): what each producer
// makes right now, in integer W. The balance (power.go) adds them up per
// net and applies the switches and the solar flare; genOutput is the
// potential output of the piece itself.
//
//   - wood-fired: 800 W while it holds fuel;
//   - solar: up to 900 W following daylight (lighting.Outdoor, zero at the
//     night floor), times the sky's share of the sun (weather.SunFactor,
//     the factor the renderer dims daylight by) and the share of its
//     footprint not roofed;
//   - wind: 250 W per unit of wind speed, at most 1100 W, times the clear
//     share of a band beside the turbine;
//   - geothermal: 1800 W, always.
//
// Floats become permille once per call and the rest is integer maths.
const (
	woodGenW  = 800
	solarMaxW = 900
	// solarNightPermil is lighting.Outdoor's night floor (0.12): the sun
	// gives nothing at or below it.
	solarNightPermil = 120
	windPerSpeedW    = 250
	windMaxW         = 1100
	// windBandDepth is how many cells wide the band beside each long side
	// of a turbine is.
	windBandDepth = 5
	geothermalW   = 1800
)

// genOutput is generator id's output in W (0 for a piece that makes none,
// or one that is gone).
func (g *Game) genOutput(id entity.EntityID) int {
	b, ok := g.bldgs.Get(id)
	if !ok {
		return 0
	}
	switch b.Kind {
	case construction.WoodGenerator:
		if g.fuel[id] > 0 {
			return woodGenW
		}
	case construction.GeothermalGenerator:
		return geothermalW
	case construction.SolarPanel:
		return g.solarOutput(b)
	case construction.WindTurbine:
		return g.windOutput(id, b)
	}
	return 0
}

// permil turns a fraction into rounded permille.
func permil(f float64) int { return int(math.Round(f * 1000)) }

// solarOutput is the panel's output: the sun above the night floor, scaled
// by the weather's share of the sun and the share of its footprint open
// to the sky (walls and roofs cover).
func (g *Game) solarOutput(b construction.Building) int {
	sun := max(0, permil(float64(skyLight(g)))-solarNightPermil) // light.go: none in an eclipse (FOOD-6)
	if sun == 0 {
		return 0
	}
	w := solarMaxW * sun / (1000 - solarNightPermil)
	w = w * int(weather.SunFactor(g.weather.Kind, g.weather.Intensity)) / 1000
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if len(cells) == 0 {
		return 0
	}
	sky := 0
	for _, c := range cells {
		if g.underSky(c[0], c[1]) {
			sky++
		}
	}
	return w * (sky * 1000 / len(cells)) / 1000
}

// windOutput is the turbine's output: the wind, times the clear share of
// its band.
func (g *Game) windOutput(id entity.EntityID, b construction.Building) int {
	speed := max(0, permil(g.weather.WindSpeed)) // thousandths of a tile per second
	w := min(windMaxW, windPerSpeedW*speed/1000)
	if w == 0 {
		return 0
	}
	return w * g.windClear(id, b) / 1000
}

// windCache holds each turbine's clear share (permille), valid for one
// structure, roof state and game hour (trees grow, so a band is looked at
// again every hour). Never saved.
type windCache struct {
	ok      bool
	structV uint64
	roofV   uint64
	hour    uint64
	bldgs   any // the store it was built from (a load swaps it)
	share   map[entity.EntityID]int
	builds  int // band scans so far (tests)
}

// windClear is the share (permille) of turbine id's band that is open air.
func (g *Game) windClear(id entity.EntityID, b construction.Building) int {
	c := &g.vents.wind
	hour := uint64(g.clock.Now()) / clock.TicksPerHour
	if !c.ok || c.structV != g.structVer || c.roofV != g.roofVer() || c.hour != hour || c.bldgs != any(g.bldgs) {
		clear(c.share)
		if c.share == nil {
			c.share = map[entity.EntityID]int{}
		}
		c.ok, c.structV, c.roofV, c.hour, c.bldgs = true, g.structVer, g.roofVer(), hour, g.bldgs
	}
	if s, ok := c.share[id]; ok {
		return s
	}
	s := g.scanWindBand(b)
	c.share[id] = s
	c.builds++
	return s
}

// scanWindBand counts the open cells of the band along both long sides of
// turbine b: windBandDepth cells out from each side, over the turbine's
// length. Cells off the map are open air.
func (g *Game) scanWindBand(b construction.Building) int {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if len(cells) == 0 {
		return 0
	}
	x0, y0, x1, y1 := cells[0][0], cells[0][1], cells[0][0], cells[0][1]
	for _, c := range cells[1:] {
		x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
	}
	total, open := 0, 0
	visit := func(x, y int) {
		total++
		if !g.windBlocked(x, y) {
			open++
		}
	}
	if x1-x0 > y1-y0 { // lies east-west: the band is above and below
		for x := x0; x <= x1; x++ {
			for d := 1; d <= windBandDepth; d++ {
				visit(x, y0-d)
				visit(x, y1+d)
			}
		}
	} else { // lies north-south: the band is left and right
		for y := y0; y <= y1; y++ {
			for d := 1; d <= windBandDepth; d++ {
				visit(x0-d, y)
				visit(x1+d, y)
			}
		}
	}
	return open * 1000 / total
}

// windBlocked reports a cell that stops the wind: a roof, rock, a wall or
// door, a building that blocks movement, or a tree. Off the map is open.
func (g *Game) windBlocked(x, y int) bool {
	if !g.grid.InBounds(x, y) {
		return false
	}
	if g.roofAt(x, y).Roofed() || terrain.IsRockWall(g.terr.At(x, y)) || g.layerBlockerAt(x, y) != 0 {
		return true
	}
	if b := g.bldgs.Ref(g.layerBldgAt(x, y, construction.LayerObject)); b != nil && (b.Kind == construction.Wall || b.Kind.IsDoor()) {
		return true
	}
	if p := g.plants.Ref(g.plantAt(x, y)); p != nil {
		return plant.DefOf(p.Species).Trunk && p.Species != plant.Cactus
	}
	return false
}
