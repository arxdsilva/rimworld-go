// Package trait is a colonist's personality traits: original set, conflict
// groups, stat mods and thought scaling (NG-1 §3.6, §4.6).
package trait

import (
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

// ID is a trait (append only, saved). 0 is "none".
type ID uint8

const (
	None ID = iota
	Tireless
	Unhurried
	Sunny
	Brooding
	Steady
	Brittle
	FleetFooted
	Plodding
	HeartyEater
	LightEater
	Roughneck
	Fussy
	DeliberateAim
	QuickDraw
	GreenThumb
	Bookish
	Gentle
	ColdHardy
	Sturdy
	NumTraits
)

// Def is one trait's definition.
type Def struct {
	ID           ID
	Key, Label   string
	Desc         string
	Group        uint8 // traits sharing a non-zero group conflict
	Mods         []stat.Mod
	ThoughtScale map[mood.Kind]int16 // percent; 100 = unchanged
	AddThought   map[mood.Kind]int8
	WorkSpeed    map[skill.ID]int16 // percent bonus to work using that skill
	LearnMul     map[skill.ID]int16 // percent learning multiplier
	SkillBonus   map[skill.ID]int8  // levels added at generation
	Incapable    backstory.Bits
}

// DefOf, All, Parse and Conflicts are the trait catalogue.
func DefOf(id ID) Def             { return Def{} }       // NG-1 stub (PAWN-D)
func All() []Def                  { return nil }         // NG-1 stub (PAWN-D)
func Parse(key string) (ID, bool) { return None, false } // NG-1 stub (PAWN-D)
func Conflicts(a, b ID) bool      { return false }       // NG-1 stub (PAWN-D)
