package ebiten

import (
	"os"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	appscenario "rimworld-go/internal/application/scenario"
)

// BenchmarkDrawArt is one whole world frame (terrain, shadows, buildings,
// plants, items, animals and pawns) of a generated colony with the real
// art, at a zoom that shows most of the map. It measures the CPU side of
// the draw: building and queueing draw calls (the GPU is not flushed in
// tests). ART-010 gate: at most +35% over the pre-lighting baseline.
func BenchmarkDrawArt(b *testing.B) {
	benchDraw(b, func(*game.Game) {})
}

// BenchmarkDrawArtMountain is BenchmarkDrawArt on a map that is about 60%
// natural rock in five stones, with ore veins and smoothed rock: the
// linked rock path (linkgrid.go) with its corners and tints.
func BenchmarkDrawArtMountain(b *testing.B) {
	benchDraw(b, func(g *game.Game) {
		stones := []string{"sandstone", "granite", "limestone", "slate", "marble"}
		for i, s := range stones { // five bands, 48 of 80 columns
			g.SetGround(0, i*10, 47, i*10+9, "rock", s, "")
		}
		g.SetGround(10, 12, 13, 15, "rock", "granite", "steel")
		g.SetGround(30, 32, 31, 34, "rock", "slate", "gold")
		g.SetGround(20, 5, 27, 8, "rock_smooth", "sandstone", "")
		g.SetGround(48, 20, 52, 24, "stone_smooth", "granite", "")
	})
}

func benchDraw(b *testing.B, setup func(*game.Game)) {
	root, err := LocateArt()
	if err != nil {
		b.Skip("generated art not present:", err)
	}
	art, err := loadArtCatalog(root)
	if err != nil {
		b.Fatal(err)
	}
	g := game.NewColony(game.Config{Seed: 7, Width: 80, Height: 50})
	setup(g)
	g.Step(600)
	r := New(g)
	r.art = art
	r.cam.Zoom = 0.6
	screen := ebiten.NewImage(1280, 800)
	tile := r.tilePx()
	frame := func() {
		r.present++
		r.advance()
		r.setView(1280, 800, tile)
		r.drawArt(screen, 1280, 800, tile)
	}
	for i := 0; i < 5; i++ { // textures uploaded, caches warm
		frame()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame()
	}
}

// BenchmarkDrawArt500 is one world frame of the grown 500² colony
// (internal/application/scenario; set RIMWORLD_FIXTURE_DIR to keep the
// fixture between runs) at each zoom the LOD tiers cover: z1 (64 px
// tiles, live), z0.5 and z0.25 (32 and 16 px, baked chunks) and zmin (the
// furthest zoom-out, the whole-map bake). The sim is paused, so only the
// draw is timed; draw-calls/op counts lit sprites, water tiles and baked
// pages (light.go litRenderer.calls). NG-1 §6.1: ≤ 6 ms and ≤ 6 000 draw
// calls at every zoom.
func BenchmarkDrawArt500(b *testing.B) {
	benchDrawGrown(b, 500)
}

// BenchmarkDrawArt250 is BenchmarkDrawArt500 at 250² (the §6.9 gate for
// standard sizes).
func BenchmarkDrawArt250(b *testing.B) {
	benchDrawGrown(b, 250)
}

func benchDrawGrown(b *testing.B, side int) {
	root, err := LocateArt()
	if err != nil {
		b.Skip("generated art not present:", err)
	}
	art, err := loadArtCatalog(root)
	if err != nil {
		b.Fatal(err)
	}
	art.PrewarmAtlases() // page cuts are a one-time cost, not the frame's
	if d := os.Getenv("RIMWORLD_FIXTURE_DIR"); d != "" {
		appscenario.CacheDir = d
	}
	data := appscenario.GrownColony(1, side, side, false)
	for _, z := range []struct {
		name string
		zoom float64 // 0: the furthest zoom-out
		run  bool    // the sim runs at normal speed (its tick is not timed): bakes follow the changes
	}{{"z1", 1, false}, {"z0.5", 0.5, false}, {"z0.25", 0.25, false}, {"zmin", 0, false},
		{"z0.25-run", 0.25, true}, {"zmin-run", 0, true}} {
		b.Run(z.name, func(b *testing.B) {
			g, err := appscenario.Load(data)
			if err != nil {
				b.Fatal(err)
			}
			r := New(g)
			r.art = art
			r.speed = speedPaused
			if z.run {
				r.speed = speedNormal
			}
			const sw, sh = 1280, 800
			v := colonyViewport(sw, sh)
			r.cam.Zoom = z.zoom
			if z.zoom == 0 {
				r.cam.Zoom = r.MinZoomFor(v)
			}
			r.ClampCamera(v) // centred on the colony
			screen := ebiten.NewImage(sw, sh)
			tile := r.tilePx()
			frame := func() {
				r.advance()
				r.setView(sw, sh, tile)
				r.drawArt(screen, sw, sh, tile)
			}
			for i := 0; i < 60; i++ { // textures uploaded, atlas pages cut, bakes made
				frame()
			}
			calls := r.lit.Calls
			snowBakes, snowDraws := r.snowc.Bakes, r.snowc.Draws // snow_cache.go (PERF-002): 0 bakes/op once warm
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				r.advance()
				b.StartTimer()
				r.setView(sw, sh, tile)
				r.drawArt(screen, sw, sh, tile)
			}
			b.ReportMetric(float64(r.lit.Calls-calls)/float64(b.N), "draw-calls/op")
			b.ReportMetric(float64(r.snowc.Bakes-snowBakes)/float64(b.N), "snow-bakes/op")
			b.ReportMetric(float64(r.snowc.Draws-snowDraws)/float64(b.N), "snow-draws/op")
		})
	}
}
