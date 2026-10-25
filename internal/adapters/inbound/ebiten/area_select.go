package ebiten

import (
	"strconv"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The allowed-area selectors that live on legacy panels (PLAY-015 §1.3):
// the Schedule tab's "Allowed area" column and the pawn inspector's
// "Area: ..." line. Each is areaDropdown (area_menu.go) declared from the
// colony's widget hook, over the legacy panel's own layout.

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
	r := workRect(a.sw, a.sh)
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
	r := workRect(a.sw, a.sh)
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

// inspectorAreaOn reports the inspector's info rows (and so the Area line)
// are showing for the selected pawn p.
func (a *App) inspectorAreaOn(p game.PawnView) bool {
	if a.workOpen || a.archOpen || p.Status == "dead" {
		return false
	}
	switch a.inspectTab {
	case "needs", "gear", "log", "health", "social", "bio", "prisoner":
		return false
	}
	return true
}

// inspectorAreaRect is the Area line's rectangle: the row below the info
// rows (job, activity, tile, weapon and the optional postponed and
// dirty-boots lines).
func inspectorAreaRect(p game.PawnView) rect {
	r := inspectorRect()
	rows := 4
	if postponedLine(p.Postponed) != "" {
		rows++
	}
	if dirtyBootsLine(p) != "" {
		rows++
	}
	rowH := float32(lineStep(tsBody))
	return rect{r.x + 72, r.y + insInfoY + float32(rows)*rowH, r.w - 84, rowH}
}

// inspectorAreaUI is the pawn inspector's "Area: Indoors" line; a click
// opens the same dropdown as the Schedule tab.
func (a *App) inspectorAreaUI() {
	if !a.renderer.HasSelect() {
		return
	}
	p, ok := a.selectedPawn()
	if !ok || !a.inspectorAreaOn(p) {
		return
	}
	ref, known := a.colonistArea(p.ID)
	if !known {
		return
	}
	br := inspectorAreaRect(p)
	label := "Area: " + areaRefLabel(a.renderer.game.Areas(), ref)
	if a.uiButton("inspector.area", br, label, btnOpts{Tip: assignAreaTip}) {
		a.openAreaPicker("Allowed area", ref, true, func(ref int) { a.assignArea(p.ID, ref) })
	}
}
