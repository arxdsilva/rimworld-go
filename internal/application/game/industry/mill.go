package industry

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// The watermill (wiki: Watermill generator): 1100 W while its wheel turns
// in moving water, 330 W while another watermill stands in its exclusion
// area (13x7: the footprint and 4 cells beyond it along the river, one
// past the wheel). The wheel is the footprint's row on its facing side
// (construction.WheelCell); its share of cells in moving water scales
// the output (our rule: the wiki's wheel must sit wholly in it, which
// placement enforces, so a built mill reads full flow).
const (
	MillW        = 1100 // wiki: Watermill generator
	MillCrowdedW = 330  // wiki: with an overlapping exclusion area
	millReach    = 4    // wiki: the exclusion area's cells beyond
)

// Placement refusals.
const (
	ReasonNoRiver = "The wheel needs a river's moving water"
)

// Mill is the watermills' rules over a State and its host.
type Mill struct {
	s *State
	h Host
}

// Mill is the watermills of s.
func (s *State) Mill(h Host) Mill { return Mill{s, h} }

// millCache is each watermill's output before switches, per structure.
type millCache struct {
	ok  bool
	sv  uint64
	out map[entity.EntityID]int
}

// Place is the watermill's whole-footprint rule: every wheel cell (which
// the cell rule skips) is on the map in moving water.
func (m Mill) Place(k construction.Kind, f construction.Facing, x, y int) (bool, string) {
	if k != construction.Watermill {
		return true, ""
	}
	cells, _, _ := construction.Footprint(k, f, x, y)
	for i, c := range cells {
		if construction.WheelCell(k, i) && !m.moving(c[0], c[1]) {
			return false, ReasonNoRiver
		}
	}
	return true, ""
}

// moving reports river water still flowing on (x, y).
func (m Mill) moving(x, y int) bool { return m.s.Moving(x, y) && terrain.IsWater(m.h.Terrain(x, y)) }

// Flow is watermill b's flow: the permille of its wheel in moving water.
func (m Mill) Flow(b construction.Building) int {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	n, wet := 0, 0
	for i, c := range cells {
		if construction.WheelCell(b.Kind, i) {
			n++
			if m.moving(c[0], c[1]) {
				wet++
			}
		}
	}
	if n == 0 {
		return 0
	}
	return wet * 1000 / n
}

// Output is watermill id's output in W (0 for anything else).
func (m Mill) Output(id entity.EntityID) int {
	c := &m.s.mill
	if !c.ok || c.sv != m.h.StructVer() {
		c.ok, c.sv, c.out = true, m.h.StructVer(), map[entity.EntityID]int{}
	}
	if w, ok := c.out[id]; ok {
		return w
	}
	b, ok := m.h.Building(id)
	if !ok || b.Kind != construction.Watermill {
		return 0
	}
	w := MillW
	if m.crowded(id, b) {
		w = MillCrowdedW
	}
	w = w * m.Flow(b) / 1000
	c.out[id] = w
	return w
}

// crowded reports another watermill in b's exclusion area.
func (m Mill) crowded(id entity.EntityID, b construction.Building) bool {
	x0, y0, x1, y1 := m.exclusion(b)
	for _, oid := range m.s.sitesOf(m.h).mills {
		o, ok := m.h.Building(oid)
		if oid == id || !ok {
			continue
		}
		cells, _, _ := construction.Footprint(o.Kind, o.Facing, o.X, o.Y)
		for _, c := range cells {
			if c[0] >= x0 && c[0] <= x1 && c[1] >= y0 && c[1] <= y1 {
				return true
			}
		}
	}
	return false
}

// exclusion is b's exclusion rectangle (inclusive).
func (m Mill) exclusion(b construction.Building) (x0, y0, x1, y1 int) {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	x0, y0, x1, y1 = cells[0][0], cells[0][1], cells[0][0], cells[0][1]
	for _, c := range cells {
		x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
	}
	switch b.Facing {
	case construction.South:
		return x0 - millReach, y0, x1 + millReach, y1 + 1
	case construction.North:
		return x0 - millReach, y0 - 1, x1 + millReach, y1
	case construction.East:
		return x0, y0 - millReach, x1 + 1, y1 + millReach
	}
	return x0 - 1, y0 - millReach, x1, y1 + millReach
}
