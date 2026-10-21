package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Moving shadows (ART §6.1): every caster (metadata shadow.kind blob or
// box) on open-sky ground casts a soft shadow away from the sun (or the
// moon), long at dawn and dusk, short at noon. Shadows are drawn into one
// offscreen layer and composited at a single alpha, so overlaps never
// double-darken, after floors and zones and before every object.

// drawShadows draws the frame's shadow layer onto screen.
func (r *Renderer) drawShadows(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	// ART-010 W5
}
