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
	// PLAY-021 (our weights, Nature pool): a solar flare stops every power
	// net for a few hours; a short circuit blows a conduit, drains its
	// net's batteries and starts a fire. Both need power devices on the
	// map (the game's situational check).
	SolarFlare
	ShortCircuit
	// MED-04 (our weights and cooldowns, Nature pool, scaled by the
	// difficulty's disease frequency): a disease strikes one colonist (gut
	// worms, muscle parasites, sleeping sickness) or several (flu,
	// plague, malaria).
	Flu
	Plague
	Malaria
	SleepingSickness
	GutWorms
	MuscleParasites
	// PLAY-024 (our weights and cooldowns, Nature pool): a trader caravan
	// walks in and camps near the colony; an orbital trader passes within
	// radio range (it needs a powered radio console).
	TraderCaravan
	OrbitalTrader
	// DEF-4 (Threat pool): a ship part crashes and its dormant mechanoid
	// guards wake when disturbed (game/raid/shippart.go).
	CrashedShipPart
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
	// Disease: its weight scales with the difficulty's disease frequency
	// (WeightAt).
	Disease bool
}

var (
	ice = biome.Of(biome.IceSheet, biome.SeaIce)
	// tropical: sleeping sickness only strikes here. wiki: Sleeping
	// sickness (tropical rainforest and swamp).
	tropical = biome.Of(biome.TropicalRainforest, biome.TropicalSwamp)
	// warm: the warm, wet lands malaria strikes in (the wiki makes it
	// most frequent in the tropics; adding the arid shrubland and the
	// temperate swamp is our choice).
	warm = biome.Of(biome.TropicalRainforest, biome.TropicalSwamp, biome.AridShrubland, biome.TemperateSwamp)
)

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
	SolarFlare:    {Key: "solar_flare", Name: "Solar flare", Category: Nature, Weight: 6, Cooldown: 12, MinDay: 12},
	ShortCircuit:  {Key: "short_circuit", Name: "Short circuit", Category: Nature, Weight: 5, Cooldown: 8, MinDay: 10},
	// MED-04: the wiki gives no weights or cooldowns for our storyteller;
	// these are original tuning (diseases from day 15, a few a year).
	Flu:              {Key: "flu", Name: "Flu", Category: Nature, Weight: 6, Cooldown: 20, MinDay: 15, Disease: true},
	Plague:           {Key: "plague", Name: "Plague", Category: Nature, Weight: 2, Cooldown: 40, MinDay: 15, Disease: true},
	Malaria:          {Key: "malaria", Name: "Malaria", Category: Nature, Weight: 4, Cooldown: 25, MinDay: 15, Biomes: warm, Disease: true},
	SleepingSickness: {Key: "sleeping_sickness", Name: "Sleeping sickness", Category: Nature, Weight: 3, Cooldown: 40, MinDay: 15, Biomes: tropical, Disease: true},
	GutWorms:         {Key: "gut_worms", Name: "Gut worms", Category: Nature, Weight: 4, Cooldown: 25, MinDay: 15, Not: ice, Disease: true},
	MuscleParasites:  {Key: "muscle_parasites", Name: "Muscle parasites", Category: Nature, Weight: 4, Cooldown: 25, MinDay: 15, Not: ice, Disease: true},
	// PLAY-024: original tuning (PLAY024_PLAN.md).
	TraderCaravan: {Key: "trader_caravan", Name: "Trader caravan", Category: Nature, Weight: 14, Cooldown: 9, MinDay: 4},
	OrbitalTrader: {Key: "orbital_trader", Name: "Orbital trader", Category: Nature, Weight: 12, Cooldown: 7, MinDay: 6},
	// DEF-4, wiki: Events (base weight 2.00 against a raid's 7.40; it
	// needs 400 raid points, the game's own check). Cooldown and first day
	// are original tuning (the wiki gives none).
	CrashedShipPart: {Key: "crashed_ship_part", Name: "Crashed ship part", Category: Threat, Weight: 2, Cooldown: 15, MinDay: 10},
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

// FullFreq is the disease frequency (percent) at which diseases keep
// their catalogue weights.
const FullFreq = 100

// WeightAt is d's weight at disease frequency freqPct (percent): a
// disease's weight scales with it, rounded, at least 1 while freqPct is
// above 0, and 0 (never picked) at 0; any other event keeps its weight.
func (d Def) WeightAt(freqPct int) int {
	if !d.Disease {
		return d.Weight
	}
	if freqPct <= 0 {
		return 0
	}
	return max((d.Weight*freqPct+FullFreq/2)/FullFreq, 1)
}

// Pick chooses among the events of category c allowed now (and passing
// ok, the caller's situational check), weighted; roll is a random number
// in [0, total weight) supplied by the caller, who asks Total first. It
// is pure and deterministic. Diseases keep their catalogue weights
// (PickAt sets the frequency).
func Pick(c Category, st State, now uint64, b biome.ID, roll int, ok func(ID) bool) (ID, bool) {
	return PickAt(c, st, now, b, FullFreq, roll, ok)
}

// PickAt is Pick with the diseases weighted at frequency freqPct
// (WeightAt); roll is in [0, TotalAt).
func PickAt(c Category, st State, now uint64, b biome.ID, freqPct, roll int, ok func(ID) bool) (ID, bool) {
	for _, d := range eligible(c, st, now, b, freqPct, ok) {
		w := d.WeightAt(freqPct)
		if roll < w {
			return d.ID, true
		}
		roll -= w
	}
	return 0, false
}

// Total is the summed weight of the events Pick would choose among.
func Total(c Category, st State, now uint64, b biome.ID, ok func(ID) bool) int {
	return TotalAt(c, st, now, b, FullFreq, ok)
}

// TotalAt is the summed weight of the events PickAt would choose among.
func TotalAt(c Category, st State, now uint64, b biome.ID, freqPct int, ok func(ID) bool) int {
	t := 0
	for _, d := range eligible(c, st, now, b, freqPct, ok) {
		t += d.WeightAt(freqPct)
	}
	return t
}

// eligible is the events of category c allowed now with a weight at
// freqPct, passing ok (asked only for those).
func eligible(c Category, st State, now uint64, b biome.ID, freqPct int, ok func(ID) bool) []Def {
	var out []Def
	for _, d := range defs {
		if d.Category == c && d.WeightAt(freqPct) > 0 && d.Allowed(st, now, b) && (ok == nil || ok(d.ID)) {
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
