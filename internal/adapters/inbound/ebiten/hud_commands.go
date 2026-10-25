package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// Command buttons (PLAY-018: CORE-PANE owns them): every action on the
// selection is a square button (icon, label, hotkey, disabled reason) in a
// grid right of the inspect pane, filling the bottom row first and
// wrapping upward (the layout's GridCells). Each selection kind declares
// its commands (cmds_*.go); a group selection merges its members' commands
// by key. Hotkeys follow declaration order.

// cmdOption is one entry of a command's options menu (right-click, or the
// corner arrow): "now" by a colonist, a material, an owner.
type cmdOption struct {
	Label    string
	Disabled bool
	Reason   string
	Cmd      *command.Command // enqueued when picked, else Run
	Run      func(a *App)
}

// cmdButton is one command button.
type cmdButton struct {
	ID     string // click id; legacy ids are kept ("pawn.draft", "item.forbid", ...)
	Key    string // merge key across a group ("" = ID)
	Label  string
	Icon   string // ui_icon name
	Desc   string // tooltip body
	Hotkey input.Action

	Disabled bool
	Reason   string
	Toggle   bool // a toggle (Draft, Forbid): On says which way
	On       bool
	Mixed    bool // a merged toggle that is on for some members only

	Run     func(a *App)      // run on click, else Cmds, else applyClick(ID)
	Cmds    []command.Command // enqueued on click
	Options func(a *App) []cmdOption
}

// selectionCommands are the current selection's (or the Architect's)
// commands, in declaration order.
func (a *App) selectionCommands() []cmdButton {
	if a.renderer == nil {
		return nil
	}
	switch {
	case a.archOpen:
		return a.archToolCommands()
	case a.grouped():
		return a.groupCommands()
	case a.renderer.HasSelect():
		return a.pawnCommands(a.renderer.Selected())
	case a.hasAnimal:
		return a.animalCommands(a.selAnimal)
	case a.hasItem:
		return a.itemCommands(a.selItem)
	case a.hasPlant:
		return a.plantCommands(a.selPlant)
	case a.hasBuilding:
		return a.buildingCommands(a.selBuilding)
	case a.hasFrame:
		return a.frameCommands(a.selFrame)
	}
	return nil
}

// commandCount is the number of command buttons the layout reserves (0
// while the inspectors keep their legacy in-panel buttons).
func (a *App) commandCount() int { return 0 }

// groupCommands are a group's merged commands.
func (a *App) groupCommands() []cmdButton { return nil }

// commandGridButtons are the command buttons as hit targets.
func (a *App) commandGridButtons() []button { return nil }

// drawCommandGrid draws the command buttons.
func (a *App) drawCommandGrid(screen *ebiten.Image) {}

// commandGridChrome is the grid's legacy area (none).
func (a *App) commandGridChrome() []rect { return nil }

// applyCommandClick handles a "cmd." click.
func (a *App) applyCommandClick(id string) {}

// rightClickCommand opens a command's options under a right-click.
func (a *App) rightClickCommand(mx, my int) bool { return false }

// commandGridUI declares the grid's widget-kit parts (stub).
func (a *App) commandGridUI(sw, sh int) {}

// hintAnchor is where disabled-reason hints and toasts sit: their left
// edge and the bottom they rest on (legacy: just above the colonist bar).
func (a *App) hintAnchor(sh int) (x, bottom float32) { return hudPad, hudBarY(sh) - 4 }
