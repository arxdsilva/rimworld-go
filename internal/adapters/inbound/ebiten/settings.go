package ebiten

import (
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// SettingsHooks connect the Options screen to storage (zero value: the
// defaults, nothing saved).
type SettingsHooks struct {
	Load func() (config.Settings, error)
	Save func(config.Settings) error
}

// loadSettings reads the stored settings once at startup.
func (a *App) loadSettings() {
	a.binds = nil // rebuilt from the loaded controls
	a.settings = config.Defaults()
	if a.cfg.Settings.Load == nil {
		return
	}
	s, err := a.cfg.Settings.Load()
	a.settings = s.Normalize()
	if err != nil {
		a.hint = "settings: " + err.Error()
	}
}

// setSettings applies a change everywhere and stores it.
func (a *App) setSettings(s config.Settings) {
	a.settings = s.Normalize()
	a.applySettings()
	if a.cfg.Settings.Save != nil {
		if err := a.cfg.Settings.Save(a.settings); err != nil {
			a.hint = "settings not saved: " + err.Error()
		}
	}
}

// applySettings pushes the settings into the UI scale, the window, the
// colony renderer and the sound.
func (a *App) applySettings() {
	a.binds = input.FromSettings(a.settings.Controls.Bindings)
	a.updateScale() // UI scale and HiDPI apply live (ui_scale.go)
	if !a.settings.Gameplay.DevMode {
		a.dev = devState{}
	}
	if a.cfg.Window != nil {
		a.cfg.Window(a.settings.Graphics.Fullscreen, a.settings.Graphics.VSync)
	}
	a.syncRenderer()
	a.cfg.Sound.SetVolumes(a.settings.Audio)
}

// syncRenderer copies the presentation settings into the colony renderer.
func (a *App) syncRenderer() {
	r := a.renderer
	if r == nil {
		return
	}
	s := a.settings
	r.pauseOnThreat = s.Gameplay.PauseOnThreat
	r.fx = s.Graphics.FX()
	r.edgeScroll = s.Gameplay.EdgeScroll && a.cfg.Capture == nil
	r.smooth.speed = float64(s.Gameplay.PanSpeed) / 100
	r.reducedFlashing = s.Accessibility.ReducedFlashing
	r.reducedMotion = s.Accessibility.ReducedMotion
	r.smooth.instant = s.Accessibility.ReducedMotion // no eased pan or zoom
	if !s.Gameplay.DevMode && r.speed == speedUltra {
		r.speed = speedVeryFast // ultrafast is a developer tier
	}
}
