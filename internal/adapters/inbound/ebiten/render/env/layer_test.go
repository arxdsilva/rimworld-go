package env

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The layer driven on hand-built values: a catalog of pixel sprites (no
// files), lighting off, a view of 32 px tiles and a fake host for the
// renderer's chunk index, ground pipeline and creature motion. The
// renderer-level tests (the root's environment_test.go and friends) drive
// the same code through Renderer with the simulation stub.

// fakeHost records what the layer asks of the renderer.
type fakeHost struct {
	plants    []int32 // PlantsOn's answer
	movers    map[entity.EntityID]motion.MoveState
	views     map[[2]int]game.ChunkView
	lists     bool // FrameLists's answer
	baked     int  // DrawBakedColor calls
	flat      int  // DrawChunkFlat calls
	tiles     []string
	posed     []string
	chunkAsks int
}

func (h *fakeHost) ChunkView(cx, cy int) game.ChunkView {
	h.chunkAsks++
	return h.views[[2]int{cx, cy}]
}
func (h *fakeHost) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	for _, i := range h.plants {
		if p := plants[i]; p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
			buf = append(buf, i)
		}
	}
	return buf
}
func (h *fakeHost) FrameLists(Lists) bool            { return h.lists }
func (h *fakeHost) Current([]game.BuildingView) bool { return h.lists }
func (h *fakeHost) DrawBakedColor(_, _ *ebiten.Image, _, _ int, _, _, _ float64, _, _, _, _ int, _ [4]float32) {
	h.baked++
}
func (h *fakeHost) DrawChunkFlat(_, _ *ebiten.Image, _, _, _, _, _, _ float64, _ [4]float32) {
	h.flat++
}
func (h *fakeHost) DrawTile(_ *ebiten.Image, key string, _, _, _ float64, _, _ int, _ sprite.ColorScale) {
	h.tiles = append(h.tiles, key)
}
func (h *fakeHost) DrawSpritePose(_ *ebiten.Image, key string, _ color.NRGBA, _, _, _ float64, _ motion.Pose) {
	h.posed = append(h.posed, key)
}
func (h *fakeHost) InView(_, _, _, _ int) bool { return true }
func (h *fakeHost) PawnMotion(id entity.EntityID) motion.MoveState {
	return h.movers[id]
}
func (h *fakeHost) AnimalMotion(id entity.EntityID) motion.MoveState { return h.movers[id] }

// pixSprite is a w×h sprite of one opaque block, canvas c, origin o.
func pixSprite(w, h int, block image.Rectangle, c [2]int, o [2]float64) *art.Sprite {
	pix := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := block.Min.Y; y < block.Max.Y; y++ {
		for x := block.Min.X; x < block.Max.X; x++ {
			pix.SetNRGBA(x, y, color.NRGBA{120, 150, 80, 255})
		}
	}
	return &art.Sprite{Meta: art.Meta{Canvas: c, Origin: o, Footprint: [2]int{1, 1}}, Pix: pix, Img: ebiten.NewImageFromImage(pix)}
}

// layerRig is a layer over test art on a mw×mh map seen at 32 px a tile.
type layerRig struct {
	e    *Layer
	v    *render.View
	h    *fakeHost
	dst  *ebiten.Image
	tile float64
}

func newLayerRig(t *testing.T, w game.WorldView, mw, mh int, keys ...string) *layerRig {
	t.Helper()
	c := &art.Catalog{Sprites: map[string]*art.Sprite{}, Tinted: map[art.TintKey]*ebiten.Image{}}
	for _, k := range keys {
		c.Sprites[k] = pixSprite(32, 32, image.Rect(8, 8, 24, 24), [2]int{128, 128}, [2]float64{0, 0})
	}
	for v := 0; v < 6; v++ {
		c.Sprites[fmt.Sprintf("plant/grass_short_v%d", v)] = pixSprite(40, 46, image.Rect(14, 20, 26, 30), [2]int{160, 184}, [2]float64{16, 40})
	}
	for _, sp := range []string{"corn", "wheat"} {
		for _, name := range CropStageNames {
			for v := 0; v < CropVariants; v++ {
				c.Sprites[fmt.Sprintf("plant/%s_%s_v%d", sp, name, v)] = pixSprite(64, 96, image.Rect(22, 30, 42, 68), [2]int{128, 192}, [2]float64{0, 72})
			}
		}
	}
	for v := 0; v < 4; v++ {
		c.Sprites[fmt.Sprintf("overlay/tilled_%d", v)] = pixSprite(64, 64, image.Rect(0, 0, 64, 64), [2]int{128, 128}, [2]float64{0, 0})
	}
	lit := &light.Lit{Off: true}
	e := New(sprite.NewPainter(c, lit), lit)
	tile := 32.0
	v := &render.View{Cam: camera.Camera{Zoom: tile / camera.TileSize, Dev: 1}, Art: c, MapW: mw, MapH: mh, W: w, Present: 1}
	rig := &layerRig{e: e, v: v, h: &fakeHost{views: map[[2]int]game.ChunkView{}, movers: map[entity.EntityID]motion.MoveState{}}, dst: ebiten.NewImage(mw*32, mh*32), tile: tile}
	e.SetView(v, mw*32, mh*32, tile)
	return rig
}

func soilWorld(mw, mh int) game.WorldView {
	w := game.WorldView{TerrainW: mw, Terrain: make([]uint8, mw*mh), Grass: make([]uint8, mw*mh), StructVer: 1, GrassVer: 1, Season: "Summer"}
	for i := range w.Grass {
		w.Grass[i] = 1
	}
	return w
}

func TestLayerWindLeansPlantsWithinTheirLimits(t *testing.T) {
	t.Parallel()
	w := soilWorld(16, 12)
	w.Weather, w.WindX = "cloudy", 2
	rig := newLayerRig(t, w, 16, 12)
	e, v := rig.e, rig.v
	e.SetWeather(w)
	if vx, vy := e.WindAt(v, 3, 4); vx <= 0 || math.Abs(vy) > 2 {
		t.Fatalf("wind at (3, 4) = (%.2f, %.2f), want blowing east", vx, vy)
	}
	for f := uint64(1); f < 120; f++ {
		v.Present = f
		for _, k := range []PlantKind{KindGrass, KindCrop, KindBush, KindTree} {
			ps := e.PlantPose(v, 5, 5, k, 0.05, 1)
			if math.Abs(ps.Skew) > leanLimits[k]+trunkLimit+1e-9 {
				t.Fatalf("kind %d leans %.3f past its limit %.3f", k, ps.Skew, leanLimits[k])
			}
		}
	}
	// off screen: no lean computed
	if ps := e.PlantPose(v, 200, 200, KindGrass, 0.05, 1); ps.Skew != 0 {
		t.Fatal("an off-screen plant leans")
	}
	// snow hides grass tufts outdoors
	w.SnowDepth = 1
	e.SetWeather(w)
	if ps := e.PlantPose(v, 5, 5, KindGrass, 0.05, 1); ps.Cs[3] > 0.01 {
		t.Fatalf("grass under deep snow shows: alpha %.2f", ps.Cs[3])
	}
}

func TestLayerStepPushesPlantsAndDropsAutumnLeaves(t *testing.T) {
	t.Parallel()
	w := soilWorld(16, 12)
	w.Weather, w.Season = "clear", "Fall"
	w.Plants = []game.PlantView{{X: 6, Y: 5, Species: "tree"}}
	rig := newLayerRig(t, w, 16, 12)
	rig.h.plants = []int32{0}
	pawns := []game.PawnView{{ID: 7, X: 6, Y: 5}}
	rig.h.movers[7] = motion.MoveState{X: 6, Y: 5.2, VX: 0.05}
	for f := uint64(1); f < 4; f++ {
		rig.v.Present = f
		rig.e.Step(rig.v, w, pawns, 1, rig.h, rig.h)
	}
	if len(rig.e.Springs) == 0 {
		t.Fatal("a walking colonist pushed no plant")
	}
	if len(rig.e.Leaves) != 0 {
		t.Fatalf("leaves fell again on a held brush: %v", rig.e.Leaves)
	}
	// the first frame's brush listed the tree once
	e2 := newLayerRig(t, w, 16, 12)
	e2.h.plants, e2.h.movers = rig.h.plants, rig.h.movers
	e2.e.Step(e2.v, w, pawns, 1, e2.h, e2.h)
	if len(e2.e.Leaves) != 1 || e2.e.Leaves[0] != [2]int{6, 5} {
		t.Fatalf("brushed autumn tree drops leaves once: %v", e2.e.Leaves)
	}
	// paused (stepped 0): nothing pushes, the springs settle away
	for f := 0; f < 600; f++ {
		rig.v.Present++
		rig.e.Step(rig.v, w, pawns, 0, rig.h, rig.h)
	}
	if len(rig.e.Springs) != 0 {
		t.Fatalf("%d springs still live after the colonist stopped", len(rig.e.Springs))
	}
}

func TestLayerWeatherDropsDrawAndFlash(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"rain", "storm", "snow", "blizzard"} {
		w := soilWorld(16, 12)
		w.Weather, w.WeatherIntensity, w.WindX, w.Wetness = kind, 1, 1, 0.6
		rig := newLayerRig(t, w, 16, 12, "weather/snowflake_00", "weather/splash_00")
		for f := uint64(1); f < 90; f++ {
			rig.v.Present = f
			rig.e.Step(rig.v, w, nil, 1, rig.h, rig.h)
		}
		if len(rig.e.Drops) == 0 {
			t.Fatalf("%s: no drops", kind)
		}
		rig.e.DrawWeather(rig.v, rig.dst, w, 512, 384, rig.tile)
		rig.e.FlashSeen, rig.e.FlashAt = true, rig.v.Present
		rig.v.ReducedFlashing = kind == "storm"
		rig.e.DrawLightning(rig.v, rig.dst, 512, 384)
	}
	if veil, _ := WeatherVeil(game.WorldView{Weather: "storm", WeatherIntensity: 1}); veil.A == 0 {
		t.Fatal("a storm does not darken the sky")
	}
}

func TestLayerGrassBatchLiveBakedAndTinted(t *testing.T) {
	t.Parallel()
	w := soilWorld(16, 12)
	rig := newLayerRig(t, w, 16, 12)
	e, v := rig.e, rig.v
	e.DrawGrassBatch(v, rig.dst, &w, 0, 0, 15, 11, rig.tile, 512, 384, 9, false, rig.h)
	if e.Grass.Quads == 0 || e.Grass.Flushes != 1 {
		t.Fatalf("live batch: %d quads, %d flushes", e.Grass.Quads, e.Grass.Flushes)
	}
	live := e.Grass.Quads
	e.DrawGrassBatch(v, rig.dst, &w, 0, 0, 15, 11, rig.tile, 512, 384, GrassMidCap, true, rig.h)
	if e.Grass.Baked != 1 || e.Grass.Quads >= live {
		t.Fatalf("bake: %d bakes, %d quads (live %d)", e.Grass.Baked, e.Grass.Quads, live)
	}
	e.Grass.Level = "sparse"
	if sig := e.GrassChunkSig(v, &w, 0, 0, rig.h); sig == 0 {
		t.Fatal("a grassy chunk has no signature")
	}
	a := e.GrassMapSig(v, &w, 0, 0, 1, 1)
	if b := e.GrassMapSig(v, &w, 0, 0, 1, 1); a == 0 || a != b {
		t.Fatalf("map signature %x then %x", a, b)
	}
	white := ebiten.NewImage(3, 3)
	white.Fill(color.White)
	e.BakeGrassTint(v, rig.dst, &w, 0, 0, 16, 12, rig.tile, white)
	if e.Grass.Tinted != 1 {
		t.Fatal("no grass tint baked")
	}
	w.GrassCut = [][2]int{{3, 3}}
	e.GrassFrameMask(&w, 0, 0, 15, 11, rig.h)
	e.DrawGrassCutMarks(v, sprite.Pass{}, rig.dst, &w, 0, 0, 15, 11, rig.tile, 512, 384)
	// the frame's chunk lists cover tiles: a building hides the tufts under it
	w.Buildings = []game.BuildingView{{X: 2, Y: 2, W: 2, H: 2}}
	rig.h.lists, rig.h.views[[2]int{0, 0}] = true, game.ChunkView{Buildings: []int32{0}}
	e.GrassFrameMask(&w, 0, 0, 15, 11, rig.h)
	if e.Grass.mask[2*16+2]&GrassCovered == 0 || rig.h.chunkAsks == 0 {
		t.Fatal("the chunk lists' building does not cover its tiles")
	}
}

func TestLayerCropsQueueFlushAndFurrow(t *testing.T) {
	t.Parallel()
	w := soilWorld(16, 12)
	w.Zones = []game.ZoneView{{Kind: game.ZoneGrow, MinX: 4, MinY: 2, MaxX: 4, MaxY: 8, Cells: [][2]int{{4, 2}, {4, 3}}}}
	w.Plants = []game.PlantView{
		{X: 4, Y: 3, Species: "corn", Zone: "corn", Growth: 60, Stage: 2},
		{X: 5, Y: 3, Species: "wheat", Zone: "wheat", Growth: 100, Stage: 3, Damage: 80, DamageKind: "cold"},
	}
	rig := newLayerRig(t, w, 16, 12)
	e, v := rig.e, rig.v
	e.Crops.Begin()
	for i := range w.Plants {
		e.DrawCropObj(v, rig.dst, &w, &w.Plants[i], rig.tile)
	}
	if e.Crops.Quads == 0 {
		t.Fatal("no crop units queued")
	}
	e.FlushCrops(rig.dst)
	if e.Crops.Flushes == 0 || len(e.Crops.Idx) != 0 {
		t.Fatalf("flush: %d batches, %d indexes left", e.Crops.Flushes, len(e.Crops.Idx))
	}
	e.FlushCrops(rig.dst) // nothing pending: a no-op
	e.DrawFurrows(v, rig.dst, w, rig.tile, nil)
	if e.Crops.Furrows != 2 {
		t.Fatalf("furrows under %d crops", e.Crops.Furrows)
	}
	if !e.Crops.vertical(&w, 4, 2) || e.Crops.vertical(&w, 5, 3) {
		t.Fatal("the north-south grow zone's cells run vertical, others not")
	}
	// zoomed out: one unit a tile
	e.Crops.Begin()
	e.DrawCropObj(v, rig.dst, &w, &w.Plants[0], 20)
	if e.Crops.Quads != 1 {
		t.Fatalf("zoomed out: %d units", e.Crops.Quads)
	}
}

func TestLayerSnowAndWetVeilsBakeAndStamp(t *testing.T) {
	t.Parallel()
	w := soilWorld(40, 20)
	w.SnowDepth, w.Wetness, w.Weather = 0.8, 0.9, "snow"
	w.OutdoorTemp = -60
	rig := newLayerRig(t, w, 40, 20, "weather/snow_cover_04", "weather/snow_cover_00", "weather/frost_00")
	e, v := rig.e, rig.v
	e.DrawPuddleVeil(v, rig.dst, w, 1280, 640, rig.tile, rig.h, rig.h)
	if e.SnowC.Bakes == 0 || e.SnowC.WetDraws == 0 || rig.h.baked == 0 {
		t.Fatalf("wet veil: %d bakes, %d draws, %d stamps", e.SnowC.Bakes, e.SnowC.WetDraws, rig.h.baked)
	}
	bakes := e.SnowC.Bakes
	e.DrawGroundCover(v, rig.dst, w, 1280, 640, rig.tile, rig.h, rig.h)
	if e.SnowC.Draws == 0 || e.SnowC.Bakes != bakes {
		t.Fatalf("snow veil: %d draws, %d new bakes", e.SnowC.Draws, e.SnowC.Bakes-bakes)
	}
	// the decals draw at live zoom
	rig.v.Cam.Zoom = 48 / camera.TileSize
	e.DrawGroundCover(v, rig.dst, w, 1280, 640, 48, rig.h, rig.h)
	if len(rig.h.posed) == 0 {
		t.Fatal("no snow decals at live zoom")
	}
	// map zoom stamps one flat quad a chunk
	e.DrawGroundCover(v, rig.dst, w, 1280, 640, 4, rig.h, rig.h)
	if rig.h.flat == 0 {
		t.Fatal("no flat stamps at map zoom")
	}
	if a := e.SnowC.PuddleAt(-1, 0); a != 0 {
		t.Fatal("puddle cover off the map")
	}
	e.SnowC.evict()
	e.SnowC.release()
}

func TestLayerWaterTileAndEdges(t *testing.T) {
	t.Parallel()
	w := soilWorld(8, 8)
	rig := newLayerRig(t, w, 8, 8, "terrain/shore_foam_edge_01")
	e, v := rig.e, rig.v
	e.Water.Simple = true
	var draws []string
	e.Water.OnDraw = func(key string, shader bool) {
		if shader {
			t.Error("simple water went through the shader")
		}
		draws = append(draws, key)
	}
	e.DrawWaterTile(v, rig.dst, "shallow_water", 2, 2, rig.tile, 256, 256, sprite.ColorScale{}, false, rig.h)
	if len(draws) != 1 || len(rig.h.tiles) != 1 || rig.h.tiles[0] != draws[0] {
		t.Fatalf("simple water: draws %v, tiles %v", draws, rig.h.tiles)
	}
	if e.Water.WaterClock(v) != 0 {
		t.Fatal("simple water moves")
	}
	if cs := e.Water.FoamCS(v, 2, 2); cs[3] != 0.85 {
		t.Fatalf("still foam alpha %v", cs[3])
	}
	e.Water.Simple = false
	if e.Water.WaterFrameKey(v, "deep_water", 1, 1) == "" || e.Water.WaterClock(v) == 0 {
		t.Fatal("moving water has no frame or clock")
	}
	_ = e.Water.FoamCS(v, 2, 2)
	e.DrawWaterTile(v, rig.dst, "deep_water", 3, 3, rig.tile, 256, 256, sprite.ColorScale{}, true, rig.h)
	// deep water next to shallow on the north: the shallow edge, and foam where ground meets it
	e.DrawWaterEdges(v, sprite.Pass{}, rig.dst, "deep_water", [4]int{-1, 0, -2, -2}, -1, 3, 3, rig.tile, 256, 256, rig.h)
	if got := rig.h.tiles[len(rig.h.tiles)-1]; got != "terrain/shallow_water_edge_01" {
		t.Fatalf("shallow edge %q", got)
	}
}

func TestLayerTreeCanopiesFadeAndSweep(t *testing.T) {
	t.Parallel()
	fx := &TreeFx{}
	v := &render.View{Present: 1}
	creatures := map[[2]int]bool{{4, 4}: true}
	if a := fx.TreeCanopyAlpha(v, 9, 9, creatures); a != 1 {
		t.Fatalf("nobody near: alpha %v", a)
	}
	for f := uint64(2); f < 60; f++ {
		v.Present = f
		fx.TreeCanopyAlpha(v, 4, 4, creatures)
	}
	if a := fx.TreeCanopyAlpha(v, 4, 4, creatures); math.Abs(float64(a)-TreeSeeThrough) > 0.02 {
		t.Fatalf("creature behind the tree: alpha %v", a)
	}
	fx.SetTreeFocus([][2]int{{7, 7}})
	v.ReducedMotion = true
	if a := fx.TreeCanopyAlpha(v, 7, 7, nil); a != TreeSeeThrough {
		t.Fatalf("focused tree with Reduced Motion: alpha %v", a)
	}
	v.Present += 10
	fx.SweepTreeFades(v)
	if len(fx.Fades) != 0 {
		t.Fatalf("%d fades kept for trees out of view", len(fx.Fades))
	}
	c := &art.Catalog{Sprites: map[string]*art.Sprite{"plant/tree_oak_00_trunk": {}, "plant/tree_oak_00_canopy": {}}}
	if _, _, ok := TreePartKeys(&render.View{Art: c}, "plant/tree_oak_00"); !ok {
		t.Fatal("tree parts not found")
	}
}

func TestLayerSeasonKeysAndExtras(t *testing.T) {
	t.Parallel()
	w := soilWorld(8, 8)
	w.Season = "Fall"
	rig := newLayerRig(t, w, 8, 8, "plant/tree_oak_00_autumn", "plant/tree_birch_00", "structure/wall_ruin_overlay_00", "structure/wall_ruin_overlay_01", "structure/wall_ruin_overlay_02")
	e, v := rig.e, rig.v
	if got := e.SeasonKey(v, "plant/tree_oak_00", w); got != "plant/tree_oak_00_autumn" {
		t.Fatalf("autumn oak %q", got)
	}
	if got := e.SeasonKey(v, "plant/tree_birch_00", w); got != "plant/tree_birch_00" {
		t.Fatalf("birch without autumn art %q", got)
	}
	if !OnScreenTile(v, rig.dst, 2, 2, rig.tile) || OnScreenTile(v, rig.dst, 500, 2, rig.tile) {
		t.Fatal("OnScreenTile")
	}
	b := game.BuildingView{Kind: "wall", Ruin: true, X: 1, Y: 1}
	e.DrawRuinOverlay(v, sprite.Pass{}, rig.dst, &b, rig.tile, 256, 256)
	w.Plants = []game.PlantView{{X: 2, Y: 2, Species: "duskmoss", Growth: 100}}
	e.DrawDuskGlow(v, rig.dst, w, rig.tile, true, []game.ChunkView{{Plants: []int32{0}}})
	if x0, y0, x1, y1 := e.VisibleTiles(v); x0 != 0 || y0 != 0 || x1 != 7 || y1 != 7 {
		t.Fatalf("visible tiles %d,%d..%d,%d", x0, y0, x1, y1)
	}
	if !e.GroundOpen(w, 1, 1) {
		t.Fatal("open soil is not open ground")
	}
}
