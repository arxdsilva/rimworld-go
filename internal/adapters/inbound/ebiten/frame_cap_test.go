package ebiten

import (
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/perf"
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
		{"probe", func(a *App) { a.cfg.FPSProbe = &perf.FPSProbe{Seconds: 1} }},
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

// TestFrameCapMenuStaysAtSixty (PERF-003): off the colony (the menu,
// loading, the prewarm) draws are never faster than 60 fps, with the cap
// Off or at 120; VSync on still leaves the display to pace. The colony
// keeps the player's cap, Off included.
func TestFrameCapMenuStaysAtSixty(t *testing.T) {
	t.Parallel()
	menu := testApp(&stubSim{w: 4, h: 4})
	menu.finishBoot()
	for _, cap := range []int{0, 120, 144} {
		menu.settings.Graphics.FrameCap = cap
		if got := menu.frameCapGap(); got != menuDrawGap {
			t.Fatalf("menu with cap %d: gap %v, want %v", cap, got, menuDrawGap)
		}
	}
	menu.settings.Graphics.FrameCap = 30
	if got := menu.frameCapGap(); got != time.Second/30 {
		t.Fatalf("menu with cap 30: gap %v, want 1/30 s", got)
	}
	menu.settings.Graphics.FrameCap, menu.settings.Graphics.VSync = 0, true
	if got := menu.frameCapGap(); got != 0 {
		t.Fatalf("menu with vsync: gap %v, want 0", got)
	}
	if menuDrawGap != time.Second/60 {
		t.Fatalf("menuDrawGap = %v, want 60 fps", menuDrawGap)
	}

	colony, _, _ := cappedColony(t)
	colony.settings.Graphics.FrameCap = 0
	if got := colony.frameCapGap(); got != 0 {
		t.Fatalf("colony with the cap Off: gap %v, want none", got)
	}
	colony.settings.Graphics.FrameCap = 144
	if got := colony.frameCapGap(); got != time.Second/144 {
		t.Fatalf("colony at 144: gap %v", got)
	}
}

// TestFrameCapRow: the Options row cycles the caps and is a preset row.
func TestFrameCapRow(t *testing.T) {
	t.Parallel()
	if got := options.FrameCapLabel(0); got != "Off" {
		t.Fatalf("label for 0 = %q", got)
	}
	if got := options.FrameCapLabel(120); got != "120" {
		t.Fatalf("label for 120 = %q", got)
	}
	store := graphicsStore()
	a := optionsApp(store)
	a.opt.Tab = "graphics"
	a.opt.ApplyOption("options.framecap")
	if g := store.s.Graphics; g.FrameCap != 144 || g.Quality != config.QualityCustom {
		t.Fatalf("after one cycle: cap %d quality %s, want 144 and custom", g.FrameCap, g.Quality)
	}
}
