package world

import (
	"image"
	"image/color"
	"slices"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// The pipeline driven without the renderer: real layers over a catalog of
// pixel sprites (no files), lighting off, a fake simulation source over
// the read model's chunk index, and fake layer hosts. The renderer-level
// tests (the root's lod_test.go, chunk_cache_test.go and the draw tests)
// drive the same code through Renderer with the simulation stub. These
// tests stay serial: the frame calls gfx's process-wide canvas state.

// fakeSource is the simulation side of a rig.
type fakeSource struct {
	w      game.WorldView
	pawns  []game.PawnView
	mw, mh int
	idx    game.ChunkIndex
	routes int
}

func (s *fakeSource) World() game.WorldView     { return s.w }
func (s *fakeSource) Snapshot() []game.PawnView { return s.pawns }
func (s *fakeSource) Date() clock.Date          { return clock.Date{} }
func (s *fakeSource) ChunkLayer(cx, cy int) game.ChunkView {
	s.idx.Update(&s.w, s.mw, s.mh)
	return s.idx.Layer(cx, cy)
}
func (s *fakeSource) PlantsIn([]game.PlantView, int, int, int, int, []int32) ([]int32, bool) {
	return nil, false
}
func (s *fakeSource) DrawOrderRoute(*ebiten.Image, float64) { s.routes++ }

// fakeHosts is the renderer's side of the layers' hosts, drawing through
// the pipeline as the renderer's wrappers do.
type fakeHosts struct{ p *Pipeline }

func (h *fakeHosts) ChunkView(cx, cy int) game.ChunkView { return h.p.ChunkView(cx, cy) }
func (h *fakeHosts) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return h.p.PlantsOn(plants, x0, y0, x1, y1, buf)
}
func (h *fakeHosts) FrameLists(env.Lists) bool        { return false }
func (h *fakeHosts) Current([]game.BuildingView) bool { return false }
func (h *fakeHosts) DrawBakedColor(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int, col [4]float32) {
	h.p.Bk.DrawBakedColor(screen, src, bx, by, sx0, sy0, bpx, tx0, ty0, tx1, ty1, col)
}
func (h *fakeHosts) DrawChunkFlat(screen, src *ebiten.Image, srcX, srcY, dstX, dstY, span, srcSpan float64, col [4]float32) {
	h.p.Bk.DrawChunkFlat(screen, src, srcX, srcY, dstX, dstY, span, srcSpan, col)
}
func (h *fakeHosts) DrawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs sprite.ColorScale) {
	h.p.DrawTile(screen, key, wx, wy, tile, sw, sh, cs)
}
func (h *fakeHosts) DrawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps motion.Pose) {
	h.p.DrawSpritePose(screen, key, tint, wx, wy, tile, ps)
}
func (h *fakeHosts) Objects(game.WorldView, []game.PawnView) []light.Caster { return nil }
func (h *fakeHosts) Creatures(game.WorldView, []game.PawnView) []light.Creature[creature.FigurePose] {
	return nil
}
func (h *fakeHosts) CastFigure(*light.Creature[creature.FigurePose], *light.Cast, *ebiten.Image, float64) {
}
func (h *fakeHosts) VisibleTiles() (x0, y0, x1, y1 int) { return h.p.visibleTiles() }
func (h *fakeHosts) White() *ebiten.Image               { return h.p.LOD.WhiteImg() }

// figures is the creature layer's host: plain figures, no blood, no cues.
type figures struct{}

func (figures) FigureLayers(dst []creature.Layer, _ game.PawnView, fs string) []creature.Layer {
	return append(dst, creature.Layer{Key: "pawn/body_" + fs})
}
func (figures) PawnBlood(game.PawnView) float32                   { return 0 }
func (figures) AnimalBlood(game.AnimalView) float32               { return 0 }
func (figures) LayerBlood(_ string, b float32) float32            { return b }
func (figures) Visual(entity.EntityID) creature.Visual            { return creature.Visual{} }
func (figures) ColonyCloths() []string                            { return nil }
func (figures) ItemSprite(kind string) string                     { return ItemSprite(kind) }
func (figures) HealthColor(int) color.RGBA                        { return color.RGBA{A: 255} }
func (figures) TurretMuzzle(entity.EntityID) (creature.Vec, bool) { return creature.Vec{}, false }
func (figures) BurstCue(float64, float64)                         {}
func (figures) TraceSound(game.TraceView)                         {}
func (figures) WorkCue(string, float64, float64)                  {}
func (figures) HostileMarker(*ebiten.Image, game.PawnView, float64, float64, float64, float64) {
}
func (figures) StoneItem(key, _ string, cs sprite.ColorScale) (string, sprite.ColorScale) {
	return key, cs
}
func (figures) BuildingAt(game.WorldView, int, int) (game.BuildingView, bool) {
	return game.BuildingView{}, false
}
func (figures) Spray(game.WorldView, entity.EntityID, int) (creature.PartKind, int, float64) {
	return creature.PartBlood, 0, 0
}

// rig is a pipeline over test art on a mw×mh map.
type rig struct {
	p      *Pipeline
	v      *render.View
	cam    *camera.Camera
	src    *fakeSource
	layers []string
	screen *ebiten.Image
}

// tileSprite is a w×w opaque sprite of one colour at 128 px a tile.
func tileSprite(w int, c color.NRGBA) *art.Sprite {
	pix := image.NewNRGBA(image.Rect(0, 0, w, w))
	for i := 0; i < len(pix.Pix); i += 4 {
		pix.Pix[i], pix.Pix[i+1], pix.Pix[i+2], pix.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return &art.Sprite{Meta: art.Meta{Canvas: [2]int{128, 128}, Footprint: [2]int{1, 1}}, Pix: pix, Img: ebiten.NewImageFromImage(pix)}
}

func newRig(t *testing.T, w game.WorldView, mw, mh int, keys ...string) *rig {
	t.Helper()
	c := &art.Catalog{Sprites: map[string]*art.Sprite{}, Tinted: map[art.TintKey]*ebiten.Image{},
		Tints: map[string]map[string]color.NRGBA{"stone_tint": {"granite": {0x69, 0x5f, 0x61, 255}}, "cloth": {"blue": {40, 60, 200, 255}, RaiderCloth: {120, 20, 20, 255}}}}
	for _, k := range append([]string{"terrain/soil_00", "terrain/soil_01", "terrain/soil_02", "terrain/soil_03", "terrain/stone_00",
		"structure/floor_wood", "overlay/blueprint", "overlay/designation_mine"}, keys...) {
		c.Sprites[k] = tileSprite(32, color.NRGBA{100, 120, 80, 255})
	}
	lit := &light.Lit{Off: true}
	paint := sprite.NewPainter(c, lit)
	e := env.New(paint, lit) // New makes its grass and crop batches the zoom state's
	cr := creature.New(paint, &motion.State[creature.Body]{}, lit)
	cr.Host = figures{}
	ov := overlay.New(paint, lit)
	ov.Host = overlay.Host{
		Cue:      func(string, float64, float64, float64) {},
		Burst:    func(overlay.Particle, float64, float64, int, uint64) {},
		Flame:    func(*ebiten.Image, float32, float32, float64, float64, float64) {},
		Puff:     func(*ebiten.Image, int, float32, float32, float64, float64, sprite.ColorScale) bool { return false },
		PuffAt:   func(uint64) (int, float64, bool) { return 0, 0, false },
		BuildKey: BuildSprite,
		HitAge:   func(entity.EntityID, uint64, int, int) (float64, bool) { return 0, false },
		Flash:    func(uint64) {},
	}
	src := &fakeSource{w: w, mw: mw, mh: mh}
	cam := &camera.Camera{Zoom: 1}
	var cast *light.Cast
	var blood float32
	r := &rig{cam: cam, src: src, screen: ebiten.NewImage(640, 400)}
	r.v = &render.View{Cam: *cam, Art: c, MapW: mw, MapH: mh, Present: 1, Fx: 1, Tick: 1,
		Trace: &render.Trace{Layer: func(name string) { r.layers = append(r.layers, name) }}}
	h := &fakeHosts{}
	r.p = New(Layers{Lit: lit, Env: e, Creatures: cr, Overlays: ov, Cues: cue.New(), Shadows: &light.Shadows[creature.FigurePose]{}, Motion: &motion.State[creature.Body]{}},
		Hosts{Index: h, Stamp: h, Casters: h},
		Shared{Cam: cam, Cast: &cast, Blood: &blood},
		r.v, src)
	h.p = r.p
	r.p.SetArt(c)
	return r
}

// frame draws one frame at tile px a tile, the camera at the origin.
func (r *rig) frame(tile float64) {
	r.cam.Zoom, r.cam.OffsetX, r.cam.OffsetY = tile/camera.TileSize, 0, 0
	r.v.Cam = *r.cam
	r.v.SW, r.v.SH, r.v.Tile = 640, 400, tile
	r.layers = r.layers[:0]
	r.p.Draw(r.screen)
}

// rigWorld is a w×h soil world with a stone patch, a floor, a tree, a
// stack and a mine designation.
func rigWorld(w, h int) game.WorldView {
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), StructVer: 1}
	stone := uint8(0)
	for i, n := range TerrainNames {
		if n == "stone" {
			stone = uint8(i)
		}
	}
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			v.Terrain[y*w+x] = stone
		}
	}
	v.Buildings = []game.BuildingView{{ID: 1, Kind: "floor", Floor: true, X: 3, Y: 3, W: 1, H: 1, Stuff: "wood"}}
	v.Plants = []game.PlantView{{ID: 2, Species: "tree", X: 7, Y: 4, Growth: 100}}
	v.Items = []game.ItemView{{ID: 3, Kind: "wood", Qty: 30, X: 4, Y: 4}}
	v.Mines = [][2]int{{8, 8}}
	return v
}

// TestPipelineTiersAndBakes is the zoom tiers through the pipeline alone:
// live at 64 px (no bakes), chunk bakes at 32 px for the chunks in view,
// the whole-map bake at 8 px; an unchanged frame bakes nothing more, and
// the frame's chunk lists cover the view.
func TestPipelineTiersAndBakes(t *testing.T) {
	r := newRig(t, rigWorld(64, 64), 64, 64)
	l := r.p.LOD
	r.frame(64)
	if l.Tier != Live || l.Chunks.Bakes != 0 || l.Pages.Pages != nil {
		t.Fatalf("64 px: tier %d, %d chunk bakes, pages %v", l.Tier, l.Chunks.Bakes, l.Pages.Pages != nil)
	}
	if len(l.Views) == 0 || r.src.routes != 1 {
		t.Fatalf("64 px: %d chunk views, %d order routes", len(l.Views), r.src.routes)
	}
	r.frame(32)
	if l.Tier != Chunk || len(l.Chunks.ByChunk) != 2 { // 640×400 at 32 px: chunks (0,0) and (1,0)
		t.Fatalf("32 px: tier %d, %d chunks baked, want the 2 in view", l.Tier, len(l.Chunks.ByChunk))
	}
	bakes := l.Chunks.Bakes
	r.frame(32)
	if l.Chunks.Bakes != bakes {
		t.Fatalf("an unchanged frame rebaked %d chunks", l.Chunks.Bakes-bakes)
	}
	r.frame(8)
	baked := 0
	for _, b := range l.Pages.Baked {
		if b {
			baked++
		}
	}
	if l.Tier != Map || baked != 16 { // 640×400 at 8 px shows all 4×4 chunks of the 64² map
		t.Fatalf("8 px: tier %d, %d map chunks baked, want 16", l.Tier, baked)
	}
}

// TestPipelineGroundLayerOrder: floors, then zones, then (with the stack
// in view at 64 px) the object pass; the map tier skips the decals.
func TestPipelineGroundLayerOrder(t *testing.T) {
	w := rigWorld(32, 32)
	w.Dirt = []game.DirtView{{X: 1, Y: 1, Amount: 5}}
	r := newRig(t, w, 32, 32)
	r.frame(64)
	fl, dirt, zones := slices.Index(r.layers, "floors"), slices.Index(r.layers, "dirt"), slices.Index(r.layers, "zones")
	if fl < 0 || dirt < fl || zones < dirt {
		t.Fatalf("layers %v: want floors, dirt, zones in that order", r.layers)
	}
	r.frame(8)
	if slices.Contains(r.layers, "dirt") || !slices.Contains(r.layers, "floors") {
		t.Fatalf("map tier layers %v: floors, no decals", r.layers)
	}
}

// TestPipelineQueueReusesItsBuffer: the object queue keeps its backing
// array from frame to frame and holds no closure after the frame.
func TestPipelineQueueReusesItsBuffer(t *testing.T) {
	r := newRig(t, rigWorld(32, 32), 32, 32)
	r.frame(64)
	first := cap(r.p.draws)
	if first == 0 {
		t.Fatal("no object queued")
	}
	r.frame(64)
	if cap(r.p.draws) != first || len(r.p.draws) != 0 {
		t.Fatalf("queue cap %d -> %d, len %d", first, cap(r.p.draws), len(r.p.draws))
	}
	for _, d := range r.p.draws[:first] {
		if d.Fn != nil {
			t.Fatal("a closure outlived its frame")
		}
	}
}

// TestBakeRestoresTheCamera: WithBake moves the camera, the view and the
// cast to the bake's and puts them back.
func TestBakeRestoresTheCamera(t *testing.T) {
	r := newRig(t, rigWorld(16, 16), 16, 16)
	r.frame(64)
	before, cast := *r.cam, *r.p.Cast
	dst := ebiten.NewImage(64, 64)
	r.p.Bk.WithBake(dst, 4, 2, 3, 1, 1, func(sw, sh int, tile float64) {
		if tile != 4 || sw != 64 || sh != 64 || !r.p.LOD.Baking || *r.p.Cast == nil {
			t.Fatalf("bake: tile %v, %d×%d, baking %v", tile, sw, sh, r.p.LOD.Baking)
		}
		if r.cam.OffsetX != 2*4-1 || r.cam.OffsetY != 3*4-1 || r.v.Cam != *r.cam {
			t.Fatalf("bake camera %+v, view camera %+v", *r.cam, r.v.Cam)
		}
	})
	if *r.cam != before || r.v.Cam != before || r.p.LOD.Baking || *r.p.Cast != cast {
		t.Fatal("the bake left its camera, its cast or the baking flag behind")
	}
}

// TestPipelineObjectsAndLabels draws what the frame's object pass and
// labels reach: walls and a door, a fence without art (the vector
// fallback), a trough without art, a frame, a forbidden stack, a hauled
// mark, a damaged building, a vent and pawns chatting, fighting and in a
// mental break, at 64 px.
func TestPipelineObjectsAndLabels(t *testing.T) {
	w := rigWorld(32, 32)
	w.Buildings = append(w.Buildings,
		game.BuildingView{ID: 10, Kind: "wall", X: 10, Y: 2, W: 1, H: 1, Stuff: "wood", HPPct: 50},
		game.BuildingView{ID: 11, Kind: "door", X: 11, Y: 2, W: 1, H: 1, Stuff: "steel"},
		game.BuildingView{ID: 12, Kind: "fence", X: 12, Y: 2, W: 1, H: 1, Stuff: "wood"},
		game.BuildingView{ID: 13, Kind: TroughKind, X: 13, Y: 2, W: 1, H: 1, TroughFill: 2},
		game.BuildingView{ID: 14, Kind: "drying rack", X: 14, Y: 2, W: 1, H: 1, DryKind: "meat", DryQty: 3, DryPct: 40, Deconstruct: true},
		game.BuildingView{ID: 15, Kind: env.SteamVentKind, X: 2, Y: 8, W: 2, H: 2},
		game.BuildingView{ID: 16, Kind: "bed", X: 15, Y: 2, W: 1, H: 1},
	)
	w.Frames = []game.FrameView{{Kind: "wall", X: 10, Y: 3, W: 1, H: 1, Stuff: "granite blocks", Wood: 1}, {Kind: "floor", Floor: true, X: 9, Y: 3, Stuff: "wood"}}
	w.Items = append(w.Items, game.ItemView{ID: 20, Kind: "granite chunk", Qty: 1, X: 5, Y: 5, Forbidden: true, HaulMarked: true})
	r := newRig(t, w, 32, 32, "item/wood_pile", "effect/steam_puff_1", "effect/steam_puff_2", "overlay/forbidden")
	r.src.pawns = []game.PawnView{
		{ID: 1, X: 3, Y: 6, Job: "chat do", Hostile: true},
		{ID: 2, X: 4, Y: 6, SocialFight: true, Drafted: true, Interaction: "insult", InteractTick: 1},
		{ID: 3, X: 5, Y: 6, MentalBreak: "sad wander"},
	}
	r.v.HasSelect, r.v.Selected = true, 2
	for f := uint64(0); f < 3; f++ {
		r.v.Present, r.v.Tick = 100+f, clock.Tick(40*f)
		r.frame(64)
	}
	if r.p.LOD.Tier != Live || len(r.p.LOD.Creatures) != 3 {
		t.Fatalf("tier %d, %d creature cells", r.p.LOD.Tier, len(r.p.LOD.Creatures))
	}
	if key, cs := r.p.StoneItem("item/stone_chunk", "granite chunk", sprite.NoCS); key != "item/stone_blocks" || cs == sprite.NoCS {
		t.Fatalf("chunk without art: %s %v", key, cs)
	}
	if k, cs := r.p.TerrainTileKey("sand", 2); k != "terrain/gravel_02" || cs == sprite.NoCS {
		t.Fatalf("sand without art: %s %v", k, cs)
	}
	if got := r.p.ColonyCloths(); !slices.Equal(got, []string{"blue"}) {
		t.Fatalf("colony cloths %v: the raider tint is held back", got)
	}
	if r.p.BuildKey("door", "steel", "", 0, false, false) != "structure/door_steel_ns_closed" {
		t.Fatal("a steel door without wood art keeps its key")
	}
}
