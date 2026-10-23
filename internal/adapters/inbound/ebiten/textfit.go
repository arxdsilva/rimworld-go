package ebiten

import (
	"image/color"
	"math"
)

// Small helpers for sizing UI around the real font (NG-1 §1.1): panels
// and rows are measured from the text, never from a glyph-width guess.

// withColor is st drawn in c.
func withColor(st textStyle, c color.Color) textStyle {
	st.Color = c
	return st
}

// lineStep is the whole-pixel line height of st.
func lineStep(st textStyle) float32 {
	return float32(math.Ceil(lineH(st)))
}

// maxTextW is the widest of lines in st.
func maxTextW(lines []string, st textStyle) float32 {
	var w float32
	for _, l := range lines {
		w = max(w, textW(l, st))
	}
	return w
}
