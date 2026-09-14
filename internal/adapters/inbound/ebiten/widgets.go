package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// UI chrome tokens. Hex values must match assets/palettes/ui.yaml.
var (
	uiSpace       = color.RGBA{R: 0x0A, G: 0x10, B: 0x18, A: 0xFF}
	uiDust        = color.RGBA{R: 0x8C, G: 0x4A, B: 0x2E, A: 0xFF}
	uiHorizon     = color.RGBA{R: 0xA8, G: 0x5C, B: 0x32, A: 0xFF}
	uiPanel       = color.RGBA{R: 0x2A, G: 0x1E, B: 0x16, A: 0xE6}
	uiPanelBorder = color.RGBA{R: 0x6B, G: 0x53, B: 0x40, A: 0xFF}
	uiBtn         = color.RGBA{R: 0x5C, G: 0x40, B: 0x30, A: 0xFF}
	uiBtnHover    = color.RGBA{R: 0x6E, G: 0x4E, B: 0x3A, A: 0xFF}
	uiBtnSelected = color.RGBA{R: 0x8B, G: 0x5A, B: 0x2B, A: 0xFF}
	uiBtnDisabled = color.RGBA{R: 0x3A, G: 0x2E, B: 0x28, A: 0xFF}
	uiBtnBorder   = color.RGBA{R: 0x8A, G: 0x6A, B: 0x48, A: 0xFF}
	uiAccent      = color.RGBA{R: 0xE8, G: 0xC0, B: 0x70, A: 0xFF}
	uiDim         = color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xA0}
)

var uiTokens = map[string]color.RGBA{
	"ui.space":           uiSpace,
	"ui.dust":            uiDust,
	"ui.horizon":         uiHorizon,
	"ui.panel":           uiPanel,
	"ui.panel_border":    uiPanelBorder,
	"ui.button":          uiBtn,
	"ui.button_hover":    uiBtnHover,
	"ui.button_selected": uiBtnSelected,
	"ui.button_disabled": uiBtnDisabled,
	"ui.button_border":   uiBtnBorder,
	"ui.accent":          uiAccent,
	"ui.dim":             uiDim,
}

const menuBackdropBands = 12

func drawMenuBackdrop(dst *ebiten.Image, sw, sh int) {
	dst.Fill(uiSpace)
	if sw < 1 || sh < 1 {
		return
	}
	h := float32(sh)/menuBackdropBands + 1
	for i := 0; i < menuBackdropBands; i++ {
		t := float64(i) / float64(menuBackdropBands-1)
		c := lerpRGBA(uiSpace, uiHorizon, t*t)
		c.A = uint8(28 + t*110)
		y := float32(sh) * float32(i) / menuBackdropBands
		vector.FillRect(dst, 0, y, float32(sw), h, c, false)
	}
	glow := uiDust
	glow.A = 70
	vector.FillCircle(dst, float32(sw)/2, float32(sh)*1.08, float32(sw)*0.52, glow, false)
}

func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: uint8(float64(a.A) + (float64(b.A)-float64(a.A))*t),
	}
}

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
		border = uiAccent
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
