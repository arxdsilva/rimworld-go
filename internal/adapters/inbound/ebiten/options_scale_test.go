package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/config"
)

func optionBtn(t *testing.T, a *App, id string) button {
	t.Helper()
	for _, b := range a.optionsButtons(a.sw, a.sh) {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("no options button %q", id)
	return button{}
}

func TestOptionsUIScaleCyclesAutoAndPresets(t *testing.T) {
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	a.optionsTab = "access"
	if !a.settings.Accessibility.UIScaleAuto {
		t.Fatal("defaults should be Auto")
	}
	for _, p := range config.UIScalePresets {
		a.applyOption("options.uiscale")
		ac := store.s.Accessibility
		if ac.UIScaleAuto || ac.UIScale != p {
			t.Fatalf("after step: auto=%v scale=%d, want %d", ac.UIScaleAuto, ac.UIScale, p)
		}
	}
	a.applyOption("options.uiscale")
	if !store.s.Accessibility.UIScaleAuto {
		t.Fatal("after the last preset the stepper returns to Auto")
	}
	if a.optionsTab != "access" {
		t.Fatalf("tab changed to %q", a.optionsTab)
	}
	if store.saves != len(config.UIScalePresets)+1 {
		t.Fatalf("saves = %d", store.saves)
	}
}

func TestOptionsUIScaleAutoLabelShowsPercent(t *testing.T) {
	a := scaledTestApp(t, 1, nil)
	a.LayoutF(1920, 1080) // Auto: 1.5
	a.optionsTab = "access"
	if got := optionBtn(t, a, "options.uiscale").Label; got != "Auto (150%)" {
		t.Fatalf("label = %q", got)
	}
	a.applyOption("options.uiscale")
	if got := optionBtn(t, a, "options.uiscale").Label; got != "75%" {
		t.Fatalf("label = %q", got)
	}
}

func TestOptionsHiDPIToggle(t *testing.T) {
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	a.optionsTab = "graphics"
	b := optionBtn(t, a, "options.hidpi")
	if b.Label != "On" || b.Tip == "" {
		t.Fatalf("hidpi button %+v", b)
	}
	a.applyOption("options.hidpi")
	if store.s.Graphics.HiDPI || optionBtn(t, a, "options.hidpi").Label != "Off" {
		t.Fatal("HiDPI should be off and saved")
	}
}

func TestOptionsScaleChangeKeepsStepperHit(t *testing.T) {
	a := scaledTestApp(t, 1, nil)
	a.LayoutF(1920, 1080)
	a.finishBoot()
	a.optionsTab = "access"
	before := optionBtn(t, a, "options.uiscale")
	a.applyOption("options.uiscale") // Auto -> 75%
	a.LayoutF(1920, 1080)
	if a.currentOptionsTab() != "access" {
		t.Fatal("tab lost")
	}
	b := optionBtn(t, a, "options.uiscale")
	if b.X != before.X || b.Y != before.Y {
		t.Fatalf("stepper moved logically: %+v -> %+v", before, b)
	}
	if a.scale.UI != 0.75 {
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
			a.optionsTab = tab
			bs := a.optionsButtons(a.sw, a.sh)
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
			for i, r := range optionRows(tab) {
				if w, _ := measure(r.label, tsBody); 56+w > optCtrlX-8 {
					t.Fatalf("%s: label %q (%v px) runs into row %d control", tab, r.label, w, i)
				}
			}
			a.Draw(ebiten.NewImage(a.pw, a.ph))
		}
	}
}
