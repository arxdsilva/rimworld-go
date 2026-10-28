package uikit

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// UI chrome tokens (ART-E neutral charcoal: translucent dark-grey panels,
// restrained grey rules, warm off-white text and one warm gold accent).
// Hex values must match assets/palettes/ui.yaml; widgets_test.go holds the
// chrome neutral and translucent, the text to WCAG contrast, and every
// token clear of the reference hexes.
var (
	Space       = color.RGBA{R: 0x0E, G: 0x0F, B: 0x12, A: 0xFF}
	Dust        = color.RGBA{R: 0x8C, G: 0x85, B: 0x76, A: 0xFF}
	Horizon     = color.RGBA{R: 0x24, G: 0x26, B: 0x2A, A: 0xFF}
	Panel       = color.RGBA{R: 0x26, G: 0x29, B: 0x2E, A: 0xE0}
	PanelBorder = color.RGBA{R: 0x5A, G: 0x5D, B: 0x61, A: 0xFF}
	Btn         = color.RGBA{R: 0x3C, G: 0x3F, B: 0x43, A: 0xFF}
	BtnHover    = color.RGBA{R: 0x4B, G: 0x4E, B: 0x52, A: 0xFF}
	BtnSelected = color.RGBA{R: 0x5E, G: 0x54, B: 0x40, A: 0xFF}
	BtnDisabled = color.RGBA{R: 0x24, G: 0x26, B: 0x2B, A: 0xFF}
	BtnBorder   = color.RGBA{R: 0x66, G: 0x69, B: 0x6E, A: 0xFF}
	Accent      = color.RGBA{R: 0xD9, G: 0xB4, B: 0x5B, A: 0xFF}
	Dim         = color.RGBA{R: 0x05, G: 0x05, B: 0x06, A: 0xA0}
	// NG-1 §1.2: text, states, fields, sliders, scrollbars, tooltips, modals.
	Text         = gfx.TextColor
	TextDim      = color.RGBA{R: 0xB8, G: 0xB4, B: 0xAA, A: 0xFF}
	TextDisabled = color.RGBA{R: 0x6D, G: 0x6A, B: 0x64, A: 0xFF}
	Heading      = color.RGBA{R: 0xF0, G: 0xD9, B: 0xA0, A: 0xFF}
	Warning      = color.RGBA{R: 0xED, G: 0xA9, B: 0x4A, A: 0xFF}
	Danger       = color.RGBA{R: 0xFF, G: 0xA0, B: 0x94, A: 0xFF}
	Good         = color.RGBA{R: 0x8A, G: 0xCF, B: 0x6A, A: 0xFF}
	Field        = color.RGBA{R: 0x1F, G: 0x20, B: 0x22, A: 0xFF}
	FieldBorder  = color.RGBA{R: 0x5A, G: 0x5D, B: 0x61, A: 0xFF}
	FieldFocus   = color.RGBA{R: 0xD9, G: 0xB4, B: 0x5B, A: 0xFF}
	SliderTrack  = color.RGBA{R: 0x21, G: 0x23, B: 0x26, A: 0xFF}
	SliderFill   = color.RGBA{R: 0x9C, G: 0x82, B: 0x40, A: 0xFF}
	SliderKnob   = color.RGBA{R: 0xD9, G: 0xB4, B: 0x5B, A: 0xFF}
	ScrollTrack  = color.RGBA{R: 0x1F, G: 0x20, B: 0x22, A: 0xFF}
	ScrollThumb  = color.RGBA{R: 0x5F, G: 0x62, B: 0x67, A: 0xFF}
	Tooltip      = color.RGBA{R: 0x22, G: 0x24, B: 0x28, A: 0xF5}
	TooltipEdge  = color.RGBA{R: 0x6E, G: 0x71, B: 0x75, A: 0xFF}
	ModalDim     = color.RGBA{R: 0x05, G: 0x05, B: 0x06, A: 0xB8}
	// PLAY-012 plan marks: a soft fill and a brighter edge.
	PlanFill = color.RGBA{R: 0xB9, G: 0xA8, B: 0xF0, A: 0x30}
	PlanEdge = color.RGBA{R: 0xE6, G: 0xD2, B: 0xFF, A: 0x90}
	// PLAY-018: letter kinds, alert backings, the learning helper's warm
	// brown and gold (set apart from the grey chrome) and the mood bar.
	LetterThreat  = color.RGBA{R: 0xFF, G: 0xA0, B: 0x94, A: 0xFF}
	LetterCaution = color.RGBA{R: 0xED, G: 0xA9, B: 0x4A, A: 0xFF}
	LetterInfo    = color.RGBA{R: 0x7F, G: 0xA8, B: 0xDC, A: 0xFF}
	LetterGood    = color.RGBA{R: 0x8A, G: 0xCF, B: 0x6A, A: 0xFF}
	AlertDangerBg = color.RGBA{R: 0x5A, G: 0x1C, B: 0x24, A: 0xE8}
	AlertWarnBg   = color.RGBA{R: 0x4F, G: 0x38, B: 0x10, A: 0xE8}
	TutorPanel    = color.RGBA{R: 0x3E, G: 0x27, B: 0x16, A: 0xF2}
	TutorBorder   = color.RGBA{R: 0xE2, G: 0xAC, B: 0x48, A: 0xFF}
	TutorHeading  = color.RGBA{R: 0xF7, G: 0xD6, B: 0x8A, A: 0xFF}
	TutorText     = color.RGBA{R: 0xF5, G: 0xEA, B: 0xD7, A: 0xFF}
	MoodGood      = color.RGBA{R: 0x8A, G: 0xCF, B: 0x6A, A: 0xFF}
	MoodMid       = color.RGBA{R: 0xE6, G: 0xC2, B: 0x4E, A: 0xFF}
	MoodBad       = color.RGBA{R: 0xFF, G: 0xA0, B: 0x94, A: 0xFF}
	// PLAY-021 power overlay: a colour per grid, handed out in turn
	// (power_overlay.go), and grey for pieces on no grid.
	PowerNet1 = color.RGBA{R: 0xF5, G: 0xD5, B: 0x47, A: 0xFF}
	PowerNet2 = color.RGBA{R: 0x4F, G: 0xE0, B: 0xD0, A: 0xFF}
	PowerNet3 = color.RGBA{R: 0xF2, G: 0x7E, B: 0xB8, A: 0xFF}
	PowerNet4 = color.RGBA{R: 0xA3, G: 0xE0, B: 0x5A, A: 0xFF}
	PowerNet5 = color.RGBA{R: 0xA9, G: 0x8B, B: 0xFF, A: 0xFF}
	PowerNet6 = color.RGBA{R: 0x3E, G: 0x8B, B: 0xFF, A: 0xFF}
	PowerNone = color.RGBA{R: 0x9A, G: 0x94, B: 0xA8, A: 0xFF}
)

var Tokens = map[string]color.RGBA{
	"ui.space":           Space,
	"ui.dust":            Dust,
	"ui.horizon":         Horizon,
	"ui.panel":           Panel,
	"ui.panel_border":    PanelBorder,
	"ui.button":          Btn,
	"ui.button_hover":    BtnHover,
	"ui.button_selected": BtnSelected,
	"ui.button_disabled": BtnDisabled,
	"ui.button_border":   BtnBorder,
	"ui.accent":          Accent,
	"ui.dim":             Dim,
	"ui.text":            Text,
	"ui.text_dim":        TextDim,
	"ui.text_disabled":   TextDisabled,
	"ui.heading":         Heading,
	"ui.warning":         Warning,
	"ui.danger":          Danger,
	"ui.good":            Good,
	"ui.field":           Field,
	"ui.field_border":    FieldBorder,
	"ui.field_focus":     FieldFocus,
	"ui.slider_track":    SliderTrack,
	"ui.slider_fill":     SliderFill,
	"ui.slider_knob":     SliderKnob,
	"ui.scroll_track":    ScrollTrack,
	"ui.scroll_thumb":    ScrollThumb,
	"ui.tooltip":         Tooltip,
	"ui.tooltip_border":  TooltipEdge,
	"ui.modal_dim":       ModalDim,
	"ui.plan_fill":       PlanFill,
	"ui.plan_edge":       PlanEdge,
	"ui.letter_threat":   LetterThreat,
	"ui.letter_caution":  LetterCaution,
	"ui.letter_info":     LetterInfo,
	"ui.letter_good":     LetterGood,
	"ui.alert_danger_bg": AlertDangerBg,
	"ui.alert_warn_bg":   AlertWarnBg,
	"ui.tutor_panel":     TutorPanel,
	"ui.tutor_border":    TutorBorder,
	"ui.tutor_heading":   TutorHeading,
	"ui.tutor_text":      TutorText,
	"ui.mood_good":       MoodGood,
	"ui.mood_mid":        MoodMid,
	"ui.mood_bad":        MoodBad,
	"ui.power_net1":      PowerNet1,
	"ui.power_net2":      PowerNet2,
	"ui.power_net3":      PowerNet3,
	"ui.power_net4":      PowerNet4,
	"ui.power_net5":      PowerNet5,
	"ui.power_net6":      PowerNet6,
	"ui.power_none":      PowerNone,
}

const menuBackdropBands = 12

func DrawMenuBackdrop(dst *ebiten.Image, sw, sh int) {
	dst.Fill(Space)
	if sw < 1 || sh < 1 {
		return
	}
	h := float32(sh)/menuBackdropBands + 1
	for i := 0; i < menuBackdropBands; i++ {
		t := float64(i) / float64(menuBackdropBands-1)
		c := LerpRGBA(Space, Horizon, t*t)
		c.A = uint8(28 + t*110)
		y := float32(sh) * float32(i) / menuBackdropBands
		gfx.FillRect(dst, 0, y, float32(sw), h, c, false)
	}
	glow := Dust
	glow.A = 70
	gfx.FillCircle(dst, float32(sw)/2, float32(sh)*1.08, float32(sw)*0.52, glow, false)
}

func LerpRGBA(a, b color.RGBA, t float64) color.RGBA {
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

type Button struct {
	ID       string
	Label    string
	X, Y     float32
	W, H     float32
	Disabled bool
	Reason   string
	Selected bool
	Quiet    bool   // disabled without the "(disabled)" suffix (the reason shows on hover)
	Tip      string // hover line for an enabled button (e.g. its hotkey)
}

func (b Button) Contains(mx, my int) bool {
	x, y := float32(mx), float32(my)
	return x >= b.X && x < b.X+b.W && y >= b.Y && y < b.Y+b.H
}

type ButtonSpec struct {
	ID, Label string
	Disabled  bool
	Reason    string
	selected  bool
	w, h      float32
}

func StackButtons(x, y, w, h, gap float32, specs []ButtonSpec) []Button {
	out := make([]Button, 0, len(specs))
	cy := y
	for _, s := range specs {
		bw, bh := s.w, s.h
		if bw == 0 {
			bw = w
		}
		if bh == 0 {
			bh = h
		}
		out = append(out, Button{
			ID:       s.ID,
			Label:    s.Label,
			X:        x,
			Y:        cy,
			W:        bw,
			H:        bh,
			Disabled: s.Disabled,
			Reason:   s.Reason,
			Selected: s.selected,
		})
		cy += bh + gap
	}
	return out
}

func DrawPanel(dst *ebiten.Image, x, y, w, h float32) {
	gfx.FillRect(dst, x, y, w, h, Panel, false)
	gfx.StrokeRect(dst, x, y, w, h, 1, PanelBorder, false)
}

// ButtonColors is a DrawButton's fill, border and label colour. Disabled
// wins over selected (dim label, no accent edge), selected over hover.
func ButtonColors(b Button, hover bool) (fill, border, label color.RGBA) {
	switch {
	case b.Disabled:
		return BtnDisabled, BtnBorder, TextDisabled
	case b.Selected:
		return BtnSelected, Accent, Text
	case hover:
		return BtnHover, BtnBorder, Text
	}
	return Btn, BtnBorder, Text
}

func DrawButton(dst *ebiten.Image, b Button, hover bool) {
	fill, border, _ := ButtonColors(b, hover)
	gfx.FillRect(dst, b.X, b.Y, b.W, b.H, fill, false)
	gfx.StrokeRect(dst, b.X, b.Y, b.W, b.H, 1, border, false)
	label := b.Label
	if b.Disabled && label != "" && !b.Quiet {
		if w, _ := gfx.Measure(label+"  (disabled)", gfx.Body); w <= float64(b.W)-16 {
			label += "  (disabled)" // else the dimmed fill and the hover reason say it
		}
	}
	drawButtonLabel(dst, label, b)
}

// drawButtonLabel prints a button's label vertically centred: inset from
// the left when it fits with room, centred when it only just fits (the
// "-", "+", "x" stubs), else ellipsized.
func drawButtonLabel(dst *ebiten.Image, label string, b Button) {
	tw, _ := gfx.Measure(label, gfx.Body)
	bw := float64(b.W)
	x := float64(b.X) + 10
	switch {
	case tw <= bw-20:
	case tw <= bw-4:
		x = float64(b.X) + (bw-tw)/2
	default:
		label = gfx.Ellipsize(label, gfx.Body, bw-8)
		x = float64(b.X) + 4
	}
	st := gfx.Body
	_, _, c := ButtonColors(b, false)
	st.Color = c
	gfx.Text(dst, label, x, float64(b.Y)+(float64(b.H)-gfx.LineH(gfx.Body))/2+1, st)
}

// DrawCentered prints s in st centred on (cx, cy).
func DrawCentered(dst *ebiten.Image, s string, cx, cy float64, st gfx.TextStyle) {
	st.Align = gfx.AlignCenter
	gfx.Text(dst, s, cx, cy-gfx.LineH(st)/2+1, st)
}

// DrawLabelClip is DrawLabel cut with an ellipsis to maxW pixels.
func DrawLabelClip(dst *ebiten.Image, x, y, maxW int, text string) {
	DrawLabel(dst, x, y, gfx.Ellipsize(text, gfx.Body, float64(maxW)))
}

// DrawFitSmall prints s in st at (x, y) top-left, dropping to the small style
// and then ellipsizing so it stays within maxW.
func DrawFitSmall(dst *ebiten.Image, s string, x, y, maxW float64, st gfx.TextStyle) {
	if w, _ := gfx.Measure(s, st); w > maxW {
		small := gfx.Small
		small.Bold, small.Color = st.Bold, st.Color
		if sm := small; sm.Size < st.Size {
			st = sm
		}
		s = gfx.Ellipsize(s, st, maxW)
	}
	gfx.Text(dst, s, x, y, st)
}

// labelDY lifts Noto Sans so a DrawLabel (x, y) lands where the 16-px
// debug font's glyphs did: baselines match to within a pixel.
const labelDY = -2

func DrawLabel(dst *ebiten.Image, x, y int, text string) {
	gfx.Text(dst, text, float64(x), float64(y+labelDY), gfx.Body)
}

func HitButton(buttons []Button, mx, my int) (Button, bool) {
	for i := range buttons {
		if buttons[i].Contains(mx, my) {
			return buttons[i], true
		}
	}
	return Button{}, false
}

// DebugPrintAt draws body-size UI text at the old debug-font position
// (kept for its many call sites; drawText is already goroutine-safe).
func DebugPrintAt(dst *ebiten.Image, s string, x, y int) {
	DrawLabel(dst, x, y, s)
}

// Fade is token c (an opaque colour) at alpha a, premultiplied as
// color.RGBA expects: backings, scrims and separators drawn over the map
// or a panel.
func Fade(c color.RGBA, a uint8) color.RGBA {
	m := uint32(a)
	return color.RGBA{R: uint8(uint32(c.R) * m / 255), G: uint8(uint32(c.G) * m / 255), B: uint8(uint32(c.B) * m / 255), A: a}
}

// Shade is the dark backing behind small labels over the map (stack
// counts, damage numbers, zone names, health bars) at alpha a.
func Shade(a uint8) color.RGBA {
	return Fade(color.RGBA{R: Tooltip.R, G: Tooltip.G, B: Tooltip.B, A: 0xFF}, a)
}
