package perf

import (
	"fmt"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The performance box (PERF-002): five text lines and a frame-time
// sparkline in a small panel right of the resource list.
const (
	sparkLen = 120 // frames in the sparkline
	sparkH   = 24
	pad      = 6
	amberMs  = 16.7
	redMs    = 33.0
)

// Lines are the overlay's strings, rebuilt twice a second.
type Lines struct {
	FPS, Frame, Work, Sim, Mem string
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// format builds the strings from the stats.
func (st Stats) format() Lines {
	perTick := 0.0
	if st.TicksPerUpdate > 0 {
		perTick = ms(st.UpdateAvg) / st.TicksPerUpdate
	}
	return Lines{
		FPS:   fmt.Sprintf("%.0f fps · 1%% low %.0f", st.FPS, st.Low1),
		Frame: fmt.Sprintf("frame %.1f ms · p99 %.1f", ms(st.FrameAvg), ms(st.FrameP99)),
		Work:  fmt.Sprintf("update %.1f ms · draw %.1f ms", ms(st.UpdateAvg), ms(st.DrawAvg)),
		Sim:   fmt.Sprintf("sim %.0f× · %.1f ms/tick · held %d", st.TicksPerUpdate, perTick, st.Held),
		Mem:   fmt.Sprintf("heap %d MiB · gc %.0f/min", st.HeapMiB, st.GCPerMin),
	}
}

// All is the five strings, top to bottom.
func (l Lines) All() []string { return []string{l.FPS, l.Frame, l.Work, l.Sim, l.Mem} }

// widest are the longest the lines get, so the box keeps its size as the
// numbers change.
var widest = []string{
	"999 fps · 1% low 999", "frame 99.9 ms · p99 99.9", "update 99.9 ms · draw 99.9 ms",
	"sim 9× · 99.9 ms/tick · held 9999", "heap 9999 MiB · gc 999/min",
}

func (m *Meter) on() bool { return m.sh.Settings().Graphics.PerfOverlay }

// Box is the box's size at the current UI scale (0, 0: off).
func (m *Meter) Box() (w, h float32) {
	if !m.on() {
		return 0, 0
	}
	c := m.Clock
	if c.boxW == 0 || c.boxScale != gfx.HUDScale() { // text is measured at the HUD scale, world pass or not
		c.boxScale = gfx.HUDScale()
		c.boxW = gfx.MaxTextW(widest, gfx.Small) + 2*pad
		c.boxH = float32(len(widest))*gfx.LineStep(gfx.Small) + sparkH + 3*pad
	}
	return c.boxW, c.boxH
}

// Draw draws the performance box (PERF-002: fps, 1% low, frame, update
// and draw time, sim rate and memory) when Graphics.PerfOverlay is on. It
// draws after the HUD and also while the UI is hidden, so the overlay can
// measure a bare map.
func (m *Meter) Draw(screen *ebiten.Image) {
	if !m.on() || !m.sh.Env().InGame {
		return
	}
	r := m.sh.PerfRect()
	if r.Empty() {
		return
	}
	m.refresh(m.sh.Now())
	c := m.Clock
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	step := float64(gfx.LineStep(gfx.Small))
	y := float64(r.Y + pad)
	x := float64(r.X + pad)
	frameSt := gfx.Small
	switch p99 := ms(c.stats.FrameP99); {
	case p99 > redMs:
		frameSt = gfx.WithColor(gfx.Small, uikit.Danger)
	case p99 > amberMs:
		frameSt = gfx.WithColor(gfx.Small, uikit.Warning)
	}
	gfx.Text(screen, c.Lines.FPS, x, y, gfx.Small)
	gfx.Text(screen, c.Lines.Frame, x, y+step, frameSt)
	gfx.Text(screen, c.Lines.Work, x, y+2*step, gfx.Small)
	gfx.Text(screen, c.Lines.Sim, x, y+3*step, gfx.Small)
	gfx.Text(screen, c.Lines.Mem, x, y+4*step, gfx.Small)
	uikit.DrawSparkline(screen, gfx.Rect{X: r.X + pad, Y: r.Bottom() - pad - sparkH, W: r.W - 2*pad, H: sparkH}, c.Spark, uikit.Accent)
}

// SeedCapture fills the ring with a fixed series and freezes the clock so
// a screenshot is identical run to run.
func (m *Meter) SeedCapture() {
	c := m.Clock
	c.ring = Ring{}
	for i := 0; i < RingLen; i++ {
		wob := time.Duration((i*37)%11) * 100 * time.Microsecond
		fr := 8*time.Millisecond + wob
		if i%97 == 0 {
			fr += 6 * time.Millisecond // a hitch
		}
		c.ring.Push(Sample{Frame: fr, Update: 2100*time.Microsecond + wob/2, Draw: 4*time.Millisecond + wob/3})
	}
	c.frozen = true
	c.computed = time.Time{}
	m.refresh(time.Time{}.Add(time.Hour))
}
