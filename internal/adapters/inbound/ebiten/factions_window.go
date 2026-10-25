package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
)

// The Factions window in the colony (NG-1 §2.8): the setup dialog's table
// over the live read model Simulation.Factions, a goodwill sparkline over
// the last 60 days and the last five events that moved goodwill, for the
// faction picked in the table. Opened from the HUD bottom bar's Factions
// tab; read-only.

const (
	colFacModalID = "colony.factions"
	colFacW       = 1120
	colFacH       = 640
	colFacListMax = 290 // the table's height cap; more rows scroll
	colFacChartH  = 128
	colFacAxisW   = 40
)

// colFacRow is one table row and the faction's detail.
type colFacRow struct {
	facRow
	History []int
	Events  []string
}

// relationOfView maps the read model's relation word to the domain value.
func relationOfView(s string) faction.Relation {
	switch s {
	case "hostile":
		return faction.Hostile
	case "friendly":
		return faction.Friendly
	}
	return faction.Neutral
}

// accentOfView parses a "#rrggbb" accent; a bad value gives the neutral
// text colour.
func accentOfView(hex string) color.NRGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 6 {
		if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
		}
	}
	return color.NRGBA{R: uiTextDim.R, G: uiTextDim.G, B: uiTextDim.B, A: 0xFF}
}

// colonyFactionRows turns the read model into table rows: the visible
// factions in world order, then the always-present Swarm, as in the setup
// dialog. Hidden factions (the colony itself) are left out.
func colonyFactionRows(views []game.FactionView) []colFacRow {
	rows := make([]colFacRow, 0, len(views)+1)
	for _, v := range views {
		if v.Hidden {
			continue
		}
		r := facRow{
			ID: v.ID, Name: v.Name, TypeLabel: v.Type, Goodwill: v.Goodwill,
			Relation: relationOfView(v.Relation), Settlements: v.Settlements, Tech: v.Tech,
			Emblem: v.Emblem, Accent: accentOfView(v.Accent), Capital: -1,
		}
		if v.Leader != "" {
			r.Leader = strings.TrimSpace(v.LeaderTitle + " " + v.Leader)
		}
		hist := make([]int, len(v.History))
		for i, h := range v.History {
			hist[i] = int(h)
		}
		rows = append(rows, colFacRow{facRow: r, History: hist, Events: v.Events})
	}
	for _, s := range factionRows(nil) { // the Swarm
		rows = append(rows, colFacRow{facRow: s})
	}
	return rows
}

// factionsTabUI makes the HUD's Factions tab open the window: the legacy
// button is drawn by the HUD, this widget answers its click.
func (a *App) factionsTabUI(sw, sh int) {
	for _, b := range a.hudTabButtons(sw, sh) {
		if b.id != "tab.factions" || b.disabled {
			continue
		}
		a.uiSt().add(uiWidget{id: b.id, label: b.label, r: rect{b.x, b.y, b.w, b.h}, click: a.openColonyFactions})
	}
}

// openColonyFactions opens the window on the first faction.
func (a *App) openColonyFactions() {
	if a.renderer == nil {
		return
	}
	sel := 0
	a.uiPushModal(modal{
		id: colFacModalID, title: "Factions", w: colFacW, h: colFacH,
		draw:    func(a *App, body rect) { a.colonyFactionsBody(body, &sel) },
		buttons: []modalButton{{id: "close", label: "Close", style: btnPrimary, closes: true}},
	})
}

// colonyFactionsBody declares the table over the live rows and the
// selected faction's detail under it.
func (a *App) colonyFactionsBody(body rect, sel *int) {
	if a.renderer == nil {
		return
	}
	rows := colonyFactionRows(a.renderer.game.Factions())
	plain := make([]facRow, len(rows))
	for i, r := range rows {
		plain[i] = r.facRow
	}
	*sel = max(0, min(*sel, len(rows)-1))
	listH := min(float32(tableHeaderH+len(rows)*facRowH+4), colFacListMax)
	tb, rest := body.splitV(listH)
	a.factionsTable(tb, plain, sel, nil)
	a.colonyFactionDetail(rect{rest.X, rest.Y + 10, rest.W, rest.H - 10}, rows[*sel])
}

// colonyFactionDetail is the picked faction's goodwill history and events.
func (a *App) colonyFactionDetail(r rect, row colFacRow) {
	u := a.uiSt()
	left, right := r.splitH(300)
	chartR, evR := right.splitH(right.W * 0.56)
	left.W -= 12

	hs := tsLarge
	hs.Color = uiHeading
	y := left.Y
	a.uiLabel(left.X, y, fitText(row.Name, hs, left.W), hs)
	y += float32(lineH(hs)) + 2
	a.uiLabel(left.X, y, fitText(row.TypeLabel+", "+row.Tech, dimText(), left.W), dimText())
	y += 20
	rs := tsLabel
	rs.Color = relationColor(row.Relation)
	a.uiLabel(left.X, y, fmt.Sprintf("%s  (goodwill %s)", relationLabel(row.Relation), goodwillText(row.Goodwill)), rs)
	y += 22
	ds := tsSmall
	ds.Color = uiTextDim
	a.textLines(left.X, y, left.W, goodwillTip(row.facRow), ds)

	// The sparkline.
	capR := rect{chartR.X + 12, chartR.Y, chartR.W - 12, 18}
	a.uiLabel(capR.X, capR.Y, "Goodwill, last 60 days", tsLabel)
	plot := rect{capR.X + colFacAxisW, capR.Y + 24, capR.W - colFacAxisW - 4, colFacChartH}
	hist, relc := row.History, goodwillLineColor(row.Relation)
	u.queue(func(dst *ebiten.Image) { drawGoodwillChart(dst, plot, hist, relc) })
	a.uiTip("colfac.chart", plot, "Goodwill history\nOne point a day, from -100 (hostile) to +100 (friendly). Raids defeated and gifts move it; it drifts back toward the faction's usual mood.")

	// The events.
	ex := evR.X + 12
	a.uiLabel(ex, evR.Y, "Recent events", tsLabel)
	ey := evR.Y + 24
	if len(row.Events) == 0 {
		a.uiLabel(ex, ey, "Nothing has moved their goodwill yet.", dimText())
		return
	}
	for i := len(row.Events) - 1; i >= 0; i-- { // newest first
		ey += a.textLines(ex, ey, evR.W-24, "- "+row.Events[i], tsBody) + 3
	}
}

// goodwillLineColor is the colour of a faction's history line.
func goodwillLineColor(r faction.Relation) color.RGBA {
	if r == faction.Neutral {
		return uiAccent
	}
	return relationColor(r)
}

// chartPoints maps a goodwill history (oldest first, at most 60 days) onto
// plot: the last value sits on the right edge, a shorter history leaves the
// left empty, and -100..+100 spans the plot's height.
func chartPoints(hist []int, plot rect) [][2]float32 {
	n := len(hist)
	if n > faction.HistoryDays {
		hist = hist[n-faction.HistoryDays:]
		n = len(hist)
	}
	pts := make([][2]float32, n)
	for i, v := range hist {
		v = max(-100, min(100, v))
		slot := faction.HistoryDays - n + i
		x := plot.X + plot.W*float32(slot)/float32(faction.HistoryDays-1)
		y := plot.bottom() - plot.H*float32(v+100)/200
		pts[i] = [2]float32{x, y}
	}
	return pts
}

// drawGoodwillChart draws the history line over a -100..+100 scale with the
// zero line and the hostile / friendly bands marked.
func drawGoodwillChart(dst *ebiten.Image, plot rect, hist []int, line color.RGBA) {
	fillRect(dst, plot, uiField)
	strokeRect(dst, plot, uiPanelBorder)
	ls := tsSmall
	ls.Color, ls.Align = uiTextDim, alignRight
	for _, g := range []struct {
		v int
		c color.NRGBA
	}{{100, chartGrid}, {50, chartGrid}, {0, chartZero}, {-50, chartGrid}, {-100, chartGrid}} {
		y := plot.bottom() - plot.H*float32(g.v+100)/200
		if g.v != 100 && g.v != -100 {
			uiLine(dst, plot.X, y, plot.right(), y, 1, g.c, false)
		}
		drawText(dst, goodwillAxis(g.v), float64(plot.X-6), float64(y)-lineH(ls)/2, ls)
	}
	if len(hist) == 0 {
		cs := dimText()
		cs.Align = alignCenter
		drawText(dst, "No history yet.", float64(plot.X+plot.W/2), float64(plot.Y+plot.H/2)-lineH(cs)/2, cs)
		return
	}
	pts := chartPoints(hist, plot)
	for i := 1; i < len(pts); i++ {
		uiLine(dst, pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], 1.5, line, true)
	}
	last := pts[len(pts)-1]
	uiFillCircle(dst, last[0], last[1], 3, line, true)
	es := tsSmall
	es.Color = uiTextDim
	drawText(dst, "60 days ago", float64(plot.X), float64(plot.bottom())+3, es)
	es.Align = alignRight
	drawText(dst, "today", float64(plot.right()), float64(plot.bottom())+3, es)
}

// goodwillAxis labels an axis value: "+50", "0", "-100".
func goodwillAxis(v int) string {
	if v == 0 {
		return "0"
	}
	return goodwillText(v)
}

// Faint gridlines of the chart (the zero line is a little brighter).
var (
	chartGrid = color.NRGBA{R: uiPanelBorder.R, G: uiPanelBorder.G, B: uiPanelBorder.B, A: 0x80}
	chartZero = color.NRGBA{R: uiTextDim.R, G: uiTextDim.G, B: uiTextDim.B, A: 0xB0}
)
