package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// grassTestArt is a catalog with soil tiles and the PLAY-010 grass tuft
// keys, each tuft a small opaque block standing on the canvas bottom
// third (so the atlas finds a real alpha box).
func grassTestArt() *artCatalog {
	c := envArt("terrain/soil_00", "terrain/soil_01", "terrain/soil_02", "terrain/soil_03", "terrain/stone_00")
	tuft := func(h int) *artSprite {
		pix := image.NewNRGBA(image.Rect(0, 0, 40, 46))
		for y := 30 - h; y < 30; y++ {
			for x := 14; x < 26; x++ {
				pix.SetNRGBA(x, y, color.NRGBA{112, 128, 82, 255})
			}
		}
		return &artSprite{meta: artMeta{Canvas: [2]int{160, 184}, Origin: [2]float64{16, 40}}, pix: pix}
	}
	for v := 0; v < 6; v++ {
		c.sprites[fmt.Sprintf("plant/grass_short_v%d", v)] = tuft(10)
	}
	for v := 0; v < 4; v++ {
		c.sprites[fmt.Sprintf("plant/grass_tall_v%d", v)] = tuft(16)
	}
	for v := 0; v < 3; v++ {
		c.sprites[fmt.Sprintf("plant/grass_short_dormant_v%d", v)] = tuft(9)
	}
	return c
}

// grassWorld is a w×h soil world, grass on every tile at growth g
// (GrassGrowth empty when g is 100: the read model's "all full").
func grassWorld(w, h int, g uint8) game.WorldView {
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), Grass: make([]uint8, w*h), StructVer: 1, GrassVer: 1}
	for i := range v.Grass {
		v.Grass[i] = 1
	}
	if g < 100 {
		v.GrassGrowth = make([]uint8, w*h)
		for i := range v.GrassGrowth {
			v.GrassGrowth[i] = g
		}
	}
	return v
}

func grassRenderer(w game.WorldView, mw, mh int) (*Renderer, *stubSim) {
	sim := &stubSim{w: mw, h: mh, world: w}
	r := New(sim)
	r.art = grassTestArt()
	r.speed = speedPaused
	return r, sim
}

// batchAt draws the tuft batch over tiles 0..mw-1 × 0..mh-1 at tile px
// per tile on a screen showing the whole map with a one-tile margin (so
// no copy is culled), and returns its quads.
func batchAt(r *Renderer, mw, mh int, tile float64, limit int) int {
	r.cam.zoom, r.cam.offsetX, r.cam.offsetY = tile/TileSize, -tile, -tile
	sw, sh := int(float64(mw+2)*tile), int(float64(mh+2)*tile)
	r.setView(sw, sh, tile)
	w := r.game.World()
	r.view.W = w
	r.stepEnvironment(w, nil)
	r.drawGrassBatch(ebiten.NewImage(max(sw, 1), max(sh, 1)), &w, 0, 0, mw-1, mh-1, tile, sw, sh, limit)
	return r.lod.grass.quads
}

func TestTuftCopiesFollowGrowth(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		g    uint8
		want int
	}{{0, 1}, {24, 1}, {25, 2}, {49, 2}, {50, 4}, {74, 4}, {75, 6}, {89, 6}, {90, 9}, {100, 9}} {
		if got := tuftCopies(c.g); got != c.want {
			t.Fatalf("growth %d: %d copies, want %d", c.g, got, c.want)
		}
		if n := len(grassCopies(3, 4, c.g, 9, false, nil)); n != c.want {
			t.Fatalf("growth %d: grassCopies made %d, want %d", c.g, n, c.want)
		}
		if n := len(grassCopies(3, 4, c.g, 4, false, nil)); n != min(c.want, 4) {
			t.Fatalf("growth %d capped at 4: %d copies", c.g, n)
		}
	}
}

// TestGrassCopiesSpillPastEdge: copies sit in a jittered 3×3 grid; edge
// copies reach past the tile edge (so neighbours interleave) but never
// more than grassEdgeSpill.
func TestGrassCopiesSpillPastEdge(t *testing.T) {
	t.Parallel()
	var maxSpill float64
	var buf []tuftCopy
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			buf = grassCopies(x, y, 100, 9, x%3 == 0, buf[:0])
			for _, c := range buf {
				spill := max(-c.dx, c.dx-1, -c.dy, c.dy-1)
				if spill > grassEdgeSpill+1e-9 {
					t.Fatalf("tile (%d,%d): copy at (%.3f,%.3f) spills %.3f > %.2f", x, y, c.dx, c.dy, spill, grassEdgeSpill)
				}
				maxSpill = max(maxSpill, spill)
			}
		}
	}
	if maxSpill < 0.1 {
		t.Fatalf("copies never spill past the edge (max %.3f)", maxSpill)
	}
	// the centre copy stays within ±0.18 of the tile centre
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			c := grassCopies(x, y, 10, 9, false, nil)[0] // one copy: the centre cell
			if math.Abs(c.dx-0.5) > grassCellJitter+1e-9 || math.Abs(c.dy-0.5) > grassCellJitter+1e-9 {
				t.Fatalf("centre copy at (%.3f,%.3f)", c.dx, c.dy)
			}
		}
	}
}

// TestGrassCopiesDeterministic: a tile's copies (mirror, variant, place)
// are a pure function of the tile; tiles differ.
func TestGrassCopiesDeterministic(t *testing.T) {
	t.Parallel()
	mirrored, plain := 0, 0
	variants := map[uint32]bool{}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			a := grassCopies(x, y, 100, 9, false, nil)
			b := grassCopies(x, y, 100, 9, false, nil)
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("tile (%d,%d) copy %d differs: %+v vs %+v", x, y, i, a[i], b[i])
				}
				if a[i].mirror {
					mirrored++
				} else {
					plain++
				}
				variants[a[i].variant%6] = true
			}
		}
	}
	if mirrored < 300 || plain < 300 {
		t.Fatalf("mirroring not mixed: %d mirrored, %d plain", mirrored, plain)
	}
	if len(variants) != 6 {
		t.Fatalf("variants used %v", variants)
	}
}

// TestGrassScaleRanges: short copies scale 0.45–0.65, tall 0.7–0.9 at full
// growth; tall patches cluster (valueNoise) and rich soil is tall.
func TestGrassScaleRanges(t *testing.T) {
	t.Parallel()
	for y := 0; y < 30; y++ {
		for x := 0; x < 30; x++ {
			for _, tall := range []bool{false, true} {
				for _, c := range grassCopies(x, y, 100, 9, tall, nil) {
					lo, hi := 0.45, 0.65
					if c.tall {
						lo, hi = 0.7, 0.9
					}
					if c.scale < lo-1e-9 || c.scale > hi+1e-9 {
						t.Fatalf("tall=%v copy scale %.3f outside %.2f–%.2f", c.tall, c.scale, lo, hi)
					}
					if c.tall && !tall {
						t.Fatal("tall copy on a short tile")
					}
				}
			}
		}
	}
	soil, rich := terrainCodeOf("soil"), terrainCodeOf("rich_soil")
	if !grassTallAt(rich, 3, 3) {
		t.Fatal("rich soil is not tall")
	}
	same, pairs, tall := 0, 0, 0
	for y := 0; y < 120; y++ {
		for x := 0; x < 120; x++ {
			a := grassTallAt(soil, x, y)
			if a {
				tall++
			}
			if grassTallAt(soil, x+1, y) == a {
				same++
			}
			pairs++
		}
	}
	if tall == 0 || tall > pairs/2 {
		t.Fatalf("%d of %d soil tiles tall", tall, pairs)
	}
	if float64(same)/float64(pairs) < 0.85 {
		t.Fatalf("tall patches checkerboard: neighbours agree %.2f", float64(same)/float64(pairs))
	}
}

func terrainCodeOf(name string) uint8 {
	for i, n := range terrainNames {
		if n == name {
			return uint8(i)
		}
	}
	panic(name)
}

// TestGrassBatchIsOneDraw: every tuft copy in view is one triangle batch,
// drawn after all the ground and before the objects.
func TestGrassBatchIsOneDraw(t *testing.T) {
	t.Parallel()
	w := grassWorld(24, 16, 100)
	w.Plants = []game.PlantView{{ID: 9, Species: "bush", X: 20, Y: 12, Growth: 100}}
	r, _ := grassRenderer(w, 24, 16)
	r.art.sprites["plant/bush_mature_v0"] = fakeSprite([2]int{128, 128}, pt{0, 0}, nil)
	var order []string
	ground := map[*ebiten.Image]bool{}
	for v := 0; v < 4; v++ {
		ground[r.art.image(artKey("terrain", "soil", v), color.NRGBA{})] = true
	}
	r.lit.onDraw = func(img *ebiten.Image, _ float32) {
		if ground[img] {
			order = append(order, "ground")
		} else {
			order = append(order, "object")
		}
	}
	r.lod.grass.onFlush = func() { order = append(order, "grass") }
	lodFrame(r, 64) // 640×400: 10×6.25 tiles
	if r.lod.grass.flushes != 1 {
		t.Fatalf("tufts drawn in %d batches, want 1", r.lod.grass.flushes)
	}
	if r.lod.grass.quads < 10*6*9 {
		t.Fatalf("batch holds %d quads, want every copy of the view (≥ %d)", r.lod.grass.quads, 10*6*9)
	}
	at := -1
	for i, n := range order {
		switch {
		case n == "grass":
			at = i
		case n == "ground" && at >= 0:
			t.Fatalf("ground drawn after the tuft batch: %v", order)
		}
	}
	if at < 0 {
		t.Fatal("no tuft batch drawn")
	}
}

// TestNoGrassUnderFloors: covered tiles (floors, buildings, frames,
// plants) draw no tufts.
func TestNoGrassUnderFloors(t *testing.T) {
	t.Parallel()
	open := grassWorld(10, 10, 100)
	r, _ := grassRenderer(open, 10, 10)
	full := batchAt(r, 10, 10, 64, 9)
	if full != 100*9 {
		t.Fatalf("open field: %d quads, want 900", full)
	}
	cov := grassWorld(10, 10, 100)
	cov.Buildings = []game.BuildingView{{ID: 1, Kind: "floor", Floor: true, X: 4, Y: 4, W: 1, H: 1, Stuff: "wood"}}
	cov.Frames = []game.FrameView{{X: 6, Y: 6}}
	cov.Plants = []game.PlantView{{ID: 2, Species: "bush", X: 2, Y: 2, Growth: 100}}
	r2, _ := grassRenderer(cov, 10, 10)
	if got := batchAt(r2, 10, 10, 64, 9); got != full-3*9 {
		t.Fatalf("covered: %d quads, want %d", got, full-3*9)
	}
}

// TestGrassSnowFade: deep snow fades outdoor tufts (vertex alpha); no
// snow leaves them opaque.
func TestGrassSnowFade(t *testing.T) {
	t.Parallel()
	w := grassWorld(8, 8, 100)
	r, sim := grassRenderer(w, 8, 8)
	batchAt(r, 8, 8, 64, 9)
	for _, v := range r.lod.grass.verts {
		if v.ColorA != 1 {
			t.Fatalf("no snow: vertex alpha %v", v.ColorA)
		}
	}
	sim.world.SnowDepth = 0.55
	r2 := New(sim)
	r2.art = grassTestArt()
	n := batchAt(r2, 8, 8, 64, 9)
	faded := 0
	for _, v := range r2.lod.grass.verts {
		if v.ColorA < 1 {
			faded++
		}
	}
	if faded == 0 || n == 0 {
		t.Fatalf("snow 0.55: %d faded vertices of %d quads", faded, n)
	}
	sim.world.SnowDepth = 1
	r3 := New(sim)
	r3.art = grassTestArt()
	if n := batchAt(r3, 8, 8, 64, 9); n != 0 {
		t.Fatalf("deep snow still shows %d tuft quads", n)
	}
}

// TestGrassLODTiers: ≥ 48 px all copies live, 24–48 px at most four per
// tile live, 16–24 px tufts baked into the chunk images, under 16 px a
// grass tint in the map bake.
func TestGrassLODTiers(t *testing.T) {
	t.Parallel()
	w := grassWorld(64, 64, 100)
	r, _ := grassRenderer(w, 64, 64)
	lodFrame(r, 64)
	if r.lod.grass.quads < 10*6*9 { // 640×400 shows 10×6 whole tiles
		t.Fatalf("64 px: %d quads, want nine copies a tile (≥ %d)", r.lod.grass.quads, 10*6*9)
	}
	lodFrame(r, 32)
	tiles := (640/32 + 2) * (400/32 + 3)
	if q := r.lod.grass.quads; q == 0 || q > 4*tiles {
		t.Fatalf("32 px: %d quads, want 1..%d (four a tile)", q, 4*tiles)
	}
	baked := r.lod.grass.baked
	lodFrame(r, 20)
	if r.lod.grass.quads != 0 {
		t.Fatalf("20 px drew %d live tuft quads, want them baked", r.lod.grass.quads)
	}
	if r.lod.grass.baked == baked {
		t.Fatal("20 px: no tufts baked into the chunk images")
	}
	tints := r.lod.grass.tinted
	lodFrame(r, 8)
	if r.lod.grass.quads != 0 || r.lod.grass.tinted == tints {
		t.Fatalf("8 px: %d live quads, %d tint bakes", r.lod.grass.quads, r.lod.grass.tinted-tints)
	}
}

// TestChunkVersionFollowsGrassGrowth: in the baked-tuft tier a growth
// change rebakes only its chunk; the plain chunk tier ignores grass.
func TestChunkVersionFollowsGrassGrowth(t *testing.T) {
	t.Parallel()
	w := grassWorld(64, 64, 100)
	r, sim := grassRenderer(w, 64, 64)
	lodFrame(r, 20)
	first := r.lod.chunks.bakes
	if first == 0 {
		t.Fatal("no chunk baked")
	}
	lodFrame(r, 20)
	if r.lod.chunks.bakes != first {
		t.Fatalf("an unchanged frame rebaked %d chunks", r.lod.chunks.bakes-first)
	}
	g := make([]uint8, 64*64)
	for i := range g {
		g[i] = 100
	}
	g[8*64+8] = 10 // shoots in chunk (0,0)
	sim.world.GrassGrowth, sim.world.GrassVer = g, 2
	lodFrame(r, 20)
	if got := r.lod.chunks.bakes - first; got != 1 {
		t.Fatalf("one regrown tile rebaked %d chunks, want 1", got)
	}
	// the 24–48 px tier bakes no tufts: growth never rebakes it
	r2, sim2 := grassRenderer(grassWorld(64, 64, 100), 64, 64)
	lodFrame(r2, 32)
	b := r2.lod.chunks.bakes
	sim2.world.GrassGrowth, sim2.world.GrassVer = g, 2
	lodFrame(r2, 32)
	if r2.lod.chunks.bakes != b {
		t.Fatalf("32 px rebaked %d chunks on a growth change", r2.lod.chunks.bakes-b)
	}
}

// BenchmarkGrassBatch is the tuft batch alone for a full 1280×800 screen
// of fully grown grass at z1 (64 px tiles), with the real art when it is
// present. PLAY-010 budget: ≤ 1.0 ms.
func BenchmarkGrassBatch(b *testing.B) {
	art := grassTestArt()
	if root, err := LocateArt(); err == nil {
		if c, err := loadArtCatalog(root); err == nil {
			art = c
		}
	}
	const mw, mh, sw, sh = 40, 30, 1280, 800
	w := grassWorld(mw, mh, 100)
	w.WindX, w.WindY, w.Weather = 1.2, 0.3, "cloudy"
	sim := &stubSim{w: mw, h: mh, world: w}
	r := New(sim)
	r.art = art
	r.cam.zoom = 1
	screen := ebiten.NewImage(sw, sh)
	frame := func() {
		r.present++
		r.setView(sw, sh, 64)
		r.view.W = w
		r.stepEnvironment(w, nil)
		x0, y0, x1, y1 := r.visibleTiles()
		r.drawGrassBatch(screen, &w, x0, y0, x1, y1, 64, sw, sh, 9)
	}
	for i := 0; i < 3; i++ {
		frame()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame()
	}
	b.ReportMetric(float64(r.lod.grass.quads), "quads/op")
	b.ReportMetric(1, "draw-calls/op")
}
