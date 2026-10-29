package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// cropTestHeights is each stage's unit height in tiles at scale 1 in the
// test art: sown specks, a sprout, a young plant, full foliage, the
// ripe plant, the wilted one.
var cropTestHeights = [env.NumCropStages]float64{6.0 / 64, 13.0 / 64, 29.0 / 64, 51.0 / 64, 58.0 / 64, 45.0 / 64}

// cropTestRoot is the stored row of a test unit's root: the art's
// tile-centre point (origin.y + 64 = 136 canvas px, at half size), where
// crop art stands a unit's root (plant_base.py CX, CY = 0.5, -0.5).
const cropTestRoot = 68

// cropUnitSprite is a unit sprite h tiles tall: a block whose bottom
// edge is the root (metadata root = (origin.x + 64, origin.y + 64) of a
// 128×192 canvas stored at half size).
func cropUnitSprite(h float64) *artpkg.Sprite {
	return cropBlockSprite(max(int(math.Round(h*64)), 1), 0) // 64 stored px a tile
}

// cropBlockSprite is a unit sprite whose block reaches up rows stored
// px above the root and down rows below it.
func cropBlockSprite(up, down int) *artpkg.Sprite {
	pix := image.NewNRGBA(image.Rect(0, 0, 64, 96))
	for y := cropTestRoot - up; y < cropTestRoot+down; y++ {
		for x := 22; x < 42; x++ {
			pix.SetNRGBA(x, y, color.NRGBA{120, 150, 80, 255})
		}
	}
	return &artpkg.Sprite{Meta: artpkg.Meta{Canvas: [2]int{128, 192}, Origin: [2]float64{0, 72}, Footprint: [2]int{1, 1}}, Pix: pix}
}

// cropTestArt is a soil catalog with unit sprites (four variants of each
// stage) for the species, and the four tilled furrow decals.
func cropTestArt(species ...string) *artpkg.Catalog {
	c := envArt("terrain/soil_00", "terrain/soil_01", "terrain/soil_02", "terrain/soil_03", "terrain/stone_00")
	for _, sp := range species {
		for st, name := range env.CropStageNames {
			for v := 0; v < env.CropVariants; v++ {
				c.Sprites[fmt.Sprintf("plant/%s_%s_v%d", sp, name, v)] = cropUnitSprite(cropTestHeights[st])
			}
		}
	}
	for v := 0; v < 4; v++ {
		pix := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		for i := range pix.Pix {
			pix.Pix[i] = 200
		}
		c.Sprites[fmt.Sprintf("overlay/tilled_%d", v)] = &artpkg.Sprite{Meta: artpkg.Meta{Canvas: [2]int{128, 128}, Footprint: [2]int{1, 1}}, Pix: pix}
	}
	return c
}

// cropPlants is a w×h field of one crop from (x0, y0), ripe and in a
// grow zone, edit applied to each.
func cropPlants(species string, x0, y0, w, h int, edit func(*game.PlantView)) []game.PlantView {
	var out []game.PlantView
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			p := game.PlantView{ID: entity.EntityID(1000 + y*100 + x), X: x, Y: y, Species: species, Zone: species, Growth: 100, Stage: 3, Harvestable: true}
			if edit != nil {
				edit(&p)
			}
			out = append(out, p)
		}
	}
	return out
}

func cropWorld(mw, mh int, plants []game.PlantView, zones ...game.ZoneView) game.WorldView {
	return game.WorldView{TerrainW: mw, Terrain: make([]uint8, mw*mh), StructVer: 1, Plants: plants, Zones: zones}
}

func cropRenderer(w game.WorldView, mw, mh int, species ...string) *Renderer {
	sim := &stubSim{w: mw, h: mh, world: w}
	r := New(sim)
	r.art = cropTestArt(species...)
	r.speed = speedPaused
	return r
}

// cropDraw queues every plant of the stub world through drawCropObj at
// tile px on a screen showing the whole map, and returns the unit quads
// queued (still pending in the batch, bar those a new species flushed).
func cropDraw(r *Renderer, mw, mh int, tile float64) int {
	r.cam.Zoom, r.cam.OffsetX, r.cam.OffsetY = tile/camerapkg.TileSize, -tile, -tile
	sw, sh := int(float64(mw+2)*tile), int(float64(mh+2)*tile)
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	w := r.game.World()
	r.view.W = w
	r.stepEnvironment(w, nil)
	dst := ebiten.NewImage(sw, sh)
	r.pipe().LOD.Crops.Begin()
	for i := range w.Plants {
		r.drawCropObj(dst, &w, &w.Plants[i], tile)
	}
	return r.pipe().LOD.Crops.Quads
}

func TestCropUnitCounts(t *testing.T) {
	t.Parallel()
	want := map[string]int{"corn": 2, "wheat": 4, "brewgrain": 4, "cotton": 3, "potato": 3, "beans": 3,
		"healroot": 3, "ashleaf": 3, "lettuce": 4, "radish": 4, "pumpkin": 1}
	for sp, n := range want {
		if got := env.CropUnitCount(sp); got != n {
			t.Errorf("%s: %d units, want %d", sp, got, n)
		}
		for _, vert := range []bool{false, true} {
			if got := len(env.CropUnits(3, 4, n, vert, nil)); got != n {
				t.Errorf("%s vertical=%v: layout of %d units", sp, vert, got)
			}
		}
	}
	for _, c := range game.Crops() {
		if env.CropUnitCount(c.Name) < 1 {
			t.Errorf("crop %s has no unit count", c.Name)
		}
	}
	if env.CropUnitCount("not a crop") != 1 {
		t.Error("unknown species is one unit")
	}
}

// TestCropGrowthScaleContinuous: a unit's drawn height (stage sprite
// height × scale) rises smoothly with growth, with no pop where the stage
// changes.
func TestCropGrowthScaleContinuous(t *testing.T) {
	t.Parallel()
	a := buildCropAtlas(cropTestArt("corn"), []string{"corn"})
	set := a.Sets["corn"]
	if set == nil {
		t.Fatal("no corn set in the atlas")
	}
	drawn := func(g int) float64 {
		st := env.CropGrowthStage(game.PlantView{Species: "corn", Growth: g, Stage: stageOfGrowth(g)})
		return cropTestHeights[st] * set.Scale(st, g)
	}
	prev := drawn(8)
	for g := 9; g <= 100; g++ {
		h := drawn(g)
		if h < prev-1e-6 {
			t.Errorf("growth %d: height %.3f < %.3f at %d (shrinks)", g, h, prev, g-1)
		}
		if h-prev > 0.045 {
			t.Errorf("growth %d: height jumps %.3f -> %.3f", g, prev, h)
		}
		prev = h
	}
	if h := drawn(100); math.Abs(h-cropTestHeights[env.StHarvestable]) > 1e-6 {
		t.Errorf("ripe plant draws %.3f tall, want its sprite's %.3f", h, cropTestHeights[env.StHarvestable])
	}
	if s := set.Scale(env.StSown, 3); s != 1 {
		t.Errorf("sown scale %v, want 1", s)
	}
}

// stageOfGrowth is the sim's stage for a growth percent (plant.Stage).
func stageOfGrowth(g int) int {
	switch {
	case g >= 100:
		return 3
	case g >= 55:
		return 2
	case g >= 20:
		return 1
	}
	return 0
}

// TestCropDamageThinsTile: through the draw path, a damaged tile draws
// fewer unit quads as damage passes the thresholds, never more.
func TestCropDamageThinsTile(t *testing.T) {
	t.Parallel()
	prev := 4
	counts := map[int]bool{}
	for d := 0; d <= 100; d += 2 {
		w := cropWorld(4, 4, cropPlants("wheat", 1, 1, 1, 1, func(p *game.PlantView) { p.Damage, p.DamageKind = d, "cold" }))
		r := cropRenderer(w, 4, 4, "wheat")
		got := cropDraw(r, 4, 4, 64)
		if got > prev {
			t.Fatalf("damage %d: %d quads after %d (more units as it gets worse)", d, got, prev)
		}
		if d < 70 && got != 4 {
			t.Fatalf("damage %d: %d units, want all four until 70%%", d, got)
		}
		if d == 100 && got != 0 {
			t.Fatalf("destroyed crop still draws %d units", got)
		}
		counts[got] = true
		prev = got
	}
	if len(counts) < 4 {
		t.Fatalf("the tile thins out in %d steps only: %v", len(counts), counts)
	}
}

// TestCropDrawsUnitsPerTile: every crop tile in view queues its units,
// by species, in one pending batch.
func TestCropDrawsUnitsPerTile(t *testing.T) {
	t.Parallel()
	var ps []game.PlantView
	ps = append(ps, cropPlants("corn", 0, 0, 3, 2, nil)...)    // 6 tiles × 2
	ps = append(ps, cropPlants("wheat", 0, 3, 2, 2, nil)...)   // 4 tiles × 4
	ps = append(ps, cropPlants("pumpkin", 5, 0, 2, 1, nil)...) // 2 tiles × 1
	r := cropRenderer(cropWorld(10, 10, ps), 10, 10, "corn", "wheat", "pumpkin")
	if got, want := cropDraw(r, 10, 10, 64), 6*2+4*4+2*1; got != want {
		t.Fatalf("%d unit quads, want %d", got, want)
	}
	for _, v := range r.pipe().LOD.Crops.Verts {
		if v.ColorA != 1 {
			t.Fatalf("healthy crop vertex alpha %v", v.ColorA)
		}
	}
	// each species joining the atlas flushed the units before it (2), the last run now
	r.pipe().Env.FlushCrops(ebiten.NewImage(10, 10))
	if r.pipe().LOD.Crops.Flushes != 3 || len(r.pipe().LOD.Crops.Idx) != 0 {
		t.Fatalf("flush: %d draws, %d indices left", r.pipe().LOD.Crops.Flushes, len(r.pipe().LOD.Crops.Idx))
	}
	r.pipe().Env.FlushCrops(ebiten.NewImage(10, 10))
	if r.pipe().LOD.Crops.Flushes != 3 {
		t.Fatal("an empty batch drew")
	}
}

// TestCropStageSprites: a unit draws the sprite of its stage: a sown
// crop is the 0.1-tile specks, a ripe one the 0.9-tile plant; damage from
// 50% draws the wilted one.
func TestCropStageSprites(t *testing.T) {
	t.Parallel()
	height := func(edit func(*game.PlantView)) float64 {
		w := cropWorld(3, 3, cropPlants("pumpkin", 1, 1, 1, 1, edit))
		r := cropRenderer(w, 3, 3, "pumpkin")
		if cropDraw(r, 3, 3, 64) != 1 {
			t.Fatal("pumpkin is one unit")
		}
		vs := r.pipe().LOD.Crops.Verts
		return float64(vs[2].DstY-vs[0].DstY) / 64 // screen height in tiles at scale 1 (no scale: trim and growth)
	}
	sown := height(func(p *game.PlantView) { p.Growth, p.Stage, p.Harvestable = 2, 0, false })
	ripe := height(nil)
	wilted := height(func(p *game.PlantView) { p.Damage, p.DamageKind = 50, "cold" })
	if !(sown < 0.15 && ripe > 0.8 && ripe < 1.1) {
		t.Fatalf("heights: sown %.3f ripe %.3f", sown, ripe)
	}
	if !(wilted < ripe && wilted > 0.5) {
		t.Fatalf("wilted %.3f vs ripe %.3f: the wilted sprite is shorter", wilted, ripe)
	}
}

// TestCropLODTiers: from 48 px every unit draws; below, one
// representative unit a tile.
func TestCropLODTiers(t *testing.T) {
	t.Parallel()
	ps := cropPlants("wheat", 0, 0, 4, 3, nil)
	r := cropRenderer(cropWorld(8, 8, ps), 8, 8, "wheat")
	if got := cropDraw(r, 8, 8, 64); got != 12*4 {
		t.Fatalf("64 px: %d units, want 48", got)
	}
	r = cropRenderer(cropWorld(8, 8, ps), 8, 8, "wheat")
	if got := cropDraw(r, 8, 8, 32); got != 12 {
		t.Fatalf("32 px: %d quads, want one a tile (12)", got)
	}
	// the representative unit is larger than a single unit of the tile
	// where the tile has room for it (a young crop); a ripe one fills the
	// tile's height either way (PLAY-019: every unit stays on its tile),
	// so its representative is at least as large as its largest unit
	widths := func(edit func(*game.PlantView)) (maxW, repW float64) {
		r := cropRenderer(cropWorld(8, 8, cropPlants("wheat", 1, 1, 1, 1, edit)), 8, 8, "wheat")
		cropDraw(r, 8, 8, 64)
		for i := 0; i+3 < len(r.pipe().LOD.Crops.Verts); i += 4 {
			maxW = max(maxW, float64(r.pipe().LOD.Crops.Verts[i+1].DstX-r.pipe().LOD.Crops.Verts[i].DstX))
		}
		r = cropRenderer(cropWorld(8, 8, cropPlants("wheat", 1, 1, 1, 1, edit)), 8, 8, "wheat")
		cropDraw(r, 8, 8, 32)
		return maxW, float64(r.pipe().LOD.Crops.Verts[1].DstX-r.pipe().LOD.Crops.Verts[0].DstX) * 2 // at 64 px
	}
	if maxW, repW := widths(func(p *game.PlantView) { p.Growth, p.Stage, p.Harvestable = 37, 1, false }); repW <= maxW*1.05 {
		t.Fatalf("young: representative unit %.1f px wide at 64 px vs %.1f for a unit", repW, maxW)
	}
	if maxW, repW := widths(nil); repW < maxW*0.999 {
		t.Fatalf("ripe: representative unit %.1f px wide at 64 px vs %.1f for a unit", repW, maxW)
	}
}

// TestCropFrameTiers: through drawArt, units draw live from 48 px, one a
// tile in the chunk tier, and the map tier draws marks only.
func TestCropFrameTiers(t *testing.T) {
	t.Parallel()
	ps := cropPlants("wheat", 1, 1, 4, 3, nil)
	r := cropRenderer(cropWorld(64, 64, ps), 64, 64, "wheat")
	lodFrame(r, 64)
	if q := r.pipe().LOD.Crops.Quads; q != 12*4 {
		t.Fatalf("64 px: %d unit quads, want 48", q)
	}
	lodFrame(r, 32)
	if q := r.pipe().LOD.Crops.Quads; q != 12 {
		t.Fatalf("32 px: %d unit quads, want 12", q)
	}
	lodFrame(r, 8)
	if r.pipe().LOD.Tier != worldpkg.Map || r.pipe().LOD.Crops.Quads != 0 {
		t.Fatalf("8 px: tier %d, %d unit quads, want the map marks only", r.pipe().LOD.Tier, r.pipe().LOD.Crops.Quads)
	}
}

// TestCropBatchFlushOrder: crops of a draw run share one batch; an object
// between them in draw order (a pawn, a building) splits it, so units
// never paint over what stands in front of them.
func TestCropBatchFlushOrder(t *testing.T) {
	t.Parallel()
	ps := cropPlants("wheat", 1, 1, 6, 1, nil)
	w := cropWorld(12, 12, ps)
	w.Buildings = []game.BuildingView{{ID: 5, Kind: "wall", X: 4, Y: 1, W: 1, H: 1, Stuff: "wood"}}
	r := cropRenderer(w, 12, 12, "wheat")
	lodFrame(r, 64)
	if r.pipe().LOD.Crops.Quads != 6*4 {
		t.Fatalf("%d unit quads, want 24", r.pipe().LOD.Crops.Quads)
	}
	if got := r.pipe().LOD.Crops.Flushes; got != 2 {
		t.Fatalf("a wall splits the row into %d crop batches, want 2", got)
	}
	field := cropWorld(12, 12, cropPlants("wheat", 1, 1, 8, 6, nil))
	r2 := cropRenderer(field, 12, 12, "wheat")
	lodFrame(r2, 64)
	if got := r2.pipe().LOD.Crops.Flushes; got != 1 {
		t.Fatalf("an open field drew in %d batches, want 1", got)
	}
}

// TestCropFurrowsUnderCropsOnly: a furrow decal lies under every crop
// cell (sown or grown), not under trees, bushes or wild plants.
func TestCropFurrowsUnderCropsOnly(t *testing.T) {
	t.Parallel()
	var ps []game.PlantView
	ps = append(ps, cropPlants("corn", 1, 1, 3, 2, nil)...)
	ps = append(ps, cropPlants("potato", 1, 4, 2, 1, func(p *game.PlantView) { p.Growth, p.Stage, p.Harvestable = 1, 0, false })...) // sown
	ps = append(ps,
		game.PlantView{ID: 1, Species: "tree", X: 7, Y: 1, Growth: 100},
		game.PlantView{ID: 2, Species: "berry_bush", X: 7, Y: 2, Growth: 100},
		game.PlantView{ID: 3, Species: "healroot", X: 7, Y: 3, Growth: 100}, // wild: no zone
		game.PlantView{ID: 4, Species: "bramble", X: 7, Y: 4, Growth: 100},
	)
	r := cropRenderer(cropWorld(12, 12, ps), 12, 12, "corn", "potato")
	lodFrame(r, 64)
	if got, want := r.pipe().LOD.Crops.Furrows, 3*2+2; got != want {
		t.Fatalf("%d furrow quads, want %d (one per crop cell)", got, want)
	}
	// none in the map tier
	lodFrame(r, 8)
	if r.pipe().LOD.Crops.Furrows != 0 {
		t.Fatalf("map tier drew %d furrows", r.pipe().LOD.Crops.Furrows)
	}
	// without the decal art nothing is drawn and nothing breaks
	r = cropRenderer(cropWorld(12, 12, ps), 12, 12, "corn", "potato")
	for v := 0; v < 4; v++ {
		delete(r.art.Sprites, fmt.Sprintf("overlay/tilled_%d", v))
	}
	lodFrame(r, 64)
	if r.pipe().LOD.Crops.Furrows != 0 || r.pipe().LOD.Crops.Quads == 0 {
		t.Fatalf("no furrow art: %d furrows, %d units", r.pipe().LOD.Crops.Furrows, r.pipe().LOD.Crops.Quads)
	}
}

// TestCropFurrowVariantAndTurn: the decal is one of the four tilled
// ones chosen by the tile, turned a quarter when the field's long axis
// runs north-south and left alone on an east-west or zoneless one.
func TestCropFurrowVariantAndTurn(t *testing.T) {
	t.Parallel()
	zoneOf := func(ps []game.PlantView, minX, minY, maxX, maxY int) game.ZoneView {
		z := game.ZoneView{ID: 1, Kind: game.ZoneGrow, Crop: "corn", MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
		for _, p := range ps {
			z.Cells = append(z.Cells, [2]int{p.X, p.Y})
		}
		return z
	}
	// furrows is, per tile, which way the decal's top-left corner points
	// from the quad's centre (signs of dx, dy) and which decal it is
	furrows := func(w game.WorldView) map[[2]int][4]float32 {
		r := cropRenderer(w, 12, 12, "corn")
		lodFrame(r, 64)
		cb := &r.pipe().LOD.Crops
		out := map[[2]int][4]float32{}
		for q := 0; q+3 < len(cb.Fverts); q += 4 {
			cx := (cb.Fverts[q].DstX + cb.Fverts[q+3].DstX) / 2
			cy := (cb.Fverts[q].DstY + cb.Fverts[q+3].DstY) / 2
			sign := func(v float32) float32 {
				if v < 0 {
					return -1
				}
				return 1
			}
			out[[2]int{int(cx / 64), int(cy / 64)}] = [4]float32{sign(cb.Fverts[q].DstX - cx), sign(cb.Fverts[q].DstY - cy), cb.Fverts[q].SrcX, cb.Fverts[q].SrcY}
		}
		return out
	}
	tall := cropPlants("corn", 2, 1, 2, 6, nil) // 2 wide, 6 tall
	none := furrows(cropWorld(12, 12, tall))
	vert := furrows(cropWorld(12, 12, tall, zoneOf(tall, 2, 1, 3, 6)))
	if len(none) != 12 || len(vert) != 12 {
		t.Fatalf("furrow cells %d / %d, want 12", len(none), len(vert))
	}
	for c, u := range none {
		v := vert[c]
		// turned a quarter: the same decal, its top-left corner pointing elsewhere
		if v[2] != u[2] || v[3] != u[3] || v[0] != -u[1] || v[1] != u[0] {
			t.Errorf("tile %v: furrow %v on a north-south field vs %v on a plain one is not a quarter turn", c, v, u)
		}
	}
	wide := cropPlants("corn", 2, 1, 6, 2, nil)
	flat := furrows(cropWorld(12, 12, wide, zoneOf(wide, 2, 1, 7, 2)))
	plain := furrows(cropWorld(12, 12, wide))
	for c, u := range plain {
		if flat[c] != u {
			t.Errorf("tile %v: an east-west field's furrow differs from the zoneless one", c)
		}
	}
	decals := map[[2]float32]bool{}
	for _, u := range plain {
		decals[[2]float32{u[2], u[3]}] = true
	}
	if len(decals) < 2 {
		t.Fatalf("furrows use %d decals, want a mix of the four", len(decals))
	}
}

// TestCropLabels: the inspector and the hover name cold and heat damage.
func TestCropLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		p    game.PlantView
		want string
	}{
		{game.PlantView{Species: "corn", Damage: 40, DamageKind: "cold"}, "Freezing: 40% damaged"},
		{game.PlantView{Species: "corn", Damage: 7, DamageKind: "heat"}, "Scorching: 7% damaged"},
		{game.PlantView{Species: "corn", Damage: 63, DamageKind: "cold"}, "Freezing: 63% damaged"},
		{game.PlantView{Species: "corn", Damage: 12}, "Damaged: 12%"},
		{game.PlantView{Species: "corn", Damage: 130, DamageKind: "cold"}, "Freezing: 100% damaged"},
		{game.PlantView{Species: "corn"}, ""},
	}
	for _, c := range cases {
		if got := env.PlantDamageLine(c.p); got != c.want {
			t.Errorf("%+v: %q, want %q", c.p, got, c.want)
		}
		lines := inspectorpkg.PlantLines(c.p)
		has := false
		for _, l := range lines {
			has = has || l == c.want
		}
		if c.want != "" && !has {
			t.Errorf("inspector lines %q lack %q", lines, c.want)
		}
		if c.want == "" {
			for _, l := range lines {
				if strings.Contains(l, "damaged") {
					t.Errorf("undamaged crop shows %q", l)
				}
			}
		}
	}
	w := game.WorldView{Plants: []game.PlantView{{X: 3, Y: 2, Species: "wheat", Growth: 80, Damage: 25, DamageKind: "cold"}}}
	if got := env.PlantDamageLabelAt(w, 3, 2); got != "Freezing: 25% damaged" {
		t.Errorf("hover = %q", got)
	}
	if env.PlantDamageLabelAt(w, 0, 0) != "" || env.PlantDamageLabelAt(game.WorldView{}, 3, 2) != "" {
		t.Error("hover line where no damaged plant stands")
	}
}

// TestCropSway: units in a tile sway out of step; taller crops lean more.
func TestCropSway(t *testing.T) {
	t.Parallel()
	if !(env.CropSwayAmount("corn") > env.CropSwayAmount("lettuce") && env.CropSwayAmount("wheat") > env.CropSwayAmount("radish")) {
		t.Fatalf("corn %v wheat %v lettuce %v radish %v", env.CropSwayAmount("corn"), env.CropSwayAmount("wheat"), env.CropSwayAmount("lettuce"), env.CropSwayAmount("radish"))
	}
	if !(env.CropSwayFactor("corn") > env.CropSwayFactor("lettuce")) {
		t.Fatal("corn does not sway more than lettuce")
	}
	for _, c := range game.Crops() {
		if f := env.CropSwayFactor(c.Name); f < 0.4 || f > 1.5 {
			t.Errorf("%s sway factor %v is out of range", c.Name, f)
		}
	}
	w := cropWorld(4, 4, cropPlants("wheat", 1, 1, 1, 1, nil))
	w.WindX, w.WindY, w.Weather = 2.4, 0.4, "cloudy"
	r := cropRenderer(w, 4, 4, "wheat")
	r.present = 40
	cropDraw(r, 4, 4, 64)
	vs := r.pipe().LOD.Crops.Verts
	if len(vs) != 16 {
		t.Fatalf("%d vertices, want four units", len(vs))
	}
	shears := map[float32]bool{}
	for q := 0; q < 4; q++ {
		shears[vs[4*q].DstX-vs[4*q+2].DstX] = true
	}
	if len(shears) < 3 {
		t.Fatalf("units lean alike: %v", shears)
	}
}

// TestCropFallbackArt: a missing variant falls back to the stage's v0
// (PLAY-016: the old alias sprites are gone), a variant with no fallback
// is skipped, and with no crop art at all nothing draws and nothing
// panics.
func TestCropFallbackArt(t *testing.T) {
	t.Parallel()
	w := cropWorld(6, 6, cropPlants("corn", 1, 1, 2, 2, nil))
	// only the stage's first variant exists: every unit draws it
	r := cropRenderer(w, 6, 6)
	r.art.Sprites["plant/corn_harvestable_v0"] = cropUnitSprite(0.9)
	if got := cropDraw(r, 6, 6, 64); got != 4*2 {
		t.Fatalf("v0 fallback: %d quads, want 8", got)
	}
	// the old art's three variants: the fourth is simply not there
	r = cropRenderer(w, 6, 6, "corn")
	for k := range r.art.Sprites {
		if strings.HasPrefix(k, "plant/corn_") && strings.HasSuffix(k, "_v3") {
			delete(r.art.Sprites, k)
		}
	}
	if got := cropDraw(r, 6, 6, 64); got != 4*2 {
		t.Fatalf("three variants: %d quads, want 8", got)
	}
	if n := len(r.pipe().LOD.Crops.Atlas.Sets["corn"].Stages[env.StHarvestable]); n != 3 {
		t.Fatalf("%d ripe variants packed, want 3", n)
	}
	// no crop art at all
	r = cropRenderer(w, 6, 6)
	if got := cropDraw(r, 6, 6, 64); got != 0 {
		t.Fatalf("no crop art: %d quads", got)
	}
	r.art = nil
	r.drawCropObj(ebiten.NewImage(8, 8), &w, &w.Plants[0], 64) // no art catalog: no panic
	r.drawFurrows(ebiten.NewImage(8, 8), w, 64)
}

// TestCropSpeciesJoinAtlasLazily: a second species added mid-frame
// flushes the pending units before the atlas is repacked.
func TestCropSpeciesJoinAtlasLazily(t *testing.T) {
	t.Parallel()
	ps := append(cropPlants("corn", 1, 1, 2, 1, nil), cropPlants("wheat", 1, 3, 2, 1, nil)...)
	r := cropRenderer(cropWorld(8, 8, ps), 8, 8, "corn", "wheat")
	lodFrame(r, 64)
	if r.pipe().LOD.Crops.Quads != 2*2+2*4 {
		t.Fatalf("%d unit quads, want 12", r.pipe().LOD.Crops.Quads)
	}
	a := r.pipe().LOD.Crops.Atlas
	if a == nil || a.Sets["corn"] == nil || a.Sets["wheat"] == nil {
		t.Fatal("atlas lacks a species that drew")
	}
	if a.Sets["pumpkin"] != nil {
		t.Fatal("atlas built a species nothing draws")
	}
}

func TestCropMarkColor(t *testing.T) {
	t.Parallel()
	mark := func(p game.PlantView) color.RGBA { p.Species = "corn"; c, _ := worldpkg.PlantMarkColor(&p); return c }
	sown, ripe := mark(game.PlantView{Growth: 2, Stage: 0}), mark(game.PlantView{Growth: 100, Stage: 3})
	if sown == ripe {
		t.Error("a sown field and a ripe one share a mark colour")
	}
	if cold, plain := mark(game.PlantView{Growth: 100, Stage: 3, Damage: 40, DamageKind: "cold"}), ripe; cold == plain || cold.B <= plain.B {
		t.Errorf("a freezing crop's mark %v vs %v", cold, plain)
	}
	if heat := mark(game.PlantView{Growth: 100, Stage: 3, Damage: 40, DamageKind: "heat"}); heat == ripe || heat.B >= ripe.B {
		t.Errorf("a scorched crop's mark %v vs %v", heat, ripe)
	}
	if dead := mark(game.PlantView{Growth: 100, Stage: 3, Damage: 80, DamageKind: "cold"}); dead == ripe {
		t.Error("a dying crop's mark is the healthy one")
	}
	if _, k := worldpkg.PlantMarkColor(&game.PlantView{Species: "corn", Growth: 100}); k <= 0 {
		t.Errorf("mark size %v", k)
	}
}

// BenchmarkCropField500 is the object pass for a 25×20 field of 500
// mixed crops plus its furrows, with the real art when it is present:
// "default" at the default zoom (64 px a tile) on a 1280×800 screen (about
// 190 of them in view), "all500" at 48 px, the smallest live tile, on a
// 1920×1080 screen with every crop in view. Budget (PLAY-016): ≤ 1.0 ms.
func BenchmarkCropField500(b *testing.B) {
	species := []string{"corn", "wheat", "potato", "lettuce", "pumpkin"}
	const mw, mh = 40, 30
	var ps []game.PlantView
	for y := 0; y < 20; y++ {
		for x := 0; x < 25; x++ {
			sp := species[(x/5)%len(species)]
			ps = append(ps, cropPlants(sp, x+2, y+2, 1, 1, func(p *game.PlantView) {
				p.Stage, p.Growth = 1+(x+y)%3, 30+(x+y)*3%70
				p.Harvestable = false
			})...)
		}
	}
	w := cropWorld(mw, mh, ps)
	w.WindX, w.WindY, w.Weather = 1.2, 0.3, "cloudy"
	for _, c := range []struct {
		name       string
		tile       float64
		sw, sh     int
		wantInView int
	}{{"default", 64, 1280, 800, 150}, {"all500", 48, 1920, 1080, 500}} {
		b.Run(c.name, func(b *testing.B) {
			sim := &stubSim{w: mw, h: mh, world: w}
			r := New(sim)
			r.art = cropTestArt(species...)
			if root, err := artpkg.Locate(); err == nil {
				if cat, err := loadArtCatalog(root); err == nil && cat.Has("plant/corn_mature_v0") {
					r.art = cat
				}
			}
			r.cam.Zoom = c.tile / camerapkg.TileSize
			screen := ebiten.NewImage(c.sw, c.sh)
			frame := func() {
				r.present++
				r.pipe().Env.SetView(r.liveView(), c.sw, c.sh, c.tile)
				r.view.W = w
				r.stepEnvironment(w, nil)
				r.pipe().LOD.Crops.Begin()
				r.drawFurrows(screen, w, c.tile)
				for i := range w.Plants {
					if p := &w.Plants[i]; r.pipe().LOD.InView(p.X, p.Y, 0, 0) {
						r.drawCropObj(screen, &w, p, c.tile)
					}
				}
				r.pipe().Env.FlushCrops(screen)
			}
			for i := 0; i < 3; i++ {
				frame()
			}
			if r.pipe().LOD.Crops.Furrows < c.wantInView {
				b.Fatalf("%d crops in view, want at least %d", r.pipe().LOD.Crops.Furrows, c.wantInView)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame()
			}
			b.ReportMetric(float64(r.pipe().LOD.Crops.Quads), "unit-quads/op")
			b.ReportMetric(float64(r.pipe().LOD.Crops.Furrows), "furrows/op")
		})
	}
}
