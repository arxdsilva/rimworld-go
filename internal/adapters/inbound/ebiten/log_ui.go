package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Creature logs (LOG-001): the colonist inspector's Log tab and the
// animal inspector's Recent / Log section render LogLineView read models,
// newest first.

const (
	logEmptyText  = "Nothing has happened yet."
	logRecentRows = 4 // animal inspector's Recent list
)

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

// logPanelRect is the pawn Log tab's text area inside the inspector.
func logPanelRect() (x, y, w, h int) {
	r := inspectorRect()
	return int(r.x) + 12, int(r.y) + insBodyY, int(r.w) - 24, int(r.h) - insBodyY - 70
}

// animalLogRect is the animal inspector's expanded log area.
func animalLogRect(r button, extra int) (x, y, w, h int) {
	return int(r.x) + 12, int(r.y) + 106 + extra, int(r.w) - 24, int(r.h) - 106 - extra - 44
}

// handleLogWheel scrolls the log under the cursor with the mouse wheel and
// reports whether it used the wheel (so the map does not zoom).
func (a *App) handleLogWheel() bool {
	if a.renderer == nil {
		return false
	}
	wy := a.in.raw.wheelY
	if wy == 0 {
		return false
	}
	mx, my := a.currentCursor()
	return a.scrollLogAt(mx, my, wy)
}

func (a *App) scrollLogAt(mx, my int, wy float64) bool {
	step := 0
	if wy > 0 {
		step = -3
	} else if wy < 0 {
		step = 3
	}
	if p, ok := a.selectedPawn(); ok && a.inspectTab == "log" && a.inspectorVisible() {
		x, y, w, h := logPanelRect()
		if mx >= x && mx < x+w && my >= y && my < y+h {
			a.logScroll = max(0, a.logScroll+step)
			a.syncLogFor(p.ID)
			return true
		}
	}
	if an, ok := a.selectedAnimal(); ok && a.animalLogOpen && !a.workOpen && !a.archOpen {
		r := a.animalPanel()
		x, y, w, h := animalLogRect(r, animalExtraH(an)+animalBondH(an))
		if mx >= x && mx < x+w && my >= y && my < y+h {
			a.animalLogScroll = max(0, a.animalLogScroll+step)
			return true
		}
	}
	return false
}

func (a *App) inspectorVisible() bool {
	return a.renderer != nil && a.renderer.HasSelect() && !a.workOpen && !a.archOpen
}

// syncLogFor resets the colonist log scroll when the selection changed.
func (a *App) syncLogFor(id entity.EntityID) {
	if a.logFor != id {
		a.logFor = id
		a.logScroll = 0
	}
}

// drawPawnLog is the Log tab body.
func (a *App) drawPawnLog(screen *ebiten.Image, p game.PawnView) {
	a.syncLogFor(p.ID)
	x, y, w, h := logPanelRect()
	rows := wrapLogRows(p.Log, float64(w)-12)
	a.logScroll = drawLogRows(screen, rows, x, y, w, h/logLineH(), a.logScroll)
}

// recentLogRows is the animal inspector's Recent list: the newest
// logRecentRows lines, each clipped to one row of at most maxW px.
func recentLogRows(log []game.LogLineView, maxW float64) []string {
	if len(log) > logRecentRows {
		log = log[:logRecentRows]
	}
	rows := make([]string, 0, len(log))
	for _, l := range log {
		rows = append(rows, ellipsize(logLineText(l), logStyle, maxW))
	}
	return rows
}

// syncAnimalLog resets the animal log view when another animal is selected.
func (a *App) syncAnimalLog(id entity.EntityID) {
	if a.animalLogFor != id {
		a.animalLogFor, a.animalLogScroll, a.animalLogOpen = id, 0, false
	}
}

// drawAnimalLog draws the animal inspector's Recent list, or the full
// scrollable log when expanded.
func (a *App) drawAnimalLog(screen *ebiten.Image, an game.AnimalView, r button) {
	x, y, w, h := animalLogRect(r, animalExtraH(an)+animalBondH(an))
	if a.animalLogOpen {
		rows := wrapLogRows(an.Log, float64(w)-12)
		a.animalLogScroll = drawLogRows(screen, rows, x, y, w, h/logLineH(), a.animalLogScroll)
		return
	}
	drawText(screen, "Recent:", float64(x), float64(y)-2, withColor(tsSmall, uiTextDim))
	rows := recentLogRows(an.Log, float64(w)-12)
	drawLogRows(screen, rows, x, y+logLineH(), w, logRecentRows, 0)
}
