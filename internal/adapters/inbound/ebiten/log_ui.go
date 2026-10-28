package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Creature logs (LOG-001): the colonist pane's Log tab and the animal
// pane's Log tab (animal_inspector.go) render LogLineView read models,
// newest first.

const logEmptyText = "Nothing has happened yet."

// logStyle is the log's text: the small size, so a panel shows many rows.
var logStyle = tsSmall

// logLineH is the px height of one log row.
func logLineH() int { return int(lineStep(logStyle)) }

// logLineText is "Day D, HHh  Text".
func logLineText(l game.LogLineView) string {
	return fmt.Sprintf("Day %d, %02dh  %s", l.Day, l.Hour, l.Text)
}

// wrapLogRows lays lines (newest first) out as rows no wider than maxW px;
// a long line wraps onto indented continuation rows.
func wrapLogRows(lines []game.LogLineView, maxW float64) []string {
	maxW = max(maxW, 60)
	indent := "  "
	iw := float64(textW(indent, logStyle))
	var rows []string
	for _, l := range lines {
		for i, row := range wrapPx(logLineText(l), logStyle, maxW-iw) {
			if i > 0 {
				row = indent + row
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// clampLogScroll keeps a scroll offset (in rows) within the list.
func clampLogScroll(scroll, rows, visible int) int {
	return max(0, min(scroll, rows-visible))
}

// drawLogRows draws the visible slice of rows from scroll, with a small
// scroll hint at the right edge (hintX) when more are hidden. Returns the
// clamped offset.
func drawLogRows(screen *ebiten.Image, rows []string, x, y, w, visible, scroll int) int {
	lh := logLineH()
	if len(rows) == 0 {
		drawText(screen, logEmptyText, float64(x), float64(y), withColor(logStyle, uiTextDim))
		return 0
	}
	scroll = clampLogScroll(scroll, len(rows), visible)
	for i := 0; i < visible && scroll+i < len(rows); i++ {
		drawText(screen, rows[scroll+i], float64(x), float64(y+i*lh), logStyle)
	}
	hint := textStyle{Size: logStyle.Size, Color: uiAccent, Align: alignRight}
	if scroll > 0 {
		drawText(screen, "^", float64(x+w-4), float64(y-lh), hint)
	}
	if scroll+visible < len(rows) {
		drawText(screen, "v", float64(x+w-4), float64(y+visible*lh), hint)
	}
	return scroll
}

// logBodyRect is the pawn Log tab's text area: r, the expanded panel's
// inner rectangle, in whole pixels.
func logBodyRect(r rect) (x, y, w, h int) {
	return int(r.X), int(r.Y), int(r.W), int(r.H)
}

// handleLogWheel scrolls the log under the cursor with the mouse wheel and
// reports whether it used the wheel (so the map does not zoom).
func (a *App) handleLogWheel() bool {
	if a.renderer == nil {
		return false
	}
	wy := a.in.Raw.WheelY
	if wy == 0 {
		return false
	}
	mx, my := a.currentCursor()
	return a.scrollLogAt(mx, my, wy)
}

// scrollLogAt scrolls the colonist Log tab under the cursor (the animal's
// Log tab is a widget-kit scroll area: declareAnimalLog).
func (a *App) scrollLogAt(mx, my int, wy float64) bool {
	return a.paneOpenTab() == "inspector.log" && a.inspectorVisible() && a.scrollLogIn(a.expandedRect().Inset(paneInset), mx, my, wy)
}

// scrollLogIn scrolls the selected colonist's Log tab, whose expanded
// panel's inner rectangle is r, when the cursor (mx, my) is over its text
// area; wy is the wheel. It reports it used the wheel.
func (a *App) scrollLogIn(r rect, mx, my int, wy float64) bool {
	p, ok := a.selectedPawn()
	if !ok || wy == 0 {
		return false
	}
	step := 3
	if wy > 0 {
		step = -3
	}
	x, y, w, h := logBodyRect(r)
	if mx < x || mx >= x+w || my < y || my >= y+h {
		return false
	}
	a.logScroll = max(0, a.logScroll+step)
	a.syncLogFor(p.ID)
	return true
}

func (a *App) inspectorVisible() bool {
	_, ok := a.shell.Selected()
	return ok && !a.workOpen && !a.archOpen
}

// syncLogFor resets the colonist log scroll when the selection changed.
func (a *App) syncLogFor(id entity.EntityID) {
	if a.logFor != id {
		a.logFor = id
		a.logScroll = 0
	}
}

// drawPawnLog is colonist id's Log tab body, drawn into r, the expanded
// panel's inner rectangle.
func (a *App) drawPawnLog(screen *ebiten.Image, id entity.EntityID, r rect) {
	if p, ok := a.pawnView(id); ok {
		a.drawPawnLogRows(screen, p, r)
	}
}

// drawPawnLogRows draws p's log, newest first, in the text area r.
func (a *App) drawPawnLogRows(screen *ebiten.Image, p game.PawnView, r rect) {
	a.syncLogFor(p.ID)
	x, y, w, h := logBodyRect(r)
	rows := wrapLogRows(p.Log, float64(w)-12)
	a.logScroll = drawLogRows(screen, rows, x, y, w, h/logLineH(), a.logScroll)
}
