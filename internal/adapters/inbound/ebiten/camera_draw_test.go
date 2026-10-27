package ebiten

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// colonyForCameraDraw is a live colony whose clock the test drives.
func colonyForCameraDraw(t *testing.T) (*App, *time.Time, *ebiten.Image) {
	t.Helper()
	a, _, _ := liveColony(t, 0)
	fakeOn(a).frame(t, a)
	clock := time.Unix(1000, 0)
	a.now = func() time.Time { return clock }
	img := ebiten.NewImage(a.sw, a.sh)
	a.Draw(img) // primes lastDraw
	return a, &clock, img
}

// TestCameraStepsPerDraw: one Update's held direction moves the camera on
// every Draw until the next Update; a released key clears it there and the
// pan coasts to a stop without reversing.
func TestCameraStepsPerDraw(t *testing.T) {
	a, clock, img := colonyForCameraDraw(t)
	r := a.renderer
	r.applyCamera(a.mapViewport(), camInput{dx: 1}) // one Update of a held key
	if r.smooth.dirX != 1 {
		t.Fatalf("dirX = %v after the push, want it held for the Draws", r.smooth.dirX)
	}
	last := r.cam.offsetX
	for i := 0; i < 2; i++ {
		*clock = clock.Add(8 * time.Millisecond)
		a.Draw(img)
		if r.cam.offsetX <= last {
			t.Fatalf("draw %d: offset %v did not advance past %v", i, r.cam.offsetX, last)
		}
		if r.smooth.dirX != 1 {
			t.Fatalf("draw %d: dirX = %v, want it held until the next Update", i, r.smooth.dirX)
		}
		last = r.cam.offsetX
	}
	r.applyCamera(a.mapViewport(), camInput{}) // the key is up at the next Update
	if r.smooth.dirX != 0 {
		t.Fatalf("dirX = %v after a release, want 0", r.smooth.dirX)
	}
	for i := 0; i < 200 && r.smooth.vx != 0; i++ { // coasts: forward only, then still
		*clock = clock.Add(8 * time.Millisecond)
		a.Draw(img)
		if r.cam.offsetX < last {
			t.Fatalf("coast reversed: %v < %v", r.cam.offsetX, last)
		}
		last = r.cam.offsetX
	}
	if r.smooth.vx != 0 || !r.smooth.idle() {
		t.Fatalf("the pan never stopped: vx %v", r.smooth.vx)
	}
}

func TestCameraStepClampsStall(t *testing.T) {
	a, clock, img := colonyForCameraDraw(t)
	r := a.renderer
	r.smooth.pushPan(1, 0)
	before := r.cam.offsetX
	*clock = clock.Add(2 * time.Second)
	a.Draw(img)
	moved := r.cam.offsetX - before
	if max := panMaxSpeed * camMaxDt * r.cam.dev; r.cam.dev > 0 && moved > max+0.01 || moved < 0 {
		t.Fatalf("a 2 s stall moved %v, want at most %v", moved, max)
	}
}

// TestStepCameraLeavesStillCameraAlone: a camera that code placed (a jump,
// a test scene) stays exactly there through Draws while nothing is in
// motion, even past the clamp limits: Update's applyCamera clamps, a
// Draw-time step never touches an idle smoother.
func TestStepCameraLeavesStillCameraAlone(t *testing.T) {
	a, clock, img := colonyForCameraDraw(t)
	r := a.renderer
	r.smooth.halt()
	if !r.smooth.idle() {
		t.Fatal("a halted smoother must be idle")
	}
	r.cam.offsetX, r.cam.offsetY = -1e5, 1e5 // far outside the map's limits
	zoom := r.cam.zoom
	for i := 0; i < 3; i++ {
		*clock = clock.Add(8 * time.Millisecond)
		a.Draw(img)
		if r.cam.offsetX != -1e5 || r.cam.offsetY != 1e5 || r.cam.zoom != zoom {
			t.Fatalf("draw %d moved a still camera to (%v, %v) zoom %v", i, r.cam.offsetX, r.cam.offsetY, r.cam.zoom)
		}
	}
	r.smooth.pushPan(1, 0)
	if r.smooth.idle() {
		t.Fatal("a pushed pan is not idle")
	}
}
