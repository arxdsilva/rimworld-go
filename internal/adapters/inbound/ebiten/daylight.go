package ebiten

import (
	"image/color"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/lighting"
)

// skyOverlayMaxAlpha is the night-time veil. Keep it low so sprites
// stay readable; the grade is a hint, not a filter.
const skyOverlayMaxAlpha = 96

// skyOverlay is a presentation-only grade over the world, used only by
// the fallback (no art) renderer; the art path is lit per sprite by
// light.go (ART §6.1). Morning and afternoon stay clear. Evening and
// night raise alpha as outdoor light falls; hue follows the hour.
// HUD stays above it.
func skyOverlay(t clock.Tick) color.RGBA {
	hour := int(clock.HourOfDay(t))
	if hour >= clock.HoursPerDay {
		hour = clock.HoursPerDay - 1
	}
	switch clock.PeriodAt(hour) {
	case clock.Morning, clock.Afternoon:
		return color.RGBA{}
	}
	rgb := skyHue(clock.HourOfDay(t))
	level := float64(lighting.Outdoor(t))
	a := uint8((1 - level) * skyOverlayMaxAlpha)
	// color.RGBA is alpha-premultiplied: un-premultiplied sky RGB would
	// add light instead of tinting the scene darker.
	pm := func(v uint8) uint8 { return uint8(uint16(v) * uint16(a) / 255) }
	return color.RGBA{R: pm(rgb[0]), G: pm(rgb[1]), B: pm(rgb[2]), A: a}
}

func skyHue(h float64) [3]uint8 {
	stops := [...]struct {
		h   float64
		rgb [3]uint8
	}{
		{0, [3]uint8{48, 58, 82}},
		{5, [3]uint8{48, 58, 82}},
		{6.5, [3]uint8{120, 100, 118}},
		{8, [3]uint8{196, 176, 154}},
		{12, [3]uint8{255, 255, 255}},
		{16, [3]uint8{214, 198, 176}},
		{18, [3]uint8{176, 132, 112}},
		{20, [3]uint8{56, 54, 80}},
		{24, [3]uint8{48, 58, 82}},
	}
	if h <= stops[0].h {
		return stops[0].rgb
	}
	for i := 1; i < len(stops); i++ {
		if h <= stops[i].h {
			span := stops[i].h - stops[i-1].h
			if span <= 0 {
				return stops[i].rgb
			}
			t := (h - stops[i-1].h) / span
			return [3]uint8{
				lerpU8(stops[i-1].rgb[0], stops[i].rgb[0], t),
				lerpU8(stops[i-1].rgb[1], stops[i].rgb[1], t),
				lerpU8(stops[i-1].rgb[2], stops[i].rgb[2], t),
			}
		}
	}
	return stops[len(stops)-1].rgb
}

func lerpU8(a, b uint8, t float64) uint8 {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return uint8(float64(a) + (float64(b)-float64(a))*t)
}
