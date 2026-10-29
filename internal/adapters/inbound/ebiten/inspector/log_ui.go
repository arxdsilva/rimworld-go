package inspector

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Creature logs (LOG-001): the colonist pane's Log tab and the animal
// pane's Log tab (animal_inspector.go) render LogLineView read models,
// newest first.

const logEmptyText = "Nothing has happened yet."

// logStyle is the log's text: the small size, so a panel shows many rows.
var LogStyle = gfx.Small

// logLineH is the px height of one log row.
func logLineH() int { return int(gfx.LineStep(LogStyle)) }

// logLineText is "Day D, HHh  Text".
func LogLineText(l game.LogLineView) string {
	return fmt.Sprintf("Day %d, %02dh  %s", l.Day, l.Hour, l.Text)
}

// wrapLogRows lays lines (newest first) out as rows no wider than maxW px;
// a long line wraps onto indented continuation rows.
func WrapLogRows(lines []game.LogLineView, maxW float64) []string {
	maxW = max(maxW, 60)
	indent := "  "
	iw := float64(gfx.TextW(indent, LogStyle))
	var rows []string
	for _, l := range lines {
		for i, row := range gfx.WrapPx(LogLineText(l), LogStyle, maxW-iw) {
			if i > 0 {
				row = indent + row
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// clampLogScroll keeps a scroll offset (in rows) within the list.
func ClampLogScroll(scroll, rows, visible int) int {
	return max(0, min(scroll, rows-visible))
}

// drawLogRows draws the visible slice of rows from scroll, with a small
// scroll hint at the right edge (hintX) when more are hidden. Returns the
// clamped offset.
func drawLogRows(screen *ebiten.Image, rows []string, x, y, w, visible, scroll int) int {
	lh := logLineH()
	if len(rows) == 0 {
		gfx.Text(screen, logEmptyText, float64(x), float64(y), gfx.WithColor(LogStyle, uikit.TextDim))
		return 0
	}
	scroll = ClampLogScroll(scroll, len(rows), visible)
	for i := 0; i < visible && scroll+i < len(rows); i++ {
		gfx.Text(screen, rows[scroll+i], float64(x), float64(y+i*lh), LogStyle)
	}
	hint := gfx.TextStyle{Size: LogStyle.Size, Color: uikit.Accent, Align: gfx.AlignRight}
	if scroll > 0 {
		gfx.Text(screen, "^", float64(x+w-4), float64(y-lh), hint)
	}
	if scroll+visible < len(rows) {
		gfx.Text(screen, "v", float64(x+w-4), float64(y+visible*lh), hint)
	}
	return scroll
}

// logBodyRect is the pawn Log tab's text area: r, the expanded panel's
// inner rectangle, in whole pixels.
func LogBodyRect(r gfx.Rect) (x, y, w, h int) {
	return int(r.X), int(r.Y), int(r.W), int(r.H)
}

// handleLogWheel scrolls the log under the cursor with the mouse wheel and
// reports whether it used the wheel (so the map does not zoom).
func (a Pawn) HandleLogWheel() bool {
	if a.sim() == nil {
		return false
	}
	wy := a.env().WheelY
	if wy == 0 {
		return false
	}
	mx, my := a.shell.Cursor()
	return a.ScrollLogAt(mx, my, wy)
}

// scrollLogAt scrolls the colonist Log tab under the cursor (the animal's
// Log tab is a widget-kit scroll area: declareAnimalLog).
func (a Pawn) ScrollLogAt(mx, my int, wy float64) bool {
	return a.PaneOpenTab() == "inspector.log" && a.inspectorVisible() && a.ScrollLogIn(a.ExpandedRect().Inset(PaneInset), mx, my, wy)
}

// scrollLogIn scrolls the selected colonist's Log tab, whose expanded
// panel's inner rectangle is r, when the cursor (mx, my) is over its text
// area; wy is the wheel. It reports it used the wheel.
func (a Pawn) ScrollLogIn(r gfx.Rect, mx, my int, wy float64) bool {
	p, ok := a.SelectedPawn()
	if !ok || wy == 0 {
		return false
	}
	step := 3
	if wy > 0 {
		step = -3
	}
	x, y, w, h := LogBodyRect(r)
	if mx < x || mx >= x+w || my < y || my >= y+h {
		return false
	}
	a.LogScroll = max(0, a.LogScroll+step)
	a.SyncLogFor(p.ID)
	return true
}

func (a Pawn) inspectorVisible() bool {
	_, ok := a.shell.Selected()
	return ok && !a.env().WinOpen && !a.env().ArchOpen
}

// syncLogFor resets the colonist log scroll when the selection changed.
func (a Pawn) SyncLogFor(id entity.EntityID) {
	if a.LogFor != id {
		a.LogFor = id
		a.LogScroll = 0
	}
}

// drawPawnLog is colonist id's Log tab body, drawn into r, the expanded
// panel's inner rectangle.
func (a Pawn) drawPawnLog(screen *ebiten.Image, id entity.EntityID, r gfx.Rect) {
	if p, ok := a.pawnView(id); ok {
		a.DrawPawnLogRows(screen, p, r)
	}
}

// drawPawnLogRows draws p's log, newest first, in the text area r.
func (a Pawn) DrawPawnLogRows(screen *ebiten.Image, p game.PawnView, r gfx.Rect) {
	a.SyncLogFor(p.ID)
	x, y, w, h := LogBodyRect(r)
	rows := WrapLogRows(p.Log, float64(w)-12)
	a.LogScroll = drawLogRows(screen, rows, x, y, w, h/logLineH(), a.LogScroll)
}
