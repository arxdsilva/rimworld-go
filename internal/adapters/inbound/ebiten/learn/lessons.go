package learn

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The learning helper's lessons (PLAY-018: LEARN owns them). Short lessons in
// our own words, shown three at a time in order. A lesson is done
// when the player does the thing (a command seen through Host.Chain, or a UI
// event read each Update, helper.go) or presses "Got it".

// KeyFunc names an action's key for lesson text (Settings.KeyName).
type KeyFunc func(input.Action) string

// Lesson is one entry of the helper.
type Lesson struct {
	ID    string // saved in Gameplay.LearnedLessons: never change one
	Title string // the row's heading
	Goal  string // one line: what to do (the row's second line)
	// Text is the detail card's text: 2-3 short sentences, translated by tr.
	// It takes the key namer so it can show the player's live key bindings.
	Text func(tr lang.Tr, key KeyFunc) string
	// OnCmd reports the lesson done by a command the player sent; m is the
	// helper's memory of earlier ones (nil: no command finishes it).
	OnCmd func(m *Memo, c command.Command) bool
	// OnUI reports the lesson done by what changed on screen this Update
	// (nil: no UI event finishes it).
	OnUI func(f Frame) bool
	// Needs says the lesson applies with these settings (nil: always). One
	// that does not is neither shown nor counted.
	Needs func(config.Settings) bool
}

// Body is the detail card's text in English.
func (l *Lesson) Body(key KeyFunc) string { return l.Text(en, key) }

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

	CardRead    bool // the pointer rested on the colonist card
	SetupOpened bool // the card's setup window just opened
	LookRead    bool // the setup window stayed on its Look tab a while
	LayoutsSeen bool // the setup window is on its Sections or Presets tab
	SetupClosed bool // the card's setup window just closed
}

// cardOn gates the colonist card's lessons: without the card there is no gear
// to press.
func cardOn(s config.Settings) bool { return s.HUD.Enabled }

// cmdIn reports c is one of kinds.
func cmdIn(c command.Command, kinds ...command.Kind) bool {
	for _, k := range kinds {
		if c.Kind == k {
			return true
		}
	}
	return false
}

// en marks the English source text of a lesson; the helper translates it
// where it draws.
var en lang.English

// Catalog is every lesson, in the order they are offered.
var Catalog = []Lesson{
	{ID: "camera", Title: en.T("Look around"), Goal: en.T("Pan or zoom the map."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Move the view with %s %s %s %s, or hold the middle mouse button and drag. Scroll the wheel to zoom in on a colonist or out to see the whole colony. Hold Shift while panning to move faster.",
			key(input.CamUp), key(input.CamLeft), key(input.CamDown), key(input.CamRight))
	},
		OnUI: func(f Frame) bool { return f.Panned }},

	{ID: "select", Title: en.T("Select things"), Goal: en.T("Click a colonist or anything on the map."), Text: func(tr lang.Tr, _ KeyFunc) string {
		return tr.T("Click a colonist, animal, plant, item or building to see its details and commands at the lower left. Drag a box on the map to select several colonists at once. Click a portrait along the top to jump to that colonist.")
	},
		OnUI: func(f Frame) bool { return f.Selected }},

	{ID: "build", Title: en.T("Build with the Architect"), Goal: en.T("Place a blueprint on the map."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Open the Architect with %s, pick a category, then a tool. Click or drag on the map to place blueprints; colonists haul the materials and build them when they have time. Right-click or press Escape to put the tool away.", key(input.TabArchitect))
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return cmdIn(c, command.DesignateBuild, command.DesignateWall, command.DesignateBed)
		}},

	{ID: "zones", Title: en.T("Stockpiles and fields"), Goal: en.T("Paint a stockpile or a growing zone."), Text: func(tr lang.Tr, _ KeyFunc) string {
		return tr.T("The Architect's Zones category paints areas on the ground. A stockpile is where colonists carry loose goods, and a growing zone is soil sown with the crop you pick. Paint a stockpile near your door and a growing zone on fertile ground.")
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return cmdIn(c, command.DesignateStockpile, command.DesignateGrowZone)
		}},

	{ID: "work", Title: en.T("Set work priorities"), Goal: en.T("Change a priority in the Work tab."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Open the Work tab with %s. Each column is a kind of job and each cell is one colonist's priority: 1 is done first, 4 last, and an empty cell means never. Click a cell to cycle it.", key(input.TabWork))
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return cmdIn(c, command.SetWorkPriority, command.SetWorkEnabled)
		}},

	{ID: "schedule", Title: en.T("Plan the day"), Goal: en.T("Paint some hours in the Schedule tab."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Open the Schedule tab with %s. Each row is a colonist and each cell an hour of the day: pick a brush such as work, recreation or sleep, then click or drag across cells. Colonists follow the plan hour by hour.", key(input.TabSchedule))
	},
		OnCmd: func(_ *Memo, c command.Command) bool { return cmdIn(c, command.SetSchedule) }},

	{ID: "draft", Title: en.T("Draft and move"), Goal: en.T("Draft a colonist, then move it."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Select a colonist and press %s to draft them. A drafted colonist drops their routine and obeys you: right-click the ground to send them there. Press %s again to send them back to work.", key(input.Draft), key(input.Draft))
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

	{ID: "forbid", Title: en.T("Forbid and haul"), Goal: en.T("Forbid a stack, or mark one for hauling."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Select a stack of items and press %s to forbid it; colonists leave forbidden goods alone until you allow them again. Goods lying outside a stockpile are not carried anywhere until you mark them with a Haul button. The Architect's Orders category has a Haul tool for whole areas.", key(input.Forbid))
	},
		OnCmd: func(_ *Memo, c command.Command) bool {
			return c.Enabled && cmdIn(c, command.SetForbidden, command.DesignateForbid, command.DesignateHaul, command.SetHaulMark)
		}},

	{ID: "research", Title: en.T("Start research"), Goal: en.T("Pick a research project."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Open the Research tab with %s, select a project in the tree and press Start research. Research needs a research bench that can work it and a colonist with Research work turned on. Finished projects open up new options.", key(input.TabResearch))
	},
		OnCmd: func(_ *Memo, c command.Command) bool { return cmdIn(c, command.SetResearch) }},

	{ID: "speed", Title: en.T("Control time"), Goal: en.T("Pause, or change the game speed."), Text: func(tr lang.Tr, key KeyFunc) string {
		return lang.Tf(tr, "Press %s to pause and think, and %s, %s and %s for normal, fast and superfast speed. The game pauses itself when danger arrives unless you turn Pause on threat off in Options. Orders you give while paused still take effect.",
			key(input.Pause), key(input.Speed1), key(input.Speed2), key(input.Speed3))
	},
		OnUI: func(f Frame) bool { return f.SpeedSet }},

	{ID: "dossier_hover", Title: en.T("Read the colonist card"), Goal: en.T("Rest the pointer on a colonist's card."), Needs: cardOn, Text: func(tr lang.Tr, _ KeyFunc) string {
		return tr.T("Select a colonist and the card shows their state, needs and work at a glance. Rest the pointer on a line and a tip says where its number comes from. Some lines also open what they name when you click them.")
	},
		OnUI: func(f Frame) bool { return f.CardRead }},

	{ID: "dossier_gear", Title: en.T("Open the card settings"), Goal: en.T("Click the gear on the colonist card."), Needs: cardOn, Text: func(tr lang.Tr, _ KeyFunc) string {
		return tr.T("A small gear shows on the card while the pointer is over it. It opens the setup window, where every change reaches the card at once and nothing touches the colony.")
	},
		OnUI: func(f Frame) bool { return f.SetupOpened }},

	{ID: "dossier_look", Title: en.T("Settings on the Look tab"), Goal: en.T("Read the Look tab of the card setup."), Needs: cardOn, Text: func(tr lang.Tr, _ KeyFunc) string {
		return tr.T("The Look tab sets where the card sits, how big it is, its text and how often it refreshes. A greyed row says why it is off right now. Back to defaults puts everything as it was.")
	},
		OnUI: func(f Frame) bool { return f.LookRead }},

	{ID: "dossier_sections", Title: en.T("Sections and presets"), Goal: en.T("Open the Sections or Presets tab."), Needs: cardOn, Text: func(tr lang.Tr, _ KeyFunc) string {
		return tr.T("Sections lists what the card shows and in what order, and you can move, remove or add any line. Presets keeps a card you like as a file, to load again or to pass to a friend.")
	},
		OnUI: func(f Frame) bool { return f.LayoutsSeen }},

	{ID: "dossier_close", Title: en.T("Close the setup window"), Goal: en.T("Press Close when you are done."), Needs: cardOn, Text: func(tr lang.Tr, _ KeyFunc) string {
		return tr.T("Your changes are already kept, so Close only puts the window away. The card stays as you left it in every colony, and a switch in Options turns it off if you prefer the plain pane.")
	},
		OnUI: func(f Frame) bool { return f.SetupClosed }},
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
