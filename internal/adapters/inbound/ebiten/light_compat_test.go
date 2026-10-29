package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	lightpkg "rimworld-go/internal/adapters/inbound/ebiten/render/light"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
)

// drawLit draws a world sprite lit as the ground under its foot at world
// (wx, wy) is (light.Lit.Draw), in the renderer's pass: a bake, the
// shadow pass's cast, a figure's blood.
func (r *Renderer) drawLit(dst, img *ebiten.Image, geo ebiten.GeoM, cs spritepkg.ColorScale, wx, wy float64) {
	r.litLayer().Draw(r.lightView(), dst, img, geo, cs, wx, wy, r.cast, r.lod.Baking, r.figureBlood)
}

// groundLamp ramps a ground quad's lamp light (light.Lit.GroundLamp).
func (r *Renderer) groundLamp(vs *[4]ebiten.Vertex) { r.litLayer().GroundLamp(r.lightView(), vs) }

// syncLight points the light grid at this frame's world (light.Lit.Sync).
func (r *Renderer) syncLight() *lightpkg.Grid {
	r.view.Art = r.art
	return r.litLayer().Sync(&r.view)
}

// lightTile is the light at tile (tx, ty) (light.Lit.LightTile).
func (r *Renderer) lightTile(tx, ty int) (sky, lamp float32) {
	r.view.Art = r.art
	return r.litLayer().LightTile(&r.view, tx, ty)
}

// drawShadows draws the frame's shadow layer onto screen
// (light.Shadows.Draw); the renderer lists the casters (shadow.go).
func (r *Renderer) drawShadows(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	r.env.Shadow.Draw(r.lightView(), r.litLayer(), shadowHost{r}, r.shadowView(), screen, w, sw, sh, tile)
}

// pointLights lists the frame's creature-shadow lights near the view
// (light.Shadows.PointLights).
func (r *Renderer) pointLights() []lightpkg.PointLight {
	return r.env.Shadow.PointLights(r.lightView(), r.litLayer(), r.shadowView())
}

// addBox queues a box caster's swept footprint (light.AddBox).
func (r *Renderer) addBox(b *gfx.TriBatch, c lightpkg.Caster, l lightpkg.State, tile, a float64) {
	light.AddBox(r.lightView(), b, c, l, a)
}
