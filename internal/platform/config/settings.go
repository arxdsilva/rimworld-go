package config

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SettingsVersion is the settings file format. Older files decode over
// the defaults, so new fields arrive with their default values.
// Version 2 (PLAY-012) added Accessibility.UIScaleAuto and Graphics.HiDPI;
// Normalize migrates anything older with both on.
// Version 3 (PLAY-017) added Gameplay.RunInBackground; older files decode
// over the defaults, so it arrives off.
// Version 4 (PLAY-018) added the learning helper (ShowLearningHelper,
// LearnCollapsed, LearnedLessons), CategorizedResources and
// ShowColonistBar; Normalize turns the helper and the bar on for anything
// older.
// Version 5 (PLAY-019) added the graphics quality preset and its knobs
// (Quality, Shadows, Lighting, Water, Grass) plus Detected and Chosen
// (graphics.go); an older file keeps today's look, with Detected empty so
// the next start detects the hardware tier.
const SettingsVersion = 5

// MaxLearnedLessons caps the learning helper's done list.
const MaxLearnedLessons = 64

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
	// HiDPI renders at the screen's true resolution (device scale factor)
	// instead of upscaling; it is presentation-only.
	HiDPI bool `json:"hidpi"`
	// PLAY-019 (graphics.go): the quality preset, the detail knobs it sets,
	// the tier detected from the hardware ("" = not detected yet: detect at
	// the next start) and the rows the player changed by hand.
	Quality  string   `json:"quality"`
	Shadows  bool     `json:"shadows"`
	Lighting bool     `json:"lighting"`
	Water    string   `json:"water"`
	Grass    string   `json:"grass"`
	Detected string   `json:"detected,omitempty"`
	Chosen   []string `json:"chosen,omitempty"`
	// PERF-002: RenderScale is the share of the physical pixels the map
	// draws at on a high-density screen (one of RenderScales; the HUD and
	// text stay native), and PerfOverlay shows the in-game frame-rate box.
	RenderScale float64 `json:"render_scale,omitempty"`
	PerfOverlay bool    `json:"perf_overlay,omitempty"`
}

// AutosaveChoices are the autosave intervals in in-game days (0 = off).
var AutosaveChoices = []int{1, 3, 7, 0}

// Gameplay are conveniences that never change the simulation's rules.
type Gameplay struct {
	PauseOnThreat bool `json:"pause_on_threat"`
	AutosaveDays  int  `json:"autosave_days"`
	EdgeScroll    bool `json:"edge_scroll"`
	PanSpeed      int  `json:"pan_speed"` // percent, PanSpeedMin..PanSpeedMax
	// RunInBackground keeps the colony running while the window is in the
	// background or minimised. Off (the default): the game pauses until
	// the player comes back.
	RunInBackground bool `json:"run_in_background"`
	// PLAY-018: the learning helper (shown, collapsed, the lessons done),
	// the resource list's grouping and whether the colonist bar shows.
	ShowLearningHelper   bool     `json:"show_learning_helper"`
	LearnCollapsed       bool     `json:"learn_collapsed,omitempty"`
	LearnedLessons       []string `json:"learned_lessons,omitempty"`
	CategorizedResources bool     `json:"categorized_resources,omitempty"`
	ShowColonistBar      bool     `json:"show_colonist_bar"`
	// RAMNoticeSeen is the total RAM in bytes the low-RAM notice was
	// dismissed for (PLAY-019; 0 = never): it shows again only on a
	// different amount.
	RAMNoticeSeen uint64 `json:"ram_notice_seen,omitempty"`
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
	// UIScale is the UI scale in percent, one of UIScalePresets (0 = 100).
	// It applies when UIScaleAuto is off.
	UIScale int `json:"ui_scale,omitempty"`
	// UIScaleAuto sizes the UI from the window (PLAY-012 B.1).
	UIScaleAuto bool `json:"ui_scale_auto"`
}

// Ranges and steps.
const (
	VolumeStep   = 10
	PanSpeedMin  = 50
	PanSpeedMax  = 200
	PanSpeedStep = 25
)

// UI scale range (PLAY-012 B.1): a fixed scale is one of the presets.
const (
	UIScaleMin = 75
	UIScaleMax = 200
)

// UIScalePresets are the fixed UI scales in percent, ascending.
var UIScalePresets = []int{75, 90, 100, 115, 130, 150, 175, 200}

// snapPreset is the preset nearest v (the lower one on a tie).
func snapPreset(v int) int {
	best := UIScalePresets[0]
	for _, p := range UIScalePresets {
		if abs(p-v) < abs(best-v) {
			best = p
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Defaults are the settings of a fresh install.
func Defaults() Settings {
	return Settings{
		Version: SettingsVersion,
		Audio:   Audio{Master: 80, Music: 60, Effects: 80, Ambience: 70, UI: 70, MuteUnfocused: true},
		Graphics: Graphics{VSync: true, Effects: EffectsFull, HiDPI: true, Quality: QualityHigh,
			Shadows: true, Lighting: true, Water: WaterAnimated, Grass: GrassFull, RenderScale: 1},
		Gameplay:      Gameplay{PauseOnThreat: true, AutosaveDays: 1, PanSpeed: 100, ShowLearningHelper: true, ShowColonistBar: true},
		Accessibility: Accessibility{UIScale: 100, UIScaleAuto: true},
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
	if s.Version < 2 { // before PLAY-012: no Auto or HiDPI field, both default on
		s.Accessibility.UIScaleAuto = true
		s.Graphics.HiDPI = true
	}
	if s.Version < 4 { // before PLAY-018: no helper or bar switch, both on
		s.Gameplay.ShowLearningHelper = true
		s.Gameplay.ShowColonistBar = true
	}
	if s.Version < 5 { // before PLAY-019: today's look; Detected stays "" so the next start detects
		s.Graphics.Shadows, s.Graphics.Lighting = true, true
	}
	s.Version = SettingsVersion
	s.Gameplay.LearnedLessons = dedupeLessons(s.Gameplay.LearnedLessons)
	for _, v := range []*int{&s.Audio.Master, &s.Audio.Music, &s.Audio.Effects, &s.Audio.Ambience, &s.Audio.UI} {
		*v = snap(*v, 0, 100, VolumeStep)
	}
	if !contains(EffectLevels, s.Graphics.Effects) {
		s.Graphics.Effects = d.Graphics.Effects
	}
	s.Graphics = s.Graphics.normalizeGraphics(d.Graphics)
	if !contains(AutosaveChoices, s.Gameplay.AutosaveDays) {
		s.Gameplay.AutosaveDays = d.Gameplay.AutosaveDays
	}
	s.Gameplay.PanSpeed = snap(s.Gameplay.PanSpeed, PanSpeedMin, PanSpeedMax, PanSpeedStep)
	if s.Accessibility.UIScale == 0 {
		s.Accessibility.UIScale = 100
	}
	s.Accessibility.UIScale = snapPreset(s.Accessibility.UIScale)
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

// dedupeLessons keeps the first of each lesson id, at most
// MaxLearnedLessons of them.
func dedupeLessons(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] && len(out) < MaxLearnedLessons {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
