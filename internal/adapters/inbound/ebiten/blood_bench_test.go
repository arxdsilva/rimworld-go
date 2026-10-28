package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// stainSim is a colony whose world view carries hand-built stains, for
// stain benchmarks before (and independent of) the sim's own.
type stainSim struct {
	*game.Game
	stains []game.StainView
}

func (s *stainSim) World() game.WorldView {
	w := s.Game.World()
	w.Stains = append(w.Stains, s.stains...)
	return w
}

// benchStains is n stains spread over a w×h map, every shape, kind and
// age.
func benchStains(n, w, h int) []game.StainView {
	out := make([]game.StainView, n)
	for i := range out {
		kind := "blood"
		if i%5 == 0 {
			kind = "ichor"
		}
		out[i] = game.StainView{X: (i * 7) % w, Y: (i * 13) % h, Kind: kind,
			Amount: 0.05 + float64(i%20)/20, AgeTicks: uint64(i) * 200, Variant: uint8(i % 6)}
	}
	return out
}

// stainBench is a 80×50 colony renderer with the real art and 500
// stains, at the zoom of BenchmarkDrawArt.
func stainBench(b *testing.B) (*Renderer, *ebiten.Image) {
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
	r := New(&stainSim{Game: g, stains: benchStains(500, 80, 50)})
	r.art = art
	r.cam.zoom = 0.6
	return r, ebiten.NewImage(1280, 800)
}

// BenchmarkDrawStains is the stain pass alone with 500 stains (C1 gate:
// at most 0.3 ms).
func BenchmarkDrawStains(b *testing.B) {
	r, screen := stainBench(b)
	tile := r.tilePx()
	r.setView(1280, 800, tile)
	r.view.W = r.game.World()
	r.lit.light = lightAt(12, summerDay, "clear", 0)
	for i := 0; i < 5; i++ {
		r.drawStains(screen, r.view.W, 1280, 800, tile)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.drawStains(screen, r.view.W, 1280, 800, tile)
	}
}

// BenchmarkDrawArtStains is BenchmarkDrawArt's whole frame with 500
// stains on the ground.
func BenchmarkDrawArtStains(b *testing.B) {
	r, screen := stainBench(b)
	tile := r.tilePx()
	frame := func() {
		r.present++
		r.advance()
		r.setView(1280, 800, tile)
		r.drawArt(screen, 1280, 800, tile)
	}
	for i := 0; i < 5; i++ {
		frame()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame()
	}
}
