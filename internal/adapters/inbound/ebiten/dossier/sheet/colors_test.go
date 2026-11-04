package sheet

import (
	"image/color"
	"math"
	"testing"

	"rimworld-go/internal/application/game/dossierview"
)

func TestHSLRoundTrip(t *testing.T) {
	t.Parallel()
	check := func(c color.RGBA) {
		h, s, l := RGBToHSL(c)
		if h < 0 || h >= 360 || s < 0 || s > 1 || l < 0 || l > 1 {
			t.Fatalf("%v: out of range %v %v %v", c, h, s, l)
		}
		if back := HSLToRGB(h, s, l, c.A); back != c {
			t.Fatalf("%v -> %v %v %v -> %v", c, h, s, l, back)
		}
	}
	for r := 0; r < 256; r += 5 {
		for g := 0; g < 256; g += 5 {
			for b := 0; b < 256; b += 5 {
				check(color.RGBA{uint8(r), uint8(g), uint8(b), 200})
			}
		}
	}
	for _, c := range []color.RGBA{{0, 0, 0, 255}, {255, 255, 255, 255}, {255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255},
		{1, 2, 3, 0}, {254, 253, 255, 9}} {
		check(c)
	}
}

func TestHSLKnownValues(t *testing.T) {
	t.Parallel()
	h, s, l := RGBToHSL(color.RGBA{255, 0, 0, 255})
	if h != 0 || s != 1 || l != 0.5 {
		t.Fatalf("red: %v %v %v", h, s, l)
	}
	if h, _, _ := RGBToHSL(color.RGBA{0, 0, 255, 255}); h != 240 {
		t.Fatalf("blue hue %v", h)
	}
	if h, s, l := RGBToHSL(color.RGBA{128, 128, 128, 255}); h != 0 || s != 0 || math.Abs(l-128.0/255) > 1e-9 {
		t.Fatalf("grey: %v %v %v", h, s, l)
	}
	if got := HSLToRGB(-120, 1, 0.5, 255); got != (color.RGBA{0, 0, 255, 255}) {
		t.Fatalf("hue wraps: %v", got)
	}
	if got := HSLToRGB(0, 9, -3, 255); got != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("s and l clamp: %v", got)
	}
}

func TestColorText(t *testing.T) {
	t.Parallel()
	c, ok := ParseColor("#ff8000cc")
	if !ok || c != (color.RGBA{255, 128, 0, 204}) {
		t.Fatalf("%v %v", c, ok)
	}
	if FormatColor(c) != "#FF8000CC" {
		t.Fatalf("%s", FormatColor(c))
	}
	if c, ok := ParseColor("102030"); !ok || c != (color.RGBA{16, 32, 48, 255}) {
		t.Fatalf("six digits are opaque: %v %v", c, ok)
	}
	for _, bad := range []string{"", "#", "#12345", "#GGGGGGGG", "#1234567", "red", "#+1+2+3+4"} {
		if _, ok := ParseColor(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestBarColorStyles(t *testing.T) {
	t.Parallel()
	low, main := color.RGBA{200, 0, 0, 255}, color.RGBA{0, 200, 0, 255}
	cases := []struct {
		style string
		frac  float64
		want  color.RGBA
	}{
		{BarMainOnly, 0, main}, {BarMainOnly, 1, main}, {"", 0.1, main}, {"junk", 0.1, main},
		{BarLowOnly, 0, low}, {BarLowOnly, 1, low},
		{BarFadeUp, 0, low}, {BarFadeUp, 1, main}, {BarFadeUp, 0.5, color.RGBA{100, 100, 0, 255}},
		{BarFadeDown, 1, main}, {BarFadeDown, 0.5, main}, {BarFadeDown, 0, low}, {BarFadeDown, 0.25, color.RGBA{100, 100, 0, 255}},
		{BarFadeUp, -3, low}, {BarFadeUp, 9, main},
	}
	for _, c := range cases {
		if got := BarColor(c.style, c.frac, low, main); got != c.want {
			t.Errorf("%s at %v: %v want %v", c.style, c.frac, got, c.want)
		}
	}
	for _, s := range BarStyles() {
		if !ValidBarStyle(s) || BarStyleLabel(s) == "" {
			t.Errorf("style %s", s)
		}
	}
	if ValidBarStyle("") {
		t.Error("empty is no style")
	}
}

func TestRelationTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		r    dossierview.Relation
		key  string
		tone Tone
	}{
		{dossierview.RelationOwn, "side_own", ToneGood},
		{dossierview.RelationAlly, "side_ally", ToneTop},
		{dossierview.RelationNone, "side_free", ToneNeutral},
		{dossierview.RelationFoe, "side_foe", ToneCritical},
		{dossierview.RelationHeld, "side_held", ToneWarning},
		{dossierview.RelationWild, "side_wild", ToneText},
	}
	for _, c := range cases {
		if RelationKey(c.r) != c.key || RelationTone(c.r) != c.tone {
			t.Errorf("%d: %s %v", c.r, RelationKey(c.r), RelationTone(c.r))
		}
	}
}

func TestToneKeysAreDistinct(t *testing.T) {
	t.Parallel()
	seen := map[string]Tone{}
	for tn := ToneText; tn <= ToneTop; tn++ {
		k := ToneKey(tn)
		if prev, dup := seen[k]; dup {
			t.Errorf("tones %d and %d share %s", prev, tn, k)
		}
		seen[k] = tn
	}
}

func TestMoodTipTones(t *testing.T) {
	t.Parallel()
	for v, want := range map[int]Tone{-30: ToneCritical, -10: ToneCritical, -9: ToneWarning, -1: ToneWarning, 0: ToneNeutral, 9: ToneNeutral, 10: ToneGood, 40: ToneGood} {
		if got := MoodTipTone(v); got != want {
			t.Errorf("%d: %v want %v", v, got, want)
		}
	}
}
