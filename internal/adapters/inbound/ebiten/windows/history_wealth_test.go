package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game/wealth"
)

// TestWealthPointsFillThePlot (EAI-0 HUD): the first sample sits on the left
// edge, the last on the right, the largest value on the top and a value of
// zero on the bottom.
func TestWealthPointsFillThePlot(t *testing.T) {
	t.Parallel()
	plot := gfx.Rect{X: 10, Y: 20, W: 200, H: 100}
	pts := WealthPoints([]wealth.Sample{{Tick: 0, Value: 0}, {Tick: 60000, Value: 500}, {Tick: 120000, Value: 1000}}, plot)
	if len(pts) != 3 || pts[0] != [2]float32{10, 120} || pts[1] != [2]float32{110, 70} || pts[2] != [2]float32{210, 20} {
		t.Fatalf("points %v", pts)
	}
	if one := WealthPoints([]wealth.Sample{{Value: 40}}, plot); len(one) != 1 || one[0][0] != plot.Right() {
		t.Fatalf("one sample sits on the right edge: %v", one)
	}
	if WealthPoints(nil, plot) == nil || len(WealthPoints(nil, plot)) != 0 {
		t.Fatal("no samples, no points")
	}
}
