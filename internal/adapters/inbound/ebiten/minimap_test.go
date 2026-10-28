package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func TestMinimapClickMovesCamera(t *testing.T) {
	t.Parallel()
	const sw, sh = 1280, 800
	v := colonyViewport(sw, sh)
	r := New(&stubSim{w: 200, h: 200, world: game.WorldView{TerrainW: 200, Terrain: make([]uint8, 200*200)}})
	r.cam.Zoom = 1
	r.ClampCamera(v)
	if !r.minimapOn() {
		t.Fatal("a 200² map should show the minimap")
	}
	bx, by, mx, my, mw, mh := r.minimapBox(v)
	// the box is the layout's Minimap: 180 square, 8 px from the right edge,
	// 8 px above Global (which sits 8 px above the tab strip)
	if want := computeHUDLayout(sw, sh, hudState{MinimapOn: true}).Minimap; bx != float64(want.X) || by != float64(want.Y) ||
		bx+minimapSide != v.X1-hudMargin || by+minimapSide != v.Y1-tabsH-hudMargin-hudMargin {
		t.Fatalf("box (%v,%v) is not the layout's Minimap %+v in %+v", bx, by, want, v)
	}
	// a click a quarter of the way in centres the camera on tile (50, 150)
	cx, cy := mx+mw*0.25, my+mh*0.75
	if !r.minimapClick(int(cx), int(cy), v, v) {
		t.Fatal("click on the minimap not taken")
	}
	tile := r.tilePx()
	centreX := (r.cam.OffsetX + (v.X0+v.X1)/2) / tile
	centreY := (r.cam.OffsetY + (v.Y0+v.Y1)/2) / tile
	if math.Abs(centreX-50) > 1.5 || math.Abs(centreY-150) > 1.5 {
		t.Fatalf("camera centred on (%.1f, %.1f), want (50, 150)", centreX, centreY)
	}
	if r.minimapClick(int(bx)-5, int(by)+10, v, v) {
		t.Fatal("a click beside the minimap was taken")
	}
	r.toggleMinimap()
	if r.minimapClick(int(cx), int(cy), v, v) {
		t.Fatal("a hidden minimap took a click")
	}
	small := New(&stubSim{w: 150, h: 150})
	if small.minimapOn() || small.minimapClick(int(cx), int(cy), v, v) {
		t.Fatal("a 150² map has no minimap")
	}
}

func TestMinimapPollsLayer(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 160, Terrain: make([]uint8, 160*160)}
	sim := &stubSim{w: 160, h: 160, world: w, pawns: []game.PawnView{{ID: 1, X: 10, Y: 10}, {ID: 2, X: 20, Y: 20, Hostile: true}}}
	r := New(sim)
	screen := ebiten.NewImage(1280, 800)
	v := colonyViewport(1280, 800)
	r.drawMinimap(screen, v, v)
	if r.mini.polls != 1 || r.mini.img == nil || r.mini.w != 160 {
		t.Fatalf("first draw: polls %d img %v", r.mini.polls, r.mini.img != nil)
	}
	r.present += minimapPoll
	r.drawMinimap(screen, v, v)
	if r.mini.polls != 1 {
		t.Fatal("an unchanged layer was rebuilt")
	}
	t2 := append([]uint8(nil), w.Terrain...)
	t2[0] = 4
	sim.world.Terrain, sim.world.StructVer = t2, 9
	r.drawMinimap(screen, v, v) // within the poll interval: not asked yet
	if r.mini.polls != 1 {
		t.Fatal("polled before the interval")
	}
	r.present += minimapPoll
	r.drawMinimap(screen, v, v)
	if r.mini.polls != 2 {
		t.Fatalf("a changed ground was not picked up: polls %d", r.mini.polls)
	}
	if c := minimapColor(game.MinimapStructure | 1); c != minimapStructureClr {
		t.Fatal("structures take the structure colour")
	}
}

// The minimap is the layout's Minimap rect: a 180 square just above
// Global, only on big maps, never chrome (it keeps its own click path),
// and a click on it recentres the camera.
func TestMinimapIsTheLayoutRect(t *testing.T) {
	big := &globalSim{stubSim: &stubSim{w: 200, h: 200}, world: game.WorldView{TerrainW: 200, Terrain: make([]uint8, 200*200)}}
	for _, sc := range globalScales {
		a, f := globalColony(t, big, sc.ui, sc.ww, sc.wh)
		l := a.hud()
		m := l.Minimap
		if m.W != minimapSize || m.H != minimapSize {
			t.Fatalf("ui %d: minimap %+v, want a %d square", sc.ui, m, minimapSize)
		}
		if m.Bottom() != l.Global.Y-hudMargin || m.Right() != float32(a.sw)-hudMargin {
			t.Errorf("ui %d: minimap %+v should sit just above Global %+v at the right edge", sc.ui, m, l.Global)
		}
		bx, by, _, _, _, _ := a.renderer.minimapBox(colonyViewport(a.sw, a.sh))
		if float32(bx) != m.X || float32(by) != m.Y {
			t.Errorf("ui %d: minimapBox (%v,%v) disagrees with the layout %+v", sc.ui, bx, by, m)
		}
		cx, cy := m.Center()
		if a.colonyChromeHit(int(cx), int(cy), a.sw, a.sh) {
			t.Errorf("ui %d: the minimap is chrome; it keeps its own click path", sc.ui)
		}
		before := a.renderer.cam.OffsetX
		a.renderer.cam.OffsetX = before + 1                               // so a recentre is visible
		f.onRect(a, rect{X: m.X + m.W*0.1, Y: m.Y + m.H*0.5, W: 1, H: 1}) // near the left of the map
		f.Click(t, a, mbLeft, 0)
		if a.renderer.cam.OffsetX == before+1 {
			t.Errorf("ui %d: clicking the minimap did not move the camera", sc.ui)
		}
	}
	small, _ := globalColonyP(t, newGlobalSim())
	if m := small.hud().Minimap; !m.Empty() {
		t.Errorf("a 16x16 map has a minimap: %+v", m)
	}
}

// The minimap's frame is the camera's view of the whole window, from the
// screen's top-left corner.
func TestMinimapFrameCoversTheWholeScreen(t *testing.T) {
	t.Parallel()
	v := colonyViewport(1280, 720)
	r := New(&stubSim{w: 200, h: 200})
	r.cam.Zoom, r.cam.Placed = 1, true
	r.cam.OffsetX, r.cam.OffsetY = 0, 0
	fx0, fy0, fx1, fy1 := r.minimapView(v, 100, 50, 180, 180)
	tile := r.tilePx()
	if fx0 != 100 || fy0 != 50 {
		t.Errorf("frame starts at (%v,%v), want the map's corner (100,50): the view begins at the screen's corner", fx0, fy0)
	}
	if wantW, wantH := 1280/tile/200*180, 720/tile/200*180; math.Abs(fx1-fx0-wantW) > 1e-6 || math.Abs(fy1-fy0-wantH) > 1e-6 {
		t.Errorf("frame %vx%v, want %vx%v: the whole window", fx1-fx0, fy1-fy0, wantW, wantH)
	}
}
