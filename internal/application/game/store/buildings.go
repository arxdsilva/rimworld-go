package store

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Buildings lays and lifts the zones storage buildings carry (PRD-4
// shelves, STO-2 hoppers, graves and sarcophagi) and links them into
// storage groups. A building's zone lies over its footprint from the
// moment it is built until it goes; a moved building takes its zone's
// settings (and group) along. It holds no state: the zones are the zone
// map's.
type Buildings struct{}

// ZoneKind is the zone a storage building of kind k lays and the
// settings a new one starts with (ok false for any other building).
func ZoneKind(k construction.Kind) (zone.Kind, zone.Zone, bool) {
	switch k {
	case construction.Shelf:
		return zone.KindShelf, zone.NewShelf(), true
	case construction.Hopper:
		return zone.KindHopper, zone.NewHopper(), true
	case construction.Grave, construction.Sarcophagus:
		z := zone.NewGrave(k == construction.Sarcophagus)
		return z.Kind, z, true
	}
	return 0, zone.Zone{}, false
}

// Of is the storage building zone z lies under (0 for any other zone, or
// one whose building is gone).
func (Buildings) Of(zs *zone.Map, m Machines, z zone.Zone) entity.EntityID {
	if !z.Kind.Building() {
		return 0
	}
	var id entity.EntityID
	zs.EachCell(z.ID, func(x, y int) bool { id = m.ObjectAt(x, y); return false })
	if b, ok := m.Building(id); ok {
		if zk, _, ok := ZoneKind(b.Kind); ok && zk == z.Kind {
			return id
		}
	}
	return 0
}

// ZoneOf is the zone storage building id carries (ok false for none).
func (Buildings) ZoneOf(zs *zone.Map, m Machines, id entity.EntityID) (zone.Zone, bool) {
	b, ok := m.Building(id)
	zk, _, isStore := ZoneKind(b.Kind)
	if !ok || !isStore {
		return zone.Zone{}, false
	}
	zid, in := zs.At(b.X, b.Y)
	z, _ := zs.Zone(zid)
	return z, in && z.Kind == zk
}

// Lay lays a finished (or moved) storage building's zone over cells.
// Zones under the footprint give up the cells; a zone of its kind left
// without its building (the building moved away) is lifted, and its
// settings go with the building.
func (s Buildings) Lay(zs *zone.Map, m Machines, k construction.Kind, cells [][2]int) {
	zk, proto, ok := ZoneKind(k)
	if !ok || len(cells) == 0 {
		return
	}
	for _, c := range cells {
		if id, in := zs.At(c[0], c[1]); in {
			if z, _ := zs.Zone(id); z.Kind == zk {
				proto = z // a move onto its own old cells
			}
		}
	}
	zs.RemoveTiles(cells)
	var stale [][2]int
	zs.Each(func(z zone.Zone) bool {
		if z.Kind == zk && s.Of(zs, m, z) == 0 {
			proto, stale = z, append(stale, zs.Cells(z.ID)...)
		}
		return true
	})
	zs.RemoveTiles(stale)
	r := zone.Rect{MinX: cells[0][0], MinY: cells[0][1], MaxX: cells[0][0], MaxY: cells[0][1]}
	for _, c := range cells {
		r.MinX, r.MinY, r.MaxX, r.MaxY = min(r.MinX, c[0]), min(r.MinY, c[1]), max(r.MaxX, c[0]), max(r.MaxY, c[1])
	}
	zs.Paint(zk, proto, r, nil)
}

// Lift takes a removed storage building's zone off its footprint (and
// out of its group).
func (Buildings) Lift(zs *zone.Map, b construction.Building) {
	zk, _, ok := ZoneKind(b.Kind)
	if !ok {
		return
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	var gone [][2]int
	for _, c := range cells {
		if id, in := zs.At(c[0], c[1]); in {
			if z, _ := zs.Zone(id); z.Kind == zk {
				zs.Unlink(id)
				gone = append(gone, c)
			}
		}
	}
	zs.RemoveTiles(gone)
}

// Link links storage building target into building leader's storage
// group (LinkStorage): target takes the group's settings. Buildings of
// different kinds never link.
func (s Buildings) Link(zs *zone.Map, m Machines, target, leader entity.EntityID) bool {
	t, ok := s.ZoneOf(zs, m, target)
	l, lok := s.ZoneOf(zs, m, leader)
	return ok && lok && zs.Link(t.ID, l.ID)
}

// Unlink takes storage building target out of its group (UnlinkStorage).
func (s Buildings) Unlink(zs *zone.Map, m Machines, target entity.EntityID) bool {
	t, ok := s.ZoneOf(zs, m, target)
	return ok && zs.Unlink(t.ID)
}

// Fill is the storage readout of zone z for the views: its group, the
// stacks lying in it and how many it holds (a filled grave holds its one
// body).
func (Buildings) Fill(zs *zone.Map, l Larder, z zone.Zone, filled bool) (group uint64, stored, capacity int) {
	if !z.Kind.Settings() {
		return 0, 0, 0
	}
	if z.Kind == zone.KindGrave {
		if filled {
			stored = 1
		}
		return uint64(z.Group), stored, 1
	}
	zs.EachCell(z.ID, func(x, y int) bool { stored += l.StacksAt(x, y); return true })
	return uint64(z.Group), stored, z.Cells * z.Kind.Stacks()
}

// Grave is grave or sarcophagus bid's settings (its zone, else a new
// one's: an older save's grave) and whether they take body st (colonist:
// the dead was one of the colony).
func (s Buildings) Grave(zs *zone.Map, m Machines, bid entity.EntityID, st item.Stack, colonist bool) (zone.Zone, bool) {
	b, ok := m.Building(bid)
	if !ok || b.Kind != construction.Grave && b.Kind != construction.Sarcophagus {
		return zone.Zone{}, false
	}
	z, ok := s.ZoneOf(zs, m, bid)
	if !ok {
		_, z, _ = ZoneKind(b.Kind)
	}
	return z, z.Allows(st.Kind) && z.Special.TakesBody(st, colonist)
}

// LayMissing lays the zone of every storage building without one (an
// older save's graves).
func (s Buildings) LayMissing(zs *zone.Map, m Machines) {
	for _, bid := range m.BuildingIDs() {
		if b, ok := m.Building(bid); ok {
			if _, has := s.ZoneOf(zs, m, bid); !has {
				cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
				s.Lay(zs, m, b.Kind, cells)
			}
		}
	}
}

// AnyGrave reports some grave or sarcophagus not filled takes body st.
func (s Buildings) AnyGrave(zs *zone.Map, m Machines, st item.Stack, colonist bool, filled func(entity.EntityID) bool) bool {
	for _, bid := range m.BuildingIDs() {
		if _, ok := s.Grave(zs, m, bid, st, colonist); ok && !filled(bid) {
			return true
		}
	}
	return false
}
