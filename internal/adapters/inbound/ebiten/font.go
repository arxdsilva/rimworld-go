package ebiten

import (
	"bytes"
	_ "embed"
	"image/color"
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

// The type scale (NG-1 §1.1); UI scale multiplies Size.
var (
	tsSmall = textStyle{Size: 11}
	tsBody  = textStyle{Size: 13}
	tsLabel = textStyle{Size: 13, Bold: true}
	tsLarge = textStyle{Size: 16, Bold: true}
	tsTitle = textStyle{Size: 22, Bold: true}
	tsHuge  = textStyle{Size: 32, Bold: true}
)

type faceKey struct {
	size float64
	bold bool
}

var (
	fontMu      sync.Mutex
	fontSources [2]*text.GoTextFaceSource
	fontFaces   = map[faceKey]*text.GoTextFace{}
	uiScale     = 1.0 // Accessibility UI scale (settings), 0.9..1.3
)

// face returns the cached face for a style (call with fontMu held).
func face(st textStyle) *text.GoTextFace {
	k := faceKey{st.Size * uiScale, st.Bold}
	if f, ok := fontFaces[k]; ok {
		return f
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
	return f
}

// drawText draws s with its top-left at (x, y) (or its top-centre/
// top-right for centred/right-aligned styles). Newlines start new lines.
func drawText(dst *ebiten.Image, s string, x, y float64, st textStyle) {
	fontMu.Lock()
	defer fontMu.Unlock()
	f := face(st)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.LineSpacing = lineHeightOf(f)
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
	text.Draw(dst, s, f, op)
}

// measure is the size of s drawn in st.
func measure(s string, st textStyle) (w, h float64) {
	fontMu.Lock()
	defer fontMu.Unlock()
	f := face(st)
	return text.Measure(s, f, lineHeightOf(f))
}

// lineH is the line height of st: ascent + descent + 2 px leading.
func lineH(st textStyle) float64 {
	fontMu.Lock()
	defer fontMu.Unlock()
	return lineHeightOf(face(st))
}

func lineHeightOf(f *text.GoTextFace) float64 {
	m := f.Metrics()
	return m.HAscent + m.HDescent + 2
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

// setUIScale sets the Accessibility UI scale (percent) and drops cached faces.
func setUIScale(pct int) {
	fontMu.Lock()
	defer fontMu.Unlock()
	s := float64(max(90, min(130, pct))) / 100
	if s != uiScale {
		uiScale = s
		fontFaces = map[faceKey]*text.GoTextFace{}
	}
}
