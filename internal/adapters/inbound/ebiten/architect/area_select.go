package architect

import (
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The allowed-area selectors that live on legacy panels (PLAY-015 §1.3):
// the Schedule tab's "Allowed area" column (Dropdown, area_menu.go,
// declared from the colony's widget hook over the panel's own layout) and
// the colonist pane's "Area: ..." chip (PLAY-018), which opens the same
// picker. The panels hand their geometry over through Panes.

const (
	// SchedAreaW is the Schedule column's width.
	SchedAreaW    = 170
	schedAreaMinW = 80 // narrower than this and the column is left out
	// SchedAreaGap is the gap before the Schedule column.
	SchedAreaGap = 8
	// AreaChipID is the colonist pane's allowed-area chip (it opens the
	// area picker; the pane's own name for it is paneAreaChipID).
	AreaChipID = "inspector.area"
)

// Selectors declares the selectors and keeps the area overlay in step
// with the armed tool. Called once per Update by the colony's widgets.
func (a *Areas) Selectors(p Panes) {
	if a.sim() == nil {
		return
	}
	a.SyncOverlay()
	env := a.sh.Env()
	if env.UIHidden {
		return
	}
	if env.WinOpen && env.Window == "schedule" {
		a.scheduleColumn(p.Schedule())
	}
	a.inspectorChip(p)
}

// scheduleRect is the column of colonist row `row` of the Schedule tab
// (row 0, Everyone, has none); ok false when the panel is too narrow.
func scheduleRect(s Schedule, row int) (gfx.Rect, bool) {
	r := s.Win
	x := r.X + 12 + s.NameW + 24*s.CellW + SchedAreaGap
	w := min(SchedAreaW, r.X+r.W-12-x)
	if w < schedAreaMinW {
		return gfx.Rect{}, false
	}
	y := r.Y + s.Top + float32(row)*s.RowH
	return gfx.Rect{X: x, Y: y, W: w, H: s.RowH - 3}, true
}

// scheduleColumn is the Schedule tab's "Allowed area" column: one
// dropdown per colonist, emitting AssignArea{Pawn, Index: ref}.
func (a *Areas) scheduleColumn(s Schedule) {
	r := s.Win
	if hr, ok := scheduleRect(s, 0); ok {
		a.sh.Kit().Label(hr.X, hr.Y-float32(gfx.LineH(gfx.Small))-1, "Allowed area", gfx.WithColor(gfx.Small, uikit.TextDim))
	}
	for row, id := range s.Rows {
		if row == 0 {
			continue
		}
		br, ok := scheduleRect(s, row)
		if !ok || br.Y > r.Y+r.H-20 {
			break
		}
		pawn := entity.EntityID(id)
		ref, known := a.ColonistArea(pawn)
		if !known {
			continue
		}
		a.Dropdown("sched.area."+strconv.FormatUint(id, 10), br, ref, func(ref int) { a.Assign(pawn, ref) })
	}
}

// Chip is the pane's allowed-area chip for colonist p, "Area: Indoors"
// (ok false: a dead colonist, or a game that reports no areas). A click on
// it opens the same picker as the Schedule tab's dropdown.
func (a *Areas) Chip(p game.PawnView) (Chip, bool) {
	sim := a.sim()
	if sim == nil || p.Status == "dead" {
		return Chip{}, false
	}
	ref, known := a.ColonistArea(p.ID)
	if !known {
		return Chip{}, false
	}
	return Chip{ID: AreaChipID, Label: "Area: " + AreaRefLabel(sim.Areas(), ref), Tip: AssignAreaTip}, true
}

// inspectorChip answers the pane's area chip: a widget with the chip's
// id, over the chip's button when the pane has drawn it, whose click
// opens the area picker (the click path runs a clicked widget id's action
// first, so the shell needs no route for it).
func (a *Areas) inspectorChip(panes Panes) {
	p, ok := panes.SelectedPawn()
	if !ok {
		return
	}
	chip, ok := a.Chip(p)
	if !ok {
		return
	}
	var r gfx.Rect
	for _, b := range panes.PaneButtons() {
		if b.ID == chip.ID {
			r = gfx.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
			break
		}
	}
	pawn := p.ID
	a.sh.Kit().Add(uikit.Widget{ID: chip.ID, Label: chip.Label, R: r, Click: func() { a.openChip(pawn) }})
}

// openChip opens colonist id's allowed-area picker; a pick emits
// AssignArea.
func (a *Areas) openChip(id entity.EntityID) {
	if a.sim() == nil {
		return
	}
	ref, known := a.ColonistArea(id)
	if !known {
		return
	}
	a.OpenPicker("Allowed area", ref, true, func(ref int) { a.Assign(id, ref) })
}
