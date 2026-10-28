package devtools

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Env is the program as the developer panels read it, as of the call.
type Env struct {
	// On is developer mode (Options > Gameplay), never in a commitment
	// colony.
	On bool
	// Game is the running colony's simulation (nil: none, a menu screen).
	// The debug actions and the map fields overlay assert the narrow
	// behaviour they need (Tools, FieldSource) on it.
	Game any
	// Owed and Held are the Update budget's carried and held-back ticks.
	Owed, Held int
}

// Selected is what is selected, for the debug actions and the inspector:
// at most one of the single selections is set; Group holds a multi-select.
type Selected struct {
	Pawn     *game.PawnView
	Animal   *game.AnimalView
	Item     *game.ItemView
	Building *game.BuildingView
	Plant    *game.PlantView
	Frame    *game.FrameView
	Group    []entity.EntityID
}

// Host is what the feature needs of the program that no ports role
// carries.
type Host interface {
	// Env is the program now.
	Env() Env
	// HUD is this frame's HUD layout (the dev column sits between its
	// regions).
	HUD() hudlayout.Layout
	// Toast shows a transient message.
	Toast(msg string)
	// Selected is the current selection.
	Selected() Selected
	// StepTick asks for one simulation tick while paused.
	StepTick()
	// Icon is the UI icon "ui_icon/<name>" (nil while the art is missing).
	Icon(name string) *ebiten.Image
}

// Shell composes the roles the panels call with their Host.
type Shell interface {
	ports.Sizer
	ports.Commands
	ports.World
	ports.View
	ports.Speed
	ports.Widgets
	ports.Settings
	Host
}

// Panel is developer mode: the debug actions menu, the log and inspector
// panels and the map fields overlay (UI-016, MAPGEN-2 §6.2). It writes the
// game only through Tools and commands.
type Panel struct {
	sh Shell
	// State is the panels' presentation state. The root points it at its
	// own copy until the sweep deletes that.
	State *State
}

// New builds the feature over its shell.
func New(sh Shell) *Panel { return &Panel{sh: sh, State: &State{}} }
