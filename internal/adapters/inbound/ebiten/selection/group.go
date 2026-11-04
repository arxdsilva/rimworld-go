package selection

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// InView reports the centre of the world cell with corner (wx, wy) lies in
// the area asked about: the map's view for a double click, the dragged
// rectangle for a box. The renderer knows the camera; these rules do not.
type InView func(wx, wy float64) bool

// Multi reports a building can join a group. What comes in long rows (walls,
// fences, floors, carpet, conduits) and the hopper are worked on one at a
// time; a plain click still selects them.
func Multi(b game.BuildingView) bool {
	if b.Floor {
		return false
	}
	switch b.Kind {
	case construction.Wall.String(), construction.Hopper.String(), construction.Fence.String(),
		construction.Carpet.String(), construction.PowerConduit.String(),
		construction.HiddenConduit.String(), construction.WaterproofConduit.String():
		return false
	}
	return true
}

// paintedZone reports a zone the player laid; a shelf's, hopper's, grave's or
// bookcase's zone belongs to its building and never groups.
func paintedZone(z *game.ZoneView) bool {
	switch z.Kind {
	case game.ZoneStockpile, game.ZoneGrow, game.ZonePen, game.ZoneDump:
		return true
	}
	return false
}

func buildingByID(w game.WorldView, id entity.EntityID) *game.BuildingView {
	for i := range w.Buildings {
		if w.Buildings[i].ID == id {
			return &w.Buildings[i]
		}
	}
	return nil
}

func frameByID(w game.WorldView, id entity.EntityID) *game.FrameView {
	for i := range w.Frames {
		if w.Frames[i].ID == id {
			return &w.Frames[i]
		}
	}
	return nil
}

func zoneByID(w game.WorldView, id entity.EntityID) *game.ZoneView {
	for i := range w.Zones {
		if entity.EntityID(w.Zones[i].ID) == id {
			return &w.Zones[i]
		}
	}
	return nil
}

// CanGroup reports c may join a group; a thing that cannot takes a plain
// click instead.
func (m *Model) CanGroup(c Cand) bool {
	w := m.sh.World()
	switch c.Kind {
	case Pawn, Animal, Item:
		return true
	case Building:
		b := buildingByID(w, c.ID)
		return b != nil && Multi(*b)
	case Frame:
		f := frameByID(w, c.ID)
		return f != nil && !f.Floor
	case Zone:
		z := zoneByID(w, c.ID)
		return z != nil && paintedZone(z)
	}
	return false
}

// footprintIn reports the centre of a w x h footprint lies in view.
func footprintIn(in InView, x, y, w, h int) bool {
	return in(float64(x)+float64(max(w, 1))/2-0.5, float64(y)+float64(max(h, 1))/2-0.5)
}

func buildingIn(w game.WorldView, b *game.BuildingView, in InView) bool {
	return !fogged(w, b.X, b.Y) && footprintIn(in, b.X, b.Y, b.W, b.H)
}

func frameIn(w game.WorldView, f *game.FrameView, in InView) bool {
	return !fogged(w, f.X, f.Y) && footprintIn(in, f.X, f.Y, f.W, f.H)
}

// zoneIn reports a zone the player can see has a cell in view; a hidden one
// shows nothing on the map, so nothing takes it unasked.
func zoneIn(w game.WorldView, z *game.ZoneView, in InView) bool {
	if z.Hidden {
		return false
	}
	for _, c := range z.Cells {
		if !fogged(w, c[0], c[1]) && in(float64(c[0]), float64(c[1])) {
			return true
		}
	}
	return false
}

// Alike is the double click on a building, blueprint, zone or a plant in a
// zone: the group it stands for, what was clicked first. False means no
// group: a wall, a floor's blueprint, a wild plant and the like take a plain
// click.
func Alike(w game.WorldView, c Cand, in InView) ([]Cand, bool) {
	switch c.Kind {
	case Building:
		b := buildingByID(w, c.ID)
		if b == nil || !Multi(*b) {
			return nil, false
		}
		out := []Cand{c}
		for i := range w.Buildings {
			if o := &w.Buildings[i]; o.ID != c.ID && o.Kind == b.Kind && buildingIn(w, o, in) {
				out = append(out, Cand{Kind: Building, ID: o.ID})
			}
		}
		return out, true
	case Frame:
		f := frameByID(w, c.ID)
		if f == nil || f.Floor {
			return nil, false
		}
		out := []Cand{c}
		for i := range w.Frames {
			if o := &w.Frames[i]; o.ID != c.ID && o.Kind == f.Kind && !o.Floor && frameIn(w, o, in) {
				out = append(out, Cand{Kind: Frame, ID: o.ID})
			}
		}
		return out, true
	case Zone:
		z := zoneByID(w, c.ID)
		if z == nil || !paintedZone(z) {
			return nil, false
		}
		return alikeZones(w, z, in), true
	case Plant:
		for i := range w.Plants {
			if p := &w.Plants[i]; p.ID == c.ID {
				z, ok := zoneCellAt(w, p.X, p.Y)
				if !ok || !paintedZone(&z) {
					return nil, false
				}
				return alikeZones(w, &z, in), true
			}
		}
	}
	return nil, false
}

// alikeZones is z and the other zones of its kind in view.
func alikeZones(w game.WorldView, z *game.ZoneView, in InView) []Cand {
	out := []Cand{{Kind: Zone, ID: entity.EntityID(z.ID)}}
	for i := range w.Zones {
		if o := &w.Zones[i]; o.ID != z.ID && o.Kind == z.Kind && zoneIn(w, o, in) {
			out = append(out, Cand{Kind: Zone, ID: entity.EntityID(o.ID)})
		}
	}
	return out
}

// Boxed lists what a dragged box takes after colonists, animals and items
// (the renderer's hit test): one kind, buildings that can group, else
// blueprints, else zones.
func Boxed(w game.WorldView, in InView) (Kind, []entity.EntityID) {
	var ids []entity.EntityID
	for i := range w.Buildings {
		if b := &w.Buildings[i]; Multi(*b) && buildingIn(w, b, in) {
			ids = append(ids, b.ID)
		}
	}
	if len(ids) > 0 {
		return Building, ids
	}
	for i := range w.Frames {
		if f := &w.Frames[i]; !f.Floor && frameIn(w, f, in) {
			ids = append(ids, f.ID)
		}
	}
	if len(ids) > 0 {
		return Frame, ids
	}
	for i := range w.Zones {
		if z := &w.Zones[i]; paintedZone(z) && zoneIn(w, z, in) {
			ids = append(ids, entity.EntityID(z.ID))
		}
	}
	if len(ids) > 0 {
		return Zone, ids
	}
	return None, nil
}
