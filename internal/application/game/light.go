package game

import (
	"math"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/lighting"
	"rimworld-go/internal/domain/world/plant"
)

// The simulation's light (MAPGEN-3): what plants that need light, or
// wither in it, see. The renderer's sprite lighting is separate.
//
// Artificial light per cell comes from lamps, campfires and glowing
// plants (duskmoss), each falling off linearly with distance to its
// radius and stopped by rock, walls and doors (the line-of-sight rule
// bullets use). Overlapping lights take the brightest, not the sum. The
// grid is cached and rebuilt only when structVer moves (any building or
// terrain change: lamps and fires come and go with it) or a glowing
// plant is added or removed (lightVer); a cell's light is then one read.
const (
	lampRadius     = 6
	lampLight      = 0.7
	campfireRadius = 5
	campfireLight  = 0.6
	glowRadius     = 1
)

// lightGrid is the cached artificial light, never saved.
type lightGrid struct {
	ok        bool
	w, h      int
	structVer uint64
	// ver bumps when a glowing plant is added or removed.
	ver, builtVer uint64
	bldgs, plants any // the stores it was built from (a load swaps them)
	art           []float32
	builds        int // rebuilds so far (tests)
}

// cellLight is the simulation's light on (x, y): daylight on unroofed
// cells, the brightest artificial light anywhere.
func (g *Game) cellLight(x, y int) lighting.Level {
	return lighting.Cell(lighting.Outdoor(g.clock.Now()), g.roofAt(x, y).Roofed(), g.artificialLight(x, y))
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

// lightCells returns the artificial light grid, rebuilt if stale.
func (g *Game) lightCells() []float32 {
	c := &g.light
	w, h := g.grid.Width(), g.grid.Height()
	if c.ok && c.w == w && c.h == h && c.structVer == g.structVer && c.builtVer == c.ver &&
		c.bldgs == any(g.bldgs) && c.plants == any(g.plants) {
		return c.art
	}
	if len(c.art) != w*h {
		c.art = make([]float32, w*h)
	} else {
		clear(c.art)
	}
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		switch b.Kind {
		case construction.Lamp:
			g.stampLight(c.art, b.X, b.Y, lampRadius, lampLight)
		case construction.Campfire:
			g.stampLight(c.art, b.X, b.Y, campfireRadius, campfireLight)
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
	c.bldgs, c.plants = g.bldgs, g.plants
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
			if v <= art[i] || !g.lineOfSight(sx, sy, x, y) {
				continue
			}
			art[i] = v
		}
	}
}
