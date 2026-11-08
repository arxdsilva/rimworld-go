package game

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/display"
	"rimworld-go/internal/platform/config"
)

// The Ebitengine screen is the client's Display.
var _ Display = display.Screen{}

type fixedScale float64

func (s fixedScale) DeviceScale() float64 { return float64(s) }

// A capture without overrides uses the defaults and saves nothing.
func TestCapturesUseTheDefaultSettings(t *testing.T) {
	t.Parallel()
	if h := settingsHooks(true, devGraphics{}, fixedScale(1)); h.Load != nil || h.Save != nil {
		t.Error("a capture read or saved the player's settings")
	}
}

// A dev override applies for this run only: it loads over the defaults and
// is never saved.
func TestDevGraphicsOverridesAreNeverSaved(t *testing.T) {
	t.Parallel()
	off := false
	h := settingsHooks(true, devGraphics{hidpi: &off, vsync: &off, renderScale: config.RenderScales[0], windowed: true}, fixedScale(2))
	if h.Save != nil || h.Load == nil {
		t.Fatal("an override must load and never save")
	}
	s, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	g := s.Graphics
	if g.HiDPI || g.VSync || g.Fullscreen || g.RenderScale != config.RenderScales[0] {
		t.Errorf("overrides not applied: %+v", g)
	}
}

func TestFPSProbeRunsAtTheBudgetSpeed(t *testing.T) {
	t.Parallel()
	if fpsProbeOf(0, 1, 1) != nil {
		t.Error("no -fpsprobe made a probe")
	}
	if p := fpsProbeOf(10, 0.5, 1); p == nil || p.Speed != 2 || p.Seconds != 10 || p.Zoom != 0.5 {
		t.Errorf("probe %+v", p)
	}
}
