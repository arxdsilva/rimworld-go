package ebiten

import (
	_ "embed"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Dynamic light (ART §6.1): a sun that rises in the east, crosses the
// southern sky and sets in the west, a low cool moon at night, indoor
// ambient under roofs and warm lamp light in lit rooms. Every world sprite
// is drawn through drawLit, which shades its normal map (made by
// pkg/spritenormals, a port of spritefusion-sprite-normals) with lit.kage.
// The light comes from the sim clock only, so captures are deterministic.

//go:embed lit.kage
var litKage []byte

// lightState is the light for one frame. Vectors point toward the light
// in screen space (+x right, +y down, +z toward the viewer); colours are
// linear multipliers (1 = the sprite's own colour).
type lightState struct {
	SunL, MoonL               [3]float32
	SunColor, MoonColor       [3]float32
	SkyAmbient, IndoorAmbient [3]float32
	LampColor                 [3]float32
	ShadowDir                 [2]float64 // unit ground direction shadows fall
	ShadowLen                 func(h float64) float64
	ShadowAlpha               float64 // 0 when no shadows
}

// lightAt is the light at hour (0..24) on dayOfYear in the given weather.
func lightAt(hour float64, dayOfYear int, weather string, intensity float64) lightState {
	// ART-010 W4: the sun, moon and colour keys of ART §6.1.
	one := [3]float32{1, 1, 1}
	return lightState{SkyAmbient: one, IndoorAmbient: one, ShadowLen: func(float64) float64 { return 0 }}
}

// litShader is lit.kage, compiled on first use.
func (r *Renderer) litShader() *ebiten.Shader {
	if r.lit.shader == nil && r.lit.err == nil {
		r.lit.shader, r.lit.err = ebiten.NewShader(litKage)
	}
	return r.lit.shader
}

// litRenderer is the lighting state the Renderer keeps between frames.
type litRenderer struct {
	shader *ebiten.Shader
	err    error
	light  lightState // this frame's light (set at the start of drawArt)
}

// lightTile is the light at tile (tx, ty): sky 1 under open sky, 0 under
// a roof; lamp 0..1 from lit rooms and nearby campfires.
func (r *Renderer) lightTile(tx, ty int) (sky, lamp float32) {
	return 1, 0 // ART-010 W4
}

// drawLit draws a world sprite img with geo and colour scale cs, lit as
// the ground tile (tx, ty) is (its normal map comes from the catalog; a
// sprite without one is lit flat).
func (r *Renderer) drawLit(dst, img *ebiten.Image, geo ebiten.GeoM, cs colorScale, tx, ty int) {
	// ART-010 W4: DrawTrianglesShader with lit.kage, sky/lamp per vertex.
	op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
	if cs != noCS && cs != (colorScale{}) {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	dst.DrawImage(img, op)
}
