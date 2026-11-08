// Package idindex is the id containers behind the game's indexes (NG-1
// §6.3 E1, never saved): Cells, one id per map cell and layer behind the
// plant, building and frame cell indexes, with the cells where two live
// entries meet (answered by a scan) and whether any footprint reached off
// the map; and Set, the insertion-ordered marked-plant set behind
// nearestPlant. The game verifies every id against its stores; this
// package only holds them. No host.
package idindex

import "rimworld-go/internal/domain/sim/entity"

// Cells is one id per cell, for a fixed number of layers.
type Cells struct {
	w, h     int
	IDs      [][]entity.EntityID // [layer][cell]
	Conflict map[int]bool        // cells answered by scan
	OffMap   bool                // some footprint reached off the map: off-map queries scan
	src      any                 // the store the index was built from
	built    bool
}

// Reset empties c for a w × h map of layers layers, built from src.
func (c *Cells) Reset(w, h, layers int, src any) {
	c.w, c.h, c.src, c.built, c.OffMap = w, h, src, true, false
	c.Conflict = nil
	if len(c.IDs) != layers {
		c.IDs = make([][]entity.EntityID, layers)
	}
	for l := range c.IDs {
		if len(c.IDs[l]) != w*h {
			c.IDs[l] = make([]entity.EntityID, w*h)
		} else {
			clear(c.IDs[l])
		}
	}
}

// Stale reports whether c needs a rebuild for a w × h map over src.
func (c *Cells) Stale(w, h int, src any) bool {
	return !c.built || c.w != w || c.h != h || c.src != src
}

// Cell is the slot index of (x, y), or -1 off the map.
func (c *Cells) Cell(x, y int) int {
	if uint(x) >= uint(c.w) || uint(y) >= uint(c.h) {
		return -1
	}
	return y*c.w + x
}

// Put records id on (x, y) in layer l; live reports whether an id
// already there still covers the cell (then the cell is conflicted).
func (c *Cells) Put(l, x, y int, id entity.EntityID, live func(entity.EntityID) bool) {
	i := c.Cell(x, y)
	if i < 0 {
		c.OffMap = true
		return
	}
	if old := c.IDs[l][i]; old != 0 && old != id && live(old) {
		if c.Conflict == nil {
			c.Conflict = map[int]bool{}
		}
		c.Conflict[i] = true
		return
	}
	c.IDs[l][i] = id
}
