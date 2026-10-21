package config

import (
	"encoding/json"
	"fmt"
	"io"
)

// SettingsVersion is the settings file format. Older files decode over
// the defaults, so new fields arrive with their default values.
const SettingsVersion = 1

// Settings are the player's preferences (Options screen).
type Settings struct {
	Version       int           `json:"version"`
	Audio         Audio         `json:"audio"`
	Graphics      Graphics      `json:"graphics"`
	Gameplay      Gameplay      `json:"gameplay"`
	Accessibility Accessibility `json:"accessibility"`
}

// Audio volumes are 0..100 in steps of VolumeStep.
type Audio struct {
	Master        int  `json:"master"`
	Music         int  `json:"music"`
	Effects       int  `json:"effects"`
	Ambience      int  `json:"ambience"`
	UI            int  `json:"ui"`
	MuteUnfocused bool `json:"mute_unfocused"`
}

// Effects detail levels.
const (
	EffectsFull    = "full"
	EffectsReduced = "reduced"
	EffectsMinimal = "minimal"
)

// EffectLevels is the cycle order of Graphics.Effects.
var EffectLevels = []string{EffectsFull, EffectsReduced, EffectsMinimal}

// Graphics are window and detail settings.
type Graphics struct {
	Fullscreen bool   `json:"fullscreen"`
	VSync      bool   `json:"vsync"`
	Effects    string `json:"effects"`
}

// AutosaveChoices are the autosave intervals in in-game days (0 = off).
var AutosaveChoices = []int{1, 3, 7, 0}

// Gameplay are conveniences that never change the simulation's rules.
type Gameplay struct {
	PauseOnThreat bool `json:"pause_on_threat"`
	AutosaveDays  int  `json:"autosave_days"`
	EdgeScroll    bool `json:"edge_scroll"`
	PanSpeed      int  `json:"pan_speed"` // percent, PanSpeedMin..PanSpeedMax
}

// Accessibility options.
type Accessibility struct {
	ReducedMotion   bool `json:"reduced_motion"`
	ReducedFlashing bool `json:"reduced_flashing"`
}

// Ranges and steps.
const (
	VolumeStep   = 10
	PanSpeedMin  = 50
	PanSpeedMax  = 200
	PanSpeedStep = 25
)

// Defaults are the settings of a fresh install.
func Defaults() Settings {
	return Settings{
		Version:  SettingsVersion,
		Audio:    Audio{Master: 80, Music: 60, Effects: 80, Ambience: 70, UI: 70, MuteUnfocused: true},
		Graphics: Graphics{VSync: true, Effects: EffectsFull},
		Gameplay: Gameplay{PauseOnThreat: true, AutosaveDays: 1, PanSpeed: 100},
	}
}

func snap(v, lo, hi, step int) int {
	v = max(lo, min(hi, v))
	return lo + (v-lo+step/2)/step*step
}

// Normalize clamps every value into range and replaces unknown choices
// with their defaults.
func (s Settings) Normalize() Settings {
	d := Defaults()
	s.Version = SettingsVersion
	for _, v := range []*int{&s.Audio.Master, &s.Audio.Music, &s.Audio.Effects, &s.Audio.Ambience, &s.Audio.UI} {
		*v = snap(*v, 0, 100, VolumeStep)
	}
	if !contains(EffectLevels, s.Graphics.Effects) {
		s.Graphics.Effects = d.Graphics.Effects
	}
	if !contains(AutosaveChoices, s.Gameplay.AutosaveDays) {
		s.Gameplay.AutosaveDays = d.Gameplay.AutosaveDays
	}
	s.Gameplay.PanSpeed = snap(s.Gameplay.PanSpeed, PanSpeedMin, PanSpeedMax, PanSpeedStep)
	return s
}

func contains[T comparable](xs []T, v T) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// Decode reads settings JSON over the defaults, so missing fields keep
// their default values, then normalizes.
func Decode(r io.Reader) (Settings, error) {
	s := Defaults()
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return Defaults(), fmt.Errorf("read settings: %w", err)
	}
	return s.Normalize(), nil
}

// Encode writes settings as indented JSON.
func Encode(w io.Writer, s Settings) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s.Normalize())
}

// Gain maps a 0..100 volume to a perceptual amplitude (0..1).
func Gain(v int) float64 {
	f := float64(max(0, min(100, v))) / 100
	return f * f
}

// FX is the particle multiplier for an effects level.
func (g Graphics) FX() float64 {
	switch g.Effects {
	case EffectsReduced:
		return 0.5
	case EffectsMinimal:
		return 0.2
	}
	return 1
}
