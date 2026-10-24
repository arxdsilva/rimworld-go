package incident

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// Category groups events the storyteller paces separately.
type Category uint8

const (
	Nature Category = iota // wildlife comings and goings, windfalls
	Threat
)

// ID is an event. Saved: append only.
type ID uint8

const (
	HerdCrossing ID = iota
	StrayJoins
	FarmAnimals
	TakenALiking // a wild animal tames itself
	TimbergnawGnawing
	MadAnimal // one wild animal goes manhunter (threat pool)
	// ENG-003 (our weights): a group of visitors stops by, a wanderer asks
	// to join, the colony throws a party.
	VisitorGroup
	WandererJoins
	Party
	NumIDs
)

// Def is an event's definition. Weights and cooldowns are our own.
type Def struct {
	ID       ID
	Key      string
	Name     string
	Category Category
	Weight   int // relative chance among the eligible events
	Cooldown int // days before it can fire again
	MinDay   int // earliest day it can fire
	// Biomes it can happen in; Not excludes some. Both empty: anywhere.
	Biomes, Not biome.Set
}

var ice = biome.Of(biome.IceSheet, biome.SeaIce)

var defs = [NumIDs]Def{
	HerdCrossing: {Key: "herd_crossing", Name: "Herd crossing", Category: Nature, Weight: 30, Cooldown: 10, MinDay: 3, Not: ice},
	StrayJoins:   {Key: "stray_joins", Name: "Stray joins", Category: Nature, Weight: 10, Cooldown: 15, MinDay: 5},
	FarmAnimals:  {Key: "farm_animals", Name: "Farm animals wander in", Category: Nature, Weight: 15, Cooldown: 12, MinDay: 4, Not: ice},
	TakenALiking: {Key: "taken_a_liking", Name: "Taken a liking", Category: Nature, Weight: 10, Cooldown: 8, MinDay: 6},
	TimbergnawGnawing: {Key: "timbergnaw_gnawing", Name: "Timbergnaw gnawing", Category: Nature, Weight: 7, Cooldown: 20, MinDay: 10,
		Biomes: biome.Of(biome.BorealForest, biome.Tundra, biome.ColdBog, biome.TemperateForest)},
	MadAnimal:     {Key: "mad_animal", Name: "Mad animal", Category: Threat, Weight: 1, Cooldown: 10, MinDay: 6},
	VisitorGroup:  {Key: "visitor_group", Name: "Visitors", Category: Nature, Weight: 12, Cooldown: 8, MinDay: 4},
	WandererJoins: {Key: "wanderer_joins", Name: "Wanderer joins", Category: Nature, Weight: 6, Cooldown: 20, MinDay: 8},
	Party:         {Key: "party", Name: "Party", Category: Nature, Weight: 8, Cooldown: 10, MinDay: 6},
}

func init() {
	for i := range defs {
		defs[i].ID = ID(i)
	}
}

// DefOf is event id's definition.
func DefOf(id ID) Def {
	if id >= NumIDs {
		return Def{}
	}
	return defs[id]
}

// All lists every event.
func All() []Def { return append([]Def(nil), defs[:]...) }

// State is the storyteller's memory of events (saved).
type State struct {
	NextNature uint64        // tick of the next nature roll (0: not scheduled)
	Last       map[ID]uint64 // tick each event last fired
}

// Allowed reports whether d may fire at tick now in biome b, ignoring any
// situational checks the caller adds.
func (d Def) Allowed(st State, now uint64, b biome.ID) bool {
	day := int(now / clock.TicksPerDay)
	if day < d.MinDay || d.Not.Has(b) || d.Biomes != 0 && !d.Biomes.Has(b) {
		return false
	}
	last, fired := st.Last[d.ID]
	return !fired || now >= last+uint64(d.Cooldown)*clock.TicksPerDay
}

// Pick chooses among the events of category c allowed now (and passing
// ok, the caller's situational check), weighted; roll is a random number
// in [0, total weight) supplied by the caller, who asks Total first. It
// is pure and deterministic.
func Pick(c Category, st State, now uint64, b biome.ID, roll int, ok func(ID) bool) (ID, bool) {
	for _, d := range eligible(c, st, now, b, ok) {
		if roll < d.Weight {
			return d.ID, true
		}
		roll -= d.Weight
	}
	return 0, false
}

// Total is the summed weight of the events Pick would choose among.
func Total(c Category, st State, now uint64, b biome.ID, ok func(ID) bool) int {
	t := 0
	for _, d := range eligible(c, st, now, b, ok) {
		t += d.Weight
	}
	return t
}

func eligible(c Category, st State, now uint64, b biome.ID, ok func(ID) bool) []Def {
	var out []Def
	for _, d := range defs {
		if d.Category == c && d.Allowed(st, now, b) && (ok == nil || ok(d.ID)) {
			out = append(out, d)
		}
	}
	return out
}

// Fired records that id happened at tick now.
func (st *State) Fired(id ID, now uint64) {
	if st.Last == nil {
		st.Last = make(map[ID]uint64)
	}
	st.Last[id] = now
}

// Nature rolls come every NatureGapMin..NatureGapMax days (our pacing),
// at a random hour of the day.
const (
	NatureGapMin = 2
	NatureGapMax = 5
)

// NextNatureAt is the tick of a nature roll gapDays after the day of tick
// now, at hour (0..23; clamped). It is always after now.
func NextNatureAt(now uint64, gapDays, hour int) uint64 {
	hour = min(max(hour, 0), clock.HoursPerDay-1)
	day := now/clock.TicksPerDay + uint64(max(gapDays, 1))
	return day*clock.TicksPerDay + uint64(hour)*clock.TicksPerHour
}
