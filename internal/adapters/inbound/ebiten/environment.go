package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Environment presentation (ART §27.3, §16): the wind field, plants and
// grass waving in it and bending around creatures, rain and snow,
// snow cover and puddles, seasonal looks. Presentation-only: reads
// WorldView weather and creature motion, never feeds the simulation.

// plantKind groups vegetation by how it moves.
type plantKind uint8

const (
	kindGrass plantKind = iota
	kindCrop
	kindBush
	kindTree
)

// envState is the environment's presentation state, kept on the Renderer.
type envState struct{}

// windAt is the local wind at world (x, y) in tiles per second: the
// world's wind plus travelling gusts.
func (r *Renderer) windAt(x, y float64) (vx, vy float64) { return 0, 0 }

// plantPose is how a plant (or grass tuft) at tile (x, y) leans this
// frame: wind plus any creature brushing through. scale is its growth
// scale.
func (r *Renderer) plantPose(x, y int, kind plantKind, amount, scale float64) pose {
	return pose{px: 0.5, py: 1, sx: scale, sy: scale, skew: r.sway(x, y, amount)}
}

// seasonKey swaps a plant sprite for its seasonal look (autumn, bare).
func (r *Renderer) seasonKey(key string, w game.WorldView) string { return key }

// stepEnvironment advances springs and weather particles once per frame.
func (r *Renderer) stepEnvironment(w game.WorldView, pawns []game.PawnView) {}

// drawGroundCover draws snow cover and puddles over the terrain.
func (r *Renderer) drawGroundCover(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
}

// drawWeather draws falling rain or snow and the storm's darkening.
func (r *Renderer) drawWeather(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {}
