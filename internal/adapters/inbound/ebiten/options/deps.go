package options

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/options/saves"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/gameplay"
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
	Hardware   func() sysinfo.Info // nil: the machine cannot be read
	// DeviceScale is the display's pixel density (read lazily: it asks the
	// window).
	DeviceScale func() float64
	// RenderLimit is the physical size the render cap holds the frame to
	// (ui_scale.go), and whether it is holding it.
	RenderLimit func() (w, h int, capped bool)
	UIScale     float64 // the UI step in use, physical px per logical px (Auto shows it)
	// Window is the window's physical size in pixels (nil: the logical size times UIScale),
	// which decides whether a UI step is allowed (UIS-003).
	Window   func() (w, h int)
	Bindings input.Bindings // the live key bindings
	Pressed  []string       // keys that went down this frame
	Mods     input.Mod      // modifiers held with them
	// OpenFolder shows a folder in the file browser (nil: platform/openfolder).
	OpenFolder func(path string) error
	// SavesDir and LogDir are the folders the General tab opens ("": the
	// per-user config folder's saves and logs).
	SavesDir, LogDir string
	// LangDir is assets/lang, where the Language row lists the catalogs
	// ("": English only).
	LangDir string
}

// Host is what the screen needs of the program that no ports role
// carries.
type Host interface {
	ports.Texts
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

// Story is what the Story and Habits tabs borrow from the New Colony wizard
// and the running colony.
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
	// Rules is the colony's habit rules, for the Habits tab (the zero value
	// outside a colony).
	Rules() gameplay.Settings
}

// Nav is how the screen tells the shell something: a hint, or an intent (the
// window X sends ports.Back, UIS-075).
type Nav interface {
	ports.Hinter
	ports.Dispatcher
}

// Shell composes the roles the screen calls with its Host. The save slots
// are the Saves book's (options/saves), with a shell of its own.
type Shell interface {
	Nav
	ports.Sizer
	ports.Settings
	ports.Widgets
	ports.World
	Host
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

// Screen is the Options screen and the settings store's UI (ARCH-005); it
// holds the save slots' book (Saves) for the root.
type Screen struct {
	sh Shell

	// The state other root files read until the sweep repoints them to
	// methods (ARCH-005 wave 5.3; the lead moved these from App).
	Tab        string // the open Options tab ("" the first)
	namesDraft string // the preferred names dialog's text
	// Saves is the running colony's saves and the Load screen (SAV-001…025);
	// the root builds it over its own shell.
	Saves *saves.Book
	// Scale is the UI scale row's list, refusal and keep-or-revert question.
	Scale  ScaleControl
	rowOff int // the first row shown when a tab has more rows than fit
}

// New builds the feature over its shell.
func New(sh Shell) *Screen { return &Screen{sh: sh, Scale: ScaleControl{sh: sh}} }
