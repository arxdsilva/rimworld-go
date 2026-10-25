package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
)

// Plan marks (PLAY-012): tiles the player flagged as "build here later".
// They have no gameplay effect; no job, path or room reads them. A finished
// non-floor building (or a move landing) clears the marks under it.

// planState is the plan marks, one byte per tile (nil until first painted).
type planState struct {
	cells []uint8
	ver   uint32 // bumped on every real change (WorldView.PlanVer)
}

// paintPlan marks (on) or erases the plan over the rectangle; the version
// only moves when a cell changes.
func (g *Game) paintPlan(x, y, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x, y, x2, y2)
	w, h := g.grid.Width(), g.grid.Height()
	minX, minY = max(0, minX), max(0, minY)
	maxX, maxY = min(w-1, maxX), min(h-1, maxY)
	if minX > maxX || minY > maxY {
		return
	}
	ps := &g.planMarks
	if len(ps.cells) != w*h {
		if !on {
			return
		}
		ps.cells = make([]uint8, w*h)
	}
	v := uint8(0)
	if on {
		v = 1
	}
	changed := false
	for ty := minY; ty <= maxY; ty++ {
		for tx := minX; tx <= maxX; tx++ {
			if i := ty*w + tx; ps.cells[i] != v {
				ps.cells[i], changed = v, true
			}
		}
	}
	if changed {
		ps.ver++
	}
}

// clearPlanFootprint erases the marks under a finished building's cells
// unless it is a floor (floors are laid over a plan, not instead of it).
func (g *Game) clearPlanFootprint(k construction.Kind, cells [][2]int) {
	if construction.DefOf(k).Layer == construction.LayerFloor {
		return
	}
	for _, c := range cells {
		g.paintPlan(c[0], c[1], c[0], c[1], false)
	}
}

// fillPlanView copies the plan into the view (nil when nothing is marked).
func (g *Game) fillPlanView(w *WorldView) {
	if len(g.planMarks.cells) == g.grid.Width()*g.grid.Height() {
		w.Plan = shareLayer(&g.viewPlan, g.planMarks.cells) // indexes.go: shared, copy-on-write
	}
	w.PlanVer = g.planMarks.ver
}

// exportPlan lists the marked tile indexes, sorted; nil when none.
func (g *Game) exportPlan() []int32 {
	var out []int32
	for i, v := range g.planMarks.cells {
		if v != 0 {
			out = append(out, int32(i))
		}
	}
	return out
}

// importPlan rebuilds the marks from saved indexes; the version restarts
// at 1 when any are present so renderer caches refresh.
func (g *Game) importPlan(idx []int32) {
	g.planMarks = planState{}
	n := g.grid.Width() * g.grid.Height()
	idx = slices.DeleteFunc(slices.Clone(idx), func(i int32) bool { return i < 0 || int(i) >= n })
	if len(idx) == 0 {
		return
	}
	g.planMarks.cells = make([]uint8, n)
	for _, i := range idx {
		g.planMarks.cells[i] = 1
	}
	g.planMarks.ver = 1
}
