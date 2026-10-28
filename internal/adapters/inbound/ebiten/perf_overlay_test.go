package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/config"
)

func durs(n int, d time.Duration) []time.Duration {
	out := make([]time.Duration, n)
	for i := range out {
		out[i] = d
	}
	return out
}

func TestFrameStats(t *testing.T) {
	t.Parallel()
	frames := durs(99, 10*time.Millisecond)
	frames = append(frames, 50*time.Millisecond) // one hitch
	st := frameStats(frames, durs(10, 2*time.Millisecond), durs(10, 4*time.Millisecond))
	if st.FrameP99 != 50*time.Millisecond || st.Low1 != 20 {
		t.Fatalf("p99 %v low %v", st.FrameP99, st.Low1)
	}
	if want := float64(time.Second) / float64(st.FrameAvg); st.FPS != want || st.FPS < 90 || st.FPS > 100 {
		t.Fatalf("fps %v", st.FPS)
	}
	if st.UpdateAvg != 2*time.Millisecond || st.DrawP99 != 4*time.Millisecond {
		t.Fatalf("update %v draw p99 %v", st.UpdateAvg, st.DrawP99)
	}
	if z := frameStats(nil, nil, nil); z.FPS != 0 || z.Low1 != 0 {
		t.Fatalf("empty stats %+v", z)
	}
}

func TestFrameStatsMatchProbeReport(t *testing.T) {
	t.Parallel()
	st := &fpsProbeState{}
	for i := 1; i <= 200; i++ {
		st.frames = append(st.frames, time.Duration(7+i%9)*time.Millisecond)
		st.update = append(st.update, time.Duration(1+i%4)*time.Millisecond)
		st.draw = append(st.draw, time.Duration(3+i%5)*time.Millisecond)
	}
	f := frameStats(st.frames, st.update, st.draw)
	got := fpsReport(st, uiScaleState{S: 1, Dev: 1})
	want := fmt.Sprintf("fps probe: %d frames, avg %.1f fps, 1%% low %.1f fps (frame p99 %v); update avg %v p99 %v; draw avg %v p99 %v; work avg %v p99 %v; scale s 1 dev 1 world 1; ",
		200, f.FPS, f.Low1, f.FrameP99, f.UpdateAvg, f.UpdateP99, f.DrawAvg, f.DrawP99, f.UpdateAvg+f.DrawAvg, f.UpdateP99+f.DrawP99)
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("report\n%q\nwant prefix\n%q", got, want)
	}
}

func TestPerfRingWraps(t *testing.T) {
	t.Parallel()
	var r perfRing
	for i := 1; i <= perfRingLen+50; i++ {
		r.push(perfSample{frame: time.Duration(i)})
	}
	if r.frame.n != perfRingLen {
		t.Fatalf("held %d", r.frame.n)
	}
	got := r.frame.last(perfRingLen, nil)
	if got[0] != 51 || got[len(got)-1] != perfRingLen+50 {
		t.Fatalf("window %v .. %v", got[0], got[len(got)-1])
	}
	if l := r.frame.last(3, nil); l[0] != perfRingLen+48 || len(l) != 3 {
		t.Fatalf("newest three %v", l)
	}
}

func perfApp(t *testing.T, on bool) (*App, *fakeInput) {
	a, f := colonyApp(t, &stubSim{})
	a.settings.Graphics.PerfOverlay = on
	return a, f
}

func traceDraw(a *App, w, h int, draw func(*ebiten.Image)) []traceEvent {
	var evs []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { evs = append(evs, e) })
	defer func() { gfx.SetDrawTrace(nil) }()
	draw(ebiten.NewImage(w, h))
	return evs
}

func TestPerfOverlayDrawsOnlyWhenOn(t *testing.T) {
	for _, on := range []bool{false, true} {
		a, _ := perfApp(t, on)
		a.seedPerfCapture()
		evs := traceDraw(a, 1280, 720, func(img *ebiten.Image) { a.drawPerfOverlay(img, 1280, 720) })
		if (len(evs) > 0) != on {
			t.Fatalf("on=%v: %d draws", on, len(evs))
		}
		if !on && !a.hud().Perf.Empty() {
			t.Fatal("off but the layout reserves a rect")
		}
	}
}

func TestPerfOverlayShowsWithHiddenUI(t *testing.T) {
	a, _ := perfApp(t, true)
	a.seedPerfCapture()
	a.hideUI = true
	if a.hud().Perf.Empty() {
		t.Fatal("no rect")
	}
	evs := traceDraw(a, 1280, 720, func(img *ebiten.Image) { a.Draw(img) })
	found := false
	for _, e := range evs {
		found = found || (e.Kind == "text" && e.Text == a.perf.lines.fps)
	}
	if !found {
		t.Fatal("hidden UI hides the overlay")
	}
}

func TestPerfOverlayStaysInsideItsRect(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprintf("x%v", s), func(t *testing.T) {
			a, f := perfApp(t, true)
			pw, ph := rightAtScale(t, a, f, s)
			a.seedPerfCapture()
			l := a.hud()
			if l.Perf.Empty() {
				t.Fatal("no Perf rect")
			}
			evs := traceDraw(a, pw, ph, func(img *ebiten.Image) { a.drawPerfOverlay(img, a.sw, a.sh) })
			texts := 0
			for _, e := range evs {
				x, y, w, h := e.X/float32(s), e.Y/float32(s), e.W/float32(s), e.H/float32(s)
				const eps = 0.6
				if x < l.Perf.X-eps || y < l.Perf.Y-eps || x+w > l.Perf.Right()+eps || y+h > l.Perf.Bottom()+eps {
					t.Errorf("%s %q at %g,%g %gx%g leaves %+v", e.Kind, e.Text, x, y, w, h, l.Perf)
				}
				if e.Kind == "text" {
					texts++
				}
			}
			if texts != 5 {
				t.Errorf("%d text runs, want 5", texts)
			}
		})
	}
}

func TestPerfOverlayAvoidsHUDRegions(t *testing.T) {
	t.Parallel()
	busy := hudState{Colonists: 30, PaneShown: true, Commands: 13, GlobalH: 190, MinimapOn: true, LearnH: 200,
		Alerts: 8, Letters: 8, ResourcesH: 600, Messages: 5, ExpandedW: 500, ExpandedH: 300, PerfW: 215, PerfH: 120}
	idle := hudState{GlobalH: 150, ResourcesH: 100, PerfW: 215, PerfH: 120}
	for _, sz := range [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}} {
		for n, st := range map[string]hudState{"busy": busy, "idle": idle} {
			l := computeHUDLayout(sz[0], sz[1], st)
			if l.Perf.Empty() {
				continue
			}
			for _, r := range append(l.Chrome(), l.Minimap) {
				if r == l.Perf {
					continue
				}
				if i := l.Perf.Intersect(r); i.W > 0 && i.H > 0 {
					t.Errorf("%s %v: perf %+v collides with %+v", n, sz, l.Perf, r)
				}
			}
			if col := devColumn(l); !col.Empty() {
				if i := col.Intersect(l.Perf); i.W > 0 && i.H > 0 {
					t.Errorf("%s %v: dev column %+v overlaps perf %+v", n, sz, col, l.Perf)
				}
			}
		}
	}
	if !computeHUDLayout(1280, 720, hudState{ResourcesH: 100, GlobalH: 150, PerfW: 215, PerfH: 120}).Perf.Empty() == false {
		t.Error("an idle 1280x720 screen should fit the overlay")
	}
}

// The overlay adds no allocation of its own in steady state: the strings
// are cached between recomputes, so a frame costs what the canvas
// primitives it calls cost (ebiten's text and vector paths allocate by
// themselves) and nothing more.
func TestPerfOverlayDrawNoAllocs(t *testing.T) {
	a, _ := perfApp(t, true)
	a.seedPerfCapture()
	img := ebiten.NewImage(1280, 720)
	r := a.hud().Perf
	prims := func() {
		a.hud()
		drawPanel(img, r.X, r.Y, r.W, r.H)
		for i, l := range a.perf.lines.all() {
			drawText(img, l, 10, float64(10+i*12), tsSmall)
		}
		drawSparkline(img, r, a.perf.spark, uiAccent)
	}
	prims()
	a.drawPerfOverlay(img, 1280, 720) // warm the glyph caches
	base := testing.AllocsPerRun(10, prims)
	got := testing.AllocsPerRun(10, func() { a.drawPerfOverlay(img, 1280, 720) })
	if got > base+base/20+4 {
		t.Fatalf("overlay allocates %v per frame, its primitives alone %v", got, base)
	}
}

func TestCapturePerfIsDeterministic(t *testing.T) {
	var got [2][]string
	for i := range got {
		a, _ := perfApp(t, false)
		a.captureUI("perf")
		if !a.settings.Graphics.PerfOverlay {
			t.Fatal("capture did not turn the overlay on")
		}
		got[i] = append(a.perf.lines.all(), fmt.Sprint(a.perf.spark))
	}
	if fmt.Sprint(got[0]) != fmt.Sprint(got[1]) {
		t.Fatalf("captures differ:\n%v\n%v", got[0], got[1])
	}
}

func TestOptionsPerfOverlayToggle(t *testing.T) {
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	a.optionsTab = "graphics"
	b := optionBtn(t, a, "options.perfoverlay")
	if b.Label != "Off" || b.Tip == "" {
		t.Fatalf("perf overlay button %+v", b)
	}
	a.applyOption("options.perfoverlay")
	if !store.s.Graphics.PerfOverlay || optionBtn(t, a, "options.perfoverlay").Label != "On" {
		t.Fatal("overlay should be on and saved")
	}
}
