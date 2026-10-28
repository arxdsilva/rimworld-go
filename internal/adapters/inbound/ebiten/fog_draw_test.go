package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// fogWorld is a w x h map whose tiles x >= hideFrom are unexplored.
func fogWorld(w, h, hideFrom int) game.WorldView {
	fog := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := hideFrom; x < w; x++ {
			fog[y*w+x] = 1
		}
	}
	return game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), Fog: fog, FogVer: 1}
}

func TestFogCacheRebuildsOnlyOnFogVerChange(t *testing.T) {
	t.Parallel()
	w := fogWorld(40, 40, 20)
	r := New(&stubSim{w: 40, h: 40, world: w})
	r.cam.Zoom, r.cam.Dev = 1, 1
	screen := ebiten.NewImage(256, 256)
	tile := r.cam.Tile()
	r.drawFogLayer(screen, w.Fog, 1, 40, 40, tile, 256, 256)
	first := r.fogc.Bakes
	if first == 0 {
		t.Fatal("first draw baked nothing")
	}
	r.drawFogLayer(screen, w.Fog, 1, 40, 40, tile, 256, 256)
	if r.fogc.Bakes != first {
		t.Fatalf("same FogVer rebaked: %d -> %d", first, r.fogc.Bakes)
	}
	r.drawFogLayer(screen, w.Fog, 2, 40, 40, tile, 256, 256)
	if r.fogc.Bakes != 2*first {
		t.Fatalf("FogVer change rebaked %d chunks, want %d", r.fogc.Bakes-first, first)
	}
	r.drawFogLayer(screen, nil, 0, 40, 40, tile, 256, 256)
	if r.fogc.Chunks != nil {
		t.Fatal("empty fog kept its bakes")
	}
}

func TestFogDrawsHiddenTilesNearBlack(t *testing.T) {
	t.Parallel()
	// The baked chunk (kept in fc.pix; offscreen images cannot be read
	// back before the game starts): hidden tiles are the fog colour, far
	// revealed tiles clear, the tile beside the fog a gradient.
	w := fogWorld(16, 16, 8)
	var fc fogCache
	var c fogChunk
	fc.Bake(&c, w.Fog, 16, 16, 0, 0)
	at := func(tx, ty int) [4]uint8 {
		o := 4 * ((ty*fogPx+fogPx/2)*fogChunkPx + tx*fogPx + fogPx/2)
		return [4]uint8{fc.Pix[o], fc.Pix[o+1], fc.Pix[o+2], fc.Pix[o+3]}
	}
	if got := at(12, 5); got != [4]uint8{0x0B, 0x0B, 0x0E, 255} {
		t.Fatalf("hidden tile pixel = %v, want fog colour", got)
	}
	if got := at(2, 5); got[3] != 0 {
		t.Fatalf("revealed tile far from the edge is fogged: %v", got)
	}
	if got := at(7, 5); got[3] == 0 || got[3] == 255 {
		t.Fatalf("tile beside the fog is not a gradient: %v", got)
	}
}

func TestFogNoFogDoesNothing(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, world: game.WorldView{TerrainW: 16}})
	screen := ebiten.NewImage(64, 64)
	r.drawFog(screen, game.WorldView{TerrainW: 16}, 64, 64, 32)
	if r.fogc.Bakes != 0 || r.fogc.Chunks != nil {
		t.Fatal("empty fog did work")
	}
}

func TestSelectionIgnoresHiddenTiles(t *testing.T) {
	t.Parallel()
	w := fogWorld(10, 10, 5)
	w.Items = []game.ItemView{{ID: 1, X: 7, Y: 2}, {ID: 2, X: 1, Y: 2}}
	pawns := []game.PawnView{{ID: 9, X: 8, Y: 8}}
	if got := selectablesAt(w, pawns, 7, 2); len(got) != 0 {
		t.Fatalf("hidden tile picked %v", got)
	}
	if got := selectablesAt(w, pawns, 8, 8); len(got) != 0 {
		t.Fatalf("hidden pawn picked %v", got)
	}
	if got := selectablesAt(w, pawns, 1, 2); len(got) != 1 {
		t.Fatalf("revealed item not picked: %v", got)
	}
}

func TestHoverSaysUnexplored(t *testing.T) {
	t.Parallel()
	w := fogWorld(10, 10, 5)
	if got := unexploredLines(w, 6, 3); len(got) != 1 || got[0] != "Unexplored" {
		t.Fatalf("hidden hover = %v", got)
	}
	if unexploredLines(w, 2, 3) != nil || unexploredLines(game.WorldView{}, 2, 3) != nil {
		t.Fatal("revealed or fogless tile reads Unexplored")
	}
}

func TestMinimapHidesFoggedTiles(t *testing.T) {
	t.Parallel()
	w := fogWorld(160, 160, 80)
	r := New(&stubSim{w: 160, h: 160, world: w})
	screen := ebiten.NewImage(1280, 800)
	v := colonyViewport(1280, 800)
	r.drawMinimap(screen, v, v)
	i := 10*160 + 100
	if got := [3]byte{r.mini.Pix[4*i], r.mini.Pix[4*i+1], r.mini.Pix[4*i+2]}; got != [3]byte{fogClr.R, fogClr.G, fogClr.B} {
		t.Fatalf("hidden minimap pixel = %v", got)
	}
	j := 10*160 + 10
	if r.mini.Pix[4*j] == fogClr.R && r.mini.Pix[4*j+1] == fogClr.G && r.mini.Pix[4*j+2] == fogClr.B {
		t.Fatal("revealed minimap pixel is black")
	}
}

func TestCameraStartsOnLanding(t *testing.T) {
	t.Parallel()
	v := colonyViewportPhysical(1280, 800, 1)
	w := game.WorldView{TerrainW: 200, Terrain: make([]uint8, 200*200), Landing: [2]int{40, 60}}
	r := New(&stubSim{w: 200, h: 200, world: w})
	r.cam.Zoom = 1
	r.ClampCamera(v)
	t0 := r.tilePx()
	cx := (r.cam.OffsetX + (v.X0+v.X1)/2) / t0
	cy := (r.cam.OffsetY + (v.Y0+v.Y1)/2) / t0
	if cx < 39 || cx > 42 || cy < 59 || cy > 62 {
		t.Fatalf("camera centred on (%.1f, %.1f), want the landing (40, 60)", cx, cy)
	}
}
