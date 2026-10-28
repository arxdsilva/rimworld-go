package perf

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
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
	st := FrameStats(frames, durs(10, 2*time.Millisecond), durs(10, 4*time.Millisecond))
	if st.FrameP99 != 50*time.Millisecond || st.Low1 != 20 {
		t.Fatalf("p99 %v low %v", st.FrameP99, st.Low1)
	}
	if want := float64(time.Second) / float64(st.FrameAvg); st.FPS != want || st.FPS < 90 || st.FPS > 100 {
		t.Fatalf("fps %v", st.FPS)
	}
	if st.UpdateAvg != 2*time.Millisecond || st.DrawP99 != 4*time.Millisecond {
		t.Fatalf("update %v draw p99 %v", st.UpdateAvg, st.DrawP99)
	}
	if z := FrameStats(nil, nil, nil); z.FPS != 0 || z.Low1 != 0 {
		t.Fatalf("empty stats %+v", z)
	}
}

func TestFrameStatsMatchProbeReport(t *testing.T) {
	t.Parallel()
	st := &Probe{}
	for i := 1; i <= 200; i++ {
		st.frames = append(st.frames, time.Duration(7+i%9)*time.Millisecond)
		st.update = append(st.update, time.Duration(1+i%4)*time.Millisecond)
		st.draw = append(st.draw, time.Duration(3+i%5)*time.Millisecond)
	}
	f := FrameStats(st.frames, st.update, st.draw)
	got := Report(st, gfx.Scale{S: 1, Dev: 1})
	want := fmt.Sprintf("fps probe: %d frames, avg %.1f fps, 1%% low %.1f fps (frame p99 %v); update avg %v p99 %v; draw avg %v p99 %v; work avg %v p99 %v; scale s 1 dev 1 world 1; ",
		200, f.FPS, f.Low1, f.FrameP99, f.UpdateAvg, f.UpdateP99, f.DrawAvg, f.DrawP99, f.UpdateAvg+f.DrawAvg, f.UpdateP99+f.DrawP99)
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("report\n%q\nwant prefix\n%q", got, want)
	}
}

func TestRingWraps(t *testing.T) {
	t.Parallel()
	var r Ring
	for i := 1; i <= RingLen+50; i++ {
		r.Push(Sample{Frame: time.Duration(i)})
	}
	if r.Frame.N != RingLen {
		t.Fatalf("held %d", r.Frame.N)
	}
	got := r.Frame.Last(RingLen, nil)
	if got[0] != 51 || got[len(got)-1] != RingLen+50 {
		t.Fatalf("window %v .. %v", got[0], got[len(got)-1])
	}
	if l := r.Frame.Last(3, nil); l[0] != RingLen+48 || len(l) != 3 {
		t.Fatalf("newest three %v", l)
	}
}

func TestMeterRecordsFramesAndFreezes(t *testing.T) {
	t.Parallel()
	f := &fakeShell{env: Env{InGame: true, Stepped: 3, Held: 2}}
	m := New(f)
	t0 := time.Unix(100, 0)
	for i := 0; i < 10; i++ {
		start := t0.Add(time.Duration(i) * 10 * time.Millisecond)
		m.BeginUpdate(start)
		m.EndUpdate(start.Add(2 * time.Millisecond))
		m.EndDraw(start, start.Add(4*time.Millisecond))
	}
	m.refresh(t0.Add(time.Hour))
	l := m.Clock.Lines
	if !strings.HasPrefix(l.Sim, "sim 3×") || !strings.HasSuffix(l.Sim, "held 2") || m.Clock.stats.UpdateAvg != 2*time.Millisecond {
		t.Fatalf("lines %+v stats %+v", l, m.Clock.stats)
	}
	if n := len(m.Clock.Spark); n != 9 {
		t.Fatalf("spark holds %d frame gaps, want 9", n)
	}
	m.SeedCapture()
	a, b := m.Clock.Lines.All(), fmt.Sprint(m.Clock.Spark)
	m.BeginUpdate(t0)
	m.EndDraw(t0, t0.Add(time.Second)) // frozen: ignored
	m.SeedCapture()
	if fmt.Sprint(a) != fmt.Sprint(m.Clock.Lines.All()) || b != fmt.Sprint(m.Clock.Spark) {
		t.Fatal("a seeded capture is deterministic")
	}
}

func TestBoxIsOffWithoutTheSetting(t *testing.T) {
	t.Parallel()
	f := &fakeShell{}
	m := New(f)
	if w, h := m.Box(); w != 0 || h != 0 {
		t.Fatalf("off box %v x %v", w, h)
	}
	f.set.Graphics.PerfOverlay = true
	if w, h := m.Box(); w <= 0 || h <= 0 {
		t.Fatalf("on box %v x %v", w, h)
	}
}

func TestProbeStartsRunsAndHoldsSpeed(t *testing.T) {
	t.Parallel()
	f := &fakeShell{env: Env{InGame: true, Probe: &FPSProbe{Seconds: 1}, Speed: 2, Held: 5}}
	m := New(f)
	if m.ProbeUpdate() || !m.Probe.started || m.Probe.speed != 2 || m.Probe.tick0 != 10 {
		t.Fatalf("probe %+v", m.Probe)
	}
	f.env.Speed = 0 // a threat paused the game
	m.Probe.drawn = true
	m.ProbeDraw(time.Now())
	if m.ProbeUpdate() || len(f.forced) != 1 || f.forced[0] != 2 || m.Probe.held != 1 {
		t.Fatalf("forced %v held %d", f.forced, m.Probe.held)
	}
	m.Probe.warmupTo = time.Now().Add(-time.Second)
	m.ProbeUpdate()
	if !m.Probe.measured || m.Probe.BudgetHeld0 != 5 || f.resets != 1 {
		t.Fatalf("measured %v held0 %d resets %d", m.Probe.measured, m.Probe.BudgetHeld0, f.resets)
	}
	m.Probe.begin = time.Now().Add(-time.Minute)
	if !m.ProbeUpdate() || !strings.HasPrefix(m.Probe.Report(), "fps probe:") || !strings.Contains(m.Probe.Report(), "budget held 0 updates") {
		t.Fatalf("report %q", m.Probe.Report())
	}
}

func TestScreenshotWritesAPNGNamedAfterTheDate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := &fakeShell{now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), env: Env{InGame: true}}
	m := New(f)
	m.Shot.Dir = func() (string, error) { return dir, nil }
	m.TakeScreenshot(func(*image.RGBA) {}, 4, 4)
	if want := filepath.Join(dir, "rimworld-go-0-0-0-030405.png"); m.Shot.Last != want {
		t.Fatalf("last %q want %q", m.Shot.Last, want)
	}
	if _, err := os.Stat(m.Shot.Last); err != nil || len(f.toasts) != 1 || f.toasts[0] != "Screenshot saved" {
		t.Fatalf("stat %v toasts %v", err, f.toasts)
	}
	f.env.InGame = false
	if n := m.name(); !strings.HasPrefix(n, "rimworld-go-menu-") {
		t.Fatalf("menu name %q", n)
	}
	m.Shot.Dir = func() (string, error) { return filepath.Join(dir, "x", "\x00"), nil }
	m.TakeScreenshot(func(*image.RGBA) {}, 1, 1)
	if !strings.HasPrefix(f.toasts[len(f.toasts)-1], "Screenshot failed") {
		t.Fatalf("toasts %v", f.toasts)
	}
}
