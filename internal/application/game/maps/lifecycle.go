package maps

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Lifecycle is the open maps' requests and hourly upkeep (MAP-01..04).
type Lifecycle struct{ f Fleet }

// Life is the fleet's map lifecycle.
func (f Fleet) Life() Lifecycle { return Lifecycle{f} }

// Apply takes a map command: EnterMap (caravan Target opens or enters the
// map at its tile), CaravanReform (the pawns on map Target leave as a
// caravan), AbandonMap (map Target closes), and the settling, encounter and
// travel orders. Whatever changes a map is applied at the end of the tick.
func (l Lifecycle) Apply(c command.Command) {
	f := l.f
	switch c.Kind {
	case command.AttackSettlement:
		l.Attack(c) // settle.go (MAP-05)
	case command.SettleHere:
		if why := f.Found(c.Target); why != "" { // found.go (MAP-06)
			f.h.Note("%s", why)
		}
	case command.EncounterAnswer:
		f.Answer(c.Target, c.Mode) // encounter.go (MAP-08)
	case command.CaravanTravel:
		f.Travel(entity.EntityID(c.Target), c.Count) // road.go
	case command.EnterMap:
		cv := f.At(entity.EntityID(c.Target))
		if cv == nil || cv.C.State != caravan.Arrived || cv.Back {
			f.h.Note("Only a caravan waiting at its destination can enter it.")
			return
		}
		if id, ok := f.mapAt(cv.C.Tile); ok {
			f.enterMap(cv, id)
			return
		}
		f.a.Queue = append(f.a.Queue, Request{Op: OpOpen, Kind: Site, Tile: cv.C.Tile, Caravan: cv.C.ID})
	case command.AbandonMap:
		id := uint32(c.Target)
		if m := f.a.Meta(id); id != 0 && (m == nil || m.Kind == Settlement) {
			f.h.Note("That map cannot be left that way.")
			return
		}
		f.a.Queue = append(f.a.Queue, Request{Op: OpClose, Map: id}) // found.go (MAP-07)
	case command.CaravanReform:
		m := f.a.Meta(uint32(c.Target))
		if m == nil {
			f.h.Note("That map cannot be left that way.")
			return
		}
		f.a.Queue = append(f.a.Queue, Request{Op: OpReform, Map: m.ID})
	}
}

// Flush applies the queued requests, then on the hour takes each
// temporary map's census: the day-10 raid on a mineral site, the pack-up
// countdown once its job is done, the reform when it runs out, and the
// close once nobody of ours is left. It runs once per tick, after every
// map's phases; it leaves any map current (the caller restores its own).
func (l Lifecycle) Flush(now uint64) {
	a := l.f.a
	for len(a.Queue) > 0 {
		r := a.Queue[0]
		a.Queue = slices.Delete(a.Queue, 0, 1)
		l.apply(r, now)
	}
	if now%clock.TicksPerHour == 0 {
		for _, id := range l.ids() {
			l.hourly(id, now)
		}
	}
	for i, fs := 0, l.f.h.Factions(); i < len(fs); i++ {
		fs[i].Besieged = a.Attacking(i + 1)
	}
}

// ids is the open temporary maps' ids.
func (l Lifecycle) ids() []uint32 {
	var out []uint32
	for _, m := range l.f.a.Maps {
		out = append(out, m.ID)
	}
	return out
}

// apply carries out request r.
func (l Lifecycle) apply(r Request, now uint64) {
	f := l.f
	switch r.Op {
	case OpOpen:
		c := f.At(entity.EntityID(r.Caravan))
		if c == nil {
			return
		}
		id, ok := f.mapAt(r.Tile)
		if !ok {
			if id, ok = l.open(r, now); !ok {
				f.h.Note("The caravan could not find its way onto that ground.")
				if r.Kind == Encounter {
					c.C.State = r.Resume // nowhere to fight: it goes on
				}
				return
			}
			if r.Quest != 0 && f.h.Switch(id) {
				f.h.SiteOpened(r.Quest, id) // the quest lays its site (QST-104)
			}
		}
		if r.Kind == Encounter {
			l.waylay(c, id, r) // encounter.go (MAP-08)
			return
		}
		if r.Kind == Settlement {
			l.storm(c, id, r) // settle.go (MAP-05): defenders, then the edge or centre
			return
		}
		l.enter(c, id)
	case OpEnter:
		if c := f.At(entity.EntityID(r.Caravan)); c != nil && (r.Centre || r.Spot) {
			l.landOn(c, r) // pod.go (MAP-10)
		} else if c != nil {
			l.enter(c, r.Map)
		}
	case OpReform:
		l.reform(r.Map, now)
	case OpClose:
		l.abandon(r.Map) // found.go (MAP-07)
	case OpSettle:
		l.settle(r, now) // found.go (MAP-06)
	}
}

// Open opens a map of kind k on world tile now, outside the queue (a
// dev hook; play opens maps only in Flush): its id.
func (l Lifecycle) Open(k Kind, tile int, now uint64) (uint32, bool) {
	return l.open(Request{Kind: k, Tile: tile}, now)
}

// open opens a map for r: its id.
func (l Lifecycle) open(r Request, now uint64) (uint32, bool) {
	a, h := l.f.a, l.f.h
	id := a.newID()
	side := SizeFor(r.Kind, h.ColonySide())
	o, ok := h.Open(id, r.Tile, side, side, SeedFor(h.Seed(), r.Tile, id), r.Lump, r.Qty)
	if !ok {
		return 0, false
	}
	m := a.add(id, r.Kind, r.Tile, now)
	m.Land, m.HasLand, m.Quest = o.Land, o.HasLand, r.Quest
	if len(o.Cells) > 0 {
		m.Lump, m.Cells = r.Lump, o.Cells
	}
	return id, true
}

// enter unstashes caravan c onto map id (the first colony when id has
// closed meanwhile) and dissolves it.
func (l Lifecycle) enter(c *Caravan, id uint32) {
	f := l.f
	if !f.h.Switch(id) {
		f.h.Switch(0)
	}
	cid := entity.EntityID(c.C.ID)
	if c.Back {
		f.Home(c)
	} else if names := f.walkIn(c); len(names) > 0 {
		f.h.Letter(false, "Caravan entered", entity.EntityID(c.C.Members[0]), "%s entered %s.", travel.JoinNames(names), f.h.TileName(c.C.Tile))
	}
	f.drop(cid)
}

// hourly is map id's census.
func (l Lifecycle) hourly(id uint32, now uint64) {
	f := l.f
	m := f.a.Meta(id)
	if m.Kind == Colony {
		return // a colony never closes by itself
	}
	if !f.h.Switch(id) {
		l.forgetMeta(id)
		return
	}
	c := f.h.Census(m.Cells)
	c.Exiting = f.exiting(id)
	if c.Hostiles > 0 {
		m.Hostile = true
	}
	if m.Kind == Settlement {
		l.besiege(m, now) // settle.go (MAP-05): defeat after 3 hourly results with no defender
	}
	if m.Lump != 0 && !m.Raided && m.PackBy == 0 && now >= m.Opened+raidDays*clock.TicksPerDay {
		m.Raided = true
		f.h.Raid() // wiki: Long-range mineral scanner: hostiles come after 10 days
	}
	busy := m.Quest != 0 && f.h.SiteCensus(m.Quest, id, c) // the quest's own rules (QST-108)
	if m = f.a.Meta(id); m == nil {
		return // the quest closed it
	}
	if ShouldClose(m, c) {
		l.close(id)
		return
	}
	if m.PackBy == 0 && streak(&m.Done, !busy && JobDone(m, c)) {
		m.PackBy = now + packTicks
		f.h.Letter(false, "Time to pack up", 0, "The work at %s is done. Whoever is still there leaves as a caravan in a day.", f.h.TileName(m.Tile))
	}
	if m.PackBy != 0 && now >= m.PackBy {
		l.reform(id, now)
	}
}

// reform has our people and tame animals on map id leave at once as a
// caravan (with the map's unforbidden goods), heading for the map's
// resume destination or home; then the map closes.
func (l Lifecycle) reform(id uint32, now uint64) {
	f := l.f
	m := f.a.Meta(id)
	if m == nil || !f.h.Switch(id) {
		return
	}
	dest := m.ResumeDest
	if dest < 0 {
		dest = f.h.ColonyTile()
	}
	people, animals := f.h.Aboard()
	var route []int
	if p := f.h.Planet(); p != nil {
		route = p.Route(m.Tile, dest)
	}
	if len(people) > 0 && len(route) < 2 {
		m.PackBy = now + packTicks // no way out yet: nobody is left behind for that
		f.h.Note("The people there have no way to leave yet.")
		return
	}
	if len(people) > 0 {
		cid := f.h.NewID()
		*f.list = append(*f.list, Caravan{C: caravan.Caravan{ID: cid, Tile: m.Tile, Dest: dest, Path: slices.Clone(route[1:]), State: caravan.Forming},
			Home: m.Tile, Out: route, Formed: now, Map: id, NextEnc: now + encounterCooldown}) // listed first: a stash asks whether its holder is a caravan
		var members, beasts []uint64
		for _, p := range people {
			if f.h.Stash(p, entity.EntityID(cid)) {
				members = append(members, uint64(p))
			}
		}
		for _, a := range animals {
			if f.h.Stash(a, entity.EntityID(cid)) {
				beasts = append(beasts, uint64(a))
			}
		}
		f.h.PackUp(cid)
		c := f.At(entity.EntityID(cid))
		c.C.Members, c.C.Animals = members, beasts
		f.depart(c, now)
	}
	l.close(id)
}

// close closes map id: whatever is left on it is lost. A caravan still
// forming there loses the members not yet off the map.
func (l Lifecycle) close(id uint32) {
	f := l.f
	for i := 0; i < len(*f.list); i++ {
		c := &(*f.list)[i]
		if c.Map != id || c.C.State != caravan.Forming {
			continue
		}
		for _, x := range c.Exiting {
			c.C.Members = slices.DeleteFunc(c.C.Members, func(m uint64) bool { return m == uint64(x) })
		}
		c.Exiting = nil
		if len(c.C.Members) == 0 {
			*f.list = slices.Delete(*f.list, i, i+1)
			i--
			continue
		}
		f.depart(c, f.h.Now())
	}
	f.h.Close(id)
	l.forgetMeta(id)
}

// forgetMeta drops map id's row and its queued requests.
func (l Lifecycle) forgetMeta(id uint32) {
	a := l.f.a
	if m := a.Meta(id); m != nil && m.Quest != 0 {
		l.f.h.SiteClosed(m.Quest, id) // QST-109
	}
	a.Maps = slices.DeleteFunc(a.Maps, func(m Meta) bool { return m.ID == id })
	a.Queue = slices.DeleteFunc(a.Queue, func(r Request) bool { return r.Op != OpOpen && r.Map == id })
}

// Label is map id's switcher label, kind and pack-up countdown at now
// (map 0 and unknown ids: label and kind as given).
func (l Lifecycle) Label(id uint32, label, kind string, now uint64) (string, string, int) {
	m := l.f.a.Meta(id)
	if m == nil {
		return label, kind, 0
	}
	name := l.f.h.TileName(m.Tile)
	if m.Lump != 0 {
		name = m.Lump.String() + " site"
	}
	if m.Kind == Colony {
		name = fmt.Sprintf("Colony %d", id)
	}
	return name, m.Kind.String(), l.f.a.Countdown(id, now)
}
