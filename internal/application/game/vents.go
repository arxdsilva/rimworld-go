package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/temperature"
)

// Steam vents (MAPGEN-2 §3.5): natural 2x2 geothermal vents left by map
// generation. They can't be built over, deconstructed, moved or swapped
// (setup_mapgen.go: ReasonNatural), and they warm the air: the cells
// within ventReach of the footprint by ventOutdoorHeat outdoors, and the
// room a vent stands in by ventRoomHeat (heat.go: heatRooms). Bursts of
// steam are presentation only. Owned by INTEGRATE.

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
// (never saved).
type ventCache struct {
	ver  uint64
	ok   bool
	list [][2]int
}

// ventList is every steam vent's top-left cell, in building order.
func (g *Game) ventList() [][2]int {
	c := &g.vents
	if c.ok && c.ver == g.structVer {
		return c.list
	}
	c.list = c.list[:0]
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && b.Kind == construction.SteamVent {
			c.list = append(c.list, [2]int{b.X, b.Y})
		}
	}
	c.ver, c.ok = g.structVer, true
	return c.list
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
