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
// (wx, wy) is (light.Lit.Draw), in the renderer's pass: a bake, a
// figure's blood.
func (r *Renderer) drawLit(dst, img *ebiten.Image, geo ebiten.GeoM, cs spritepkg.ColorScale, wx, wy float64) {
	r.pipe().Lit.Draw(r.liveView(), dst, img, geo, cs, wx, wy, r.pipe().LOD.Baking, r.figureBlood)
}

// groundLamp ramps a ground quad's lamp light (light.Lit.GroundLamp).
func (r *Renderer) groundLamp(vs *[4]ebiten.Vertex) { r.pipe().Lit.GroundLamp(r.liveView(), vs) }

// syncLight points the light grid at this frame's world (light.Lit.Sync).
func (r *Renderer) syncLight() *lightpkg.Grid {
	r.view.Art = r.art
	return r.pipe().Lit.Sync(&r.view)
}

// lightTile is the light at tile (tx, ty) (light.Lit.LightTile).
func (r *Renderer) lightTile(tx, ty int) (sky, lamp float32) {
	r.view.Art = r.art
	return r.pipe().Lit.LightTile(&r.view, tx, ty)
}

// drawShadows draws the frame's shadow layer onto screen
// (light.Shadows.Draw); the renderer lists the casters (shadow.go).
func (r *Renderer) drawShadows(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	r.env.Shadow.Draw(r.liveView(), r.pipe().Lit, shadowHost{r}, screen, w, sw, sh, tile)
}

// addBox queues a box caster's swept footprint (light.AddBox).
func (r *Renderer) addBox(b *gfx.TriBatch, c lightpkg.Caster, l lightpkg.State, tile, a float64) {
	light.AddBox(r.liveView(), b, c, l, a)
}

// shadowCasters lists the frame's box and blob casters (shadowHost.Objects)
// in the live frame view.
func (r *Renderer) shadowCasters(w game.WorldView, pawns []game.PawnView) []lightpkg.Caster {
	r.pipe()
	r.liveView()
	return shadowHost{r}.Objects(w, pawns)
}
