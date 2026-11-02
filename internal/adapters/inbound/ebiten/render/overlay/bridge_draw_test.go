package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

func TestBridgeMaskOpenSidesOnly(t *testing.T) {
	// 3x3 deep water, a lone bridge in the middle: all four sides open.
	w := bridgeWorld(3, 3, terrain.DeepWater, [2]int{1, 1})
	set := BridgeCells(w)
	if got := BridgeMask(w, set, 1, 1); got != 15 {
		t.Errorf("lone bridge over water mask = %d, want 15", got)
	}
	// A bridge neighbour closes that side (N at (1,0), E at (2,1)).
	w = bridgeWorld(3, 3, terrain.DeepWater, [2]int{1, 1}, [2]int{1, 0}, [2]int{2, 1})
	set = BridgeCells(w)
	if got := BridgeMask(w, set, 1, 1); got != 4|8 { // S and W open
		t.Errorf("mask with N,E bridge neighbours = %d, want %d", got, 4|8)
	}
}

func TestBridgeMaskLandEndsAndMapEdgeGetNoEdge(t *testing.T) {
	w := bridgeWorld(3, 3, terrain.Soil, [2]int{1, 1})
	w.Terrain[1*3+0] = uint8(terrain.Marsh) // W of the bridge
	w.Terrain[1*3+2] = uint8(terrain.Mud)   // E
	w.Terrain[0*3+1] = uint8(terrain.ShallowWater)
	set := BridgeCells(w)
	// N shallow (1), E mud (2), S soil (0), W marsh (8)
	if got := BridgeMask(w, set, 1, 1); got != 1|2|8 {
		t.Errorf("mask = %d, want %d", got, 1|2|8)
	}
	// On the map edge the outside is not water: no edge.
	w = bridgeWorld(2, 1, terrain.DeepWater, [2]int{0, 0})
	if got := BridgeMask(w, BridgeCells(w), 0, 0); got != 2 { // only E
		t.Errorf("edge mask = %d, want 2", got)
	}
}

func TestBridgeCellsCachedOnStructVer(t *testing.T) {
	var c BridgeCache
	w := bridgeWorld(3, 3, terrain.DeepWater, [2]int{1, 1})
	a := c.Sync(w)
	if !a[[2]int{1, 1}] || len(a) != 1 {
		t.Fatalf("cells = %v", a)
	}
	w.Buildings = append(w.Buildings, game.BuildingView{ID: 9, Kind: "bridge", X: 0, Y: 0})
	if b := c.Sync(w); len(b) != 1 {
		t.Errorf("same StructVer must reuse the cache, got %d cells", len(b))
	}
	w.StructVer = 2
	if b := c.Sync(w); len(b) != 2 {
		t.Errorf("new StructVer must rebuild, got %d cells", len(b))
	}
}

func TestBridgeLabelAt(t *testing.T) {
	cases := []struct {
		t    terrain.Type
		want string
	}{
		{terrain.DeepWater, "Bridge over deep water"},
		{terrain.ShallowWater, "Bridge over shallow water"},
		{terrain.Marsh, "Bridge over marsh"},
		{terrain.Mud, "Bridge over mud"},
		{terrain.Soil, "Bridge"},
	}
	for _, c := range cases {
		w := bridgeWorld(2, 2, c.t, [2]int{1, 1})
		if got := BridgeLabelAt(w, 1, 1); got != c.want {
			t.Errorf("over %v: %q, want %q", c.t, got, c.want)
		}
	}
	w := bridgeWorld(2, 2, terrain.DeepWater, [2]int{1, 1})
	if got := BridgeLabelAt(w, 0, 0); got != "" {
		t.Errorf("no bridge: %q", got)
	}
}

func bridgeWorld(w, h int, fill terrain.Type, bridges ...[2]int) game.WorldView {
	wv := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), StructVer: 1}
	for i := range wv.Terrain {
		wv.Terrain[i] = uint8(fill)
	}
	for i, b := range bridges {
		wv.Buildings = append(wv.Buildings, game.BuildingView{ID: entity.EntityID(i + 1), Kind: "bridge", X: b[0], Y: b[1], W: 1, H: 1, Stuff: "wood"})
	}
	return wv
}
