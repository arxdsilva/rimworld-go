package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Blood on screen (SIM-020 / ART-013): stains on the ground (procedural
// splat, pool and drip textures, faded by their amount and dried by
// age), blood on hurt creatures (lit.kage Custom3, set through
// r.figureBlood around figure layers), and sprays scaled by damage.

// drawStains draws the world's blood and ichor stains, after the ground
// cover and floors and before shadows. ART-013 BR: implement.
func (r *Renderer) drawStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {}

// encodeBlood packs a figure's stain for lit.kage Custom3:
// 32*ichor + 2*wetStep + coverage. ART-013 BR: implement.
func encodeBlood(coverage float64, wetStep int, ichor bool) float32 { return 0 }
