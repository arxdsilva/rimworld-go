package ebiten

import (
	"math"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
)

// Camera option journeys (QA-001, OPT-021…029): each option is set on the
// Options screen with the mouse, then the camera is driven with the wheel
// and the middle button the way a player would.

// optToggle clicks a toggle row until its setting reads want, then leaves
// Options.
func optToggle(j *journey, tab, id string, get func() bool, want bool) {
	j.t.Helper()
	optOpen(j, tab)
	if get() != want {
		j.clickID(id)
	}
	if get() != want {
		j.t.Fatalf("%s did not reach %v", id, want)
	}
	j.key("Escape", 0)
	j.key("Escape", 0)
	if j.a.nav.screen() != ports.ScreenColony || j.a.nav.overlay() != overlayNone {
		j.t.Fatalf("Escape twice left %s (overlay %v)", j.a.nav.screen(), j.a.nav.overlay())
	}
}

// TestJourneyCameraZoomToMouseOffZoomsAboutTheMiddle: with Zoom to mouse
// off the wheel keeps the view's middle still, not the tile under the
// pointer.
func TestJourneyCameraZoomToMouseOffZoomsAboutTheMiddle(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(64, 64))
	optToggle(j, "gameplay", "options.zoomtomouse", func() bool { return j.a.settings.Camera.ZoomToMouse }, false)
	j.a.renderer.cam.Zoom = playZoom // room to zoom in
	j.draw(1)
	r := j.a.renderer
	v := j.a.mapViewport()
	midX, midY := int((v.X0+v.X1)/2), int((v.Y0+v.Y1)/2)
	j.f.MoveTo(j.a, 14, 6) // well off the middle
	mx, my := j.f.Cur.Mx, j.f.Cur.My
	mw0x, mw0y := r.screenToWorld(midX, midY)
	cw0x, cw0y := r.screenToWorld(mx, my)
	z0 := r.cam.Zoom
	j.f.Cur.WheelY = 1
	j.update(1)
	j.f.Cur.WheelY = 0
	j.draw(8)
	if r.cam.Zoom <= z0 {
		t.Fatalf("zoom %v after a wheel step, want more than %v", r.cam.Zoom, z0)
	}
	if x, y := r.screenToWorld(midX, midY); math.Abs(x-mw0x) > 0.5 || math.Abs(y-mw0y) > 0.5 {
		t.Fatalf("the middle of the view moved from (%.2f, %.2f) to (%.2f, %.2f)", mw0x, mw0y, x, y)
	}
	if x, y := r.screenToWorld(mx, my); math.Abs(x-cw0x) < 0.05 && math.Abs(y-cw0y) < 0.05 {
		t.Fatalf("the tile under the pointer stayed put: zoomed toward the mouse")
	}
}

// TestJourneyCameraDragSpeed: a middle-button drag at 200 % moves the camera
// twice as far as at 100 %.
func TestJourneyCameraDragSpeed(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	base := middleDrag(j)
	if math.Abs(base+40) > 1 {
		t.Fatalf("a 40 px drag at 100%% moved the camera %v, want -40", base)
	}
	optSetList(j, "gameplay", "options.dragpct", func() int { return j.a.settings.Camera.DragPct }, 200)
	j.draw(1)
	if got := middleDrag(j); math.Abs(got-2*base) > 1 {
		t.Fatalf("a 40 px drag at 200%% moved the camera %v, want %v", got, 2*base)
	}
}

// TestJourneyCameraZoomOutToWorld: zooming out at the floor opens the world
// view only with Zoom to globe on.
func TestJourneyCameraZoomOutToWorld(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	floor := func() {
		r.cam.Zoom = r.MinZoomFor(j.a.mapViewport())
		r.smooth.Zooming = false
		j.f.MoveTo(j.a, 12, 5)
		j.f.Cur.WheelY = -1
		j.update(1)
		j.f.Cur.WheelY = 0
		j.draw(2)
	}
	floor()
	if j.a.world.Open() {
		t.Fatal("zooming out at the floor opened the globe with the option off")
	}
	optToggle(j, "gameplay", "options.zoomoutworld", func() bool { return j.a.settings.Camera.ZoomOutToWorld }, true)
	floor()
	if !j.a.world.Open() {
		t.Fatal("zooming out at the floor did not open the globe")
	}
}

// TestJourneyCameraWindowSizePreset: choosing a size on Graphics tells the
// window to take it.
func TestJourneyCameraWindowSizePreset(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	var sizes []string
	j.a.cfg.Display = func(size string, _ bool) { sizes = append(sizes, size) }
	optSetList(j, "graphics", "options.windowsize", func() string { return j.a.settings.Camera.Window }, "1600x900")
	if len(sizes) == 0 || sizes[len(sizes)-1] != "1600x900" {
		t.Fatalf("the window was asked for %q, want 1600x900 last", sizes)
	}
}

// TestJourneyCameraShakeIsOffInCaptures: a scripted capture never shakes the
// screen, whatever the option says.
func TestJourneyCameraShakeIsOffInCaptures(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.update(1)
	if j.a.settings.Camera.ShakePct <= 0 || j.a.renderer.smooth.Prefs.Shake <= 0 {
		t.Fatalf("a player's game does not shake: option %d%%, strength %v", j.a.settings.Camera.ShakePct, j.a.renderer.smooth.Prefs.Shake)
	}
	j.a.cfg.Capture = &Capture{}
	j.a.syncRenderer()
	j.update(1)
	if s := j.a.renderer.smooth.Prefs.Shake; s != 0 {
		t.Fatalf("shake strength %v in a capture, want 0", s)
	}
	j.a.renderer.smooth.Shake(10)
	if j.a.renderer.smooth.Fx.Active() {
		t.Fatal("a capture frame is shaken")
	}
}

// TestJourneyCameraHatsOnlyOnTheMap: the Interface row takes the hat off the
// portraits (the colonist bar's bust included) and leaves it on the map.
func TestJourneyCameraHatsOnlyOnTheMap(t *testing.T) {
	t.Parallel()
	art := testArt(t)
	j := newJourney(t)
	p := game.PawnView{ID: 1, Apparel: []string{"t-shirt", "pants", "cowboy hat"}}
	hat := func(noHat bool) bool {
		for _, l := range pawnDressLayers(art, p, "south", noHat) {
			if strings.Contains(l.Key, "cowboy_hat") {
				return true
			}
		}
		return false
	}
	if j.a.renderer.view.Prefs.HatsOnlyOnMap || !hat(j.a.renderer.view.Prefs.HatsOnlyOnMap) {
		t.Fatal("a portrait loses its hat by default")
	}
	optToggle(j, "access", "options.hatsonmap", func() bool { return j.a.settings.Interface.HatsOnlyOnMap }, true)
	j.update(1)
	if !j.a.renderer.view.Prefs.HatsOnlyOnMap {
		t.Fatal("the option never reached the renderer")
	}
	if hat(j.a.renderer.view.Prefs.HatsOnlyOnMap) {
		t.Fatal("the portrait still wears its hat")
	}
	var onMap bool
	for _, l := range pawnFigureLayers(art, p, "south") {
		onMap = onMap || strings.Contains(l.Key, "cowboy_hat")
	}
	if !onMap {
		t.Fatal("the map figure lost its hat")
	}
}
