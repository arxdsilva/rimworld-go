package creature

import (
	"image/color"
	"math"
	"reflect"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// Floating damage numbers (presentation only).

// Floater is a number drifting up from a hit.
type Floater struct {
	X, Y float64 // tiles
	Text string
	Clr  color.RGBA
	Born uint64
}

// FloaterLife is how long a floating number lasts, in frames.
const FloaterLife = 50

var (
	floatHit      = color.RGBA{255, 236, 210, 255}
	floatBuilding = color.RGBA{230, 190, 120, 255}
	floatMiss     = color.RGBA{190, 190, 190, 255}
	floatDodge    = color.RGBA{150, 210, 255, 255}
)

// traceDodged reports a blow the target dodged (TraceView.Dodged, CBT-3;
// read by name so the floater builds before the field exists).
func traceDodged(t game.TraceView) bool {
	f := reflect.ValueOf(&t).Elem().FieldByName("Dodged")
	return f.IsValid() && f.Kind() == reflect.Bool && f.Bool()
}

// FloatTrace starts a floating number for a newly seen attack.
func (c *Set) FloatTrace(v *render.View, t game.TraceView, seed uint64) {
	jx := float64(seed%7)/20 - 0.15
	f := Floater{X: float64(t.ToX) + 0.5 + jx, Y: float64(t.ToY) - 0.1, Born: v.Present}
	switch {
	case traceDodged(t):
		f.Text, f.Clr = "dodged", floatDodge
	case t.Hit && t.Damage > 0:
		f.Text, f.Clr = "-"+strconv.Itoa(t.Damage), floatHit
		if t.Building {
			f.Clr = floatBuilding
		}
	case !t.Hit && !t.Melee:
		f.Text, f.Clr = "miss", floatMiss
	default:
		return
	}
	*c.Floaters = append(*c.Floaters, f)
	if len(*c.Floaters) > 64 {
		*c.Floaters = (*c.Floaters)[len(*c.Floaters)-64:]
	}
}

// DrawFloaters draws and ages the floating numbers.
func (c *Set) DrawFloaters(v *render.View, screen *ebiten.Image, tile float64) {
	kept := (*c.Floaters)[:0]
	for _, f := range *c.Floaters {
		age := v.Present - f.Born
		if age >= FloaterLife {
			continue
		}
		kept = append(kept, f)
		rise := float64(age) / FloaterLife * 0.7
		sx, sy := v.WorldToScreenF(f.X, f.Y-rise)
		// the anchor is world space (physical); the label is UI (logical)
		ls := float32(gfx.UIScale())
		sx, sy = sx/ls, sy/ls
		// text scale follows the zoom, in half steps so few faces are cached
		k := math.Round(min(3, max(1.5, tile/36))*2) / 2
		st := gfx.TextStyle{Size: gfx.Small.Size * k, Bold: true, Align: gfx.AlignCenter}
		w := gfx.TextW(f.Text, st) + float32(6*k)
		h := gfx.LineStep(st)
		fade := float32(1)
		if age > FloaterLife/2 {
			fade = float32(FloaterLife-age) / float32(FloaterLife/2)
		}
		gfx.FillRect(screen, sx-w/2, sy-h/2, w, h, uikit.Shade(uint8(170*fade)), false)
		st.Color = color.NRGBA{f.Clr.R, f.Clr.G, f.Clr.B, uint8(255 * fade)}
		gfx.Text(screen, f.Text, float64(sx), float64(sy-h/2), st)
	}
	*c.Floaters = kept
}
