// Package faction is the other peoples of the planet: original types,
// generated names and leaders, goodwill with hysteresis, settlements and
// raid attribution (NG-1 §3.4, §4).
package faction

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/pkg/randx"
)

// Type is a faction type (append only, saved).
type Type uint8

const (
	Player Type = iota
	Settlers
	Clans
	Drifters
	Freebooters
	Swarm
	NumTypes
)

// Tech is a technology level.
type Tech uint8

const (
	Neolithic Tech = iota
	Industrial
	Spacer
)

// Relation is how a faction stands with the player.
type Relation uint8

const (
	Neutral Relation = iota
	Hostile
	Friendly
)

// Def is a faction type's static definition.
type Def struct {
	Type          Type
	Key, Label    string
	Tech          Tech
	GoodwillMin   int
	GoodwillMax   int
	Fixed         bool // goodwill never changes (Freebooters, Swarm)
	Baseline      int  // daily drift target
	CanBeFriendly bool
	PerTenK       int // settlements per 10,000 land tiles at normal population, x10
	Hidden        bool
	MaxCount      int
	PointsPct     int // raid points multiplier
	Leaders       []string
}

// Leader is a faction leader.
type Leader struct {
	Name, Title string
}

// Faction is one generated faction.
type Faction struct {
	ID          int
	Type        Type
	Name        string
	Leader      Leader
	Goodwill    int
	Relation    Relation
	Settlements []int // world tile ids, capital first
	// SettlementNames are the settlements' names, parallel to Settlements.
	SettlementNames []string
	Defeated        bool
	History         []int16 // daily goodwill, last 60 days
	Events          []string
}

// Slot is one configured faction type on the Create World screen.
type Slot struct {
	Type  Type
	Count int
}

// Globe is what settlement placement needs from the planet (planet
// implements it; faction never imports planet).
type Globe interface {
	TileCount() int
	Land(i int) bool
	Biome(i int) biome.ID
	Hills(i int) uint8
	Hops(a, b, limit int) int
}

// Goodwill bands (NG-1 §3.4). A faction turns hostile at or below
// hostileAt and only calms down at or above calmAt; it turns friendly at or
// above friendlyAt and stays friendly while at or above friendlyKeep.
const (
	hostileAt    = -50
	calmAt       = -10
	friendlyAt   = 50
	friendlyKeep = 30
	goodwillMin  = -100
	goodwillMax  = 100
	// HistoryDays is how many daily goodwill values are kept.
	HistoryDays = 60
	// EventCount is how many recent goodwill events are kept.
	EventCount = 5
)

var defs = [NumTypes]Def{
	Player: {Key: "player", Label: "Colony", Tech: Industrial, Hidden: true, PointsPct: 100},
	Settlers: {Key: "settlers", Label: "Settlers", Tech: Industrial, GoodwillMin: -20, GoodwillMax: 60,
		Baseline: 10, CanBeFriendly: true, PerTenK: 12, MaxCount: 4, PointsPct: 100,
		Leaders: []string{"Warden", "Steward"}},
	Clans: {Key: "clans", Label: "Clans", Tech: Neolithic, GoodwillMin: -40, GoodwillMax: 40,
		Baseline: 0, CanBeFriendly: true, PerTenK: 16, MaxCount: 4, PointsPct: 100,
		Leaders: []string{"Elder", "Hearth-mother", "Hearth-father"}},
	Drifters: {Key: "drifters", Label: "Drifters", Tech: Industrial, GoodwillMin: -60, GoodwillMax: 10,
		Baseline: -30, CanBeFriendly: true, PerTenK: 10, MaxCount: 3, PointsPct: 100,
		Leaders: []string{"Boss", "Caller"}},
	Freebooters: {Key: "freebooters", Label: "Freebooters", Tech: Industrial, GoodwillMin: -100, GoodwillMax: -100,
		Fixed: true, Baseline: -100, PerTenK: 8, MaxCount: 3, PointsPct: 120,
		Leaders: []string{"Captain", "Ringleader"}},
	Swarm: {Key: "swarm", Label: "Swarm", Tech: Neolithic, GoodwillMin: -100, GoodwillMax: -100,
		Fixed: true, Baseline: -100, Hidden: true, MaxCount: 1, PointsPct: 100},
}

func init() {
	for i := range defs {
		defs[i].Type = Type(i)
	}
}

// def is DefOf without copying the leader titles.
func def(t Type) *Def {
	if t >= NumTypes {
		return &Def{}
	}
	return &defs[t]
}

// DefOf is t's definition (the zero Def for an unknown type).
func DefOf(t Type) Def {
	if t >= NumTypes {
		return Def{}
	}
	d := defs[t]
	d.Leaders = append([]string(nil), d.Leaders...)
	return d
}

// All lists every type's definition in Type order.
func All() []Def {
	out := make([]Def, NumTypes)
	for i := range out {
		out[i] = DefOf(Type(i))
	}
	return out
}

// Human reports a type whose raiders are people (not the Swarm or the
// player).
func (t Type) Human() bool { return t != Player && t != Swarm && t < NumTypes }

func (t Type) String() string { return DefOf(t).Key }

// DefaultSlots is the default factions of a new world.
func DefaultSlots() []Slot {
	return []Slot{{Settlers, 2}, {Clans, 2}, {Drifters, 1}, {Freebooters, 1}}
}

// Generate makes the factions for the slots (names, leaders, goodwill).
// Counts above a type's MaxCount are clamped; the player type is skipped.
// Names are unique within the result.
func Generate(slots []Slot, rng *randx.Rand) []Faction {
	var out []Faction
	used := map[string]bool{}
	for _, sl := range slots {
		d := DefOf(sl.Type)
		if sl.Type == Player || sl.Type >= NumTypes {
			continue
		}
		n := sl.Count
		if d.MaxCount > 0 {
			n = min(n, d.MaxCount)
		}
		for i := 0; i < n; i++ {
			f := Faction{ID: len(out), Type: sl.Type, Leader: leaderFor(d, rng)}
			f.Name = uniqueName(sl.Type, rng, used)
			f.Goodwill = d.GoodwillMin
			if d.GoodwillMax > d.GoodwillMin {
				f.Goodwill += rng.IntN(d.GoodwillMax - d.GoodwillMin + 1)
			}
			f.Relation = RelationFor(Neutral, f.Goodwill)
			if f.Relation == Friendly && !d.CanBeFriendly {
				f.Relation = Neutral
			}
			f.History = []int16{int16(f.Goodwill)}
			out = append(out, f)
		}
	}
	return out
}

// uniqueName draws a faction name nobody has used yet; after many clashes
// it numbers the name.
func uniqueName(t Type, rng *randx.Rand, used map[string]bool) string {
	for try := 0; try < 60; try++ {
		if n := factionName(t, rng); !used[n] {
			used[n] = true
			return n
		}
	}
	base := factionName(t, rng)
	for i := 2; ; i++ {
		if n := base + " " + roman(i); !used[n] {
			used[n] = true
			return n
		}
	}
}

// RelationFor applies the hysteresis bands to goodwill gw given the
// current relation: hostile at or below -50 and calm again only at or above
// -10; friendly at or above +50 and neutral again only below +30.
func RelationFor(cur Relation, gw int) Relation {
	if gw <= hostileAt {
		return Hostile
	}
	if cur == Hostile && gw < calmAt {
		return Hostile
	}
	if gw >= friendlyAt || cur == Friendly && gw >= friendlyKeep {
		return Friendly
	}
	return Neutral
}

// Update refreshes f.Relation from its goodwill, honouring the type (some
// types can never be friendly).
func Update(f *Faction) {
	r := RelationFor(f.Relation, f.Goodwill)
	if r == Friendly && !def(f.Type).CanBeFriendly {
		r = Neutral
	}
	f.Relation = r
}

// Adjust changes f's goodwill by delta (clamped to -100..100; fixed types
// ignore it), records event among the last five and updates the relation.
func Adjust(f *Faction, delta int, event string) {
	if !def(f.Type).Fixed {
		f.Goodwill = min(max(f.Goodwill+delta, goodwillMin), goodwillMax)
	}
	if event != "" {
		f.Events = append(f.Events, event)
		if len(f.Events) > EventCount {
			f.Events = append([]string(nil), f.Events[len(f.Events)-EventCount:]...)
		}
	}
	Update(f)
}

// DailyDrift moves goodwill one step toward the type's baseline, updates
// the relation and appends today's value to the history (last 60 days).
// Fixed types do not move.
func DailyDrift(f *Faction) {
	d := def(f.Type)
	if !d.Fixed {
		switch {
		case f.Goodwill < d.Baseline:
			f.Goodwill++
		case f.Goodwill > d.Baseline:
			f.Goodwill--
		}
	}
	Update(f)
	f.History = append(f.History, int16(f.Goodwill))
	if len(f.History) > HistoryDays {
		f.History = append([]int16(nil), f.History[len(f.History)-HistoryDays:]...)
	}
}

// PickRaider chooses a hostile human faction (index into fs), or -1. The
// choice is weighted by settlement count; a hostile faction with no
// placed settlements counts as one.
func PickRaider(fs []Faction, rng *randx.Rand) int {
	total := 0
	weight := func(f Faction) int {
		if !f.Type.Human() || f.Relation != Hostile || f.Defeated {
			return 0
		}
		return max(len(f.Settlements), 1)
	}
	for _, f := range fs {
		total += weight(f)
	}
	if total == 0 {
		return -1
	}
	roll := rng.IntN(total)
	for i, f := range fs {
		w := weight(f)
		if roll < w {
			return i
		}
		roll -= w
	}
	return -1
}
