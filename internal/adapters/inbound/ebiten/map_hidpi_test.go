package ebiten

import (
	"math"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/perf"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// PLAY-012 B.1/B.4 (HIDPI-MAP): the map renders at physical resolution.
// Its tile size is TileSize x zoom x dev, the UI scale never zooms it, and
// the camera, clicks and the minimap work in physical pixels. These tests
// change curUIScale and must not run in parallel.

// withMapScale sets curUIScale to a device density and a UI scale for the
// rest of the test.
func withMapScale(t testing.TB, dev, ui float64) {
	t.Helper()
	old := gfx.CurrentScale()
	gfx.SetScale(gfx.Scale{Dev: dev, UI: ui, S: dev * ui})
	t.Cleanup(func() { gfx.SetScale(old) })
}

func TestMapTileScalesWithDevOnly(t *testing.T) {
	r := New(&stubSim{w: 40, h: 40})
	r.cam.Zoom = 1
	withMapScale(t, 1, 1.5) // the UI scale alone
	r.SyncDev()
	if got := r.cam.Tile(); got != camerapkg.TileSize {
		t.Fatalf("ui 1.5, dev 1: tile %v, want %v (the UI scale never zooms the map)", got, camerapkg.TileSize)
	}
	withMapScale(t, 2, 1)
	r.SyncDev()
	if got := r.cam.Tile(); got != 2*camerapkg.TileSize {
		t.Fatalf("dev 2: tile %v, want %v", got, 2*camerapkg.TileSize)
	}
	// A bake's camera (lod.go withBake) has no density: px per tile as given.
	if c := (camerapkg.Camera{Zoom: 0.5}); c.Tile() != camerapkg.TileSize/2 {
		t.Fatalf("bake camera tile %v", c.Tile())
	}
}

// TestDevChangeKeepsTheView: switching HiDPI (or moving to another
// screen) keeps the same world point under every point-space position.
func TestDevChangeKeepsTheView(t *testing.T) {
	r := New(&stubSim{w: 40, h: 40})
	r.cam = camerapkg.Camera{Zoom: 1.3, OffsetX: 210, OffsetY: 95, Placed: true}
	withMapScale(t, 1, 1)
	r.SyncDev()
	wx, wy := r.screenToWorld(300, 200)
	withMapScale(t, 2, 1)
	r.SyncDev()
	if gx, gy := r.screenToWorld(600, 400); math.Abs(gx-wx) > 1e-9 || math.Abs(gy-wy) > 1e-9 {
		t.Fatalf("after dev 1 -> 2: (%v,%v), want (%v,%v)", gx, gy, wx, wy)
	}
}

// hidpiColony is a live colony App on a dev x devFn screen with a
// 1280x720-point window and the UI scale ui (percent, 0: Auto).
func hidpiColony(t *testing.T, dev float64, ui int) (*App, *fakeInput) {
	t.Helper()
	t.Cleanup(func() { gfx.SetScale(gfx.UnitScale) })
	s := &stubSim{w: 40, h: 40, pawns: []game.PawnView{{ID: 7, Name: "Ada", X: 21, Y: 19, Status: "alive"}}}
	a := testApp(s)
	a.devFn = func() float64 { return dev }
	st := a.settings
	if ui > 0 {
		st.Accessibility.UIScaleAuto, st.Accessibility.UIScale = false, ui
	}
	a.setSettings(st)
	a.LayoutF(1280, 720)
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != ports.ScreenColony {
		t.Fatalf("screen = %s, want colony", a.nav.screen())
	}
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = -50, -50
	f.Frame(t, a)
	a.renderer.speed = speedPaused
	return a, f
}

func TestHiDPIClickSelectsSameTile(t *testing.T) {
	one, _ := hidpiColony(t, 1, 0)
	if one.scale.S != 1 || one.renderer.cam.Tile() != camerapkg.TileSize {
		t.Fatalf("dev 1: scale %+v tile %v", one.scale, one.renderer.cam.Tile())
	}
	sx1, sy1 := one.renderer.worldToScreenF(21.5, 19.5)

	two, f := hidpiColony(t, 2, 0)
	if two.scale.Dev != 2 || two.renderer.cam.Tile() != 2*camerapkg.TileSize {
		t.Fatalf("dev 2: scale %+v tile %v, want %v", two.scale, two.renderer.cam.Tile(), 2*camerapkg.TileSize)
	}
	sx2, sy2 := two.renderer.worldToScreenF(21.5, 19.5)
	if math.Abs(float64(sx2-2*sx1)) > 1 || math.Abs(float64(sy2-2*sy1)) > 1 {
		t.Fatalf("pawn drawn at (%v,%v) at dev 2, want 2 x (%v,%v)", sx2, sy2, sx1, sy1)
	}
	// A raw (physical) click at (x, y) on the dev-2 screen hits the tile a
	// click at (x/2, y/2) hits on the dev-1 screen, and selects the pawn.
	x, y := int(sx2), int(sy2)
	if tx, ty := two.renderer.TileAt(x, y); tx != 21 || ty != 19 {
		t.Fatalf("dev 2 TileAt(%d,%d) = %d,%d", x, y, tx, ty)
	}
	if tx, ty := one.renderer.TileAt(x/2, y/2); tx != 21 || ty != 19 {
		t.Fatalf("dev 1 TileAt(%d,%d) = %d,%d", x/2, y/2, tx, ty)
	}
	f.Cur.Mx, f.Cur.My = x, y
	f.Click(t, two, inputframe.Left, 0)
	if !two.renderer.HasSelect() || two.renderer.Selected() != 7 {
		t.Fatalf("a physical click on the pawn at dev 2 did not select it (select %v %v)", two.renderer.HasSelect(), two.renderer.Selected())
	}
}

func TestUIScaleLeavesMapTileSize(t *testing.T) {
	a, _ := hidpiColony(t, 1, 150)
	if a.scale.S != 1.5 || a.scale.Dev != 1 {
		t.Fatalf("scale %+v, want ui 1.5 at dev 1", a.scale)
	}
	if got := a.renderer.cam.Tile(); got != camerapkg.TileSize*a.renderer.cam.Zoom {
		t.Fatalf("tile %v at zoom %v: the UI scale zoomed the map", got, a.renderer.cam.Zoom)
	}
	// The map's viewport is the physical screen: the map fills all of it
	// and the HUD floats over it (PLAY-018), so its edges are the physical
	// size, not the logical one.
	v := a.mapViewport()
	if v.X0 != 0 || v.Y0 != 0 || v.X1 != float64(a.pw) || v.Y1 != float64(a.ph) {
		t.Fatalf("map viewport %+v, want the physical screen 0,0-%d,%d", v, a.pw, a.ph)
	}
	if a.pw == a.sw {
		t.Fatalf("physical width %d equals the logical %d at scale 1.5: the check proves nothing", a.pw, a.sw)
	}
}

func TestCameraClampCoversPhysicalViewport(t *testing.T) {
	one, _ := hidpiColony(t, 1, 0)
	two, _ := hidpiColony(t, 2, 0)
	v1, v2 := one.mapViewport(), two.mapViewport()
	if v2.W() != 2*v1.W() || v2.H() != 2*v1.H() {
		t.Fatalf("physical viewports %+v and %+v", v1, v2)
	}
	// Full zoom-out is the same apparent zoom on both screens.
	if z1, z2 := one.renderer.MinZoomFor(v1), two.renderer.MinZoomFor(v2); math.Abs(z1-z2) > 1e-9 {
		t.Fatalf("min zoom %v at dev 1, %v at dev 2", z1, z2)
	}
	r := two.renderer
	r.cam.Zoom = 0.01
	r.ClampCamera(v2)
	mw, mh := 40*r.cam.Tile(), 40*r.cam.Tile()
	if long := math.Max(mw/v2.W(), mh/v2.H()); long < camerapkg.MinCover-1e-9 {
		t.Fatalf("zoomed out, the map is %vx%v in a %vx%v physical viewport", mw, mh, v2.W(), v2.H())
	}
	one.renderer.cam.Zoom = 0.01
	one.renderer.ClampCamera(v1)
	if w1 := 40 * one.renderer.cam.Tile(); math.Abs(mw-2*w1) > 1e-6 {
		t.Fatalf("zoomed out, the map is %v physical px at dev 2, %v at dev 1", mw, w1)
	}
	// Pushed far past the bottom-right, the map's far edges stay within
	// the slack of the physical viewport's edges.
	r.cam.Zoom = 1
	r.cam.OffsetX, r.cam.OffsetY = 1e6, 1e6
	r.ClampCamera(v2)
	t2 := r.cam.Tile()
	if right := 40*t2 - r.cam.OffsetX; right < v2.X1-camerapkg.EdgeSlack*v2.W()-1e-6 {
		t.Fatalf("map right edge %v, viewport right %v", right, v2.X1)
	}
	if bottom := 40*t2 - r.cam.OffsetY; bottom < v2.Y1-camerapkg.EdgeSlack*v2.H()-1e-6 {
		t.Fatalf("map bottom edge %v, viewport bottom %v", bottom, v2.Y1)
	}
}

func TestZoomToCursorAnchorHiDPI(t *testing.T) {
	a, f := hidpiColony(t, 2, 0)
	r := a.renderer
	r.smooth.Instant = true
	x, y := 1700, 900 // physical, inside the map viewport
	wx, wy := r.screenToWorld(x, y)
	f.Cur.Mx, f.Cur.My, f.Cur.WheelY = x, y, 2
	f.Frame(t, a)
	if r.cam.Zoom <= 1 {
		t.Fatalf("zoom %v after a wheel step in", r.cam.Zoom)
	}
	if gx, gy := r.screenToWorld(x, y); math.Abs(gx-wx) > 0.01 || math.Abs(gy-wy) > 0.01 {
		t.Fatalf("the cursor's world point drifted: (%v,%v), want (%v,%v)", gx, gy, wx, wy)
	}
}

func TestMinimapClickAtUIScale(t *testing.T) {
	withMapScale(t, 1, 1.5)
	const sw, sh = 1280, 720 // logical
	lv, pv := camerapkg.ColonyViewport(sw, sh), camerapkg.ColonyViewportPhysical(sw, sh, 1.5)
	r := New(&stubSim{w: 200, h: 200, world: game.WorldView{TerrainW: 200, Terrain: make([]uint8, 200*200)}})
	r.cam.Zoom = 1
	r.ClampCamera(pv)
	_, _, mx, my, mw, mh := r.minimapBox(lv) // the box is UI: logical
	cx, cy := mx+mw*0.25, my+mh*0.75
	if !r.minimapClick(int(cx), int(cy), lv, pv) {
		t.Fatal("a logical click on the minimap not taken")
	}
	tile := r.cam.Tile()
	centreX := (r.cam.OffsetX + (pv.X0+pv.X1)/2) / tile
	centreY := (r.cam.OffsetY + (pv.Y0+pv.Y1)/2) / tile
	if math.Abs(centreX-50) > 1.5 || math.Abs(centreY-150) > 1.5 {
		t.Fatalf("camera centred on (%.1f, %.1f), want (50, 150)", centreX, centreY)
	}
}

func TestFPSReportNamesScale(t *testing.T) {
	st := &perf.Probe{}
	if got := perf.Report(st, gfx.Scale{Dev: 2, UI: 1.5, S: 3, World: 0.5}); !strings.Contains(got, "s 3") || !strings.Contains(got, "dev 2") ||
		!strings.Contains(got, "world 0.5") {
		t.Fatalf("report %q does not name the scale", got)
	}
}

// TestGlobeAreaScales: the globe lays out in a logical rectangle, draws at
// physical size inside it (x s) and picks with the raw cursor.
func TestGlobeAreaScales(t *testing.T) {
	g := testGlobe(t)
	tile := landTile(t, g)
	centreNow(g, tile)
	g.SetArea(100, 50, 600, 400) // logical
	withMapScale(t, 1, 1.5)
	if x, y, w, h := g.AreaRect(); x != 150 || y != 75 || w != 900 || h != 600 {
		t.Fatalf("areaRect at s=1.5 = %v,%v %vx%v, want the logical rect x 1.5", x, y, w, h)
	}
	// The raw cursor at the rect's physical centre picks the centred tile.
	if got := g.PickTile(int(400*1.5), int(250*1.5)); got != tile {
		t.Fatalf("pick at the physical centre = %d, want %d", got, tile)
	}
	screen := ebiten.NewImage(1920, 1080)
	g.Draw(screen, 1280, 720)
	if g.StarW != 1920 || g.StarH != 1080 {
		t.Fatalf("starfield %dx%d, want the physical screen", g.StarW, g.StarH)
	}
	// An image-bound globe (the setup backdrop) is physical and unscaled.
	g.SetAreaPx(0, 0, 300, 300)
	if x, y, w, h := g.AreaRect(); x != 0 || y != 0 || w != 300 || h != 300 {
		t.Fatalf("SetAreaPx rect scaled: %v,%v %vx%v", x, y, w, h)
	}
}
