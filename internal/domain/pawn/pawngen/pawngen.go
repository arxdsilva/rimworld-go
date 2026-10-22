// Package pawngen generates colonists deterministically from a seed: name,
// sex, build, age, backstories, traits, skills, passions, look and
// starting conditions (NG-1 §4.6, §4.7).
package pawngen

import (
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
)

// Condition is a starting health condition (append only, saved).
type Condition uint8

const (
	NoCondition Condition = iota
	OldFracture
	Nearsighted
	HandTremor
	Exhausted
)

// Temperament mirrors the game's idle temperament (Wanderer, Social,
// Homebody) so the domain can choose it from traits.
type Temperament uint8

const (
	Wanderer Temperament = iota
	SocialType
	Homebody
)

// Pawn is a generated colonist's identity. AgeDays is biological age in
// in-game days (60 per year).
type Pawn struct {
	First, Nick, Last    string
	Look                 look.Look
	AgeDays              int32
	Childhood, Adulthood backstory.ID
	Traits               []trait.ID
	Skills               skill.Set
	Conditions           []Condition
	Temperament          Temperament
}

// ForcedTrait makes a trait appear with a chance (percent).
type ForcedTrait struct {
	Trait  trait.ID
	Chance uint8
}

// Rules are a scenario's constraints on generation.
type Rules struct {
	MinAge, MaxAge int // years; 0 = defaults (14..68)
	Forced         []ForcedTrait
	ForcedCond     Condition
	CondChance     uint8
	Tech           backstory.Tag
	NoClothes      bool
	Taken          map[string]bool // nicknames already used
}

// YearsPerDay converts AgeDays to years.
const DaysPerYear = 60

// Generate makes candidate slot's colonist on its roll-th reroll.
func Generate(seed uint64, slot, roll int, r Rules) Pawn { return Pawn{} } // NG-1 stub (PAWN-D)

// Legacy is the neutral identity for an existing pawn (old saves,
// SpawnNamedPawn): all skills 7, no traits or incapabilities, age 30,
// male average, the legacy look and today's name-hash temperament.
func Legacy(id uint64, name string, skins, hairs, cloths int) Pawn { return Pawn{} } // NG-1 stub (PAWN-D)

// Incapable is everything p can't do (from backstories and traits).
func (p Pawn) Incapable() backstory.Bits { return 0 } // NG-1 stub (PAWN-D)

// AgeYears is p's age in whole years.
func (p Pawn) AgeYears() int { return int(p.AgeDays) / DaysPerYear }
