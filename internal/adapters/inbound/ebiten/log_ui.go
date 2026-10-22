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
	logLineH      = 14 // px per text row
	logCharW      = 7  // debug font advance (ART: 7 px/char)
	logEmptyText  = "Nothing has happened yet."
	logRecentRows = 4 // animal inspector's Recent list
)

// logLineText is "Day D, HHh  Text".
func logLineText(l game.LogLineView) string {
	return fmt.Sprintf("Day %d, %02dh  %s", l.Day, l.Hour, l.Text)
}

// wrapLogRows lays lines (newest first) out as rows of at most cols
// characters; a long line wraps onto indented continuation rows.
func wrapLogRows(lines []game.LogLineView, cols int) []string {
	if cols < 8 {
		cols = 8
	}
	var rows []string
	for _, l := range lines {
		s := []rune(logLineText(l))
		first := true
		for len(s) > 0 {
			w := cols
			if !first {
				w = cols - 2
			}
			n := min(len(s), w)
			row := string(s[:n])
			if !first {
				row = "  " + row
			}
			rows = append(rows, row)
			s = s[n:]
			first = false
		}
	}
	return rows
}

// clampLogScroll keeps a scroll offset (in rows) within the list.
func clampLogScroll(scroll, rows, visible int) int {
	return max(0, min(scroll, rows-visible))
}

// drawLogRows draws the visible slice of rows from scroll, with a small
// scroll hint when more are hidden. Returns the clamped offset.
func drawLogRows(screen *ebiten.Image, rows []string, x, y, visible, scroll int) int {
	if len(rows) == 0 {
		drawLabel(screen, x, y, logEmptyText)
		return 0
	}
	scroll = clampLogScroll(scroll, len(rows), visible)
	for i := 0; i < visible && scroll+i < len(rows); i++ {
		drawLabel(screen, x, y+i*logLineH, rows[scroll+i])
	}
	if scroll > 0 {
		drawLabel(screen, x+logCharW*38, y-logLineH, "^")
	}
	if scroll+visible < len(rows) {
		drawLabel(screen, x+logCharW*38, y+visible*logLineH, "v")
	}
	return scroll
}

// logPanelRect is the pawn Log tab's text area inside the inspector.
func logPanelRect() (x, y, w, h int) {
	r := inspectorRect()
	return int(r.x) + 12, int(r.y) + 126, int(r.w) - 24, int(r.h) - 126 - 70
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
	_, wy := ebiten.Wheel()
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
		x, y, w, h := animalLogRect(r, animalExtraH(an))
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
	rows := wrapLogRows(p.Log, w/logCharW)
	a.logScroll = drawLogRows(screen, rows, x, y, h/logLineH, a.logScroll)
}

// recentLogRows is the animal inspector's Recent list: the newest
// logRecentRows lines, each clipped to one row.
func recentLogRows(log []game.LogLineView, cols int) []string {
	if len(log) > logRecentRows {
		log = log[:logRecentRows]
	}
	rows := make([]string, 0, len(log))
	for _, l := range log {
		t := []rune(logLineText(l))
		if len(t) > cols {
			t = t[:cols]
		}
		rows = append(rows, string(t))
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
	x, y, w, h := animalLogRect(r, animalExtraH(an))
	cols := w / logCharW
	if a.animalLogOpen {
		rows := wrapLogRows(an.Log, cols)
		a.animalLogScroll = drawLogRows(screen, rows, x, y, h/logLineH, a.animalLogScroll)
		return
	}
	drawLabel(screen, x, y-2, "Recent:")
	rows := recentLogRows(an.Log, cols)
	drawLogRows(screen, rows, x, y+logLineH, logRecentRows, 0)
}
