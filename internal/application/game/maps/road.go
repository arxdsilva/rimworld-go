package maps

import (
	"math"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// On the road (ENG-003 M9). A caravan travels the planet route on the
// hour at the pace of its slowest member. At a friendly or neutral
// settlement it visits for a day (and restocks), then comes back the way
// it went and its members walk in from an edge cell. Away, members eat
// the caravan's food; once it runs out they go hungry. Members talk among
// themselves every few hours (the caravan is one room), and the best
// medic among them tends the others hourly (WS-M).

// Step advances every travelling caravan on the hour, ends visits, lets
// members talk and starve, and brings the caravans home.
func (f Fleet) Step(now uint64) {
	hour := now%clock.TicksPerHour == 0
	dm := f.difficulty(now)
	for i := 0; i < len(*f.list); i++ {
		c := &(*f.list)[i]
		id := entity.EntityID(c.C.ID)
		switch c.C.State {
		case caravan.Forming:
			continue
		case caravan.Travelling, caravan.Returning:
			if !hour {
				break
			}
			if c.Ask.Letter != 0 { // a demand holds it up (encounter.go)
				if now >= c.Ask.Until {
					f.Answer(c.Ask.Letter, 1)
				}
				break
			}
			caravan.Advance(&c.C, int(now%clock.TicksPerDay/clock.TicksPerHour), dm)
			if c.C.FoodDays == 0 {
				for _, m := range c.C.Members {
					f.h.Starve(entity.EntityID(m))
				}
			}
			if c = f.At(id); c != nil && caravan.IsArrived(&c.C) {
				f.Arrived(c, now)
			} else if c != nil {
				f.roll(c, now) // encounter.go (MAP-08)
			}
		case caravan.Visiting:
			if now >= c.C.VisitUntil && !f.h.HoldsVisit(c.C.Tile) {
				f.startReturn(c)
			}
		}
		if c = f.At(id); c == nil {
			i--
			continue
		}
		if len(c.C.Members) == 0 {
			f.h.Letter(true, "Caravan lost", 0, "Nobody in the caravan survived the journey.")
			*f.list = slices.Delete(*f.list, i, i+1)
			i--
			continue
		}
		if c.C.State == caravan.Home {
			*f.list = slices.Delete(*f.list, i, i+1)
			i--
			continue
		}
		if hour && c.C.State != caravan.Forming {
			f.care(c, now) // WS-M: the medic on the road
		}
		if now >= c.NextTalk {
			c.NextTalk = now + talkPeriod
			f.talk(c, now)
		}
	}
}

// Arrived: c reached the end of its path: home (or back to the map it
// left), an open map (it enters), a mineral site (a site map opens), a
// friendly settlement (a day's visit, food restocked) or anywhere else
// (it waits for orders: return, or EnterMap).
func (f Fleet) Arrived(c *Caravan, now uint64) {
	if c.Back {
		id, ok := f.mapAt(c.Home)
		if !ok {
			id = 0 // the map it left is gone: the first colony
		}
		f.enterMap(c, id)
		return
	}
	enc := c.NextEnc
	f.h.CaravanArrived(c.C.ID, c.C.Tile) // quests (QST-074): a site, a delivery or talks here
	if c = f.At(entity.EntityID(c.C.ID)); c == nil || c.NextEnc != enc {
		return // the talks went wrong: an ambush (Fleet.Ambush) has it
	}
	if id, ok := f.mapAt(c.C.Tile); ok {
		f.enterMap(c, id)
		return
	}
	if q, ok := f.h.SiteAt(c.C.Tile); ok { // a quest's site (QST-103): its map opens
		c.C.State = caravan.Arrived
		f.a.Queue = append(f.a.Queue, Request{Op: OpOpen, Kind: Site, Tile: c.C.Tile, Caravan: c.C.ID, Quest: q})
		return
	}
	if lump, qty, ok := f.h.Cargo().Claim(c.C.Tile); ok { // game/travel (AX-9): a mineral site (MAP-04)
		c.C.State = caravan.Arrived
		f.a.Queue = append(f.a.Queue, Request{Op: OpOpen, Kind: Site, Tile: c.C.Tile, Caravan: c.C.ID, Lump: lump, Qty: qty})
		return
	}
	fa := f.h.Faction(c.C.Tile)
	if fa != nil && fa.Type.Human() && fa.Relation != faction.Hostile && !fa.Defeated {
		c.C.State = caravan.Visiting
		c.C.VisitUntil = now + visitTicks
		c.C.FoodDays = FoodDays
		f.h.Letter(false, "Caravan arrived", 0, "The caravan reached %s and stays a day as their guests.", f.h.TileName(c.C.Tile))
		return
	}
	f.h.Letter(false, "Caravan arrived", 0, "The caravan reached %s and waits there for your orders.", f.h.TileName(c.C.Tile))
}

// enterMap brings c onto map id: at once when it is the map being
// stepped (the first colony's pass, as before MAP-01), else at the end of
// the tick (Flush). Meanwhile the caravan waits where it is.
func (f Fleet) enterMap(c *Caravan, id uint32) {
	if id == f.h.Current() {
		f.Home(c)
		return
	}
	c.C.State = caravan.Arrived
	f.a.Queue = append(f.a.Queue, Request{Op: OpEnter, Map: id, Caravan: c.C.ID})
}

// Travel is command.CaravanTravel: caravan id heads for world tile t from
// where it is. The way back stays the way it came.
func (f Fleet) Travel(id entity.EntityID, t int) {
	c, p := f.At(id), f.h.Planet()
	if c == nil || p == nil {
		return
	}
	if c.C.State == caravan.Forming || c.C.State == caravan.Home || c.Ask.Letter != 0 {
		f.h.Note("The caravan cannot change course now.")
		return
	}
	route := p.Route(c.C.Tile, t)
	if len(route) < 2 {
		f.h.Note("The caravan has no way to get there.")
		return
	}
	k := max(slices.Index(c.Out, c.C.Tile), 0)
	c.Out = append(slices.Clone(c.Out[:k]), route...)
	c.C.Path, c.C.Progress, c.C.Dest, c.Back = slices.Clone(route[1:]), 0, t, false
	c.C.State = caravan.Travelling
}

// mapAt is the open map on world tile t.
func (f Fleet) mapAt(t int) (uint32, bool) {
	if t >= 0 && t == f.h.ColonyTile() {
		return 0, true
	}
	for _, m := range f.a.Maps {
		if m.Tile == t {
			return m.ID, true
		}
	}
	return 0, false
}

// tileOf is open map id's world tile (-1: none).
func (f Fleet) tileOf(id uint32) int {
	if m := f.a.Meta(id); m != nil {
		return m.Tile
	}
	return f.h.ColonyTile()
}

// startReturn turns c for home along the way it came.
func (f Fleet) startReturn(c *Caravan) {
	if _, ok := f.mapAt(c.Home); !ok { // the map it left has closed: home is the first colony
		if p, home := f.h.Planet(), f.h.ColonyTile(); p != nil {
			if route := p.Route(c.C.Tile, home); len(route) > 0 {
				c.Home, c.Out = home, slices.Clone(route)
				slices.Reverse(c.Out)
			}
		}
	}
	k := slices.Index(c.Out, c.C.Tile)
	if k < 0 {
		k = 0
	}
	back := slices.Clone(c.Out[:k])
	slices.Reverse(back)
	c.C.Path, c.C.Progress, c.Back = back, 0, true
	c.C.State = caravan.Returning
	if len(back) == 0 {
		f.Arrived(c, f.h.Now())
	}
}

// Home brings c's members back onto the current map; they walk in
// towards its centre.
func (f Fleet) Home(c *Caravan) {
	if names := f.walkIn(c); len(names) > 0 {
		f.h.Letter(false, "Caravan returned", entity.EntityID(c.C.Members[0]), "%s came back from the journey.", travel.JoinNames(names))
	}
}

// walkIn brings c's members (and the tame animals riding with it) onto
// the current map at an edge cell, its goods set down there; they walk in
// towards its centre. It names the members who came in.
func (f Fleet) walkIn(c *Caravan) []string {
	c.C.State = caravan.Home
	ex, ey, ok := f.h.Edge()
	if !ok {
		ex, ey = f.h.Home()
	}
	hx, hy := f.h.Home()
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
				x, y = hx, hy
			}
		}
		used = append(used, [2]int{x, y})
		if !f.h.Unstash(id, x, y) {
			continue
		}
		if i < len(c.C.Members) {
			names = append(names, f.h.Name(id))
		}
		if tx, ty, ok := f.h.HomeSpot(i); ok {
			f.h.Move(id, tx, ty)
		}
	}
	f.h.Cargo().Home(c.C.ID, ex, ey) // game/travel (AX-9): the pods' cargo
	return names
}

// talk pairs two members up for a conversation on the road.
func (f Fleet) talk(c *Caravan, now uint64) {
	var able []entity.EntityID
	for _, m := range c.C.Members {
		if id := entity.EntityID(m); f.h.Alive(id) {
			able = append(able, id)
		}
	}
	if len(able) < 2 {
		return
	}
	i, k2 := f.h.Roll(len(able)), f.h.Roll(len(able)-1)
	if k2 >= i {
		k2++
	}
	a, b := able[i], able[k2]
	if f.h.Capacity(a, body.Talking) <= 0 || f.h.Capacity(b, body.Hearing) <= 0 {
		return
	}
	k := social.Chitchat
	if f.h.Roll(100) < deepTalkPct {
		k = social.DeepTalk
	}
	f.h.Talk(a, b, k, talkJoy, now)
}

// difficulty is the planet's movement difficulty of a tile on now's day,
// in thousandths (1000 without a planet; MAP-09, WG-03).
func (f Fleet) difficulty(now uint64) func(t int) int {
	p, day := f.h.Planet(), int(now/clock.TicksPerDay)
	return func(t int) int {
		if p == nil {
			return 1000
		}
		return int(math.Round(p.MoveDifficulty(t, day) * 1000))
	}
}

// Estimate is how long caravan id would take from where it is to world
// tile t, nights included (MAP-09), and why it cannot ("" when it can).
func (f Fleet) Estimate(id entity.EntityID, t int, now uint64) (int, string) {
	c, p := f.At(id), f.h.Planet()
	switch {
	case c == nil:
		return 0, "No such caravan."
	case p == nil:
		return 0, "No world map."
	case t == c.C.Tile:
		return 0, ""
	}
	route := p.Route(c.C.Tile, t)
	if len(route) < 2 {
		return 0, "No way to get there."
	}
	cp := Caravan{C: c.C}
	if cp.C.State == caravan.Forming || cp.C.Slowest == 0 {
		f.pace(&cp) // not set out yet: its pace as it would leave
	}
	ticks := caravan.Estimate(route[1:], cp.C.Speed(), int(now%clock.TicksPerDay/clock.TicksPerHour), f.difficulty(now))
	if ticks < 0 {
		return 0, "Too slow to get there."
	}
	return ticks, ""
}
