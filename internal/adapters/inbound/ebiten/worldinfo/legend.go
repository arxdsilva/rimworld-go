package worldinfo

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
)

// Legend metrics (px).
const (
	LegendW    = 200
	legendRowH = 18
	legendPad  = 8
	legendKeyH = float32(int(planet.NumRoadClasses)+4) * legendRowH // road classes and river sizes
)

// LegendHeight is the legend's height for a view mode.
func LegendHeight(m globe.Mode) float32 {
	body := float32(46) // a data ramp: the bar and its end labels
	switch m {
	case globe.ModeBiome:
		body = float32(len(biome.All())) * legendRowH
	case globe.ModeHilliness:
		body = float32(len(globe.HillLegend())) * legendRowH
	}
	return 2*legendPad + body + legendKeyH + 6
}

func rgba(c [3]float32) color.RGBA {
	return color.RGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), 255}
}

// DrawLegend paints the legend for mode m into the rectangle with the
// top-right corner at (right, y) and returns it: biome swatches, the data
// ramp with its unit, or the hilliness swatches, then every road class and
// river size as the globe draws them. Temperatures read in u.
func DrawLegend(k *uikit.Kit, right, y float32, m globe.Mode, u units.Temp) gfx.Rect {
	r := gfx.Rect{X: right - LegendW, Y: y, W: LegendW, H: LegendHeight(m)}
	k.Queue(func(dst *ebiten.Image) {
		uikit.DrawPanel(dst, r.X, r.Y, r.W, r.H)
		st := gfx.Small
		cy := r.Y + legendPad
		swatch := func(c color.RGBA, name string) {
			uikit.FillRect(dst, gfx.Rect{X: r.X + 10, Y: cy + 2, W: 14, H: 12}, c)
			gfx.Text(dst, name, float64(r.X+32), float64(cy), st)
			cy += legendRowH
		}
		switch m {
		case globe.ModeBiome:
			for _, d := range biome.All() {
				swatch(rgba(globe.PalRGB(d.Key)), d.Name)
			}
		case globe.ModeHilliness:
			for _, e := range globe.HillLegend() {
				swatch(e.Colour, e.Label)
			}
		default:
			drawRamp(dst, gfx.Rect{X: r.X + 12, Y: cy + 4, W: r.W - 24, H: 16}, m, u)
			cy += 46
		}
		cy += 6
		line := func(e globe.LegendEntry) {
			h := max(1, 2*e.Width)
			uikit.FillRect(dst, gfx.Rect{X: r.X + 10, Y: cy + 8 - h/2, W: 16, H: h}, e.Colour)
			gfx.Text(dst, e.Label, float64(r.X+32), float64(cy), st)
			cy += legendRowH
		}
		for _, e := range globe.RoadLegend() {
			line(e)
		}
		for _, e := range globe.RiverLegend() {
			line(e)
		}
	})
	return r
}

// drawRamp paints a data mode's colour ramp with its end values.
func drawRamp(dst *ebiten.Image, bar gfx.Rect, m globe.Mode, u units.Temp) {
	var stops []globe.ColourStop
	var lo, hi string
	switch m {
	case globe.ModeTemperature:
		stops = globe.TempRamp
	case globe.ModeRainfall:
		stops = globe.RainRamp
	default:
		stops = globe.ElevRamp
	}
	a, b := stops[0].At, stops[len(stops)-1].At
	switch m {
	case globe.ModeTemperature:
		lo, hi = u.Whole(float64(a)), u.Whole(float64(b))
	case globe.ModeRainfall:
		lo, hi = fmt.Sprintf("%.0f mm", a), fmt.Sprintf("%.0f mm", b)
	default:
		lo, hi = fmt.Sprintf("%.0f m", a), fmt.Sprintf("%.0f m", b)
	}
	for x := 0; x < int(bar.W); x++ {
		c := globe.Ramp(stops, a+(b-a)*float32(x)/bar.W)
		uikit.FillRect(dst, gfx.Rect{X: bar.X + float32(x), Y: bar.Y, W: 1, H: bar.H}, rgba(c))
	}
	uikit.StrokeRect(dst, bar, uikit.PanelBorder)
	ls := gfx.Small
	ls.Color = uikit.TextDim
	gfx.Text(dst, lo, float64(bar.X), float64(bar.Bottom()+4), ls)
	ls.Align = gfx.AlignRight
	gfx.Text(dst, hi, float64(bar.Right()), float64(bar.Bottom()+4), ls)
}
