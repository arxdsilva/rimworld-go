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
	Defeated    bool
	History     []int16 // daily goodwill, last 60 days
	Events      []string
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

// DefOf, All, DefaultSlots: the type catalogue.
func DefOf(t Type) Def     { return Def{} } // NG-1 stub (RULES)
func All() []Def           { return nil }   // NG-1 stub (RULES)
func DefaultSlots() []Slot { return nil }   // NG-1 stub (RULES)

// Generate makes the factions for the slots (names, leaders, goodwill).
func Generate(slots []Slot, rng *randx.Rand) []Faction { return nil } // NG-1 stub (RULES)

// PlaceSettlements places settlements on g for fs (popFactor percent).
func PlaceSettlements(g Globe, fs []Faction, popPct int, rng *randx.Rand) {} // NG-1 stub (RULES)

// RelationFor applies the hysteresis bands to goodwill gw given the
// current relation.
func RelationFor(cur Relation, gw int) Relation { return cur } // NG-1 stub (RULES)

// DailyDrift moves goodwill one step toward the type's baseline.
func DailyDrift(f *Faction) {} // NG-1 stub (RULES)

// PickRaider chooses a hostile human faction (index into fs), or -1.
func PickRaider(fs []Faction, rng *randx.Rand) int { return -1 } // NG-1 stub (RULES)
