package overlay

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"testing"
)

func quadsOf(b *gfx.TriBatch) int { return len(b.Idx) / 6 }

// A dashed run far longer than the view only emits the dashes on screen.
func TestDashedRunCullsOffscreen(t *testing.T) {
	const tile, dash, gap = 16.0, 6.0, 4.8
	var b gfx.TriBatch
	// 400 tiles long, starting 100 tiles left of the view, 20 tiles visible
	DashedRun(&b, -100*tile, 50, 300*tile, 50, 2, dash, gap, StockEdgeClr, 20*tile, 400)
	want := 20 * tile / (dash + gap)
	if got := quadsOf(&b); float64(got) < want-2 || float64(got) > want+3 {
		t.Fatalf("dashes = %d, want about %.0f", got, want)
	}
	b.Reset()
	DashedRun(&b, 50, -100*tile, 50, 300*tile, 2, dash, gap, StockEdgeClr, 400, 20*tile) // vertical
	if got := quadsOf(&b); float64(got) < want-2 || float64(got) > want+3 {
		t.Fatalf("vertical dashes = %d, want about %.0f", got, want)
	}
	b.Reset()
	DashedRun(&b, 1e5, 50, 2e5, 50, 2, dash, gap, StockEdgeClr, 320, 200) // wholly off to the right
	if quadsOf(&b) != 0 {
		t.Fatalf("an off-screen run emitted %d dashes", quadsOf(&b))
	}
}
