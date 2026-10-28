package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestPlanRebuildOutline(t *testing.T) {
	t.Parallel()
	var s PlanDrawState
	s.Rebuild(planWorld(1))
	if len(s.Runs) != 2 || s.Runs[0] != (HomeRun{3, 2, 5}) || len(s.Edges) != 10 {
		t.Fatalf("runs %v edges %d", s.Runs, len(s.Edges))
	}
	s.Rebuild(game.WorldView{})
	if len(s.Runs) != 0 || len(s.Edges) != 0 {
		t.Fatal("no grid, no geometry")
	}
}

func planWorld(ver uint32) game.WorldView {
	w := game.WorldView{TerrainW: 10, Plan: make([]uint8, 100), PlanVer: ver}
	for y := 3; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			w.Plan[y*10+x] = 1
		}
	}
	return w
}
