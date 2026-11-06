package sheet

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"

	"rimworld-go/internal/platform/config"
)

// PresetVersion is the file format's version. A file with a larger number
// is refused; a smaller one still loads and the caller may say it is old.
const PresetVersion = 1

// MaxPresetBytes caps a preset file.
const MaxPresetBytes = 256 << 10

// MaxPresetName is the longest preset name.
const MaxPresetName = 40

// Errors from the preset codec.
var (
	ErrPresetTooBig  = errors.New("the preset file is too big")
	ErrPresetNewer   = errors.New("the preset was made by a newer game")
	ErrPresetInvalid = errors.New("the preset file is not a card preset")
	ErrPresetName    = errors.New("that name cannot be used for a preset")
)

// presetWire is the file's shape.
type presetWire struct {
	Version  int             `json:"version"`
	Name     string          `json:"name"`
	Docked   *config.HUDNode `json:"docked,omitempty"`
	Floating *config.HUDNode `json:"floating,omitempty"`
	DockW    int             `json:"dock_w,omitempty"`
	DockH    int             `json:"dock_h,omitempty"`
	DockTabs int             `json:"dock_tabs,omitempty"`
	FloatW   int             `json:"float_w,omitempty"`
	FloatH   int             `json:"float_h,omitempty"`
	Text     *config.HUDText `json:"text,omitempty"`
}

// EncodePreset writes a preset as indented JSON, stamped with the current
// version.
func EncodePreset(p Preset) ([]byte, error) {
	w := presetWire{Version: PresetVersion, Name: p.Name, Docked: p.Docked, Floating: p.Floating,
		DockW: p.DockW, DockH: p.DockH, DockTabs: p.DockTabs, FloatW: p.FloatW, FloatH: p.FloatH, Text: p.Text}
	return json.MarshalIndent(w, "", "  ")
}

// DecodePreset reads a preset file. It never panics on bad input: oversized,
// malformed, newer-version, unknown-element and over-limit files come back as
// errors, and sizes are clamped to the ranges the settings allow.
func DecodePreset(data []byte) (Preset, error) {
	if len(data) > MaxPresetBytes {
		return Preset{}, ErrPresetTooBig
	}
	var w presetWire
	if err := json.Unmarshal(data, &w); err != nil {
		return Preset{}, fmt.Errorf("%w: %v", ErrPresetInvalid, err)
	}
	switch {
	case w.Version > PresetVersion:
		return Preset{}, ErrPresetNewer
	case w.Version < 1:
		return Preset{}, fmt.Errorf("%w: no version", ErrPresetInvalid)
	}
	for _, t := range []struct {
		name string
		n    *config.HUDNode
	}{{"docked", w.Docked}, {"floating", w.Floating}} {
		if t.n == nil {
			continue
		}
		if ps := Validate(lang.English{}, *t.n); len(ps) > 0 {
			return Preset{}, fmt.Errorf("%w: %s layout: %s: %s", ErrPresetInvalid, t.name, ps[0].Path, ps[0].Text)
		}
	}
	w.Docked, w.Floating = canonical(w.Docked), canonical(w.Floating)
	if len(w.Name) > MaxPresetName*4 {
		return Preset{}, fmt.Errorf("%w: name too long", ErrPresetInvalid)
	}
	p := Preset{Version: w.Version, Name: w.Name, Docked: w.Docked, Floating: w.Floating,
		DockW: clampOrZero(w.DockW, 72, 150), DockH: clampOrZero(w.DockH, config.HUDPaneMinH, config.HUDPaneMaxH),
		DockTabs: clampOrZero(w.DockTabs, 6, 12), FloatW: clampOrZero(w.FloatW, 100, 1000),
		FloatH: clampOrZero(w.FloatH, 100, 1000)}
	if w.Text != nil {
		t := clampText(*w.Text)
		p.Text = &t
	}
	return p, nil
}

// canonical makes empty child lists nil, so a tree equals its own file.
func canonical(n *config.HUDNode) *config.HUDNode {
	if n == nil {
		return nil
	}
	c := walk(*n, func(x config.HUDNode) config.HUDNode {
		if len(x.Kids) == 0 {
			x.Kids = nil
		}
		return x
	})
	return &c
}

func clampOrZero(v, lo, hi int) int {
	if v == 0 {
		return 0
	}
	return min(max(v, lo), hi)
}

func clampText(t config.HUDText) config.HUDText {
	cl := func(v, lo, hi int) int { return min(max(v, lo), hi) }
	t.BodySize = cl(t.BodySize, 7, 20)
	t.BodyLinePct = cl(t.BodyLinePct, config.HUDLineMinPct, 250)
	t.LargeDelta = cl(t.LargeDelta, 0, 24)
	t.LargeLinePct = cl(t.LargeLinePct, config.HUDLineMinPct, 250)
	t.SmallDelta = cl(t.SmallDelta, -5, 0)
	t.SmallLinePct = cl(t.SmallLinePct, config.HUDLineMinPct, 250)
	t.LabelW = cl(t.LabelW, 30, 150)
	t.ValueW = cl(t.ValueW, 30, 150)
	return t
}

// IsOlder reports whether a preset was written by an older format.
func IsOlder(p Preset) bool { return p.Version < PresetVersion }

// BuiltInNames are the names players cannot save over.
func BuiltInNames() []string { return []string{"Standard", "Roomy", "Snug", "Snug with traits"} }

// BuiltIn finds a built-in preset by name (case-insensitive).
func BuiltIn(name string) (Preset, bool) {
	for _, p := range BuiltIns() {
		if strings.EqualFold(p.Name, strings.TrimSpace(name)) {
			return p, true
		}
	}
	return Preset{}, false
}

// BuiltIns are the presets that ship with the game, in menu order. Standard
// is the default look; Roomy shows every row for every target at a larger
// size; Snug is smaller and narrows some rows by target; Snug with traits
// adds a traits row. Each docked tree fits its own pane height.
func BuiltIns() []Preset {
	def := config.Defaults().HUD
	docked, floating := DefaultDocked(), DefaultFloating()
	std := def.Text
	roomy := std
	roomy.BodySize, roomy.LabelW, roomy.ValueW, roomy.BodyLinePct = 13, 110, 40, 100
	snug := std
	snug.BodySize, snug.LargeDelta, snug.SmallDelta, snug.LabelW, snug.ValueW, snug.BodyLinePct = 10, 3, -1, 80, 32, 88
	narrow := map[string]string{"hunger_bar": "cp", "rest_bar": "c", "doing": "cp"}
	snugD, snugF := narrowed(docked, narrow), narrowed(floating, narrow)
	return []Preset{
		{Version: PresetVersion, Name: "Standard", Docked: &docked, Floating: &floating,
			DockW: def.Pane.TabW, DockH: def.Pane.Height, DockTabs: def.Pane.MinTabs, FloatW: def.Card.W, FloatH: def.Card.H, Text: &std},
		{Version: PresetVersion, Name: "Roomy", Docked: ptr(roomyDocked()), Floating: ptr(everyTarget(floating)),
			DockW: 96, DockH: config.HUDPaneMaxH, DockTabs: 7, FloatW: 460, FloatH: 560, Text: &roomy},
		{Version: PresetVersion, Name: "Snug", Docked: &snugD, Floating: &snugF,
			DockW: 80, DockH: 152, DockTabs: 6, FloatW: 340, FloatH: 400, Text: &snug},
		{Version: PresetVersion, Name: "Snug with traits", Docked: ptr(withTraits(snugD)), Floating: &snugF,
			DockW: 80, DockH: 168, DockTabs: 6, FloatW: 340, FloatH: 420, Text: &snug},
	}
}

func ptr(n config.HUDNode) *config.HUDNode { return &n }

// ValidName reports why a name cannot be used for a player's preset, or nil.
// A name has 1 to MaxPresetName letters, digits, spaces, '-' or '_', and is
// not a built-in's.
func ValidName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > MaxPresetName {
		return fmt.Errorf("%w: use 1 to %d characters", ErrPresetName, MaxPresetName)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == ' ':
		default:
			return fmt.Errorf("%w: only letters, digits, spaces, - and _", ErrPresetName)
		}
	}
	if _, ok := BuiltIn(name); ok {
		return fmt.Errorf("%w: that is a built-in", ErrPresetName)
	}
	if Slug(name) == "" {
		return fmt.Errorf("%w: it needs a letter or digit", ErrPresetName)
	}
	return nil
}

// Slug is the file name for a preset name: lower case, spaces as '-'.
// Letters, digits, '-' and '_' stay; anything else is dropped.
func Slug(name string) string {
	var b strings.Builder
	hasAlnum := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			hasAlnum = true
		case r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	if !hasAlnum {
		return ""
	}
	return b.String()
}

// Capture takes the parts of the current look that the flags name.
func Capture(h config.HUD, name string, f SaveFlags) Preset {
	p := Preset{Version: PresetVersion, Name: strings.TrimSpace(name)}
	if f.Docked {
		n := dockedOrDefault(h)
		p.Docked = &n
	}
	if f.Floating {
		n := floatingOrDefault(h)
		p.Floating = &n
	}
	if f.Width {
		p.DockW, p.FloatW = h.Pane.TabW, h.Card.W
	}
	if f.Height {
		p.DockH, p.FloatH = h.Pane.Height, h.Card.H
	}
	if f.Tabs {
		p.DockTabs = h.Pane.MinTabs
	}
	if f.Text {
		t := h.Text
		p.Text = &t
	}
	return p
}

func dockedOrDefault(h config.HUD) config.HUDNode {
	if h.Layouts.Docked != nil {
		return *h.Layouts.Docked
	}
	return DefaultDocked()
}

func floatingOrDefault(h config.HUD) config.HUDNode {
	if h.Layouts.Floating != nil {
		return *h.Layouts.Floating
	}
	return DefaultFloating()
}

// Apply loads a preset into the settings: only what the preset holds changes
// (DockW is the pane's tab width: the pane is as wide as its tabs).
// A tree equal to the built-in one is stored as nil. The current-preset name
// is set to the preset's.
func Apply(p Preset, h *config.HUD) {
	if p.Docked != nil {
		h.Layouts.Docked = unlessDefault(*p.Docked, DefaultDocked())
	}
	if p.Floating != nil {
		h.Layouts.Floating = unlessDefault(*p.Floating, DefaultFloating())
	}
	if p.DockW > 0 {
		h.Pane.TabW = p.DockW
	}
	if p.DockH > 0 {
		h.Pane.Height = p.DockH
	}
	if p.DockTabs > 0 {
		h.Pane.MinTabs = p.DockTabs
	}
	if p.FloatW > 0 {
		h.Card.W = p.FloatW
	}
	if p.FloatH > 0 {
		h.Card.H = p.FloatH
	}
	if p.Text != nil {
		h.Text = *p.Text
	}
	h.Preset = p.Name
}

func unlessDefault(n, def config.HUDNode) *config.HUDNode {
	if reflect.DeepEqual(n, def) {
		return nil
	}
	return &n
}
