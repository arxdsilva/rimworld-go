package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/config"
)

// PLAY-012 B.5: the UI scale core. These tests change the package's
// curUIScale and must not run in parallel.

// withUIScale sets curUIScale to s for the rest of the test (dev 1) and
// restores it after. Not for parallel tests.
func withUIScale(t testing.TB, s float64) {
	t.Helper()
	old := gfx.CurrentScale()
	gfx.SetScale(uiScaleState{Dev: 1, UI: s, S: s})
	t.Cleanup(func() { gfx.SetScale(old) })
}

func TestAutoUIScale(t *testing.T) {
	for _, c := range []struct {
		w, h float64
		want float64
	}{
		{1280, 720, 1.0},
		{1920, 1080, 1.5},
		{2560, 1440, 2.0},
		{1066, 600, 1.0},  // below the baseline: never shrinks
		{3840, 2160, 2.0}, // capped
		{0, 0, 1.0},       // unknown window
		{1680, 1050, 1.3}, // 16:10: the width limits it, so 1280 logical px still fit
		{1600, 900, 1.25},
		{1340, 760, 1.0}, // floored to 0.05: never below the 1280x720 minimum
	} {
		if got := autoUIScale(c.w, c.h); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("autoUIScale(%v, %v) = %v, want %v", c.w, c.h, got, c.want)
		}
		if ui := autoUIScale(c.w, c.h); c.w >= minSetupW && c.h >= minSetupH && (c.w/ui < minSetupW-0.01 || c.h/ui < minSetupH-0.01) {
			t.Errorf("auto %v on %vx%v leaves less than the minimum logical window", ui, c.w, c.h)
		}
	}
}

func TestComputeUIScale(t *testing.T) {
	s := config.Defaults()
	s.Accessibility.UIScaleAuto = false
	s.Accessibility.UIScale = 150
	if got := computeUIScale(s, 1280, 720, 1, false); got != (uiScaleState{Dev: 1, UI: 1.5, S: 1.5, World: 1}) {
		t.Fatalf("preset 150 = %+v", got)
	}
	if got := computeUIScale(s, 1280, 720, 2, false); got != (uiScaleState{Dev: 2, UI: 1.5, S: 3, World: 1}) {
		t.Fatalf("preset 150 on a Retina screen = %+v", got)
	}
	s.Graphics.HiDPI = false
	if got := computeUIScale(s, 1280, 720, 2, false); got != (uiScaleState{Dev: 1, UI: 1.5, S: 1.5, World: 1}) {
		t.Fatalf("HiDPI off = %+v", got)
	}
	s = config.Defaults() // Auto
	if got := computeUIScale(s, 1920, 1080, 1, false); got != (uiScaleState{Dev: 1, UI: 1.5, S: 1.5, World: 1}) {
		t.Fatalf("auto at 1080p = %+v", got)
	}
	if got := computeUIScale(s, 1920, 1080, 2, true); got != unitUIScale {
		t.Fatalf("capture = %+v, want unit", got)
	}
}

func scaledTestApp(t *testing.T, dev float64, mutate func(*config.Settings)) *App {
	t.Helper()
	t.Cleanup(func() { gfx.SetScale(unitUIScale) })
	a := testApp(&stubSim{w: 8, h: 8})
	a.devFn = func() float64 { return dev }
	s := a.settings
	if mutate != nil {
		mutate(&s)
	}
	a.setSettings(s)
	return a
}

func TestLayoutFHiDPI(t *testing.T) {
	a := scaledTestApp(t, 2, nil) // Auto, HiDPI on, a Retina screen
	pw, ph := a.LayoutF(1280, 720)
	if pw != 2560 || ph != 1440 {
		t.Fatalf("LayoutF = %vx%v, want 2560x1440 physical", pw, ph)
	}
	if a.pw != 2560 || a.ph != 1440 || a.sw != 1280 || a.sh != 720 || a.scale.S != 2 || a.scale.Dev != 2 {
		t.Fatalf("physical %dx%d logical %dx%d scale %+v", a.pw, a.ph, a.sw, a.sh, a.scale)
	}
	if lw, lh := a.Layout(1280, 720); lw != 2560 || lh != 1440 {
		t.Fatalf("Layout = %dx%d, want the LayoutF size rounded", lw, lh)
	}

	off := scaledTestApp(t, 2, func(s *config.Settings) { s.Graphics.HiDPI = false })
	if pw, ph := off.LayoutF(1280, 720); pw != 1280 || ph != 720 || off.scale.S != 1 {
		t.Fatalf("HiDPI off: LayoutF %vx%v scale %+v", pw, ph, off.scale)
	}

	big := scaledTestApp(t, 1, nil)
	big.LayoutF(1920, 1080)
	if big.sw != 1280 || big.sh != 720 || big.pw != 1920 {
		t.Fatalf("auto 1080p: logical %dx%d physical %d", big.sw, big.sh, big.pw)
	}
	// A settings change applies live, with no new LayoutF.
	s := big.settings
	s.Accessibility.UIScaleAuto, s.Accessibility.UIScale = false, 100
	big.setSettings(s)
	if big.scale.S != 1 || big.sw != 1920 || big.sh != 1080 {
		t.Fatalf("after preset 100: scale %+v logical %dx%d", big.scale, big.sw, big.sh)
	}
}

func TestCaptureForcesUnitScale(t *testing.T) {
	t.Cleanup(func() { gfx.SetScale(unitUIScale) })
	a := NewApp(AppConfig{Version: "test", Capture: &Capture{},
		LoadArt:       func() (*artCatalog, error) { return nil, nil },
		LoadSplash:    func() (*ebiten.Image, error) { return nil, nil },
		LoadMenuScene: func() (*menuDiorama, error) { return nil, nil },
	})
	a.devFn = func() float64 { return 2 }
	s := a.settings
	s.Accessibility.UIScaleAuto, s.Accessibility.UIScale = false, 200
	a.setSettings(s)
	if pw, ph := a.LayoutF(1920, 1080); pw != 1920 || ph != 1080 || a.scale != unitUIScale || a.sw != 1920 {
		t.Fatalf("capture LayoutF %vx%v scale %+v logical %d", pw, ph, a.scale, a.sw)
	}
	if a.scale.World != 1 {
		t.Fatalf("capture world %v, want 1", a.scale.World)
	}
	// dev escape hatch: an explicit -render-scale (captures otherwise load
	// the defaults) shows the render-scaled map in a screenshot
	s.Graphics.RenderScale = 0.5
	a.setSettings(s)
	if a.scale.WorldK() != 0.5 || a.scale.S != 1 || a.pw != 1920 {
		t.Fatalf("capture with render scale 0.5: scale %+v physical %d", a.scale, a.pw)
	}
}

func TestTextMeasureScaleInvariant(t *testing.T) {
	strs := []string{"Broken Hull", "New Colony", "Supercalifragilistic", "A wanderer joins: Ilse, 31, cook"}
	styles := []textStyle{tsSmall, tsBody, tsLabel, tsLarge, tsTitle}
	type m struct{ w, lh float64 }
	at := func(s float64) []m {
		withUIScale(t, s)
		var out []m
		for _, st := range styles {
			for _, str := range strs {
				w, _ := measure(str, st)
				out = append(out, m{w, lineH(st)})
			}
		}
		return out
	}
	one, two, oneHalf := at(1), at(2), at(1.5)
	for i := range one {
		for _, o := range []m{two[i], oneHalf[i]} {
			if math.Abs(one[i].w-o.w) > 1 || math.Abs(one[i].lh-o.lh) > 1 {
				t.Errorf("#%d: s=1 %+v vs scaled %+v", i, one[i], o)
			}
		}
	}
	// Text rasterises at the real pixel size: whole pixels, never above
	// size x s, exactly the style size at s = 1.
	for _, c := range []struct{ size, s, px float64 }{{13, 1, 13}, {13, 2, 26}, {13, 1.5, 19}, {11, 1.25, 13}, {22, 1.75, 38}} {
		if px, k := gfx.FaceSize(c.size, c.s); px != c.px || k != c.px/c.size {
			t.Errorf("faceSize(%v, %v) = %v, %v; want %v", c.size, c.s, px, k, c.px)
		}
	}
	withUIScale(t, 2)
	lines := wrapPx("three crew members survive a hull breach and ride the lifeboats down", tsBody, 120)
	withUIScale(t, 1)
	if want := wrapPx("three crew members survive a hull breach and ride the lifeboats down", tsBody, 120); len(lines) != len(want) {
		t.Errorf("wrap at s=2 %q, at s=1 %q", lines, want)
	}
}

// traceCanvas records the canvas's physical rectangles for the test.
func traceCanvas(t *testing.T) *[]string {
	t.Helper()
	var got []string
	gfx.SetCanvasTrace(func(kind string, x, y, w, h float32) {
		got = append(got, fmt.Sprintf("%s %g,%g %gx%g", kind, x, y, w, h))
	})
	t.Cleanup(func() { gfx.SetCanvasTrace(nil) })
	return &got
}

func TestButtonDrawnAndHitAtScale(t *testing.T) {
	// Drawn: a 100x40 button at (20, 20) covers (30, 30) 150x60 physical.
	withUIScale(t, 1.5)
	got := traceCanvas(t)
	img := ebiten.NewImage(300, 150)
	drawButton(img, button{x: 20, y: 20, w: 100, h: 40, label: "New Colony"}, true)
	want := []string{"fill 30,30 150x60", "stroke 30,30 150x60"}
	if len(*got) != 2 || (*got)[0] != want[0] || (*got)[1] != want[1] {
		t.Fatalf("drawButton at s=1.5 drew %q, want %q", *got, want)
	}

	// Hit: a raw (physical) click at the logical centre x 1.5 presses the
	// first main-menu button.
	a := scaledTestApp(t, 1, func(s *config.Settings) {
		s.Accessibility.UIScaleAuto, s.Accessibility.UIScale = false, 150
	})
	a.LayoutF(1920, 1080)
	a.finishBoot()
	f := fakeOn(a)
	var b button
	for _, x := range a.mainMenuButtons(a.sw, a.sh) {
		if x.id == "menu.newgame" {
			b = x
		}
	}
	cx, cy := b.x+b.w/2, b.y+b.h/2
	f.cur.mx, f.cur.my = int(cx*1.5), int(cy*1.5)
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if mx, my := a.currentCursor(); math.Abs(float64(mx)-float64(cx)) > 1 || math.Abs(float64(my)-float64(cy)) > 1 {
		t.Fatalf("ui cursor (%d,%d), want the logical centre (%v,%v)", mx, my, cx, cy)
	}
	if px, py := a.mapCursor(); px != int(cx*1.5) || py != int(cy*1.5) {
		t.Fatalf("map cursor (%d,%d), want raw physical", px, py)
	}
	if a.nav.screen() != screenScenario {
		t.Fatalf("after a scaled click on New Colony: %s, want scenario", a.nav.screen())
	}
	screen := ebiten.NewImage(a.pw, a.ph)
	a.Draw(screen) // draws at the physical size with the logical layout
	if a.sw != 1280 || a.sh != 720 {
		t.Fatalf("Draw kept logical %dx%d, want 1280x720", a.sw, a.sh)
	}
}

func TestColonyViewportPhysical(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		l := colonyViewport(1280, 720)
		p := colonyViewportPhysical(1280, 720, s)
		if p.x0 != l.x0*s || p.y0 != l.y0*s || p.x1 != l.x1*s || p.y1 != l.y1*s {
			t.Fatalf("s=%v: physical %+v, logical %+v", s, p, l)
		}
	}
}

func TestCanvasHelpersScale(t *testing.T) {
	withUIScale(t, 2)
	got := traceCanvas(t)
	img := ebiten.NewImage(100, 100)
	uiFillRect(img, 10, 10, 10, 10, color.White, false)
	uiStrokeRect(img, 1, 2, 3, 4, 1, color.White, false)
	src := ebiten.NewImage(4, 4)
	uiImage(img, src, 5, 5, 10, 10, nil) // logical 5..15 -> physical 10..30
	want := []string{"fill 20,20 20x20", "stroke 2,4 6x8", "image 10,10 20x20"}
	if fmt.Sprint(*got) != fmt.Sprint(want) {
		t.Fatalf("canvas drew %q, want %q", *got, want)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(3, 4)
	uiImageOp(op)
	if x, y := op.GeoM.Apply(0, 0); x != 6 || y != 8 {
		t.Fatalf("uiImageOp origin = (%v,%v), want (6,8)", x, y)
	}
	op2 := &ebiten.DrawImageOptions{}
	op2.GeoM.Translate(3, 4)
	uiDrawImage(img, src, op2)
	if x, _ := op2.GeoM.Apply(0, 0); x != 3 {
		t.Fatal("uiDrawImage changed the caller's op")
	}
	vs := []ebiten.Vertex{{DstX: 1, DstY: 2}, {DstX: 3, DstY: 4}, {DstX: 5, DstY: 6}}
	if sv := gfx.ScaledVerts(vs); sv[0].DstX != 2 || sv[2].DstY != 12 {
		t.Fatalf("scaledVerts = %+v", sv)
	}
	uiTriangles(img, vs, []uint16{0, 1, 2}, src, nil)
	uiTriangles32(img, vs, []uint32{0, 1, 2}, src, nil)
	if vs[0].DstX != 1 || vs[2].DstY != 6 {
		t.Fatal("uiTriangles changed the caller's vertices")
	}
	uiLine(img, 0, 0, 1, 1, 1, color.White, true)
	uiFillCircle(img, 5, 5, 2, color.White, true)
	uiStrokeCircle(img, 5, 5, 2, 1, color.White, true)
	uiFillPoly(img, [][2]float32{{0, 0}, {4, 0}, {0, 4}}, color.White, true)
	uiFillPoly(img, nil, color.White, true) // too few points: nothing
	if r := uiPhysRect(rect{X: 1, Y: 2, W: 3, H: 4}); r != (rect{X: 2, Y: 4, W: 6, H: 8}) {
		t.Fatalf("uiPhysRect = %+v", r)
	}
	if c := clipTo(img, rect{X: 10, Y: 10, W: 20, H: 20}); c.Bounds() != image.Rect(20, 20, 60, 60) {
		t.Fatalf("clipTo at s=2 = %v, want the physical rectangle", c.Bounds())
	}
}
