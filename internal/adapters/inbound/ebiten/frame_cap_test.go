package ebiten

import (
	"testing"
	"time"

	"rimworld-go/internal/platform/config"
)

// cappedColony is a live colony with an injected clock and sleep, so a
// frame's wait can be read back.
func cappedColony(t *testing.T) (*App, *time.Time, *[]time.Duration) {
	t.Helper()
	a, _, _, _ := bgColony(t)
	clock := time.Unix(1000, 0)
	a.now = func() time.Time { return clock }
	var slept []time.Duration
	a.bg.sleep = func(d time.Duration) { slept = append(slept, d); clock = clock.Add(d) }
	return a, &clock, &slept
}

// TestFrameCapPacesDraws: with VSync off and the default cap, a draw waits
// out the rest of 1/120 s since the previous deadline; the first paced
// draw only sets the pace; a stall restarts it instead of catching up.
func TestFrameCapPacesDraws(t *testing.T) {
	t.Parallel()
	a, clock, slept := cappedColony(t)
	if g := a.settings.Graphics; g.VSync || g.FrameCap != config.FrameCapDefault {
		t.Fatalf("defaults: vsync %v cap %d, want off and %d", g.VSync, g.FrameCap, config.FrameCapDefault)
	}
	gap := time.Second / time.Duration(config.FrameCapDefault)

	a.capBackgroundDraw() // sets the pace
	*clock = clock.Add(2 * time.Millisecond)
	a.capBackgroundDraw()
	if len(*slept) != 1 || (*slept)[0] != gap-2*time.Millisecond {
		t.Fatalf("slept %v, want one wait of %v", *slept, gap-2*time.Millisecond)
	}
	// an overshooting sleep: the next wait is shorter by the overshoot
	*clock = clock.Add(time.Millisecond) // the sleep returned 1 ms late
	*clock = clock.Add(2 * time.Millisecond)
	a.capBackgroundDraw()
	if len(*slept) != 2 || (*slept)[1] != gap-3*time.Millisecond {
		t.Fatalf("slept %v, want the second wait of %v", *slept, gap-3*time.Millisecond)
	}
	// a stall of two gaps: nothing to wait for, the pace restarts here
	*clock = clock.Add(2 * gap)
	a.capBackgroundDraw()
	*clock = clock.Add(time.Millisecond)
	a.capBackgroundDraw()
	if len(*slept) != 3 || (*slept)[2] != gap-time.Millisecond {
		t.Fatalf("slept %v, want a fresh wait of %v after the stall", *slept, gap-time.Millisecond)
	}
}

// TestFrameCapOff: VSync on, the cap Off, a capture and the fps probe all
// draw unpaced.
func TestFrameCapOff(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		set  func(a *App)
	}{
		{"vsync", func(a *App) { a.settings.Graphics.VSync = true }},
		{"off", func(a *App) { a.settings.Graphics.FrameCap = 0 }},
		{"capture", func(a *App) { a.cfg.Capture = &Capture{} }},
		{"probe", func(a *App) { a.cfg.FPSProbe = &FPSProbe{Seconds: 1} }},
	} {
		a, clock, slept := cappedColony(t)
		c.set(a)
		for i := 0; i < 3; i++ {
			a.capBackgroundDraw()
			*clock = clock.Add(time.Millisecond)
		}
		if len(*slept) != 0 {
			t.Fatalf("%s: slept %v, want no pacing", c.name, *slept)
		}
	}
}

// TestFrameCapRow: the Options row cycles the caps and is a preset row.
func TestFrameCapRow(t *testing.T) {
	t.Parallel()
	if got := frameCapLabel(0); got != "Off" {
		t.Fatalf("label for 0 = %q", got)
	}
	if got := frameCapLabel(120); got != "120" {
		t.Fatalf("label for 120 = %q", got)
	}
	store := graphicsStore()
	a := optionsApp(store)
	a.optionsTab = "graphics"
	a.applyOption("options.framecap")
	if g := store.s.Graphics; g.FrameCap != 144 || g.Quality != config.QualityCustom {
		t.Fatalf("after one cycle: cap %d quality %s, want 144 and custom", g.FrameCap, g.Quality)
	}
}
