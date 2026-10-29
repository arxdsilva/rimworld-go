package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	perfpkg "rimworld-go/internal/adapters/inbound/ebiten/perf"
	portspkg "rimworld-go/internal/adapters/inbound/ebiten/ports"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/perf"
)

// The frame-timing instruments live in perf (ARCH-005). These aliases and
// one-line wrappers keep the root's call sites unchanged while the
// features move out one package at a time; the last mover deletes this
// file (SOLID_GO.md §11).

type (
	// FPSProbe is the dev frame-rate measurement (cmd/game's -fpsprobe).
	FPSProbe = perf.FPSProbe
)

// ---- the App's instruments. The three timing wrappers run every frame:
// they hand the clock reading to the meter and nothing else.

func (a *App) perfBeginUpdate()            { a.meter.BeginUpdate(a.now()) }
func (a *App) perfEndUpdate()              { a.meter.EndUpdate(a.now()) }
func (a *App) perfEndDraw(start time.Time) { a.meter.EndDraw(start, a.now()) }

func (a *App) drawPerfOverlay(s *ebiten.Image, _, _ int) { a.meter.Draw(s) }

func (a *App) fpsProbeUpdate() bool {
	return a.cfg.FPSProbe != nil && a.meter.ProbeUpdate()
}

func (a *App) fpsProbeAfterUpdate() {
	if a.cfg.FPSProbe != nil {
		a.meter.ProbeAfterUpdate()
	}
}

func (a *App) fpsProbeDraw(start time.Time) {
	if a.cfg.FPSProbe != nil {
		a.meter.ProbeDraw(start)
	}
}

// perfHost is the root's side of perf.Host.
type perfHost struct{ a *App }

var _ perf.Host = perfHost{}

// perfShell is the feature's view of the App: the ports roles plus its Host.
type perfShell struct {
	shell
	perfHost
}

// newPerf builds the instruments over the App. Until the sweep deletes
// App.perf, App.fps and App.shot the instruments' state is those fields, so
// the root's own readers see the same state.
func newPerf(a *App) *perf.Meter {
	m := perf.New(perfShell{a.shell, perfHost{a}})
	return m
}

// Now is the presentation clock.
func (h perfHost) Now() time.Time { return h.a.now() }

// Env is the running game, the window, the Update budget and the probe.
func (h perfHost) Env() perf.Env {
	a := h.a
	e := perf.Env{PW: a.pw, PH: a.ph, Held: a.debt.held, MaxOwed: a.debt.maxOwed, Scale: a.scale, Probe: a.cfg.FPSProbe}
	if r := a.renderer; r != nil {
		e.InGame, e.Stepped, e.Tick, e.Date, e.Speed = true, r.mo.Stepped, uint64(r.game.Tick()), r.game.Date(), int(r.speed)
	}
	return e
}

// ResetMaxOwed forgets the largest tick debt.
func (h perfHost) ResetMaxOwed() { h.a.debt.maxOwed = 0 }

// PerfRect is where the HUD reserves the performance box.
func (h perfHost) PerfRect() gfx.Rect { return h.a.hud().Perf }

// Toast shows a transient message.
func (h perfHost) Toast(msg string) { h.a.chrome.Toast(msg) }

// ArtMem describes the art catalogue's memory.
func (h perfHost) ArtMem() string { return fmt.Sprint(h.a.art.MemStats()) }

// ForceSpeed puts the game back on a speed tier.
func (h perfHost) ForceSpeed(tier int) {
	if r := h.a.renderer; r != nil {
		r.speed = speed(tier)
	}
}

// StartProbe starts the probe's colony and frames the camera: the probe
// measures the whole map, in map pixels.
func (h perfHost) StartProbe(p *perfpkg.FPSProbe) (int, uint64, string) {
	a := h.a
	if a.nav.screen() == portspkg.ScreenBoot {
		a.finishBoot()
	}
	a.startGame()
	r := a.renderer
	if r == nil {
		return 0, 0, max(a.sessionErr, "the colony did not start")
	}
	switch p.Speed {
	case 1:
		r.speed = speedNormal
	case 3:
		r.speed = speedVeryFast
	default:
		r.speed = speedFast
	}
	tick0 := uint64(r.game.Tick())
	if p.Zoom > 0 {
		r.cam.Zoom = clampZoom(p.Zoom)
	}
	r.SyncDev() // the probe frames the screen, in map pixels
	tile, k := r.cam.Tile(), a.scale.WorldK()
	r.cam.OffsetX = (float64(r.game.Width())/2+0.5)*tile - float64(a.pw)*k/2
	r.cam.OffsetY = (float64(r.game.Height())/2)*tile - float64(a.ph)*k/2
	return int(r.speed), tick0, ""
}
