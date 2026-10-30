package windows

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// The History window (MAIN_UI_SCREENS §28-§32, ENG-003): the colony's
// notable events from Simulation.History, newest first. Opened from the
// HUD's History tab; read-only. Clicking an entry that names colonists
// selects the first one.

const (
	HistoryModalID = "colony.history"
	historyRowH    = 22
	historyTabH    = 28
	historyWhenW   = 120
	historyEmpty   = "Nothing noteworthy has happened yet."
)

// rowStripe shades every other row (the Bio tab's stripe).
var rowStripe = color.NRGBA{R: uikit.PanelBorder.R, G: uikit.PanelBorder.G, B: uikit.PanelBorder.B, A: 0x28}

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

// HistoryRows lists the entries newest first with their date text.
func HistoryRows(es []game.HistoryEntryView) []historyRow {
	out := make([]historyRow, 0, len(es))
	for i := len(es) - 1; i >= 0; i-- {
		e := es[i]
		out = append(out, historyRow{When: historyDate(e.Tick), Text: e.Text, Kind: e.Kind, Pawns: e.Pawns})
	}
	return out
}

// Toggle opens the window, or closes it when it is the open one.
func (w History) Toggle() { w.toggleKit(HistoryModalID, w.Launch) }

// Launch opens the History window: a main-tab window in the Window
// rect (window.go).
func (w History) Launch() {
	if !w.live() {
		return
	}
	w.OpenKit(uikit.Modal{
		ID: HistoryModalID, Title: "History", W: 640, H: 520, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.body(body) },
	})
}

// body declares the scrolling list of entries.
func (w History) body(body gfx.Rect) {
	if !w.live() {
		return
	}
	strip := gfx.Rect{X: body.X, Y: body.Y, W: body.W, H: historyTabH}
	w.kit().Tabs("history.tabs", strip, []string{"Events", "Combat"}, &w.histTab, uikit.TabsOpts{})
	body = gfx.Rect{X: body.X, Y: body.Y + historyTabH + 6, W: body.W, H: body.H - historyTabH - 6}
	if w.histTab == 1 {
		w.combatList(body)
		return
	}
	rows := HistoryRows(w.sh.Env().Game.History())
	if len(rows) == 0 {
		w.kit().Label(body.X, body.Y, historyEmpty, uikit.DimText())
		return
	}
	area := w.kit().ScrollArea("history.list", body, float32(len(rows))*historyRowH, uikit.ScrollOpts{Step: historyRowH * 2, Focusable: true})
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
	w.kit().Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for _, v := range shown {
			r := rows[v.i]
			row := gfx.Rect{X: area.View.X, Y: v.y, W: area.View.W, H: historyRowH}
			if v.i%2 == 1 {
				uikit.FillRect(sub, row, rowStripe)
			}
			dim := gfx.Small
			dim.Color = uikit.TextDim
			st := gfx.Body
			if len(r.Pawns) > 0 {
				st.Color = uikit.Heading
			}
			ty := float64(row.Y) + (historyRowH-gfx.LineH(st))/2
			gfx.Text(sub, r.When, float64(row.X+6), float64(row.Y)+(historyRowH-gfx.LineH(dim))/2, dim)
			gfx.Text(sub, uikit.FitText(r.Text, st, row.W-historyWhenW-10), float64(row.X+historyWhenW), ty, st)
		}
	})
	for _, v := range shown {
		r := rows[v.i]
		wd := uikit.Widget{ID: fmt.Sprintf("history.row.%d", v.i), Label: r.Text,
			R: gfx.Rect{X: area.View.X, Y: v.y, W: area.View.W, H: historyRowH}.Intersect(area.View)}
		if len(r.Pawns) == 0 {
			w.kit().Add(wd)
			continue
		}
		first := r.Pawns[0]
		wd.Click = func() { w.focusPawn(first) }
		if w.kit().Add(wd) && w.kit().Press(wd.R) {
			w.kit().OK()
			w.focusPawn(first)
		}
	}
}

// focusPawn selects and centres the colonist an entry names, when
// they are still in the colony.
func (w History) focusPawn(id uint64) {
	if !w.live() {
		return
	}
	for _, p := range w.sh.Pawns() {
		if uint64(p.ID) != id {
			continue
		}
		w.sh.Select(p.ID)
		w.sh.CenterOnMap(float64(p.X)+0.5, float64(p.Y)+0.5)
		if !w.kit().CloseTop() { // the top modal, else this window
			w.CloseKit()
		}
		return
	}
}
