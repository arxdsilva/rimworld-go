package ebiten

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/entity"
)

// The inspect pane (PLAY-018: CORE-PANE owns it): a fixed pane at the
// lower-left (header, compact status bars, chips, scrollable lines), its
// tabs in a strip just above it, and a tab's larger panel opening upward
// from there. Each selection kind describes itself as a paneSpec
// (cmds_*.go); this file draws it.

// paneBar is one compact status bar ("Health 82%").
type paneBar struct {
	Label string
	Pct   int // 0..100
	Color color.RGBA
}

// paneChip is a small clickable chip on the status row (e.g. the allowed
// area, which opens the area picker).
type paneChip struct {
	ID, Label, Tip string
	Disabled       bool
	Reason         string
}

// paneTab is one inspector tab. Its body draws into the expanded panel:
// legacy bodies through Draw, widget-kit bodies through Declare (called
// from paneUI each Update).
type paneTab struct {
	ID, Label    string // ID "inspector.<tab>"
	Disabled     bool
	Reason       string
	WantW, WantH float32 // the panel size it asks for
	Draw         func(screen *ebiten.Image, r rect)
	Declare      func(r rect)
}

// paneSpec is what the pane shows for the selection.
type paneSpec struct {
	Title, Subtitle string
	Bars            []paneBar
	Chips           []paneChip
	Lines           []string
	Tabs            []paneTab
	Close           string // the close button's id ("" none)
}

// selectionPane is the pane for the current selection.
func (a *App) selectionPane() paneSpec {
	if a.renderer == nil {
		return paneSpec{}
	}
	switch {
	case a.grouped():
		return a.groupPane()
	case a.renderer.HasSelect():
		return a.pawnPane(a.renderer.Selected())
	case a.hasAnimal:
		return a.animalPane(a.selAnimal)
	case a.hasItem:
		return a.itemPane(a.selItem)
	case a.hasPlant:
		return a.plantPane(a.selPlant)
	case a.hasBuilding:
		return a.buildingPane(a.selBuilding)
	case a.hasFrame:
		return a.framePane(a.selFrame)
	}
	return paneSpec{}
}

// paneShown reports the layout should reserve the pane (false while the
// inspectors draw the legacy way).
func (a *App) paneShown() bool { return false }

// expandedWant is the open tab's wanted panel size (0, 0: none).
func (a *App) expandedWant() (float32, float32) { return 0, 0 }

// paneRect and expandedRect are the pane and the open tab's panel.
func (a *App) paneRect() rect     { return a.hud().Pane }
func (a *App) expandedRect() rect { return a.hud().Expanded }

// drawPane draws the pane, its tabs and the open tab's panel.
func (a *App) drawPane(screen *ebiten.Image) {}

// paneButtons are the pane's buttons: tabs, chips, close.
func (a *App) paneButtons() []button { return nil }

// paneChrome is the legacy inspector panels' area while they still draw
// the old way (top-left).
func (a *App) paneChrome() []rect {
	if a.renderer == nil || a.workOpen {
		return nil
	}
	var out []button
	if a.grouped() && !a.archOpen {
		out = append(out, groupRect(len(a.groupRows())))
	}
	if a.renderer.HasSelect() {
		out = append(out, inspectorRect())
	}
	if p, ok := a.selectedPawn(); ok && len(p.QueuedOrders) > 0 {
		out = append(out, orderQueueRect(len(p.QueuedOrders)))
	}
	if a.hasBuilding && !a.archOpen {
		out = append(out, a.buildingPanelRect())
	}
	if _, ok := a.selectedFrame(); ok && !a.archOpen {
		out = append(out, frameRect())
	}
	rs := make([]rect, len(out))
	for i, b := range out {
		rs[i] = rect{b.x, b.y, b.w, b.h}
	}
	return rs
}

// selectPaneTab handles a pane tab click ("inspector.<tab>").
func (a *App) selectPaneTab(id string) {
	switch tab := strings.TrimPrefix(id, "inspector."); tab {
	case "character", "needs", "gear", "log", "health", "social":
		a.inspectTab = tab
	}
}

// paneUI declares the pane's widget-kit parts (stub).
func (a *App) paneUI(sw, sh int) {}

// groupPane is the pane for a group selection (multiselect).
func (a *App) groupPane() paneSpec { return paneSpec{} }

var _ = entity.EntityID(0)
