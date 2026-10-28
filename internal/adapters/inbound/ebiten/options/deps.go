package options

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
	"rimworld-go/internal/platform/sysinfo"
)

// Env is what the screen reads of the program around it, as of the call:
// what is running, what the wiring can do, the machine, the live key
// bindings and the keys pressed this frame.
type Env struct {
	Running    bool                // a colony is running (the Story tab exists)
	Commitment bool                // that colony is in commitment mode
	Capture    bool                // a scripted capture or the fps probe runs
	CanSave    bool                // a colony runs and a save hook is wired
	CanLoad    bool                // the save list and loading are wired
	Hardware   func() sysinfo.Info // nil: the machine cannot be read
	// DeviceScale is the display's pixel density (read lazily: it asks the
	// window).
	DeviceScale func() float64
	UIScale     float64        // the live UI scale factor (Auto shows it)
	Bindings    input.Bindings // the live key bindings
	Pressed     []string       // keys that went down this frame
	Mods        input.Mod      // modifiers held with them
}

// Host is what the screen needs of the program that no ports role
// carries.
type Host interface {
	// Env is the program as of now.
	Env() Env
	// SetSettings applies changed settings everywhere (renderer, window,
	// scale, sound) and stores them.
	SetSettings(config.Settings)
	// Cue plays a UI sound by id.
	Cue(id string)
	// Toast shows a short message in the toast stack.
	Toast(text string)
	// Send enqueues a command and applies it at once (the colony is not
	// stepping while Options is open).
	Send(command.Command)
	// PaintMenu draws the menu backdrop, the title, an optional body and
	// the buttons.
	PaintMenu(dst *ebiten.Image, sw, sh int, title string, body []string, buttons []uikit.Button)
	// Rebind is the Controls tab's capture state; the root keeps the value
	// while other root files read it.
	Rebind() *RebindState
}

// Saver is the saved-game storage as the screen uses it.
type Saver interface {
	// Save writes the running colony to slot (its commitment slot or the
	// manual one).
	Save(slot string) error
	// List reads the save list.
	List() ([]ports.SaveSlot, error)
	// Load replaces the session with the save at path and returns its slot
	// name and tick.
	Load(path string) (slot string, tick clock.Tick, err error)
}

// Story is what the Story tab borrows from the New Colony wizard and the
// running colony.
type Story interface {
	// StoryView is the colony's storyteller, difficulty and save mode.
	StoryView() game.StoryView
	// PaintCard draws a storyteller's card.
	PaintCard(dst *ebiten.Image, r gfx.Rect, p storyteller.Profile, selected, hot bool)
	// CardTip is a storyteller card's hover text.
	CardTip(p storyteller.Profile) string
	// PresetTip is a difficulty row's hover text.
	PresetTip(n difficulty.Named) string
	// SaveMode names a save mode and says what it does.
	SaveMode(commitment bool) (label, text string)
	// OpenCustom opens the Custom difficulty dialog on the colony's values;
	// accept gets the settings when the player accepts.
	OpenCustom(v game.StoryView, accept func(difficulty.Settings))
}

// Shell composes the roles the screen calls with its Host.
type Shell interface {
	ports.Hinter
	ports.Sizer
	ports.Settings
	ports.Widgets
	ports.World
	Host
	Saver
	Story
}

// RebindState is the Controls tab's capture and paging.
type RebindState struct {
	On      bool // waiting for a key
	act     input.Action
	slot    int
	Page    int
	Pending *PendingBind // a captured key that collides
}

// PendingBind is a captured key waiting for "swap?".
type PendingBind struct {
	act    input.Action
	slot   int
	key    input.Binding
	others []input.Action
}

// Screen is the Options screen, the settings store's UI and the save
// slots (ARCH-005).
type Screen struct {
	sh Shell

	// The state other root files read until the sweep repoints them to
	// methods (ARCH-005 wave 5.3; the lead moved these from App).
	Tab         string           // the open Options tab ("" the first)
	Slots       []ports.SaveSlot // the Load screen's list
	SlotsRead   bool             // the main menu read the list once
	SaveSlot    string           // the current colony's save slot
	AutosaveDay int              // in-game day of the last autosave
}

// New builds the feature over its shell.
func New(sh Shell) *Screen { return &Screen{sh: sh} }
