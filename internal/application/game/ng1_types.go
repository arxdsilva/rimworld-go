package game

import (
	"rimworld-go/internal/application/game/factionview"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/application/game/setupview"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
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
	// GenVersion is the map generator that built the colony
	// (mapgen.Version; 0: the pre-MAPGEN-2 site generator). Debug views
	// regenerate fields only for their own version.
	GenVersion uint16
	// GoneAt is the tick everyone was gone from (0: someone is here), and
	// WandererGroups how many new groups came since (MAP-19). Absent in
	// older saves.
	GoneAt         uint64
	WandererGroups int
}

// MaxEnabledSide is the largest map side the wizard offers; raised by the
// lead only after the engine gates pass (NG-1 §6.9).
// Raised to 500 after the gate passed: 500x500 grown colony at 3 ticks per
// frame held avg 117.5 fps, 1% low 68.4 fps over 10 minutes at the
// farthest zoom (ENG-001).
const MaxEnabledSide = 500

// The setup read models (game/setupview); DetailLine is one line of a
// details panel (Heading lines start groups).
type (
	MapSizeView  = setupview.MapSize
	ScenarioView = setupview.ScenarioRow
	DetailLine   = setupview.Line
	StoryView    = setupview.Story
)

// The identity read models and the character-creation pool live in
// game/roster; these are their names for the adapters.
type (
	SkillView     = roster.SkillView
	TraitView     = roster.TraitView
	BioView       = roster.BioView
	CandidateView = roster.CandidateView
	TeamReport    = roster.TeamReport
	TeamSkill     = roster.TeamSkill
	CandidatePool = roster.CandidatePool
)

// NewCandidatePool is roster.NewCandidatePool.
func NewCandidatePool(seed uint64, fx scenario.Effects) *CandidatePool {
	return roster.NewCandidatePool(seed, fx)
}

const (
	MaxNameRunes   = roster.MaxNameRunes
	WarnNoFighter  = roster.WarnNoFighter
	WarnNoDoctor   = roster.WarnNoDoctor
	WarnNoCook     = roster.WarnNoCook
	WarnNoGrower   = roster.WarnNoGrower
	WarnNoBuilder  = roster.WarnNoBuilder
	WarnNoResearch = roster.WarnNoResearch
	WarnYoungOrOld = roster.WarnYoungOrOld
)

// Rename errors (the screen shows err.Error()).
var (
	ErrNickRequired = roster.ErrNickRequired
	ErrNameTooLong  = roster.ErrNameTooLong
	ErrNameChars    = roster.ErrNameChars
	ErrNickTaken    = roster.ErrNickTaken
)

// FactionView is a faction row (setup and colony; game/factionview).
type FactionView = factionview.Row
