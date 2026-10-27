package ebiten

import "testing"

// TestBoxSelectAtHalfRenderScale: a left drag around a pawn selects it at
// Half render scale as it does at Full (the drag box is read in map
// pixels through mapCursor). It sets the UI scale (LayoutF), so it is not
// a journey (journey_*_test.go run in parallel at scale 1).
func TestBoxSelectAtHalfRenderScale(t *testing.T) {
	for _, k := range []float64{1, 0.5} {
		a, f := renderScaleColony(t, 2, k)
		x0, y0 := a.renderer.worldToScreenF(20.2, 18.2) // map pixels around the pawn at (21, 19)
		x1, y1 := a.renderer.worldToScreenF(22.8, 20.8)
		raw := func(x, y float32) (int, int) { return int(float64(x) / k), int(float64(y) / k) }
		f.cur.mx, f.cur.my = raw(x0, y0)
		f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
		f.frame(t, a)
		f.cur.mx, f.cur.my = raw(x1, y1)
		f.frame(t, a)
		f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
		f.frame(t, a)
		if !a.renderer.HasSelect() || a.renderer.Selected() != 7 {
			t.Fatalf("world %v: the drag around the pawn selected %v %v, want pawn 7", k, a.renderer.HasSelect(), a.renderer.Selected())
		}
	}
}
