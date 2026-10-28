package ebiten

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017 §9: double-clicking a colonist in the bar centres the camera on
// it. These run at UI scale 1.5 (the bar is laid out in logical pixels, the
// cursor and the camera are physical) and change curUIScale, so they must
// not run in parallel.

const barPawnID = entity.EntityID(7) // hidpiColony's colonist, at tile (21, 19)

// clickBarSlot left-clicks the colonist's bar slot through the real input
// path (two Updates: press, then release).
func clickBarSlot(t *testing.T, a *App, f *fakeInput, id entity.EntityID) {
	t.Helper()
	for _, b := range a.colonistBarButtons(a.sw, a.sh) {
		if b.id == barSlotID(id) {
			f.cur.mx = int(float64(b.x+b.w/2) * a.scale.S)
			f.cur.my = int(float64(b.y+b.h/2) * a.scale.S)
			f.click(t, a, mbLeft, 0)
			return
		}
	}
	t.Fatalf("no bar slot for pawn %d", id)
}

// barColony is hidpiColony at 150%, with the camera put well away from the
// colonist so a jump is obvious.
func barColony(t *testing.T) (*App, *fakeInput) {
	t.Helper()
	a, f := hidpiColony(t, 1, 150)
	if a.scale.S != 1.5 {
		t.Fatalf("scale = %v, want 1.5", a.scale.S)
	}
	a.renderer.cam.zoom = 1
	a.renderer.cam.offsetX, a.renderer.cam.offsetY = 0, 0
	a.renderer.cam.placed = true
	f.frame(t, a)
	return a, f
}

func pawnCentred(a *App) bool {
	v := a.mapViewport()
	sx, sy := a.renderer.worldToScreenF(21.5, 19.5)
	return math.Abs(float64(sx)-(v.x0+v.x1)/2) <= 1.5 && math.Abs(float64(sy)-(v.y0+v.y1)/2) <= 1.5
}

func TestBarDoubleClickCentresCamera(t *testing.T) {
	a, f := barColony(t)
	if pawnCentred(a) {
		t.Fatal("setup: the colonist is already centred")
	}
	cam := a.renderer.cam
	clickBarSlot(t, a, f, barPawnID)
	if !a.renderer.HasSelect() || a.renderer.Selected() != barPawnID {
		t.Fatal("a single bar click should select the colonist")
	}
	if a.renderer.cam.offsetX != cam.offsetX || a.renderer.cam.offsetY != cam.offsetY {
		t.Fatalf("a single bar click moved the camera: %+v -> %+v", cam, a.renderer.cam)
	}
	clickBarSlot(t, a, f, barPawnID)
	if !pawnCentred(a) {
		sx, sy := a.renderer.worldToScreenF(21.5, 19.5)
		t.Fatalf("after a double click the colonist is drawn at (%v,%v), want the viewport centre of %+v", sx, sy, a.mapViewport())
	}
	if !a.renderer.HasSelect() || a.renderer.Selected() != barPawnID {
		t.Fatal("the double click should leave the colonist selected")
	}
}

func TestBarClicksFurtherApartDoNotCentre(t *testing.T) {
	a, f := barColony(t)
	clickBarSlot(t, a, f, barPawnID)
	for i := 0; i <= doubleClickFrames; i++ {
		f.frame(t, a)
	}
	cam := a.renderer.cam
	clickBarSlot(t, a, f, barPawnID)
	if a.renderer.cam.offsetX != cam.offsetX || a.renderer.cam.offsetY != cam.offsetY || pawnCentred(a) {
		t.Fatalf("two clicks more than %d frames apart moved the camera: %+v -> %+v", doubleClickFrames, cam, a.renderer.cam)
	}
	// ...but the second click is the first of a new pair.
	clickBarSlot(t, a, f, barPawnID)
	if !pawnCentred(a) {
		t.Fatal("a click right after that one should have centred the camera")
	}
}

// A third quick click starts a new pair; it does not centre again.
func TestBarDoubleClickDoesNotChain(t *testing.T) {
	a, f := barColony(t)
	clickBarSlot(t, a, f, barPawnID)
	clickBarSlot(t, a, f, barPawnID)
	if !pawnCentred(a) {
		t.Fatal("the double click should have centred the camera")
	}
	a.renderer.cam.offsetX += 200 // the player pans away
	cam := a.renderer.cam
	clickBarSlot(t, a, f, barPawnID)
	if a.renderer.cam.offsetX != cam.offsetX {
		t.Fatalf("the third click centred again: %+v -> %+v", cam, a.renderer.cam)
	}
}

// A bar click selects the colonist and drops every other selection, like
// selecting on the map does.
func TestBarClickClearsOtherSelections(t *testing.T) {
	a, f := barColony(t)
	a.selAnimal, a.hasAnimal = 90, true
	a.selItem, a.hasItem = 91, true
	a.selPlant, a.hasPlant = 92, true
	a.selFrame, a.hasFrame = 93, true
	a.selBuilding, a.hasBuilding = 94, true
	clickBarSlot(t, a, f, barPawnID)
	if a.hasAnimal || a.hasItem || a.hasPlant || a.hasFrame || a.hasBuilding {
		t.Fatalf("selections left after a bar click: animal %v item %v plant %v frame %v building %v",
			a.hasAnimal, a.hasItem, a.hasPlant, a.hasFrame, a.hasBuilding)
	}
	if !a.renderer.HasSelect() || a.renderer.Selected() != barPawnID {
		t.Fatal("the colonist should be the selection")
	}
}

// The jump kills pan velocity and a zoom ease in flight, so the camera does
// not drift off the colonist afterwards.
func TestBarDoubleClickStopsCameraSmoothing(t *testing.T) {
	a, f := barColony(t)
	clickBarSlot(t, a, f, barPawnID)
	sm := &a.renderer.smooth
	sm.vx, sm.vy = 900, -700
	sm.zooming, sm.targetZoom, sm.anchorX, sm.anchorY = true, 2, 100, 100
	sm.pendingScale = 1.2
	zoom := a.renderer.cam.zoom
	clickBarSlot(t, a, f, barPawnID)
	if sm.vx != 0 || sm.vy != 0 || sm.zooming || sm.pendingScale != 0 {
		t.Fatalf("smoothing not stopped: %+v", *sm)
	}
	for i := 0; i < 20; i++ {
		f.frame(t, a)
	}
	if !pawnCentred(a) || a.renderer.cam.zoom > zoom*1.0001 {
		t.Fatalf("camera drifted after the jump: zoom %v -> %v, centred %v", zoom, a.renderer.cam.zoom, pawnCentred(a))
	}
}

func TestCamSmoothHaltStopsMotion(t *testing.T) {
	t.Parallel()
	s := camSmooth{vx: 5, vy: 6, dirX: 1, dirY: -1, zooming: true, targetZoom: 2, pendingScale: 1.3, speed: 1.5}
	s.halt()
	if s.vx != 0 || s.vy != 0 || s.dirX != 0 || s.dirY != 0 || s.zooming || s.pendingScale != 0 {
		t.Fatalf("halt left motion: %+v", s)
	}
	if s.speed != 1.5 {
		t.Fatalf("halt must keep the pan-speed setting, got %v", s.speed)
	}
}
