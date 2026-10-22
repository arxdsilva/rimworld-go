package game

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
)

// Fences (SIM-028). A fence keeps every animal out (or in); a fence gate
// lets the colony's tame animals through but no wild ones. Colonists and
// raiders climb over a fence at fenceClimb times the step cost and walk
// through a gate. Fence cells are a derived per-map index, rebuilt after
// any building change (markRoomsDirty), so saves need nothing extra.

// fenceClimb multiplies the step cost of a pawn crossing a fence.
const fenceClimb = 3

// Fence cell values.
const (
	fenceNone uint8 = iota
	fencePost
	fenceGate
)

// fenceIndex marks the cells holding a built fence (fencePost) or gate
// (fenceGate). built is false until the next query after a building
// change; gen counts rebuilds. reach caches, per animal kind (0 wild, 1
// tame), which connected area each walkable cell belongs to while fences
// stand, so grazers never pick food behind a fence.
type fenceIndex struct {
	built bool
	cells []uint8
	n     int
	gen   uint64
	reach [2]fenceReach
}

type fenceReach struct {
	gen     uint64
	gridVer uint64
	label   []int32
	ok      bool
}

// fences is the current fence index, rebuilt from the buildings when
// stale.
func (g *Game) fences() *fenceIndex {
	f := &g.fence
	if f.built {
		return f
	}
	f.built, f.n = true, 0
	f.gen++
	w, h := g.grid.Width(), g.grid.Height()
	if len(f.cells) != w*h {
		f.cells = make([]uint8, w*h)
	} else {
		clear(f.cells)
	}
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok || !construction.DefOf(b.Kind).AnimalBlock {
			continue
		}
		v := fencePost
		if construction.DefOf(b.Kind).Gate {
			v = fenceGate
		}
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			if g.grid.InBounds(c[0], c[1]) {
				f.cells[c[1]*w+c[0]] = v
				f.n++
			}
		}
	}
	return f
}

// fenceDirty drops the fence index after a building change.
func (g *Game) fenceDirty() { g.fence.built = false }

// fenceAt is what stands on (x, y): fenceNone, fencePost or fenceGate.
func (g *Game) fenceAt(x, y int) uint8 {
	f := g.fences()
	if f.n == 0 || !g.grid.InBounds(x, y) {
		return fenceNone
	}
	return f.cells[y*g.grid.Width()+x]
}

// fenceBars reports that an animal (tame or not) may not enter (x, y):
// a fence, or a gate for a wild one.
func (g *Game) fenceBars(tame bool, x, y int) bool {
	switch g.fenceAt(x, y) {
	case fencePost:
		return true
	case fenceGate:
		return !tame
	}
	return false
}

// fenceSlow is the step cost base after climbing a fence on (x, y).
func (g *Game) fenceSlow(x, y, base int) int {
	if g.fenceAt(x, y) == fencePost {
		return base * fenceClimb
	}
	return base
}

// fenceQuery is an animal's walkability with fences shut (gates open for
// tame ones).
type fenceQuery struct {
	pathing.Query
	cells  []uint8
	w      int
	gateOK bool
}

func (q fenceQuery) Walkable(x, y int) bool {
	if !q.Query.Walkable(x, y) {
		return false
	}
	switch q.cells[y*q.w+x] {
	case fencePost:
		return false
	case fenceGate:
		return q.gateOK
	}
	return true
}

// animalQuery wraps an animal's base walkability (the grid for tame
// ones, the hostile map for wild ones) with the fences; with no fences
// on the map it is base itself.
func (g *Game) animalQuery(tame bool, base pathing.Query) pathing.Query {
	f := g.fences()
	if f.n == 0 {
		return base
	}
	return fenceQuery{Query: base, cells: f.cells, w: g.grid.Width(), gateOK: tame}
}

// animalBase is the walkability an animal plans with before fences.
func (g *Game) animalBase(tame bool) pathing.Query {
	if tame {
		return g.grid // the colony's own animals use its doors
	}
	return g.hostileMap()
}

// fenceReachable returns, while fences stand, a filter of the cells an
// animal standing at pos could walk to (or stand beside, for cells it
// cannot enter); nil when there are no fences (everything is a
// candidate, as before fences existed).
func (g *Game) fenceReachable(a *animal.State, pos Position) func(x, y int) bool {
	f := g.fences()
	if f.n == 0 {
		return nil
	}
	label := g.fenceLabels(a.Tame)
	w, h := g.grid.Width(), g.grid.Height()
	if !g.grid.InBounds(pos.X, pos.Y) {
		return nil
	}
	own := label[pos.Y*w+pos.X]
	if own < 0 {
		// standing on a fence cell (built around it): any neighbour's area
		for _, d := range stepDirs {
			x, y := pos.X+d[0], pos.Y+d[1]
			if x >= 0 && y >= 0 && x < w && y < h && label[y*w+x] >= 0 {
				own = label[y*w+x]
				break
			}
		}
	}
	if own < 0 {
		return func(int, int) bool { return false } // boxed in: nothing in reach
	}
	return func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		if l := label[y*w+x]; l >= 0 {
			return l == own
		}
		if f.cells[y*w+x] != fenceNone {
			return false // food on a fence cell is behind it
		}
		for _, d := range stepDirs { // a tree or trough: eaten from beside
			nx, ny := x+d[0], y+d[1]
			if nx >= 0 && ny >= 0 && nx < w && ny < h && label[ny*w+nx] == own {
				return true
			}
		}
		return false
	}
}

// fenceLabels labels the connected areas (4-neighbour flood: paths never
// cut corners) of the animal map, cached until the fences or the grid
// change.
func (g *Game) fenceLabels(tame bool) []int32 {
	f := g.fences()
	k := 0
	if tame {
		k = 1
	}
	r := &f.reach[k]
	if r.ok && r.gen == f.gen && r.gridVer == g.grid.Version() {
		return r.label
	}
	w, h := g.grid.Width(), g.grid.Height()
	if len(r.label) != w*h {
		r.label = make([]int32, w*h)
	}
	q := g.animalQuery(tame, g.animalBase(tame))
	for i := range r.label {
		r.label[i] = -2 // unvisited
	}
	var next int32
	stack := make([]int, 0, 64)
	for i := range r.label {
		if r.label[i] != -2 {
			continue
		}
		if !q.Walkable(i%w, i/w) {
			r.label[i] = -1
			continue
		}
		r.label[i] = next
		stack = append(stack[:0], i)
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			cx, cy := c%w, c/w
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := cx+d[0], cy+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				ni := ny*w + nx
				if r.label[ni] != -2 {
					continue
				}
				if !q.Walkable(nx, ny) {
					r.label[ni] = -1
					continue
				}
				r.label[ni] = next
				stack = append(stack, ni)
			}
		}
		next++
	}
	r.gen, r.gridVer, r.ok = f.gen, g.grid.Version(), true
	return r.label
}
