package industry

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// The wind turbine's clear band (PLAY-021, moved from game/power_gen.go
// by AX-4): the share of a band beside a turbine that is open air, which
// scales its output.

// windBandDepth is how many cells wide the band beside each long side of
// a turbine is.
const windBandDepth = 5

// Air is what the wind band needs of the map.
type Air interface {
	StructVer() uint64
	// RoofVer moves whenever a cell gains or loses its roof.
	RoofVer() uint64
	// Hour is the game hour (trees grow, so a band is looked at again
	// every hour).
	Hour() uint64
	// WindBlocked reports a cell that stops the wind (a roof, rock, a
	// wall or door, a blocking building, a tree); off the map is open.
	WindBlocked(x, y int) bool
}

// windCache holds each turbine's clear share (permille), valid for one
// structure, roof state and game hour. Never saved.
type windCache struct {
	ok      bool
	structV uint64
	roofV   uint64
	hour    uint64
	stamp   any // the building store it was built from (a load swaps it)
	share   map[entity.EntityID]int
	Builds  int // band scans so far (tests)
}

// WindBuilds is how many turbine bands were scanned so far (tests).
func (s *State) WindBuilds() int { return s.wind.Builds }

// WindClear is the share (permille) of turbine id's band (b, the turbine
// itself) that is open air. stamp is the host's building store: another
// store (a load) drops the cache.
func (s *State) WindClear(a Air, stamp any, id entity.EntityID, b construction.Building) int {
	c := &s.wind
	if !c.ok || c.structV != a.StructVer() || c.roofV != a.RoofVer() || c.hour != a.Hour() || c.stamp != stamp {
		clear(c.share)
		if c.share == nil {
			c.share = map[entity.EntityID]int{}
		}
		c.ok, c.structV, c.roofV, c.hour, c.stamp = true, a.StructVer(), a.RoofVer(), a.Hour(), stamp
	}
	if v, ok := c.share[id]; ok {
		return v
	}
	v := scanWindBand(a, b)
	c.share[id] = v
	c.Builds++
	return v
}

// scanWindBand counts the open cells of the band along both long sides of
// turbine b: windBandDepth cells out from each side, over the turbine's
// length.
func scanWindBand(a Air, b construction.Building) int {
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
		if !a.WindBlocked(x, y) {
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
