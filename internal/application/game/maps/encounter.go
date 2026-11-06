package maps

import (
	"cmp"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/caravan"
)

// Caravan encounters (MAP-08; wiki: Caravan). Hourly on the road a caravan
// may meet trouble: an ambush, a manhunter pack, or a demand for part of
// its cargo (refused, it is an ambush). A fight opens a small map at the
// caravan's tile with the caravan at its centre; once it is won the
// temporary map's pack-up runs and the caravan goes on to where it was
// heading. Trading with a passing caravan is deferred (no caravan trading).

// Trouble is what a caravan meets (saved).
type Trouble uint8

const (
	NoTrouble Trouble = iota
	Ambush
	Manhunters
	Demand
)

const (
	// encounterMTBDays is the mean days between rolls that would hit, before
	// visibility, and encounterRemoteDays in an extreme biome (original
	// tuning: the wiki publishes no numbers).
	encounterMTBDays    = 10
	encounterRemoteDays = 20
	// encounterCooldown keeps a caravan from meeting trouble again for two
	// days (original tuning, no flapping).
	encounterCooldown = 2 * clock.TicksPerDay
	// demandWait is how long a demand waits before it counts as refused;
	// ambushPct and packPct split the encounters (original tuning).
	demandWait = clock.TicksPerDay
	ambushPct  = 50
	packPct    = 25
	waylaySalt = 0x7A1F00D
)

// Waylayer is the simulation's side of an encounter, on the current map.
type Waylayer interface {
	// RoadRoll draws from the encounter stream.
	RoadRoll(n int) int
	// Ambush sends raiders of faction fac (1 + index; 0 none) in from an
	// edge, Hunt a manhunter pack of n; seed feeds their draws, so no
	// simulation stream moves.
	Ambush(seed uint64, fac int)
	Hunt(seed uint64, n int)
	// DemandLetter posts a demand letter (Give or Refuse): its id;
	// DropLetter takes a letter off the stack.
	DemandLetter(title, format string, args ...any) uint64
	DropLetter(id uint64)
}

// Ask is a demand waiting for its answer (zero: none; saved).
type Ask struct {
	Letter  uint64
	Faction int
	Kind    item.Kind
	Qty     int
	Until   uint64
}

// Visibility is how visible a caravan of n people is, permille: 1 person
// 200, 2 people 360 (wiki: Caravan), the same 1-0.8^n beyond (original fit).
func Visibility(n int) int {
	v := 1000
	for range n {
		v = v * 800 / 1000
	}
	return 1000 - v
}

// roll is the hourly encounter roll of caravan c on the road. A caravan
// cooling down draws nothing.
func (f Fleet) roll(c *Caravan, now uint64) {
	p := f.h.Planet()
	if now < c.NextEnc || len(c.C.Members) == 0 || p == nil {
		return
	}
	if _, ok := f.mapAt(c.C.Tile); ok {
		return
	}
	mtb := encounterMTBDays
	if t := c.C.Tile; t >= 0 && t < len(p.Biome) && biome.DefOf(p.Biome[t]).Difficulty == "extreme" {
		mtb = encounterRemoteDays
	}
	if f.h.RoadRoll(mtb*24*1000) >= Visibility(len(c.C.Members)) {
		return
	}
	k := Demand
	switch r := f.h.RoadRoll(100); {
	case r < ambushPct:
		k = Ambush
	case r < ambushPct+packPct:
		k = Manhunters
	}
	f.Waylay(c, k, now)
}

// Waylay has caravan c meet k now: it stops where it is. A demand with
// nothing to demand, or nobody to make it, is an ambush.
func (f Fleet) Waylay(c *Caravan, k Trouble, now uint64) {
	c.NextEnc = now + encounterCooldown
	fac := f.hostile()
	if k == Demand && !f.demand(c, fac, now) {
		k = Ambush
	}
	if k != Demand {
		f.fight(c, k, fac)
	}
}

// fight stops c and queues its encounter map.
func (f Fleet) fight(c *Caravan, k Trouble, fac int) {
	c.Ask = Ask{}
	f.a.Queue = append(f.a.Queue, Request{Op: OpOpen, Kind: Encounter, Tile: c.C.Tile, Caravan: c.C.ID, Trouble: k, Faction: fac, Resume: c.C.State})
	c.C.State = caravan.Arrived
}

// hostile is a hostile human faction (1 + index) picked from the encounter
// stream, 0 when there is none (no draw then).
func (f Fleet) hostile() int {
	var out []int
	for i, fa := range f.h.Factions() {
		if fa.Type.Human() && fa.Relation == faction.Hostile && !fa.Defeated {
			out = append(out, i+1)
		}
	}
	if len(out) == 0 {
		return 0
	}
	return out[f.h.RoadRoll(len(out))]
}

// demand has faction fac ask for half of c's most valuable goods (ties: the
// lower kind), by letter; false when there is nobody or nothing to ask.
func (f Fleet) demand(c *Caravan, fac int, now uint64) bool {
	goods := f.h.Cargo().Goods(c.C.ID)
	if fac == 0 || len(goods) == 0 {
		return false
	}
	st := slices.MaxFunc(goods, func(a, b item.Stack) int {
		if d := cmp.Compare(a.Kind.Value()*a.Qty, b.Kind.Value()*b.Qty); d != 0 {
			return d
		}
		return cmp.Compare(b.Kind, a.Kind)
	})
	qty := (st.Qty + 1) / 2
	name := f.h.Factions()[fac-1].Name
	id := f.h.DemandLetter("A demand on the road", "Fighters of %s block the caravan's way near %s and want %d %s. Give it up and they let it pass; refuse and they attack.", name, f.h.TileName(c.C.Tile), qty, st.Kind)
	c.Ask = Ask{Letter: id, Faction: fac, Kind: st.Kind, Qty: qty, Until: now + demandWait}
	return true
}

// Answer is command.EncounterAnswer: letter's demand is met (mode 0) or
// refused (1). Goods it no longer has count as a refusal.
func (f Fleet) Answer(letter uint64, mode uint8) {
	for i := range *f.list {
		c := &(*f.list)[i]
		if c.Ask.Letter == 0 || c.Ask.Letter != letter {
			continue
		}
		a := c.Ask
		c.Ask = Ask{}
		f.h.DropLetter(letter)
		if mode == 0 && f.h.Cargo().Yield(c.C.ID, a.Kind, a.Qty) {
			f.h.Note("The caravan gave up %d %s and was let through.", a.Qty, a.Kind)
			return
		}
		f.fight(c, Ambush, a.Faction)
		return
	}
}

// waylay carries out an encounter on its freshly opened map id: the caravan
// at the centre, drafted; the hostiles at an edge; a letter to jump there.
// Afterwards the caravan is the map's and heads for its old destination.
func (l Lifecycle) waylay(c *Caravan, id uint32, r Request) {
	f := l.f
	if !f.h.Switch(id) {
		c.C.State = r.Resume
		return
	}
	m := f.a.Meta(id)
	if m == nil { // a colony opened here this very tick: it walks in
		l.enter(c, id)
		return
	}
	m.ResumeDest = c.C.Dest
	if r.Resume == caravan.Returning {
		m.ResumeDest = c.Home
	}
	seed := SeedFor(f.h.Seed(), m.Tile, id) ^ waylaySalt
	n := len(c.C.Members)
	names := l.walkInAt(c, true)
	title, what := "Ambush!", "were ambushed"
	if r.Trouble == Manhunters {
		title, what = "Manhunters on the road", "were set upon by enraged animals"
		f.h.Hunt(seed, 2+n)
	} else {
		f.h.Ambush(seed, r.Faction)
	}
	if len(names) > 0 {
		f.h.Letter(true, title, entity.EntityID(c.C.Members[0]), "%s %s near %s.", travel.JoinNames(names), what, f.h.TileName(m.Tile))
	}
	f.drop(entity.EntityID(c.C.ID))
}
