package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// TestColonyHUDDrawsAtScale (PLAY-012 B.4): the colony HUD (every layout
// region, conditions, alerts, drag box) draws at s = 1.5 without
// panicking, every canvas rect stays inside the physical screen, and each
// HUD button's logical centre is inside the logical screen and hits it.
func TestColonyHUDDrawsAtScale(t *testing.T) {
	a, _, _ := liveColony(t, 2)
	withUIScale(t, 1.5)
	a.sw, a.sh = 1280, 720
	var rects [][4]float32
	canvasTrace = func(kind string, x, y, w, h float32) { rects = append(rects, [4]float32{x, y, w, h}) }
	t.Cleanup(func() { canvasTrace = nil })

	screen := ebiten.NewImage(1920, 1080)
	a.drawMapOverlays(screen)
	a.drawColonyHUD(screen, a.sw, a.sh)
	a.drawStatusChips(screen, a.sw)
	a.drawAlerts(screen, a.sw)
	a.drawBoxSelect(screen)
	if len(rects) == 0 {
		t.Fatal("HUD drew nothing through the canvas")
	}
	for _, r := range rects {
		if r[0] < 0 || r[1] < 0 || r[0]+r[2] > 1920.5 || r[1]+r[3] > 1080.5 {
			t.Errorf("canvas rect %v outside the 1920x1080 physical screen", r)
		}
	}
	hits := a.layoutHits(a.sw, a.sh)
	if len(hits) == 0 {
		t.Fatal("no HUD hit targets")
	}
	for _, b := range hits {
		cx, cy := b.x+b.w/2, b.y+b.h/2
		if cx > float32(a.sw) || cy > float32(a.sh) {
			t.Errorf("%s centre (%v,%v) outside the logical screen", b.id, cx, cy)
		}
		if !b.contains(int(cx), int(cy)) {
			t.Errorf("%s does not contain its own centre", b.id)
		}
	}
}
