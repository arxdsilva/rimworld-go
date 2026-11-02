package industry

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// siteCache is the industry's buildings by kind, in building order, per
// structure version: never saved.
type siteCache struct {
	ok                             bool
	sv                             uint64
	n                              int
	drills, scanners, pumps, mills []entity.EntityID
}

// sitesOf is h's industry buildings, rebuilt only when the structure
// changes.
func (s *State) sitesOf(h Sites) *siteCache {
	c := &s.sites
	ids := h.BuildingIDs()
	if c.ok && c.sv == h.StructVer() && c.n == len(ids) {
		return c
	}
	c.drills, c.scanners, c.pumps, c.mills = c.drills[:0], c.scanners[:0], c.pumps[:0], c.mills[:0]
	for _, id := range ids {
		b, ok := h.Building(id)
		if !ok {
			continue
		}
		switch b.Kind {
		case construction.DeepDrill:
			c.drills = append(c.drills, id)
		case construction.GroundScanner:
			c.scanners = append(c.scanners, id)
		case construction.MoisturePump:
			c.pumps = append(c.pumps, id)
		case construction.Watermill:
			c.mills = append(c.mills, id)
		}
	}
	c.ok, c.sv, c.n = true, h.StructVer(), len(ids)
	return c
}
