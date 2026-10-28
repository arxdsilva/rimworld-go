package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/devtools"
	"rimworld-go/internal/application/game"
)

// Developer mode lives in devtools (ARCH-005). These aliases and one-line
// wrappers keep the root's call sites (the HUD, the click routes, the
// hotkeys, the tests) unchanged while the features move out one package at
// a time; the last mover deletes this file (SOLID_GO.md §11).

type (
	devState         = devtools.State
	devAction        = devtools.Action
	devTools         = devtools.Tools
	devPanelRects    = devtools.PanelRects
	mapFieldsOverlay = devtools.FieldsOverlay
)

const (
	devLosePickID   = devtools.LosePickID
	devLosePickList = devtools.LosePickList
	fieldAlpha      = devtools.FieldAlpha
)

func devColumn(l hudLayout) rect                     { return devtools.Column(l) }
func devLoseParts() []string                         { return devtools.LoseParts() }
func fieldPixels(l game.MapFieldLayer) []byte        { return devtools.FieldPixels(l) }
func fieldLegendLines(l game.MapFieldLayer) []string { return devtools.FieldLegendLines(l) }

// ---- the App's developer-mode methods

func (a *App) devMode() bool                           { return a.tools.Mode() }
func (a *App) devActions(tx, ty int) []devAction       { return a.tools.Actions(tx, ty) }
func (a *App) handleDevKeys()                          { a.tools.HandleKeys(&a.in) }
func (a *App) devMenuRect() button                     { return a.tools.MenuRect() }
func (a *App) devButtons() []button                    { return a.tools.Buttons() }
func (a *App) applyDevClick(id string)                 { a.tools.Click(id) }
func (a *App) devStack() devPanelRects                 { return a.tools.Stack() }
func (a *App) drawMapFields(screen *ebiten.Image)      { a.tools.DrawMapFields(screen) }
func (a *App) drawMapFieldLegend(s *ebiten.Image)      { a.tools.DrawMapFieldLegend(s) }
func (a *App) drawDevPanels(s *ebiten.Image, w, h int) { a.tools.Draw(s, w, h) }

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
func (h devtoolsHost) HUD() hudLayout { return h.a.hud() }

// Toast shows a transient message.
func (h devtoolsHost) Toast(msg string) { h.a.toast(msg) }

// Selected is the current selection.
func (h devtoolsHost) Selected() devtools.Selected {
	a := h.a
	var s devtools.Selected
	if v, ok := a.selectedPawn(); ok {
		s.Pawn = &v
	}
	if v, ok := a.selectedAnimal(); ok {
		s.Animal = &v
	}
	if v, ok := a.selectedItem(); ok {
		s.Item = &v
	}
	if v, ok := a.selectedBuilding(); ok {
		s.Building = &v
	}
	if v, ok := a.selectedPlant(); ok {
		s.Plant = &v
	}
	if v, ok := a.selectedFrame(); ok {
		s.Frame = &v
	}
	if a.grouped() {
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
func (h devtoolsHost) Icon(name string) *ebiten.Image { return h.a.icon(name) }
