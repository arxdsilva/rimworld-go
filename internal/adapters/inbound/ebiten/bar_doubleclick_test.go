package ebiten

import (
	"math"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
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
	for _, b := range a.chrome.Bar().Buttons(a.sw, a.sh) {
		if b.ID == hud.BarSlotID(id) {
			f.Cur.Mx = int(float64(b.X+b.W/2) * a.scale.S)
			f.Cur.My = int(float64(b.Y+b.H/2) * a.scale.S)
			f.Click(t, a, inputframe.Left, 0)
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
	a.renderer.cam.Zoom = 1
	a.renderer.cam.OffsetX, a.renderer.cam.OffsetY = 0, 0
	a.renderer.cam.Placed = true
	f.Frame(t, a)
	return a, f
}

func pawnCentred(a *App) bool {
	v := a.mapViewport()
	sx, sy := a.renderer.worldToScreenF(21.5, 19.5)
	return math.Abs(float64(sx)-(v.X0+v.X1)/2) <= 1.5 && math.Abs(float64(sy)-(v.Y0+v.Y1)/2) <= 1.5
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
	if a.renderer.cam.OffsetX != cam.OffsetX || a.renderer.cam.OffsetY != cam.OffsetY {
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
	for i := 0; i <= selectionpkg.DoubleClickFrames; i++ {
		f.Frame(t, a)
	}
	cam := a.renderer.cam
	clickBarSlot(t, a, f, barPawnID)
	if a.renderer.cam.OffsetX != cam.OffsetX || a.renderer.cam.OffsetY != cam.OffsetY || pawnCentred(a) {
		t.Fatalf("two clicks more than %d frames apart moved the camera: %+v -> %+v", selectionpkg.DoubleClickFrames, cam, a.renderer.cam)
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
	a.renderer.cam.OffsetX += 200 // the player pans away
	cam := a.renderer.cam
	clickBarSlot(t, a, f, barPawnID)
	if a.renderer.cam.OffsetX != cam.OffsetX {
		t.Fatalf("the third click centred again: %+v -> %+v", cam, a.renderer.cam)
	}
}

// A bar click selects the colonist and drops every other selection, like
// selecting on the map does.
func TestBarClickClearsOtherSelections(t *testing.T) {
	a, f := barColony(t)
	a.sel.SelAnimal, a.sel.HasAnimal = 90, true
	a.sel.SelItem, a.sel.HasItem = 91, true
	a.sel.SelPlant, a.sel.HasPlant = 92, true
	a.sel.SelFrame, a.sel.HasFrame = 93, true
	a.sel.SelBuilding, a.sel.HasBuilding = 94, true
	clickBarSlot(t, a, f, barPawnID)
	if a.sel.HasAnimal || a.sel.HasItem || a.sel.HasPlant || a.sel.HasFrame || a.sel.HasBuilding {
		t.Fatalf("selections left after a bar click: animal %v item %v plant %v frame %v building %v",
			a.sel.HasAnimal, a.sel.HasItem, a.sel.HasPlant, a.sel.HasFrame, a.sel.HasBuilding)
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
	sm.Vx, sm.Vy = 900, -700
	sm.Zooming, sm.TargetZoom, sm.AnchorX, sm.AnchorY = true, 2, 100, 100
	sm.PendingScale = 1.2
	zoom := a.renderer.cam.Zoom
	clickBarSlot(t, a, f, barPawnID)
	if sm.Vx != 0 || sm.Vy != 0 || sm.Zooming || sm.PendingScale != 0 {
		t.Fatalf("smoothing not stopped: %+v", *sm)
	}
	for i := 0; i < 20; i++ {
		f.Frame(t, a)
	}
	if !pawnCentred(a) || a.renderer.cam.Zoom > zoom*1.0001 {
		t.Fatalf("camera drifted after the jump: zoom %v -> %v, centred %v", zoom, a.renderer.cam.Zoom, pawnCentred(a))
	}
}

func TestCamSmoothHaltStopsMotion(t *testing.T) {
	t.Parallel()
	s := camerapkg.Smooth{Vx: 5, Vy: 6, DirX: 1, DirY: -1, Zooming: true, TargetZoom: 2, PendingScale: 1.3, Speed: 1.5}
	s.Halt()
	if s.Vx != 0 || s.Vy != 0 || s.DirX != 0 || s.DirY != 0 || s.Zooming || s.PendingScale != 0 {
		t.Fatalf("halt left motion: %+v", s)
	}
	if s.Speed != 1.5 {
		t.Fatalf("halt must keep the pan-speed setting, got %v", s.Speed)
	}
}
