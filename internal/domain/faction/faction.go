// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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
	// Existing save IDs above stay stable.
	FierceClans
	SavageClans
	AncientQuiet
	AncientHostile
	MechHive
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
	CanBeFriendly bool
	PerTenK       int // settlements per 10,000 land tiles at normal population, x10
	Hidden        bool
	MaxCount      int
	PointsPct     int // raid points multiplier
	Leaders       []string
	// Accent is the faction type's UI accent colour ("#rrggbb", original).
	Accent  string
	IconKey string // share an authored faction glyph instead of requiring new art
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
	Besieged    bool // MAP-16: we are attacking a settlement of theirs, so goodwill can't rise (wiki: Faction base)
	Relation    Relation
	Settlements []int // world tile ids, capital first
	// SettlementNames are the settlements' names, parallel to Settlements.
	SettlementNames []string
	Defeated        bool
	History         []int16 // daily goodwill, last 60 days
	Events          []string
	// Enemies are the ids of the factions this one is at war with,
	// ascending (wars.go, EAI-015: set once by SetEnemies, never changed;
	// a save before v17 gets them on load).
	Enemies []int
	// ThankedDay is 1 + the colony day it last gave goodwill for a beaten
	// enemy raid (0: never; EnemyBeaten's cooldown).
	ThankedDay     int
	DriftDay       uint8  // fractional goodwill drift's five-day phase
	TradeRemainder uint16 // uncredited paid trade value toward next 500-silver goodwill
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

// Goodwill bands (NG-1 §3.4). A faction turns hostile at or below HostileAt
// and only calms down at or above CalmAt; it turns friendly at or above
// FriendlyAt (+75) and stays friendly until goodwill reaches FriendlyEndAt (0).
const (
	HostileAt     = -75 // wiki: Factions
	CalmAt        = 0   // wiki: Factions
	FriendlyAt    = 75  // wiki: Factions
	FriendlyEndAt = 0   // wiki: Factions ("remain allies until it reaches 0")
	goodwillMin   = -100
	goodwillMax   = 100
	// HistoryDays is how many daily goodwill values are kept.
	HistoryDays = 60
	// EventCount is how many recent goodwill events are kept.
	EventCount = 5
)

var defs = [NumTypes]Def{
	Player: {Key: "player", Label: "Colony", Tech: Industrial, Hidden: true, PointsPct: 100, Accent: "#d8c27a"},
	Settlers: {Key: "settlers", Label: "Settlers", Tech: Industrial, GoodwillMin: -50, GoodwillMax: 50,
		CanBeFriendly: true, PerTenK: 12, MaxCount: 4, PointsPct: 100,
		Leaders: []string{"Warden", "Steward"}, Accent: "#5b8fb9"},
	Clans: {Key: "clans", Label: "Clans", Tech: Neolithic, GoodwillMin: -50, GoodwillMax: 50,
		CanBeFriendly: true, PerTenK: 16, MaxCount: 4, PointsPct: 100,
		Leaders: []string{"Elder", "Hearth-mother", "Hearth-father"}, Accent: "#7f9e4c"},
	Drifters: {Key: "drifters", Label: "Drifters", Tech: Industrial, GoodwillMin: -100, GoodwillMax: -80,
		CanBeFriendly: true, PerTenK: 10, MaxCount: 3, PointsPct: 100,
		Leaders: []string{"Boss", "Caller"}, Accent: "#c0904e"},
	Freebooters: {Key: "freebooters", Label: "Freebooters", Tech: Industrial, GoodwillMin: -100, GoodwillMax: -100,
		Fixed: true, PerTenK: 8, MaxCount: 3, PointsPct: 120,
		Leaders: []string{"Captain", "Ringleader"}, Accent: "#b84a42"},
	Swarm: {Key: "swarm", Label: "Swarm", Tech: Neolithic, GoodwillMin: -100, GoodwillMax: -100,
		Fixed: true, Hidden: true, MaxCount: 1, PointsPct: 100, Accent: "#8b6bb3"},
	FierceClans: {Key: "fierce-clans", Label: "Warclans", Tech: Neolithic,
		GoodwillMin: -100, GoodwillMax: -80, CanBeFriendly: true, PerTenK: 16,
		MaxCount: 4, PointsPct: 100, Leaders: []string{"Elder", "Hearth-mother", "Hearth-father"},
		Accent: "#a56e46", IconKey: "clans"},
	SavageClans: {Key: "savage-clans", Label: "Bloodclans", Tech: Neolithic,
		GoodwillMin: -100, GoodwillMax: -100, Fixed: true, PerTenK: 16,
		MaxCount: 4, PointsPct: 120, Leaders: []string{"Elder", "Hearth-mother", "Hearth-father"},
		Accent: "#9b594a", IconKey: "clans"},
	AncientQuiet: {Key: "old-kin", Label: "Old kin", Tech: Spacer, Hidden: true,
		CanBeFriendly: true, MaxCount: 1, PointsPct: 100, Accent: "#718894", IconKey: "settlers"},
	AncientHostile: {Key: "lost-kin", Label: "Lost kin", Tech: Spacer, GoodwillMin: -100,
		GoodwillMax: -100, Fixed: true, Hidden: true, MaxCount: 1,
		PointsPct: 100, Accent: "#665e80", IconKey: "freebooters"},
	MechHive: {Key: "iron-host", Label: "Iron host", Tech: Spacer, GoodwillMin: -100,
		GoodwillMax: -100, Fixed: true, Hidden: true, MaxCount: 1,
		PointsPct: 100, Accent: "#68777a", IconKey: "swarm"},
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

// AffiliationLabel projects a known ordinary faction or a hidden origin.
// Neither affiliation is fabricated for an unattributed prisoner.
func AffiliationLabel(f *Faction, origin Type) string {
	if f != nil {
		return f.Name
	}
	if f, ok := Special(origin); ok {
		return f.Name
	}
	if origin != Player {
		return DefOf(origin).Label
	}
	return ""
}

// Emblem resolves the already authored icon for a faction variant.
func (d Def) Emblem() string {
	key := d.Key
	if d.IconKey != "" {
		key = d.IconKey
	}
	return "ui_icon/faction_" + key
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
func (t Type) Human() bool {
	return t < NumTypes && t != Player && t != Swarm && t != MechHive
}

// HasWorldSettlements selects surface factions eligible for diplomacy.
func (t Type) HasWorldSettlements() bool { return t.Human() && !def(t).Hidden }

// CanVisit is the single filter for a peaceful, standing surface faction.
func (f Faction) CanVisit() bool {
	return f.Type.HasWorldSettlements() && !f.Defeated && f.Relation != Hostile
}

func (t Type) String() string { return DefOf(t).Key }

// DefaultSlots is the default factions of a new world.
func DefaultSlots() []Slot {
	return []Slot{{Settlers, 1}, {Drifters, 1}, {Clans, 1}, {FierceClans, 1}, {SavageClans, 1}, {Freebooters, 1}}
}

// Generate makes the factions for the slots (names, leaders, goodwill)
// and their wars (SetEnemies).
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
	SetEnemies(out) // wars.go: no draw from rng
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
// current relation: hostile at or below -75 and calm again only at or above
// 0; friendly at or above +75 and neutral again once goodwill reaches 0
// (an ally is kept while goodwill stays above FriendlyEndAt).
func RelationFor(cur Relation, gw int) Relation {
	if gw <= HostileAt {
		return Hostile
	}
	if cur == Hostile && gw < CalmAt {
		return Hostile
	}
	if gw >= FriendlyAt || cur == Friendly && gw > FriendlyEndAt {
		return Friendly
	}
	return Neutral
}

// Update refreshes f.Relation from its goodwill, honouring the type (some
// types can never be friendly).
func Update(f *Faction) {
	if def(f.Type).Fixed {
		f.Goodwill, f.Relation = -100, Hostile
		return
	}
	r := RelationFor(f.Relation, f.Goodwill)
	if r == Friendly && !def(f.Type).CanBeFriendly {
		r = Neutral
	}
	f.Relation = r
}

// Adjust changes f's goodwill by delta (clamped to -100..100; fixed types
// ignore it), records event among the last five and updates the relation.
func Adjust(f *Faction, delta int, event string) {
	if f.Besieged && delta > 0 {
		return
	}
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

// CaptiveHostility records the successful capture of a non-hostile surface
// faction's member. Hidden/spacer factions do not use this diplomacy.
// The caller supplies its existing hostile goodwill floor; an arrest that
// already turned the faction hostile must not charge or log a second event.
func CaptiveHostility(f *Faction, floor int) {
	if f == nil || f.Relation == Hostile || !f.Type.HasWorldSettlements() || DefOf(f.Type).Tech == Spacer {
		return
	}
	RecordHostility(f, floor, "visitor captured")
}

// RecordHostility applies the existing goodwill floor and records the
// diplomatic event. Unknown factions have no diplomacy to update.
func RecordHostility(f *Faction, floor int, event string) {
	if f != nil {
		Adjust(f, min(0, floor-f.Goodwill), event)
	}
}

// DailyDrift restores goodwill to its natural band at +0.2 or -0.4/day.
// The phase survives save/load so no day gains or loses fractional progress.
func DailyDrift(f *Faction) {
	d := def(f.Type)
	f.DriftDay = (f.DriftDay + 1) % 5
	if !d.Fixed {
		switch {
		case f.Goodwill > d.GoodwillMax:
			if f.DriftDay == 0 || f.DriftDay == 2 {
				f.Goodwill--
			}
		case f.Goodwill < d.GoodwillMin && !f.Besieged:
			if f.DriftDay == 0 {
				f.Goodwill++
			}
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
		if !Raider(f) {
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
