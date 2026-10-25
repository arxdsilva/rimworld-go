package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// bridgeWorld is a w x h world of one terrain type with bridge buildings.
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

func TestBridgeMaskOpenSidesOnly(t *testing.T) {
	// 3x3 deep water, a lone bridge in the middle: all four sides open.
	w := bridgeWorld(3, 3, terrain.DeepWater, [2]int{1, 1})
	set := bridgeCells(w)
	if got := bridgeMask(w, set, 1, 1); got != 15 {
		t.Errorf("lone bridge over water mask = %d, want 15", got)
	}
	// A bridge neighbour closes that side (N at (1,0), E at (2,1)).
	w = bridgeWorld(3, 3, terrain.DeepWater, [2]int{1, 1}, [2]int{1, 0}, [2]int{2, 1})
	set = bridgeCells(w)
	if got := bridgeMask(w, set, 1, 1); got != 4|8 { // S and W open
		t.Errorf("mask with N,E bridge neighbours = %d, want %d", got, 4|8)
	}
}

func TestBridgeMaskLandEndsAndMapEdgeGetNoEdge(t *testing.T) {
	w := bridgeWorld(3, 3, terrain.Soil, [2]int{1, 1})
	w.Terrain[1*3+0] = uint8(terrain.Marsh) // W of the bridge
	w.Terrain[1*3+2] = uint8(terrain.Mud)   // E
	w.Terrain[0*3+1] = uint8(terrain.ShallowWater)
	set := bridgeCells(w)
	// N shallow (1), E mud (2), S soil (0), W marsh (8)
	if got := bridgeMask(w, set, 1, 1); got != 1|2|8 {
		t.Errorf("mask = %d, want %d", got, 1|2|8)
	}
	// On the map edge the outside is not water: no edge.
	w = bridgeWorld(2, 1, terrain.DeepWater, [2]int{0, 0})
	if got := bridgeMask(w, bridgeCells(w), 0, 0); got != 2 { // only E
		t.Errorf("edge mask = %d, want 2", got)
	}
}

func TestBridgeKeys(t *testing.T) {
	for _, c := range []struct {
		mask int
		want string
	}{{0, "structure/bridge_00"}, {5, "structure/bridge_05"}, {15, "structure/bridge_15"}} {
		if got := bridgeKey(c.mask); got != c.want {
			t.Errorf("bridgeKey(%d) = %q, want %q", c.mask, got, c.want)
		}
		if got := buildSprite("bridge", "", c.mask, false, false); got != c.want {
			t.Errorf("buildSprite(bridge, %d) = %q, want %q", c.mask, got, c.want)
		}
	}
}

func TestBridgeCellsCachedOnStructVer(t *testing.T) {
	var c bridgeCache
	w := bridgeWorld(3, 3, terrain.DeepWater, [2]int{1, 1})
	a := c.sync(w)
	if !a[[2]int{1, 1}] || len(a) != 1 {
		t.Fatalf("cells = %v", a)
	}
	w.Buildings = append(w.Buildings, game.BuildingView{ID: 9, Kind: "bridge", X: 0, Y: 0})
	if b := c.sync(w); len(b) != 1 {
		t.Errorf("same StructVer must reuse the cache, got %d cells", len(b))
	}
	w.StructVer = 2
	if b := c.sync(w); len(b) != 2 {
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
		if got := bridgeLabelAt(w, 1, 1); got != c.want {
			t.Errorf("over %v: %q, want %q", c.t, got, c.want)
		}
	}
	w := bridgeWorld(2, 2, terrain.DeepWater, [2]int{1, 1})
	if got := bridgeLabelAt(w, 0, 0); got != "" {
		t.Errorf("no bridge: %q", got)
	}
}

func TestBridgeShadowSides(t *testing.T) {
	// Shadow lies on the open sides only; mask bits N,E,S,W = 1,2,4,8.
	if got := bridgeShadowSides(0); len(got) != 0 {
		t.Errorf("mask 0 shadows %v", got)
	}
	if got := bridgeShadowSides(15); len(got) != 4 {
		t.Errorf("mask 15 shadows %v", got)
	}
	if got := bridgeShadowSides(1 | 4); len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("mask 5 shadows %v", got)
	}
}

func TestDrawFoundationPaintsDeckAndShadow(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.cam.zoom = 32.0 / float64(TileSize)
	r.art = &artCatalog{} // no bridge sprites landed: the plain deck stands in
	r.lod.tier = lodLive
	w := bridgeWorld(8, 8, terrain.DeepWater, [2]int{3, 3})
	r.lod.views = []game.ChunkView{{Buildings: []int32{0}}}
	var order []string
	r.layerTrace = func(n string) { order = append(order, n) }
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	r.drawFoundation(screen, w, 320, 320, tile)
	if len(order) != 1 || order[0] != "foundation" {
		t.Errorf("layer trace = %v", order)
	}
	// not in the live tier: baked elsewhere, nothing traced
	order = nil
	r.lod.tier = lodChunk
	r.drawFoundation(screen, w, 320, 320, tile)
	if len(order) != 1 {
		t.Errorf("baked tier trace = %v", order)
	}
	// the bake path draws a bridge cell too
	r.drawBridgeCell(screen, w, bridgeCells(w), &w.Buildings[0], tile, 320, 320)
}

func TestBridgeShadowRects(t *testing.T) {
	t.Parallel()
	got := bridgeShadowRects(100, 50, 40, 1|8) // N and W open
	want := [][4]float32{{100, 46, 40, 4}, {96, 50, 4, 40}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("rects = %v, want %v", got, want)
	}
	all := bridgeShadowRects(0, 0, 40, 15)
	if len(all) != 4 || all[1] != ([4]float32{40, 0, 4, 40}) || all[2] != ([4]float32{0, 40, 40, 4}) {
		t.Errorf("all = %v", all)
	}
	if got := bridgeShadowRects(0, 0, 5, 1); got[0][3] != 1 {
		t.Errorf("thin strips are at least 1 px: %v", got)
	}
	if len(bridgeShadowRects(0, 0, 40, 0)) != 0 {
		t.Error("a closed bridge casts no shadow")
	}
}
