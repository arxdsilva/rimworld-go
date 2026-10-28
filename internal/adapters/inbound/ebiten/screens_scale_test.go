package ebiten

import (
	"fmt"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// PLAY-012 B.4: every setup screen draws and hit-tests at UI scales other
// than 1. The window is sized so the logical screen stays 1280x720 (the
// setup minimum): the physical screen is 1280s x 720s.

// atLogical1280 switches a to UI scale s over a physical screen that is
// 1280x720 logical, and returns the physical size.
func atLogical1280(t *testing.T, a *App, f *fakeInput, s float64) (int, int) {
	t.Helper()
	t.Cleanup(func() { gfx.SetScale(unitUIScale) })
	a.devFn = func() float64 { return 1 } // not the test machine's Retina factor
	st := a.settings
	st.Accessibility.UIScaleAuto, st.Accessibility.UIScale = false, int(s*100)
	a.setSettings(st)
	pw, ph := a.LayoutF(1280*s, 720*s)
	if a.sw != 1280 || a.sh != 720 {
		t.Fatalf("scale %v: logical %dx%d, want 1280x720", s, a.sw, a.sh)
	}
	f.Frame(t, a)
	return int(pw), int(ph)
}

// checkScaledScreen draws the current screen at scale s and checks every
// registered hit rect lies inside the logical screen and that forward (if
// any) is hot at its logical centre.
func checkScaledScreen(t *testing.T, a *App, f *fakeInput, s float64, forward string) {
	t.Helper()
	pw, ph := atLogical1280(t, a, f, s)
	a.Draw(ebiten.NewImage(pw, ph))
	n := 0
	for _, h := range a.layoutHits(a.sw, a.sh) {
		n++
		if h.W <= 0 || h.H <= 0 || h.X < 0 || h.Y < 0 || h.X+h.W > float32(a.sw) || h.Y+h.H > float32(a.sh) {
			t.Errorf("%s at x%v: hit %q %+v is outside the %dx%d logical screen", a.nav.screen(), s, h.ID, h, a.sw, a.sh)
		}
	}
	if n == 0 {
		t.Errorf("%s at x%v declares no hits", a.nav.screen(), s)
	}
	if forward == "" {
		return
	}
	var fb button
	found := false
	for _, h := range a.layoutHits(a.sw, a.sh) {
		if h.ID == forward {
			fb, found = h, true
		}
	}
	if !found {
		t.Fatalf("%s at x%v: no hit %q", a.nav.screen(), s, forward)
	}
	cx, cy := fb.X+fb.W/2, fb.Y+fb.H/2
	f.Cur.Mx, f.Cur.My = int(cx*float32(s)), int(cy*float32(s)) // raw cursor is physical
	f.Frame(t, a)
	f.Frame(t, a)
	mx, my := a.currentCursor()
	if math.Abs(float64(float32(mx)-cx)) > 1 || math.Abs(float64(float32(my)-cy)) > 1 {
		t.Errorf("%s at x%v: raw cursor maps to logical (%d,%d), want (%v,%v)", a.nav.screen(), s, mx, my, cx, cy)
	}
	for _, h := range a.layoutHits(a.sw, a.sh) { // topmost first
		if cx >= h.X && cx < h.X+h.W && cy >= h.Y && cy < h.Y+h.H {
			if h.ID != forward {
				t.Errorf("%s at x%v: %q is covered by %q at its logical centre", a.nav.screen(), s, forward, h.ID)
			}
			break
		}
	}
	f.Cur.Mx, f.Cur.My = -50, -50
}

func TestSetupScreensDrawAndHitAtUIScales(t *testing.T) {
	screens := []struct {
		name, forward string
		go_           func(t *testing.T, a *App, f *fakeInput)
	}{
		{"menu", "menu.newgame", func(t *testing.T, a *App, f *fakeInput) {}},
		{"scenario", "wizard.next", func(t *testing.T, a *App, f *fakeInput) { click(t, a, "menu.newgame") }},
		{"editor", "wizard.back", func(t *testing.T, a *App, f *fakeInput) {
			click(t, a, "menu.newgame")
			click(t, a, "scen.edit")
		}},
		{"storyteller", "wizard.next", func(t *testing.T, a *App, f *fakeInput) {
			click(t, a, "menu.newgame")
			click(t, a, "wizard.next")
		}},
		{"worldgen", "world.generate", func(t *testing.T, a *App, f *fakeInput) { toWorld(t, a) }},
		{"landing", "land.random", func(t *testing.T, a *App, f *fakeInput) { toLanding(t, a) }},
		{"pawns", "pawns.reroll", func(t *testing.T, a *App, f *fakeInput) { toPawns(t, a) }},
	}
	for _, sc := range screens {
		for _, s := range []float64{1.5, 2.0} {
			t.Run(fmt.Sprintf("%s/x%v", sc.name, s), func(t *testing.T) {
				a, f, _ := ngTest(t, nil)
				sc.go_(t, a, f)
				f.Frame(t, a)
				checkScaledScreen(t, a, f, s, sc.forward)
			})
		}
	}
}

// The setup backdrop is a full-screen background: it bakes and draws at
// the physical size even though the screen's logical size is smaller.
func TestSetupBackdropFillsPhysicalScreen(t *testing.T) {
	a, f, _ := ngTest(t, nil)
	resetSetupBackdrop()
	pw, ph := atLogical1280(t, a, f, 1.5)
	a.drawSetupBackdrop(ebiten.NewImage(pw, ph), a.sw, a.sh, nil)
	setupBackdropMu.Lock()
	defer setupBackdropMu.Unlock()
	if setupBackdrop.starW != pw || setupBackdrop.starH != ph {
		t.Fatalf("starfield %dx%d, want the physical %dx%d", setupBackdrop.starW, setupBackdrop.starH, pw, ph)
	}
}
