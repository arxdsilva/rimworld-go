package game

import (
	"math"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/lighting"
	"rimworld-go/internal/domain/world/plant"
)

// The simulation's light (MAPGEN-3): what plants that need light, or
// wither in it, see. The renderer's sprite lighting is separate.
//
// Artificial light per cell comes from lamps while powered (PLAY-021),
// torch lamps while they burn, campfires with fuel and glowing plants
// (duskmoss), each falling off linearly with distance to its radius and
// stopped by rock, walls and doors (the line-of-sight rule bullets use).
// Overlapping lights take the brightest, not the sum. The grid is cached
// and rebuilt only when structVer moves (any building or terrain change:
// lamps and fires come and go with it), ver moves (a glowing plant is
// added or removed, a burner's fuel goes between 0 and above 0, PLAY-017,
// or a lamp gains or loses power, setPowered in power.go), or, while
// there are torch lamps, the rain starts or stops putting them out or a
// roof changes over one; a cell's light is then one read.
//
// The light sources (lamps, torch lamps, campfires) are listed once per
// structure (lightSources). The grid, the renderer's Glow (glowOf) and the
// rooms' Lit (roomViews) read that one list; each decides from it which
// sources shine now.
const (
	lampRadius     = 6
	lampLight      = 0.7
	campfireRadius = 5
	campfireLight  = 0.6
	// torchRadius and torchLight: a torch lamp is a smaller, dimmer light
	// than a lamp (its art's light_radius is 4).
	torchRadius = 4
	torchLight  = 0.5
	glowRadius  = 1
)

// lightGrid is the cached artificial light, never saved.
type lightGrid struct {
	ok        bool
	w, h      int
	structVer uint64
	// ver bumps when a glowing plant is added or removed, a burner lights
	// or burns out (setFuel), or a lamp gains or loses power (setPowered).
	ver, builtVer uint64
	bldgs, plants any // the stores it was built from (a load swaps them)
	// douse and roofV: the rain and roofs the grid was built under, kept
	// only while there are torch lamps (they go out in the rain under the
	// sky), else false and 0.
	douse  bool
	roofV  uint64
	art    []float32
	builds int // rebuilds so far (tests)

	srcs lightSrcList
}

// lightSrcList is the light sources, in building order, per structure
// (structVer, the building count and the store); never saved.
type lightSrcList struct {
	ok      bool
	sv      uint64
	n       int
	bldgs   any
	ids     []entity.EntityID
	torches int // torch lamps among them
	builds  int // rebuilds so far (tests)
}

// isLightSource reports a kind that can give light: a lamp, a torch lamp
// or a campfire.
func isLightSource(k construction.Kind) bool {
	return k == construction.Lamp || k == construction.TorchLamp || k == construction.Campfire || k == construction.SunLamp
}

// lightSources is the cached list of light sources; callers must not keep
// it across a change to the buildings.
func (g *Game) lightSources() *lightSrcList {
	c := &g.light.srcs
	if c.ok && c.sv == g.structVer && c.n == len(g.bldgIDs) && c.bldgs == any(g.bldgs) {
		return c
	}
	c.ids, c.torches = c.ids[:0], 0
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && isLightSource(b.Kind) {
			c.ids = append(c.ids, id)
			if b.Kind == construction.TorchLamp {
				c.torches++
			}
		}
	}
	c.ok, c.sv, c.n, c.bldgs = true, g.structVer, len(g.bldgIDs), g.bldgs
	c.builds++
	return c
}

// shining reports light source b (id) giving light now, with its reach
// and strength: a lamp while powered, a torch lamp or a campfire while it
// burns (fuelled, not doused by rain or snow, a campfire not banked).
func (g *Game) shining(id entity.EntityID, b construction.Building) (bool, int, float64) {
	switch b.Kind {
	case construction.Lamp:
		return g.powered(id), lampRadius, lampLight
	case construction.TorchLamp:
		return g.burning(id, b), torchRadius, torchLight
	case construction.Campfire:
		return g.burning(id, b), campfireRadius, campfireLight
	case construction.SunLamp: // FOOD-2: plant_light.go bumps the grid at its hours
		return g.powered(id) && plant.SunLampOn(uint64(g.clock.Now())), plant.SunLampReach, 1
	}
	return false, 0, 0
}

// rainDouses reports rain or snow heavy enough to put out a fire under
// the sky (weather.go doused, without the cell).
func (g *Game) rainDouses() bool {
	return g.weather.Kind.Falling() && g.weather.Intensity >= dousedIntensity
}

// cellLight is the simulation's light on (x, y): daylight on unroofed
// cells, the brightest artificial light anywhere.
func (g *Game) cellLight(x, y int) lighting.Level {
	return lighting.Cell(skyLight(g), g.roofAt(x, y).Roofed(), g.artificialLight(x, y))
}

// skyLight is the daylight now, none in an eclipse (FOOD-6, wiki: Eclipse).
func skyLight(g *Game) lighting.Level {
	if g.incidents.Eclipsed(uint64(g.clock.Now())) {
		return 0
	}
	return lighting.Outdoor(g.clock.Now())
}

// cellLightPermil is cellLight in permille (rounded), for the plant rules.
func (g *Game) cellLightPermil(x, y int) int {
	return int(math.Round(float64(g.cellLight(x, y)) * 1000))
}

// artificialLight is the cached artificial light on (x, y).
func (g *Game) artificialLight(x, y int) float64 {
	if !g.grid.InBounds(x, y) {
		return 0
	}
	l := g.lightCells()
	return float64(l[y*g.grid.Width()+x])
}

// plantLightChanged records a plant of species s added or removed: a
// glowing one moves the light grid.
func (g *Game) plantLightChanged(s plant.Species) {
	if plant.DefOf(s).Glow > 0 {
		g.light.ver++
	}
}

// setFuel sets burner bid's fuel to v, moving the light grid when the fire
// lights (0 to above 0) or burns out. Every write to g.fuel goes through it.
func (g *Game) setFuel(bid entity.EntityID, v int) {
	if (g.fuel[bid] > 0) != (v > 0) {
		g.light.ver++
	}
	g.fuel[bid] = v
}

// glowOf is the light building b (id) gives off for the renderer
// (BuildingView.Glow) and its reach in tiles, the radius this grid lights
// with: a lamp while powered, a torch lamp or a campfire while it burns
// (not banked or doused).
func (g *Game) glowOf(id entity.EntityID, b construction.Building) (bool, int) {
	on, rad, _ := g.shining(id, b)
	return on, rad
}

// lightCells returns the artificial light grid, rebuilt if stale.
func (g *Game) lightCells() []float32 {
	c := &g.light
	w, h := g.grid.Width(), g.grid.Height()
	src := g.lightSources()
	douse, roofV := false, uint64(0)
	if src.torches > 0 { // a torch under the sky goes out in the rain
		douse, roofV = g.rainDouses(), g.roofVer()
	}
	if c.ok && c.w == w && c.h == h && c.structVer == g.structVer && c.builtVer == c.ver &&
		c.bldgs == any(g.bldgs) && c.plants == any(g.plants) && c.douse == douse && c.roofV == roofV {
		return c.art
	}
	if len(c.art) != w*h {
		c.art = make([]float32, w*h)
	} else {
		clear(c.art)
	}
	for _, id := range src.ids {
		b := g.bldgs.Ref(id)
		if b == nil {
			continue
		}
		var on bool
		var rad int
		var strength float64
		if b.Kind == construction.Campfire { // PLAY-017: a fire with fuel lights, banked or rained on too
			on, rad, strength = g.fuel[id] > 0, campfireRadius, campfireLight
		} else {
			on, rad, strength = g.shining(id, *b)
		}
		if on {
			g.stampLight(c.art, b.X, b.Y, rad, strength)
		}
	}
	for _, id := range g.plantIDs {
		if p, ok := g.plants.Get(id); ok {
			if gl := plant.DefOf(p.Species).Glow; gl > 0 {
				g.stampLight(c.art, p.X, p.Y, glowRadius, float64(gl)/100)
			}
		}
	}
	c.ok, c.w, c.h, c.structVer, c.builtVer = true, w, h, g.structVer, c.ver
	c.bldgs, c.plants, c.douse, c.roofV = g.bldgs, g.plants, douse, roofV
	c.builds++
	return c.art
}

// stampLight raises art to a light of strength at (sx, sy) falling off
// linearly to rad: strength × (rad+1−d)/(rad+1) at distance d ≤ rad, where
// the cell can see the source.
func (g *Game) stampLight(art []float32, sx, sy, rad int, strength float64) {
	w := g.grid.Width()
	for y := sy - rad; y <= sy+rad; y++ {
		for x := sx - rad; x <= sx+rad; x++ {
			if !g.grid.InBounds(x, y) {
				continue
			}
			dx, dy := float64(x-sx), float64(y-sy)
			d := math.Sqrt(dx*dx + dy*dy)
			if d > float64(rad) {
				continue
			}
			v := float32(strength * (float64(rad) + 1 - d) / (float64(rad) + 1))
			i := y*w + x
			if v <= art[i] || !g.combat.Sight.Line(sx, sy, x, y) {
				continue
			}
			art[i] = v
		}
	}
}
