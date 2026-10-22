package config

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
	Controls      Controls      `json:"controls"`
}

// Controls are the player's key bindings: action id -> binding strings
// ("Shift+R"), only where they differ from the defaults (package input
// owns the action table, the defaults and the full normalization). An
// empty list unbinds the action.
type Controls struct {
	Bindings map[string][]string `json:"bindings,omitempty"`
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
	// DevMode turns on the developer keys: ultrafast speed, single-tick
	// stepping, debug actions, the debug log and the debug inspector.
	DevMode bool `json:"dev_mode"`
	// NG-1: the last new-game choices (pre-filled next time), whether a new
	// colony starts running instead of paused, and birthday letters.
	LastStoryteller string `json:"last_storyteller,omitempty"`
	LastDifficulty  string `json:"last_difficulty,omitempty"`
	LastSaveMode    string `json:"last_save_mode,omitempty"`
	RunOnStart      bool   `json:"run_on_start,omitempty"`
	BirthdayLetters bool   `json:"birthday_letters,omitempty"`
}

// Accessibility options.
type Accessibility struct {
	ReducedMotion   bool `json:"reduced_motion"`
	ReducedFlashing bool `json:"reduced_flashing"`
	// UIScale is the text and UI scale in percent (UIScaleMin..UIScaleMax; 0 = 100).
	UIScale int `json:"ui_scale,omitempty"`
}

// Ranges and steps.
const (
	VolumeStep   = 10
	PanSpeedMin  = 50
	PanSpeedMax  = 200
	PanSpeedStep = 25
)

// Defaults are the settings of a fresh install.
// UI scale range (NG-1 §1.1).
const (
	UIScaleMin  = 90
	UIScaleMax  = 130
	UIScaleStep = 5
)

func Defaults() Settings {
	return Settings{
		Version:       SettingsVersion,
		Audio:         Audio{Master: 80, Music: 60, Effects: 80, Ambience: 70, UI: 70, MuteUnfocused: true},
		Graphics:      Graphics{VSync: true, Effects: EffectsFull},
		Gameplay:      Gameplay{PauseOnThreat: true, AutosaveDays: 1, PanSpeed: 100},
		Accessibility: Accessibility{UIScale: 100},
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
	if s.Accessibility.UIScale == 0 {
		s.Accessibility.UIScale = 100
	}
	s.Accessibility.UIScale = snap(s.Accessibility.UIScale, UIScaleMin, UIScaleMax, UIScaleStep)
	s.Controls = s.Controls.normalize()
	return s
}

// normalize trims binding strings, drops empty ones and duplicates, and
// copies the map so callers never share it.
func (c Controls) normalize() Controls {
	if len(c.Bindings) == 0 {
		return Controls{}
	}
	out := make(map[string][]string, len(c.Bindings))
	for id, keys := range c.Bindings {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		clean := []string{}
		for _, k := range keys {
			if k = strings.TrimSpace(k); k != "" && !contains(clean, k) {
				clean = append(clean, k)
			}
		}
		out[id] = clean
	}
	return Controls{Bindings: out}
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
