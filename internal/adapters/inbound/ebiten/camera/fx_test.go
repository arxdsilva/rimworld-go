package camera

import (
	"math"
	"testing"
)

func TestFxGlideSettlesAndShakeDecays(t *testing.T) {
	var f Fx
	f.Jump(0, 0, 400, -200)
	if dx, dy := f.Offset(); dx != -400 || dy != 200 {
		t.Fatalf("a jump starts where the camera was: %v %v", dx, dy)
	}
	for i := 0; i < 60; i++ {
		f.Step(1.0 / 60)
	}
	if f.Active() {
		t.Fatalf("a second later nothing is in flight: %+v", f)
	}
	f.Shake(10)
	first, _ := f.Offset()
	f.Step(0.01)
	if dx, _ := f.Offset(); first == dx && dx == 0 {
		t.Fatal("a shake moves the frame")
	}
	for i := 0; i < 120; i++ {
		f.Step(1.0 / 60)
	}
	if dx, dy := f.Offset(); f.Active() || dx != 0 || dy != 0 {
		t.Fatalf("a shake dies away: %+v", f)
	}
}

func TestFxBounds(t *testing.T) {
	var f Fx
	f.Jump(0, 0, 1e6, 0)
	if f.GlideX != -glideMax {
		t.Fatalf("glide capped: %v", f.GlideX)
	}
	f.Shake(1e6)
	if f.amp != shakeMax {
		t.Fatalf("shake capped: %v", f.amp)
	}
	f.Clear()
	f.Shake(-3)
	if f.Active() {
		t.Fatal("no negative shake")
	}
}

func TestFxIsDeterministic(t *testing.T) {
	run := func() (float64, float64) {
		var f Fx
		f.Shake(12)
		for i := 0; i < 7; i++ {
			f.Step(0.016)
		}
		return f.Offset()
	}
	ax, ay := run()
	bx, by := run()
	if ax != bx || ay != by || math.IsNaN(ax) {
		t.Fatal("same input, same shake")
	}
}

func TestZoomOptions(t *testing.T) {
	v := Viewport{0, 0, 200, 100}
	if x, y := ZoomAnchor(false, 5, 6, v); x != 100 || y != 50 {
		t.Fatalf("centre anchor %v %v", x, y)
	}
	if x, y := ZoomAnchor(true, 5, 6, v); x != 5 || y != 6 {
		t.Fatalf("mouse anchor %v %v", x, y)
	}
	if DragScale(0) != 1 || DragScale(150) != 1.5 {
		t.Fatal("drag scale")
	}
	if !WantsWorld(0.3, 0.3, false, true) || WantsWorld(0.5, 0.3, false, true) ||
		WantsWorld(0.3, 0.3, true, true) || WantsWorld(0.3, 0.3, false, false) {
		t.Fatal("the world opens only for an out notch at the floor with the ease done")
	}
}
