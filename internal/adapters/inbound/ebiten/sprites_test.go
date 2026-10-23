package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// TestRendererWithoutArtDrawsFallback: with no art catalog (the vendor
// pack is gone, PLAY-010) the colony still draws a frame: plain terrain
// and a coloured square per pawn, selection included, without panicking.
func TestRendererWithoutArtDrawsFallback(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 6, h: 4, pawns: []game.PawnView{{ID: 1, X: 2, Y: 2, Status: "alive"}}})
	if r.art != nil {
		t.Fatal("a fresh renderer has no art")
	}
	r.cam.zoom = 1
	r.hasSelect, r.selected = true, 1
	r.Draw(ebiten.NewImage(320, 240))
}
