package maps

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Founding and abandoning colonies (MAP-06, MAP-07; wiki: Caravan, Colony).
// A caravan waiting on a tile settles it: refused at the colony cap
// (Options, Gameplay, Max colonies) or on or next to a settlement. A colony
// map opens there, or the temporary map already there becomes one. A
// colony never closes by itself. Abandoning one loses its pawns and items,
// carried ones too; abandoning the first colony makes the next colony
// first, and with no other colony the map stays, emptied.

// ColonyCapLimit is the highest colony cap Options offers (wiki: Colony).
const ColonyCapLimit = 5

// PickWeighted is the index a weighted roll lands on (-1: none to pick;
// one entry draws nothing).
func PickWeighted(weights []int, intN func(int) int) int {
	if len(weights) < 2 {
		return len(weights) - 1
	}
	total := 0
	for _, w := range weights {
		total += w
	}
	x := intN(total)
	for i, w := range weights {
		if x -= w; x < 0 {
			return i
		}
	}
	return -1
}

// Founder is the simulation's side of founding and abandoning.
type Founder interface {
	// MaxColonies is the colony cap (at most 0 means 1).
	MaxColonies() int
	// ColonySide is a new colony map's side.
	ColonySide() int
	// StartHome lays the current map's Home area around its landing spot
	// and lets wildlife migrate in.
	StartHome()
	// Promote makes colony map next the first map, under id 0; the old
	// first map takes id next. land is next's landing spot.
	Promote(next uint32, land [2]int, hasLand bool)
	// Wipe removes the current map's pawns and items; the map stays.
	Wipe()
}

// colonies counts the open colony maps, the first map among them.
func (a *Atlas) colonies() int {
	n := 1
	for _, m := range a.Maps {
		if m.Kind == Colony {
			n++
		}
	}
	return n
}

// FoundReason is why a colony cannot be founded on tile ("" it can).
func (f Fleet) FoundReason(tile int) string {
	p := f.h.Planet()
	if p == nil {
		return "No world map."
	}
	if id, ok := f.mapAt(tile); ok && (id == 0 || f.a.Meta(id).Kind == Colony) {
		return "There is already a colony here."
	}
	if limit := max(f.h.MaxColonies(), 1); f.a.colonies() >= limit {
		return fmt.Sprintf("You already have the most colonies allowed (%d). Options, Gameplay, Max colonies.", limit)
	}
	if ok, why := p.ValidLanding(tile); !ok {
		return why
	}
	return ""
}

// SettleReason is why caravan id cannot settle where it is ("" it can).
func (f Fleet) SettleReason(id entity.EntityID) string {
	c := f.At(id)
	switch {
	case c == nil:
		return "No such caravan."
	case c.Back || c.C.State != caravan.Arrived && c.C.State != caravan.Visiting:
		return "Only a caravan waiting at its destination can settle."
	}
	return f.FoundReason(c.C.Tile)
}

// Found has caravan cid found a colony on its tile at the end of the tick:
// why it cannot ("" it will).
func (f Fleet) Found(cid uint64) string {
	id := entity.EntityID(cid)
	if why := f.SettleReason(id); why != "" {
		return why
	}
	c := f.At(id)
	c.C.State = caravan.Arrived
	f.a.Queue = append(f.a.Queue, Request{Op: OpSettle, Kind: Colony, Tile: c.C.Tile, Caravan: cid})
	return ""
}

// settle carries out OpSettle: the new colony's map, its Home, and the
// caravan walking in. Refused now (another settled first) it keeps waiting.
func (l Lifecycle) settle(r Request, now uint64) {
	f := l.f
	c := f.At(entity.EntityID(r.Caravan))
	if c == nil {
		return
	}
	if why := f.FoundReason(r.Tile); why != "" {
		f.h.Note(why)
		return
	}
	id, ok := f.mapAt(r.Tile)
	if ok {
		m := f.a.Meta(id) // a temporary map becomes the colony
		m.Kind, m.PackBy, m.Lump, m.Cells, m.Hostile, m.Empty, m.Done = Colony, 0, 0, nil, false, 0, 0
	} else if id, ok = l.open(r, now); !ok {
		f.h.Note("The caravan could not find its way onto that ground.")
		return
	}
	if !f.h.Switch(id) {
		return
	}
	f.h.StartHome()
	cid := entity.EntityID(c.C.ID)
	if names := f.walkIn(c); len(names) > 0 {
		f.h.Letter(false, "Colony founded", fmt.Sprintf("%s founded a new colony.", travel.JoinNames(names)), entity.EntityID(c.C.Members[0]))
	}
	f.drop(cid)
}

// abandon is OpClose: map id closes and everything left on it is lost. The
// first map hands over to the next colony; with none it stays, emptied.
func (l Lifecycle) abandon(id uint32) {
	f := l.f
	if id != 0 {
		if f.a.Meta(id) != nil {
			l.close(id)
		}
		return
	}
	i := slices.IndexFunc(f.a.Maps, func(m Meta) bool { return m.Kind == Colony })
	if i < 0 {
		if f.h.Switch(0) {
			f.h.Wipe()
		}
		f.h.Letter(true, "Colony abandoned", "Nothing is left of the colony. Only those still on the road remain.", 0)
		return
	}
	next := f.a.Maps[i]
	f.h.Promote(next.ID, next.Land, next.HasLand)
	for k := range *f.list {
		switch c := &(*f.list)[k]; c.Map {
		case 0:
			c.Map = next.ID
		case next.ID:
			c.Map = 0
		}
	}
	l.close(next.ID) // the old first map, under its new id
	f.h.Switch(0)
	f.h.Letter(true, "Colony abandoned", "The colony was abandoned; whatever was left there is lost.", 0)
}
