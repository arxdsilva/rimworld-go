package ebiten

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// homeWorld is a 10x10 map whose Home is the 3x2 block at (2,3)..(4,4).
func homeWorld(ver uint32) game.WorldView {
	w := game.WorldView{TerrainW: 10, Home: make([]uint8, 100), HomeVer: ver}
	for y := 3; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			w.Home[y*10+x] = 1
		}
	}
	return w
}

func TestHomeOverlayOnlyWhenOn(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	w := homeWorld(1)
	r.drawHomeOverlay(screen, w, 320, 320, tile)
	if r.overlays().HomeDraw.Verts != 0 || r.overlays().HomeDraw.Builds != 0 {
		t.Fatalf("overlay off draws nothing: %d verts, %d builds", r.overlays().HomeDraw.Verts, r.overlays().HomeDraw.Builds)
	}
	r.overlays().Home.On = true
	if !r.homeOverlayOn() {
		t.Fatal("toggled on")
	}
	r.drawHomeOverlay(screen, w, 320, 320, tile)
	// one fill rect per row run (2) plus the 1 px outline: 10 edge strips
	if got := r.overlays().HomeDraw.Verts; got != 4*(2+10) {
		t.Fatalf("on: %d verts, want %d", got, 4*(2+10))
	}
	r.overlays().Home.On = false
	r.drawHomeOverlay(screen, w, 320, 320, tile)
	if r.overlays().HomeDraw.Verts != 0 {
		t.Fatalf("off again draws nothing: %d", r.overlays().HomeDraw.Verts)
	}
}

func TestHomeOverlayForced(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	r.overlays().Home.Forced = true // a Home tool is held
	if !r.homeOverlayOn() {
		t.Fatal("forced on")
	}
	r.drawHomeOverlay(ebiten.NewImage(320, 320), homeWorld(1), 320, 320, r.tilePx())
	if r.overlays().HomeDraw.Verts == 0 {
		t.Fatal("forced overlay draws")
	}
	// no Home data (or a map without a grid) draws nothing and never panics
	r.drawHomeOverlay(ebiten.NewImage(320, 320), game.WorldView{}, 320, 320, r.tilePx())
	if r.overlays().HomeDraw.Verts != 0 {
		t.Fatal("no Home, nothing to draw")
	}
}

func TestHomeOverlayCachedByVer(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	r.overlays().Home.On = true
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	w := homeWorld(5)
	for i := 0; i < 4; i++ {
		r.drawHomeOverlay(screen, w, 320, 320, tile)
	}
	if r.overlays().HomeDraw.Builds != 1 {
		t.Fatalf("same HomeVer rebuilt %d times", r.overlays().HomeDraw.Builds)
	}
	w2 := homeWorld(6)
	w2.Home[8*10+8] = 1 // Home grew
	r.drawHomeOverlay(screen, w2, 320, 320, tile)
	if r.overlays().HomeDraw.Builds != 2 {
		t.Fatalf("new HomeVer must rebuild: %d", r.overlays().HomeDraw.Builds)
	}
	if len(r.overlays().HomeDraw.Runs) != 3 {
		t.Fatalf("runs after growth: %v", r.overlays().HomeDraw.Runs)
	}
	// panning does not rebuild; it only changes what is culled
	r.cam.OffsetX += 500
	r.drawHomeOverlay(screen, w2, 320, 320, tile)
	if r.overlays().HomeDraw.Builds != 2 || r.overlays().HomeDraw.Verts != 0 {
		t.Fatalf("panned away: %d builds, %d verts", r.overlays().HomeDraw.Builds, r.overlays().HomeDraw.Verts)
	}
}

func TestRoofOverlayDrawsHomeToo(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	r.overlays().Home.On = true
	r.drawRoofOverlay(ebiten.NewImage(320, 320), homeWorld(1), 320, 320, r.tilePx())
	if r.overlays().HomeDraw.Verts == 0 {
		t.Fatal("the Roof overlay pass draws the Home overlay")
	}
}
