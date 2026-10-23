package game

import (
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
)

// NG-1 contract types (lead-owned): the new-game setup carried from the
// wizard into the colony, and the read models of the new screens.

// Season is a starting season (resolved; never Auto inside Setup).
type Season uint8

const (
	SeasonSpring Season = iota
	SeasonSummer
	SeasonFall
	SeasonWinter
)

// Setup is everything the new-game wizard chose (NG-1 §5.1). A nil Setup
// in Config is the legacy start (tests, -scenario set pieces).
type Setup struct {
	Scenario    scenario.Def
	Storyteller string
	Preset      difficulty.Preset
	Difficulty  difficulty.Settings
	SaveMode    difficulty.SaveMode
	World       planet.Params
	Tile        int
	Site        site.Site
	Factions    []faction.Faction
	MapSize     int
	Season      Season
	Colonists   []pawngen.Pawn
	Seed        uint64
}

// MaxEnabledSide is the largest map side the wizard offers; raised by the
// lead only after the engine gates pass (NG-1 §6.9).
// Raised to 500 after the gate passed: 500x500 grown colony at 3 ticks per
// frame held avg 117.5 fps, 1% low 68.4 fps over 10 minutes at the
// farthest zoom (ENG-001).
const MaxEnabledSide = 500

// MapSizeView is one map size option.
type MapSizeView struct {
	Side    int
	Enabled bool
	Reason  string
	Large   bool // 350+ ("needs a capable computer")
}

// ScenarioView is a scenario in the Choose Scenario list.
type ScenarioView struct {
	Key, Name, Summary string
	Builtin            bool
	Hardness           int
	Unreadable         string // decode error for a broken file
	Problems           []scenario.Problem
	Lines              []DetailLine
	Def                scenario.Def
}

// DetailLine is one line of a details panel (Heading lines start groups).
type DetailLine struct {
	Heading bool
	Text    string
	Tip     string
}

// SkillView is one skill row.
type SkillView struct {
	Key, Label string
	Level      int
	LevelLabel string
	Passion    int // 0 none, 1 minor, 2 major
	Incapable  bool
	XPPct      int
	SpeedPct   int
	Affects    string // work it drives
}

// TraitView is one trait chip.
type TraitView struct {
	Key, Label, Desc, Effects string
}

// BioView is a colonist's identity (pawn Bio tab, character creation).
type BioView struct {
	First, Nick, Last string
	Sex, Build        string
	AgeYears          int
	Childhood         string
	ChildhoodDesc     string
	Adulthood         string
	AdulthoodDesc     string
	Traits            []TraitView
	Skills            []SkillView
	Incapable         []string // "Violence (backstory: Convoy mechanic)"
	Conditions        []string
	Look              look.Look
}

// CandidateView is a character-creation candidate.
type CandidateView struct {
	BioView
	Selected bool
	Index    int
}

// TeamReport is the Selected colonists' combined skills and gaps.
type TeamReport struct {
	Best     []TeamSkill
	Warnings []string
}

// TeamSkill is a skill's best level among the Selected and who has it.
type TeamSkill struct {
	Key, Label string
	Level      int
	Who        string
}

// FactionView is a faction row (setup and colony).
type FactionView struct {
	ID          int
	Name, Type  string
	Leader      string
	LeaderTitle string
	Goodwill    int
	Relation    string // "hostile", "neutral", "friendly"
	Settlements int
	Tech        string
	Emblem      string // ui_icon key
	Accent      string // hex
	History     []int16
	Events      []string
	Hidden      bool
}

// StoryView is the colony's storyteller and difficulty.
type StoryView struct {
	Storyteller, Title string
	Preset             string
	Difficulty         difficulty.Settings
	Adaptation         int
	Commitment         bool
}
