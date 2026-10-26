package ebiten

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUITokensMatchPaletteYAML(t *testing.T) {
	t.Parallel()
	path, err := locateUIPaletteYAML()
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseUIPaletteYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(uiTokens) {
		t.Fatalf("yaml has %d tokens, go has %d", len(got), len(uiTokens))
	}
	for name, want := range uiTokens {
		c, ok := got[name]
		if !ok {
			t.Fatalf("yaml missing %s", name)
		}
		if c != want {
			t.Fatalf("%s yaml=%v go=%v", name, c, want)
		}
	}
}

func TestLerpRGBAClamps(t *testing.T) {
	t.Parallel()
	a := color.RGBA{R: 10, G: 20, B: 30, A: 40}
	b := color.RGBA{R: 110, G: 120, B: 130, A: 140}
	if got := lerpRGBA(a, b, 0); got != a {
		t.Fatalf("t=0: %v", got)
	}
	if got := lerpRGBA(a, b, 1); got != b {
		t.Fatalf("t=1: %v", got)
	}
	mid := lerpRGBA(a, b, 0.5)
	if mid.R != 60 || mid.G != 70 || mid.B != 80 || mid.A != 90 {
		t.Fatalf("t=0.5: %v", mid)
	}
}

// TestUIContrast holds the chrome tokens to WCAG 2.x contrast: text 4.5:1
// on every surface it is drawn on, dim text 3:1, disabled text clearly
// dimmer than dim text yet still legible, and non-text marks (accent,
// letter and mood colours) 3:1. A translucent floating panel is judged
// over both a white and a black map, the worst cases behind it.
func TestUIContrast(t *testing.T) {
	t.Parallel()
	white := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	black := color.RGBA{0, 0, 0, 0xFF}
	panelW, panelK := overOpaque(uiPanel, white), overOpaque(uiPanel, black)
	tipW, tutorW := overOpaque(uiTooltip, white), overOpaque(uiTutorPanel, white)
	kitHover, _, kitHoverText := btnNormal.colors(true, false, false)
	kitSel, _, kitSelText := btnNormal.colors(false, false, true)
	primary, _, primaryText := btnPrimary.colors(false, false, false)
	primaryHover, _, _ := btnPrimary.colors(true, false, false)
	type pair struct {
		name   string
		fg, bg color.RGBA
		min    float64
	}
	pairs := []pair{
		{"text on panel (white map)", uiText, panelW, 4.5},
		{"text on panel (black map)", uiText, panelK, 4.5},
		{"text on button", uiText, uiBtn, 4.5},
		{"text on button hover", uiText, uiBtnHover, 4.5},
		{"text on button selected", uiText, uiBtnSelected, 4.5},
		{"text on kit hover", kitHoverText, kitHover, 4.5},
		{"text on kit selected", kitSelText, kitSel, 4.5},
		{"text on kit primary", primaryText, primary, 4.5},
		{"text on kit primary hover", primaryText, primaryHover, 4.5},
		{"text on field", uiText, uiField, 4.5},
		{"text on tooltip (white map)", uiText, tipW, 4.5},
		{"heading on panel (white map)", uiHeading, panelW, 4.5},
		{"warning on panel (white map)", uiWarning, panelW, 4.5},
		{"danger on panel (white map)", uiDanger, panelW, 4.5},
		{"good on panel (white map)", uiGood, panelW, 4.5},
		{"text on danger alert (white map)", uiText, overOpaque(uiAlertDangerBg, white), 4.5},
		{"text on warning alert (white map)", uiText, overOpaque(uiAlertWarnBg, white), 4.5},
		{"tutor text on tutor panel (white map)", uiTutorText, tutorW, 4.5},
		{"tutor heading on tutor panel (white map)", uiTutorHeading, tutorW, 4.5},
		{"dim text on panel (white map)", uiTextDim, panelW, 3},
		{"dim text on panel (black map)", uiTextDim, panelK, 3},
		{"dim text on button", uiTextDim, uiBtn, 3},
		{"dim text on tooltip (white map)", uiTextDim, tipW, 3},
		{"disabled text on disabled button", uiTextDisabled, uiBtnDisabled, 1.8},
		{"disabled text on panel (black map)", uiTextDisabled, panelK, 1.8},
		{"accent on panel (black map)", uiAccent, panelK, 3},
		{"tutor border on tutor panel (white map)", uiTutorBorder, tutorW, 3},
	}
	for _, k := range []struct {
		name string
		c    color.RGBA
	}{{"letter threat", uiLetterThreat}, {"letter caution", uiLetterCaution}, {"letter info", uiLetterInfo},
		{"letter good", uiLetterGood}, {"mood good", uiMoodGood}, {"mood mid", uiMoodMid}, {"mood bad", uiMoodBad}} {
		pairs = append(pairs, pair{k.name + " on panel (white map)", k.c, panelW, 3})
	}
	for _, p := range pairs {
		if got := contrastRatio(p.fg, p.bg); got < p.min {
			t.Errorf("%s: %.2f:1, want >= %.1f:1", p.name, got, p.min)
		}
	}
	// Disabled reads clearly dimmer than dim, which reads dimmer than text.
	if !(relLum(uiText) > relLum(uiTextDim) && relLum(uiTextDim) > relLum(uiTextDisabled)) {
		t.Errorf("text %.3f, dim %.3f, disabled %.3f: want strictly falling luminance",
			relLum(uiText), relLum(uiTextDim), relLum(uiTextDisabled))
	}
	for _, bg := range []color.RGBA{panelK, uiBtnDisabled} {
		dim, off := contrastRatio(uiTextDim, bg), contrastRatio(uiTextDisabled, bg)
		if off > 0.6*dim {
			t.Errorf("disabled %.2f:1 vs dim %.2f:1 on %v: want disabled <= 60%% of dim", off, dim, bg)
		}
	}
}

// uiReferenceHexes are another game's UI colours. The palette is original
// (ART §17/§33): the chrome may share that game's general look (neutral
// grey panels, a warm accent) but no token may copy one of these.
var uiReferenceHexes = []string{"15191D", "616C7A", "2A2B2C", "878787", "85552C", "B08B3D",
	"CCD9FF", "4DB3E6", "363636", "524736", "13161B"}

// uiNeutralTokens are the chrome surfaces, rules and backdrops that must
// read as plain grey: colour comes only from the accent and the semantic
// tokens drawn on them.
var uiNeutralTokens = []string{"ui.panel", "ui.panel_border", "ui.button", "ui.button_hover",
	"ui.button_disabled", "ui.button_border", "ui.tooltip", "ui.tooltip_border", "ui.field",
	"ui.field_border", "ui.scroll_track", "ui.scroll_thumb", "ui.slider_track", "ui.space",
	"ui.horizon", "ui.dim", "ui.modal_dim"}

// TestUIPanelsAreNeutralTranslucent: every chrome surface is a near-neutral
// grey (CIELAB chroma C* <= 6, below a plainly tinted colour), and the
// floating panel lets the map show through without washing out
// (alpha 0xB0-0xE6).
func TestUIPanelsAreNeutralTranslucent(t *testing.T) {
	t.Parallel()
	const maxChroma = 6
	for _, name := range uiNeutralTokens {
		c, ok := uiTokens[name]
		if !ok {
			t.Fatalf("no token %s", name)
		}
		if ch := labChroma(c); ch > maxChroma {
			t.Errorf("%s %02X%02X%02X has chroma %.1f, want <= %d", name, c.R, c.G, c.B, ch, maxChroma)
		}
	}
	if uiPanel.A < 0xB0 || uiPanel.A > 0xE6 {
		t.Errorf("panel alpha %#02x, want 0xB0-0xE6", uiPanel.A)
	}
	if relLum(uiBtnSelected) <= relLum(uiBtn) || labChroma(uiBtnSelected) <= maxChroma {
		t.Errorf("selected button %v: want a lighter, warm (tinted) fill that reads apart from the grey ones", uiBtnSelected)
	}
}

// TestUITextContrast holds body text, dim text and headings to WCAG 2.x
// AA (4.5:1) on the surfaces they sit on: the translucent panel over a
// dark map and over a mid-grey (bright ground) map, the plain button and
// the tooltip. TestUIContrast covers the white-map worst case and the
// semantic colours.
func TestUITextContrast(t *testing.T) {
	t.Parallel()
	black := color.RGBA{0, 0, 0, 0xFF}
	grey := color.RGBA{0x80, 0x80, 0x80, 0xFF}
	surfaces := []struct {
		name string
		bg   color.RGBA
	}{
		{"panel (dark map)", overOpaque(uiPanel, black)},
		{"panel (grey map)", overOpaque(uiPanel, grey)},
		{"button", uiBtn},
		{"tooltip (grey map)", overOpaque(uiTooltip, grey)},
	}
	inks := []struct {
		name string
		c    color.RGBA
	}{{"text", uiText}, {"dim text", uiTextDim}, {"heading", uiHeading}}
	for _, s := range surfaces {
		for _, ink := range inks {
			if got := contrastRatio(ink.c, s.bg); got < 4.5 {
				t.Errorf("%s on %s: %.2f:1, want >= 4.5:1", ink.name, s.name, got)
			}
		}
	}
}

// TestUITokensAreNotCopies keeps every token at least CIE76 dE 3 (a
// just-noticeable difference side by side) from each reference hex, so no
// colour is lifted from another game, even near-identically.
func TestUITokensAreNotCopies(t *testing.T) {
	t.Parallel()
	const minDE = 3
	for name, c := range uiTokens {
		for _, h := range uiReferenceHexes {
			ref, err := parseHexRGBA(h + "FF")
			if err != nil {
				t.Fatal(err)
			}
			if d := deltaE76(c, ref); d < minDE {
				t.Errorf("%s %02X%02X%02X is dE %.1f from #%s, want >= %d", name, c.R, c.G, c.B, d, h, minDE)
			}
		}
	}
}

// TestButtonColorsStates: drawButton's states read apart, and disabled
// wins over selected and hover (dim label, no accent edge).
func TestButtonColorsStates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		b                   button
		hover               bool
		fill, border, label color.RGBA
	}{
		{"normal", button{}, false, uiBtn, uiBtnBorder, uiText},
		{"hover", button{}, true, uiBtnHover, uiBtnBorder, uiText},
		{"selected", button{selected: true}, false, uiBtnSelected, uiAccent, uiText},
		{"selected hover", button{selected: true}, true, uiBtnSelected, uiAccent, uiText},
		{"disabled", button{disabled: true}, true, uiBtnDisabled, uiBtnBorder, uiTextDisabled},
		{"disabled selected", button{disabled: true, selected: true}, false, uiBtnDisabled, uiBtnBorder, uiTextDisabled},
	}
	for _, c := range cases {
		fill, border, label := buttonColors(c.b, c.hover)
		if fill != c.fill || border != c.border || label != c.label {
			t.Errorf("%s: got %v %v %v, want %v %v %v", c.name, fill, border, label, c.fill, c.border, c.label)
		}
	}
	if !(relLum(uiBtnHover) > relLum(uiBtn) && relLum(uiBtn) > relLum(uiBtnDisabled)) {
		t.Errorf("want hover lighter than a button and disabled darker")
	}
	if uiBtnSelected == uiBtn || uiBtnSelected == uiBtnHover {
		t.Errorf("selected fill must differ from the plain and hover fills")
	}
}

// overOpaque draws token c over an opaque backdrop the way ebiten blends a
// color.RGBA (premultiplied source-over).
func overOpaque(c, bg color.RGBA) color.RGBA {
	k := 255 - int(c.A)
	ch := func(s, d uint8) uint8 { return uint8(min(255, int(s)+(int(d)*k+127)/255)) }
	return color.RGBA{ch(c.R, bg.R), ch(c.G, bg.G), ch(c.B, bg.B), 0xFF}
}

func srgbToLinear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// relLum is the WCAG relative luminance of an opaque colour.
func relLum(c color.RGBA) float64 {
	return 0.2126*srgbToLinear(c.R) + 0.7152*srgbToLinear(c.G) + 0.0722*srgbToLinear(c.B)
}

func contrastRatio(a, b color.RGBA) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// deltaE76 is the CIE76 distance between two colours in CIELAB (D65).
func deltaE76(a, b color.RGBA) float64 {
	l1, a1, b1 := cieLab(a)
	l2, a2, b2 := cieLab(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// labChroma is the CIELAB chroma C* (distance from the grey axis).
func labChroma(c color.RGBA) float64 {
	_, a, b := cieLab(c)
	return math.Hypot(a, b)
}

func cieLab(c color.RGBA) (l, a, b float64) {
	r, g, bl := srgbToLinear(c.R), srgbToLinear(c.G), srgbToLinear(c.B)
	x := (0.4124*r + 0.3576*g + 0.1805*bl) / 0.95047
	y := 0.2126*r + 0.7152*g + 0.0722*bl
	z := (0.0193*r + 0.1192*g + 0.9505*bl) / 1.08883
	f := func(t float64) float64 {
		if t > 216.0/24389 {
			return math.Cbrt(t)
		}
		return (24389.0/27*t + 16) / 116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
}

func locateUIPaletteYAML() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	const rel = "assets/palettes/ui.yaml"
	for i := 0; i < 12; i++ {
		cand := filepath.Join(dir, rel)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("palette %q not found from cwd", rel)
}

func parseUIPaletteYAML(path string) (map[string]color.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]color.RGBA{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, hexstr, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("bad palette line %q", line)
		}
		key = strings.TrimSpace(key)
		hexstr = strings.Trim(strings.TrimSpace(hexstr), `"`)
		c, err := parseHexRGBA(hexstr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out[key] = c
	}
	return out, sc.Err()
}

func parseHexRGBA(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 8 {
		return color.RGBA{}, fmt.Errorf("need #RRGGBBAA, got %q", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: b[0], G: b[1], B: b[2], A: b[3]}, nil
}
