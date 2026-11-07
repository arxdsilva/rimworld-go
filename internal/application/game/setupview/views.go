package setupview

import (
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/scenario"
)

// The new-game screens' and the colony's setup read models (NG-1); the game
// names them game.MapSizeView, game.ScenarioView and game.StoryView.

// MapSize is one map size option.
type MapSize struct {
	Side    int
	Enabled bool
	Reason  string
	Large   bool // 350+ ("needs a capable computer")
}

// ScenarioRow is a scenario in the Choose Scenario list.
type ScenarioRow struct {
	Key, Name, Summary string
	Builtin            bool
	Hardness           int
	Unreadable         string // decode error for a broken file
	Problems           []scenario.Problem
	Lines              []Line
	Def                scenario.Def
}

// Story is the colony's storyteller and difficulty.
type Story struct {
	Storyteller, Title string
	Preset             string
	Difficulty         difficulty.Settings
	Adaptation         int
	Commitment         bool
}
