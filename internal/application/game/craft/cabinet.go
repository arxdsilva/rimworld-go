package craft

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	// cabinetPct is the bench work bonus of one tool cabinet; cabinetMax
	// cabinets count; cabinetReach is how near (Chebyshev tiles) they must be.
	cabinetPct   = 6
	cabinetMax   = 2
	cabinetReach = 6
)

// Cabinets is the tool-cabinet bonus of each bench (NG-1 §4.8), cached per
// bench for one StructVer; never saved.
type Cabinets struct {
	h   Host
	ver uint64
	by  map[entity.EntityID]int
}

// Bonus is the percent bill-work bonus of bench from tool cabinets within
// cabinetReach tiles (cabinetPct each, at most cabinetMax). It is cached
// per bench until a building or terrain change moves StructVer.
func (c *Cabinets) Bonus(bench entity.EntityID) int {
	if ver := c.h.StructVer(); c.by == nil || c.ver != ver {
		c.by, c.ver = map[entity.EntityID]int{}, ver
	}
	if pct, ok := c.by[bench]; ok {
		return pct
	}
	pct := c.count(bench) * cabinetPct
	c.by[bench] = pct
	return pct
}

func (c *Cabinets) count(bench entity.EntityID) int {
	b, ok := c.h.Building(bench)
	if !ok {
		return 0
	}
	n := 0
	for _, bid := range c.h.BuildingIDs() {
		o, ok := c.h.Building(bid)
		if !ok || o.Kind != construction.ToolCabinet || max(abs(o.X-b.X), abs(o.Y-b.Y)) > cabinetReach {
			continue
		}
		if n++; n == cabinetMax {
			break
		}
	}
	return n
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
