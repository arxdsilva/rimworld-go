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
