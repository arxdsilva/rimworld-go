package windows

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
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
	colFacW       = 1010 // UIS-068
	colFacChartH  = 128
	colFacAxisW   = 40
	ColFacListID  = "colfac.list"
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
		ID: ColFacModalID, Title: w.sh.T("Factions"), W: colFacW, H: w.height(), Anchor: w.Anchor, Frameless: true,
		Draw: func(body gfx.Rect) { w.body(body, &sel) },
	})
}

// height is the window's outer height (UIS-068): the header, one 80 px row
// per faction and the foot. The layout caps it to the room above the tab
// bar; the rows then scroll.
func (w Factions) height() float32 {
	n := 0
	if w.live() {
		n = len(w.sh.Factions())
	}
	return float32(hudlayout.FactionHeadH + n*hudlayout.FactionRowH + hudlayout.FactionFootH)
}

// size is the window's outer size.
func (w Factions) size() (float32, float32) { return colFacW, w.height() }

// factionRegions splits the body: the header (the picked faction's chart),
// the scrolling rows and the foot. FactionHeadH and FactionFootH are outer
// heights, so the body's WindowPad is taken off each.
func factionRegions(body gfx.Rect) (head, rows, foot gfx.Rect) {
	head = gfx.Rect{X: body.X, Y: body.Y, W: body.W, H: hudlayout.FactionHeadH - hudlayout.WindowPad}
	foot = gfx.Rect{X: body.X, Y: body.Bottom() - (hudlayout.FactionFootH - hudlayout.WindowPad), W: body.W, H: hudlayout.FactionFootH - hudlayout.WindowPad}
	rows = gfx.Rect{X: body.X, Y: head.Bottom(), W: body.W - (hudlayout.FactionGutter - uikit.ScrollBarW), H: max(0, foot.Y-head.Bottom())}
	return head, rows, foot
}

// body declares the picked faction's header, the scrolling rows over the
// live factions and the foot hint.
func (w Factions) body(body gfx.Rect, sel *int) {
	if !w.live() {
		return
	}
	rows := w.sh.Factions()
	if len(rows) == 0 {
		return
	}
	*sel = max(0, min(*sel, len(rows)-1))
	head, list, foot := factionRegions(body)
	w.header(head, rows[*sel])
	w.kit().List(ColFacListID, list, len(rows), sel, uikit.ListSpec{
		RowH: hudlayout.FactionRowH,
		Draw: func(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool) {
			w.drawRow(dst, rows[i], row, i, hover, selected)
		},
		Tip: func(i int) string { return rows[i].Tip },
	})
	hs := uikit.DimText()
	w.kit().Label(foot.X, float32(gfx.CenterTop(float64(foot.Y), float64(foot.H), hs)), uikit.FitText(w.sh.T("Click a faction to see how its goodwill moved."), hs, foot.W), hs)
}

// header is the picked faction's name, standing, goodwill chart and the
// events that moved it (the old 190 px detail pane, folded into 150).
func (w Factions) header(r gfx.Rect, row Faction) {
	u := w.kit()
	left, rest := r.SplitH(300)
	chartR, evR := rest.SplitH(rest.W * 0.56)
	left.W -= 12

	hs := gfx.Title
	hs.Color = uikit.Heading
	y := left.Y
	u.Label(left.X, y, uikit.FitText(row.Name, hs, left.W), hs)
	y += float32(gfx.LineH(hs)) + 2
	u.Label(left.X, y, uikit.FitText(row.TypeLabel+", "+row.Tech, uikit.DimText(), left.W), uikit.DimText())
	y += 20
	rs := gfx.Label
	rs.Color = row.Color
	u.Label(left.X, y, uikit.FitText(lang.Tf(w.sh, "%s  (goodwill %s)", row.Standing, goodwillText(row.Goodwill)), rs, left.W), rs)
	y += 22
	ds := gfx.Small
	ds.Color = uikit.TextDim
	for i, l := range gfx.WrapPx(row.Tip, ds, float64(left.W)) {
		if y+float32(gfx.LineH(ds))*float32(i+1) > r.Bottom() {
			break
		}
		u.Label(left.X, y+float32(gfx.LineH(ds))*float32(i), l, ds)
	}

	capR := gfx.Rect{X: chartR.X + 12, Y: chartR.Y, W: chartR.W - 12, H: 18}
	u.Label(capR.X, capR.Y, w.sh.T("Goodwill, last 60 days"), gfx.Label)
	ls := gfx.Small
	plot := gfx.Rect{X: capR.X + colFacAxisW, Y: capR.Y + 22, W: capR.W - colFacAxisW - 4, H: r.Bottom() - capR.Y - 22 - float32(gfx.LineH(ls)) - 6}
	hist, relc := row.History, goodwillLineColor(row)
	u.Queue(func(dst *ebiten.Image) { drawGoodwillChart(w.sh, dst, plot, hist, relc) })
	u.Tip("colfac.chart", plot, w.sh.T("Goodwill history\nOne point a day, from -100 (hostile) to +100 (friendly). Raids defeated and gifts move it; it drifts back toward the faction's usual mood."))

	ex := evR.X + 12
	u.Label(ex, evR.Y, w.sh.T("Recent events"), gfx.Label)
	ey := evR.Y + 22
	if len(row.Events) == 0 {
		u.Label(ex, ey, uikit.FitText(w.sh.T("Nothing has moved their goodwill yet."), uikit.DimText(), evR.W-12), uikit.DimText())
		return
	}
	es := gfx.Small
	for i := len(row.Events) - 1; i >= 0; i-- { // newest first
		for _, l := range gfx.WrapPx("- "+row.Events[i], es, float64(evR.W-24)) {
			if ey+float32(gfx.LineH(es)) > r.Bottom() {
				return
			}
			u.Label(ex, ey, l, es)
			ey += float32(gfx.LineH(es))
		}
	}
}

// Faction row columns, right to left (offsets from the row's right edge).
const (
	facColSettleW = 110
	facColGoodW   = 110
	facColRelW    = 150
	facIconSize   = 48
)

// drawRow draws one 80 px faction row: striped, an emblem disc, name, type,
// leader, then relation, goodwill and settlements.
func (w Factions) drawRow(dst *ebiten.Image, f Faction, row gfx.Rect, i int, hover, selected bool) {
	switch {
	case selected:
		uikit.FillRect(dst, row, uikit.BtnSelected)
	case hover:
		uikit.FillRect(dst, row, uikit.Btn)
	default:
		uikit.RowStripe(dst, row, i)
	}
	pad := float32(10)
	cy := row.Y + row.H/2
	gfx.FillCircle(dst, row.X+pad+facIconSize/2, cy, facIconSize/2, f.Color, true)
	is := gfx.Title
	is.Color, is.Align = uikit.Field, gfx.AlignCenter
	initial := facInitial(f.Name)
	gfx.Text(dst, initial, float64(row.X+pad+facIconSize/2), gfx.CenterTop(float64(cy)-facIconSize/2, facIconSize, is), is)

	right := row.Right() - pad
	settle := gfx.Rect{X: right - facColSettleW, Y: row.Y, W: facColSettleW, H: row.H}
	good := gfx.Rect{X: settle.X - facColGoodW, Y: row.Y, W: facColGoodW, H: row.H}
	rel := gfx.Rect{X: good.X - facColRelW, Y: row.Y, W: facColRelW, H: row.H}
	tx := row.X + pad + facIconSize + 14
	tw := rel.X - tx - 8

	ns := gfx.Title
	ns.Color = uikit.Heading
	ds := uikit.DimText()
	lines := []struct {
		s  string
		st gfx.TextStyle
	}{
		{f.Name, ns},
		{f.TypeLabel + ", " + f.Tech, ds},
	}
	if f.Leader != "" {
		lines = append(lines, struct {
			s  string
			st gfx.TextStyle
		}{f.Leader, ds})
	}
	total := 0.0
	for _, l := range lines {
		total += gfx.LineH(l.st)
	}
	ty := float64(cy) - total/2
	for _, l := range lines {
		h := gfx.LineH(l.st)
		gfx.Text(dst, uikit.FitText(l.s, l.st, tw), float64(tx), gfx.CenterTop(ty, h, l.st), l.st)
		ty += h
	}

	cell := func(c gfx.Rect, s string, st gfx.TextStyle) {
		gfx.Text(dst, uikit.FitText(s, st, c.W-8), float64(c.X), gfx.CenterTop(float64(c.Y), float64(c.H), st), st)
	}
	rs := gfx.Label
	rs.Color = f.Color
	cell(rel, f.Standing, rs)
	cell(good, goodwillText(f.Goodwill), gfx.Label)
	cell(settle, fmt.Sprint(f.Settlements), gfx.Label)
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
	drawGoodwillChart(lang.English{}, dst, plot, hist, line)
}

func drawGoodwillChart(tr lang.Tr, dst *ebiten.Image, plot gfx.Rect, hist []int, line color.RGBA) {
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
		gfx.Text(dst, goodwillAxis(g.v), float64(plot.X-6), gfx.CenterTop(float64(y)-gfx.LineH(ls)/2, gfx.LineH(ls), ls), ls)
	}
	if len(hist) == 0 {
		cs := uikit.DimText()
		cs.Align = gfx.AlignCenter
		gfx.Text(dst, tr.T("No history yet."), float64(plot.X+plot.W/2), gfx.CenterTop(float64(plot.Y+plot.H/2)-gfx.LineH(cs)/2, gfx.LineH(cs), cs), cs)
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
	gfx.Text(dst, tr.T("60 days ago"), float64(plot.X), float64(plot.Bottom())+3, es)
	es.Align = gfx.AlignRight
	gfx.Text(dst, tr.T("today"), float64(plot.Right()), float64(plot.Bottom())+3, es)
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

// facInitial is the letter on a faction's disc: the name's first letter,
// upper case, after a leading article ("the Lone Drift" shows "L").
func facInitial(name string) string {
	name = strings.TrimSpace(name)
	if rest, ok := strings.CutPrefix(strings.ToLower(name), "the "); ok && rest != "" {
		name = name[len(name)-len(rest):]
	}
	for _, r := range name {
		return string(unicode.ToUpper(r))
	}
	return "?"
}
