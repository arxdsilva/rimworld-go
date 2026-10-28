package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// cropTestHeights is each stage's unit height in tiles at scale 1 in the
// test art: sown specks, a sprout, a young plant, full foliage, the
// ripe plant, the wilted one.
var cropTestHeights = [numCropStages]float64{6.0 / 64, 13.0 / 64, 29.0 / 64, 51.0 / 64, 58.0 / 64, 45.0 / 64}

// cropTestRoot is the stored row of a test unit's root: the art's
// tile-centre point (origin.y + 64 = 136 canvas px, at half size), where
// crop art stands a unit's root (plant_base.py CX, CY = 0.5, -0.5).
const cropTestRoot = 68

// cropUnitSprite is a unit sprite h tiles tall: a block whose bottom
// edge is the root (metadata root = (origin.x + 64, origin.y + 64) of a
// 128×192 canvas stored at half size).
func cropUnitSprite(h float64) *artSprite {
	return cropBlockSprite(max(int(math.Round(h*64)), 1), 0) // 64 stored px a tile
}

// cropBlockSprite is a unit sprite whose block reaches up rows stored
// px above the root and down rows below it.
func cropBlockSprite(up, down int) *artSprite {
	pix := image.NewNRGBA(image.Rect(0, 0, 64, 96))
	for y := cropTestRoot - up; y < cropTestRoot+down; y++ {
		for x := 22; x < 42; x++ {
			pix.SetNRGBA(x, y, color.NRGBA{120, 150, 80, 255})
		}
	}
	return &artSprite{meta: artMeta{Canvas: [2]int{128, 192}, Origin: [2]float64{0, 72}, Footprint: [2]int{1, 1}}, pix: pix}
}

// cropTestArt is a soil catalog with unit sprites (four variants of each
// stage) for the species, and the four tilled furrow decals.
func cropTestArt(species ...string) *artCatalog {
	c := envArt("terrain/soil_00", "terrain/soil_01", "terrain/soil_02", "terrain/soil_03", "terrain/stone_00")
	for _, sp := range species {
		for st, name := range cropStageNames {
			for v := 0; v < cropVariants; v++ {
				c.sprites[fmt.Sprintf("plant/%s_%s_v%d", sp, name, v)] = cropUnitSprite(cropTestHeights[st])
			}
		}
	}
	for v := 0; v < 4; v++ {
		pix := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		for i := range pix.Pix {
			pix.Pix[i] = 200
		}
		c.sprites[fmt.Sprintf("overlay/tilled_%d", v)] = &artSprite{meta: artMeta{Canvas: [2]int{128, 128}, Footprint: [2]int{1, 1}}, pix: pix}
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
	r.cam.zoom, r.cam.offsetX, r.cam.offsetY = tile/TileSize, -tile, -tile
	sw, sh := int(float64(mw+2)*tile), int(float64(mh+2)*tile)
	r.setView(sw, sh, tile)
	w := r.game.World()
	r.view.W = w
	r.stepEnvironment(w, nil)
	dst := ebiten.NewImage(sw, sh)
	r.lod.crops.begin()
	for i := range w.Plants {
		r.drawCropObj(dst, &w, &w.Plants[i], tile)
	}
	return r.lod.crops.quads
}

func TestCropUnitCounts(t *testing.T) {
	t.Parallel()
	want := map[string]int{"corn": 2, "wheat": 4, "brewgrain": 4, "cotton": 3, "potato": 3, "beans": 3,
		"healroot": 3, "ashleaf": 3, "lettuce": 4, "radish": 4, "pumpkin": 1}
	for sp, n := range want {
		if got := cropUnitCount(sp); got != n {
			t.Errorf("%s: %d units, want %d", sp, got, n)
		}
		for _, vert := range []bool{false, true} {
			if got := len(cropUnits(3, 4, n, vert, nil)); got != n {
				t.Errorf("%s vertical=%v: layout of %d units", sp, vert, got)
			}
		}
	}
	for _, c := range game.Crops() {
		if cropUnitCount(c.Name) < 1 {
			t.Errorf("crop %s has no unit count", c.Name)
		}
	}
	if cropUnitCount("not a crop") != 1 {
		t.Error("unknown species is one unit")
	}
}

// TestCropUnitsDeterministic: a tile's units are a pure function of the
// tile; tiles differ.
func TestCropUnitsDeterministic(t *testing.T) {
	t.Parallel()
	same := 0
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			a := cropUnits(x, y, 4, false, nil)
			b := cropUnits(x, y, 4, false, nil)
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("tile (%d,%d) unit %d differs between calls: %+v vs %+v", x, y, i, a[i], b[i])
				}
			}
			if o := cropUnits(x+1, y, 4, false, nil); o[0] == a[0] {
				same++
			}
		}
	}
	if same > 4 {
		t.Fatalf("%d of 144 neighbouring tiles share their first unit exactly", same)
	}
}

// TestCropUnitsRowAlignment: on an east-west field every unit of a
// species sits on one of the pattern's rows (across jitter), so a field
// reads in rows; a north-south field turns the rows on their side.
func TestCropUnitsRowAlignment(t *testing.T) {
	t.Parallel()
	check := func(n int, vertical bool, rows []float64) {
		t.Helper()
		var buf []cropUnit
		hit := make([]int, len(rows))
		for y := 0; y < 30; y++ {
			for x := 0; x < 30; x++ {
				buf = cropUnits(x, y, n, vertical, buf[:0])
				for _, u := range buf {
					across, along := u.dy, u.dx
					if vertical {
						across, along = u.dx, u.dy
					}
					found := false
					for i, r := range rows {
						if math.Abs(across-r) <= cropJitterAcross+1e-9 {
							found = true
							hit[i]++
						}
					}
					if !found {
						t.Fatalf("n=%d vertical=%v tile (%d,%d): unit across=%.3f is off every row %v", n, vertical, x, y, across, rows)
					}
					if along < -0.2 || along > 1.2 {
						t.Fatalf("n=%d: unit along=%.3f strays from its tile", n, along)
					}
				}
			}
		}
		for i, h := range hit {
			if h == 0 {
				t.Errorf("n=%d vertical=%v: row %d (v=%.2f) never used", n, vertical, i, rows[i])
			}
		}
	}
	check(2, false, []float64{0.5})
	check(2, true, []float64{0.5})
	check(4, false, []float64{0.3, 0.76})
	check(4, true, []float64{0.3, 0.76})
	check(3, false, []float64{0.3, 0.78})
	check(1, false, []float64{0.55})
	// the along-row spread: wheat's four units never pile on one spot
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			us := cropUnits(x, y, 4, false, nil)
			for i := range us {
				for j := i + 1; j < len(us); j++ {
					if math.Hypot(us[i].dx-us[j].dx, us[i].dy-us[j].dy) < 0.12 {
						t.Fatalf("tile (%d,%d): units %d and %d overlap", x, y, i, j)
					}
				}
			}
		}
	}
}

// TestCropUnitsMirrorVariantTrim: units mirror at random, pick all four
// variants and trim their size within 0.9–1.1; sway share and phase vary.
func TestCropUnitsMirrorVariantTrim(t *testing.T) {
	t.Parallel()
	mirrored, plain := 0, 0
	variants := map[uint32]bool{}
	phases := map[float64]bool{}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			for _, u := range cropUnits(x, y, 4, false, nil) {
				if u.mirror {
					mirrored++
				} else {
					plain++
				}
				variants[u.variant%cropVariants] = true
				phases[u.phase] = true
				if u.trim < 0.9-1e-9 || u.trim > 1.1+1e-9 {
					t.Fatalf("trim %.3f outside 0.9–1.1", u.trim)
				}
				if u.sway < 0.8-1e-9 || u.sway > 1.2+1e-9 {
					t.Fatalf("sway share %.3f outside 0.8–1.2", u.sway)
				}
			}
		}
	}
	if mirrored < 500 || plain < 500 {
		t.Fatalf("mirroring not mixed: %d mirrored, %d plain", mirrored, plain)
	}
	if len(variants) != cropVariants {
		t.Fatalf("variants used %v, want all %d", variants, cropVariants)
	}
	if len(phases) < 1000 {
		t.Fatalf("only %d distinct sway phases in 1600 units", len(phases))
	}
}

// TestCropUnitRanks: a tile's drop-out ranks are a permutation of 0..n-1.
func TestCropUnitRanks(t *testing.T) {
	t.Parallel()
	first := map[int]int{}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			for n := 1; n <= 4; n++ {
				seen := make([]bool, n)
				for _, u := range cropUnits(x, y, n, false, nil) {
					if u.rank < 0 || u.rank >= n || seen[u.rank] {
						t.Fatalf("tile (%d,%d) n=%d: bad rank %d", x, y, n, u.rank)
					}
					seen[u.rank] = true
				}
			}
			for i, u := range cropUnits(x, y, 4, false, nil) {
				if u.rank == 0 {
					first[i]++
				}
			}
		}
	}
	if len(first) < 4 {
		t.Fatalf("the first unit to go is always the same slot: %v", first)
	}
}

// TestCropGrowthScaleContinuous: a unit's drawn height (stage sprite
// height × scale) rises smoothly with growth, with no pop where the stage
// changes.
func TestCropGrowthScaleContinuous(t *testing.T) {
	t.Parallel()
	a := buildCropAtlas(cropTestArt("corn"), []string{"corn"})
	set := a.sets["corn"]
	if set == nil {
		t.Fatal("no corn set in the atlas")
	}
	drawn := func(g int) float64 {
		st := cropGrowthStage(game.PlantView{Species: "corn", Growth: g, Stage: stageOfGrowth(g)})
		return cropTestHeights[st] * set.scale(st, g)
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
	if h := drawn(100); math.Abs(h-cropTestHeights[stHarvestable]) > 1e-6 {
		t.Errorf("ripe plant draws %.3f tall, want its sprite's %.3f", h, cropTestHeights[stHarvestable])
	}
	if s := set.scale(stSown, 3); s != 1 {
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

func TestCropStageFollowsDamage(t *testing.T) {
	t.Parallel()
	base := game.PlantView{Species: "corn", Growth: 100, Stage: 3}
	for _, c := range []struct {
		damage int
		want   int
	}{{0, stHarvestable}, {25, stHarvestable}, {49, stHarvestable}, {50, stWilted}, {80, stWilted}, {100, stWilted}} {
		p := base
		p.Damage = c.damage
		if got := cropStage(p); got != c.want {
			t.Errorf("damage %d: stage %d, want %d", c.damage, got, c.want)
		}
	}
	// a crop with the plain wilt flag but no damage keeps its normal look
	if got := cropStage(game.PlantView{Species: "corn", Growth: 60, Stage: 2, Wilted: true}); got != stMature {
		t.Errorf("Wilted flag alone: stage %d, want mature", got)
	}
	if got := cropStage(game.PlantView{Species: "corn", Growth: 3, Stage: 0}); got != stSown {
		t.Errorf("fresh sowing: stage %d", got)
	}
	if got := cropStage(game.PlantView{Species: "corn", Growth: 9, Stage: 0}); got != stSprout {
		t.Errorf("sprouting: stage %d", got)
	}
}

// TestCropDamageTint: below 50% damage the sprite is the normal one with
// a rime (cold: cooler, lighter) or scorch (heat: browner, darker blue)
// tint that grows with the damage.
func TestCropDamageTint(t *testing.T) {
	t.Parallel()
	if cs := cropDamageTint(0, "cold"); cs != noCS {
		t.Errorf("no damage tints %v", cs)
	}
	prevB := float32(1)
	prevScorchB := float32(1)
	for d := 5; d < cropWiltDamage; d += 5 {
		cold, heat := cropDamageTint(d, "cold"), cropDamageTint(d, "heat")
		if cold[2] <= prevB {
			t.Errorf("cold tint's blue %.3f at %d%% does not grow (was %.3f)", cold[2], d, prevB)
		}
		if heat[2] >= prevScorchB {
			t.Errorf("scorch blue %.3f at %d%% does not fall (was %.3f)", heat[2], d, prevScorchB)
		}
		if cold[2] <= cold[0] {
			t.Errorf("rime %v is not blue-white at %d%%", cold, d)
		}
		if heat[0] <= heat[2] || heat[1] <= heat[2] {
			t.Errorf("scorch %v is not brown-yellow at %d%%", heat, d)
		}
		prevB, prevScorchB = cold[2], heat[2]
	}
	if cropDamageTint(40, "cold") == cropDamageTint(40, "heat") {
		t.Error("cold and heat tint alike")
	}
	if a := cropDamageTint(40, "cold")[3]; a != 1 {
		t.Errorf("tint alpha %v", a)
	}
}

// TestCropUnitLook: from 50% the units shrink towards 60% and, over the
// last 30%, fade out one by one, each in its own band, so a tile thins
// out; at 100% nothing is left.
func TestCropUnitLook(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 3, 4} {
		for rank := 0; rank < n; rank++ {
			if s, a := cropUnitLook(0, rank, n); s != 1 || a != 1 {
				t.Errorf("n=%d rank %d undamaged: scale %v alpha %v", n, rank, s, a)
			}
			if s, a := cropUnitLook(49, rank, n); s != 1 || a != 1 {
				t.Errorf("n=%d rank %d at 49%%: scale %v alpha %v", n, rank, s, a)
			}
			if s, a := cropUnitLook(50, rank, n); s != 1 || a != 1 {
				t.Errorf("n=%d rank %d at 50%%: scale %v alpha %v", n, rank, s, a)
			}
			if s, _ := cropUnitLook(100, rank, n); math.Abs(s-cropMinShrink) > 1e-9 {
				t.Errorf("n=%d rank %d at 100%%: scale %v, want %v", n, rank, s, cropMinShrink)
			}
			if _, a := cropUnitLook(100, rank, n); a != 0 {
				t.Errorf("n=%d rank %d still drawn at 100%%: alpha %v", n, rank, a)
			}
			if _, a := cropUnitLook(70, rank, n); a < 1-1e-9 && rank > 0 {
				t.Errorf("n=%d rank %d fades before its band: %v", n, rank, a)
			}
		}
		// shrink is monotone and continuous
		prev := 1.0
		for d := 50; d <= 100; d++ {
			s, _ := cropUnitLook(d, 0, n)
			if s > prev+1e-9 || prev-s > 0.01 {
				t.Errorf("n=%d damage %d: shrink %.4f after %.4f", n, d, s, prev)
			}
			prev = s
		}
		// units go one by one, rank 0 first, each fully gone before the next
		// starts to fade, and the tile is empty at 100
		goneAt := make([]int, n)
		for rank := 0; rank < n; rank++ {
			goneAt[rank] = -1
			for d := 50; d <= 100; d++ {
				if _, a := cropUnitLook(d, rank, n); a == 0 {
					goneAt[rank] = d
					break
				}
			}
			if goneAt[rank] < 0 {
				t.Errorf("n=%d rank %d never disappears", n, rank)
			}
			if rank > 0 && goneAt[rank] <= goneAt[rank-1] && n > 1 {
				t.Errorf("n=%d: rank %d gone at %d, not after rank %d at %d", n, rank, goneAt[rank], rank-1, goneAt[rank-1])
			}
		}
		if goneAt[n-1] != 100 {
			t.Errorf("n=%d: last unit gone at %d, want 100", n, goneAt[n-1])
		}
	}
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
	for _, v := range r.lod.crops.verts {
		if v.ColorA != 1 {
			t.Fatalf("healthy crop vertex alpha %v", v.ColorA)
		}
	}
	// each species joining the atlas flushed the units before it (2), the last run now
	r.flushCrops(ebiten.NewImage(10, 10))
	if r.lod.crops.flushes != 3 || len(r.lod.crops.idx) != 0 {
		t.Fatalf("flush: %d draws, %d indices left", r.lod.crops.flushes, len(r.lod.crops.idx))
	}
	r.flushCrops(ebiten.NewImage(10, 10))
	if r.lod.crops.flushes != 3 {
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
		vs := r.lod.crops.verts
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
		for i := 0; i+3 < len(r.lod.crops.verts); i += 4 {
			maxW = max(maxW, float64(r.lod.crops.verts[i+1].DstX-r.lod.crops.verts[i].DstX))
		}
		r = cropRenderer(cropWorld(8, 8, cropPlants("wheat", 1, 1, 1, 1, edit)), 8, 8, "wheat")
		cropDraw(r, 8, 8, 32)
		return maxW, float64(r.lod.crops.verts[1].DstX-r.lod.crops.verts[0].DstX) * 2 // at 64 px
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
	if q := r.lod.crops.quads; q != 12*4 {
		t.Fatalf("64 px: %d unit quads, want 48", q)
	}
	lodFrame(r, 32)
	if q := r.lod.crops.quads; q != 12 {
		t.Fatalf("32 px: %d unit quads, want 12", q)
	}
	lodFrame(r, 8)
	if r.lod.tier != lodMap || r.lod.crops.quads != 0 {
		t.Fatalf("8 px: tier %d, %d unit quads, want the map marks only", r.lod.tier, r.lod.crops.quads)
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
	if r.lod.crops.quads != 6*4 {
		t.Fatalf("%d unit quads, want 24", r.lod.crops.quads)
	}
	if got := r.lod.crops.flushes; got != 2 {
		t.Fatalf("a wall splits the row into %d crop batches, want 2", got)
	}
	field := cropWorld(12, 12, cropPlants("wheat", 1, 1, 8, 6, nil))
	r2 := cropRenderer(field, 12, 12, "wheat")
	lodFrame(r2, 64)
	if got := r2.lod.crops.flushes; got != 1 {
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
	if got, want := r.lod.crops.furrows, 3*2+2; got != want {
		t.Fatalf("%d furrow quads, want %d (one per crop cell)", got, want)
	}
	// none in the map tier
	lodFrame(r, 8)
	if r.lod.crops.furrows != 0 {
		t.Fatalf("map tier drew %d furrows", r.lod.crops.furrows)
	}
	// without the decal art nothing is drawn and nothing breaks
	r = cropRenderer(cropWorld(12, 12, ps), 12, 12, "corn", "potato")
	for v := 0; v < 4; v++ {
		delete(r.art.sprites, fmt.Sprintf("overlay/tilled_%d", v))
	}
	lodFrame(r, 64)
	if r.lod.crops.furrows != 0 || r.lod.crops.quads == 0 {
		t.Fatalf("no furrow art: %d furrows, %d units", r.lod.crops.furrows, r.lod.crops.quads)
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
		cb := &r.lod.crops
		out := map[[2]int][4]float32{}
		for q := 0; q+3 < len(cb.fverts); q += 4 {
			cx := (cb.fverts[q].DstX + cb.fverts[q+3].DstX) / 2
			cy := (cb.fverts[q].DstY + cb.fverts[q+3].DstY) / 2
			sign := func(v float32) float32 {
				if v < 0 {
					return -1
				}
				return 1
			}
			out[[2]int{int(cx / 64), int(cy / 64)}] = [4]float32{sign(cb.fverts[q].DstX - cx), sign(cb.fverts[q].DstY - cy), cb.fverts[q].SrcX, cb.fverts[q].SrcY}
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
		if got := plantDamageLine(c.p); got != c.want {
			t.Errorf("%+v: %q, want %q", c.p, got, c.want)
		}
		lines := plantLines(c.p)
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
	if got := plantDamageLabelAt(w, 3, 2); got != "Freezing: 25% damaged" {
		t.Errorf("hover = %q", got)
	}
	if plantDamageLabelAt(w, 0, 0) != "" || plantDamageLabelAt(game.WorldView{}, 3, 2) != "" {
		t.Error("hover line where no damaged plant stands")
	}
}

// TestCropSway: units in a tile sway out of step; taller crops lean more.
func TestCropSway(t *testing.T) {
	t.Parallel()
	if !(cropSwayAmount("corn") > cropSwayAmount("lettuce") && cropSwayAmount("wheat") > cropSwayAmount("radish")) {
		t.Fatalf("corn %v wheat %v lettuce %v radish %v", cropSwayAmount("corn"), cropSwayAmount("wheat"), cropSwayAmount("lettuce"), cropSwayAmount("radish"))
	}
	if !(cropSwayFactor("corn") > cropSwayFactor("lettuce")) {
		t.Fatal("corn does not sway more than lettuce")
	}
	for _, c := range game.Crops() {
		if f := cropSwayFactor(c.Name); f < 0.4 || f > 1.5 {
			t.Errorf("%s sway factor %v is out of range", c.Name, f)
		}
	}
	w := cropWorld(4, 4, cropPlants("wheat", 1, 1, 1, 1, nil))
	w.WindX, w.WindY, w.Weather = 2.4, 0.4, "cloudy"
	r := cropRenderer(w, 4, 4, "wheat")
	r.present = 40
	cropDraw(r, 4, 4, 64)
	vs := r.lod.crops.verts
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
	r.art.sprites["plant/corn_harvestable_v0"] = cropUnitSprite(0.9)
	if got := cropDraw(r, 6, 6, 64); got != 4*2 {
		t.Fatalf("v0 fallback: %d quads, want 8", got)
	}
	// the old art's three variants: the fourth is simply not there
	r = cropRenderer(w, 6, 6, "corn")
	for k := range r.art.sprites {
		if strings.HasPrefix(k, "plant/corn_") && strings.HasSuffix(k, "_v3") {
			delete(r.art.sprites, k)
		}
	}
	if got := cropDraw(r, 6, 6, 64); got != 4*2 {
		t.Fatalf("three variants: %d quads, want 8", got)
	}
	if n := len(r.lod.crops.atlas.sets["corn"].stages[stHarvestable]); n != 3 {
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
	if r.lod.crops.quads != 2*2+2*4 {
		t.Fatalf("%d unit quads, want 12", r.lod.crops.quads)
	}
	a := r.lod.crops.atlas
	if a == nil || a.sets["corn"] == nil || a.sets["wheat"] == nil {
		t.Fatal("atlas lacks a species that drew")
	}
	if a.sets["pumpkin"] != nil {
		t.Fatal("atlas built a species nothing draws")
	}
}

func TestCropMarkColor(t *testing.T) {
	t.Parallel()
	mark := func(p game.PlantView) color.RGBA { p.Species = "corn"; c, _ := plantMarkColor(&p); return c }
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
	if _, k := plantMarkColor(&game.PlantView{Species: "corn", Growth: 100}); k <= 0 {
		t.Errorf("mark size %v", k)
	}
}

// TestPlantDamageLookOnOtherPlants: trees and bushes shrink, tint and
// fade with damage too; undamaged plants draw as before.
func TestPlantDamageLookOnOtherPlants(t *testing.T) {
	t.Parallel()
	cs, k := plantDamageLook(game.PlantView{Species: "tree"})
	if cs != noCS || k != 1 {
		t.Fatalf("undamaged: %v %v", cs, k)
	}
	cs, k = plantDamageLook(game.PlantView{Species: "tree", Damage: 30, DamageKind: "cold"})
	if cs == noCS || k != 1 {
		t.Fatalf("30%%: tint %v scale %v", cs, k)
	}
	cs, k = plantDamageLook(game.PlantView{Species: "berry_bush", Damage: 85, DamageKind: "heat"})
	if k >= 1 || cs[3] >= 1 {
		t.Fatalf("85%%: scale %v alpha %v", k, cs[3])
	}
}

func TestCropIsCropPlant(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		p    game.PlantView
		want bool
	}{
		{game.PlantView{Species: "corn"}, true},
		{game.PlantView{Species: "healroot", Zone: "healroot"}, true},
		{game.PlantView{Species: "healroot"}, false}, // wild
		{game.PlantView{Species: "tree"}, false},
		{game.PlantView{Species: "berry_bush"}, false},
	} {
		if got := isCropPlant(&c.p); got != c.want {
			t.Errorf("%+v: %v, want %v", c.p, got, c.want)
		}
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
			if root, err := LocateArt(); err == nil {
				if cat, err := loadArtCatalog(root); err == nil && cat.has("plant/corn_mature_v0") {
					r.art = cat
				}
			}
			r.cam.zoom = c.tile / TileSize
			screen := ebiten.NewImage(c.sw, c.sh)
			frame := func() {
				r.present++
				r.setView(c.sw, c.sh, c.tile)
				r.view.W = w
				r.stepEnvironment(w, nil)
				r.lod.crops.begin()
				r.drawFurrows(screen, w, c.tile)
				for i := range w.Plants {
					if p := &w.Plants[i]; r.lod.inView(p.X, p.Y, 0, 0) {
						r.drawCropObj(screen, &w, p, c.tile)
					}
				}
				r.flushCrops(screen)
			}
			for i := 0; i < 3; i++ {
				frame()
			}
			if r.lod.crops.furrows < c.wantInView {
				b.Fatalf("%d crops in view, want at least %d", r.lod.crops.furrows, c.wantInView)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame()
			}
			b.ReportMetric(float64(r.lod.crops.quads), "unit-quads/op")
			b.ReportMetric(float64(r.lod.crops.furrows), "furrows/op")
		})
	}
}

// PLAY-016: a dying plant's shadow shrinks with its units and is gone when
// they are.
func TestDamageShadowFactor(t *testing.T) {
	t.Parallel()
	corn := game.PlantView{Species: "corn", Stage: 2}
	if damageShadowFactor(corn) != 1 {
		t.Fatal("healthy: full shadow")
	}
	prev := 1.0
	for _, d := range []int{30, 60, 80, 90, 99} {
		corn.Damage = d
		f := damageShadowFactor(corn)
		if f > prev || f < 0 {
			t.Fatalf("damage %d: factor %v after %v", d, f, prev)
		}
		prev = f
	}
	corn.Damage = 100
	if damageShadowFactor(corn) != 0 {
		t.Fatal("dead: no shadow")
	}
	if f := damageShadowFactor(game.PlantView{Species: "tree", Damage: 85}); f <= 0 || f >= 1 {
		t.Fatalf("a dying tree's shadow: %v", f)
	}
}
