package uikit

import (
	"image/color"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestDossierThemeIsComplete: the key list and the table agree, every key of
// RHUD family G2 is there, and an unknown key is reported.
func TestDossierThemeIsComplete(t *testing.T) {
	t.Parallel()
	keys := DossierColorKeys()
	if len(keys) != len(dossierTheme) {
		t.Fatalf("%d keys, %d colours", len(keys), len(dossierTheme))
	}
	for _, k := range keys {
		if _, ok := DossierColor(k); !ok {
			t.Errorf("key %q has no colour", k)
		}
	}
	sorted := slices.Sorted(slices.Values(keys))
	if len(slices.Compact(sorted)) != len(keys) {
		t.Errorf("duplicate keys in %v", keys)
	}
	for _, k := range []string{"rule", "text", "text_off", "tone_critical", "tone_warning", "tone_neutral", "tone_good",
		"tone_top", "bar_back", "bar_main", "bar_low", "bar_tick", "picker_text", "picker_back", "skill_like",
		"skill_love", "skill_full", "skill_busy", "side_own", "side_ally", "side_free", "side_foe", "side_wild",
		"side_held", "tree_stack", "tree_block", "tree_line", "tree_widget", "tree_bad", "tree_core", "tree_mine"} {
		if !slices.Contains(keys, k) {
			t.Errorf("G2 key %q is missing", k)
		}
	}
	if _, ok := DossierColor("no_such_key"); ok {
		t.Error("an unknown key resolved")
	}
	keys[0] = "changed"
	if DossierColorKeys()[0] == "changed" {
		t.Error("DossierColorKeys hands out its own slice")
	}
}

// TestDossierColorsReadOnThePanel holds what the card writes to WCAG contrast
// on the translucent panel, over the white map that is its worst case: the
// dim Off tone 3:1, everything else 4.5:1. Over a black map they only gain.
func TestDossierColorsReadOnThePanel(t *testing.T) {
	t.Parallel()
	white := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	onPanel := dossierOver(Panel, white)
	for _, k := range DossierColorKeys() {
		c, _ := DossierColor(k)
		var want float64
		switch {
		case k == "rule", strings.HasPrefix(k, "bar_"), k == "picker_back":
			continue // not ink: TestDossierBarColorsRead has the bars
		case k == "text_off":
			want = 3
		default:
			want = 4.5
		}
		if got := dossierContrast(c, onPanel); got < want {
			t.Errorf("%s %02X%02X%02X on the panel: %.2f:1, want >= %.1f:1", k, c.R, c.G, c.B, got, want)
		}
	}
}

// TestDossierOffIsDimmerThanNeutral: the Off tone stays below the neutral one.
func TestDossierOffIsDimmerThanNeutral(t *testing.T) {
	t.Parallel()
	off, _ := DossierColor("text_off")
	neutral, _ := DossierColor("tone_neutral")
	if dossierLum(off) >= dossierLum(neutral)*0.8 {
		t.Errorf("off %.3f, neutral %.3f: want off clearly dimmer", dossierLum(off), dossierLum(neutral))
	}
}

// TestDossierBarColorsRead: a fill reads on its track, and a threshold tick
// reads on the track and on the main fill it crosses (3:1, non-text marks).
func TestDossierBarColorsRead(t *testing.T) {
	t.Parallel()
	get := func(k string) color.RGBA { c, _ := DossierColor(k); return c }
	for _, p := range [][2]string{{"bar_main", "bar_back"}, {"bar_low", "bar_back"}, {"bar_tick", "bar_back"}, {"bar_tick", "bar_main"}} {
		if got := dossierContrast(get(p[0]), get(p[1])); got < 3 {
			t.Errorf("%s on %s: %.2f:1, want >= 3:1", p[0], p[1], got)
		}
	}
}

// TestDossierColorsReadApart: colours that sit side by side on the card, as
// tones in one column or as skills and sides in one list, stay well apart
// (CIE76 dE >= 10). The words carry the meaning; this keeps the colour useful.
func TestDossierColorsReadApart(t *testing.T) {
	t.Parallel()
	for _, group := range [][]string{
		{"text", "text_off", "tone_critical", "tone_warning", "tone_neutral", "tone_good", "tone_top"},
		{"text", "skill_like", "skill_love", "skill_full", "skill_busy", "tone_critical"},
		{"side_own", "side_ally", "side_free", "side_foe", "side_wild", "side_held"},
		{"tree_stack", "tree_block", "tree_line", "tree_widget", "tree_bad", "tree_core", "tree_mine"},
	} {
		for i, a := range group {
			for _, b := range group[i+1:] {
				ca, _ := DossierColor(a)
				cb, _ := DossierColor(b)
				if d := dossierDeltaE(ca, cb); d < 10 {
					t.Errorf("%s and %s are dE %.1f apart, want >= 10", a, b, d)
				}
			}
		}
	}
}

// TestDossierTokensAreUsedAndInTheYAML: every ui.dossier_* token is in the
// palette file with the same value and backs a key, so none is left orphaned.
func TestDossierTokensAreUsedAndInTheYAML(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "assets", "palettes", "ui.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	inYAML := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if name, v, ok := strings.Cut(line, ":"); ok && strings.HasPrefix(name, "ui.dossier_") {
			inYAML[name] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	seen := 0
	for name, c := range Tokens {
		if !strings.HasPrefix(name, "ui.dossier_") {
			continue
		}
		seen++
		if got, want := inYAML[name], dossierHex(c); got != want {
			t.Errorf("%s: yaml %q, go %q", name, got, want)
		}
		used := false
		for _, v := range dossierTheme {
			used = used || v == c
		}
		if !used {
			t.Errorf("%s backs no colour key", name)
		}
	}
	if seen == 0 || seen != len(inYAML) {
		t.Errorf("%d dossier tokens in go, %d in the yaml", seen, len(inYAML))
	}
}

func dossierHex(c color.RGBA) string {
	const digits = "0123456789ABCDEF"
	b := []byte{'#', 0, 0, 0, 0, 0, 0, 0, 0}
	for i, v := range []uint8{c.R, c.G, c.B, c.A} {
		b[1+2*i], b[2+2*i] = digits[v>>4], digits[v&15]
	}
	return string(b)
}

func dossierOver(fg, bg color.RGBA) color.RGBA {
	a := float64(fg.A) / 255
	mix := func(f, b uint8) uint8 { return uint8(math.Round(float64(f)*a + float64(b)*(1-a))) }
	return color.RGBA{mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B), 0xFF}
}

func dossierLinear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func dossierLum(c color.RGBA) float64 {
	return 0.2126*dossierLinear(c.R) + 0.7152*dossierLinear(c.G) + 0.0722*dossierLinear(c.B)
}

func dossierContrast(a, b color.RGBA) float64 {
	la, lb := dossierLum(a), dossierLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func dossierDeltaE(a, b color.RGBA) float64 {
	lab := func(c color.RGBA) (float64, float64, float64) {
		r, g, bl := dossierLinear(c.R), dossierLinear(c.G), dossierLinear(c.B)
		f := func(t float64) float64 {
			if t > 216.0/24389 {
				return math.Cbrt(t)
			}
			return (24389.0/27*t + 16) / 116
		}
		x := f((0.4124*r + 0.3576*g + 0.1805*bl) / 0.95047)
		y := f(0.2126*r + 0.7152*g + 0.0722*bl)
		z := f((0.0193*r + 0.1192*g + 0.9505*bl) / 1.08883)
		return 116*y - 16, 500 * (x - y), 200 * (y - z)
	}
	l1, a1, b1 := lab(a)
	l2, a2, b2 := lab(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}
