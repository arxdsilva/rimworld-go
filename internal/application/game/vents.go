package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// Steam vents (MAPGEN-2 §3.5): natural 2x2 geothermal vents left by map
// generation. They can't be built over, deconstructed, moved or swapped
// (setup_mapgen.go: ReasonNatural), and they warm the air: the cells
// within ventReach of the footprint by ventOutdoorHeat outdoors, and the
// room a vent stands in by ventRoomHeat (heat.go: heatRooms). Bursts of
// steam are presentation only. Owned by INTEGRATE.
//
// PLAY-021: a vent lies in the foundation layer, under everything, so a
// geothermal generator (the one piece that may) stands exactly over it in
// the object layer. The vent stays, and so does its warmth.

const (
	// ventOutdoorHeat is the outdoor warming next to a vent, tenths of a
	// degree C (+6 C).
	ventOutdoorHeat temperature.Temp = 60
	// ventRoomHeat raises the temperature a room settles at by 1.5 C per
	// vent inside it.
	ventRoomHeat temperature.Temp = 15
	// ventReach: cells up to this far (Chebyshev) from the footprint are
	// warmed outdoors (a 6x6 patch: a 5x5 can't centre on a 2x2 vent).
	ventReach = 2
)

// ventCache is the vents' top-left cells, rebuilt when structVer moves
// (never saved). It also carries the other caches that follow the sites
// buildings stand on: the fuel burners (fuel.go) and the wind turbines'
// bands (power_gen.go).
type ventCache struct {
	ver   uint64
	ok    bool
	bldgs any // the store the list was built from (a load swaps it)
	list  [][2]int

	fuel fuelList
	wind windCache
}

// ventList is every steam vent's top-left cell, in building order.
func (g *Game) ventList() [][2]int {
	c := &g.vents
	if c.ok && c.ver == g.structVer && c.bldgs == any(g.bldgs) {
		return c.list
	}
	c.list = c.list[:0]
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && b.Kind == construction.SteamVent {
			c.list = append(c.list, [2]int{b.X, b.Y})
		}
	}
	c.ver, c.ok, c.bldgs = g.structVer, true, g.bldgs
	return c.list
}

// ventAt is the steam vent covering (x, y), 0 for none.
// occupiedAt reports a building on the object layer at (x, y), or a steam
// vent: vents live on the foundation layer (PLAY-021, so a geothermal
// generator can stand on one) but still take their cells the way an
// object does: no sowing, grass, smoothing, storage or pen marker there.
func (g *Game) occupiedAt(x, y int) bool {
	return g.layerBldgAt(x, y, construction.LayerObject) != 0 || g.ventAt(x, y) != 0
}

func (g *Game) ventAt(x, y int) entity.EntityID {
	if !g.grid.InBounds(x, y) {
		return 0
	}
	id := g.layerBldgAt(x, y, construction.LayerFoundation)
	if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.SteamVent {
		return id
	}
	return 0
}

// ventUnder reports a steam vent whose top-left cell is exactly (x, y): a
// 2x2 piece placed at (x, y) covers it exactly (a geothermal generator).
func (g *Game) ventUnder(x, y int) bool {
	id := g.ventAt(x, y)
	if id == 0 {
		return false
	}
	b := g.bldgs.Ref(id)
	return b != nil && b.X == x && b.Y == y
}

// ventSiteFree reports no object-layer building over the vent that would
// stand at (x, y). A vent used to share the object layer with them, and map
// generation still refuses one over a ruin or vault piece.
func (g *Game) ventSiteFree(x, y int) bool {
	cells, _, _ := construction.Footprint(construction.SteamVent, construction.South, x, y)
	for _, c := range cells {
		if g.layerBldgAt(c[0], c[1], construction.LayerObject) != 0 {
			return false
		}
	}
	return true
}

// ventWarmth is the outdoor warming at (x, y) from a nearby steam vent.
func (g *Game) ventWarmth(x, y int) temperature.Temp {
	sz := construction.DefOf(construction.SteamVent).Size
	for _, v := range g.ventList() {
		if x >= v[0]-ventReach && x < v[0]+sz[0]+ventReach && y >= v[1]-ventReach && y < v[1]+sz[1]+ventReach {
			return ventOutdoorHeat
		}
	}
	return 0
}

// ventRoomHeats is the warming per room (indexed like g.rooms) from the
// vents standing in it.
func (g *Game) ventRoomHeats(rooms int) []temperature.Temp {
	vs := g.ventList()
	if len(vs) == 0 {
		return nil
	}
	out := make([]temperature.Temp, rooms)
	for _, v := range vs {
		if r := g.roomAt(v[0], v[1]); r >= 0 && r < rooms {
			out[r] += ventRoomHeat
		}
	}
	return out
}
