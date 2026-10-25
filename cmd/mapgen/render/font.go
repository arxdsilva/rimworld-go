package render

import (
	"image"
	"image/color"
	"strconv"
)

// digits is a 5x7 bitmap font: seven rows per digit, five bits each (bit 4
// is the left column).
var digits = [10][7]uint8{
	{0x0e, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0e},
	{0x04, 0x0c, 0x04, 0x04, 0x04, 0x04, 0x0e},
	{0x0e, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1f},
	{0x1f, 0x02, 0x04, 0x02, 0x01, 0x11, 0x0e},
	{0x02, 0x06, 0x0a, 0x12, 0x1f, 0x02, 0x02},
	{0x1f, 0x10, 0x1e, 0x01, 0x01, 0x11, 0x0e},
	{0x06, 0x08, 0x10, 0x1e, 0x11, 0x11, 0x0e},
	{0x1f, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	{0x0e, 0x11, 0x11, 0x0e, 0x11, 0x11, 0x0e},
	{0x0e, 0x11, 0x11, 0x0f, 0x01, 0x02, 0x0c},
}

// Glyph size in pixels, and the gap between glyphs.
const (
	GlyphW, GlyphH, GlyphGap = 5, 7, 1
)

// TextWidth is the pixel width of n digits at scale s.
func TextWidth(n, s int) int {
	if n <= 0 {
		return 0
	}
	return (n*(GlyphW+GlyphGap) - GlyphGap) * s
}

// DrawNumber draws v in white on a dark box with its top-left corner at
// (x, y), each font pixel s×s. Anything outside img is clipped.
func DrawNumber(img *image.RGBA, x, y int, v uint64, s int) {
	if s < 1 {
		s = 1
	}
	txt := strconv.FormatUint(v, 10)
	pad := s
	fill(img, x, y, TextWidth(len(txt), s)+2*pad, GlyphH*s+2*pad, color.RGBA{0, 0, 0, 255})
	for i, ch := range txt {
		g := digits[ch-'0']
		gx := x + pad + i*(GlyphW+GlyphGap)*s
		for row := 0; row < GlyphH; row++ {
			for col := 0; col < GlyphW; col++ {
				if g[row]>>(GlyphW-1-col)&1 != 0 {
					fill(img, gx+col*s, y+pad+row*s, s, s, color.RGBA{255, 255, 255, 255})
				}
			}
		}
	}
}

func fill(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	b := img.Bounds()
	for yy := max(y, b.Min.Y); yy < min(y+h, b.Max.Y); yy++ {
		for xx := max(x, b.Min.X); xx < min(x+w, b.Max.X); xx++ {
			img.SetRGBA(xx, yy, c)
		}
	}
}
