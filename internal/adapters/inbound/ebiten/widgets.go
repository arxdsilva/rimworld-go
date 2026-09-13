package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	uiBg           = color.RGBA{R: 16, G: 18, B: 22, A: 255}
	uiPanel        = color.RGBA{R: 28, G: 32, B: 40, A: 240}
	uiPanelBorder  = color.RGBA{R: 70, G: 78, B: 92, A: 255}
	uiBtn          = color.RGBA{R: 42, G: 50, B: 64, A: 255}
	uiBtnHover     = color.RGBA{R: 62, G: 74, B: 94, A: 255}
	uiBtnSelected  = color.RGBA{R: 86, G: 74, B: 48, A: 255}
	uiBtnDisabled  = color.RGBA{R: 30, G: 34, B: 40, A: 255}
	uiBtnBorder    = color.RGBA{R: 120, G: 108, B: 72, A: 255}
	uiDim          = color.RGBA{R: 0, G: 0, B: 0, A: 140}
	uiAccentBorder = color.RGBA{R: 196, G: 172, B: 88, A: 255}
)

type button struct {
	id       string
	label    string
	x, y     float32
	w, h     float32
	disabled bool
	reason   string
	selected bool
}

func (b button) contains(mx, my int) bool {
	x, y := float32(mx), float32(my)
	return x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h
}

type buttonSpec struct {
	id, label string
	disabled  bool
	reason    string
	selected  bool
	w, h      float32
}

func stackButtons(x, y, w, h, gap float32, specs []buttonSpec) []button {
	out := make([]button, 0, len(specs))
	cy := y
	for _, s := range specs {
		bw, bh := s.w, s.h
		if bw == 0 {
			bw = w
		}
		if bh == 0 {
			bh = h
		}
		out = append(out, button{
			id:       s.id,
			label:    s.label,
			x:        x,
			y:        cy,
			w:        bw,
			h:        bh,
			disabled: s.disabled,
			reason:   s.reason,
			selected: s.selected,
		})
		cy += bh + gap
	}
	return out
}

func drawPanel(dst *ebiten.Image, x, y, w, h float32) {
	vector.FillRect(dst, x, y, w, h, uiPanel, false)
	vector.StrokeRect(dst, x, y, w, h, 1, uiPanelBorder, false)
}

func drawButton(dst *ebiten.Image, b button, hover bool) {
	fill := uiBtn
	switch {
	case b.disabled:
		fill = uiBtnDisabled
	case b.selected:
		fill = uiBtnSelected
	case hover:
		fill = uiBtnHover
	}
	vector.FillRect(dst, b.x, b.y, b.w, b.h, fill, false)
	border := uiBtnBorder
	if b.selected {
		border = uiAccentBorder
	}
	vector.StrokeRect(dst, b.x, b.y, b.w, b.h, 1, border, false)
	label := b.label
	if b.disabled {
		label = label + "  (disabled)"
	}
	ebitenutil.DebugPrintAt(dst, label, int(b.x)+12, int(b.y)+int(b.h)/2-7)
}

func drawLabel(dst *ebiten.Image, x, y int, text string) {
	ebitenutil.DebugPrintAt(dst, text, x, y)
}

func hitButton(buttons []button, mx, my int) (button, bool) {
	for i := range buttons {
		if buttons[i].contains(mx, my) {
			return buttons[i], true
		}
	}
	return button{}, false
}
