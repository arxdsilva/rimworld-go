package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func quadsOf(b *triBatch) int { return len(b.Idx) / 6 }

// A dashed run far longer than the view only emits the dashes on screen.
func TestDashedRunCullsOffscreen(t *testing.T) {
	const tile, dash, gap = 16.0, 6.0, 4.8
	var b triBatch
	// 400 tiles long, starting 100 tiles left of the view, 20 tiles visible
	dashedRun(&b, -100*tile, 50, 300*tile, 50, 2, dash, gap, stockEdgeClr, 20*tile, 400)
	want := 20 * tile / (dash + gap)
	if got := quadsOf(&b); float64(got) < want-2 || float64(got) > want+3 {
		t.Fatalf("dashes = %d, want about %.0f", got, want)
	}
	b.Reset()
	dashedRun(&b, 50, -100*tile, 50, 300*tile, 2, dash, gap, stockEdgeClr, 400, 20*tile) // vertical
	if got := quadsOf(&b); float64(got) < want-2 || float64(got) > want+3 {
		t.Fatalf("vertical dashes = %d, want about %.0f", got, want)
	}
	b.Reset()
	dashedRun(&b, 1e5, 50, 2e5, 50, 2, dash, gap, stockEdgeClr, 320, 200) // wholly off to the right
	if quadsOf(&b) != 0 {
		t.Fatalf("an off-screen run emitted %d dashes", quadsOf(&b))
	}
}

func zoneBatchRenderer(t *testing.T) (*Renderer, float64) {
	g := game.NewColony(game.Config{Seed: 7, Width: 40, Height: 30})
	r := New(g)
	tile := 4.0
	r.setView(640, 400, tile)
	return r, tile
}

// All the outlines of every zone leave in one flush, after the fills.
func TestZoneOutlinesOneFlush(t *testing.T) {
	r, tile := zoneBatchRenderer(t)
	var w game.WorldView
	for i := 0; i < 50; i++ {
		x, y := (i%10)*3, (i/10)*3
		w.Zones = append(w.Zones, rectZone(uint16(i+1), game.ZoneStockpile, x, y, x+1, y+1))
	}
	screen := ebiten.NewImage(640, 400)
	r.drawZones(screen, w, 640, 400, tile)
	if len(r.overlays().Zones.Batch.Idx) != 0 || len(r.overlays().Zones.Batch.Verts) != 0 {
		t.Fatalf("batch not flushed: %d indices left", len(r.overlays().Zones.Batch.Idx))
	}
	// the outlines did go through the batch
	for _, z := range w.Zones {
		for _, run := range r.zoneGeomOf(z).Runs {
			r.drawZoneRun(&r.overlays().Zones.Batch, z.Kind, run, 640, 400, tile)
		}
	}
	if quadsOf(&r.overlays().Zones.Batch) < 50*4 {
		t.Fatalf("only %d outline quads for 50 stockpiles", quadsOf(&r.overlays().Zones.Batch))
	}
}

// A fence run of N tiles is a rail plus N+1 posts.
func TestFenceRunPosts(t *testing.T) {
	r, tile := zoneBatchRenderer(t)
	var b triBatch
	r.drawFenceRun(&b, edgeRun{X0: 2, Y0: 3, X1: 9, Y1: 3, Ny: -1, StartConvex: true, EndConvex: true}, tile)
	if got := quadsOf(&b); got != 1+8 {
		t.Fatalf("quads = %d, want 1 rail + 8 posts", got)
	}
}
