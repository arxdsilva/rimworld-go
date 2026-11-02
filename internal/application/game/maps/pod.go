package maps

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Transport pods landing on an open map (MAP-10; wiki: Transport pod):
// the people and goods come down at the map's centre or at a chosen spot
// (the nearest cell a pod can reach: pods pass through roofs but not
// overhead mountain), not walking in from the edge.

// LandOn has pod passengers (stashed, with their animals) come down on
// the open map at world tile t at the end of the tick: at its centre, or
// at spot (x, y). It returns the carrier caravan's id (0: no map there).
func (f Fleet) LandOn(members, animals []uint64, t int, centre, spot bool, x, y int) uint64 {
	id, ok := f.mapAt(t)
	if !ok {
		return 0
	}
	home := f.h.ColonyTile()
	c := Caravan{C: caravan.Caravan{ID: f.h.NewID(), Members: members, Animals: animals, Tile: t, Dest: t,
		State: caravan.Arrived, FoodDays: FoodDays}, Home: home, Out: f.h.Planet().Route(home, t)}
	for _, m := range members {
		f.h.Rehold(entity.EntityID(m), entity.EntityID(c.C.ID))
	}
	*f.list = append(*f.list, c)
	f.a.Queue = append(f.a.Queue, Request{Op: OpEnter, Map: id, Caravan: c.C.ID, Centre: centre, Spot: spot, X: x, Y: y})
	return c.C.ID
}

// landOn carries out an OpEnter request that names a landing spot.
func (l Lifecycle) landOn(c *Caravan, r Request) {
	f := l.f
	if !f.h.Switch(r.Map) {
		f.h.Switch(0)
	}
	x, y := -1, -1
	if r.Spot {
		x, y = r.X, r.Y
	}
	lx, ly, ok := f.h.Landing(x, y)
	if !ok {
		lx, ly, ok = f.h.Edge()
	}
	if !ok {
		lx, ly = f.h.Home()
	}
	names := f.setDown(c, lx, ly)
	if len(names) > 0 {
		f.h.Letter(false, "Transport pods landed", fmt.Sprintf("%s came down at %s.", travel.JoinNames(names), f.h.TileName(c.C.Tile)), entity.EntityID(c.C.Members[0]))
	} else {
		f.h.Letter(false, "Transport pods landed", fmt.Sprintf("The pods came down at %s.", f.h.TileName(c.C.Tile)), 0)
	}
	f.drop(entity.EntityID(c.C.ID))
}

// setDown puts c's members and animals on free cells around (ex, ey) on
// the current map, its goods beside them. It names the members.
func (f Fleet) setDown(c *Caravan, ex, ey int) []string {
	c.C.State = caravan.Home
	var used [][2]int
	var names []string
	for i, m := range append(slices.Clip(c.C.Members), c.C.Animals...) {
		id := entity.EntityID(m)
		if !f.h.IsStashed(id) {
			continue
		}
		x, y, ok := f.h.FreeLand(ex, ey, 4, used)
		if !ok {
			if x, y, ok = f.h.WalkableNear(ex, ey, 6); !ok {
				x, y = ex, ey
			}
		}
		used = append(used, [2]int{x, y})
		if f.h.Unstash(id, x, y) && i < len(c.C.Members) {
			names = append(names, f.h.Name(id))
		}
	}
	f.h.Cargo().Home(c.C.ID, ex, ey)
	return names
}

// MapOpen reports a map open on world tile t (the first colony's too).
func (f Fleet) MapOpen(t int) bool {
	_, ok := f.mapAt(t)
	return ok
}
