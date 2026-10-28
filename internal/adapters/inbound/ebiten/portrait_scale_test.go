package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// PLAY-017: the colonist bar's portrait art sits inside its bar slot at
// every UI scale. It floated mid-screen at scale 2: the portrait drew
// logical coordinates straight onto the physical screen. The slots are the
// layout's (PLAY-018: computeHUDLayout's BarSlots, top-centre).
func TestBarPortraitsInTheirSlotsAtScale(t *testing.T) {
	art := testArt(t)
	for _, s := range []float64{1, 1.5, 2} {
		a, _, _ := liveColony(t, 2)
		a.renderer.art = art
		withUIScale(t, s)
		a.sw, a.sh = 1280, 720
		var imgs [][4]float32
		gfx.SetCanvasTrace(func(kind string, x, y, w, h float32) {
			if kind == "image" {
				imgs = append(imgs, [4]float32{x, y, w, h})
			}
		})
		screen := ebiten.NewImage(int(1280*s), int(720*s))
		a.drawColonistPortraits(screen, a.sh)
		gfx.SetCanvasTrace(nil)
		if len(imgs) == 0 {
			t.Fatalf("s=%v: no portrait image traced", s)
		}
		l := a.hud()
		n := len(a.barPawns())
		if len(l.BarSlots) != n {
			t.Fatalf("s=%v: %d bar slots for %d colonists", s, len(l.BarSlots), n)
		}
		have := map[int]bool{}
		for _, r := range imgs {
			x, y, w, h := float64(r[0]), float64(r[1]), float64(r[2]), float64(r[3])
			slot := -1
			for i, sl := range l.BarSlots {
				left, top, right, bottom := float64(sl.X)*s, float64(sl.Y)*s, float64(sl.Right())*s, float64(sl.Bottom())*s
				if x >= left-0.5 && x+w <= right+0.5 && y >= top-0.5 && y+h <= bottom+0.5 {
					slot = i
					break
				}
			}
			if slot < 0 {
				t.Errorf("s=%v: portrait image %v outside every bar slot (bar %+v)", s, r, l.Bar)
				continue
			}
			have[slot] = true
		}
		for i := 0; i < n; i++ {
			if !have[i] {
				t.Errorf("s=%v: bar slot %d draws no portrait", s, i)
			}
		}
	}
	t.Cleanup(func() { gfx.SetCanvasTrace(nil) })
}

// The colonist inspector's portrait sits inside the inspector panel.
func TestInspectorPortraitInPanelAtScale(t *testing.T) {
	art := testArt(t)
	a, _, id := liveColony(t, 0)
	a.renderer.art = art
	withUIScale(t, 2)
	a.sw, a.sh = 1280, 720
	a.renderer.Select(id)
	var imgs [][4]float32
	gfx.SetCanvasTrace(func(kind string, x, y, w, h float32) {
		if kind == "image" {
			imgs = append(imgs, [4]float32{x, y, w, h})
		}
	})
	t.Cleanup(func() { gfx.SetCanvasTrace(nil) })
	screen := ebiten.NewImage(2560, 1440)
	p, ok := a.selectedPawn()
	if !ok {
		t.Fatal("no selected colonist")
	}
	a.drawPawnPortrait(screen, p, 100, 400, 48, 64)
	if len(imgs) == 0 {
		t.Fatal("no portrait image traced")
	}
	for _, r := range imgs {
		if r[0] < 200-0.5 || r[1] < 800-0.5 || r[0]+r[2] > 296+0.5 || r[1]+r[3] > 928+0.5 {
			t.Errorf("portrait image %v outside its 48x64 box at (100,400) x2", r)
		}
	}
}
