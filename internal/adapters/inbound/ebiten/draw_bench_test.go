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
