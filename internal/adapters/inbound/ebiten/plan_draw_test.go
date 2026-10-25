package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// planWorld is a 10x10 map whose plan marks are the 3x2 block at (2,3)..(4,4).
func planWorld(ver uint32) game.WorldView {
	w := game.WorldView{TerrainW: 10, Plan: make([]uint8, 100), PlanVer: ver}
	for y := 3; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			w.Plan[y*10+x] = 1
		}
	}
	return w
}

func TestPlanOverlayVisibleByDefaultAndHideable(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	r.drawPlanOverlay(screen, planWorld(1), 320, 320, tile)
	// 2 row-run fills + 10 outline sides drawn as 2 dashes each
	if got := r.planDraw.verts; got != 4*(2+20) {
		t.Fatalf("default: %d verts, want %d", got, 4*(2+20))
	}
	r.planHidden = true
	r.drawPlanOverlay(screen, planWorld(1), 320, 320, tile)
	if r.planDraw.verts != 0 {
		t.Fatalf("hidden draws nothing: %d", r.planDraw.verts)
	}
	r.planForced = true // a Plan tool is held
	r.drawPlanOverlay(screen, planWorld(1), 320, 320, tile)
	if r.planDraw.verts == 0 {
		t.Fatal("a held Plan tool forces the overlay over the toggle")
	}
}

func TestPlanOverlayOnlyForMarks(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	screen := ebiten.NewImage(320, 320)
	r.drawPlanOverlay(screen, game.WorldView{TerrainW: 10}, 320, 320, r.tilePx())
	r.drawPlanOverlay(screen, game.WorldView{}, 320, 320, r.tilePx())
	if r.planDraw.verts != 0 || r.planDraw.builds != 0 {
		t.Fatalf("no marks, nothing built or drawn: %d verts %d builds", r.planDraw.verts, r.planDraw.builds)
	}
}

func TestPlanOverlayCachedByVer(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	w := planWorld(5)
	for i := 0; i < 4; i++ {
		r.drawPlanOverlay(screen, w, 320, 320, tile)
	}
	if r.planDraw.builds != 1 {
		t.Fatalf("same PlanVer rebuilt %d times", r.planDraw.builds)
	}
	w2 := planWorld(6)
	w2.Plan[8*10+8] = 1
	r.drawPlanOverlay(screen, w2, 320, 320, tile)
	if r.planDraw.builds != 2 || len(r.planDraw.runs) != 3 {
		t.Fatalf("new PlanVer must rebuild: %d builds, runs %v", r.planDraw.builds, r.planDraw.runs)
	}
	r.cam.offsetX += 500 // panning only changes culling
	r.drawPlanOverlay(screen, w2, 320, 320, tile)
	if r.planDraw.builds != 2 || r.planDraw.verts != 0 {
		t.Fatalf("panned away: %d builds, %d verts", r.planDraw.builds, r.planDraw.verts)
	}
}

func TestPlanRebuildOutline(t *testing.T) {
	t.Parallel()
	var s planDrawState
	s.rebuild(planWorld(1))
	if len(s.runs) != 2 || s.runs[0] != (homeRun{3, 2, 5}) || len(s.edges) != 10 {
		t.Fatalf("runs %v edges %d", s.runs, len(s.edges))
	}
	s.rebuild(game.WorldView{})
	if len(s.runs) != 0 || len(s.edges) != 0 {
		t.Fatal("no grid, no geometry")
	}
}
