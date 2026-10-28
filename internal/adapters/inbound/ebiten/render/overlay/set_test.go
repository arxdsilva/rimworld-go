package overlay

import (
	"image/color"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// testSet is a Set without art over a fresh light layer, its hooks
// recording what the overlays ask of the renderer.
type testHost struct {
	cues    []string
	bursts  []Particle
	flames  int
	flashes []uint64
}

func testSet(h *testHost) *Set {
	lit := &light.Lit{}
	o := New(&sprite.Painter{Lit: lit, Stone: &sprite.StoneCache{}}, lit)
	o.Host = Host{
		Cue:      func(id string, x, y, vol float64) { h.cues = append(h.cues, id) },
		Burst:    func(p Particle, x, y float64, n int, seed uint64) { h.bursts = append(h.bursts, p) },
		Flame:    func(dst *ebiten.Image, sx, sy float32, t, tile, k float64) { h.flames++ },
		Puff:     func(*ebiten.Image, int, float32, float32, float64, float64, sprite.ColorScale) bool { return false },
		PuffAt:   func(t uint64) (int, float64, bool) { return int(t % 4), 1, true },
		BuildKey: func(kind, facing string, mask int, ew, open bool) string { return "power/" + kind },
		HitAge:   func(entity.EntityID, uint64, int, int) (float64, bool) { return 0, true },
		Flash:    func(tick uint64) { h.flashes = append(h.flashes, tick) },
	}
	return o
}

// testView is a 10x10 map seen whole at zoom 1 on a 640x640 screen.
func testView(w game.WorldView) *render.View {
	return &render.View{Cam: camera.Camera{Zoom: 1}, MapW: 10, MapH: 10, SW: 640, SH: 640, Tile: camera.TileSize, W: w}
}

func TestToggleShownAndToggle(t *testing.T) {
	t.Parallel()
	var tg Toggle
	if tg.Shown() {
		t.Fatal("zero toggle shown")
	}
	tg.Toggle()
	if !tg.Shown() || !tg.On {
		t.Fatal("toggled on not shown")
	}
	tg.Toggle()
	tg.Forced = true
	if !tg.Shown() || tg.On {
		t.Fatal("forced toggle not shown")
	}
	o := New(nil, nil)
	if !o.Plan.Shown() || o.Roof.Shown() || o.Home.Shown() || o.Power.Shown() || o.SnowClear.Shown() {
		t.Fatal("only the Plan overlay starts shown")
	}
	if o.Zones.base != o.base || o.Fires.base != o.base || o.Wiring.base != o.base {
		t.Fatal("the layers share one base")
	}
}

func TestChunksInView(t *testing.T) {
	t.Parallel()
	ch := Chunks{}
	if !ch.InView(500, 500, 0, 0) {
		t.Fatal("before a frame sets the view every tile counts")
	}
	ch = Chunks{ViewSet: true, VX0: 2, VY0: 2, VX1: 5, VY1: 5}
	for _, c := range []struct {
		x, y, pad, padS int
		want            bool
	}{{3, 3, 0, 0, true}, {1, 3, 0, 0, false}, {1, 3, 1, 0, true}, {3, 7, 1, 1, false}, {3, 7, 0, 2, true}} {
		if got := ch.InView(c.x, c.y, c.pad, c.padS); got != c.want {
			t.Errorf("InView(%+v) = %v", c, got)
		}
	}
}

func TestVisibleTilesClampToTheMap(t *testing.T) {
	t.Parallel()
	v := testView(game.WorldView{})
	if x0, y0, x1, y1 := visibleTiles(v, 640, 640, 64); x0 != 0 || y0 != 0 || x1 != 9 || y1 != 9 {
		t.Fatalf("whole map: %d,%d..%d,%d", x0, y0, x1, y1)
	}
	v.Cam.OffsetX, v.Cam.OffsetY = 128, 64
	if x0, y0, x1, y1 := visibleTiles(v, 128, 128, 64); x0 != 2 || y0 != 1 || x1 != 4 || y1 != 3 {
		t.Fatalf("panned: %d,%d..%d,%d", x0, y0, x1, y1)
	}
	if _, _, x1, _ := visibleTiles(v, 128, 128, 0); x1 >= 0 {
		t.Fatal("no tile size: empty range")
	}
}

// homeGrid is a 10x10 grid with a 3x2 block set at (2, 3).
func homeGrid() []uint8 {
	g := make([]uint8, 100)
	for y := 3; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			g[y*10+x] = 1
		}
	}
	return g
}

func TestAreaOverlaysDrawOnlyWhenShown(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	screen := ebiten.NewImage(640, 640)
	w := game.WorldView{TerrainW: 10, Home: homeGrid(), HomeVer: 1, Plan: homeGrid(), PlanVer: 1, SnowClear: homeGrid(), SnowClearVer: 1}
	v := testView(w)
	o.Plan.On = false
	o.DrawHomeOverlay(v, screen, w, 640, 640, 64)
	o.DrawPlanOverlay(v, screen, w, 640, 640, 64)
	o.DrawSnowClearOverlay(v, screen, w, 640, 640, 64)
	if o.HomeDraw.Verts != 0 || o.PlanDraw.Verts != 0 || o.SnowDraw.Verts != 0 {
		t.Fatalf("hidden overlays drew %d/%d/%d vertices", o.HomeDraw.Verts, o.PlanDraw.Verts, o.SnowDraw.Verts)
	}
	o.Home.On, o.Plan.Forced, o.SnowClear.Forced = true, true, true
	o.DrawHomeOverlay(v, screen, w, 640, 640, 64)
	o.DrawPlanOverlay(v, screen, w, 640, 640, 64)
	o.DrawSnowClearOverlay(v, screen, w, 640, 640, 64)
	// two row runs and ten outline edges, four vertices each; the plan's
	// edges carry two dashes
	if o.HomeDraw.Verts != 4*(2+10) || o.SnowDraw.Verts != 4*(2+10) || o.PlanDraw.Verts != 4*(2+20) {
		t.Fatalf("shown overlays drew %d/%d/%d vertices", o.HomeDraw.Verts, o.PlanDraw.Verts, o.SnowDraw.Verts)
	}
	o.DrawHomeOverlay(v, screen, w, 640, 640, 64)
	if o.HomeDraw.Builds != 1 {
		t.Fatalf("an unchanged HomeVer rebuilt: %d builds", o.HomeDraw.Builds)
	}
}

func TestRoofOverlayAndEaves(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	screen := ebiten.NewImage(640, 640)
	w := game.WorldView{TerrainW: 10, RoofCells: make([]uint8, 100), RoomGrid: make([]int, 100)}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	w.RoofCells[5*10+5] = RoofBuilt
	w.RoomGrid[5*10+6] = -2 // a wall east of the roofed tile
	v := testView(w)
	areas := func() []game.AreaView { t.Fatal("no area is armed: none asked for"); return nil }
	o.DrawRoofOverlay(v, screen, w, areas, 640, 640, 64)
	if o.RoofDraw.OverlayVerts != 0 {
		t.Fatal("hidden roof overlay drew")
	}
	o.Roof.On = true
	o.DrawRoofOverlay(v, screen, w, areas, 640, 640, 64)
	if o.RoofDraw.OverlayVerts == 0 {
		t.Fatal("shown roof overlay drew nothing")
	}
	o.DrawEaves(v, screen, w, 640, 640, 64)
	if len(o.RoofDraw.Edges) != 1 || o.RoofDraw.Edges[0] != (EaveEdge{X: 5, Y: 5, Side: SideE}) {
		t.Fatalf("eaves %v", o.RoofDraw.Edges)
	}
	o.DrawRoofUnsupported(v, screen, 1, 1, 64)
}

func TestAllowedAreaOverlay(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	screen := ebiten.NewImage(640, 640)
	w := game.WorldView{TerrainW: 10}
	v := testView(w)
	bits := make([]uint64, 2) // 100 cells
	bits[0] = 1<<12 | 1<<13
	areas := []game.AreaView{{ID: 3, Ver: 1, Bits: bits, Color: [4]uint8{10, 20, 30, 255}}}
	asked := 0
	o.DrawAreaOverlay(v, screen, w, func() []game.AreaView { asked++; return areas }, 640, 640, 64)
	if asked != 0 || o.Area.Verts != 0 {
		t.Fatal("nothing armed: no areas asked for, nothing drawn")
	}
	o.Area.Armed = 3
	o.DrawAreaOverlay(v, screen, w, func() []game.AreaView { asked++; return areas }, 640, 640, 64)
	if asked != 1 || o.Area.Verts != 4 || o.Area.Tint != (color.RGBA{10, 20, 30, 255}) {
		t.Fatalf("armed area: asked %d, %d vertices, tint %v", asked, o.Area.Verts, o.Area.Tint)
	}
	o.DrawAreas(v, screen, w, areas, 640, 640, 64)
	if o.Area.Builds != 1 {
		t.Fatalf("an unchanged area rebaked: %d", o.Area.Builds)
	}
}

func TestFogLayerBakesOnlyOnNewVersion(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	screen := ebiten.NewImage(640, 640)
	fog := make([]uint8, 100)
	fog[0] = 1
	w := game.WorldView{TerrainW: 10, Fog: fog, FogVer: 1}
	v := testView(w)
	o.DrawFog(v, screen, w, 640, 640, 64)
	o.DrawFog(v, screen, w, 640, 640, 64)
	if o.Fog.Bakes != 1 {
		t.Fatalf("same FogVer baked %d times", o.Fog.Bakes)
	}
	w.FogVer = 2
	o.DrawFog(v, screen, w, 640, 640, 64)
	if o.Fog.Bakes != 2 {
		t.Fatalf("new FogVer: %d bakes", o.Fog.Bakes)
	}
	o.DrawFog(v, screen, game.WorldView{TerrainW: 10}, 640, 640, 64)
	if o.Fog.Chunks != nil {
		t.Fatal("a map without fog drops the bakes")
	}
}

func TestZonesBatchEveryOutlineInOneDraw(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	screen := ebiten.NewImage(640, 640)
	z := game.ZoneView{ID: 1, Kind: game.ZoneStockpile, Cells: [][2]int{{1, 1}, {2, 1}, {1, 2}, {2, 2}}}
	g := game.ZoneView{ID: 2, Kind: game.ZoneGrow, Crop: "potato", Cells: [][2]int{{5, 5}, {6, 5}}}
	p := game.ZoneView{ID: 3, Kind: game.ZonePen, Cells: [][2]int{{8, 8}}}
	w := game.WorldView{Zones: []game.ZoneView{z, g, p}, Plants: []game.PlantView{{X: 5, Y: 5, Species: "potato", Zone: "potato"}}}
	v := testView(w)
	ch := &Chunks{}
	o.Zones.DrawZones(v, sprite.Pass{}, ch, screen, w, 640, 640, 64)
	if len(o.Zones.Batch.Idx) != 0 {
		t.Fatal("the outline batch was not flushed")
	}
	if !o.Zones.plants[[2]int{5, 5}] || len(o.Zones.plants) != 1 {
		t.Fatalf("the grow zone's own plant: %v", o.Zones.plants)
	}
	if o.Zones.ZoneGeomOf(z) != o.Zones.ZoneGeomOf(z) {
		t.Fatal("an unchanged zone rebuilt its outline")
	}
	o.Zones.DrawZoneLabels(v, screen, w, 640, 640, 64)
	o.Zones.DrawZoneHighlight(v, ch, screen, w, g, 640, 640, 64)
	if _, ok := o.Zones.ZoneLabelAt(v, w, 0, 0, 64, 0); ok {
		t.Fatal("no plate at the screen's corner")
	}
}

func TestFiresAskTheRendererForFlameAndParticles(t *testing.T) {
	t.Parallel()
	h := &testHost{}
	o := testSet(h)
	screen := ebiten.NewImage(640, 640)
	w := game.WorldView{
		Buildings: []game.BuildingView{{ID: 1, Kind: "campfire", X: 2, Y: 2, Burning: true}, {ID: 2, Kind: TorchLampKind, X: 3, Y: 3, Burning: true}},
		Fires:     []game.FireView{{X: 4, Y: 4, Size: 80}},
		Stains:    []game.StainView{{X: 1, Y: 1, Kind: ashKind, Amount: 1}},
		Strikes:   []game.StrikeView{{X: 6, Y: 6, Tick: 95}, {X: 7, Y: 7, Tick: 10}},
	}
	v := testView(w)
	v.Tick = 100
	o.Fires.DrawFires(v, screen, w, 64)
	if h.flames != 1 {
		t.Fatalf("one campfire flame (the torch burns its own): %d", h.flames)
	}
	o.Fires.DrawWorldFires(v, screen, w, 64) // no art: the procedural flame
	if h.flames != 2 {
		t.Fatalf("the burning cell's fallback flame: %d", h.flames)
	}
	for v.Present = 0; v.Present < 40; v.Present++ {
		o.Fires.EmitFireParticles(v, w)
		o.Fires.EmitBlazeParticles(v, w)
	}
	var embers, smoke int
	for _, p := range h.bursts {
		if p == Ember {
			embers++
		} else {
			smoke++
		}
	}
	if embers == 0 || smoke == 0 {
		t.Fatalf("embers %d, smoke %d", embers, smoke)
	}
	o.Fires.DrawAshStains(v, sprite.Pass{}, screen, w, 640, 640, 64)
	o.Fires.ShowStrikes(v, w)
	o.Fires.ShowStrikes(v, w)
	if len(h.flashes) != 1 || h.flashes[0] != 95 || len(h.cues) != 1 || h.cues[0] != "sfx.lightning_strike" {
		t.Fatalf("one fresh strike shown once: flashes %v, cues %v", h.flashes, h.cues)
	}
	var loops []string
	o.Fires.BlazeAmbience(w, func(x, y float64) (float64, float64, bool) { return 1, 0, true },
		func(key string, vol, pan float64) { loops = append(loops, key) })
	if len(loops) != 1 || loops[0] != "blaze:4:4" {
		t.Fatalf("one loop per cluster: %v", loops)
	}
}

func TestCasketsCueTheirOpening(t *testing.T) {
	t.Parallel()
	h := &testHost{}
	o := testSet(h)
	sealed := game.WorldView{Buildings: []game.BuildingView{{ID: 1, Kind: CasketKind, Sealed: true}}}
	open := game.WorldView{Buildings: []game.BuildingView{{ID: 1, Kind: CasketKind}}}
	o.ListenCaskets(sealed)
	o.ListenCaskets(open)
	o.ListenCaskets(open)
	if len(h.cues) != 1 || h.cues[0] != casketCueSFX {
		t.Fatalf("cues %v", h.cues)
	}
}

func TestBloodSkipsTheShadowPassAndCarriedPatients(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	v := testView(game.WorldView{})
	v.AnimT = 10
	p := game.PawnView{ID: 1, Health: 40, HitTick: 9}
	if o.PawnBlood(v, sprite.Pass{}, p) == 0 {
		t.Fatal("a hurt pawn bleeds")
	}
	if o.PawnBlood(v, sprite.Pass{Cast: &light.Cast{}}, p) != 0 {
		t.Fatal("no blood in the shadow pass")
	}
	p.CarriedBy = 2
	if o.PawnBlood(v, sprite.Pass{}, p) != 0 {
		t.Fatal("a carried patient shows no blood")
	}
	an := game.AnimalView{ID: 3, Health: 50, Species: "hive_beetle"}
	if _, _, ichor := DecodeBlood(o.AnimalBlood(v, sprite.Pass{}, an)); !ichor {
		t.Fatal("a hive insect bleeds ichor")
	}
}

func TestPowerOverlayTracesGridAndDevices(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	screen := ebiten.NewImage(640, 640)
	w := &game.WorldView{
		Buildings: []game.BuildingView{
			{ID: 1, Kind: ConduitKind, X: 1, Y: 1, PowerRole: "transmitter", PowerNet: 7},
			{ID: 2, Kind: "lamp", X: 2, Y: 1, PowerRole: "consumer", PowerNet: 7, HasLink: true, LinkFromX: 2, LinkFromY: 1, LinkX: 1, LinkY: 1, NoPowerReason: "no power"},
			{ID: 3, Kind: BatteryKind, X: 5, Y: 5, PowerRole: "storage", CapWh: 100, StoredWh: 50},
		},
		PowerNets: []game.PowerNetView{{ID: 7}},
	}
	w.Buildings[0].W, w.Buildings[0].H = 1, 1
	v := testView(*w)
	ch := &Chunks{Views: []game.ChunkView{{Buildings: []int32{0, 1, 2}}}}
	var parts []string
	o.Wiring.Nets.Trace = func(part string, x, y int, c color.NRGBA) { parts = append(parts, part) }
	o.DrawPowerOverlay(v, ch, &Links{}, screen, w, 640, 640, 64)
	if len(parts) != 0 {
		t.Fatalf("hidden overlay traced %v", parts)
	}
	o.Power.On = true
	o.DrawPowerOverlay(v, ch, &Links{}, screen, w, 640, 640, 64)
	want := map[string]bool{"grid": true, "link": true, "consumer": true, "unpowered": true, "disconnected": true, "battery": true, "storage": true}
	for _, p := range parts {
		delete(want, p)
	}
	if len(want) != 0 {
		t.Fatalf("missing parts %v in %v", want, parts)
	}
	if c := o.Wiring.Nets.NetColour(7); c == o.Wiring.Nets.NetColour(0) {
		t.Fatal("a live net is not the no-net grey")
	}
	var traced []string
	o.Wiring.Parts.Trace = func(part, key string) { traced = append(traced, part) }
	o.Wiring.DrawRangeRing(v, screen, 3, 3, TurretRangeTiles, color.NRGBA{255, 0, 0, 255})
	o.Wiring.DrawNoPowerBadges(v, ch, screen, w, 64) // no art: no badge
	if len(traced) != 1 || traced[0] != "ring" {
		t.Fatalf("traced %v", traced)
	}
	if _, ok := o.BuildingByID(v, 3); !ok {
		t.Fatal("the battery by id")
	}
}

func TestPathsAndArrivalsWithoutArt(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	screen := ebiten.NewImage(640, 640)
	v := testView(game.WorldView{})
	v.Pawns = []game.PawnView{{ID: 1, X: 1, Y: 1, Path: [][2]int{{2, 1}, {3, 1}}, HasGoal: true, GoalX: 3, GoalY: 1}}
	v.Selected, v.HasSelect = 1, true
	o.DrawPaths(v, screen, 640, 640, 64, func(p game.PawnView) (float64, float64) { return float64(p.X), float64(p.Y) })
	o.DrawArrivals(v, screen, game.WorldView{Arrivals: []game.ArrivalView{{X: 1, Y: 1, Tick: 5}}}, 64)
	o.DrawStains(v, sprite.Pass{Cast: &light.Cast{}}, screen, game.WorldView{Stains: []game.StainView{{X: 1, Y: 1, Amount: 1}}}, 640, 640, 64)
	o.DrawDirt(v, sprite.Pass{Cast: &light.Cast{}}, screen, game.WorldView{Dirt: []game.DirtView{{X: 1, Y: 1, Amount: 1}}}, 640, 640, 64)
}
