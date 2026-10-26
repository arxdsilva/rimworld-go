package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// UI chrome tokens (PLAY-018 cool charcoal: slate panels, blue-grey rules,
// a sea-glass accent). Hex values must match assets/palettes/ui.yaml;
// widgets_test.go holds them to WCAG contrast and keeps them clear of the
// reference hexes.
var (
	uiSpace       = color.RGBA{R: 0x05, G: 0x0D, B: 0x26, A: 0xFF}
	uiDust        = color.RGBA{R: 0x2C, G: 0x7A, B: 0x84, A: 0xFF}
	uiHorizon     = color.RGBA{R: 0x12, G: 0x3E, B: 0x58, A: 0xFF}
	uiPanel       = color.RGBA{R: 0x1E, G: 0x26, B: 0x38, A: 0xEB}
	uiPanelBorder = color.RGBA{R: 0x3D, G: 0x54, B: 0x70, A: 0xFF}
	uiBtn         = color.RGBA{R: 0x26, G: 0x30, B: 0x43, A: 0xFF}
	uiBtnHover    = color.RGBA{R: 0x36, G: 0x46, B: 0x5F, A: 0xFF}
	uiBtnSelected = color.RGBA{R: 0x1F, G: 0x58, B: 0x62, A: 0xFF}
	uiBtnDisabled = color.RGBA{R: 0x1C, G: 0x24, B: 0x37, A: 0xFF}
	uiBtnBorder   = color.RGBA{R: 0x4C, G: 0x62, B: 0x88, A: 0xFF}
	uiAccent      = color.RGBA{R: 0x72, G: 0xD4, B: 0xC0, A: 0xFF}
	uiDim         = color.RGBA{R: 0x02, G: 0x06, B: 0x0C, A: 0xA0}
	// NG-1 §1.2: text, states, fields, sliders, scrollbars, tooltips, modals.
	uiText         = color.RGBA{R: 0xE4, G: 0xEB, B: 0xF2, A: 0xFF}
	uiTextDim      = color.RGBA{R: 0x9D, G: 0xAE, B: 0xC2, A: 0xFF}
	uiTextDisabled = color.RGBA{R: 0x52, G: 0x61, B: 0x8A, A: 0xFF}
	uiHeading      = color.RGBA{R: 0xBF, G: 0xED, B: 0xE2, A: 0xFF}
	uiWarning      = color.RGBA{R: 0xED, G: 0xA9, B: 0x4A, A: 0xFF}
	uiDanger       = color.RGBA{R: 0xFA, G: 0x88, B: 0x7C, A: 0xFF}
	uiGood         = color.RGBA{R: 0x8A, G: 0xCF, B: 0x6A, A: 0xFF}
	uiField        = color.RGBA{R: 0x0C, G: 0x16, B: 0x2E, A: 0xFF}
	uiFieldBorder  = color.RGBA{R: 0x3D, G: 0x54, B: 0x70, A: 0xFF}
	uiFieldFocus   = color.RGBA{R: 0x72, G: 0xD4, B: 0xC0, A: 0xFF}
	uiSliderTrack  = color.RGBA{R: 0x22, G: 0x2A, B: 0x3C, A: 0xFF}
	uiSliderFill   = color.RGBA{R: 0x2F, G: 0x8F, B: 0x8A, A: 0xFF}
	uiSliderKnob   = color.RGBA{R: 0x72, G: 0xD4, B: 0xC0, A: 0xFF}
	uiScrollTrack  = color.RGBA{R: 0x0C, G: 0x16, B: 0x2E, A: 0xFF}
	uiScrollThumb  = color.RGBA{R: 0x4C, G: 0x62, B: 0x88, A: 0xFF}
	uiTooltip      = color.RGBA{R: 0x0D, G: 0x16, B: 0x30, A: 0xF5}
	uiTooltipEdge  = color.RGBA{R: 0x58, G: 0x7A, B: 0xA8, A: 0xFF}
	uiModalDim     = color.RGBA{R: 0x02, G: 0x06, B: 0x0C, A: 0xB8}
	// PLAY-012 plan marks: a soft fill and a brighter edge.
	uiPlanFill = color.RGBA{R: 0xB9, G: 0xA8, B: 0xF0, A: 0x30}
	uiPlanEdge = color.RGBA{R: 0xE6, G: 0xD2, B: 0xFF, A: 0x90}
	// PLAY-018: letter kinds, alert backings, the learning helper's warm
	// brown and gold (set apart from the cool chrome) and the mood bar.
	uiLetterThreat  = color.RGBA{R: 0xFA, G: 0x88, B: 0x7C, A: 0xFF}
	uiLetterCaution = color.RGBA{R: 0xED, G: 0xA9, B: 0x4A, A: 0xFF}
	uiLetterInfo    = color.RGBA{R: 0x7F, G: 0xA8, B: 0xDC, A: 0xFF}
	uiLetterGood    = color.RGBA{R: 0x8A, G: 0xCF, B: 0x6A, A: 0xFF}
	uiAlertDangerBg = color.RGBA{R: 0x5A, G: 0x1C, B: 0x24, A: 0xE8}
	uiAlertWarnBg   = color.RGBA{R: 0x4F, G: 0x38, B: 0x10, A: 0xE8}
	uiTutorPanel    = color.RGBA{R: 0x3E, G: 0x27, B: 0x16, A: 0xF2}
	uiTutorBorder   = color.RGBA{R: 0xE2, G: 0xAC, B: 0x48, A: 0xFF}
	uiTutorHeading  = color.RGBA{R: 0xF7, G: 0xD6, B: 0x8A, A: 0xFF}
	uiTutorText     = color.RGBA{R: 0xF5, G: 0xEA, B: 0xD7, A: 0xFF}
	uiMoodGood      = color.RGBA{R: 0x8A, G: 0xCF, B: 0x6A, A: 0xFF}
	uiMoodMid       = color.RGBA{R: 0xE6, G: 0xC2, B: 0x4E, A: 0xFF}
	uiMoodBad       = color.RGBA{R: 0xFA, G: 0x88, B: 0x7C, A: 0xFF}
	// PLAY-021 power overlay: a colour per grid, handed out in turn
	// (power_overlay.go), and grey for pieces on no grid.
	uiPowerNet1 = color.RGBA{R: 0xF5, G: 0xD5, B: 0x47, A: 0xFF}
	uiPowerNet2 = color.RGBA{R: 0x4F, G: 0xE0, B: 0xD0, A: 0xFF}
	uiPowerNet3 = color.RGBA{R: 0xF2, G: 0x7E, B: 0xB8, A: 0xFF}
	uiPowerNet4 = color.RGBA{R: 0xA3, G: 0xE0, B: 0x5A, A: 0xFF}
	uiPowerNet5 = color.RGBA{R: 0xA9, G: 0x8B, B: 0xFF, A: 0xFF}
	uiPowerNet6 = color.RGBA{R: 0x3E, G: 0x8B, B: 0xFF, A: 0xFF}
	uiPowerNone = color.RGBA{R: 0x9A, G: 0x94, B: 0xA8, A: 0xFF}
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
	"ui.text":            uiText,
	"ui.text_dim":        uiTextDim,
	"ui.text_disabled":   uiTextDisabled,
	"ui.heading":         uiHeading,
	"ui.warning":         uiWarning,
	"ui.danger":          uiDanger,
	"ui.good":            uiGood,
	"ui.field":           uiField,
	"ui.field_border":    uiFieldBorder,
	"ui.field_focus":     uiFieldFocus,
	"ui.slider_track":    uiSliderTrack,
	"ui.slider_fill":     uiSliderFill,
	"ui.slider_knob":     uiSliderKnob,
	"ui.scroll_track":    uiScrollTrack,
	"ui.scroll_thumb":    uiScrollThumb,
	"ui.tooltip":         uiTooltip,
	"ui.tooltip_border":  uiTooltipEdge,
	"ui.modal_dim":       uiModalDim,
	"ui.plan_fill":       uiPlanFill,
	"ui.plan_edge":       uiPlanEdge,
	"ui.letter_threat":   uiLetterThreat,
	"ui.letter_caution":  uiLetterCaution,
	"ui.letter_info":     uiLetterInfo,
	"ui.letter_good":     uiLetterGood,
	"ui.alert_danger_bg": uiAlertDangerBg,
	"ui.alert_warn_bg":   uiAlertWarnBg,
	"ui.tutor_panel":     uiTutorPanel,
	"ui.tutor_border":    uiTutorBorder,
	"ui.tutor_heading":   uiTutorHeading,
	"ui.tutor_text":      uiTutorText,
	"ui.mood_good":       uiMoodGood,
	"ui.mood_mid":        uiMoodMid,
	"ui.mood_bad":        uiMoodBad,
	"ui.power_net1":      uiPowerNet1,
	"ui.power_net2":      uiPowerNet2,
	"ui.power_net3":      uiPowerNet3,
	"ui.power_net4":      uiPowerNet4,
	"ui.power_net5":      uiPowerNet5,
	"ui.power_net6":      uiPowerNet6,
	"ui.power_none":      uiPowerNone,
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
		uiFillRect(dst, 0, y, float32(sw), h, c, false)
	}
	glow := uiDust
	glow.A = 70
	uiFillCircle(dst, float32(sw)/2, float32(sh)*1.08, float32(sw)*0.52, glow, false)
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
	quiet    bool   // disabled without the "(disabled)" suffix (the reason shows on hover)
	tip      string // hover line for an enabled button (e.g. its hotkey)
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
	uiFillRect(dst, x, y, w, h, uiPanel, false)
	uiStrokeRect(dst, x, y, w, h, 1, uiPanelBorder, false)
}

// buttonColors is a drawButton's fill, border and label colour. Disabled
// wins over selected (dim label, no accent edge), selected over hover.
func buttonColors(b button, hover bool) (fill, border, label color.RGBA) {
	switch {
	case b.disabled:
		return uiBtnDisabled, uiBtnBorder, uiTextDisabled
	case b.selected:
		return uiBtnSelected, uiAccent, uiText
	case hover:
		return uiBtnHover, uiBtnBorder, uiText
	}
	return uiBtn, uiBtnBorder, uiText
}

func drawButton(dst *ebiten.Image, b button, hover bool) {
	fill, border, _ := buttonColors(b, hover)
	uiFillRect(dst, b.x, b.y, b.w, b.h, fill, false)
	uiStrokeRect(dst, b.x, b.y, b.w, b.h, 1, border, false)
	label := b.label
	if b.disabled && label != "" && !b.quiet {
		if w, _ := measure(label+"  (disabled)", tsBody); w <= float64(b.w)-16 {
			label += "  (disabled)" // else the dimmed fill and the hover reason say it
		}
	}
	drawButtonLabel(dst, label, b)
}

// drawButtonLabel prints a button's label vertically centred: inset from
// the left when it fits with room, centred when it only just fits (the
// "-", "+", "x" stubs), else ellipsized.
func drawButtonLabel(dst *ebiten.Image, label string, b button) {
	tw, _ := measure(label, tsBody)
	bw := float64(b.w)
	x := float64(b.x) + 10
	switch {
	case tw <= bw-20:
	case tw <= bw-4:
		x = float64(b.x) + (bw-tw)/2
	default:
		label = ellipsize(label, tsBody, bw-8)
		x = float64(b.x) + 4
	}
	st := tsBody
	_, _, c := buttonColors(b, false)
	st.Color = c
	drawText(dst, label, x, float64(b.y)+(float64(b.h)-lineH(tsBody))/2+1, st)
}

// drawCentered prints s in st centred on (cx, cy).
func drawCentered(dst *ebiten.Image, s string, cx, cy float64, st textStyle) {
	st.Align = alignCenter
	drawText(dst, s, cx, cy-lineH(st)/2+1, st)
}

// drawLabelClip is drawLabel cut with an ellipsis to maxW pixels.
func drawLabelClip(dst *ebiten.Image, x, y, maxW int, text string) {
	drawLabel(dst, x, y, ellipsize(text, tsBody, float64(maxW)))
}

// drawFitSmall prints s in st at (x, y) top-left, dropping to the small style
// and then ellipsizing so it stays within maxW.
func drawFitSmall(dst *ebiten.Image, s string, x, y, maxW float64, st textStyle) {
	if w, _ := measure(s, st); w > maxW {
		small := tsSmall
		small.Bold, small.Color = st.Bold, st.Color
		if sm := small; sm.Size < st.Size {
			st = sm
		}
		s = ellipsize(s, st, maxW)
	}
	drawText(dst, s, x, y, st)
}

// labelDY lifts Noto Sans so a drawLabel (x, y) lands where the 16-px
// debug font's glyphs did: baselines match to within a pixel.
const labelDY = -2

func drawLabel(dst *ebiten.Image, x, y int, text string) {
	drawText(dst, text, float64(x), float64(y+labelDY), tsBody)
}

func hitButton(buttons []button, mx, my int) (button, bool) {
	for i := range buttons {
		if buttons[i].contains(mx, my) {
			return buttons[i], true
		}
	}
	return button{}, false
}

// debugPrintAt draws body-size UI text at the old debug-font position
// (kept for its many call sites; drawText is already goroutine-safe).
func debugPrintAt(dst *ebiten.Image, s string, x, y int) {
	drawLabel(dst, x, y, s)
}

// uiFade is token c (an opaque colour) at alpha a, premultiplied as
// color.RGBA expects: backings, scrims and separators drawn over the map
// or a panel.
func uiFade(c color.RGBA, a uint8) color.RGBA {
	m := uint32(a)
	return color.RGBA{R: uint8(uint32(c.R) * m / 255), G: uint8(uint32(c.G) * m / 255), B: uint8(uint32(c.B) * m / 255), A: a}
}

// uiShade is the dark backing behind small labels over the map (stack
// counts, damage numbers, zone names, health bars) at alpha a.
func uiShade(a uint8) color.RGBA {
	return uiFade(color.RGBA{R: uiTooltip.R, G: uiTooltip.G, B: uiTooltip.B, A: 0xFF}, a)
}
