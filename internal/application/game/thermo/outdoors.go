package thermo

import (
	"slices"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/world/room"
)

// Outdoors judgements (TEMP-6, wiki: Rooms). Rooms here are the
// flood-filled areas of room.Detect; the outdoor area (room.Outdoors) is
// every cell open to the map edge, judged as one room that touches it.
const (
	// workOpenPct and workOpenCells: a room is outdoors for work when more
	// than 25 % of it, or more than 100 of its cells, is unroofed.
	workOpenPct   = 25
	workOpenCells = 100
	// psychOpenCells and psychEdgePct: a room feels outdoors with 300 or
	// more unroofed cells, or open to the map edge with at least 50 % of
	// it unroofed.
	psychOpenCells = 300
	psychEdgePct   = 50
)

// openOf is room r's cells, unroofed cells and whether it touches the
// map edge (a wall or a cell of no room: none).
func (m *Model) openOf(r int) (cells, open int, edge bool) {
	m.ensure()
	switch {
	case r == room.Outdoors:
		return m.c.outCells, m.c.outOpen, true
	case r >= 0 && r < len(m.c.exp):
		e := m.c.exp[r]
		return e.Cells, e.Open, false
	}
	return 0, 0, false
}

// OutdoorsForWork reports room r (or room.Outdoors) outdoors for work:
// the surgery chance takes the outdoor factor there (wiki: Rooms,
// OutdoorsForWork).
func (m *Model) OutdoorsForWork(r int) bool {
	cells, open, _ := m.openOf(r)
	return open > workOpenCells || open*100 > cells*workOpenPct
}

// PsychOutdoors reports room r (or room.Outdoors) psychologically
// outdoors: a bench there works at the outdoor speed and a sleeper wakes
// having slept outside (wiki: Rooms, PsychologicallyOutdoors).
func (m *Model) PsychOutdoors(r int) bool {
	cells, open, edge := m.openOf(r)
	return open >= psychOpenCells || edge && cells > 0 && open*100 >= cells*psychEdgePct
}

// spotTries is how many of the nearest candidate spots get a path check.
const spotTries = 12

// spot is a candidate cell and its distance.
type spot struct{ i, d int }

// Spot is the nearest free, reachable cell in a room whose air bearable
// accepts, for a creature at (x, y) (TEMP-6: the
// warmth-seeking search over the cached room cells). bearable says a
// room's air will do, free a cell is walkable and open to the creature,
// reach it can walk there. Candidates go nearest first by pathing.Dist
// (ties by cell index) and only the spotTries nearest are path-checked;
// only bearable rooms are walked, nearest box first, and a room whose box
// lies beyond the spotTries-th candidate so far is never opened. ok is
// false when there is none.
func (m *Model) Spot(x, y int, bearable func(r int) bool, free, reach func(x, y int) bool) (int, int, bool) {
	m.ensure()
	_, w, _ := m.h.RoomGrid()
	type near struct{ r, d int }
	var rooms []near
	for r, b := range m.c.box {
		if b[2] >= 0 && bearable(r) {
			bx, by := min(max(x, int(b[0])), int(b[2])), min(max(y, int(b[1])), int(b[3]))
			rooms = append(rooms, near{r, pathing.Dist(x, y, bx, by)})
		}
	}
	slices.SortFunc(rooms, func(a, b near) int {
		if a.d != b.d {
			return a.d - b.d
		}
		return a.r - b.r
	})
	byDist := func(a, b spot) int {
		if a.d != b.d {
			return a.d - b.d
		}
		return a.i - b.i
	}
	var spots []spot
	for _, n := range rooms {
		if len(spots) >= spotTries && n.d > spots[spotTries-1].d {
			break // every cell of this room and the rest is farther
		}
		for _, i := range m.c.cells[m.c.start[n.r]:m.c.start[n.r+1]] {
			if cx, cy := int(i)%w, int(i)/w; free(cx, cy) {
				spots = append(spots, spot{int(i), pathing.Dist(x, y, cx, cy)})
			}
		}
		slices.SortFunc(spots, byDist)
		spots = spots[:min(len(spots), spotTries)]
	}
	for _, s := range spots {
		if reach(s.i%w, s.i/w) {
			return s.i % w, s.i / w, true
		}
	}
	return 0, 0, false
}
