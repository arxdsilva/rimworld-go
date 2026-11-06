package sheet

import (
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game/dossierview"
)

// Bar colour styles (config.HUDNode.BarStyle).
const (
	BarFadeUp   = "fade_up"   // from the low colour when empty to the main colour when full
	BarLowOnly  = "low_only"  // always the low colour
	BarFadeDown = "fade_down" // the main colour, shading into the low colour below half
	BarMainOnly = "main_only" // always the main colour
)

// BarStyles lists the styles in menu order.
func BarStyles() []string { return []string{BarFadeUp, BarLowOnly, BarFadeDown, BarMainOnly} }

// ValidBarStyle reports whether s is a style.
func ValidBarStyle(s string) bool {
	switch s {
	case BarFadeUp, BarLowOnly, BarFadeDown, BarMainOnly:
		return true
	}
	return false
}

// BarStyleLabel is a style's name for the menu.
func BarStyleLabel(tr lang.Tr, s string) string {
	switch s {
	case BarFadeUp:
		return tr.T("Low shades up to main")
	case BarLowOnly:
		return tr.T("Low colour only")
	case BarFadeDown:
		return tr.T("Main shades down to low")
	case BarMainOnly:
		return tr.T("Main colour only")
	}
	return ""
}

// BarColor is the fill colour of a bar filled to frac (0..1) in a style; an
// unknown or empty style reads as BarMainOnly.
func BarColor(style string, frac float64, low, main color.RGBA) color.RGBA {
	frac = math.Max(0, math.Min(1, frac))
	switch style {
	case BarFadeUp:
		return Lerp(low, main, frac)
	case BarLowOnly:
		return low
	case BarFadeDown:
		if frac >= 0.5 {
			return main
		}
		return Lerp(low, main, frac*2)
	}
	return main
}

// Lerp blends a to b by t in 0..1, channel by channel.
func Lerp(a, b color.RGBA, t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), mix(a.A, b.A)}
}

// ParseColor reads "#RRGGBBAA" (the form HUD.Colors stores) or "#RRGGBB".
// The result is straight, not premultiplied, alpha.
func ParseColor(s string) (color.RGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 && len(s) != 8 {
		return color.RGBA{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	if len(s) == 6 {
		v = v<<8 | 0xFF
	}
	return color.RGBA{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}

// FormatColor writes "#RRGGBBAA", upper case.
func FormatColor(c color.RGBA) string {
	const hex = "0123456789ABCDEF"
	b := []byte{'#', 0, 0, 0, 0, 0, 0, 0, 0}
	for i, v := range [4]uint8{c.R, c.G, c.B, c.A} {
		b[1+2*i], b[2+2*i] = hex[v>>4], hex[v&15]
	}
	return string(b)
}

// RGBToHSL converts to hue in degrees [0,360), saturation and lightness in
// [0,1]. Alpha is not touched.
func RGBToHSL(c color.RGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (hi + lo) / 2
	d := hi - lo
	if d == 0 {
		return 0, 0, l
	}
	s = math.Min(1, d/(1-math.Abs(2*l-1)))
	switch hi {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

// HSLToRGB converts back; hue wraps, saturation and lightness clamp. The
// alpha is a.
func HSLToRGB(h, s, l float64, a uint8) color.RGBA {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	s, l = math.Max(0, math.Min(1, s)), math.Max(0, math.Min(1, l))
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch int(h / 60) {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	to8 := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return color.RGBA{to8(r), to8(g), to8(b), a}
}

// ToneKey is the colour key a tone is drawn with.
func ToneKey(t Tone) string {
	switch t {
	case ToneOff:
		return "text_off"
	case ToneCritical:
		return "tone_critical"
	case ToneWarning:
		return "tone_warning"
	case ToneNeutral:
		return "tone_neutral"
	case ToneGood:
		return "tone_good"
	case ToneTop:
		return "tone_top"
	}
	return "text"
}

// RelationKey is the colour key of how the colony sees someone.
func RelationKey(r dossierview.Relation) string {
	switch r {
	case dossierview.RelationOwn:
		return "side_own"
	case dossierview.RelationAlly:
		return "side_ally"
	case dossierview.RelationNone:
		return "side_free"
	case dossierview.RelationFoe:
		return "side_foe"
	case dossierview.RelationHeld:
		return "side_held"
	case dossierview.RelationWild:
		return "side_wild"
	}
	return "text"
}

// RelationTone is the tone a line about someone carries. Words on the line
// say the same thing (spec §59).
func RelationTone(r dossierview.Relation) Tone {
	switch r {
	case dossierview.RelationOwn:
		return ToneGood
	case dossierview.RelationAlly:
		return ToneTop
	case dossierview.RelationFoe:
		return ToneCritical
	case dossierview.RelationHeld:
		return ToneWarning
	case dossierview.RelationNone:
		return ToneNeutral
	}
	return ToneText
}

// MoodTipTone is the tone of one thought's value in the mood tooltip, cut at
// -10, 0 and 10.
func MoodTipTone(v int) Tone {
	switch {
	case v <= -10:
		return ToneCritical
	case v < 0:
		return ToneWarning
	case v < 10:
		return ToneNeutral
	}
	return ToneGood
}
