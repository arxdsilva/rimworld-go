package ebiten

import (
	"strconv"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The allowed-area selectors that live on legacy panels (PLAY-015 §1.3):
// the Schedule tab's "Allowed area" column (areaDropdown, area_menu.go,
// declared from the colony's widget hook over the panel's own layout) and
// the colonist pane's "Area: ..." chip (PLAY-018), which opens the same
// picker.

const (
	schedAreaW    = 170 // the Schedule column's width
	schedAreaMinW = 80  // narrower than this and the column is left out
	schedAreaGap  = 8
)

// areaSelectorsUI declares the selectors and keeps the area overlay in
// step with the armed tool. Called once per Update by colonyUI.
func (a *App) areaSelectorsUI(sw, sh int) {
	if a.renderer == nil {
		return
	}
	a.syncAreaOverlay()
	if a.hideUI {
		return
	}
	a.scheduleAreaUI()
	a.inspectorAreaUI()
}

// scheduleAreaRect is the column of colonist row `row` of the Schedule
// tab (row 0, Everyone, has none); ok false when the panel is too narrow.
func (a *App) scheduleAreaRect(row int) (rect, bool) {
	r := a.windowRect()
	x := r.x + 12 + workNameW + 24*schedCellW + schedAreaGap
	w := min(schedAreaW, r.x+r.w-12-x)
	if w < schedAreaMinW {
		return rect{}, false
	}
	y := r.y + schedTop + float32(row*schedRowH)
	return rect{x, y, w, schedRowH - 3}, true
}

// scheduleAreaUI is the Schedule tab's "Allowed area" column: one
// dropdown per colonist, emitting AssignArea{Pawn, Index: ref}.
func (a *App) scheduleAreaUI() {
	if !a.workOpen || a.panelMode != "schedule" {
		return
	}
	r := a.windowRect()
	if hr, ok := a.scheduleAreaRect(0); ok {
		a.uiLabel(hr.X, hr.Y-float32(lineH(tsSmall))-1, "Allowed area", withColor(tsSmall, uiTextDim))
	}
	for row, id := range a.scheduleRows() {
		if row == 0 {
			continue
		}
		br, ok := a.scheduleAreaRect(row)
		if !ok || br.Y > r.y+r.h-20 {
			break
		}
		pawn := entity.EntityID(id)
		ref, known := a.colonistArea(pawn)
		if !known {
			continue
		}
		a.areaDropdown("sched.area."+strconv.FormatUint(id, 10), br, ref, func(ref int) { a.assignArea(pawn, ref) })
	}
}

// areaChip is the pane's allowed-area chip for colonist p, "Area: Indoors"
// (ok false: a dead colonist, or a game that reports no areas). A click on
// it opens the same picker as the Schedule tab's dropdown.
func (a *App) areaChip(p game.PawnView) (paneChip, bool) {
	if a.renderer == nil || p.Status == "dead" {
		return paneChip{}, false
	}
	ref, known := a.colonistArea(p.ID)
	if !known {
		return paneChip{}, false
	}
	return paneChip{ID: paneAreaChipID, Label: "Area: " + areaRefLabel(a.renderer.game.Areas(), ref), Tip: assignAreaTip}, true
}

// inspectorAreaUI answers the pane's area chip: a widget with the chip's
// id, over the chip's button when the pane has drawn it, whose click opens
// the area picker (applyClick runs a clicked widget id's action first, so
// app.go needs no case for it).
func (a *App) inspectorAreaUI() {
	if !a.renderer.HasSelect() {
		return
	}
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	chip, ok := a.areaChip(p)
	if !ok {
		return
	}
	var r rect
	for _, b := range a.paneButtons() {
		if b.id == chip.ID {
			r = rect{b.x, b.y, b.w, b.h}
			break
		}
	}
	pawn := p.ID
	a.uiSt().add(uiWidget{id: chip.ID, label: chip.Label, r: r, click: func() { a.openAreaChip(pawn) }})
}

// openAreaChip opens colonist id's allowed-area picker; a pick emits
// AssignArea.
func (a *App) openAreaChip(id entity.EntityID) {
	if a.renderer == nil {
		return
	}
	ref, known := a.colonistArea(id)
	if !known {
		return
	}
	a.openAreaPicker("Allowed area", ref, true, func(ref int) { a.assignArea(id, ref) })
}
