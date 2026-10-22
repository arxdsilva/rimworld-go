package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Realistic creature shadows (ART-012): pawns and animals cast their own
// silhouette, drawn by the same figure code with r.cast set, projected
// from the feet away from the light, softened with distance by
// shadow_blur.kage, plus a contact shadow and campfire/lamp shadows.

// shadowCast, while set on the renderer, makes drawLit draw silhouettes
// into the shadow layer instead of lit sprites onto the screen.
type shadowCast struct {
	layer *ebiten.Image // the frame's shadow layer
	proj  ebiten.GeoM   // pivot-anchored projection for the current caster
	alpha float32       // this light's strength (sun, moon or a point light)
}

// castSilhouette draws img (placed by geo) as a projected silhouette into
// r.cast's layer. ART-012 SH1: implement (height in R for the blur).
func (r *Renderer) castSilhouette(img *ebiten.Image, geo ebiten.GeoM) {}
