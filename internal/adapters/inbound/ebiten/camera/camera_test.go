package camera

import "testing"

func TestClampAxis(t *testing.T) {
	// map 1000px, viewport 0..500, slack 50
	if got := ClampAxis(-500, 1000, 0, 500, 50); got != -50 {
		t.Errorf("near edge: got %v, want -50", got)
	}
	if got := ClampAxis(5000, 1000, 0, 500, 50); got != 550 {
		t.Errorf("far edge: got %v, want 550", got)
	}
	if got := ClampAxis(200, 1000, 0, 500, 50); got != 200 {
		t.Errorf("inside: got %v, want 200", got)
	}
	// map 200px in a 500px viewport is centred: left edge at 150
	if got := ClampAxis(0, 200, 0, 500, 50); got != -150 {
		t.Errorf("small map: got %v, want -150", got)
	}
}
