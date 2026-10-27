package ebiten

import (
	"fmt"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The performance box (PERF-002): five text lines and a frame-time
// sparkline in a small panel right of the resource list.
const (
	perfSparkLen = 120 // frames in the sparkline
	perfSparkH   = 24
	perfPad      = 6
	perfAmberMs  = 16.7
	perfRedMs    = 33.0
)

// perfLines are the overlay's strings, rebuilt twice a second.
type perfLines struct {
	fps, frame, work, sim, mem string
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// format builds the strings from the stats.
func (st perfStats) format() perfLines {
	perTick := 0.0
	if st.TicksPerUpdate > 0 {
		perTick = ms(st.UpdateAvg) / st.TicksPerUpdate
	}
	return perfLines{
		fps:   fmt.Sprintf("%.0f fps · 1%% low %.0f", st.FPS, st.Low1),
		frame: fmt.Sprintf("frame %.1f ms · p99 %.1f", ms(st.FrameAvg), ms(st.FrameP99)),
		work:  fmt.Sprintf("update %.1f ms · draw %.1f ms", ms(st.UpdateAvg), ms(st.DrawAvg)),
		sim:   fmt.Sprintf("sim %.0f× · %.1f ms/tick · held %d", st.TicksPerUpdate, perTick, st.Held),
		mem:   fmt.Sprintf("heap %d MiB · gc %.0f/min", st.HeapMiB, st.GCPerMin),
	}
}

func (l perfLines) all() []string { return []string{l.fps, l.frame, l.work, l.sim, l.mem} }

// perfWidest are the longest the lines get, so the box keeps its size as
// the numbers change.
var perfWidest = []string{
	"999 fps · 1% low 999", "frame 99.9 ms · p99 99.9", "update 99.9 ms · draw 99.9 ms",
	"sim 9× · 99.9 ms/tick · held 9999", "heap 9999 MiB · gc 999/min",
}

func (a *App) perfOn() bool { return a.settings.Graphics.PerfOverlay }

// perfBox is the box's size at the current UI scale (0, 0: off).
func (a *App) perfBox() (w, h float32) {
	if !a.perfOn() {
		return 0, 0
	}
	p := &a.perf
	if p.boxW == 0 || p.boxScale != uiScale() {
		p.boxScale = uiScale()
		p.boxW = maxTextW(perfWidest, tsSmall) + 2*perfPad
		p.boxH = float32(len(perfWidest))*lineStep(tsSmall) + perfSparkH + 3*perfPad
	}
	return p.boxW, p.boxH
}

// perfRect is where the box sits: right of the resource list, clear of the
// colonist bar and the messages, never reaching the right column or the
// lower-left stack. Empty when it does not fit.
func perfRect(l hudLayout, w, h float32) rect {
	if w <= 0 || h <= 0 {
		return rect{}
	}
	x := max(l.Resources.right(), hudMargin+resourcesW) + hudMargin
	y := float32(hudMargin)
	for _, r := range []rect{l.Bar, l.Messages} {
		if !r.empty() && r.X < x+w && r.right() > x {
			y = max(y, r.bottom()+hudMargin)
		}
	}
	bottom := l.Tabs.Y - hudMargin
	for _, r := range []rect{l.Window, l.Expanded, l.PaneTabs, l.ArchDesc, l.ArchCats, l.Grid} {
		if !r.empty() && r.X < x+w {
			bottom = min(bottom, r.Y-hudMargin)
		}
	}
	if x+w > l.Right.X-hudMargin || y+h > bottom {
		return rect{}
	}
	return rect{x, y, w, h}
}

// drawPerfOverlay draws the performance box (PERF-002: fps, 1% low, frame,
// update and draw time, sim rate and memory) when Graphics.PerfOverlay is
// on. It draws after the HUD and also while the UI is hidden, so the
// overlay can measure a bare map. sw, sh are logical.
func (a *App) drawPerfOverlay(screen *ebiten.Image, sw, sh int) {
	if !a.perfOn() || a.renderer == nil {
		return
	}
	r := a.hud().Perf
	if r.empty() {
		return
	}
	a.perfRefresh(a.now())
	p := &a.perf
	drawPanel(screen, r.X, r.Y, r.W, r.H)
	step := float64(lineStep(tsSmall))
	y := float64(r.Y + perfPad)
	x := float64(r.X + perfPad)
	frameSt := tsSmall
	switch p99 := ms(p.stats.FrameP99); {
	case p99 > perfRedMs:
		frameSt = withColor(tsSmall, uiDanger)
	case p99 > perfAmberMs:
		frameSt = withColor(tsSmall, uiWarning)
	}
	drawText(screen, p.lines.fps, x, y, tsSmall)
	drawText(screen, p.lines.frame, x, y+step, frameSt)
	drawText(screen, p.lines.work, x, y+2*step, tsSmall)
	drawText(screen, p.lines.sim, x, y+3*step, tsSmall)
	drawText(screen, p.lines.mem, x, y+4*step, tsSmall)
	drawSparkline(screen, rect{r.X + perfPad, r.bottom() - perfPad - perfSparkH, r.W - 2*perfPad, perfSparkH}, p.spark, uiAccent)
}

// seedPerfCapture fills the ring with a fixed series and freezes the clock
// so a screenshot is identical run to run.
func (a *App) seedPerfCapture() {
	p := &a.perf
	p.ring = perfRing{}
	for i := 0; i < perfRingLen; i++ {
		wob := time.Duration((i*37)%11) * 100 * time.Microsecond
		fr := 8*time.Millisecond + wob
		if i%97 == 0 {
			fr += 6 * time.Millisecond // a hitch
		}
		p.ring.push(perfSample{frame: fr, update: 2100*time.Microsecond + wob/2, draw: 4*time.Millisecond + wob/3})
	}
	p.frozen = true
	p.computed = time.Time{}
	a.perfRefresh(time.Time{}.Add(time.Hour))
}
