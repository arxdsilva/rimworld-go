package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
)

// PLAY-019: the renderer's graphics knobs (shadows, lighting, water, grass
// detail) and the bound on the lit-pair cache.

// --- shadows ---------------------------------------------------------------------

func TestShadowsOffSkipsTheLayer(t *testing.T) {
	t.Parallel()
	w := shadowWorld(nil)
	w.Buildings = []game.BuildingView{{Kind: "wall", X: 2, Y: 2, W: 1, H: 1}}
	r := shadowRenderer(t, w)
	r.lit.Light = testLight(0.4, 0.4, 0.3)
	dst := ebiten.NewImage(640, 400)
	r.drawShadows(dst, w, 640, 400, r.tilePx())
	if r.env.Shadow.Composites != 1 || r.env.Shadow.Layer == nil {
		t.Fatalf("fixture: composites %d, layer %v; the shadow layer should draw with shadows on", r.env.Shadow.Composites, r.env.Shadow.Layer)
	}
	r.env.Shadow.SetOff(true)
	if r.env.Shadow.Layer != nil || r.env.Shadow.Scratch != nil {
		t.Fatal("turning shadows off should free the screen-sized layers")
	}
	for i := 0; i < 3; i++ {
		r.drawShadows(dst, w, 640, 400, r.tilePx())
	}
	if r.env.Shadow.Composites != 1 || r.env.Shadow.Layer != nil {
		t.Fatalf("shadows off still drew: composites %d, layer %v", r.env.Shadow.Composites, r.env.Shadow.Layer)
	}
	r.env.Shadow.SetOff(false)
	r.drawShadows(dst, w, 640, 400, r.tilePx())
	if r.env.Shadow.Composites != 2 {
		t.Fatalf("shadows back on: composites %d, want 2", r.env.Shadow.Composites)
	}
}

// --- lighting --------------------------------------------------------------------

// normalSprite is a sprite image with a normal map in art.
func normalSprite(art *artCatalog) *ebiten.Image {
	img, n := ebiten.NewImage(8, 6), ebiten.NewImage(8, 6)
	if art.Normals == nil {
		art.Normals = map[*ebiten.Image]*ebiten.Image{}
	}
	art.Normals[img] = n
	return img
}

func TestLightingOffDrawsPlainWithoutPairs(t *testing.T) {
	t.Parallel()
	art := &artCatalog{}
	sprite := normalSprite(art)
	r := &Renderer{art: art}
	r.view.W = roomWorld()
	r.lit.Light = lightAt(12, summerDay, "clear", 0)
	dst := ebiten.NewImage(64, 64)
	r.lit.Off = true
	for i := 0; i < 3; i++ {
		r.drawLit(dst, sprite, ebiten.GeoM{}, noCS, 0, 0)
	}
	if n := r.lit.Pairs.Size(); n != 0 {
		t.Fatalf("lighting off made %d lit pairs", n)
	}
	if r.lit.Opts.Images[0] != nil {
		t.Fatal("lighting off still drew through lit.kage")
	}
	if r.lit.Calls != 3 {
		t.Fatalf("draw calls = %d, want the 3 plain draws counted", r.lit.Calls)
	}
	r.lit.Off = false
	r.drawLit(dst, sprite, ebiten.GeoM{}, noCS, 0, 0)
	if n := r.lit.Pairs.Size(); n != 1 || r.lit.Opts.Images[0] == nil {
		t.Fatalf("lighting back on: %d pairs, shader image %v", n, r.lit.Opts.Images[0])
	}
}

// TestLightingOffBloodyFigureStillShades: blood on a creature is painted
// by lit.kage, so a bloody figure keeps the shader even with lighting off
// (flat, still no pair image).
func TestLightingOffBloodyFigureStillShades(t *testing.T) {
	t.Parallel()
	art := &artCatalog{}
	sprite := normalSprite(art)
	r := &Renderer{art: art}
	r.view.W = roomWorld()
	r.lit.Light = lightAt(12, summerDay, "clear", 0)
	r.lit.Off = true
	r.figureBlood = 1.5
	r.drawLit(ebiten.NewImage(64, 64), sprite, ebiten.GeoM{}, noCS, 0, 0)
	if r.lit.Opts.Images[0] != sprite || r.lit.Pairs.Size() != 0 {
		t.Fatalf("bloody figure: shader image is the sprite %v, %d pairs; want a flat shader draw", r.lit.Opts.Images[0] == sprite, r.lit.Pairs.Size())
	}
}

// TestLightingOffKeepsDaylightTint: the plain colour scale still follows
// the hour, the roof and the lamps, and the sprite's own colour scale.
func TestLightingOffKeepsDaylightTint(t *testing.T) {
	t.Parallel()
	noon, night := lightAt(12, summerDay, "clear", 0), lightAt(0, summerDay, "clear", 0)
	day := flatLitScale(&noon, noCS, 1, 0)
	dark := flatLitScale(&night, noCS, 1, 0)
	if math.Abs(float64(day[0])-1) > 0.15 || day[3] != 1 {
		t.Fatalf("noon scale %v, want about 1 (today's unlit look) at full alpha", day)
	}
	if !(dark[0] < day[0]/2) {
		t.Fatalf("midnight scale %v is not much darker than noon %v", dark, day)
	}
	if in := flatLitScale(&noon, noCS, 0, 0); !(in[0] < day[0]) {
		t.Fatalf("a roofed tile (%v) should be darker than open sky (%v)", in, day)
	}
	if lamp := flatLitScale(&night, noCS, 0, 1); !(lamp[0] > dark[0]) {
		t.Fatalf("a lit room at night (%v) should be brighter than the dark (%v)", lamp, dark)
	}
	half := flatLitScale(&noon, colorScale{0.5, 0.5, 0.5, 0.5}, 1, 0) // premultiplied like DrawImage's ColorScale
	if math.Abs(float64(half[0])-float64(day[0])*0.25) > 1e-5 || half[3] != 0.5 {
		t.Fatalf("scaled sprite %v, want rgb x0.25 (colour x alpha) and alpha 0.5", half)
	}
}

// TestLightingOffGrassKeepsTuftsLitByTheTileLight: tufts drawn with lighting
// off carry the daylight in their vertex colour and skip the shader.
func TestLightingOffGrassKeepsTuftsLitByTheTileLight(t *testing.T) {
	t.Parallel()
	w := grassWorld(6, 6, 100)
	r, _ := grassRenderer(w, 6, 6)
	r.lit.Light = lightAt(8, summerDay, "clear", 0)
	batchAt(r, 6, 6, 64, 9)
	on := r.lod.Grass.Verts[0]
	if r.lit.Opts.Images[0] == nil {
		t.Fatal("fixture: lit tufts should draw through the shader")
	}
	r.lit.Opts.Images[0] = nil
	r.lit.Off = true
	batchAt(r, 6, 6, 64, 9)
	off := r.lod.Grass.Verts[0]
	if r.lit.Opts.Images[0] != nil {
		t.Fatal("lighting off still drew tufts through lit.kage")
	}
	f := litFactor(&r.lit.Light, [3]float32{0, 0, 1}, 1, 0)
	for i, got := range [3]float32{off.ColorR, off.ColorG, off.ColorB} {
		want := [3]float32{on.ColorR, on.ColorG, on.ColorB}[i] * f[i]
		if math.Abs(float64(got-want)) > 1e-5 {
			t.Fatalf("channel %d: %v, want %v (tuft colour x the tile's flat light)", i, got, want)
		}
	}
	if off.ColorA != on.ColorA {
		t.Fatalf("alpha changed: %v -> %v", on.ColorA, off.ColorA)
	}
}

// --- the lit-pair cache ----------------------------------------------------------

func TestLitPairCacheEvictsLeastRecentlyUsed(t *testing.T) {
	t.Parallel()
	img := func() *ebiten.Image { return ebiten.NewImage(2, 2) }
	a, b, c, d := img(), img(), img(), img()
	pa, pb, pc, pd := img(), img(), img(), img()
	cache := litPairCache{Limit: 3}
	cache.Put(a, pa, 0)
	cache.Put(b, pb, 0)
	cache.Put(c, pc, 0)
	if got, ok := cache.Get(a, 0); !ok || got != pa { // a is now the most recent; b the oldest
		t.Fatalf("get(a) = %v %v", got, ok)
	}
	cache.Put(d, pd, 0)
	if cache.Size() != 3 || cache.Evicted != 1 {
		t.Fatalf("size %d evicted %d, want 3 and 1", cache.Size(), cache.Evicted)
	}
	if _, ok := cache.Get(b, 0); ok {
		t.Fatal("the least recently used pair (b) should be gone")
	}
	for _, k := range []struct {
		src, pair *ebiten.Image
		name      string
	}{{a, pa, "a"}, {c, pc, "c"}, {d, pd, "d"}} {
		if got, ok := cache.Get(k.src, 0); !ok || got != k.pair {
			t.Fatalf("%s lost: %v %v", k.name, got, ok)
		}
	}
}

func TestLitPairCapacityFollowsTheVisibleSprites(t *testing.T) {
	t.Parallel()
	if litPairCapFor(0) != litPairsMin || litPairCapFor(10) != litPairsMin {
		t.Fatalf("small scenes keep the floor: %d %d", litPairCapFor(0), litPairCapFor(10))
	}
	if got := litPairCapFor(400); got != 600 {
		t.Fatalf("cap for 400 visible = %d, want 600 (1.5x)", got)
	}
	if litPairCapFor(1<<20) != litPairsMax {
		t.Fatalf("cap for a huge scene = %d, want the ceiling %d", litPairCapFor(1<<20), litPairsMax)
	}
	// the cap follows last frame's distinct sprites
	var c litPairCache
	for i := 0; i < 400; i++ {
		c.Put(ebiten.NewImage(1, 1), ebiten.NewImage(1, 1), 1)
	}
	c.Put(ebiten.NewImage(1, 1), ebiten.NewImage(1, 1), 2) // a new frame: it now knows 400 were visible
	if c.Capacity() != 600 {
		t.Fatalf("capacity after a 400-sprite frame = %d, want 600", c.Capacity())
	}
}

// TestLitPairsStayBounded: drawing far more normal-mapped sprites than the
// cache holds never grows it past its bound, and an evicted sprite draws
// again (its pair is rebuilt).
func TestLitPairsStayBounded(t *testing.T) {
	t.Parallel()
	art := &artCatalog{}
	r := &Renderer{art: art}
	r.view.W = roomWorld()
	r.lit.Light = lightAt(12, summerDay, "clear", 0)
	dst := ebiten.NewImage(64, 64)
	n := litPairsMin + 60
	sprites := make([]*ebiten.Image, n)
	for i := range sprites {
		sprites[i] = normalSprite(art)
		r.drawLit(dst, sprites[i], ebiten.GeoM{}, noCS, 0, 0)
		if sz := r.lit.Pairs.Size(); sz > litPairsMin {
			t.Fatalf("after %d sprites the cache holds %d pairs, bound %d", i+1, sz, litPairsMin)
		}
	}
	if r.lit.Pairs.Evicted != n-litPairsMin {
		t.Fatalf("evicted %d, want %d", r.lit.Pairs.Evicted, n-litPairsMin)
	}
	r.drawLit(dst, sprites[0], ebiten.GeoM{}, noCS, 0, 0) // evicted: rebuilt, still bounded and correct
	if pair := r.lit.Opts.Images[0]; pair == sprites[0] || pair.Bounds().Dx() != 2*8+litGap {
		t.Fatalf("an evicted sprite should draw from a rebuilt pair, got %v", pair.Bounds())
	}
	if r.lit.Pairs.Size() > litPairsMin {
		t.Fatalf("cache grew to %d", r.lit.Pairs.Size())
	}
}

// --- water -----------------------------------------------------------------------

func waterRendererFixture() *Renderer {
	r := &Renderer{}
	r.view.W = roomWorld()
	r.lit.Light = lightAt(12, summerDay, "clear", 0)
	r.art = &artCatalog{Sprites: map[string]*artSprite{}}
	for v := 0; v < soilVariants; v++ {
		img := ebiten.NewImage(16, 16)
		img.Fill(color.NRGBA{40, 90, 160, 255})
		r.art.Sprites[artKey("terrain", "shallow_water", v)] = &artSprite{Meta: artMeta{Canvas: [2]int{16, 16}}, Img: img}
	}
	r.setView(64, 64, 32)
	return r
}

func TestSimpleWaterIsStillAndSkipsTheRippleShader(t *testing.T) {
	t.Parallel()
	r := waterRendererFixture()
	r.water.Simple = true
	dst := ebiten.NewImage(64, 64)
	keys := map[string]bool{}
	shaderDraws := 0
	r.water.OnDraw = func(key string, shader bool) {
		keys[key] = true
		if shader {
			shaderDraws++
		}
	}
	for p := uint64(0); p < 4*waterFrameTicks; p += waterFrameTicks / 2 {
		r.present = p
		r.drawWaterTile(dst, "shallow_water", 0, 0, 32, 64, 64, noCS)
	}
	if shaderDraws != 0 || r.water.Shader != nil || r.water.Err != nil {
		t.Fatalf("simple water used the ripple shader: %d draws, compiled %v err %v", shaderDraws, r.water.Shader != nil, r.water.Err)
	}
	if len(keys) != 1 {
		t.Fatalf("simple water showed %d frames, want one still frame", len(keys))
	}
	if r.waterClock() != 0 {
		t.Fatal("simple water has a still clock")
	}
	if cs := r.foamCS(2, 3); cs != (colorScale{1, 1, 1, 0.85}) {
		t.Fatalf("foam %v, want the steady foam of reduced motion", cs)
	}
	r.present = 77
	if cs := r.foamCS(2, 3); cs != (colorScale{1, 1, 1, 0.85}) {
		t.Fatalf("foam moved at another frame: %v", cs)
	}
}

func TestAnimatedWaterStillRipples(t *testing.T) {
	t.Parallel()
	r := waterRendererFixture()
	dst := ebiten.NewImage(64, 64)
	shaderDraws := 0
	r.water.OnDraw = func(_ string, shader bool) {
		if shader {
			shaderDraws++
		}
	}
	r.present = 90
	r.drawWaterTile(dst, "shallow_water", 0, 0, 32, 64, 64, noCS)
	if shaderDraws != 1 || r.waterClock() != float32(90)/60 {
		t.Fatalf("animated water: %d shader draws, clock %v", shaderDraws, r.waterClock())
	}
}

// --- grass detail ----------------------------------------------------------------

func TestGrassDetailThinsTuftsPerTile(t *testing.T) {
	t.Parallel()
	w := grassWorld(16, 16, 100)
	r, _ := grassRenderer(w, 16, 16)
	colours := map[string]map[[4]float32]bool{}
	quads := func(level string) int {
		r.lod.Grass.Level = level
		n := batchAt(r, 16, 16, 64, 9)
		colours[level] = distinctTuftColours(r)
		return n
	}
	full := quads(config.GrassFull)
	if full != 16*16*9 {
		t.Fatalf("full grass draws %d tufts, want every copy (%d)", full, 16*16*9)
	}
	reduced, sparse := quads(config.GrassReduced), quads(config.GrassSparse)
	if f := float64(reduced) / float64(full); f < 0.5 || f > 0.7 {
		t.Fatalf("reduced keeps %.2f of the tufts (%d of %d), want about 0.6", f, reduced, full)
	}
	if f := float64(sparse) / float64(full); f < 0.2 || f > 0.4 {
		t.Fatalf("sparse keeps %.2f of the tufts (%d of %d), want about 0.3", f, sparse, full)
	}
	if !(sparse < reduced && reduced < full) {
		t.Fatalf("tufts %d (sparse) < %d (reduced) < %d (full) expected", sparse, reduced, full)
	}
	// the tile look's colour never changes, only how many tufts
	for _, level := range []string{config.GrassReduced, config.GrassSparse} {
		for c := range colours[level] {
			if !colours[config.GrassFull][c] {
				t.Fatalf("%s grass introduced the colour %v", level, c)
			}
		}
	}
	// deterministic: the same frame thins the same tufts
	if again := quads(config.GrassSparse); again != sparse {
		t.Fatalf("sparse drew %d then %d tufts", sparse, again)
	}
	// no level is full grass
	if got := quads(""); got != full {
		t.Fatalf("no level draws %d tufts, want %d", got, full)
	}
}

// distinctTuftColours is the set of vertex colours of the last tuft batch.
func distinctTuftColours(r *Renderer) map[[4]float32]bool {
	out := map[[4]float32]bool{}
	for _, v := range r.lod.Grass.Verts {
		out[[4]float32{v.ColorR, v.ColorG, v.ColorB, v.ColorA}] = true
	}
	return out
}

func TestGrassKeepIsDeterministicAndNested(t *testing.T) {
	t.Parallel()
	for n := 1; n <= 9; n++ {
		for y := 0; y < 24; y++ {
			for x := 0; x < 24; x++ {
				full := grassKeep(config.GrassFull, x, y, n)
				red := grassKeep(config.GrassReduced, x, y, n)
				sp := grassKeep(config.GrassSparse, x, y, n)
				if full != n || !(sp <= red && red <= full) {
					t.Fatalf("tile (%d,%d) of %d copies keeps %d/%d/%d (sparse/reduced/full)", x, y, n, sp, red, full)
				}
				if red != grassKeep(config.GrassReduced, x, y, n) {
					t.Fatalf("tile (%d,%d) is not deterministic", x, y)
				}
			}
		}
	}
	// a single tuft survives on some tiles and not others: the thinning is per tile
	kept := 0
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			kept += grassKeep(config.GrassSparse, x, y, 1)
		}
	}
	if kept < 32*32/5 || kept > 32*32*2/5 {
		t.Fatalf("sparse keeps %d of %d single-tuft tiles, want about 30%%", kept, 32*32)
	}
}

// TestGrassDetailChangesTheChunkBakeKey: baked chunk images hold tufts, so
// they rebake when the detail changes.
func TestGrassDetailChangesTheChunkBakeKey(t *testing.T) {
	t.Parallel()
	w := grassWorld(8, 8, 100)
	r, _ := grassRenderer(w, 8, 8)
	sigs := map[uint64]bool{}
	for _, level := range []string{config.GrassFull, config.GrassReduced, config.GrassSparse} {
		r.lod.Grass.Level = level
		sigs[r.grassLookSig(&w, 1)] = true
	}
	if len(sigs) != 3 {
		t.Fatalf("the grass look signature is the same for different detail levels: %v", sigs)
	}
}

// --- every knob through the settings ---------------------------------------------

func TestGraphicsPresetsDriveEveryKnob(t *testing.T) {
	t.Parallel()
	for _, tier := range config.QualityLevels {
		t.Run(tier, func(t *testing.T) {
			t.Parallel()
			g := config.Defaults().Graphics
			g.ApplyTier(tier, 1)
			r := New(&stubSim{w: 4, h: 4})
			applyGraphicsKnobs(r, g)
			want := struct {
				shadowsOff, lightingOff, simpleWater bool
				grass                                string
			}{!g.Shadows, !g.Lighting, g.Water == config.WaterSimple, g.Grass}
			got := struct {
				shadowsOff, lightingOff, simpleWater bool
				grass                                string
			}{r.env.Shadow.Off, r.lit.Off, r.water.Simple, r.lod.Grass.Level}
			if got != want {
				t.Fatalf("%s: knobs %+v, want %+v", tier, got, want)
			}
			if tier == config.QualityLow && (!got.shadowsOff || !got.lightingOff || !got.simpleWater || got.grass != config.GrassSparse) {
				t.Fatalf("low should switch every knob down: %+v (%s)", got, fmt.Sprint(g))
			}
			// PERF-002: the map's render scale, the App's knob (world_scale.go):
			// High full, Medium 3/4 on a 2x screen, Low full (HiDPI off)
			for _, dev := range []float64{1, 2} {
				st := config.Defaults()
				st.Graphics.ApplyTier(tier, dev)
				want := 1.0
				if tier == config.QualityMedium && dev == 2 {
					want = 0.75
				}
				if got := computeUIScale(st, 1280, 720, dev, false).WorldK(); got != want {
					t.Fatalf("%s at device scale %g: world %g, want %g", tier, dev, got, want)
				}
			}
		})
	}
}
