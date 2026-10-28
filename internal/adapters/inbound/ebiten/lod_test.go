package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// lodWorld is a w×h soil world with a stone patch, a floor and a tree.
func lodWorld(w, h int) game.WorldView {
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), StructVer: 1}
	stone := uint8(0)
	for i, n := range terrainNames {
		if n == "stone" {
			stone = uint8(i)
		}
	}
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			v.Terrain[y*w+x] = stone
		}
	}
	v.Buildings = []game.BuildingView{{ID: 1, Kind: "floor", Floor: true, X: 20, Y: 20, W: 1, H: 1, Stuff: "wood"}}
	v.Plants = []game.PlantView{{ID: 2, Species: "tree", X: 30, Y: 30, Growth: 100}}
	return v
}

// lodRenderer draws a stub world at a tile size on a 640×400 screen.
func lodRenderer(w game.WorldView, mw, mh int) (*Renderer, *stubSim) {
	sim := &stubSim{w: mw, h: mh, world: w}
	r := New(sim)
	r.art = envArt("terrain/soil_00", "terrain/soil_01", "terrain/soil_02", "terrain/soil_03", "terrain/stone_00")
	r.speed = speedPaused
	return r, sim
}

func lodFrame(r *Renderer, tile float64) {
	r.cam.Zoom = tile / TileSize
	r.cam.OffsetX, r.cam.OffsetY = 0, 0
	screen := ebiten.NewImage(640, 400)
	r.setView(640, 400, r.tilePx())
	r.drawArt(screen, 640, 400, r.tilePx())
}

func TestLODSwitchesByZoom(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tile float64
		want lodTier
	}{{64, lodLive}, {48, lodLive}, {47.9, lodChunk}, {32, lodChunk}, {16, lodChunk}, {15.9, lodMap}, {6.4, lodMap}} {
		if got := lodFor(c.tile); got != c.want {
			t.Fatalf("lodFor(%v) = %d, want %d", c.tile, got, c.want)
		}
	}
	w := lodWorld(64, 64)
	r, _ := lodRenderer(w, 64, 64)
	lodFrame(r, 64)
	if r.lod.tier != lodLive || r.lod.chunks.bakes != 0 || r.lod.pages.pages != nil {
		t.Fatalf("64 px: tier %d, %d chunk bakes, pages %v", r.lod.tier, r.lod.chunks.bakes, r.lod.pages.pages != nil)
	}
	lodFrame(r, 32)
	if r.lod.tier != lodChunk || r.lod.chunks.bakes == 0 {
		t.Fatalf("32 px: tier %d, %d chunk bakes", r.lod.tier, r.lod.chunks.bakes)
	}
	// 640×400 at 32 px is 20×12.5 tiles: chunks (0,0) and (1,0)
	if n := len(r.lod.chunks.byChunk); n != 2 {
		t.Fatalf("32 px baked %d chunks, want the 2 in view", n)
	}
	lodFrame(r, 8)
	if r.lod.tier != lodMap || r.lod.pages.pages == nil || r.lod.pages.pages[0] == nil {
		t.Fatalf("8 px: tier %d, no map page", r.lod.tier)
	}
	baked := 0
	for _, b := range r.lod.pages.baked {
		if b {
			baked++
		}
	}
	if baked != 16 { // 640×400 at 8 px shows all 4×4 chunks of the 64² map
		t.Fatalf("map tier baked %d chunks, want 16", baked)
	}
}

func TestChunkBakeInvalidatedOnVersion(t *testing.T) {
	t.Parallel()
	w := lodWorld(64, 64)
	r, sim := lodRenderer(w, 64, 64)
	lodFrame(r, 24)
	first := r.lod.chunks.bakes
	if first == 0 {
		t.Fatal("no chunk baked")
	}
	lodFrame(r, 24)
	if r.lod.chunks.bakes != first {
		t.Fatalf("an unchanged frame rebaked %d chunks", r.lod.chunks.bakes-first)
	}
	// new ground in chunk (0,0) only (well inside it): one rebake
	t2 := append([]uint8(nil), w.Terrain...)
	t2[8*64+8] = t2[2*64+2]
	sim.world.Terrain, sim.world.StructVer = t2, 2
	lodFrame(r, 24)
	if got := r.lod.chunks.bakes - first; got != 1 {
		t.Fatalf("one changed chunk rebaked %d", got)
	}
	// the bake budget spreads a full rebake over frames
	r2, _ := lodRenderer(lodWorld(512, 512), 512, 512)
	lodFrame(r2, 16) // 40×25 tiles: 3×2 chunks in view, padded to more
	if r2.lod.chunks.bakes > bakeBudget {
		t.Fatalf("one frame baked %d chunks, budget %d", r2.lod.chunks.bakes, bakeBudget)
	}
}

// TestCreaturesKeepMinimumSize: below creatureMinTile a creature is drawn
// at that size, on its own tile's footprint (centre and bottom kept).
func TestCreaturesKeepMinimumSize(t *testing.T) {
	t.Parallel()
	if lodTile(6) != creatureMinTile || lodTile(40) != 40 {
		t.Fatal("lodTile")
	}
	wx, wy := creatureAt(10, 20, 8, 16) // drawn twice the tile size
	// footprint centre x and bottom stay at tile (10, 20)'s
	if cx := (wx + 0.5*2); cx != 10.5 {
		t.Fatalf("centre x %v, want 10.5", cx)
	}
	if bottom := wy + 2; bottom != 21 {
		t.Fatalf("bottom %v, want 21", bottom)
	}
	if x, y := creatureAt(3, 4, 32, 16); x != 3 || y != 4 {
		t.Fatal("no shift at or above the minimum")
	}
}

// TestLightGridBuildsOnlyReadChunks: lighting fills only the chunks read,
// and a new view of the same light keeps them.
func TestLightGridBuildsOnlyReadChunks(t *testing.T) {
	t.Parallel()
	const w, h = 64, 64
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), RoomGrid: make([]int, w*h), RoofCells: make([]uint8, w*h)}
	r := &Renderer{}
	r.view.W = v
	r.lightTile(1, 1)
	built := 0
	for _, b := range r.lit.grid.built {
		if b == r.lit.grid.gen {
			built++
		}
	}
	if built != 1 {
		t.Fatalf("%d chunks built for one read", built)
	}
	gen := r.lit.grid.gen
	v2 := v
	v2.Buildings = []game.BuildingView{{Kind: "wall", X: 5, Y: 5}} // a new list, no light change
	r.view.W = v2
	r.lightTile(1, 1)
	if r.lit.grid.gen != gen {
		t.Fatal("a new building list without fires or lit rooms rebuilt the light")
	}
	v3 := v2
	v3.Buildings = []game.BuildingView{{Kind: "campfire", X: 5, Y: 5, Burning: true, Glow: true, GlowRadius: 5}}
	r.view.W = v3
	if _, l := r.lightTile(5, 6); l <= 0 || r.lit.grid.gen == gen {
		t.Fatalf("a lit campfire: lamp %v, generation kept %v", l, r.lit.grid.gen == gen)
	}
}

// PLAY-022 A.3: the cell-based animal culls are padded by the largest
// animal canvas (artCatalog.animalExtent) plus the drift of a drawn
// animal from its cell, grown below the minimum creature size; a catalog
// without animals assumes animalSpanFallback.
func TestAnimalPadsFromLargestCanvas(t *testing.T) {
	t.Parallel()
	r := &Renderer{art: xlArt()} // canvas 6 x 3 tiles at 64 ppt: 2.5 west, 2.5 east, 2 up, 0 down
	if e := r.art.AnimalExtent(); e.Side != 2.5 || e.Up != 2 || e.Down != 0 {
		t.Fatalf("extent %+v", e)
	}
	if pad, padS := r.animalPads(64); pad != 5 || padS != 4 {
		t.Fatalf("pads %d, %d at 64 px tiles, want 5, 4", pad, padS)
	}
	if pad, padS := r.animalPads(8); pad != 8 || padS != 7 {
		t.Fatalf("pads %d, %d at 8 px tiles (drawn at 16), want 8, 7", pad, padS)
	}
	bare := &Renderer{art: envArt("terrain/soil_00")}
	if e := bare.art.AnimalExtent(); e.Side != animalSpanFallback || e.Up != animalSpanFallback {
		t.Fatalf("no animal art: %+v", e)
	}
}

// PLAY-022 A.3: an XL animal whose cell is off screen but whose body
// reaches into the view is drawn (the old 3-tile pad dropped it); one
// whose whole canvas is off screen is not.
func TestBigAnimalOffScreenStillDraws(t *testing.T) {
	t.Parallel()
	w := lodWorld(64, 64)
	w.Animals = []game.AnimalView{{ID: 1, Species: "elephant", X: 6, Y: 12}, {ID: 2, Species: "elephant", X: 4, Y: 12}}
	r, _ := lodRenderer(w, 64, 64)
	for k, sp := range xlArt().Sprites {
		sp.Meta.Origin, sp.Meta.Canvas = pt{288, 128}, [2]int{576, 192} // reaches 4.5 tiles east of its cell's west edge
		r.art.Sprites[k] = sp
	}
	drawn := map[*ebiten.Image]int{}
	r.lit.onDraw = func(img *ebiten.Image, _ float32) { drawn[img]++ }
	r.cam.Zoom, r.cam.OffsetX, r.cam.OffsetY = 1, 10*TileSize, 10*TileSize // the view starts at tile (10, 10)
	screen := ebiten.NewImage(640, 400)
	r.advance()
	r.setView(640, 400, r.tilePx())
	r.drawArt(screen, 640, 400, r.tilePx())
	img := r.art.Sprites["animal/elephant_south"].Img
	if img == nil || drawn[img] != 1 {
		t.Fatalf("the elephant reaching into the view drew %d times, want once (the other is off screen)", drawn[img])
	}
}
