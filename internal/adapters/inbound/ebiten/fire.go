package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Campfire presentation: a flickering flame drawn over the stone ring,
// a warm glow, and rising embers and smoke while it burns.

// fireHeight lifts the flame (tiles) to sit on the logs.
const fireHeight = 0.22

// drawFires draws the flame and glow of every burning campfire.
func (r *Renderer) drawFires(screen *ebiten.Image, w game.WorldView, tile float64) {
	for _, b := range w.Buildings {
		if !b.Burning {
			continue
		}
		// ground glow, then tongues that sway and shrink upward, red to yellow
		t := float64(r.present) + float64(b.ID)*17
		r.drawFlame(screen, float64(b.X)+0.5, float64(b.Y)+0.5-fireHeight, t, tile, 1)
	}
}

// emitFireParticles sends embers and smoke up from burning campfires.
func (r *Renderer) emitFireParticles(w game.WorldView) {
	for _, b := range w.Buildings {
		if !b.Burning {
			continue
		}
		seed := uint64(b.ID)*104729 + r.present
		x, y := float64(b.X)+0.5, float64(b.Y)+0.5-fireHeight-0.2
		if (r.present+uint64(b.ID))%9 == 0 {
			r.burst(partEmber, x, y, 1, seed)
		}
		if (r.present+uint64(b.ID))%14 == 0 {
			r.burst(partSmoke, x, y-0.15, 1, seed^0x5bd1e995)
		}
	}
}
