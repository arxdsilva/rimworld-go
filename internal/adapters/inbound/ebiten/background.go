package ebiten

import (
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// "Run in background" (PLAY-017 §4, Options > Gameplay).
//
// Off (the default): while the window is in the background or minimised
// the colony takes no ticks and the top strip says why. The player's own
// pause and speed are never touched, so coming back resumes exactly where
// they left off.
//
// On: the colony keeps running. A window with no focus stalls Ebitengine's
// vsync loop on macOS, so while it is in the background vsync goes off and
// Draw is capped at about 20 fps; Update keeps its 60 ticks per second and
// catches up (three per frame). Focus brings vsync back as the player set it.

// bgDrawGap is the shortest time between two draws while the colony runs
// in the background (about 20 fps).
const bgDrawGap = 50 * time.Millisecond

// bgPausedLabel replaces the speed label in the top strip while the
// window is in the background with the option off.
const bgPausedLabel = "Paused (window in background)"

// bgWindow is the part of the OS window the option drives. The App takes
// it as an interface so tests can fake it.
type bgWindow interface {
	SetRunnableOnUnfocused(on bool)
	SetVsyncEnabled(on bool)
}

type ebitenBG struct{}

func (ebitenBG) SetRunnableOnUnfocused(on bool) { ebiten.SetRunnableOnUnfocused(on) }
func (ebitenBG) SetVsyncEnabled(on bool)        { ebiten.SetVsyncEnabled(on) }

// appBackground is the window a new App drives (tests swap fields on the
// App instead). It is only touched when the option is on, so a default
// App never calls Ebitengine.
var appBackground bgWindow = ebitenBG{}

// bgState is the App's background bookkeeping.
type bgState struct {
	win   bgWindow
	sleep func(time.Duration) // time.Sleep; tests replace it
	// paused: the window is in the background with the option off, so the
	// colony takes no ticks (set each Update by syncBackground).
	paused bool
	// throttled: vsync was turned off and Draw is capped (option on).
	throttled bool
	// runnable: the window was told to keep running unfocused.
	runnable bool
	// next is the deadline the previous paced draw was held to (zero: no
	// pace yet). The next deadline is next + gap, not now + gap, so a sleep
	// that overshoots shortens the following wait and the average frame
	// rate holds the cap exactly.
	next time.Time
}

func (a *App) bgWin() bgWindow {
	if a.bg.win == nil {
		a.bg.win = appBackground
	}
	return a.bg.win
}

// syncBackground applies the option to this Update's focus. A capture or
// the fps probe runs unfocused by design and is never held back.
func (a *App) syncBackground() {
	run := a.settings.Gameplay.RunInBackground
	blurred := a.in.raw.blurred && a.cfg.Capture == nil && a.cfg.FPSProbe == nil
	a.bg.paused = blurred && !run
	if run && !a.bg.runnable {
		a.bgWin().SetRunnableOnUnfocused(true)
		a.bg.runnable = true
	}
	if throttle := blurred && run; throttle != a.bg.throttled {
		a.bg.throttled = throttle
		a.bg.next = time.Time{}
		a.bgWin().SetVsyncEnabled(!throttle && a.settings.Graphics.VSync)
	}
}

// frameCapGap is the shortest time between two draws under the frame cap
// (Graphics.FrameCap, PERF-002: 120 fps on every display by default, with
// VSync off, so the screen's refresh never paces the game). It is 0, no
// cap, while VSync is on (the display paces), with the cap Off, and for a
// capture or the fps probe (both uncapped by design).
func (a *App) frameCapGap() time.Duration {
	g := a.settings.Graphics
	if g.FrameCap <= 0 || g.VSync || a.cfg.Capture != nil || a.cfg.FPSProbe != nil {
		return 0
	}
	return time.Second / time.Duration(g.FrameCap)
}

// capBackgroundDraw ends a Draw: it waits out the rest of the frame's gap,
// bgDrawGap while throttled in the background, else the frame cap's. The
// first paced draw only sets the pace; a draw that runs a whole gap late
// (a stall) restarts it instead of racing to catch up.
func (a *App) capBackgroundDraw() {
	gap := a.frameCapGap()
	if a.bg.throttled {
		gap = bgDrawGap
	}
	if gap == 0 {
		a.bg.next = time.Time{}
		return
	}
	now := a.now()
	if a.bg.next.IsZero() {
		a.bg.next = now
		return
	}
	target := a.bg.next.Add(gap)
	if now.Sub(target) >= gap {
		a.bg.next = now
		return
	}
	if wait := target.Sub(now); wait > 0 {
		sleep := a.bg.sleep
		if sleep == nil {
			sleep = time.Sleep
		}
		sleep(wait)
	}
	a.bg.next = target
}

// speedLabel is the top strip's speed text: the background pause when the
// window is away, else the renderer's own label (which says "paused
// (threat)" when a threat paused the game).
func (a *App) speedLabel() string {
	if a.bg.paused {
		return bgPausedLabel
	}
	return a.renderer.speedRate()
}
