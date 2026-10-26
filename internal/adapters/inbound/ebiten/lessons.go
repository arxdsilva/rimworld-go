package ebiten

import (
	"fmt"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// The learning helper's lessons (PLAY-018: LEARN owns them). Ten short
// lessons in our own words, shown three at a time in order. A lesson is done
// when the player does the thing (a command seen through Renderer.onEnqueue,
// or a UI event read each Update, learn_ui.go) or presses "Got it".

// lesson is one entry of the helper.
type lesson struct {
	id    string // saved in Gameplay.LearnedLessons: never change one
	title string // the row's heading
	goal  string // one line: what to do (the row's second line)
	// body is the detail card's text: 2-3 short sentences. It takes the
	// App so it can show the player's live key bindings.
	body func(a *App) string
	// onCmd reports the lesson done by a command the player sent; m is the
	// helper's memory of earlier ones (nil: no command finishes it).
	onCmd func(m *learnMemo, c command.Command) bool
	// onUI reports the lesson done by what changed on screen this Update
	// (nil: no UI event finishes it).
	onUI func(f learnFrame) bool
}

// learnMemo is what the helper remembers of the commands so far, for the
// lessons that take two steps.
type learnMemo struct {
	drafted bool // a colonist was drafted (Draft and move)
}

// learnFrame is what the player did on screen since the helper last looked.
type learnFrame struct {
	panned   bool // the player has panned or zoomed enough (learnCameraInput)
	selected bool // a new thing was selected
	speedSet bool // the player changed the game speed
}

// cmdIn reports c is one of kinds.
func cmdIn(c command.Command, kinds ...command.Kind) bool {
	for _, k := range kinds {
		if c.Kind == k {
			return true
		}
	}
	return false
}

// lessonCatalog is every lesson, in the order they are offered.
var lessonCatalog = []lesson{
	{id: "camera", title: "Look around", goal: "Pan or zoom the map.",
		body: func(a *App) string {
			return fmt.Sprintf("Move the view with %s %s %s %s, or hold the middle mouse button and drag. "+
				"Scroll the wheel to zoom in on a colonist or out to see the whole colony. "+
				"Hold Shift while panning to move faster.",
				a.keyName(input.CamUp), a.keyName(input.CamLeft), a.keyName(input.CamDown), a.keyName(input.CamRight))
		},
		onUI: func(f learnFrame) bool { return f.panned }},

	{id: "select", title: "Select things", goal: "Click a colonist or anything on the map.",
		body: func(*App) string {
			return "Click a colonist, animal, plant, item or building to see its details and commands at the lower left. " +
				"Drag a box on the map to select several colonists at once. " +
				"Click a portrait along the top to jump to that colonist."
		},
		onUI: func(f learnFrame) bool { return f.selected }},

	{id: "build", title: "Build with the Architect", goal: "Place a blueprint on the map.",
		body: func(a *App) string {
			return fmt.Sprintf("Open the Architect with %s, pick a category, then a tool. "+
				"Click or drag on the map to place blueprints; colonists haul the materials and build them when they have time. "+
				"Right-click or press Escape to put the tool away.", a.keyName(input.TabArchitect))
		},
		onCmd: func(_ *learnMemo, c command.Command) bool {
			return cmdIn(c, command.DesignateBuild, command.DesignateWall, command.DesignateBed)
		}},

	{id: "zones", title: "Stockpiles and fields", goal: "Paint a stockpile or a growing zone.",
		body: func(*App) string {
			return "The Architect's Zones category paints areas on the ground. " +
				"A stockpile is where colonists carry loose goods, and a growing zone is soil sown with the crop you pick. " +
				"Paint a stockpile near your door and a growing zone on fertile ground."
		},
		onCmd: func(_ *learnMemo, c command.Command) bool {
			return cmdIn(c, command.DesignateStockpile, command.DesignateGrowZone)
		}},

	{id: "work", title: "Set work priorities", goal: "Change a priority in the Work tab.",
		body: func(a *App) string {
			return fmt.Sprintf("Open the Work tab with %s. "+
				"Each column is a kind of job and each cell is one colonist's priority: 1 is done first, 4 last, and an empty cell means never. "+
				"Click a cell to cycle it.", a.keyName(input.TabWork))
		},
		onCmd: func(_ *learnMemo, c command.Command) bool {
			return cmdIn(c, command.SetWorkPriority, command.SetWorkEnabled)
		}},

	{id: "schedule", title: "Plan the day", goal: "Paint some hours in the Schedule tab.",
		body: func(a *App) string {
			return fmt.Sprintf("Open the Schedule tab with %s. "+
				"Each row is a colonist and each cell an hour of the day: pick a brush such as work, recreation or sleep, then click or drag across cells. "+
				"Colonists follow the plan hour by hour.", a.keyName(input.TabSchedule))
		},
		onCmd: func(_ *learnMemo, c command.Command) bool { return cmdIn(c, command.SetSchedule) }},

	{id: "draft", title: "Draft and move", goal: "Draft a colonist, then move it.",
		body: func(a *App) string {
			return fmt.Sprintf("Select a colonist and press %s to draft them. "+
				"A drafted colonist drops their routine and obeys you: right-click the ground to send them there. "+
				"Press %s again to send them back to work.", a.keyName(input.Draft), a.keyName(input.Draft))
		},
		onCmd: func(m *learnMemo, c command.Command) bool {
			switch c.Kind {
			case command.DraftPawn:
				if c.Enabled {
					m.drafted = true
				}
			case command.MovePawn:
				return m.drafted
			}
			return false
		}},

	{id: "forbid", title: "Forbid and haul", goal: "Forbid a stack, or mark one for hauling.",
		body: func(a *App) string {
			return fmt.Sprintf("Select a stack of items and press %s to forbid it; colonists leave forbidden goods alone until you allow them again. "+
				"Goods lying outside a stockpile are not carried anywhere until you mark them with a Haul button. "+
				"The Architect's Orders category has a Haul tool for whole areas.", a.keyName(input.Forbid))
		},
		onCmd: func(_ *learnMemo, c command.Command) bool {
			return c.Enabled && cmdIn(c, command.SetForbidden, command.DesignateForbid, command.DesignateHaul, command.SetHaulMark)
		}},

	{id: "research", title: "Start research", goal: "Pick a research project.",
		body: func(a *App) string {
			return fmt.Sprintf("Open the Research tab with %s and pick a project from the list. "+
				"Research needs a research bench and a colonist with Research work turned on. "+
				"Finished projects open up new options.", a.keyName(input.TabResearch))
		},
		onCmd: func(_ *learnMemo, c command.Command) bool { return cmdIn(c, command.SetResearch) }},

	{id: "speed", title: "Control time", goal: "Pause, or change the game speed.",
		body: func(a *App) string {
			return fmt.Sprintf("Press %s to pause and think, and %s, %s and %s for normal, fast and superfast speed. "+
				"The game pauses itself when danger arrives unless you turn Pause on threat off in Options. "+
				"Orders you give while paused still take effect.",
				a.keyName(input.Pause), a.keyName(input.Speed1), a.keyName(input.Speed2), a.keyName(input.Speed3))
		},
		onUI: func(f learnFrame) bool { return f.speedSet }},
}

// lessonByID is the catalog entry with id (nil: none).
func lessonByID(id string) *lesson {
	for i := range lessonCatalog {
		if lessonCatalog[i].id == id {
			return &lessonCatalog[i]
		}
	}
	return nil
}

// keyName is an action's first key for lesson text ("its key" when unbound).
func (a *App) keyName(act input.Action) string {
	if l := a.bindings().Label(act); l != "" {
		return l
	}
	return "its key"
}
