package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/world/filth"
)

// The Home area (SIM-021): tiles the colony keeps clean. Painted with
// DesignateHome; grows around every new building; a new colony starts
// with one around its landing spot.

const (
	autoHomeRadius = 2 // Chebyshev tiles of Home around a new building
	startHomeHalf  = 6 // a new colony's Home: 13x13 around the landing spot
)

// homeState is the Home area, one byte per tile.
type homeState struct {
	cells []uint8
	ver   uint32 // bumped on every change (WorldView.HomeVer)
}

// inHome reports whether tile i is in the Home area.
func (g *Game) inHome(i int) bool {
	return i >= 0 && i < len(g.homeArea.cells) && g.homeArea.cells[i] != 0
}

// paintHome adds (on) or clears the Home area over the rectangle. Only
// the rectangle is visited: newly Home tiles over the cleaning threshold
// join the queue, cleared ones leave it.
func (g *Game) paintHome(x, y, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x, y, x2, y2)
	w, h := g.grid.Width(), g.grid.Height()
	minX, minY = max(0, minX), max(0, minY)
	maxX, maxY = min(w-1, maxX), min(h-1, maxY)
	if minX > maxX || minY > maxY {
		return
	}
	hs := &g.homeArea
	if len(hs.cells) != w*h {
		if !on {
			return
		}
		hs.cells = make([]uint8, w*h)
	}
	v := uint8(0)
	if on {
		v = 1
	}
	changed := false
	for ty := minY; ty <= maxY; ty++ {
		for tx := minX; tx <= maxX; tx++ {
			i := ty*w + tx
			if hs.cells[i] == v {
				continue
			}
			hs.cells[i], changed = v, true
			switch {
			case !on:
				g.filth.cleanQ = removeSorted(g.filth.cleanQ, int32(i))
			case g.filthTotal(i) >= filth.CleanStart:
				g.filth.cleanQ = insertSorted(g.filth.cleanQ, int32(i))
			}
		}
	}
	if changed {
		hs.ver++
	}
}

// autoHome extends the Home area around a completed building (floors and
// graves excepted).
func (g *Game) autoHome(k construction.Kind, f construction.Facing, x, y int) {
	if construction.DefOf(k).Layer == construction.LayerFloor || k == construction.Grave {
		return
	}
	cells, _, _ := construction.Footprint(k, f, x, y)
	if len(cells) == 0 {
		return
	}
	minX, minY, maxX, maxY := cells[0][0], cells[0][1], cells[0][0], cells[0][1]
	for _, c := range cells[1:] {
		minX, minY = min(minX, c[0]), min(minY, c[1])
		maxX, maxY = max(maxX, c[0]), max(maxY, c[1])
	}
	r := autoHomeRadius
	g.paintHome(minX-r, minY-r, maxX+r, maxY+r, true)
}

// startHome is a new colony's Home around its landing spot (the start
// kit's centre).
func (g *Game) startHome() {
	cx, cy := g.landingSpot() // setup.go: the map centre for legacy colonies
	g.paintHome(cx-startHomeHalf, cy-startHomeHalf, cx+startHomeHalf, cy+startHomeHalf, true)
}

// fillHomeView copies the Home area into the view.
func (g *Game) fillHomeView(w *WorldView) {
	if n := g.grid.Width() * g.grid.Height(); len(g.homeArea.cells) == n {
		w.Home = shareLayer(&g.viewHome, g.homeArea.cells) // indexes.go: shared, copy-on-write
	} else {
		w.Home = make([]uint8, n)
	}
	w.HomeVer = g.homeArea.ver
}

// exportHome and importHome save and restore the Home area and its
// version; an older save (nil) gets a Home area around its buildings.
func (g *Game) exportHome() []uint8 {
	if n := g.grid.Width() * g.grid.Height(); len(g.homeArea.cells) != n {
		return make([]uint8, n)
	}
	return slices.Clone(g.homeArea.cells)
}

func (g *Game) importHome(cells []uint8, ver uint32) {
	g.homeArea = homeState{}
	if len(cells) == g.grid.Width()*g.grid.Height() {
		g.homeArea.cells = make([]uint8, len(cells))
		for i, v := range cells {
			if v != 0 {
				g.homeArea.cells[i] = 1
			}
		}
		g.homeArea.ver = ver
	} else {
		for _, id := range g.bldgIDs {
			if b, ok := g.bldgs.Get(id); ok && !notColonyPiece(b) { // ruins and vents anchor no Home
				g.autoHome(b.Kind, b.Facing, b.X, b.Y)
			}
		}
	}
	g.refreshFilth()
}
