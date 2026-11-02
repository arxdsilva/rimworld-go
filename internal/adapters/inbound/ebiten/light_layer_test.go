package ebiten

import (
	lightpkg "rimworld-go/internal/adapters/inbound/ebiten/render/light"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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

// drawShadows draws the frame's colonist shadows of w and the frame's
// pawns onto screen (world.Pipeline.ShadowFeet, light.Shadows.Draw).
func (r *Renderer) drawShadows(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	p := r.pipe()
	p.Env.SetView(r.liveView(), sw, sh, tile)
	st := &r.env.Shadow
	st.Feet = p.ShadowFeet(&w, r.view.Pawns, tile, st.Feet)
	st.Draw(r.liveView(), r.lit.Light, screen, st.Feet, w.DayOfYear)
}
