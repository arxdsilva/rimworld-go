package game

import (
	"log"
	"slices"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/outbound/settingsfile"
	"rimworld-go/internal/platform/config"
)

// devGraphics are the dev flags that override the Graphics settings for
// one run: -hidpi and -vsync when given, -render-scale when not 0, and
// the -graphics tier when not "".
type devGraphics struct {
	hidpi, vsync *bool
	renderScale  float64
	tier         string
	windowed     bool // an fps probe: never fullscreen (see Run)
}

func (d devGraphics) any() bool {
	return d.hidpi != nil || d.vsync != nil || d.renderScale != 0 || d.tier != "" || d.windowed
}

// settingsHooks wires the Options to the per-user settings file. Captures
// get none, so screenshots always use the default settings.
// A dev override (devGraphics) applies to Graphics for this run; settings
// are then not saved, so the override never persists.
func settingsHooks(capturing bool, dev devGraphics, screen scaler) uiadapter.SettingsHooks {
	tier := dev.tier
	if tier != "" && !slices.Contains([]string{config.QualityHigh, config.QualityMedium, config.QualityLow}, tier) {
		log.Fatalf("-graphics %q: want high, medium or low", tier)
	}
	if dev.renderScale != 0 && !slices.Contains(config.RenderScales, dev.renderScale) {
		log.Fatalf("-render-scale %g: want one of %v", dev.renderScale, config.RenderScales)
	}
	hooks := uiadapter.SettingsHooks{} // captures, or no settings folder: the defaults
	if !capturing {
		if store, err := settingsfile.Default(); err != nil {
			log.Printf("settings not saved: %v", err)
		} else {
			hooks = uiadapter.SettingsHooks{Load: store.Load, Save: store.Save}
		}
	}
	if !dev.any() {
		return hooks
	}
	// A developer override applies for this run only: no Save, so nothing
	// is stored and the hardware tier is not detected over it.
	load := hooks.Load
	return uiadapter.SettingsHooks{Load: func() (config.Settings, error) {
		s, err := config.Defaults(), error(nil)
		if load != nil {
			s, err = load()
		}
		if tier != "" {
			s.Graphics.Chosen = nil
			s.Graphics.ApplyTier(tier, screen.DeviceScale()) // Medium's HiDPI and render-scale rows read the screen
			s.Graphics.Detected = tier
		}
		if dev.hidpi != nil {
			s.Graphics.HiDPI = *dev.hidpi
		}
		if dev.vsync != nil {
			s.Graphics.VSync = *dev.vsync
		}
		if dev.renderScale != 0 {
			s.Graphics.RenderScale = dev.renderScale
		}
		if dev.windowed {
			s.Graphics.Fullscreen = false
		}
		return s, err
	}}
}
