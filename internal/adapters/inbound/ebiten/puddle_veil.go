package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// drawPuddleVeil draws the rain puddles (PLAY-022): one smooth layer over
// the ground and under the grass tufts, at every zoom tier.
func (r *Renderer) drawPuddleVeil(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
}
