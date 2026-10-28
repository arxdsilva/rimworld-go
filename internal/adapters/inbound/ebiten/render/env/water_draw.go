package env

import (
	_ "embed"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
)

// Water in the colony view (NG-1 §7.3): shallow and deep water tiles drawn
// through water.kage (UV ripple and caustic glints on the presentation
// clock), the shallow-over-deep edge, and shore foam where water meets
// ground. Without the shader the tile cycles through its four baked
// frames; Reduced Motion freezes both. Simple water (Options > Graphics,
// PLAY-019) is still too: one baked frame per tile, no ripple shader.

// WaterKage is the water shader's source.
//
//go:embed water.kage
var WaterKage []byte

// waterDeepBias marks a deep tile in the vertex x position (water.kage).
const waterDeepBias = 2048

// WaterFrameTicks is how many presentation frames one baked water frame
// shows when the shader is missing.
const WaterFrameTicks = 45

// Water is the water shader's state between frames.
type Water struct {
	Shader *ebiten.Shader
	Err    error
	verts  [4]ebiten.Vertex
	opts   ebiten.DrawTrianglesShaderOptions
	time   []float32 // the Time uniform, reused
	// Simple draws still baked frames instead of the ripple shader.
	Simple bool
	// OnDraw, when set (tests), sees every water draw: shader true when it
	// went through water.kage.
	OnDraw func(key string, shader bool)
}

// WaterShader is water.kage, compiled on first use; nil when it fails.
func (wr *Water) WaterShader() *ebiten.Shader {
	if wr.Shader == nil && wr.Err == nil {
		wr.Shader, wr.Err = ebiten.NewShader(WaterKage)
	}
	return wr.Shader
}

// waterStill reports water that does not move: Reduced Motion, or simple
// water.
func (wr *Water) waterStill(v *render.View) bool { return v.ReducedMotion || wr.Simple }

// WaterClock is the Time uniform in seconds: the presentation clock at 60
// frames a second, 0 (still) with Reduced Motion or simple water.
func (wr *Water) WaterClock(v *render.View) float32 {
	if wr.waterStill(v) {
		return 0
	}
	return float32(v.Present) / 60
}

// WaterFrameKey is the baked water tile of terrain t (shallow_water or
// deep_water) at tile (x, y): its own variant, advancing with the
// presentation clock unless the water is still.
func (wr *Water) WaterFrameKey(v *render.View, t string, x, y int) string {
	n := int(sprite.TileHash(x, y) % SoilVariants)
	if !wr.waterStill(v) {
		n = (n + int(v.Present/WaterFrameTicks)) % SoilVariants
	}
	return ArtKey("terrain", t, n)
}

// DrawWaterTile draws the water of terrain t at tile (x, y), lit as flat
// ground: through water.kage when it compiles, else the baked frame.
// cs is the colour scale of the tile (a missing-art fallback tint). A
// bake or the shadow pass (baked) draws the baked frame through s.
func (e *Layer) DrawWaterTile(v *render.View, screen *ebiten.Image, t string, x, y int, tile float64, sw, sh int, cs sprite.ColorScale, baked bool, s Stamp) {
	wx, wy := float64(x), float64(y)
	var sh0 *ebiten.Shader
	if !e.Water.Simple { // simple water never compiles the ripple shader
		sh0 = e.Water.WaterShader()
	}
	if sh0 == nil || baked { // the baked frame, drawn like any ground tile
		key := e.Water.WaterFrameKey(v, t, x, y)
		if e.Water.OnDraw != nil {
			e.Water.OnDraw(key, false)
		}
		s.DrawTile(screen, key, wx, wy, tile, sw, sh, cs)
		return
	}
	key := ArtKey("terrain", t, int(sprite.TileHash(x, y)%SoilVariants))
	sx, sy := v.WorldToScreenF(wx, wy)
	x0, y0 := math.Floor(float64(sx)), math.Floor(float64(sy))
	size := math.Ceil(float64(sx)+tile) - x0 + 1
	if !sprite.Visible(float32(x0), float32(y0), float32(size), float32(size), sw, sh) {
		return
	}
	img := v.Art.Image(key, color.NRGBA{})
	if img == nil {
		s.DrawTile(screen, key, wx, wy, tile, sw, sh, cs) // missing art: the tinted fallback
		return
	}
	if e.Water.OnDraw != nil {
		e.Water.OnDraw(key, true)
	}
	k := size / float64(img.Bounds().Dx())
	geo := ebiten.GeoM{}
	geo.Scale(k, k)
	geo.Translate(x0, y0)
	if cs == (sprite.ColorScale{}) {
		cs = sprite.NoCS
	}
	sky, lamp := e.Lit.LightTile(v, x, y)
	b := img.Bounds()
	light.LitQuad(&e.Water.verts, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), geo, cs, sky, lamp, 0)
	bias := float32(0)
	if t == "deep_water" {
		bias = waterDeepBias
	}
	for i := range e.Water.verts { // Custom2, Custom3: the world position of the corner
		e.Water.verts[i].Custom2 = float32(x+i&1) + bias
		e.Water.verts[i].Custom3 = float32(y + i>>1)
	}
	e.Water.syncUniforms(v, e.Lit)
	e.Water.opts.Images[0] = img
	screen.DrawTrianglesShader32(e.Water.verts[:], light.Indices[:], sh0, &e.Water.opts)
}

// syncWaterUniforms copies this frame's light and clock into the water
// draw's uniform map (the light names are lit.kage's).
func (wr *Water) syncUniforms(v *render.View, lit *light.Lit) {
	lit.SyncUniforms() // fills the light layer's uniforms from its light
	if wr.opts.Uniforms == nil {
		wr.opts.Uniforms = map[string]any{}
		wr.time = make([]float32, 1)
		for i, name := range [...]string{"SunL", "MoonL", "SunColor", "MoonColor", "SkyAmbient", "IndoorAmbient", "LampColor"} {
			wr.opts.Uniforms[name] = lit.Uniforms[i]
		}
		wr.opts.Uniforms["Time"] = float32(0)
	}
	wr.opts.Uniforms["Time"] = wr.WaterClock(v)
}

// EdgeMask is the wall-style mask (N=1, E=2, S=4, W=8) of the neighbour
// priorities (N, E, S, W) that satisfy match.
func EdgeMask(nbPri [4]int, match func(p int) bool) int {
	mask := 0
	for i, bit := range [4]int{art.WallN, art.WallE, art.WallS, art.WallW} {
		if match(nbPri[i]) {
			mask |= bit
		}
	}
	return mask
}

// DrawWaterEdges draws the overlays of a water tile at (x, y), after its
// base: shallow water's soft edge over a deep tile, and the foam where
// water meets ground (nbPri: terrain priorities N, E, S, W; shallowPri
// is shallow water's), in sprite pass ps; the shallow edge draws through s.
func (e *Layer) DrawWaterEdges(v *render.View, ps sprite.Pass, screen *ebiten.Image, t string, nbPri [4]int, shallowPri, x, y int, tile float64, sw, sh int, s Stamp) {
	none := color.NRGBA{}
	if t == "deep_water" {
		if m := EdgeMask(nbPri, func(p int) bool { return p == shallowPri }); m != 0 {
			// snapped with the tile's 1 px overlap, so no seam shows the deep base
			s.DrawTile(screen, "terrain/shallow_water_edge_"+sprite.TwoDigits(m), float64(x), float64(y), tile, sw, sh, sprite.NoCS)
		}
	}
	if m := EdgeMask(nbPri, func(p int) bool { return p >= 0 }); m != 0 {
		e.P.DrawCS(v, ps, screen, "terrain/shore_foam_edge_"+sprite.TwoDigits(m), none, float64(x), float64(y), tile, sw, sh, e.Water.FoamCS(v, x, y))
	}
}

// FoamCS is the foam's colour scale: its opacity breathes with the
// swell, each tile on its own phase; steady with still water.
func (wr *Water) FoamCS(v *render.View, x, y int) sprite.ColorScale {
	if wr.waterStill(v) {
		return sprite.ColorScale{1, 1, 1, 0.85}
	}
	phase := float64(sprite.TileHash(x, y)%628) / 100
	a := 0.78 + 0.22*math.Sin(float64(v.Present)/38+phase)
	return sprite.ColorScale{1, 1, 1, float32(a)}
}
