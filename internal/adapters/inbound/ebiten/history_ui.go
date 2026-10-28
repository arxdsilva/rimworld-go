package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// The History window (MAIN_UI_SCREENS §28-§32, ENG-003): the colony's
// notable events from Simulation.History, newest first. Opened from the
// HUD's History tab; read-only. Clicking an entry that names colonists
// selects the first one.

const (
	historyModalID = "colony.history"
	historyRowH    = 22
	historyWhenW   = 120
	historyEmpty   = "Nothing noteworthy has happened yet."
)

// historyRow is one displayed history line.
type historyRow struct {
	When, Text, Kind string
	Pawns            []uint64
}

// historyDate is a tick as "Y1 Q2 D5 14:00" (1-based year, quadrum, day).
func historyDate(tick uint64) string {
	d := clock.DateAt(clock.Tick(tick))
	return fmt.Sprintf("Y%d Q%d D%d %02d:00", d.Year+1, d.Quadrum+1, d.Day+1, d.Hour)
}

// historyRows lists the entries newest first with their date text.
func historyRows(es []game.HistoryEntryView) []historyRow {
	out := make([]historyRow, 0, len(es))
	for i := len(es) - 1; i >= 0; i-- {
		e := es[i]
		out = append(out, historyRow{When: historyDate(e.Tick), Text: e.Text, Kind: e.Kind, Pawns: e.Pawns})
	}
	return out
}

// historyTabUI makes the HUD's History tab open the window.
func (a *App) historyTabUI(sw, sh int) {
	for _, b := range a.hudTabButtons(sw, sh) {
		if b.id != ids.TabHistory || b.disabled {
			continue
		}
		a.uiSt().add(uiWidget{id: b.id, label: b.label, r: rect{X: b.x, Y: b.y, W: b.w, H: b.h}, click: a.toggleHistory})
	}
}

// toggleHistory opens the window, or closes it when it is the open one.
func (a *App) toggleHistory() { a.toggleKitWindow(historyModalID, a.openHistory) }

// openHistory opens the History window: a main-tab window in the Window
// rect (hud_window.go).
func (a *App) openHistory() {
	if a.renderer == nil {
		return
	}
	a.uiPushModal(modal{
		id: historyModalID, title: "History", w: 640, h: 520,
		modeless: true, anchor: (*App).windowAnchor,
		draw: func(a *App, body rect) { a.historyBody(body) },
	})
}

// historyBody declares the scrolling list of entries.
func (a *App) historyBody(body rect) {
	if a.renderer == nil {
		return
	}
	rows := historyRows(a.renderer.game.History())
	if len(rows) == 0 {
		a.uiLabel(body.X, body.Y, historyEmpty, dimText())
		return
	}
	area := a.uiScrollArea("history.list", body, float32(len(rows))*historyRowH, scrollOpts{Step: historyRowH * 2, Focusable: true})
	type vis struct {
		i int
		y float32
	}
	var shown []vis
	for i := range rows {
		y := area.View.Y + float32(i)*historyRowH - area.Off
		if y+historyRowH < area.View.Y || y > area.View.Bottom() {
			continue
		}
		shown = append(shown, vis{i, y})
	}
	a.uiSt().queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for _, v := range shown {
			r := rows[v.i]
			row := rect{X: area.View.X, Y: v.y, W: area.View.W, H: historyRowH}
			if v.i%2 == 1 {
				fillRect(sub, row, uiStripe)
			}
			dim := tsSmall
			dim.Color = uiTextDim
			st := tsBody
			if len(r.Pawns) > 0 {
				st.Color = uiHeading
			}
			ty := float64(row.Y) + (historyRowH-lineH(st))/2
			drawText(sub, r.When, float64(row.X+6), float64(row.Y)+(historyRowH-lineH(dim))/2, dim)
			drawText(sub, fitText(r.Text, st, row.W-historyWhenW-10), float64(row.X+historyWhenW), ty, st)
		}
	})
	for _, v := range shown {
		r := rows[v.i]
		w := uiWidget{id: fmt.Sprintf("history.row.%d", v.i), label: r.Text,
			r: rect{X: area.View.X, Y: v.y, W: area.View.W, H: historyRowH}.Intersect(area.View)}
		if len(r.Pawns) == 0 {
			a.uiSt().add(w)
			continue
		}
		first := r.Pawns[0]
		w.click = func() { a.focusHistoryPawn(first) }
		if a.uiSt().add(w) && a.uiPress(w.r) {
			a.uiOK()
			a.focusHistoryPawn(first)
		}
	}
}

// focusHistoryPawn selects and centres the colonist an entry names, when
// they are still in the colony.
func (a *App) focusHistoryPawn(id uint64) {
	if a.renderer == nil {
		return
	}
	for _, p := range a.shell.Pawns() {
		if uint64(p.ID) != id {
			continue
		}
		a.shell.Select(p.ID)
		a.centerOnMap(float64(p.X)+0.5, float64(p.Y)+0.5)
		a.uiCloseTop()
		return
	}
}
