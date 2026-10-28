package ebiten

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/platform/config"
)

// PLAY-017 §4: the "Run in background" option.

// fakeBG records the window calls the option makes (bgWindow).
type fakeBG struct{ calls []string }

func (f *fakeBG) SetRunnableOnUnfocused(on bool) {
	f.calls = append(f.calls, fmt.Sprintf("runnable:%v", on))
}

func (f *fakeBG) SetVsyncEnabled(on bool) { f.calls = append(f.calls, fmt.Sprintf("vsync:%v", on)) }

// bgColony is a stub colony running at normal speed with no ticks taken
// yet, driven by a fake input and a fake window.
func bgColony(t *testing.T) (*App, *stubSim, *fakeInput, *fakeBG) {
	t.Helper()
	a, s, f := inputColony(t, [2]int{5, 5})
	a.renderer.speed = speedNormal
	s.steps = 0 // inputColony's settling frame already stepped once
	w := &fakeBG{}
	a.bg.win = w
	return a, s, f, w
}

func runInBackground(t *testing.T, a *App, on bool) {
	t.Helper()
	s := a.settings
	s.Gameplay.RunInBackground = on
	a.setSettings(s)
}

const bgPausedText = "Paused (window in background)"

func TestRunInBackgroundOptionRow(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	a.opt.Tab = "gameplay"
	b := optionBtn(t, a, "options.runinbackground")
	if b.Label != "Off" {
		t.Fatalf("run in background shows %q by default, want Off", b.Label)
	}
	const tip = "Keep the colony running while the window is in the background or minimised. Off: the game pauses until you come back."
	if b.Tip != tip {
		t.Fatalf("tip = %q", b.Tip)
	}
	a.applyOption("options.runinbackground")
	if !a.settings.Gameplay.RunInBackground || !store.s.Gameplay.RunInBackground {
		t.Fatalf("toggle did not set and store the option: %+v", store.s.Gameplay)
	}
	if got := optionBtn(t, a, "options.runinbackground").Label; got != "On" {
		t.Fatalf("label after toggle = %q", got)
	}
	// It survives a restart.
	if b2 := optionsApp(store); !b2.settings.Gameplay.RunInBackground {
		t.Fatal("run in background lost after restart")
	}
	a.applyOption("options.runinbackground")
	if a.settings.Gameplay.RunInBackground || store.s.Gameplay.RunInBackground {
		t.Fatal("second click should switch it off again")
	}
}

// Off (the default): an unfocused window advances no ticks, the top strip
// says why, and coming back resumes at the speed the player had.
func TestBackgroundOffPausesAndKeepsSpeed(t *testing.T) {
	t.Parallel()
	a, s, f, _ := bgColony(t)
	a.renderer.speed = speedFast
	f.Frame(t, a)
	before := s.steps
	if before == 0 {
		t.Fatal("a focused colony at speed Fast must tick")
	}
	f.Cur.Blurred = true
	for i := 0; i < 5; i++ {
		f.Frame(t, a)
	}
	if s.steps != before {
		t.Fatalf("ticks advanced while the window was in the background: %d -> %d", before, s.steps)
	}
	if got := a.hudTimeText(); !strings.Contains(got, bgPausedText) {
		t.Fatalf("top strip %q should say %q", got, bgPausedText)
	}
	if a.renderer.speed != speedFast {
		t.Fatalf("the auto-pause changed the player's speed to %v", a.renderer.speed)
	}
	f.Cur.Blurred = false
	f.Frame(t, a)
	if s.steps != before+speedFast.ticksPerFrame() {
		t.Fatalf("refocus should resume at Fast: steps %d -> %d", before, s.steps)
	}
	if got := a.hudTimeText(); strings.Contains(got, "background") {
		t.Fatalf("top strip %q still says background after refocus", got)
	}
}

// The player's own pause is never overwritten by the background pause.
func TestBackgroundKeepsManualPause(t *testing.T) {
	t.Parallel()
	a, s, f, _ := bgColony(t)
	a.renderer.togglePause()
	f.Frame(t, a)
	f.Cur.Blurred = true
	f.Frame(t, a)
	f.Cur.Blurred = false
	for i := 0; i < 3; i++ {
		f.Frame(t, a)
	}
	if a.renderer.speed != speedPaused || s.steps != 0 {
		t.Fatalf("manual pause lost across a background trip: speed %v, steps %d", a.renderer.speed, s.steps)
	}
	if got := a.hudTimeText(); strings.Contains(got, "background") {
		t.Fatalf("a manually paused colony shows %q after refocus", got)
	}
	a.renderer.togglePause() // the player resumes: ticks flow again
	f.Frame(t, a)
	if s.steps == 0 {
		t.Fatal("resume after refocus should tick")
	}
}

// On: the colony keeps ticking while unfocused and the strip stays plain.
func TestBackgroundOnKeepsTicking(t *testing.T) {
	t.Parallel()
	a, s, f, _ := bgColony(t)
	runInBackground(t, a, true)
	f.Cur.Blurred = true
	for i := 0; i < 4; i++ {
		f.Frame(t, a)
	}
	if s.steps != 4 {
		t.Fatalf("steps = %d after 4 background frames at Normal, want 4", s.steps)
	}
	if got := a.hudTimeText(); strings.Contains(got, "background") {
		t.Fatalf("top strip %q shows a background pause with the option on", got)
	}
}

// On: the window is made runnable, vsync goes off while unfocused (a
// hidden macOS window stalls the vsync loop) and comes back with focus.
func TestBackgroundOnSwitchesVsync(t *testing.T) {
	t.Parallel()
	a, _, f, w := bgColony(t)
	a.settings.Graphics.VSync = true // the player's choice; off by default since PERF-002
	runInBackground(t, a, true)
	f.Frame(t, a)
	if !slices.Equal(w.calls, []string{"runnable:true"}) {
		t.Fatalf("on, focused: calls %v, want the window made runnable once", w.calls)
	}
	f.Cur.Blurred = true
	f.Frame(t, a)
	f.Frame(t, a)
	f.Cur.Blurred = false
	f.Frame(t, a)
	f.Frame(t, a)
	want := []string{"runnable:true", "vsync:false", "vsync:true"}
	if !slices.Equal(w.calls, want) {
		t.Fatalf("calls = %v, want %v (one switch each way, not one per frame)", w.calls, want)
	}
}

// The vsync a refocus restores is the player's Graphics setting.
func TestBackgroundRestoresPlayersVsyncChoice(t *testing.T) {
	t.Parallel()
	a, _, f, w := bgColony(t)
	s := a.settings
	s.Gameplay.RunInBackground, s.Graphics.VSync = true, false
	a.setSettings(s)
	f.Cur.Blurred = true
	f.Frame(t, a)
	f.Cur.Blurred = false
	f.Frame(t, a)
	if last := w.calls[len(w.calls)-1]; last != "vsync:false" {
		t.Fatalf("calls = %v, want the refocus to keep vsync off as chosen", w.calls)
	}
}

// Off never touches the window: the pause is the App's own.
func TestBackgroundOffLeavesWindowAlone(t *testing.T) {
	t.Parallel()
	a, _, f, w := bgColony(t)
	f.Cur.Blurred = true
	f.Frame(t, a)
	f.Cur.Blurred = false
	f.Frame(t, a)
	if len(w.calls) != 0 {
		t.Fatalf("window calls with the option off: %v", w.calls)
	}
}

// Captures and the fps probe run unfocused by design: they never pause.
func TestBackgroundNeverPausesCapturesOrProbe(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(*App){
		"capture": func(a *App) { a.cfg.Capture = &Capture{} },
		"probe":   func(a *App) { a.cfg.FPSProbe = &FPSProbe{} },
	} {
		t.Run(name, func(t *testing.T) {
			a, _, _, _ := bgColony(t)
			set(a)
			a.in.Raw.Blurred = true
			a.syncBackground()
			if a.bg.paused {
				t.Fatalf("%s: a background window must not pause", name)
			}
		})
	}
}

// While throttled (option on, window unfocused) a draw waits out the rest
// of the ~20 fps frame; otherwise it never sleeps.
func TestBackgroundDrawCap(t *testing.T) {
	t.Parallel()
	a, _, f, _ := bgColony(t)
	clock := time.Unix(1000, 0)
	a.now = func() time.Time { return clock }
	var slept []time.Duration
	a.bg.sleep = func(d time.Duration) { slept = append(slept, d); clock = clock.Add(d) }
	a.settings.Graphics.FrameCap = 0 // the frame cap has its own test (frame_cap_test.go)

	a.capBackgroundDraw() // focused: no cap
	clock = clock.Add(time.Millisecond)
	a.capBackgroundDraw()
	if len(slept) != 0 {
		t.Fatalf("slept %v while focused", slept)
	}

	runInBackground(t, a, true)
	f.Cur.Blurred = true
	f.Frame(t, a)
	a.capBackgroundDraw() // first throttled frame sets the pace
	clock = clock.Add(10 * time.Millisecond)
	a.capBackgroundDraw()
	if len(slept) != 1 || slept[0] != bgDrawGap-10*time.Millisecond {
		t.Fatalf("slept %v, want one wait of %v", slept, bgDrawGap-10*time.Millisecond)
	}
	clock = clock.Add(2 * bgDrawGap) // a slow frame: nothing to wait for
	a.capBackgroundDraw()
	if len(slept) != 1 {
		t.Fatalf("slept again after a slow frame: %v", slept)
	}
	if bgDrawGap < 40*time.Millisecond || bgDrawGap > 60*time.Millisecond {
		t.Fatalf("bgDrawGap = %v, want about 20 fps", bgDrawGap)
	}
}

// The real source reports the window as backgrounded only after it has
// been focused once: a window that never had focus (tests, captures,
// launched behind the terminal) is not "in the background".
func TestEbitenInputBlurredNeedsFocusFirst(t *testing.T) {
	t.Parallel()
	var e ebitenInput
	steps := []struct{ focused, blurred bool }{
		{false, false}, // never focused yet
		{true, false},
		{false, true}, // lost focus
		{false, true},
		{true, false}, // back
	}
	for i, s := range steps {
		if got := e.BlurredFor(s.focused); got != s.blurred {
			t.Fatalf("step %d (focused=%v): blurred = %v, want %v", i, s.focused, got, s.blurred)
		}
	}
}

// A new App drives the window its config names (AppConfig.Background;
// the real one by default) the first time the setting needs it.
func TestBackgroundUsesTheAppWindowByDefault(t *testing.T) {
	t.Parallel()
	w := &fakeBG{}
	a, _, f := inputColony(t, [2]int{5, 5})
	a.cfg.Background, a.bg.win = w, nil
	runInBackground(t, a, true)
	f.Frame(t, a)
	if !slices.Equal(w.calls, []string{"runnable:true"}) {
		t.Fatalf("calls = %v, want the default window made runnable", w.calls)
	}
}
