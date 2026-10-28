package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Lighting and shadows live in render/light (ARCH-004). These aliases and
// one-line wrappers keep the root's call sites unchanged while the layers
// move out one package at a time; each mover repoints its own files to
// light and the last one deletes this file (SOLID_GO.md §11).

type (
	litRenderer    = light.Lit
	lightGrid      = light.Grid
	lightState     = light.State
	lightSource    = light.Source
	pointLight     = light.PointLight
	litPairCache   = light.PairCache
	groundTile     = light.GroundTile
	shadowState    = light.Shadows[creature.FigurePose]
	shadowCast     = light.Cast
	shadowSpec     = light.ShadowSpec
	shadowCaster   = light.Caster
	creatureCaster = light.Creature[creature.FigurePose]
)

const (
	lightLamp         = light.Lamp
	lightCampfire     = light.Campfire
	campfireReach     = light.CampfireReach
	lampDefaultRadius = light.LampDefaultRadius
	campfireLightH    = light.CampfireLightH
	lampLightH        = light.LampLightH
	lampLightReach    = light.LampLightReach
	lyingKappa        = light.LyingKappa
	shadowNone        = light.ShadowNone
	shadowBlob        = light.ShadowBlob
	shadowBox         = light.ShadowBox
	shadowMaxLen      = light.ShadowMaxLen
	projShearK        = light.ProjShearK
	litGap            = light.LitGap
	litPairsMin       = light.PairsMin
	litPairsMax       = light.PairsMax
)

var (
	litKage     = light.LitKage
	litIndices  = light.Indices
	shadowColor = light.ShadowColor
	shadowBlend = light.ShadowBlend
)

func lightAt(hour float64, dayOfYear int, weather string, intensity float64) lightState {
	return light.LightAt(hour, dayOfYear, weather, intensity)
}

func lum3(a [3]float32) float32 { return light.Lum3(a) }

func lerp3(a, b [3]float32, t float32) [3]float32 { return light.Lerp3(a, b, t) }

func litFactor(ls *lightState, n [3]float32, sky, lamp float32) [3]float32 {
	return light.LitFactor(ls, n, sky, lamp)
}

func flatLitScale(ls *lightState, cs colorScale, sky, lamp float32) [4]float32 {
	return light.FlatLitScale(ls, cs, sky, lamp)
}

func decodeNormal(texel [4]float32, has bool) [3]float32 { return light.DecodeNormal(texel, has) }

func decodeNormalMirrored(texel [4]float32, has, mirror bool) [3]float32 {
	return light.DecodeNormalMirrored(texel, has, mirror)
}

func mirrorNormalOff(off float32, geo ebiten.GeoM) float32 { return light.MirrorNormalOff(off, geo) }

func litQuad(vs *[4]ebiten.Vertex, sx, sy, w, h float32, geo ebiten.GeoM, cs colorScale, sky, lamp, normalOff float32) {
	light.LitQuad(vs, sx, sy, w, h, geo, cs, sky, lamp, normalOff)
}

func litFirst[T any](s []T) *T { return light.First(s) }

func cornerRow(dst []float32, above, below []groundTile) []float32 {
	return light.CornerRow(dst, above, below)
}

func isGroundKey(key string) bool { return light.IsGroundKey(key) }

func appendLightSources(dst []lightSource, w *game.WorldView, lampRadius float64) []lightSource {
	return light.AppendSources(dst, w, lampRadius)
}

func lampRadiusOf(c *artCatalog) float64 { return light.LampRadiusOf(c) }

func lampLevel(v float32) float32 { return light.LampLevel(v) }

func litPairCapFor(visible int) int { return light.PairCapFor(visible) }

func skyOverlay(t clock.Tick) color.RGBA { return light.SkyOverlay(t) }

func shadowFallback(key string, m *artMeta) shadowSpec { return light.ShadowFallback(key, m) }

func spriteKappa(h, opaquePx, ppt float64) float64 { return light.SpriteKappa(h, opaquePx, ppt) }

func shadowProjection(pivotX, pivotY, kappa float64, l lightState, hRef, rho float64) ebiten.GeoM {
	return light.ShadowProjection(pivotX, pivotY, kappa, l, hRef, rho)
}

func shadowCompositeScale(l lightState) colorScale { return light.ShadowCompositeScale(l) }

func roofRuns(w game.WorldView, x0, y0, x1, y1 int, out [][3]int) [][3]int {
	return light.RoofRuns(w, x0, y0, x1, y1, out)
}

// litLayer is the renderer's light layer with the fire light hooked in
// (a burning cell's reach and strength live with the fires,
// fire_world_draw.go).
func (r *Renderer) litLayer() *litRenderer {
	if r.lit.Blaze == nil {
		r.lit.Blaze = blazeLight
	}
	return &r.lit
}

// lightView is the frame view the light layer reads, with the renderer's
// live art, presentation frame and, outside a bake, camera copied in:
// tests drive these entry points without Renderer.Draw filling the view,
// and a bake moves r.cam but never the view.
func (r *Renderer) lightView() *render.View {
	v := &r.view
	v.Art, v.Present = r.art, r.present
	if !r.lod.baking {
		v.Cam = r.cam
	}
	return v
}

// litShader is lit.kage, compiled on first use (light.Lit.LitShader).
func (r *Renderer) litShader() *ebiten.Shader { return r.lit.LitShader() }

// drawLit draws a world sprite lit as the ground under its foot at world
// (wx, wy) is (light.Lit.Draw), in the renderer's pass: a bake, the
// shadow pass's cast, a figure's blood.
func (r *Renderer) drawLit(dst, img *ebiten.Image, geo ebiten.GeoM, cs colorScale, wx, wy float64) {
	r.litLayer().Draw(r.lightView(), dst, img, geo, cs, wx, wy, r.cast, r.lod.baking, r.figureBlood)
}

// drawLitGround is drawLit for a ground plane (light.Lit.DrawGround).
func (r *Renderer) drawLitGround(dst, img *ebiten.Image, geo ebiten.GeoM, cs colorScale, wx, wy float64) {
	r.litLayer().DrawGround(r.lightView(), dst, img, geo, cs, wx, wy, r.cast, r.lod.baking, r.figureBlood)
}

// litSource is the image a lit draw of img samples (light.Lit.SourceOf).
func (r *Renderer) litSource(img *ebiten.Image) (src *ebiten.Image, off float32, ground bool) {
	return r.litLayer().SourceOf(r.lightView(), img)
}

// groundLamp ramps a ground quad's lamp light (light.Lit.GroundLamp).
func (r *Renderer) groundLamp(vs *[4]ebiten.Vertex) { r.litLayer().GroundLamp(r.lightView(), vs) }

// syncLight points the light grid at this frame's world (light.Lit.Sync).
func (r *Renderer) syncLight() *lightGrid {
	r.view.Art = r.art
	return r.litLayer().Sync(&r.view)
}

// lightTile is the light at tile (tx, ty) (light.Lit.LightTile).
func (r *Renderer) lightTile(tx, ty int) (sky, lamp float32) {
	r.view.Art = r.art
	return r.litLayer().LightTile(&r.view, tx, ty)
}

// shadowView is the visible rectangle (setView) as the shadow pass reads it.
func (r *Renderer) shadowView() light.Rect {
	v := r.env.View
	return light.Rect{X0: v.X0, Y0: v.Y0, X1: v.X1, Y1: v.Y1}
}

// drawShadows draws the frame's shadow layer onto screen
// (light.Shadows.Draw); the renderer lists the casters (shadow.go).
func (r *Renderer) drawShadows(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	r.env.Shadow.Draw(r.lightView(), r.litLayer(), shadowHost{r}, r.shadowView(), screen, w, sw, sh, tile)
}

// pointLights lists the frame's creature-shadow lights near the view
// (light.Shadows.PointLights).
func (r *Renderer) pointLights() []pointLight {
	return r.env.Shadow.PointLights(r.lightView(), r.litLayer(), r.shadowView())
}

// addBox queues a box caster's swept footprint (light.AddBox).
func (r *Renderer) addBox(b *triBatch, c shadowCaster, l lightState, tile, a float64) {
	light.AddBox(r.lightView(), b, c, l, a)
}
