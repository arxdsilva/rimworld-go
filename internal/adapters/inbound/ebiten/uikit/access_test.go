package uikit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

func TestAccessTipFrames(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a    Access
		want int
	}{
		{Access{}, TipDelay},                         // unset: today's delay
		{Access{Set: true, TooltipDelayMS: 400}, 24}, // the default setting is the old constant
		{Access{Set: true, TooltipDelayMS: 0}, 0},    // instant
		{Access{Set: true, TooltipDelayMS: 1000}, 60},
		{Access{Set: true, TooltipDelayMS: 2000}, 120},
	} {
		if got := c.a.TipFrames(); got != c.want {
			t.Errorf("%+v: %d frames, want %d", c.a, got, c.want)
		}
	}
}

// tipShownAfter is how many frames of resting on a tipped widget it takes
// before the card shows (-1: not within 200).
func tipShownAfter(k *Kit, h *fakeHost) int {
	ui := func() { k.Tip("tip", gfx.Rect{X: 0, Y: 0, W: 50, H: 50}, "Title\nbody") }
	for i := 1; i <= 200; i++ {
		step(k, h, at(10, 10), ui)
		if k.TipVisible() {
			return i
		}
	}
	return -1
}

func TestTooltipDelayFollowsSetting(t *testing.T) {
	t.Parallel()
	run := func(a Access) int {
		k, h := newTestKit()
		k.Access = a
		return tipShownAfter(k, h)
	}
	def := run(Access{})
	slow := run(Access{Set: true, TooltipDelayMS: 1500})
	fast := run(Access{Set: true, TooltipDelayMS: 0})
	if !(fast < def && def < slow) {
		t.Fatalf("instant %d < default %d < slow %d expected", fast, def, slow)
	}
	if fast > 2 {
		t.Fatalf("a 0 ms delay shows at once, took %d frames", fast)
	}
	if run(Access{Set: true, TooltipDelayMS: 400}) != def {
		t.Fatal("400 ms is the default delay")
	}
}

func TestColorVisionPaletteSwitch(t *testing.T) {
	t.Parallel()
	normal := PaletteFor(ColorVisionNormal)
	if normal.Danger != Danger || normal.Good != Good || normal.Warning != Warning || PaletteFor("") != normal || PaletteFor("bogus") != normal {
		t.Fatal("normal (and an unknown name) is the chrome's own status colours")
	}
	for _, name := range []string{ColorVisionDeuter, ColorVisionProtan, ColorVisionTritan} {
		if PaletteFor(name) == normal {
			t.Errorf("%s equals normal: the switch does nothing", name)
		}
	}
	// The danger button follows the palette.
	_, nb, _ := BtnDanger.ColorsIn(normal, false, false, false)
	_, db, _ := BtnDanger.ColorsIn(PaletteFor(ColorVisionDeuter), false, false, false)
	if nb != Danger || db == nb {
		t.Fatalf("danger border normal %v deuteranopia %v", nb, db)
	}
	if f, b, _ := BtnDanger.Colors(false, false, false); b != Danger || f != LerpRGBA(Btn, Danger, 0.35) {
		t.Fatal("Colors is the normal palette")
	}
	k, _ := newTestKit()
	k.Access.ColorVision = ColorVisionTritan
	if k.Access.Palette() != PaletteFor(ColorVisionTritan) {
		t.Fatal("the kit's palette follows Access.ColorVision")
	}
}

// TestColorVisionVariantsMatchYAML: a variant's status colour that the
// palette file lists (ui.<variant>.danger, ...) equals the Go value; the
// file may not have them yet (the art pass adds them), adding the keys is
// enough to pin them. Every listed key must name a status token.
func TestColorVisionVariantsMatchYAML(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "assets", "palettes", "ui.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	inYAML := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if name, v, ok := strings.Cut(line, ":"); ok {
			inYAML[name] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	for _, variant := range []string{ColorVisionNormal, ColorVisionDeuter, ColorVisionProtan, ColorVisionTritan} {
		for token, c := range PaletteFor(variant).Tokens() {
			key := strings.Replace(token, "ui.", "ui."+variant+".", 1)
			if variant == ColorVisionNormal {
				key = token
			}
			want, ok := inYAML[key]
			if !ok {
				if variant == ColorVisionNormal {
					t.Errorf("%s missing from the palette file", key)
				}
				continue
			}
			if got := dossierHex(c); got != want {
				t.Errorf("%s: yaml %q, go %q", key, want, got)
			}
		}
	}
	for key := range inYAML {
		for _, variant := range []string{ColorVisionDeuter, ColorVisionProtan, ColorVisionTritan} {
			if rest, ok := strings.CutPrefix(key, "ui."+variant+"."); ok {
				if _, known := PaletteFor(variant).Tokens()["ui."+rest]; !known {
					t.Errorf("%s names no status token", key)
				}
			}
		}
	}
}

func TestConfirmIfOnAndOff(t *testing.T) {
	t.Parallel()
	k, _ := newTestKit()
	ran := 0
	ConfirmIf(k, "c", "Sure?", "Really?", "Yes", true, func() { ran++ })
	if ran != 1 || k.ModalOpen() {
		t.Fatalf("off: the action runs at once (ran %d, modal %v)", ran, k.ModalOpen())
	}
	k.Access.Confirm = true
	ConfirmIf(k, "c", "Sure?", "Really?", "Yes", true, func() { ran++ })
	if ran != 1 || k.Top() != "c" {
		t.Fatalf("on: nothing runs before OK (ran %d, top %q)", ran, k.Top())
	}
	k.Modals[0].Buttons[0].OnClick()
	if ran != 2 {
		t.Fatalf("OK runs the action: %d", ran)
	}
}
