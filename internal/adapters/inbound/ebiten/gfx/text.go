package gfx

import (
	"bytes"
	_ "embed"
	"image/color"
	"math"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

//go:embed fontdata/NotoSans-Regular.ttf
var notoRegular []byte

//go:embed fontdata/NotoSans-SemiBold.ttf
var notoSemiBold []byte

// Align is a text style's horizontal alignment about its x.
type Align uint8

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// TextColor is the default text colour (NG-1 §1.2 uiText).
var TextColor = color.RGBA{R: 0xE8, G: 0xE4, B: 0xDA, A: 0xFF}

// TextStyle is how a run of UI text is drawn.
type TextStyle struct {
	Size  float64
	Bold  bool
	Color color.Color
	Align Align
}

// The type scale (NG-1 §1.1) in logical pixels. Faces are rasterised at
// Size x the UI scale (PLAY-012), so text is always sharp; positions and
// metrics stay logical.
var (
	Small = TextStyle{Size: 11}
	Body  = TextStyle{Size: 13}
	Label = TextStyle{Size: 13, Bold: true}
	Large = TextStyle{Size: 16, Bold: true}
	Title = TextStyle{Size: 22, Bold: true}
	Huge  = TextStyle{Size: 32, Bold: true}
)

// faceKey is a face's pixel size and weight.
type faceKey struct {
	size float64
	bold bool
}

var (
	fontMu      sync.Mutex
	fontSources [2]*text.GoTextFaceSource
	fontFaces   = map[faceKey]*text.GoTextFace{}
)

// FaceSize is the pixel size a style is rasterised at under UI scale s,
// and k = that size / the style size (the glyph scale). At s = 1 it is
// the style size. Otherwise it is floored to whole pixels: the shaper
// rounds a fractional size up to the next whole ppem, which would make
// text wider than the layout measured. Whole sizes scale linearly, so
// logical widths (physical / k) match s = 1 to well under a pixel, and
// the drawn text never exceeds its scaled box (it may be up to one ppem
// smaller).
func FaceSize(size, s float64) (px, k float64) {
	if s == 1 || size <= 0 {
		return size, 1
	}
	px = max(1, math.Floor(size*s+1e-6))
	return px, px / size
}

// face returns the cached face for a style at UI scale s and its glyph
// scale k (call with fontMu held).
func face(st TextStyle, s float64) (*text.GoTextFace, float64) {
	px, sc := FaceSize(st.Size, s)
	k := faceKey{px, st.Bold}
	if f, ok := fontFaces[k]; ok {
		return f, sc
	}
	i := 0
	if st.Bold {
		i = 1
	}
	if fontSources[i] == nil {
		data := notoRegular
		if st.Bold {
			data = notoSemiBold
		}
		src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
		if err != nil {
			panic("ui font: " + err.Error()) // embedded data: a broken build, not a runtime state
		}
		fontSources[i] = src
	}
	f := &text.GoTextFace{Source: fontSources[i], Size: k.size}
	fontFaces[k] = f
	return f, sc
}

// Text draws s with its top-left at logical (x, y) (or its top-centre/
// top-right for centred/right-aligned styles). Newlines start new lines.
// It draws at physical (x, y) x the UI scale with a face of that size, a
// logical line height apart.
func Text(dst *ebiten.Image, s string, x, y float64, st TextStyle) {
	if deferring(dst) { // world_scale.go: drawn at native size after the map
		deferText(s, x, y, st)
		return
	}
	sc := UIScale()
	fontMu.Lock()
	defer fontMu.Unlock()
	f, k := face(st, sc)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x*sc, y*sc)
	op.LineSpacing = logicalLineH(f, k) * sc
	switch st.Align {
	case AlignCenter:
		op.PrimaryAlign = text.AlignCenter
	case AlignRight:
		op.PrimaryAlign = text.AlignEnd
	}
	c := st.Color
	if c == nil {
		c = TextColor
	}
	op.ColorScale.ScaleWithColor(c)
	if cur.drawTrace != nil && s != "" {
		traceText(dst, s, f, op, x*sc, y*sc, st.Align)
	}
	text.Draw(dst, s, f, op)
}

// traceText reports a text run to cur.drawTrace: the physical box its glyphs
// cover, from the anchor (px, py) its alignment is about (tests only).
func traceText(dst *ebiten.Image, s string, f *text.GoTextFace, op *text.DrawOptions, px, py float64, a Align) {
	w, h := text.Measure(s, f, op.LineSpacing)
	switch a {
	case AlignCenter:
		px -= w / 2
	case AlignRight:
		px -= w
	}
	emitTrace(dst, "text", s, float32(px), float32(py), float32(w), float32(h))
}

// Measure is the logical size of s drawn in st: measured on the scaled
// face, divided by its glyph scale.
func Measure(s string, st TextStyle) (w, h float64) {
	sc := HUDScale() // text is drawn at the HUD scale, also from a world pass
	fontMu.Lock()
	defer fontMu.Unlock()
	f, k := face(st, sc)
	w, h = text.Measure(s, f, logicalLineH(f, k)*k)
	if k != 1 {
		w, h = w/k, h/k
	}
	return w, h
}

// LineH is the logical line height of st: ascent + descent + 2 px leading.
func LineH(st TextStyle) float64 {
	sc := HUDScale()
	fontMu.Lock()
	defer fontMu.Unlock()
	return logicalLineH(face(st, sc))
}

// logicalLineH is a face's line height in logical pixels (glyph scale k):
// ascent + descent + 2 px leading.
func logicalLineH(f *text.GoTextFace, k float64) float64 {
	m := f.Metrics()
	if k == 1 {
		return m.HAscent + m.HDescent + 2
	}
	return (m.HAscent+m.HDescent)/k + 2
}

// WrapPx breaks s into lines no wider than maxW, on spaces; a word wider
// than maxW is broken by runes. Existing newlines are kept.
func WrapPx(s string, st TextStyle, maxW float64) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if w, _ := Measure(try, st); w <= maxW {
				line = try
				continue
			}
			if line != "" {
				out = append(out, line)
			}
			line = ""
			for w, _ := Measure(word, st); w > maxW && len([]rune(word)) > 1; w, _ = Measure(word, st) {
				r := []rune(word)
				n := len(r) - 1
				for n > 1 {
					if pw, _ := Measure(string(r[:n]), st); pw <= maxW {
						break
					}
					n--
				}
				out = append(out, string(r[:n]))
				word = string(r[n:])
			}
			line = word
		}
		out = append(out, line)
	}
	return out
}

// Ellipsize shortens s with "…" so it fits maxW.
func Ellipsize(s string, st TextStyle, maxW float64) string {
	if w, _ := Measure(s, st); w <= maxW {
		return s
	}
	r := []rune(s)
	for n := len(r) - 1; n > 0; n-- {
		t := string(r[:n]) + "…"
		if w, _ := Measure(t, st); w <= maxW {
			return t
		}
	}
	return "…"
}

// WithColor is st drawn in c.
func WithColor(st TextStyle, c color.Color) TextStyle {
	st.Color = c
	return st
}

// LineStep is the whole-pixel line height of st.
func LineStep(st TextStyle) float32 {
	return float32(math.Ceil(LineH(st)))
}

// MaxTextW is the widest of lines in st.
func MaxTextW(lines []string, st TextStyle) float32 {
	var w float32
	for _, l := range lines {
		w = max(w, TextW(l, st))
	}
	return w
}

// TextW is the width of s in style st.
func TextW(s string, st TextStyle) float32 {
	w, _ := Measure(s, st)
	return float32(w)
}
