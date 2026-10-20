package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Burial presentation (SIM-017, ART §28): human corpses drawn from the
// deceased's own look, graves with headstones, the funeral pyre.

// drawCorpseItem draws a human corpse (or bones) lying on its tile.
func (r *Renderer) drawCorpseItem(screen *ebiten.Image, it game.ItemView, tile float64, sw, sh int) {
	r.drawSpriteScaled(screen, itemSprite(it.Kind), color.NRGBA{}, float64(it.X), float64(it.Y), tile, sw, sh, itemCS(it), r.itemPop(it.ID))
}

// drawGrave draws a dug or filled grave and its headstone.
func (r *Renderer) drawGrave(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	r.drawSprite(screen, buildSpriteStuff(b.Kind, b.Stuff, b.Facing, 0, false, false), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
}

// drawPyre draws the funeral pyre, burning while it cremates.
func (r *Renderer) drawPyre(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	r.drawSprite(screen, buildSpriteStuff(b.Kind, b.Stuff, b.Facing, 0, false, false), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
}
