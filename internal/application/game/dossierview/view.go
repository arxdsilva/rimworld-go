// Package dossierview is the colonist card's read model (RHUD): the facts about
// one pawn or animal that no other view carries. Plain values, by id, no scan.
package dossierview

import (
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

// Relation is how a pawn stands to the colony.
type Relation uint8

const (
	RelationOwn Relation = iota
	RelationAlly
	RelationNone
	RelationFoe
	RelationHeld
	RelationWild
)

// BreakLevels are the mood levels, 0..100, at which each break tier starts,
// shifted by traits.
type BreakLevels struct{ Minor, Major, Extreme int }

// StatLine is one tooltip fact, such as melee hit chance or butchery yield.
// Formulas that are pure domain functions stay in sheet and do not use it.
type StatLine struct {
	Label string
	Pct   int
}

// Card is one pawn's or animal's facts for the colonist card.
type Card struct {
	AgeDayOfYear  int // AgeDays % DaysPerYear; 0 is a birthday
	Relation      Relation
	Faction       string
	FactionEmblem string // ui_icon key
	Role          string // a trader's type, "" otherwise
	BreakAt       BreakLevels
	BreakTier     mentalbreak.Tier // of the running break
	StunTicks     int
	StaggerTicks  int
	ActiveSkill   int // skill.ID, -1 none
	Saturated     [skill.NumSkills]bool
	LearnedWeek   [skill.NumSkills]int // replaces Game.LearnedThisWeek
	Stats         stat.Sheet
	EatSpeedPct   int
	WorkType      int // work.Type, -1 none
	AttackTarget  string
	ExtraLines    [skill.NumSkills][]StatLine // sim-side formulas for the skill tooltip
}
