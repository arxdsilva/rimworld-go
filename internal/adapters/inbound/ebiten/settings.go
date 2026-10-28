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

// loadSettings reads the stored settings once at startup. A file that has
// no detected graphics tier yet (a first launch, or one from before
// PLAY-019) gets one from the hardware (detectGraphics).
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
	a.detectGraphics()
}

// detectGraphics picks the graphics tier from the machine when none was
// detected yet and saves once.
//
// A settings file from before PLAY-019 keeps every graphics value the
// player had changed from that version's defaults. The file's version
// cannot say which files those are: config.Decode normalizes (and stamps
// the current version) before Load returns. It does not need to. While
// Detected is empty, no preset has been applied, so a row that differs
// from the old defaults can only be the player's own choice, and a fresh
// install has none; the marking runs whenever Detected is empty.
//
// Captures and the fps probe keep their deterministic defaults: they
// neither detect nor save. So does a store that cannot save (the -hidpi
// developer override) and a start without a hardware reader; Detected
// then stays empty and a later start detects.
func (a *App) detectGraphics() {
	g := &a.settings.Graphics
	if a.cfg.Capture != nil || a.cfg.FPSProbe != nil || g.Detected != "" ||
		a.cfg.Hardware == nil || a.cfg.Settings.Save == nil {
		return
	}
	g.MarkChosenFromOldDefaults()
	a.applyDetectedTier(&a.settings)
	a.saveSettings()
}

// applyDetectedTier reads the hardware and applies its tier to s: the
// rows the player chose by hand are kept, and Detected remembers the tier.
// The caller has checked that there is a hardware reader.
func (a *App) applyDetectedTier(s *config.Settings) {
	info := a.cfg.Hardware()
	tier := config.DetectTier(info.Total, info.Cores)
	s.Graphics.ApplyTier(tier, a.deviceScale())
	s.Graphics.Detected = tier
}

// setSettings applies a change everywhere and stores it.
func (a *App) setSettings(s config.Settings) {
	a.settings = s.Normalize()
	a.applySettings()
	a.saveSettings()
}

// saveSettings stores the settings (a failure shows as a hint).
func (a *App) saveSettings() {
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
	applyGraphicsKnobs(r, s.Graphics)
	r.edgeScroll = s.Gameplay.EdgeScroll && a.cfg.Capture == nil
	r.smooth.Speed = float64(s.Gameplay.PanSpeed) / 100
	r.reducedFlashing = s.Accessibility.ReducedFlashing
	r.reducedMotion = s.Accessibility.ReducedMotion
	r.smooth.Instant = s.Accessibility.ReducedMotion // no eased pan or zoom
	if !s.Gameplay.DevMode && r.speed == speedUltra {
		r.speed = speedVeryFast // ultrafast is a developer tier
	}
}

// applyGraphicsKnobs sets the renderer's detail knobs from the graphics
// settings (PLAY-019): the cast-shadow pass, the normal-map lighting, the
// water ripple and the grass tufts per tile. Each is read live, so an
// Options change shows on the next frame.
func applyGraphicsKnobs(r *Renderer, g config.Graphics) {
	r.env.shadow.SetOff(!g.Shadows)
	r.lit.Off = !g.Lighting
	r.water.simple = g.Water == config.WaterSimple
	r.lod.grass.level = g.Grass
}
