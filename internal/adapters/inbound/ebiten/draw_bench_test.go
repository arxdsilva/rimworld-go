package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// BenchmarkDrawArt is one whole world frame (terrain, shadows, buildings,
// plants, items, animals and pawns) of a generated colony with the real
// art, at a zoom that shows most of the map. It measures the CPU side of
// the draw: building and queueing draw calls (the GPU is not flushed in
// tests). ART-010 gate: at most +35% over the pre-lighting baseline.
func BenchmarkDrawArt(b *testing.B) {
	root, err := LocateArt()
	if err != nil {
		b.Skip("generated art not present:", err)
	}
	art, err := loadArtCatalog(root)
	if err != nil {
		b.Fatal(err)
	}
	g := game.NewColony(game.Config{Seed: 7, Width: 80, Height: 50})
	g.Step(600)
	r := New(g)
	r.art = art
	r.cam.zoom = 0.6
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
