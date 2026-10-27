package ebiten

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

// The UI font (NG-1 §1.1): Noto Sans, SIL Open Font License 1.1
// (fontdata/OFL.txt), embedded so packaging needs nothing extra.

//go:embed fontdata/NotoSans-Regular.ttf
var notoRegular []byte

//go:embed fontdata/NotoSans-SemiBold.ttf
var notoSemiBold []byte

// align is a text style's horizontal alignment about its x.
type align uint8

const (
	alignLeft align = iota
	alignCenter
	alignRight
)

// textStyle is how a run of UI text is drawn.
type textStyle struct {
	Size  float64
	Bold  bool
	Color color.Color
	Align align
}

// The type scale (NG-1 §1.1) in logical pixels. Faces are rasterised at
// Size x the UI scale (PLAY-012), so text is always sharp; positions and
// metrics stay logical.
var (
	tsSmall = textStyle{Size: 11}
	tsBody  = textStyle{Size: 13}
	tsLabel = textStyle{Size: 13, Bold: true}
	tsLarge = textStyle{Size: 16, Bold: true}
	tsTitle = textStyle{Size: 22, Bold: true}
	tsHuge  = textStyle{Size: 32, Bold: true}
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

// faceSize is the pixel size a style is rasterised at under UI scale s,
// and k = that size / the style size (the glyph scale). At s = 1 it is
// the style size. Otherwise it is floored to whole pixels: the shaper
// rounds a fractional size up to the next whole ppem, which would make
// text wider than the layout measured. Whole sizes scale linearly, so
// logical widths (physical / k) match s = 1 to well under a pixel, and
// the drawn text never exceeds its scaled box (it may be up to one ppem
// smaller).
func faceSize(size, s float64) (px, k float64) {
	if s == 1 || size <= 0 {
		return size, 1
	}
	px = max(1, math.Floor(size*s+1e-6))
	return px, px / size
}

// face returns the cached face for a style at UI scale s and its glyph
// scale k (call with fontMu held).
func face(st textStyle, s float64) (*text.GoTextFace, float64) {
	px, sc := faceSize(st.Size, s)
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

// drawText draws s with its top-left at logical (x, y) (or its top-centre/
// top-right for centred/right-aligned styles). Newlines start new lines.
// It draws at physical (x, y) x the UI scale with a face of that size, a
// logical line height apart.
func drawText(dst *ebiten.Image, s string, x, y float64, st textStyle) {
	if deferring(dst) { // world_scale.go: drawn at native size after the map
		deferText(s, x, y, st)
		return
	}
	sc := uiScale()
	fontMu.Lock()
	defer fontMu.Unlock()
	f, k := face(st, sc)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x*sc, y*sc)
	op.LineSpacing = logicalLineH(f, k) * sc
	switch st.Align {
	case alignCenter:
		op.PrimaryAlign = text.AlignCenter
	case alignRight:
		op.PrimaryAlign = text.AlignEnd
	}
	c := st.Color
	if c == nil {
		c = uiText
	}
	op.ColorScale.ScaleWithColor(c)
	if drawTrace != nil && s != "" {
		traceText(dst, s, f, op, x*sc, y*sc, st.Align)
	}
	text.Draw(dst, s, f, op)
}

// traceText reports a text run to drawTrace: the physical box its glyphs
// cover, from the anchor (px, py) its alignment is about (tests only).
func traceText(dst *ebiten.Image, s string, f *text.GoTextFace, op *text.DrawOptions, px, py float64, a align) {
	w, h := text.Measure(s, f, op.LineSpacing)
	switch a {
	case alignCenter:
		px -= w / 2
	case alignRight:
		px -= w
	}
	emitTrace(dst, "text", s, float32(px), float32(py), float32(w), float32(h))
}

// measure is the logical size of s drawn in st: measured on the scaled
// face, divided by its glyph scale.
func measure(s string, st textStyle) (w, h float64) {
	sc := hudScale() // text is drawn at the HUD scale, also from a world pass
	fontMu.Lock()
	defer fontMu.Unlock()
	f, k := face(st, sc)
	w, h = text.Measure(s, f, logicalLineH(f, k)*k)
	if k != 1 {
		w, h = w/k, h/k
	}
	return w, h
}

// lineH is the logical line height of st: ascent + descent + 2 px leading.
func lineH(st textStyle) float64 {
	sc := hudScale()
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

// wrapPx breaks s into lines no wider than maxW, on spaces; a word wider
// than maxW is broken by runes. Existing newlines are kept.
func wrapPx(s string, st textStyle, maxW float64) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if w, _ := measure(try, st); w <= maxW {
				line = try
				continue
			}
			if line != "" {
				out = append(out, line)
			}
			line = ""
			for w, _ := measure(word, st); w > maxW && len([]rune(word)) > 1; w, _ = measure(word, st) {
				r := []rune(word)
				n := len(r) - 1
				for n > 1 {
					if pw, _ := measure(string(r[:n]), st); pw <= maxW {
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

// ellipsize shortens s with "…" so it fits maxW.
func ellipsize(s string, st textStyle, maxW float64) string {
	if w, _ := measure(s, st); w <= maxW {
		return s
	}
	r := []rune(s)
	for n := len(r) - 1; n > 0; n-- {
		t := string(r[:n]) + "…"
		if w, _ := measure(t, st); w <= maxW {
			return t
		}
	}
	return "…"
}
