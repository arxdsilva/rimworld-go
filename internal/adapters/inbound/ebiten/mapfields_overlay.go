package ebiten

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Map fields overlay (MAPGEN-2 §6.2): developer mode only. The dev menu's
// "Map fields" entry cycles off and each generator field; the chosen layer
// is a world-space image over the map (alpha 0.7) with a legend panel.
// Read-only: it reads game.MapFieldLayers and never writes the game.

// mapFieldSource is the behaviour the overlay needs from the game.
type mapFieldSource interface {
	MapFieldLayers() ([]game.MapFieldLayer, error)
}

var _ mapFieldSource = (*game.Game)(nil)

// fieldAlpha is the overlay opacity (0.7 of 255).
const fieldAlpha = 178

// mapFieldsOverlay is the overlay's presentation state (part of devState,
// so leaving developer mode clears it).
type mapFieldsOverlay struct {
	layers []game.MapFieldLayer
	sel    int // index into layers; meaningful only while shown
	shown  bool
	img    *ebiten.Image // the selected layer, built on first draw
	builds int           // how many images were built (tests)
}

func (o *mapFieldsOverlay) on() bool { return o.shown }

// name is the selected layer's name, or "off".
func (o *mapFieldsOverlay) name() string {
	if !o.shown {
		return "off"
	}
	return o.layers[o.sel].Name
}

// cycle moves to the next layer (loading them when switching on) and back
// to off after the last one.
func (o *mapFieldsOverlay) cycle(src mapFieldSource) error {
	if !o.shown {
		layers, err := src.MapFieldLayers()
		if err != nil {
			return err
		}
		if len(layers) == 0 {
			return fmt.Errorf("no map field layers")
		}
		o.layers, o.sel, o.shown = layers, 0, true
	} else if o.sel++; o.sel >= len(o.layers) {
		o.shown, o.layers = false, nil
	}
	if o.img != nil {
		o.img.Deallocate()
		o.img = nil
	}
	return nil
}

// image is the selected layer as an image, built once per selection.
func (o *mapFieldsOverlay) image() *ebiten.Image {
	if !o.shown {
		return nil
	}
	if o.img == nil {
		l := o.layers[o.sel]
		o.img = ebiten.NewImage(l.W, l.H)
		o.img.WritePixels(fieldPixels(l))
		o.builds++
	}
	return o.img
}

// fieldPixels is the layer as premultiplied RGBA at fieldAlpha.
func fieldPixels(l game.MapFieldLayer) []byte {
	px := make([]byte, l.W*l.H*4)
	put := func(i int, r, g, b uint8) {
		px[i*4], px[i*4+1], px[i*4+2], px[i*4+3] =
			uint8(int(r)*fieldAlpha/255), uint8(int(g)*fieldAlpha/255), uint8(int(b)*fieldAlpha/255), fieldAlpha
	}
	for i := 0; i < l.W*l.H; i++ {
		switch {
		case l.Scalar != nil:
			r, g, b := scalarRamp(l.Scalar[i])
			put(i, r, g, b)
		case int(l.Classes[i]) < len(l.Legend):
			c := l.Legend[l.Classes[i]].RGBA
			put(i, c[0], c[1], c[2])
		default:
			put(i, 255, 0, 255)
		}
	}
	for _, m := range l.Marks {
		if m[0] >= 0 && m[1] >= 0 && m[0] < l.W && m[1] < l.H {
			put(m[1]*l.W+m[0], 255, 0, 255)
		}
	}
	return px
}

// scalarRamp maps [0, 1] from dark blue to bright yellow.
func scalarRamp(v float32) (r, g, b uint8) {
	v = max(0, min(1, v))
	return uint8(20 + 230*v), uint8(20 + 220*v), uint8(90 - 30*v)
}

// fieldLegendLines are the legend rows: one per class, or low/high for a
// scalar layer.
func fieldLegendLines(l game.MapFieldLayer) []string {
	if l.Scalar != nil {
		return []string{"low", "high"}
	}
	out := make([]string, len(l.Legend))
	for i, e := range l.Legend {
		out[i] = e.Label
	}
	return out
}

// mapFieldsAction is the dev menu entry that cycles the overlay.
func (a *App) mapFieldsAction() devAction {
	act := devAction{label: "Map fields: " + a.dev.fields.name()}
	src, ok := a.renderer.game.(mapFieldSource)
	if !ok {
		act.reason = "needs the full game"
		return act
	}
	act.run = func(devTools) string {
		if err := a.dev.fields.cycle(src); err != nil {
			return "Map fields: " + err.Error()
		}
		return "Map fields: " + a.dev.fields.name()
	}
	return act
}

// drawMapFields draws the selected layer over the map (physical pixels).
func (a *App) drawMapFields(screen *ebiten.Image) {
	if !a.devMode() || a.renderer == nil {
		return
	}
	img := a.dev.fields.image()
	if img == nil {
		return
	}
	tile := a.shell.TilePx()
	x, y := a.shell.WorldToScreen(0, 0)
	// one texel per cell, nearest-sampled so cells stay crisp at any zoom
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.GeoM.Scale(tile, tile)
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

// mapFieldLegendSize is the legend panel's size.
func (a *App) mapFieldLegendSize() (w, h float32) {
	o := &a.dev.fields
	if !o.shown {
		return 0, 0
	}
	l := o.layers[o.sel]
	lines := fieldLegendLines(l)
	title := "Map fields: " + l.Name
	return max(textW(title, tsLabel), maxTextW(lines, tsSmall)+22) + 16, float32(len(lines)*devStep + 30)
}

// drawMapFieldLegend is the legend panel (logical pixels, canvas helpers),
// first in the dev column (devmode.go).
func (a *App) drawMapFieldLegend(screen *ebiten.Image) {
	o := &a.dev.fields
	if !o.shown {
		return
	}
	r := a.devStack().Fields
	if r.empty() {
		return
	}
	l := o.layers[o.sel]
	lines := fieldLegendLines(l)
	title := "Map fields: " + l.Name
	x, y := r.X, r.Y
	drawPanel(screen, x, y, r.W, r.H)
	drawText(screen, title, float64(x)+8, float64(y)+3, tsLabel)
	for i, s := range lines {
		ry := y + 24 + float32(i*devStep)
		sw := float32(10)
		switch {
		case l.Scalar != nil:
			r, g, b := scalarRamp(float32(i))
			uiFillRect(screen, x+8, ry, sw, sw, rgba8(r, g, b), false)
		default:
			c := l.Legend[i].RGBA
			uiFillRect(screen, x+8, ry, sw, sw, rgba8(c[0], c[1], c[2]), false)
		}
		drawText(screen, s, float64(x)+22, float64(ry)-2, tsSmall)
	}
}

func rgba8(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 255} }
