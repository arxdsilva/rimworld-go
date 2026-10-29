package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
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
	r.pipe().Lit.Draw(r.liveView(), dst, img, geo, cs, wx, wy, r.cast, r.pipe().LOD.Baking, r.figureBlood)
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
	r.env.Shadow.Draw(r.liveView(), r.pipe().Lit, shadowHost{r}, shadowRect(r), screen, w, sw, sh, tile)
}

// pointLights lists the frame's creature-shadow lights near the view
// (light.Shadows.PointLights).
func (r *Renderer) pointLights() []lightpkg.PointLight {
	return r.env.Shadow.PointLights(r.liveView(), r.pipe().Lit, shadowRect(r))
}

// addBox queues a box caster's swept footprint (light.AddBox).
func (r *Renderer) addBox(b *gfx.TriBatch, c lightpkg.Caster, l lightpkg.State, tile, a float64) {
	light.AddBox(r.liveView(), b, c, l, a)
}

// shadowRect is the visible rectangle (Env.SetView) as the shadow pass
// reads it.
func shadowRect(r *Renderer) light.Rect {
	v := r.env.View
	return light.Rect{X0: v.X0, Y0: v.Y0, X1: v.X1, Y1: v.Y1}
}

// shadowCasters lists the frame's box and blob casters (shadowHost.Objects)
// in the live frame view.
func (r *Renderer) shadowCasters(w game.WorldView, pawns []game.PawnView) []lightpkg.Caster {
	r.pipe()
	r.liveView()
	return shadowHost{r}.Objects(w, pawns)
}

// creatureCasters lists the frame's casting pawns and animals
// (shadowHost.Creatures) in the live frame view.
func (r *Renderer) creatureCasters(w game.WorldView, pawns []game.PawnView) []light.Creature[creaturepkg.FigurePose] {
	r.pipe()
	r.liveView()
	return shadowHost{r}.Creatures(w, pawns)
}
