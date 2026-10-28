package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestHomeRebuildOutline(t *testing.T) {
	t.Parallel()
	var s HomeDrawState
	s.Rebuild(homeWorld(1))
	if len(s.Runs) != 2 || s.Runs[0] != (HomeRun{3, 2, 5}) || s.Runs[1] != (HomeRun{4, 2, 5}) {
		t.Fatalf("runs %v", s.Runs)
	}
	if len(s.Edges) != 10 { // 3 north + 3 south + 2 west + 2 east
		t.Fatalf("edges %d: %v", len(s.Edges), s.Edges)
	}
	// Home against the map edge still gets its outline
	w := game.WorldView{TerrainW: 2, Home: []uint8{1, 1}}
	s.Rebuild(w)
	if len(s.Runs) != 1 || len(s.Edges) != 6 {
		t.Fatalf("map-edge Home: %v %v", s.Runs, s.Edges)
	}
	s.Rebuild(game.WorldView{})
	if len(s.Runs) != 0 || len(s.Edges) != 0 {
		t.Fatal("no grid, no geometry")
	}
}

func homeWorld(ver uint32) game.WorldView {
	w := game.WorldView{TerrainW: 10, Home: make([]uint8, 100), HomeVer: ver}
	for y := 3; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			w.Home[y*10+x] = 1
		}
	}
	return w
}
