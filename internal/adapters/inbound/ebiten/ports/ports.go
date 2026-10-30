// Package ports is the contract between the ebiten client's shell (the
// root package: App and Renderer) and its feature packages. Each role is
// one small consumer-owned interface (SOLID_GO.md §6, HEXAGONAL §6): a
// feature composes the roles it calls into its own Shell and the root
// satisfies them all. Intents are the closed set of cross-feature
// requests a feature hands back to the shell instead of calling another
// feature (SOLID_GO.md §4).
package ports

import (
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// Hinter shows a one-line hint in the HUD (why a click did nothing, what
// a tool wants next).
type Hinter interface {
	Hint(string)
}

// Sizer is the logical screen and the pointer: Size in logical pixels,
// Cursor in logical pixels, MapCursor in map pixels (the render-scaled
// map, world_scale.go) and MapScale the logical-to-map factor.
type Sizer interface {
	Size() (sw, sh int)
	Cursor() (int, int)
	MapCursor() (int, int)
	MapScale() float64
}

// Commands sends a typed command to the simulation through the renderer's
// queue (applied at once while paused, observed by the learn helper).
type Commands interface {
	Enqueue(command.Command)
}

// World is this frame's read model of the colony. Nil-safe: on a menu
// screen World returns the zero view and Pawns nil.
type World interface {
	World() game.WorldView
	Pawns() []game.PawnView
	MapSize() (w, h int)
	Tick() clock.Tick
	Date() clock.Date
}

// View is the camera: tile under a physical point, the tile size in map
// pixels, world-to-screen transforms, centring, and the presentation
// frame counter animation reads.
type View interface {
	TileAt(mx, my int) (int, int)
	TilePx() float64
	WorldToScreen(tx, ty int) (float32, float32)
	WorldToScreenF(wx, wy float64) (float32, float32)
	CenterOn(tx, ty int)
	Present() uint64
}

// PawnSel is the renderer's single selected pawn.
type PawnSel interface {
	Select(entity.EntityID)
	Selected() (entity.EntityID, bool)
	ClearSelect()
}

// Overlays are the map overlays the HUD toggles and the Architect forces.
type Overlays interface {
	RoofShown() bool
	HomeShown() bool
	PlanShown() bool
	PowerShown() bool
	ToggleRoof()
	ToggleHome()
	TogglePlan()
	TogglePower()
}

// Speed is the game clock's tier: 0 paused, 1 normal, 2 fast, 3 superfast
// (4 ultrafast, developer mode). PauseReason says why the game paused
// itself ("" when the player did).
type Speed interface {
	Paused() bool
	TogglePause()
	SetSpeed(tier int)
	SpeedTier() int
	PauseReason() string
}

// Settings is the player's options and the live key names for an action
// (KeyName "Space", KeyHint "[Space]").
type Settings interface {
	Settings() config.Settings
	KeyName(input.Action) string
	KeyHint(input.Action) string
}

// Panels is what the HUD chrome reads about the colony's open panels.
type Panels interface {
	ArchitectOpen() bool
	ArchitectTool() string
	WindowMode() (mode string, open bool)
	UIHidden() bool
}

// Dispatcher takes an Intent: the one way a feature asks the shell to
// change another feature's state.
type Dispatcher interface {
	Do(Intent)
}

// Intent is a closed set: every cross-feature request the shell handles.
type Intent interface{ isIntent() }

// OpenArchitect opens the Architect panel (closing the main-tab windows).
type OpenArchitect struct{}

// CloseArchitect closes the Architect panel and drops its tool.
type CloseArchitect struct{}

// PresetBuildTool opens the Architect on a build tool with a material and
// facing (a building's Copy).
type PresetBuildTool struct{ Kind, Stuff, Facing string }

// AimMortar enters the mortar targeting mode for the mortar at (X, Y).
type AimMortar struct {
	ID   entity.EntityID
	X, Y int
}

// AimGrenade enters the grenade targeting mode for the drafted colonist
// Pawn: the next map click sends ThrowGrenade at that tile.
type AimGrenade struct{ Pawn entity.EntityID }

// OpenStorage opens the Storage settings window on storage zone Zone (a
// shelf's own zone included).
type OpenStorage struct{ Zone uint16 }

// ShowWindow opens a main-tab window ("work", "schedule", "research",
// "animals", …), closing the Architect and any float menu.
type ShowWindow struct{ Mode string }

// CloseWindows closes the open main-tab window.
type CloseWindows struct{}

// SelectPawn makes the pawn the only selection.
type SelectPawn struct{ ID entity.EntityID }

// TogglePawnInSet adds the pawn to, or removes it from, the multi-select.
type TogglePawnInSet struct{ ID entity.EntityID }

// ClearSelection drops every selection.
type ClearSelection struct{}

// CenterOnPawn pans the camera to the pawn.
type CenterOnPawn struct{ ID entity.EntityID }

func (OpenArchitect) isIntent()   {}
func (CloseArchitect) isIntent()  {}
func (PresetBuildTool) isIntent() {}
func (AimMortar) isIntent()       {}
func (AimGrenade) isIntent()      {}
func (OpenStorage) isIntent()     {}
func (ShowWindow) isIntent()      {}
func (CloseWindows) isIntent()    {}
func (SelectPawn) isIntent()      {}
func (TogglePawnInSet) isIntent() {}
func (ClearSelection) isIntent()  {}
func (CenterOnPawn) isIntent()    {}

// Screen is a full-screen route of the client (presentation only; it never
// exists in the domain). The root's screenID is this type.
type Screen int

const (
	ScreenBoot Screen = iota
	ScreenMainMenu
	ScreenScenario
	ScreenStoryteller
	ScreenWorldGen
	ScreenLanding
	ScreenPawns
	ScreenColony
	ScreenOptions
	ScreenLoad
	ScreenCredits
	// ScreenScenarioEditor is the Scenario Editor, pushed from Choose
	// Scenario (NG-1 §2.3).
	ScreenScenarioEditor
)

func (s Screen) String() string {
	switch s {
	case ScreenBoot:
		return "boot"
	case ScreenMainMenu:
		return "main_menu"
	case ScreenScenario:
		return "scenario"
	case ScreenStoryteller:
		return "storyteller"
	case ScreenWorldGen:
		return "world_gen"
	case ScreenLanding:
		return "landing"
	case ScreenPawns:
		return "pawns"
	case ScreenColony:
		return "colony"
	case ScreenOptions:
		return "options"
	case ScreenLoad:
		return "load"
	case ScreenCredits:
		return "credits"
	case ScreenScenarioEditor:
		return "scenario_editor"
	}
	return "unknown"
}

// Navigation intents: a screen asks the shell to route; the shell's
// navigator applies them (nav.go).

// Push opens a screen on top of the current one.
type Push struct{ Screen Screen }

// Back returns to the previous screen.
type Back struct{}

// StartGame leaves the wizard for the colony.
type StartGame struct{}

// Quit ends the game.
type Quit struct{}

// ReturnToMenu drops the colony and returns to the main menu.
type ReturnToMenu struct{}

// Resume closes the pause overlay.
type Resume struct{}

// Pause opens the pause overlay.
type Pause struct{}

func (Push) isIntent()         {}
func (Back) isIntent()         {}
func (StartGame) isIntent()    {}
func (Quit) isIntent()         {}
func (ReturnToMenu) isIntent() {}
func (Resume) isIntent()       {}
func (Pause) isIntent()        {}

// Widgets is the immediate-mode kit a screen declares its widgets into.
type Widgets interface {
	Kit() *uikit.Kit
}

// SaveSlot is one save as the load screen lists it.
type SaveSlot struct {
	Name      string
	Path      string
	Date      string // in-game date
	Colonists int
	Modified  time.Time
	Err       string // not loadable, and why
	// Commitment marks a commitment colony's one save (tagged in the list;
	// it loads like any other).
	Commitment bool
}

// Saves is the load screen's view of the save slots and the two actions
// on them.
type Saves interface {
	Slots() []SaveSlot
	RefreshSlots()
	LoadSlot(i int)
	NewestSlot() int
}
