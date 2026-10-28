package ebiten

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
)

// PERF-002 hot spot 6: the world render scale (world_scale.go).

// renderScaleColony is hidpiColony with the map's render scale k: a live
// colony on a dev x screen, a 1280x720-point window at UI scale 100 %.
func renderScaleColony(t *testing.T, dev, k float64) (*App, *fakeInput) {
	t.Helper()
	t.Cleanup(func() { gfx.SetScale(unitUIScale) })
	s := &stubSim{w: 40, h: 40, pawns: []game.PawnView{{ID: 7, Name: "Ada", X: 21, Y: 19, Status: "alive"}}}
	a := testApp(s)
	a.devFn = func() float64 { return dev }
	st := a.settings
	st.Accessibility.UIScaleAuto, st.Accessibility.UIScale = false, 100
	st.Graphics.HiDPI, st.Graphics.RenderScale = true, k
	a.setSettings(st)
	a.LayoutF(1280, 720)
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != screenColony {
		t.Fatalf("screen = %s, want colony", a.nav.screen())
	}
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = -50, -50
	f.Frame(t, a)
	a.renderer.speed = speedPaused
	return a, f
}

// drawOnce draws one App frame on a physical-size screen.
func drawOnce(t *testing.T, a *App) *ebiten.Image {
	t.Helper()
	screen := ebiten.NewImage(a.pw, a.ph)
	t.Cleanup(screen.Deallocate)
	a.Draw(screen)
	return screen
}

func TestComputeRenderScale(t *testing.T) {
	s := config.Defaults()
	s.Graphics.RenderScale = 0.5
	for _, c := range []struct {
		hidpi   bool
		dev     float64
		capture bool
		want    float64
	}{
		{true, 2, false, 0.5},
		{true, 1.5, false, 1 / 1.5}, // never below one map pixel per point
		{true, 1, false, 1},
		{false, 2, false, 1}, // HiDPI off: dev 1
		{true, 2, true, 1},   // captures
	} {
		s.Graphics.HiDPI = c.hidpi
		if got := computeUIScale(s, 1280, 720, c.dev, c.capture).WorldK(); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("hidpi %v dev %g capture %v: world %g, want %g", c.hidpi, c.dev, c.capture, got, c.want)
		}
	}
	s.Graphics.HiDPI, s.Graphics.RenderScale = true, 0.6 // not a choice
	if got := computeUIScale(s, 1280, 720, 2, false).WorldK(); got != 1 {
		t.Errorf("render scale 0.6: world %g, want 1", got)
	}
}

// TestRenderScaleOffscreenSize: at dev 2 and Half the screen stays
// physical, the map draws into a 1280x720 offscreen at one map pixel per
// point, and at Full no offscreen exists.
func TestRenderScaleOffscreenSize(t *testing.T) {
	a, _ := renderScaleColony(t, 2, 0.5)
	if a.pw != 2560 || a.ph != 1440 || a.sw != 1280 {
		t.Fatalf("LayoutF: physical %dx%d logical %d, want 2560x1440 and 1280", a.pw, a.ph, a.sw)
	}
	drawOnce(t, a)
	if a.wr.img == nil || a.wr.img.Bounds().Dx() != 1280 || a.wr.img.Bounds().Dy() != 720 {
		t.Fatalf("offscreen %v, want 1280x720", a.wr.img)
	}
	if a.renderer.cam.Dev != 1 {
		t.Fatalf("camera density %v, want dev x world = 1", a.renderer.cam.Dev)
	}
	if gfx.CurrentPass() != nil || gfx.WorldPassK() != 0 {
		t.Fatal("the world pass did not end")
	}
	st := a.settings
	st.Graphics.RenderScale = 1
	a.setSettings(st)
	drawOnce(t, a)
	if a.wr.img != nil || a.renderer.cam.Dev != 2 {
		t.Fatalf("at Full: offscreen %v camera density %v, want none and 2", a.wr.img, a.renderer.cam.Dev)
	}
	// odd sizes round the offscreen up
	a.LayoutF(641, 361)
	st.Graphics.RenderScale = 0.75
	a.setSettings(st)
	drawOnce(t, a)
	if b := a.wr.img.Bounds(); b.Dx() != int(math.Ceil(1282*0.75)) || b.Dy() != int(math.Ceil(722*0.75)) {
		t.Fatalf("offscreen %v for a 1282x722 screen at 3/4", b)
	}
}

// TestRenderScaleKeepsCursorMapping: at Half a point is drawn at half its
// physical position in the offscreen, which the composite doubles, and a
// click (raw or logical cursor) picks the same tile and pawn as at Full.
func TestRenderScaleKeepsCursorMapping(t *testing.T) {
	full, _ := renderScaleColony(t, 2, 1)
	sx1, sy1 := full.renderer.worldToScreenF(21.5, 19.5) // physical
	full.cursor = &[2]int{int(sx1 / 2), int(sy1 / 2)}    // logical (s = 2)
	ftx, fty := full.renderer.TileAt(full.mapCursor())

	half, f := renderScaleColony(t, 2, 0.5)
	if half.renderer.cam.Dev != 1 {
		t.Fatalf("camera density %v, want 1", half.renderer.cam.Dev)
	}
	mx, my := half.renderer.worldToScreenF(21.5, 19.5) // map pixels
	if math.Abs(float64(mx/0.5-sx1)) > 1 || math.Abs(float64(my/0.5-sy1)) > 1 {
		t.Fatalf("pawn at map (%v,%v) -> physical (%v,%v), want (%v,%v)", mx, my, mx/0.5, my/0.5, sx1, sy1)
	}
	// a logical cursor: mapCursor -> world -> map -> / world is the
	// physical cursor
	half.cursor = &[2]int{int(sx1 / 2), int(sy1 / 2)}
	cx, cy := half.mapCursor()
	wx, wy := half.renderer.screenToWorld(cx, cy)
	px, py := half.renderer.worldToScreenF(wx, wy)
	if lx, ly := float64(half.cursor[0]*2), float64(half.cursor[1]*2); math.Abs(float64(px)/0.5-lx) > 1 || math.Abs(float64(py)/0.5-ly) > 1 {
		t.Fatalf("logical cursor %v -> physical (%v,%v), want (%v,%v)", *half.cursor, float64(px)/0.5, float64(py)/0.5, lx, ly)
	}
	if tx, ty := half.renderer.TileAt(cx, cy); tx != ftx || ty != fty || tx != 21 || ty != 19 {
		t.Fatalf("tile under the logical cursor %d,%d at Half, %d,%d at Full, want 21,19", tx, ty, ftx, fty)
	}
	// a raw (physical) click on the pawn selects it
	half.cursor = nil
	f.Cur.Mx, f.Cur.My = int(sx1), int(sy1)
	f.Click(t, half, mbLeft, 0)
	if !half.renderer.HasSelect() || half.renderer.Selected() != 7 {
		t.Fatalf("a physical click on the pawn at Half did not select it (select %v %v)", half.renderer.HasSelect(), half.renderer.Selected())
	}
	// the map's viewport is the screen in map pixels
	if v := half.mapViewport(); v.X1 != 1280 || v.Y1 != 720 {
		t.Fatalf("map viewport %+v, want 1280x720 map pixels", v)
	}
}

// TestRenderScaleToggleKeepsTheView: switching Full -> Half keeps the tile
// under every screen point (syncDev rescales the camera, like HiDPI).
func TestRenderScaleToggleKeepsTheView(t *testing.T) {
	r := New(&stubSim{w: 40, h: 40})
	r.cam = camera{Zoom: 1.3, OffsetX: 410, OffsetY: 195, Placed: true}
	old := gfx.CurrentScale()
	t.Cleanup(func() { gfx.SetScale(old) })
	gfx.SetScale(uiScaleState{Dev: 2, UI: 1, S: 2, World: 1})
	r.SyncDev()
	wx, wy := r.screenToWorld(600, 400)
	sc := gfx.CurrentScale()
	sc.World = 0.5
	gfx.SetScale(sc)
	r.SyncDev()
	if gx, gy := r.screenToWorld(300, 200); math.Abs(gx-wx) > 1e-9 || math.Abs(gy-wy) > 1e-9 {
		t.Fatalf("after Full -> Half: (%v,%v), want (%v,%v)", gx, gy, wx, wy)
	}

	a, _ := renderScaleColony(t, 2, 1)
	drawOnce(t, a)
	centre := func() (int, int) {
		v := a.mapViewport()
		return a.renderer.TileAt(int((v.X0+v.X1)/2), int((v.Y0+v.Y1)/2))
	}
	tx, ty := centre()
	st := a.settings
	st.Graphics.RenderScale = 0.5
	a.setSettings(st)
	drawOnce(t, a)
	if gx, gy := centre(); gx != tx || gy != ty {
		t.Fatalf("centre tile %d,%d after Full -> Half, want %d,%d", gx, gy, tx, ty)
	}
}

// rsConfigure is visualScene.configure at dev 2, UI scale 1, zoom 1 and
// the render scale k.
func rsConfigure(t *testing.T, s *visualScene, k float64) *ebiten.Image {
	t.Helper()
	a := s.a
	a.scale = uiScaleState{Dev: 2, UI: 1, S: 2, World: k}
	a.syncUIScale()
	a.setPhysical(2*visLogicalW, 2*visLogicalH)
	r := a.renderer
	r.SyncDev()
	s.update(t)
	r.cam.Zoom = 1
	v := a.mapViewport()
	r.centerOnView(visFocus[0], visFocus[1], v, v)
	key := [2]int{a.pw, a.ph}
	if s.shots[key] == nil {
		s.shots[key] = ebiten.NewImage(a.pw, a.ph)
	}
	return s.shots[key]
}

// TestRenderScaleKeepsTextOnTheScreen: at Half every canvas draw of the
// world pass (zone labels, stack counts, the animal's name, the markers)
// lands on the screen, not the offscreen, at the size and place it has at
// Full.
func TestRenderScaleKeepsTextOnTheScreen(t *testing.T) {
	art := visArt(t)
	s := newVisualScene(t, art, false)
	labels := func(ev []traceEvent) []traceEvent {
		var out []traceEvent
		for _, e := range ev {
			if e.Pass == "world" && (e.Kind == "text" || e.Via != "") {
				out = append(out, e)
			}
		}
		return out
	}
	ref := labels(s.frame(rsConfigure(t, s, 1)))
	if len(ref) < 4 {
		t.Fatalf("the reference frame drew %d world labels", len(ref))
	}
	screen := rsConfigure(t, s, 0.5)
	ev := s.frame(screen)
	if s.a.wr.img == nil {
		t.Fatal("no offscreen at Half")
	}
	for _, e := range ev {
		if e.Pass == "world" && e.Clip != screen.Bounds() {
			t.Fatalf("a world-pass canvas draw went into %v, not the screen: %s", e.Clip, e)
		}
	}
	got := labels(ev)
	if len(got) != len(ref) {
		t.Fatalf("%d world labels at Half, %d at Full:%s\nvs%s", len(got), len(ref), visSummary(got, 20), visSummary(ref, 20))
	}
	for i := range ref {
		r, g := ref[i], got[i]
		// text is laid out in logical pixels: the same box. An icon sized
		// from the tile keeps the map-pixel margin its pass gives it (the
		// chat bubble's 2 px), so it may differ by a few pixels.
		size, pos := 0.5, 2.0
		if r.Kind != "text" {
			size, pos = 4.5, 3
		}
		if r.Text != g.Text || r.Via != g.Via || !visNear(r.W, g.W, size) || !visNear(r.H, g.H, size) ||
			!visNear(r.X, g.X, pos) || !visNear(r.Y, g.Y, pos) {
			t.Fatalf("label moved or resized at Half:\n\tfull %s\n\thalf %s", r, g)
		}
	}
}

// TestMapPixelsComeFromMapCursor: every TileAt / screenToWorld /
// pawnUnderCursor call an App method makes takes map pixels: mapCursor()
// (directly, or the names it was assigned to), a constant, a length
// converted by mapScale(), or a viewport edge (mapViewport is map pixels).
// A raw or logical point would miss the tile at a reduced render scale.
func TestMapPixelsComeFromMapCursor(t *testing.T) {
	files := packageFiles(t, ".")
	fset := token.NewFileSet()
	callsTo := func(n ast.Node, names ...string) bool {
		found := false
		ast.Inspect(n, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if s, ok := c.Fun.(*ast.SelectorExpr); ok {
					for _, nm := range names {
						found = found || s.Sel.Name == nm
					}
				}
			}
			return !found
		})
		return found
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Recv == nil || len(fd.Recv.List) != 1 {
				continue
			}
			if st, ok := fd.Recv.List[0].Type.(*ast.StarExpr); !ok || st.X.(*ast.Ident).Name != "App" {
				continue
			}
			fromCursor := map[string]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if as, ok := n.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && callsTo(as.Rhs[0], "mapCursor") {
					for _, l := range as.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							fromCursor[id.Name] = true
						}
					}
				}
				return true
			})
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				s, ok := c.Fun.(*ast.SelectorExpr)
				if !ok || (s.Sel.Name != "TileAt" && s.Sel.Name != "screenToWorld" && s.Sel.Name != "pawnUnderCursor") {
					return true
				}
				checked++
				for _, arg := range c.Args {
					switch x := arg.(type) {
					case *ast.BasicLit:
						continue
					case *ast.Ident:
						if fromCursor[x.Name] {
							continue
						}
					}
					if callsTo(arg, "mapCursor", "mapScale") || viewportEdge(arg) {
						continue
					}
					t.Errorf("%s: %s.%s argument %s is not map pixels (use mapCursor or mapScale)",
						fset.Position(arg.Pos()), fd.Name.Name, s.Sel.Name, exprString(arg))
				}
				return true
			})
		}
	}
	if checked < 10 {
		t.Fatalf("checked only %d calls: the scan is broken", checked)
	}
}

// viewportEdge reports e reads a viewport's edges (v.x0 ... v.y1).
func viewportEdge(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			switch s.Sel.Name {
			case "x0", "x1", "y0", "y1":
				found = true
			}
		}
		return !found
	})
	return found
}

// exprString is a short form of e for messages.
func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return exprString(x.Fun) + "(...)"
	}
	return "expression"
}

// TestOptionsRenderScaleRow: World detail cycles Full, 3/4, Half, records
// the choice and applies it live.
func TestOptionsRenderScaleRow(t *testing.T) {
	t.Cleanup(func() { gfx.SetScale(unitUIScale) })
	store := graphicsStore()
	a := optionsApp(store)
	a.devFn = func() float64 { return 2 }
	a.LayoutF(1280, 720)
	a.optionsTab = "graphics"
	b := optionBtn(t, a, "options.renderscale")
	if b.label != "Full" || !strings.Contains(b.tip, "Half") || a.scale.WorldK() != 1 {
		t.Fatalf("row %q tip %q world %g", b.label, b.tip, a.scale.WorldK())
	}
	for _, want := range []struct {
		label string
		k     float64
	}{{"3/4", 0.75}, {"Half", 0.5}, {"Full", 1}} {
		a.applyOption("options.renderscale")
		g := store.s.Graphics
		if got := optionBtn(t, a, "options.renderscale").label; got != want.label || g.RenderScale != want.k || a.scale.WorldK() != want.k {
			t.Fatalf("after a click: %q, stored %g, world %g; want %q %g", got, g.RenderScale, a.scale.WorldK(), want.label, want.k)
		}
		if g.Quality != config.QualityCustom || !strings.Contains(strings.Join(g.Chosen, ","), "renderscale") {
			t.Fatalf("the choice is not recorded: %+v", g)
		}
	}
}

// TestRenderScaleFogHidesLabels: before the fog is drawn, a queued label
// over an unexplored tile is dropped (the fog would cover it at Full);
// after it, labels show over the fog as they do at Full.
func TestRenderScaleFogHidesLabels(t *testing.T) {
	a, _ := renderScaleColony(t, 2, 0.5)
	r := a.renderer
	r.view.W.TerrainW = 40
	r.view.W.Fog = make([]uint8, 40*40)
	tx, ty := r.TileAt(640, 360) // the screen's centre, in map pixels
	r.view.W.Fog[ty*40+tx] = 1
	if !a.fogHidesLabel(640, 360) || a.fogHidesLabel(10, 10) {
		t.Fatal("fogHidesLabel does not read the tile under a logical point")
	}
	img := ebiten.NewImage(8, 8)
	t.Cleanup(img.Deallocate)
	a.beginWorldPass(img)
	t.Cleanup(a.endWorldPass)
	uiFillRect(img, 638, 358, 4, 4, uiAccent, false) // over the fog: dropped
	uiFillRect(img, 8, 8, 4, 4, uiAccent, false)     // explored: queued
	gfx.FogDrawn()
	drawText(img, "after", 640, 360, tsSmall) // after the fog: queued
	if n := a.wr.q.Len(); n != 2 {
		t.Fatalf("%d draws queued, want 2", n)
	}
}
