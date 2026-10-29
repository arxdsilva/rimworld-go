package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
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

func TestBridgeKeys(t *testing.T) {
	for _, c := range []struct {
		mask int
		want string
	}{{0, "structure/bridge_00"}, {5, "structure/bridge_05"}, {15, "structure/bridge_15"}} {
		if got := overlay.BridgeKey(c.mask); got != c.want {
			t.Errorf("bridgeKey(%d) = %q, want %q", c.mask, got, c.want)
		}
		if got := worldpkg.BuildSprite("bridge", "", c.mask, false, false); got != c.want {
			t.Errorf("buildSprite(bridge, %d) = %q, want %q", c.mask, got, c.want)
		}
	}
}

func TestDrawFoundationPaintsDeckAndShadow(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.cam.Zoom = 32.0 / float64(camerapkg.TileSize)
	r.art = &art.Catalog{} // no bridge sprites landed: the plain deck stands in
	r.pipe().LOD.Tier = worldpkg.Live
	w := bridgeWorld(8, 8, terrain.DeepWater, [2]int{3, 3})
	r.pipe().LOD.Views = []game.ChunkView{{Buildings: []int32{0}}}
	var order []string
	r.view.Trace = &render.Trace{Layer: func(n string) { order = append(order, n) }}
	tile := r.cam.Tile()
	screen := ebiten.NewImage(320, 320)
	r.drawFoundation(screen, w, 320, 320, tile)
	if len(order) != 1 || order[0] != "foundation" {
		t.Errorf("layer trace = %v", order)
	}
	// not in the live tier: baked elsewhere, nothing traced
	order = nil
	r.pipe().LOD.Tier = worldpkg.Chunk
	r.drawFoundation(screen, w, 320, 320, tile)
	if len(order) != 1 {
		t.Errorf("baked tier trace = %v", order)
	}
	// the bake path draws a bridge cell too
	r.drawBridgeCell(screen, w, overlay.BridgeCells(w), &w.Buildings[0], tile, 320, 320)
}
