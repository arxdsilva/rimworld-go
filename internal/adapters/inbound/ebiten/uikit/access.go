package uikit

import "image/color"

// Accessibility settings the kit honours (OPT-017, OPT-018, OPT-019). The
// host copies them from the player's Interface settings into Kit.Access
// when they change; the kit never reads settings itself.

// Color vision names, the same strings as config.ColorVisions.
const (
	ColorVisionNormal = "normal"
	ColorVisionDeuter = "deuteranopia"
	ColorVisionProtan = "protanopia"
	ColorVisionTritan = "tritanopia"
)

// Access is the kit's view of the accessibility settings. The zero value is
// today's behaviour: a 400 ms tooltip delay, the normal palette, no
// confirmations.
type Access struct {
	// Set marks the fields below as the player's choice; unset, the
	// tooltip delay is the default (TipDelay frames).
	Set bool
	// TooltipDelayMS is the hover time before a tooltip shows; 0 is instant.
	TooltipDelayMS int
	// ColorVision names the palette variant ("" is normal).
	ColorVision string
	// Confirm asks before deconstructing and releasing an animal.
	Confirm bool
}

// TipFrames is the tooltip delay in frames at 60 updates per second.
func (a Access) TipFrames() int {
	if !a.Set {
		return TipDelay
	}
	return max(0, (a.TooltipDelayMS*60+500)/1000)
}

// Palette is the part of the chrome that colour-vision variants change: the
// status colours. Everything else is neutral grey or gold and stays.
type Palette struct {
	Danger, Good, Warning color.RGBA
}

// Palette is the palette of a.ColorVision.
func (a Access) Palette() Palette { return PaletteFor(a.ColorVision) }

// PaletteFor is the variant called name; an unknown name is normal.
//
// PLACEHOLDER VALUES: the three colour-vision variants below are a plain
// blue / orange / yellow remap so the switch can be seen and tested. The
// real values come from the art pass (assets/palettes/ui.yaml keys
// ui.<variant>.danger, ui.<variant>.good, ui.<variant>.warning); once those
// keys exist TestColorVisionVariantsMatchYAML pins them, and these literals
// are replaced by the YAML's values.
func PaletteFor(name string) Palette {
	normal := Palette{Danger: Danger, Good: Good, Warning: Warning}
	switch name {
	case ColorVisionDeuter, ColorVisionProtan:
		// red/green confusion: separate danger and good by hue and lightness.
		normal.Danger = color.RGBA{R: 0xFF, G: 0x8A, B: 0x3D, A: 0xFF}
		normal.Good = color.RGBA{R: 0x6C, G: 0xB4, B: 0xFF, A: 0xFF}
	case ColorVisionTritan:
		// blue/yellow confusion: keep danger red, move good to teal.
		normal.Good = color.RGBA{R: 0x4F, G: 0xE0, B: 0xD0, A: 0xFF}
		normal.Warning = color.RGBA{R: 0xFF, G: 0xD0, B: 0xE0, A: 0xFF}
	}
	return normal
}

// Tokens returns the palette's status colours by token name (the YAML keys
// without the variant prefix).
func (p Palette) Tokens() map[string]color.RGBA {
	return map[string]color.RGBA{"ui.danger": p.Danger, "ui.good": p.Good, "ui.warning": p.Warning}
}

// ConfirmIf runs onOK at once when confirmations are off, and after the
// player presses OK in a Confirm dialog when they are on.
func ConfirmIf(k *Kit, id, title, body, okLabel string, danger bool, onOK func()) {
	if !k.Access.Confirm {
		onOK()
		return
	}
	k.Confirm(id, title, body, okLabel, danger, onOK)
}
