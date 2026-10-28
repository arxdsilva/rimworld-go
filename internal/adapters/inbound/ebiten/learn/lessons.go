package learn

import (
	"fmt"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// The learning helper's lessons (PLAY-018: LEARN owns them). Ten short
// lessons in our own words, shown three at a time in order. A lesson is done
// when the player does the thing (a command seen through Host.Chain, or a UI
// event read each Update, helper.go) or presses "Got it".

// KeyFunc names an action's key for lesson text (Settings.KeyName).
type KeyFunc func(input.Action) string

// Lesson is one entry of the helper.
type Lesson struct {
	ID    string // saved in Gameplay.LearnedLessons: never change one
	Title string // the row's heading
	Goal  string // one line: what to do (the row's second line)
	// Body is the detail card's text: 2-3 short sentences. It takes the
	// key namer so it can show the player's live key bindings.
	Body func(key KeyFunc) string
	// OnCmd reports the lesson done by a command the player sent; m is the
	// helper's memory of earlier ones (nil: no command finishes it).
	OnCmd func(m *Memo, c command.Command) bool
	// OnUI reports the lesson done by what changed on screen this Update
	// (nil: no UI event finishes it).
	OnUI func(f Frame) bool
}

// Memo is what the helper remembers of the commands so far, for the
// lessons that take two steps.
type Memo struct {
	Drafted bool // a colonist was drafted (Draft and move)
}

// Frame is what the player did on screen since the helper last looked.
type Frame struct {
	Panned   bool // the player has panned or zoomed enough (cameraInput)
	Selected bool // a new thing was selected
	SpeedSet bool // the player changed the game speed
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

// Catalog is every lesson, in the order they are offered.
var Catalog = []Lesson{
	{ID: "camera", Title: "Look around", Goal: "Pan or zoom the map.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Move the view with %s %s %s %s, or hold the middle mouse button and drag. "+
			"Scroll the wheel to zoom in on a colonist or out to see the whole colony. "+
			"Hold Shift while panning to move faster.",
			key(input.CamUp), key(input.CamLeft), key(input.CamDown), key(input.CamRight))
	},
		OnUI: func(f Frame) bool { return f.Panned }},

	{ID: "select", Title: "Select things", Goal: "Click a colonist or anything on the map.", Body: func(KeyFunc) string {
		return "Click a colonist, animal, plant, item or building to see its details and commands at the lower left. " +
			"Drag a box on the map to select several colonists at once. " +
			"Click a portrait along the top to jump to that colonist."
	},
		OnUI: func(f Frame) bool { return f.Selected }},

	{ID: "build", Title: "Build with the Architect", Goal: "Place a blueprint on the map.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Open the Architect with %s, pick a category, then a tool. "+
			"Click or drag on the map to place blueprints; colonists haul the materials and build them when they have time. "+
			"Right-click or press Escape to put the tool away.", key(input.TabArchitect))
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return cmdIn(c, command.DesignateBuild, command.DesignateWall, command.DesignateBed)
		}},

	{ID: "zones", Title: "Stockpiles and fields", Goal: "Paint a stockpile or a growing zone.", Body: func(KeyFunc) string {
		return "The Architect's Zones category paints areas on the ground. " +
			"A stockpile is where colonists carry loose goods, and a growing zone is soil sown with the crop you pick. " +
			"Paint a stockpile near your door and a growing zone on fertile ground."
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return cmdIn(c, command.DesignateStockpile, command.DesignateGrowZone)
		}},

	{ID: "work", Title: "Set work priorities", Goal: "Change a priority in the Work tab.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Open the Work tab with %s. "+
			"Each column is a kind of job and each cell is one colonist's priority: 1 is done first, 4 last, and an empty cell means never. "+
			"Click a cell to cycle it.", key(input.TabWork))
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return cmdIn(c, command.SetWorkPriority, command.SetWorkEnabled)
		}},

	{ID: "schedule", Title: "Plan the day", Goal: "Paint some hours in the Schedule tab.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Open the Schedule tab with %s. "+
			"Each row is a colonist and each cell an hour of the day: pick a brush such as work, recreation or sleep, then click or drag across cells. "+
			"Colonists follow the plan hour by hour.", key(input.TabSchedule))
	},
		OnCmd: func(_ *Memo, c command.Command) bool { return cmdIn(c, command.SetSchedule) }},

	{ID: "draft", Title: "Draft and move", Goal: "Draft a colonist, then move it.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Select a colonist and press %s to draft them. "+
			"A drafted colonist drops their routine and obeys you: right-click the ground to send them there. "+
			"Press %s again to send them back to work.", key(input.Draft), key(input.Draft))
	},
		OnCmd: func(m *Memo, c command.Command) bool {
			switch c.Kind {
			case command.DraftPawn:
				if c.Enabled {
					m.Drafted = true
				}
			case command.MovePawn:
				return m.Drafted
			}
			return false
		}},

	{ID: "forbid", Title: "Forbid and haul", Goal: "Forbid a stack, or mark one for hauling.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Select a stack of items and press %s to forbid it; colonists leave forbidden goods alone until you allow them again. "+
			"Goods lying outside a stockpile are not carried anywhere until you mark them with a Haul button. "+
			"The Architect's Orders category has a Haul tool for whole areas.", key(input.Forbid))
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return c.Enabled && cmdIn(c, command.SetForbidden, command.DesignateForbid, command.DesignateHaul, command.SetHaulMark)
		}},

	{ID: "research", Title: "Start research", Goal: "Pick a research project.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Open the Research tab with %s and pick a project from the list. "+
			"Research needs a research bench and a colonist with Research work turned on. "+
			"Finished projects open up new options.", key(input.TabResearch))
	},
		OnCmd: func(_ *Memo, c command.Command) bool { return cmdIn(c, command.SetResearch) }},

	{ID: "speed", Title: "Control time", Goal: "Pause, or change the game speed.", Body: func(key KeyFunc) string {
		return fmt.Sprintf("Press %s to pause and think, and %s, %s and %s for normal, fast and superfast speed. "+
			"The game pauses itself when danger arrives unless you turn Pause on threat off in Options. "+
			"Orders you give while paused still take effect.",
			key(input.Pause), key(input.Speed1), key(input.Speed2), key(input.Speed3))
	},
		OnUI: func(f Frame) bool { return f.SpeedSet }},
}

// ByID is the catalog entry with id (nil: none).
func ByID(id string) *Lesson {
	for i := range Catalog {
		if Catalog[i].ID == id {
			return &Catalog[i]
		}
	}
	return nil
}
