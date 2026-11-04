package thermo

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
)

// cache is what the model counts from the map and keeps until a room
// rebuild, a roof change or a building change: never saved.
type cache struct {
	ok                    bool
	gen, roofVer, structV uint64
	exp                   []temperature.Exposure // per room
	links                 []link
	sites                 []site
	// outCells and outOpen are the outdoor area's cells and unroofed
	// cells (room.Outdoors: every cell open to the map edge), for its
	// outdoors judgements (outdoors.go).
	outCells, outOpen int
	// cells lists every room's cells, room by room in cell order: room
	// r's are cells[start[r]:start[r+1]]; box is each room's bounding box
	// (x0, y0, x1, y1). TEMP-6: the warmth-seeking search and the hot
	// rooms walk these instead of the map.
	start, cells []int32
	box          [][4]int32
}

// link is a door or vent between two sides, each a room id or
// room.Outdoors.
type link struct {
	id   entity.EntityID
	a, b int
	vent bool
}

// site is a building that moves heat, with its room (a cooler: room is
// the cold side, other the hot side, either may be room.Outdoors).
type site struct {
	id          entity.EntityID
	kind        construction.Kind
	room, other int
}

// ensure recounts the cache when the rooms, roofs or buildings changed.
func (m *Model) ensure() {
	h := m.h
	c := &m.c
	if c.ok && c.gen == m.gen && c.roofVer == h.RoofVer() && c.structV == h.StructVer() {
		return
	}
	grid, w, gh := h.RoomGrid()
	c.exp, c.links, c.sites = c.exp[:0], c.links[:0], c.sites[:0]
	if len(grid) != w*gh { // no rooms detected yet
		c.ok = false
		return
	}
	c.exp = exposures(c.exp, h, grid, w, gh, len(m.Temps))
	c.index(h, grid, w)
	at := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= gh {
			return room.Outdoors
		}
		return grid[y*w+x]
	}
	for _, id := range h.BuildingIDs() {
		b, ok := h.Building(id)
		if !ok {
			continue
		}
		switch k := b.Kind; {
		case k.IsDoor():
			a, z := doorSides(at, b.X, b.Y)
			c.links = addLink(c.links, link{id: id, a: a, b: z})
		case k == construction.Vent || k == construction.Cooler:
			dx, dy := FacingStep(b.Facing)
			front, back := at(b.X+dx, b.Y+dy), at(b.X-dx, b.Y-dy)
			if k == construction.Vent {
				c.links = addLink(c.links, link{id: id, a: front, b: back, vent: true})
			} else if front >= 0 {
				c.sites = append(c.sites, site{id: id, kind: k, room: front, other: back})
			}
		case heats(k):
			if r := at(b.X, b.Y); r >= 0 {
				c.sites = append(c.sites, site{id: id, kind: k, room: r})
			}
		}
	}
	c.ok, c.gen, c.roofVer, c.structV = true, m.gen, h.RoofVer(), h.StructVer()
}

// exposures counts every room's walls and roof (temperature.Exposure)
// in one pass over the map.
func exposures(out []temperature.Exposure, h Map, grid []int, w, gh, n int) []temperature.Exposure {
	for range n {
		out = append(out, temperature.Exposure{})
	}
	cell := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= gh {
			return room.Outdoors
		}
		return grid[y*w+x]
	}
	for i, r := range grid {
		if r < 0 || r >= n {
			continue
		}
		e := &out[r]
		e.Cells++
		switch c := h.Roof(i); {
		case c.Thick():
			e.Thick++
		case c.Roofed():
			e.Thin++
		default:
			e.Open++
		}
		x, y := i%w, i/w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if cell(x+d[0], y+d[1]) != room.Wall {
				continue
			}
			switch cell(x+2*d[0], y+2*d[1]) {
			case room.Outdoors:
				e.Single++
			case room.Wall:
				if cell(x+3*d[0], y+3*d[1]) == room.Outdoors {
					e.Double++ // wiki: a second layer halves the loss, a third adds nothing
				}
			}
		}
	}
	return out
}

// index lists every room's cells and box (one pass over the grid, on a
// recount only) and counts the outdoor area's cells and open sky.
func (c *cache) index(h Map, grid []int, w int) {
	n := len(c.exp)
	c.start, c.box = c.start[:0], c.box[:0]
	at := int32(0)
	for _, e := range c.exp {
		c.start = append(c.start, at)
		c.box = append(c.box, [4]int32{1 << 30, 1 << 30, -1, -1})
		at += int32(e.Cells)
	}
	c.start = append(c.start, at)
	if cap(c.cells) < int(at) {
		c.cells = make([]int32, at)
	}
	c.cells = c.cells[:at]
	next := append([]int32(nil), c.start[:n]...)
	c.outCells, c.outOpen = 0, 0
	for i, r := range grid {
		switch {
		case r == room.Outdoors:
			c.outCells++
			if !h.Roof(i).Roofed() {
				c.outOpen++
			}
		case r >= 0 && r < n:
			c.cells[next[r]] = int32(i)
			next[r]++
			x, y, b := int32(i%w), int32(i/w), &c.box[r]
			b[0], b[1], b[2], b[3] = min(b[0], x), min(b[1], y), max(b[2], x), max(b[3], y)
		}
	}
}

// doorSides is the two sides a door on (x, y) joins: across it east to
// west, or else north to south.
func doorSides(at func(x, y int) int, x, y int) (int, int) {
	if e, w := at(x+1, y), at(x-1, y); e != room.Wall && w != room.Wall {
		return e, w
	}
	return at(x, y+1), at(x, y-1)
}

// addLink keeps l if it joins two different places with air (not two
// walls, not the outdoors with itself).
func addLink(ls []link, l link) []link {
	if l.a == room.Wall || l.b == room.Wall || l.a == l.b || l.a < 0 && l.b < 0 {
		return ls
	}
	return append(ls, l)
}

// FacingStep is the cell step toward facing f (south is +y).
func FacingStep(f construction.Facing) (int, int) {
	switch f {
	case construction.North:
		return 0, -1
	case construction.East:
		return 1, 0
	case construction.West:
		return -1, 0
	}
	return 0, 1
}

// Room is a room's exposure and roof for its stats line.
type Room struct {
	// ExposedWalls and DoubleWalls are wall faces with outdoor air one
	// and two walls behind them.
	ExposedWalls, DoubleWalls int
	// RoofKind is "thin", "thick" (overhead mountain), "mixed" or ""
	// (no roof at all).
	RoofKind string
	// Exposure is the room's full count.
	Exposure temperature.Exposure
}

// RoomOf is room r's exposure (zero for no such room).
func (m *Model) RoomOf(r int) Room {
	m.ensure()
	if r < 0 || r >= len(m.c.exp) {
		return Room{}
	}
	e := m.c.exp[r]
	out := Room{ExposedWalls: e.Single, DoubleWalls: e.Double, Exposure: e}
	switch {
	case e.Thin > 0 && e.Thick > 0:
		out.RoofKind = "mixed"
	case e.Thick > 0:
		out.RoofKind = "thick"
	case e.Thin > 0:
		out.RoofKind = "thin"
	}
	return out
}

// ColdSides is how many coolers set under 0 °C have their cold side in
// room r (CSE-043: a cooled store room). Counts the site cache, no map scan.
func (m *Model) ColdSides(r int) int {
	m.ensure()
	n := 0
	for _, s := range m.c.sites {
		if s.kind == construction.Cooler && s.room == r {
			if target, _ := m.h.Thermostat(s.id); target < 0 {
				n++
			}
		}
	}
	return n
}
