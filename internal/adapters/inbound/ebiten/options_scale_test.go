package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	optionspkg "rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/config"
)

func optionBtn(t *testing.T, a *App, id string) uikit.Button {
	t.Helper()
	for _, b := range a.opt.Buttons(a.sw, a.sh) {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("no options button %q", id)
	return uikit.Button{}
}

// uiScaleApp is an app on a physW x physH window (device scale 1) holding the
// stored choice at 1x.
func uiScaleApp(t *testing.T, physW, physH int) (*App, *fakeSettings) {
	t.Helper()
	store := &fakeSettings{s: config.Defaults()}
	a := scaledTestApp(t, 1, func(s *config.Settings) {
		s.Accessibility.UIScaleAuto, s.Accessibility.UIScale = false, 100
	})
	a.cfg.Settings = store.hooks()
	a.LayoutF(float64(physW), float64(physH))
	a.finishBoot()
	a.opt.Tab = "access"
	return a, store
}

func lastToast(a *App) string {
	ts := a.liveToasts()
	if len(ts) == 0 {
		return ""
	}
	return ts[len(ts)-1].Text
}

func TestOptionsUIScaleListsAutoAndSteps(t *testing.T) {
	a, _ := uiScaleApp(t, 5120, 3072)
	a.opt.ApplyOption("options.uiscale")
	u := a.uiSt()
	if u.Top() != "options.uiscale.pick" {
		t.Fatalf("the row opens the list, top = %q", u.Top())
	}
	for _, p := range append([]int{0}, config.UIScalePresets...) {
		if u.Top() != "options.uiscale.pick" {
			a.opt.ApplyOption("options.uiscale")
		}
		a.opt.Scale.Choose(p)
		ac := a.settings.Accessibility
		if p == 0 && !ac.UIScaleAuto || p != 0 && (ac.UIScaleAuto || ac.UIScale != p) {
			t.Fatalf("after choosing %d: auto=%v scale=%d", p, ac.UIScaleAuto, ac.UIScale)
		}
		a.opt.Scale.Keep()
		u.CloseModal("options.uiscale.keep")
		u.CloseModal("options.uiscale.pick")
	}
	if a.opt.Tab != "access" {
		t.Fatalf("tab changed to %q", a.opt.Tab)
	}
}

func TestOptionsUIScaleLabelsAreSteps(t *testing.T) {
	a := scaledTestApp(t, 1, nil)
	a.LayoutF(1920, 1200) // Auto: 1.25 (1536x960 logical, UIS-005)
	a.opt.Tab = "access"
	if got := optionBtn(t, a, "options.uiscale").Label; got != "Auto (1.25×)" {
		t.Fatalf("label = %q", got)
	}
	a.opt.Scale.Choose(100)
	if got := optionBtn(t, a, "options.uiscale").Label; got != "1×" {
		t.Fatalf("label = %q", got)
	}
}

// A stored step the window cannot hold shows both values (UIS-004).
func TestOptionsUIScaleLabelNamesTheStepInUse(t *testing.T) {
	a, _ := uiScaleApp(t, 1920, 1080)
	s := a.settings
	s.Accessibility.UIScale = 400
	a.setSettings(s)
	a.LayoutF(1920, 1080)
	if got := optionBtn(t, a, "options.uiscale").Label; got != "4× (using 1.25×)" {
		t.Fatalf("label = %q", got)
	}
}

func TestOptionsUIScaleRefusesAStepTheWindowCannotHold(t *testing.T) {
	for _, c := range []struct {
		w, h, refused, accepted int
	}{
		{1920, 1080, 150, 125}, // 1280x720 logical: 720 < 768
		{2560, 1440, 200, 150}, // 1280x720 logical
	} {
		a, store := uiScaleApp(t, c.w, c.h)
		saves := store.saves
		a.opt.Scale.Choose(c.refused)
		if got := a.settings.Accessibility; got.UIScale != 100 || got.UIScaleAuto {
			t.Fatalf("%dx%d: refused %d changed the value to %+v", c.w, c.h, c.refused, got)
		}
		if store.saves != saves || a.uiSt().Top() != "" {
			t.Fatalf("%dx%d: a refusal saves or asks nothing (saves %d, modal %q)", c.w, c.h, store.saves-saves, a.uiSt().Top())
		}
		if got := lastToast(a); !strings.Contains(got, "too small") || strings.Contains(got, "\n") {
			t.Fatalf("%dx%d: warning = %q", c.w, c.h, got)
		}
		a.opt.Scale.Choose(c.accepted)
		if got := a.settings.Accessibility; got.UIScale != c.accepted || got.UIScaleAuto {
			t.Fatalf("%dx%d: %d not applied: %+v", c.w, c.h, c.accepted, got)
		}
		if a.uiSt().Top() != "options.uiscale.keep" {
			t.Fatalf("%dx%d: an accepted step asks keep or revert, top = %q", c.w, c.h, a.uiSt().Top())
		}
	}
}

func TestOptionsUIScaleKeepRevertAndCountdown(t *testing.T) {
	a, _ := uiScaleApp(t, 2560, 1440)
	now := time.Unix(1000, 0)
	a.opt.Scale.Now = func() time.Time { return now }

	a.opt.Scale.Choose(150)
	a.opt.Scale.Revert()
	if a.settings.Accessibility.UIScale != 100 {
		t.Fatalf("revert keeps %d", a.settings.Accessibility.UIScale)
	}

	a.opt.Scale.Choose(150)
	a.opt.Scale.Keep()
	now = now.Add(time.Minute)
	a.opt.UI(a.sw, a.sh)
	if a.settings.Accessibility.UIScale != 150 {
		t.Fatalf("a kept step stays, got %d", a.settings.Accessibility.UIScale)
	}

	a.opt.Scale.Choose(175)
	now = now.Add(5 * time.Second)
	a.opt.UI(a.sw, a.sh)
	if a.settings.Accessibility.UIScale != 175 || a.uiSt().Top() != "options.uiscale.keep" {
		t.Fatalf("before the countdown ends: %d, top %q", a.settings.Accessibility.UIScale, a.uiSt().Top())
	}
	now = now.Add(6 * time.Second)
	a.opt.UI(a.sw, a.sh)
	if a.settings.Accessibility.UIScale != 150 || a.uiSt().Top() != "" {
		t.Fatalf("the countdown reverts: %d, top %q", a.settings.Accessibility.UIScale, a.uiSt().Top())
	}
}

func TestOptionsHiDPIToggle(t *testing.T) {
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	a.opt.Tab = "graphics"
	b := optionBtn(t, a, "options.hidpi")
	if b.Label != "On" || b.Tip == "" {
		t.Fatalf("hidpi button %+v", b)
	}
	a.opt.ApplyOption("options.hidpi")
	if store.s.Graphics.HiDPI || optionBtn(t, a, "options.hidpi").Label != "Off" {
		t.Fatal("HiDPI should be off and saved")
	}
}

func TestOptionsScaleChangeKeepsStepperHit(t *testing.T) {
	a := scaledTestApp(t, 1, nil)
	a.LayoutF(1920, 1200) // Auto: 1.25
	a.finishBoot()
	a.opt.Tab = "access"
	before := optionBtn(t, a, "options.uiscale")
	bx, by := before.X-float32((a.sw-650)/2), before.Y-float32((a.sh-600)/2) // the window is centred
	a.opt.Scale.Choose(100)                                                  // Auto -> 1x
	a.LayoutF(1920, 1200)
	if a.opt.CurrentTab() != "access" {
		t.Fatal("tab lost")
	}
	b := optionBtn(t, a, "options.uiscale")
	if b.X-float32((a.sw-650)/2) != bx || b.Y-float32((a.sh-600)/2) != by {
		t.Fatalf("stepper moved inside its window: %+v -> %+v", before, b)
	}
	if a.scale.UI != 1 {
		t.Fatalf("ui scale = %v", a.scale.UI)
	}
	cx, cy := int(b.X+b.W/2), int(b.Y+b.H/2)
	if !b.Contains(cx, cy) {
		t.Fatal("stepper not hit at its logical centre")
	}
}

func TestOptionsDrawAtScales(t *testing.T) {
	for _, tab := range []string{"graphics", "access"} {
		for _, s := range []float64{1, 1.5, 2} {
			a := scaledTestApp(t, 1, func(st *config.Settings) {
				st.Accessibility.UIScaleAuto, st.Accessibility.UIScale = false, int(s*100)
			})
			a.LayoutF(1920, 1080)
			a.finishBoot()
			a.opt.Tab = tab
			bs := a.opt.Buttons(a.sw, a.sh)
			for i, b := range bs {
				if b.X+b.W > float32(a.sw) || b.Y+b.H > float32(a.sh) {
					t.Fatalf("%s s=%v: %s off screen", tab, s, b.ID)
				}
				for _, o := range bs[i+1:] {
					if b.X < o.X+o.W && o.X < b.X+b.W && b.Y < o.Y+o.H && o.Y < b.Y+b.H {
						t.Fatalf("%s s=%v: %s overlaps %s", tab, s, b.ID, o.ID)
					}
				}
			}
			for i, r := range optionspkg.OptionRows(tab) {
				if w, _ := gfx.Measure(r.Label, gfx.Body); optionspkg.OptLabelX+w > optionspkg.OptCtrlX-8 {
					t.Fatalf("%s: label %q (%v px) runs into row %d control", tab, r.Label, w, i)
				}
			}
			a.Draw(ebiten.NewImage(a.pw, a.ph))
		}
	}
}
