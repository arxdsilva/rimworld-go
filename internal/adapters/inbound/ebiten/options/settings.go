package options

import (
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/sysinfo"
)

// Hooks connect the Options screen to storage (zero value: the defaults,
// nothing saved).
type Hooks struct {
	Load func() (config.Settings, error)
	Save func(config.Settings) error
}

// Detect is what first-launch graphics detection reads of the program.
type Detect struct {
	// Skip keeps the deterministic defaults (captures and the fps probe
	// neither detect nor save).
	Skip        bool
	Hardware    func() sysinfo.Info // nil: no reader, nothing detected
	DeviceScale func() float64
}

// LoadSettings reads the stored settings once at startup. A file that has
// no detected graphics tier yet (a first launch, or one from before
// PLAY-019) gets one from the hardware. It returns the settings and a
// hint to show ("" when there is none).
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
func LoadSettings(h Hooks, d Detect) (config.Settings, string) {
	if h.Load == nil {
		return config.Defaults(), ""
	}
	s, err := h.Load()
	s = s.Normalize()
	hint := ""
	if err != nil {
		hint = "settings: " + err.Error()
	}
	g := &s.Graphics
	if d.Skip || g.Detected != "" || d.Hardware == nil || h.Save == nil {
		return s, hint
	}
	g.MarkChosenFromOldDefaults()
	ApplyDetectedTier(&s, d.Hardware, d.DeviceScale)
	if msg := SaveSettings(h, s); msg != "" {
		hint = msg
	}
	return s, hint
}

// SaveSettings stores s and returns a hint when that failed.
func SaveSettings(h Hooks, s config.Settings) string {
	if h.Save == nil {
		return ""
	}
	if err := h.Save(s); err != nil {
		return "settings not saved: " + err.Error()
	}
	return ""
}

// ApplyDetectedTier reads the hardware and applies its tier to s: the
// rows the player chose by hand are kept, and Detected remembers the tier.
// The caller has checked that there is a hardware reader.
func ApplyDetectedTier(s *config.Settings, hardware func() sysinfo.Info, deviceScale func() float64) {
	info := hardware()
	tier := config.DetectTier(info.Total, info.Cores)
	s.Graphics.ApplyTier(tier, deviceScale())
	s.Graphics.Detected = tier
}
