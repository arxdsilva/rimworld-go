package ebiten

import (
	_ "embed"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Water in the colony view (NG-1 §7.3): shallow and deep water tiles drawn
// through water.kage (UV ripple and caustic glints on the presentation
// clock), the shallow-over-deep edge, and shore foam where water meets
// ground. Without the shader the tile cycles through its four baked
// frames; Reduced Motion freezes both.

//go:embed water.kage
var waterKage []byte

// waterDeepBias marks a deep tile in the vertex x position (water.kage).
const waterDeepBias = 2048

// waterFrameTicks is how many presentation frames one baked water frame
// shows when the shader is missing.
const waterFrameTicks = 45

// waterRenderer is the shader state the Renderer keeps between frames.
type waterRenderer struct {
	shader *ebiten.Shader
	err    error
	verts  [4]ebiten.Vertex
	opts   ebiten.DrawTrianglesShaderOptions
	time   []float32 // the Time uniform, reused
	// onDraw, when set (tests), sees every water draw: shader true when it
	// went through water.kage.
	onDraw func(key string, shader bool)
}

// waterShader is water.kage, compiled on first use; nil when it fails.
func (r *Renderer) waterShader() *ebiten.Shader {
	if r.water.shader == nil && r.water.err == nil {
		r.water.shader, r.water.err = ebiten.NewShader(waterKage)
	}
	return r.water.shader
}

// waterClock is the Time uniform in seconds: the presentation clock at 60
// frames a second, 0 (still) with Reduced Motion.
func (r *Renderer) waterClock() float32 {
	if r.reducedMotion {
		return 0
	}
	return float32(r.present) / 60
}

// waterFrameKey is the baked water tile of terrain t (shallow_water or
// deep_water) at tile (x, y): its own variant, advancing with the
// presentation clock unless Reduced Motion holds it.
func (r *Renderer) waterFrameKey(t string, x, y int) string {
	v := int(tileHash(x, y) % soilVariants)
	if !r.reducedMotion {
		v = (v + int(r.present/waterFrameTicks)) % soilVariants
	}
	return artKey("terrain", t, v)
}

// drawWaterTile draws the water of terrain t at tile (x, y), lit as flat
// ground: through water.kage when it compiles, else the baked frame.
// cs is the colour scale of the tile (a missing-art fallback tint).
func (r *Renderer) drawWaterTile(screen *ebiten.Image, t string, x, y int, tile float64, sw, sh int, cs colorScale) {
	wx, wy := float64(x), float64(y)
	sh0 := r.waterShader()
	if sh0 == nil || r.cast != nil { // the baked frame, drawn like any ground tile
		key := r.waterFrameKey(t, x, y)
		if r.water.onDraw != nil {
			r.water.onDraw(key, false)
		}
		r.drawTile(screen, key, wx, wy, tile, sw, sh, cs)
		return
	}
	key := artKey("terrain", t, int(tileHash(x, y)%soilVariants))
	sx, sy := r.worldToScreenF(wx, wy)
	x0, y0 := math.Floor(float64(sx)), math.Floor(float64(sy))
	size := math.Ceil(float64(sx)+tile) - x0 + 1
	if !visible(float32(x0), float32(y0), float32(size), float32(size), sw, sh) {
		return
	}
	img := r.art.image(key, color.NRGBA{})
	if img == nil {
		r.drawTile(screen, key, wx, wy, tile, sw, sh, cs) // missing art: the tinted fallback
		return
	}
	if r.water.onDraw != nil {
		r.water.onDraw(key, true)
	}
	k := size / float64(img.Bounds().Dx())
	geo := ebiten.GeoM{}
	geo.Scale(k, k)
	geo.Translate(x0, y0)
	if cs == (colorScale{}) {
		cs = noCS
	}
	sky, lamp := r.lightTile(x, y)
	b := img.Bounds()
	litQuad(&r.water.verts, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), geo, cs, sky, lamp, 0)
	bias := float32(0)
	if t == "deep_water" {
		bias = waterDeepBias
	}
	for i := range r.water.verts { // Custom2, Custom3: the world position of the corner
		r.water.verts[i].Custom2 = float32(x+i&1) + bias
		r.water.verts[i].Custom3 = float32(y + i>>1)
	}
	r.syncWaterUniforms()
	r.water.opts.Images[0] = img
	screen.DrawTrianglesShader32(r.water.verts[:], litIndices[:], sh0, &r.water.opts)
}

// syncWaterUniforms copies this frame's light and clock into the water
// draw's uniform map (the light names are lit.kage's).
func (r *Renderer) syncWaterUniforms() {
	r.lit.syncUniforms() // fills r.lit.uniforms from r.lit.light
	if r.water.opts.Uniforms == nil {
		r.water.opts.Uniforms = map[string]any{}
		r.water.time = make([]float32, 1)
		for i, name := range [...]string{"SunL", "MoonL", "SunColor", "MoonColor", "SkyAmbient", "IndoorAmbient", "LampColor"} {
			r.water.opts.Uniforms[name] = r.lit.uniforms[i]
		}
		r.water.opts.Uniforms["Time"] = float32(0)
	}
	r.water.opts.Uniforms["Time"] = r.waterClock()
}

// edgeMask is the wall-style mask (N=1, E=2, S=4, W=8) of the neighbour
// priorities (N, E, S, W) that satisfy match.
func edgeMask(nbPri [4]int, match func(p int) bool) int {
	mask := 0
	for i, bit := range [4]int{wallN, wallE, wallS, wallW} {
		if match(nbPri[i]) {
			mask |= bit
		}
	}
	return mask
}

// drawWaterEdges draws the overlays of a water tile at (x, y), after its
// base: shallow water's soft edge over a deep tile, and the foam where
// water meets ground (nbPri: terrain priorities N, E, S, W).
func (r *Renderer) drawWaterEdges(screen *ebiten.Image, t string, nbPri [4]int, x, y int, tile float64, sw, sh int) {
	none := color.NRGBA{}
	if t == "deep_water" {
		if m := edgeMask(nbPri, func(p int) bool { return p == terrainPriority["shallow_water"] }); m != 0 {
			// snapped with the tile's 1 px overlap, so no seam shows the deep base
			r.drawTile(screen, "terrain/shallow_water_edge_"+twoDigits(m), float64(x), float64(y), tile, sw, sh, noCS)
		}
	}
	if m := edgeMask(nbPri, func(p int) bool { return p >= 0 }); m != 0 {
		r.drawSpriteCS(screen, "terrain/shore_foam_edge_"+twoDigits(m), none, float64(x), float64(y), tile, sw, sh, r.foamCS(x, y))
	}
}

// foamCS is the foam's colour scale: its opacity breathes with the
// swell, each tile on its own phase; steady with Reduced Motion.
func (r *Renderer) foamCS(x, y int) colorScale {
	if r.reducedMotion {
		return colorScale{1, 1, 1, 0.85}
	}
	phase := float64(tileHash(x, y)%628) / 100
	a := 0.78 + 0.22*math.Sin(float64(r.present)/38+phase)
	return colorScale{1, 1, 1, float32(a)}
}
