package rest

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/room"
)

// Beauty a pawn sees (BED-3; wiki: Beauty). It looks at the 241 cells
// within about 8 tiles that share its room (or, outdoors, the open ground
// around it): the room boundary stands in for line of sight. Each cell
// adds its floor and filth (room.FloorBeautyTenths, FilthBeautyTenths),
// the building whose corner stands on it (room.BuildingBeauty, so a
// many-celled building counts once) and its plant; the pawn perceives the
// average. A look reads about 241 cells through the host's cell indexes
// every needs.BeautySampleTicks per awake pawn, a cell or so a tick per
// pawn, so there is no per-cell cache to keep in step with building,
// floor, filth and plant changes.

// beautyReachSq is the squared radius of the cells a pawn looks at: the
// 241 cells of the wiki's radius 8.
const beautyReachSq = 74

// Perceived is the average beauty, tenths of a point, a pawn standing on
// (x, y) sees.
func (n *Needs) Perceived(x, y int) int {
	r := n.h.RoomAt(x, y)
	if r == room.Wall {
		r = room.Outdoors // in a doorway: the open side
	}
	sum, cells := 0, 0
	for dy := -8; dy <= 8; dy++ {
		for dx := -8; dx <= 8; dx++ {
			if dx*dx+dy*dy > beautyReachSq || n.h.RoomAt(x+dx, y+dy) != r {
				continue
			}
			sum += n.TileBeauty(x+dx, y+dy)
			cells++
		}
	}
	return sum / max(1, cells)
}

// TileBeauty is the beauty of cell (x, y), tenths of a point.
func (n *Needs) TileBeauty(x, y int) int {
	t := n.h.Tile(x, y)
	f, b := t.Floor, 0
	if o, ok := n.h.Building(t.Object); ok {
		switch {
		case o.Kind == construction.Carpet:
			f = room.CarpetFloor
		case o.X == x && o.Y == y:
			b = room.BuildingBeauty(o.Kind, o.Stuff, qualityOf(o)) / 100
		}
	}
	b += room.FloorBeautyTenths(f, t.Indoors) + room.FilthBeautyTenths(t.Filth, t.Indoors)
	if t.Planted && t.Mature && (plant.DefOf(t.Plant).Trunk || t.Plant == plant.FireLily || t.Plant == plant.Dandelion) {
		b += plantBeautyTenths
	}
	return b
}

// plantBeautyTenths is a grown tree's or a flowering wild plant's beauty
// (+1). The wiki publishes no plant beauty: original tuning.
const plantBeautyTenths = 10
