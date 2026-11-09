package windows

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/wealth"
	"rimworld-go/internal/domain/sim/clock"
)

// The History window's Wealth tab (EAI-0, game.WealthHistory): the colony's
// wealth over the days the game kept samples, which the threat points come
// from. Read only.

// WealthReader is the optional reader of the wealth samples, oldest first;
// the game implements it (a type assertion on Reader, like the Assign tab's).
type WealthReader interface {
	WealthHistory() []wealth.Sample
}

const wealthAxisW = 64 // room left of the plot for the axis values

// WealthPoints maps the samples (oldest first) onto plot: the first sample
// sits on the left edge and the last on the right, 0 at the bottom and
// the largest value (at least 1) at the top. A single sample sits on the
// right edge.
func WealthPoints(hist []wealth.Sample, plot gfx.Rect) [][2]float32 {
	top := wealthTop(hist)
	pts := make([][2]float32, len(hist))
	for i, s := range hist {
		x := plot.Right()
		if len(hist) > 1 {
			x = plot.X + plot.W*float32(i)/float32(len(hist)-1)
		}
		pts[i] = [2]float32{x, plot.Bottom() - plot.H*float32(max(s.Value, 0))/float32(top)}
	}
	return pts
}

// wealthTop is the value at the plot's top: the largest sample, at least 1.
func wealthTop(hist []wealth.Sample) int {
	top := 1
	for _, s := range hist {
		top = max(top, s.Value)
	}
	return top
}

// wealthList draws the Wealth tab in body.
func (w History) wealthList(body gfx.Rect) {
	var hist []wealth.Sample
	if r, ok := w.sh.Env().Game.(WealthReader); ok {
		hist = r.WealthHistory()
	}
	u := w.kit()
	u.Label(body.X, body.Y, w.sh.T("Colony wealth"), gfx.Label)
	ls := gfx.Small
	plot := gfx.Rect{X: body.X + wealthAxisW, Y: body.Y + 26, W: body.W - wealthAxisW - 8, H: body.H - 26 - float32(gfx.LineH(ls)) - 10}
	if plot.W <= 0 || plot.H <= 0 {
		return
	}
	cur := 0
	if n := len(hist); n > 0 {
		cur = hist[n-1].Value
	}
	u.Label(body.X+200, body.Y, fmt.Sprintf("%d", cur), uikit.DimText())
	u.Queue(func(dst *ebiten.Image) { w.drawWealth(dst, plot, hist) })
	u.Tip("history.wealth", plot, w.sh.T("Wealth history\nThe value of everything the colony owns, counted about once a day. Raids grow with it."))
}

// drawWealth paints the chart.
func (w History) drawWealth(dst *ebiten.Image, plot gfx.Rect, hist []wealth.Sample) {
	uikit.FillRect(dst, plot, uikit.Field)
	uikit.StrokeRect(dst, plot, uikit.PanelBorder)
	ls := gfx.Small
	ls.Color, ls.Align = uikit.TextDim, gfx.AlignRight
	top := wealthTop(hist)
	for _, f := range []float32{0, 0.5, 1} {
		y := plot.Bottom() - plot.H*f
		if f > 0 && f < 1 {
			gfx.Line(dst, plot.X, y, plot.Right(), y, 1, chartGrid, false)
		}
		gfx.Text(dst, fmt.Sprintf("%d", int(float32(top)*f)), float64(plot.X-6), gfx.CenterTop(float64(y)-gfx.LineH(ls)/2, gfx.LineH(ls), ls), ls)
	}
	if len(hist) == 0 {
		cs := uikit.DimText()
		cs.Align = gfx.AlignCenter
		gfx.Text(dst, w.sh.T("No history yet."), float64(plot.X+plot.W/2), gfx.CenterTop(float64(plot.Y+plot.H/2)-gfx.LineH(cs)/2, gfx.LineH(cs), cs), cs)
		return
	}
	line := uikit.Heading
	pts := WealthPoints(hist, plot)
	for i := 1; i < len(pts); i++ {
		gfx.Line(dst, pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], 1.5, line, true)
	}
	last := pts[len(pts)-1]
	gfx.FillCircle(dst, last[0], last[1], 3, line, true)
	es := gfx.Small
	es.Color = uikit.TextDim
	gfx.Text(dst, lang.Tf(w.sh, "Day %d", hist[0].Tick/clock.TicksPerDay), float64(plot.X), float64(plot.Bottom())+3, es)
	es.Align = gfx.AlignRight
	gfx.Text(dst, lang.Tf(w.sh, "Day %d", hist[len(hist)-1].Tick/clock.TicksPerDay), float64(plot.Right()), float64(plot.Bottom())+3, es)
}
