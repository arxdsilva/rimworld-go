package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/devtools"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
)

// Developer mode lives in devtools (ARCH-005). These aliases and one-line
// wrappers keep the root's call sites (the HUD, the click routes, the
// hotkeys, the tests) unchanged while the features move out one package at
// a time; the last mover deletes this file (SOLID_GO.md §11).

// ---- the App's developer-mode methods

func (a *App) handleDevKeys() { a.tools.HandleKeys(&a.in) }

func (a *App) applyDevClick(id string) { a.tools.Click(id) }

// devtoolsHost is the root's side of devtools.Host.
type devtoolsHost struct{ a *App }

var _ devtools.Host = devtoolsHost{}

// devtoolsShell is the feature's view of the App: the ports roles plus its Host.
type devtoolsShell struct {
	shell
	devtoolsHost
}

// newDevtools builds the panels over the App. Until the sweep deletes
// App.dev the panels' state is that field, so the root's own readers
// (the click-away close, the HUD) see the same state.
func newDevtools(a *App) *devtools.Panel {
	p := devtools.New(devtoolsShell{a.shell, devtoolsHost{a}})
	return p
}

// Env is developer mode, the running game and the Update budget.
func (h devtoolsHost) Env() devtools.Env {
	a := h.a
	e := devtools.Env{On: a.settings.Gameplay.DevMode && !a.commitment(), Owed: a.debt.owed, Held: a.debt.held}
	if a.renderer != nil {
		e.Game = a.renderer.game
	}
	return e
}

// HUD is this frame's HUD layout.
func (h devtoolsHost) HUD() hudlayout.Layout { return h.a.hud() }

// Toast shows a transient message.
func (h devtoolsHost) Toast(msg string) { h.a.chrome.Toast(msg) }

// Selected is the current selection.
func (h devtoolsHost) Selected() devtools.Selected {
	a := h.a
	var s devtools.Selected
	if v, ok := a.insp.SelectedPawn(); ok {
		s.Pawn = &v
	}
	if v, ok := a.insp.SelectedAnimal(); ok {
		s.Animal = &v
	}
	if v, ok := a.sel.SelectedItem(); ok {
		s.Item = &v
	}
	if v, ok := a.insp.SelectedBuilding(); ok {
		s.Building = &v
	}
	if v, ok := a.sel.SelectedPlant(); ok {
		s.Plant = &v
	}
	if v, ok := a.insp.SelectedFrame(); ok {
		s.Frame = &v
	}
	if a.sel.Grouped() {
		s.Group = a.sel.Group.Ids
	}
	return s
}

// StepTick asks for one simulation tick while paused.
func (h devtoolsHost) StepTick() {
	if r := h.a.renderer; r != nil {
		r.stepTicks++
	}
}

// Icon is the UI icon "ui_icon/<name>".
func (h devtoolsHost) Icon(name string) *ebiten.Image { return h.a.arch.Icon(name) }
