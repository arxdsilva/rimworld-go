package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
	if r.env.home.verts != 0 || r.env.home.builds != 0 {
		t.Fatalf("overlay off draws nothing: %d verts, %d builds", r.env.home.verts, r.env.home.builds)
	}
	r.homeOverlay = true
	if !r.homeOverlayOn() {
		t.Fatal("toggled on")
	}
	r.drawHomeOverlay(screen, w, 320, 320, tile)
	// one fill rect per row run (2) plus the 1 px outline: 10 edge strips
	if got := r.env.home.verts; got != 4*(2+10) {
		t.Fatalf("on: %d verts, want %d", got, 4*(2+10))
	}
	r.homeOverlay = false
	r.drawHomeOverlay(screen, w, 320, 320, tile)
	if r.env.home.verts != 0 {
		t.Fatalf("off again draws nothing: %d", r.env.home.verts)
	}
}

func TestHomeOverlayForced(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	r.homeOverlayForced = true // a Home tool is held
	if !r.homeOverlayOn() {
		t.Fatal("forced on")
	}
	r.drawHomeOverlay(ebiten.NewImage(320, 320), homeWorld(1), 320, 320, r.tilePx())
	if r.env.home.verts == 0 {
		t.Fatal("forced overlay draws")
	}
	// no Home data (or a map without a grid) draws nothing and never panics
	r.drawHomeOverlay(ebiten.NewImage(320, 320), game.WorldView{}, 320, 320, r.tilePx())
	if r.env.home.verts != 0 {
		t.Fatal("no Home, nothing to draw")
	}
}

func TestHomeOverlayCachedByVer(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	r.homeOverlay = true
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	w := homeWorld(5)
	for i := 0; i < 4; i++ {
		r.drawHomeOverlay(screen, w, 320, 320, tile)
	}
	if r.env.home.builds != 1 {
		t.Fatalf("same HomeVer rebuilt %d times", r.env.home.builds)
	}
	w2 := homeWorld(6)
	w2.Home[8*10+8] = 1 // Home grew
	r.drawHomeOverlay(screen, w2, 320, 320, tile)
	if r.env.home.builds != 2 {
		t.Fatalf("new HomeVer must rebuild: %d", r.env.home.builds)
	}
	if len(r.env.home.runs) != 3 {
		t.Fatalf("runs after growth: %v", r.env.home.runs)
	}
	// panning does not rebuild; it only changes what is culled
	r.cam.OffsetX += 500
	r.drawHomeOverlay(screen, w2, 320, 320, tile)
	if r.env.home.builds != 2 || r.env.home.verts != 0 {
		t.Fatalf("panned away: %d builds, %d verts", r.env.home.builds, r.env.home.verts)
	}
}

func TestHomeRebuildOutline(t *testing.T) {
	t.Parallel()
	var s homeDrawState
	s.rebuild(homeWorld(1))
	if len(s.runs) != 2 || s.runs[0] != (homeRun{3, 2, 5}) || s.runs[1] != (homeRun{4, 2, 5}) {
		t.Fatalf("runs %v", s.runs)
	}
	if len(s.edges) != 10 { // 3 north + 3 south + 2 west + 2 east
		t.Fatalf("edges %d: %v", len(s.edges), s.edges)
	}
	// Home against the map edge still gets its outline
	w := game.WorldView{TerrainW: 2, Home: []uint8{1, 1}}
	s.rebuild(w)
	if len(s.runs) != 1 || len(s.edges) != 6 {
		t.Fatalf("map-edge Home: %v %v", s.runs, s.edges)
	}
	s.rebuild(game.WorldView{})
	if len(s.runs) != 0 || len(s.edges) != 0 {
		t.Fatal("no grid, no geometry")
	}
}

func TestRoofOverlayDrawsHomeToo(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	r.homeOverlay = true
	r.drawRoofOverlay(ebiten.NewImage(320, 320), homeWorld(1), 320, 320, r.tilePx())
	if r.env.home.verts == 0 {
		t.Fatal("the Roof overlay pass draws the Home overlay")
	}
}
