package devtools

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// Map fields overlay (MAPGEN-2 §6.2): developer mode only. The dev menu's
// "Map fields" entry cycles off and each generator field; the chosen layer
// is a world-space image over the map (alpha 0.7) with a legend panel.
// Read-only: it reads game.MapFieldLayers and never writes the game.

// FieldSource is the behaviour the overlay needs from the game.
type FieldSource interface {
	MapFieldLayers() ([]game.MapFieldLayer, error)
}

var _ FieldSource = (*game.Game)(nil)

// FieldAlpha is the overlay opacity (0.7 of 255).
const FieldAlpha = 178

// FieldsOverlay is the overlay's presentation state (part of State, so
// leaving developer mode clears it).
type FieldsOverlay struct {
	Layers []game.MapFieldLayer
	Sel    int // index into Layers; meaningful only while Shown
	Shown  bool
	Img    *ebiten.Image // the selected layer, built on first draw
	Builds int           // how many images were built (tests)
}

// On reports a layer is shown.
func (o *FieldsOverlay) On() bool { return o.Shown }

// Name is the selected layer's name, or "off".
func (o *FieldsOverlay) Name() string {
	if !o.Shown {
		return "off"
	}
	return o.Layers[o.Sel].Name
}

// Cycle moves to the next layer (loading them when switching on) and back
// to off after the last one.
func (o *FieldsOverlay) Cycle(src FieldSource) error {
	if !o.Shown {
		layers, err := src.MapFieldLayers()
		if err != nil {
			return err
		}
		if len(layers) == 0 {
			return fmt.Errorf("no map field layers")
		}
		o.Layers, o.Sel, o.Shown = layers, 0, true
	} else if o.Sel++; o.Sel >= len(o.Layers) {
		o.Shown, o.Layers = false, nil
	}
	if o.Img != nil {
		o.Img.Deallocate()
		o.Img = nil
	}
	return nil
}

// Image is the selected layer as an image, built once per selection.
func (o *FieldsOverlay) Image() *ebiten.Image {
	if !o.Shown {
		return nil
	}
	if o.Img == nil {
		l := o.Layers[o.Sel]
		o.Img = ebiten.NewImage(l.W, l.H)
		o.Img.WritePixels(FieldPixels(l))
		o.Builds++
	}
	return o.Img
}

// FieldPixels is the layer as premultiplied RGBA at FieldAlpha.
func FieldPixels(l game.MapFieldLayer) []byte {
	px := make([]byte, l.W*l.H*4)
	put := func(i int, r, g, b uint8) {
		px[i*4], px[i*4+1], px[i*4+2], px[i*4+3] =
			uint8(int(r)*FieldAlpha/255), uint8(int(g)*FieldAlpha/255), uint8(int(b)*FieldAlpha/255), FieldAlpha
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

// FieldLegendLines are the legend rows: one per class, or low/high for a
// scalar layer.
func FieldLegendLines(l game.MapFieldLayer) []string {
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
func (p *Panel) mapFieldsAction() Action {
	act := Action{Label: "Map fields: " + p.State.Fields.Name()}
	src, ok := p.sh.Env().Game.(FieldSource)
	if !ok {
		act.Reason = "needs the full game"
		return act
	}
	act.Run = func(Tools) string {
		if err := p.State.Fields.Cycle(src); err != nil {
			return "Map fields: " + err.Error()
		}
		return "Map fields: " + p.State.Fields.Name()
	}
	return act
}

// DrawMapFields draws the selected layer over the map (physical pixels).
func (p *Panel) DrawMapFields(screen *ebiten.Image) {
	if !p.active() {
		return
	}
	img := p.State.Fields.Image()
	if img == nil {
		return
	}
	tile := p.sh.TilePx()
	x, y := p.sh.WorldToScreen(0, 0)
	// one texel per cell, nearest-sampled so cells stay crisp at any zoom
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.GeoM.Scale(tile, tile)
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

// legendSize is the legend panel's size.
func (p *Panel) legendSize() (w, h float32) {
	o := &p.State.Fields
	if !o.Shown {
		return 0, 0
	}
	l := o.Layers[o.Sel]
	lines := FieldLegendLines(l)
	title := "Map fields: " + l.Name
	return max(gfx.TextW(title, gfx.Label), gfx.MaxTextW(lines, gfx.Small)+22) + 16, float32(len(lines)*devStep + 30)
}

// DrawMapFieldLegend is the legend panel (logical pixels, canvas helpers),
// first in the dev column (layout.go).
func (p *Panel) DrawMapFieldLegend(screen *ebiten.Image) {
	o := &p.State.Fields
	if !o.Shown {
		return
	}
	r := p.Stack().Fields
	if r.Empty() {
		return
	}
	l := o.Layers[o.Sel]
	lines := FieldLegendLines(l)
	title := "Map fields: " + l.Name
	x, y := r.X, r.Y
	uikit.DrawPanel(screen, x, y, r.W, r.H)
	gfx.Text(screen, title, float64(x)+8, float64(y)+3, gfx.Label)
	for i, s := range lines {
		ry := y + 24 + float32(i*devStep)
		sw := float32(10)
		switch {
		case l.Scalar != nil:
			r, g, b := scalarRamp(float32(i))
			gfx.FillRect(screen, x+8, ry, sw, sw, rgba8(r, g, b), false)
		default:
			c := l.Legend[i].RGBA
			gfx.FillRect(screen, x+8, ry, sw, sw, rgba8(c[0], c[1], c[2]), false)
		}
		gfx.Text(screen, s, float64(x)+22, float64(ry)-2, gfx.Small)
	}
}

func rgba8(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 255} }
