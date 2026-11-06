package uikit

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
)

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
	// Tr is the catalog the kit's own words ("Close", "Cancel") go
	// through (nil: English).
	Tr lang.Tr
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

// The colour-vision variants of the status colours (OPT-018): Tokens
// carries them as ui.<variant>.{danger,good,warning}, the keys of
// assets/palettes/ui.yaml. Deuteranopia and protanopia share one set,
// checked to stay apart under both (.agents/handoffs/OPT-2-art.md).
var (
	redGreenPalette = Palette{ // coral, sky blue, lemon
		Danger:  color.RGBA{R: 0xEA, G: 0x9E, B: 0x94, A: 0xFF},
		Good:    color.RGBA{R: 0x8E, G: 0xC4, B: 0xFB, A: 0xFF},
		Warning: color.RGBA{R: 0xF8, G: 0xEA, B: 0x52, A: 0xFF},
	}
	tritanPalette = Palette{ // coral, teal, pale peach
		Danger:  color.RGBA{R: 0xFF, G: 0x90, B: 0x80, A: 0xFF},
		Good:    color.RGBA{R: 0x62, G: 0xD6, B: 0xC0, A: 0xFF},
		Warning: color.RGBA{R: 0xFF, G: 0xDD, B: 0xB0, A: 0xFF},
	}
)

// PaletteFor is the variant called name; an unknown name is normal.
func PaletteFor(name string) Palette {
	switch name {
	case ColorVisionDeuter, ColorVisionProtan:
		return redGreenPalette
	case ColorVisionTritan:
		return tritanPalette
	}
	return Palette{Danger: Danger, Good: Good, Warning: Warning}
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
