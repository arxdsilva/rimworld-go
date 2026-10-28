package windows

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/faction"
)

// The Factions window in the colony (NG-1 §2.8): the setup dialog's table
// over the live read model Simulation.Factions, a goodwill sparkline over
// the last 60 days and the last five events that moved goodwill, for the
// faction picked in the table. Opened from the HUD bottom bar's Factions
// tab; read-only.

const (
	ColFacModalID = "colony.factions"
	colFacW       = 1120
	colFacH       = 640
	colFacListMax = 290 // the table's height cap; more rows scroll
	colFacChartH  = 128
	colFacAxisW   = 40
)

// Toggle opens the window, or closes it when it is the open one.
func (w Factions) Toggle() { w.toggleKit(ColFacModalID, w.Launch) }

// Launch opens the window on the first faction: a main-tab
// window in the Window rect (window.go).
func (w Factions) Launch() {
	if !w.live() {
		return
	}
	sel := 0
	w.OpenKit(uikit.Modal{
		ID: ColFacModalID, Title: "Factions", W: colFacW, H: colFacH, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.body(body, &sel) },
	})
}

// body declares the table over the live rows and the
// selected faction's detail under it.
func (w Factions) body(body gfx.Rect, sel *int) {
	if !w.live() {
		return
	}
	rows := w.sh.Factions()
	if len(rows) == 0 {
		return
	}
	*sel = max(0, min(*sel, len(rows)-1))
	rest := w.sh.FactionsTable(body, colFacListMax, sel)
	w.detail(gfx.Rect{X: rest.X, Y: rest.Y + 10, W: rest.W, H: rest.H - 10}, rows[*sel])
}

// detail is the picked faction's goodwill history and events.
func (w Factions) detail(r gfx.Rect, row Faction) {
	u := w.kit()
	left, right := r.SplitH(300)
	chartR, evR := right.SplitH(right.W * 0.56)
	left.W -= 12

	hs := gfx.Large
	hs.Color = uikit.Heading
	y := left.Y
	w.kit().Label(left.X, y, uikit.FitText(row.Name, hs, left.W), hs)
	y += float32(gfx.LineH(hs)) + 2
	w.kit().Label(left.X, y, uikit.FitText(row.TypeLabel+", "+row.Tech, uikit.DimText(), left.W), uikit.DimText())
	y += 20
	rs := gfx.Label
	rs.Color = row.Color
	w.kit().Label(left.X, y, fmt.Sprintf("%s  (goodwill %s)", row.Standing, goodwillText(row.Goodwill)), rs)
	y += 22
	ds := gfx.Small
	ds.Color = uikit.TextDim
	uikit.TextLines(w.kit(), left.X, y, left.W, row.Tip, ds)

	// The sparkline.
	capR := gfx.Rect{X: chartR.X + 12, Y: chartR.Y, W: chartR.W - 12, H: 18}
	w.kit().Label(capR.X, capR.Y, "Goodwill, last 60 days", gfx.Label)
	plot := gfx.Rect{X: capR.X + colFacAxisW, Y: capR.Y + 24, W: capR.W - colFacAxisW - 4, H: colFacChartH}
	hist, relc := row.History, goodwillLineColor(row)
	u.Queue(func(dst *ebiten.Image) { DrawGoodwillChart(dst, plot, hist, relc) })
	w.kit().Tip("colfac.chart", plot, "Goodwill history\nOne point a day, from -100 (hostile) to +100 (friendly). Raids defeated and gifts move it; it drifts back toward the faction's usual mood.")

	// The events.
	ex := evR.X + 12
	w.kit().Label(ex, evR.Y, "Recent events", gfx.Label)
	ey := evR.Y + 24
	if len(row.Events) == 0 {
		w.kit().Label(ex, ey, "Nothing has moved their goodwill yet.", uikit.DimText())
		return
	}
	for i := len(row.Events) - 1; i >= 0; i-- { // newest first
		ey += uikit.TextLines(w.kit(), ex, ey, evR.W-24, "- "+row.Events[i], gfx.Body) + 3
	}
}

// goodwillLineColor is the colour of a faction's history line.
func goodwillLineColor(f Faction) color.RGBA {
	if f.Relation == faction.Neutral {
		return uikit.Accent
	}
	return f.Color
}

// goodwillText is "+12" / "-40".
func goodwillText(g int) string { return fmt.Sprintf("%+d", g) }

// ChartPoints maps a goodwill history (oldest first, at most 60 days) onto
// plot: the last value sits on the right edge, a shorter history leaves the
// left empty, and -100..+100 spans the plot's height.
func ChartPoints(hist []int, plot gfx.Rect) [][2]float32 {
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
		y := plot.Bottom() - plot.H*float32(v+100)/200
		pts[i] = [2]float32{x, y}
	}
	return pts
}

// DrawGoodwillChart draws the history line over a -100..+100 scale with the
// zero line and the hostile / friendly bands marked.
func DrawGoodwillChart(dst *ebiten.Image, plot gfx.Rect, hist []int, line color.RGBA) {
	uikit.FillRect(dst, plot, uikit.Field)
	uikit.StrokeRect(dst, plot, uikit.PanelBorder)
	ls := gfx.Small
	ls.Color, ls.Align = uikit.TextDim, gfx.AlignRight
	for _, g := range []struct {
		v int
		c color.NRGBA
	}{{100, chartGrid}, {50, chartGrid}, {0, chartZero}, {-50, chartGrid}, {-100, chartGrid}} {
		y := plot.Bottom() - plot.H*float32(g.v+100)/200
		if g.v != 100 && g.v != -100 {
			gfx.Line(dst, plot.X, y, plot.Right(), y, 1, g.c, false)
		}
		gfx.Text(dst, goodwillAxis(g.v), float64(plot.X-6), float64(y)-gfx.LineH(ls)/2, ls)
	}
	if len(hist) == 0 {
		cs := uikit.DimText()
		cs.Align = gfx.AlignCenter
		gfx.Text(dst, "No history yet.", float64(plot.X+plot.W/2), float64(plot.Y+plot.H/2)-gfx.LineH(cs)/2, cs)
		return
	}
	pts := ChartPoints(hist, plot)
	for i := 1; i < len(pts); i++ {
		gfx.Line(dst, pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], 1.5, line, true)
	}
	last := pts[len(pts)-1]
	gfx.FillCircle(dst, last[0], last[1], 3, line, true)
	es := gfx.Small
	es.Color = uikit.TextDim
	gfx.Text(dst, "60 days ago", float64(plot.X), float64(plot.Bottom())+3, es)
	es.Align = gfx.AlignRight
	gfx.Text(dst, "today", float64(plot.Right()), float64(plot.Bottom())+3, es)
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
	chartGrid = color.NRGBA{R: uikit.PanelBorder.R, G: uikit.PanelBorder.G, B: uikit.PanelBorder.B, A: 0x80}
	chartZero = color.NRGBA{R: uikit.TextDim.R, G: uikit.TextDim.G, B: uikit.TextDim.B, A: 0xB0}
)
